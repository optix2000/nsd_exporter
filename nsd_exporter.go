package main

//go:generate go-bindata --prefix config/ config/

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/optix2000/go-nsdctl"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Args
var listenAddr = flag.String("listen-address", ":8080", "The address to listen on for HTTP requests.")
var metricPath = flag.String("metric-path", "/metrics", "The path to export Prometheus metrics to.")
var metricConfigPath = flag.String("metric-config", "", "Mapping file for metrics. Defaults to built in file for NSD 4.1.x. This allows you to add or change any metrics that this scrapes")
var nsdConfig = flag.String("config-file", "/etc/nsd/nsd.conf", "Configuration file for nsd/unbound to autodetect configuration from. Defaults to /etc/nsd/nsd.conf. Mutually exclusive with -nsd-address, -cert, -key and -ca")
var nsdType = flag.String("type", "nsd", "What nsd-like daemon to scrape (nsd or unbound). Defaults to nsd")
var cert = flag.String("cert", "", "Client cert file location. Mutually exclusive with -config-file.")
var key = flag.String("key", "", "Client key file location. Mutually exclusive with -config-file.")
var ca = flag.String("ca", "", "Server CA file location. Mutually exclusive with -config-file.")
var nsdAddr = flag.String("nsd-address", "", "NSD or Unbound control socket address.")

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
		log.Println(err)
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
			log.Println("New metric " + metricName + " found. Refreshing.")
			// Try to update the metrics list
			err = c.updateMetric(s.Text())
			if err != nil {
				log.Println(err.Error())
			}
			// Refetch metric
			m, ok = c.metrics[metricName]
			if !ok {
				log.Println("Metric " + metricName + " not configured. Skipping")
			}
			continue
		}
		value, err := strconv.ParseFloat(line[1], 64)
		if err != nil {
			log.Println(err)
			continue
		}
		metric, err := prometheus.NewConstMetric(m.desc, m.valueType, value, m.labels...)
		if err != nil {
			log.Println(err)
			continue
		}
		ch <- metric
	}
	err = s.Err()
	if err != nil {
		log.Println(err)
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
					prometheus.BuildFQName(*nsdType, "", promName),
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
							prometheus.BuildFQName(*nsdType, "", promName),
							v.Help,
							v.Labels,
							nil,
						),
						valueType: v.Type,
						labels:    labels[1:len(labels)],
					}
					// python "for-else"
					goto Found
				}
			}
			return fmt.Errorf("Metric ", metricName, " not found in config.")
		Found:
		}
	}
	return nil
}

func (c *NSDCollector) initMetricsList() error {
	r, err := c.client.Command("stats_noreset")
	if err != nil {
		log.Println(err)
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
			log.Println(err.Error(), "Skipping.")
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
	}

	err = collector.initMetricsList()
	if err != nil {
		log.Println(err)
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
	}

	err = collector.initMetricsList()
	if err != nil {
		log.Println(err)
		return nil, err
	}
	return collector, err
}

// Main

func main() {
	flag.Parse()

	// Load config
	err := loadConfig(*metricConfigPath, metricConfiguration)
	if err != nil {
		log.Fatal(err)
	}

	// If one is set, all must be set.
	var nsdCollector *NSDCollector
	if *cert != "" || *key != "" || *ca != "" || *nsdAddr != "" {
		if *cert != "" && *key != "" && *ca != "" && *nsdAddr != "" {
			// Build from arguments
			nsdCollector, err = NewNSDCollector(*nsdType, *nsdAddr, *ca, *key, *cert, false)
			if err != nil {
				log.Fatal(err)
			}
		} else {
			log.Fatal("-nsd-address, -cert, -key, and -ca must all be defined.")
		}
	} else {
		// Build from config
		nsdCollector, err = NewNSDCollectorFromConfig(*nsdConfig)
		if err != nil {
			log.Fatal(err)
		}
	}
	prometheus.MustRegister(nsdCollector)
	log.Println("Started.")
	http.Handle(*metricPath, promhttp.Handler())
	log.Fatal(http.ListenAndServe(*listenAddr, nil))
}
