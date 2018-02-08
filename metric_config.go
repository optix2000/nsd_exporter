package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"regexp"
	"strings"
)

var stringToValueType = map[string]prometheus.ValueType{
	"counter": prometheus.CounterValue,
	"gauge":   prometheus.GaugeValue,
	"untyped": prometheus.UntypedValue,
}

type metricConfig struct {
	Metrics      map[string]*metric      `yaml:"metrics"`
	LabelMetrics map[string]*labelMetric `yaml:"label_metrics"`
}

type metric struct {
	Help string               `yaml:"help"`
	Type prometheus.ValueType `yaml:"type"`
}

type labelMetric struct {
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
		log.Println("Invalid type:", s, "Assuming Gauge")
		valueType = prometheus.GaugeValue
	}
	return valueType
}

// Serialize YAML into correct types
func (m *metric) UnmarshalYAML(unmarshal func(interface{}) error) error {
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

func (m *labelMetric) UnmarshalYAML(unmarshal func(interface{}) error) error {
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

func loadConfig(path string, metricConf *metricConfig) error {
	var b []byte
	if path == "" {
		var err error
		b, err = Asset("config.yaml")
		if err != nil {
			return err
		}
	} else {
		var err error
		b, err = ioutil.ReadFile(*metricConfigPath)
		if err != nil {
			return err
		}
	}
	err := yaml.Unmarshal(b, metricConf)
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
