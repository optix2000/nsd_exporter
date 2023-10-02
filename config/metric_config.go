package config

import (
	"embed"
	"log/slog"
	"os"
	"regexp"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"gopkg.in/yaml.v3"
)

// ClassifierCRM have share files for CRM114 classifer
//
//go:embed config.yaml
var configFS embed.FS

var stringToValueType = map[string]prometheus.ValueType{
	"counter": prometheus.CounterValue,
	"gauge":   prometheus.GaugeValue,
	"untyped": prometheus.UntypedValue,
}

type MetricConfig struct {
	Metrics      map[string]*Metric      `yaml:"metrics"`
	LabelMetrics map[string]*LabelMetric `yaml:"label_metrics"`
}

type Metric struct {
	Help string               `yaml:"help"`
	Type prometheus.ValueType `yaml:"type"`
}

type LabelMetric struct {
	Help   string               `yaml:"help"`
	Name   string               `yaml:"name"`
	Labels []string             `yaml:"labels"`
	Type   prometheus.ValueType `yaml:"type"`
	Regex  *regexp.Regexp
}

// Convert config to prom types
func stringToPromType(s string) prometheus.ValueType {
	valueType, ok := stringToValueType[strings.ToLower(s)]
	if !ok {
		slog.Warn("Invalid type of metric. Assumed Gauge", "type", s)
		valueType = prometheus.GaugeValue
	}
	return valueType
}

// Serialize YAML into correct types
func (m *Metric) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var tmp struct {
		Help string
		Type string
	}

	err := unmarshal(&tmp)
	if err != nil {
		return err
	}
	m.Help = tmp.Help
	m.Type = stringToPromType(tmp.Type)
	return nil
}

func (m *LabelMetric) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var tmp struct {
		Name   string
		Help   string
		Type   string
		Labels []string
	}

	err := unmarshal(&tmp)
	if err != nil {
		return err
	}

	m.Name = tmp.Name
	m.Help = tmp.Help
	m.Labels = tmp.Labels
	m.Type = stringToPromType(tmp.Type)
	return nil
}

func LoadConfig(path string, metricConf *MetricConfig) error {
	var b []byte
	var err error

	if path == "" {
		b, err = configFS.ReadFile("config.yaml")
		if err != nil {
			return err
		}
	} else {
		b, err = os.ReadFile(path)
		if err != nil {
			return err
		}
	}

	err = yaml.Unmarshal(b, metricConf)
	if err != nil {
		return err
	}

	for k, v := range metricConf.LabelMetrics {
		v.Regex, err = regexp.Compile(k)
		if err != nil {
			return err
		}
	}

	return nil
}
