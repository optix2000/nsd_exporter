package main

import (
	"bufio"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/go-kit/log/level"
	"github.com/optix2000/go-nsdctl"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/promlog"
	"github.com/prometheus/common/promlog/flag"
	"github.com/prometheus/common/version"
	"github.com/prometheus/exporter-toolkit/web"
	"github.com/prometheus/exporter-toolkit/web/kingpinflag"
)

// Prom stuff
var nsdToProm = strings.NewReplacer(".", "_")

var nsdUpDesc = prometheus.NewDesc(
	prometheus.BuildFQName("nsd", "", "up"),
	"Whether scraping nsd's metrics was successful.",
	nil, nil)

var metricConfiguration = &metricConfig{}

type NSDCollector struct {
	client  *nsdctl.NSDClient
	metrics map[string]*promMetric // Map of metric names to prom metrics
	typ     string
}

type promMetric struct {
	desc      *prometheus.Desc
	valueType prometheus.ValueType
	labels    []string
}

func (c *NSDCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- nsdUpDesc
	for _, metric := range c.metrics {
		ch <- metric.desc
	}
}

func (c *NSDCollector) Collect(ch chan<- prometheus.Metric) {
	r, err := c.client.Command("stats_noreset")
	if err != nil {
		ch <- prometheus.MustNewConstMetric(
			nsdUpDesc,
			prometheus.GaugeValue,
			0.0)
		slog.Error("Stats request failed", "err", err)
		return
	}
	ch <- prometheus.MustNewConstMetric(
		nsdUpDesc,
		prometheus.GaugeValue,
		1.0)

	s := bufio.NewScanner(r)
	for s.Scan() {
		line := strings.Split(s.Text(), "=")
		metricName := strings.TrimSpace(line[0])
		m, ok := c.metrics[metricName]
		if !ok {
			slog.Info("New metric found. Refreshing.", "name", metricName)
			// Try to update the metrics list
			err = c.updateMetric(s.Text())
			if err != nil {
				slog.Error("Update failed", "err", err)
			}
			// Refetch metric
			_, ok = c.metrics[metricName]
			if !ok {
				slog.Warn("Metric not configured. Skipping", "name", metricName)
			}
			continue
		}
		value, err := strconv.ParseFloat(line[1], 64)
		if err != nil {
			slog.Error("Parse error", "err", err)
			continue
		}
		metric, err := prometheus.NewConstMetric(m.desc, m.valueType, value, m.labels...)
		if err != nil {
			slog.Error("New const metric failed", "err", err)
			continue
		}
		ch <- metric
	}
	err = s.Err()
	if err != nil {
		slog.Error("Bufio error", "err", err)
		return
	}

}

func (c *NSDCollector) updateMetric(s string) error {
	// Assume line is in "metric=#" format
	line := strings.Split(s, "=")
	metricName := strings.TrimSpace(line[0])

	_, exists := c.metrics[metricName]
	if !exists {
		metricConf, ok := metricConfiguration.Metrics[metricName]
		if ok {
			promName := nsdToProm.Replace(line[0])
			c.metrics[metricName] = &promMetric{
				desc: prometheus.NewDesc(
					prometheus.BuildFQName(c.typ, "", promName),
					metricConf.Help,
					nil,
					nil,
				),
				valueType: metricConf.Type,
			}
		} else { // Try labeled metric
			for _, v := range metricConfiguration.LabelMetrics {
				labels := v.Regex.FindStringSubmatch(metricName)
				if labels != nil {
					var promName string
					if v.Name != "" {
						promName = v.Name
					} else {
						promName = nsdToProm.Replace(line[0])
					}
					c.metrics[metricName] = &promMetric{
						desc: prometheus.NewDesc(
							prometheus.BuildFQName(c.typ, "", promName),
							v.Help,
							v.Labels,
							nil,
						),
						valueType: v.Type,
						labels:    labels[1:],
					}
					// python "for-else"
					goto Found
				}
			}
			return fmt.Errorf("Metric %s not found in config.", metricName)
		Found:
		}
	}
	return nil
}

func (c *NSDCollector) initMetricsList() error {
	r, err := c.client.Command("stats_noreset")
	if err != nil {
		slog.Error("Stats request failed", "err", err)
		return err
	}

	if c.metrics == nil {
		c.metrics = make(map[string]*promMetric)
	}

	// Grab metrics
	s := bufio.NewScanner(r)
	for s.Scan() {
		err = c.updateMetric(s.Text())
		if err != nil {
			slog.Error("Bufio failed, Skipping.", "err", err)
		}
	}
	return s.Err()
}

func NewNSDCollector(nsdType string, hostString string, caPath string, keyPath string, certPath string, skipVerify bool) (*NSDCollector, error) {
	client, err := nsdctl.NewClient(nsdType, hostString, caPath, keyPath, certPath, skipVerify)
	if err != nil {
		return nil, err
	}

	collector := &NSDCollector{
		client: client,
		typ:    nsdType,
	}

	err = collector.initMetricsList()
	if err != nil {
		slog.Error("Init failed", "err", err)
		return nil, err
	}
	return collector, err
}

func NewNSDCollectorFromConfig(path string) (*NSDCollector, error) {
	client, err := nsdctl.NewClientFromConfig(path)
	if err != nil {
		return nil, err
	}

	collector := &NSDCollector{
		client: client,
		typ:    "nsd",
	}

	err = collector.initMetricsList()
	if err != nil {
		slog.Error("Init failed", "err", err)
		return nil, err
	}
	return collector, err
}

// Main
func main() {
	var (
		metricsPath      = kingpin.Flag("web.telemetry-path", "The path to export Prometheus metrics to.").Default("/metrics").String()
		metricConfigPath = kingpin.Flag("metrics-config", "Mapping file for metrics. Defaults to built in file for NSD 4.1.x. This allows you to add or change any metrics that this scrapes").String()
		nsdConfig        = kingpin.Flag("nsd.config", "Configuration file for nsd/unbound to autodetect configuration from. Mutually exclusive with --control.address, -control.cert, --control.key and --control.ca").Default("/etc/nsd/nsd.conf").String()
		nsdType          = kingpin.Flag("type", "What nsd-like daemon to scrape (nsd or unbound). Defaults to nsd").Default("nsd").Enum("nsd", "unbound")
		nsdAddr          = kingpin.Flag("control.address", "NSD or Unbound control socket address.").String()
		cert             = kingpin.Flag("control.cert", "Client cert file location. Mutually exclusive with --nsd.config.").ExistingFile()
		key              = kingpin.Flag("control.key", "Client key file location. Mutually exclusive with --nsd.config.").ExistingFile()
		ca               = kingpin.Flag("control.ca", "Server CA file location. Mutually exclusive with --nsd.config.").ExistingFile()
		toolkitFlags     = kingpinflag.AddFlags(kingpin.CommandLine, ":9167")
	)

	promlogConfig := &promlog.Config{}
	flag.AddFlags(kingpin.CommandLine, promlogConfig)
	kingpin.Version(version.Print("nsd_exporter"))
	kingpin.CommandLine.UsageWriter(os.Stdout)
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()
	logger := promlog.New(promlogConfig)

	// Load config
	err := loadConfig(*metricConfigPath, metricConfiguration)
	if err != nil {
		slog.Error("Failed to load config", "err", err)
		os.Exit(1)
	}

	// If one is set, all must be set.
	var nsdCollector *NSDCollector
	if *cert != "" || *key != "" || *ca != "" || *nsdAddr != "" {
		if *cert != "" && *key != "" && *ca != "" && *nsdAddr != "" {
			// Build from arguments
			nsdCollector, err = NewNSDCollector(*nsdType, *nsdAddr, *ca, *key, *cert, false)
			if err != nil {
				slog.Error("Failed to create collector", "err", err)
				os.Exit(1)
			}
		} else {
			slog.Error("--control.address, --control.cert, --control.key, and --control.ca must all be defined.")
			os.Exit(1)
		}
	} else {
		// Build from config
		nsdCollector, err = NewNSDCollectorFromConfig(*nsdConfig)
		if err != nil {
			slog.Error("Failed to create collector", "err", err)
			os.Exit(1)
		}
	}

	_ = level.Info(logger).Log("msg", "Starting nsd_exporter", "version", version.Info())
	_ = level.Info(logger).Log("msg", "Build context", "context", version.BuildContext())

	prometheus.MustRegister(nsdCollector)
	http.Handle(*metricsPath, promhttp.Handler())
	if *metricsPath != "/" && *metricsPath != "" {
		landingPage, err := web.NewLandingPage(
			web.LandingConfig{
				Name:        "NSd Exporter",
				Description: "NSd Exporter for Prometheus",
				Version:     version.Info(),
				Links: []web.LandingLinks{
					{Address: *metricsPath, Text: "Metrics"},
				},
			})
		if err != nil {
			slog.Error("Failed to create landing page", "err", err)
			os.Exit(1)
		}
		http.Handle("/", landingPage)
	}

	server := new(http.Server)
	err = web.ListenAndServe(server, toolkitFlags, logger)
	if err != nil {
		slog.Error("Server error", "err", err)
		os.Exit(1)
	}
}
