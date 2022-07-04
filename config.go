package main

import (
	"fmt"
	"io/ioutil"
	"regexp"
	"strconv"
	"time"

	"gopkg.in/yaml.v2"
)

type Pattern struct {
	Regex              string
	regexParsed        *regexp.Regexp
	Discard            bool
	Add                map[string]string
	Level              string
	levelSet           bool
	IgnoreMetricLabels []string `yaml:"ignoreMetricLabels"`
	SampleRate         *float32 `yaml:"sampleRate"`
}

type Config struct {
	Prometheus       *Prometheus
	Statsd           *Statsd
	Glog             string
	glogSet          bool
	TimestampKey     string `yaml:"timestampKey"`
	timestampKeySet  bool
	LevelKey         string `yaml:"levelKey"`
	levelKeySet      bool
	MessageKey       string `yaml:"messageKey"`
	Patterns         []Pattern
	Preprocess       string
	preprocessSet    bool
	preprocessParsed *regexp.Regexp
}

var glogRegex = regexp.MustCompile(`^([IWEF])(\d{2})(\d{2}) (\d{2}):(\d{2}):(\d{2})\.\d+ +\d+ \S+:\d+] `)
var glogLevels = map[string]string{
	"I": "INFO",
	"W": "WARN",
	"E": "ERROR",
	"F": "FATAL",
}
var timeFormat = time.RFC3339

func NewConfig(path string) (*Config, error) {
	// read config
	var config Config
	content, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if err = yaml.UnmarshalStrict(content, &config); err != nil {
		return nil, err
	}

	// we always need a message key
	if config.MessageKey == "" {
		config.MessageKey = "message"
	}

	// optimizations to avoid doing multiple times
	for i := range config.Patterns {
		config.Patterns[i].regexParsed =
			helpfulMustCompile(config.Patterns[i].Regex, "patterns["+strconv.Itoa(i)+"].regex")
		config.Patterns[i].levelSet = (config.Patterns[i].Level != "")

		if config.Patterns[i].SampleRate != nil {
			rate := *config.Patterns[i].SampleRate
			if rate < 0.0 || rate > 1.0 {
				return nil, fmt.Errorf("sample must be between 0.0 - 1.0 but was %f", rate)
			}
		}
	}
	config.timestampKeySet = (config.TimestampKey != "")
	config.levelKeySet = (config.LevelKey != "")
	config.glogSet = (config.Glog != "")

	// preprocess
	config.preprocessSet = (config.Preprocess != "")
	if config.preprocessSet {
		config.preprocessParsed = helpfulMustCompile(config.Preprocess, "preprocess")
	}

	// store all possible labels
	if config.Prometheus != nil {
		config.Prometheus.Labels = config.possibleLabels()
	}

	return &config, nil
}

// all labels that could ever be used by the given config
func (c *Config) possibleLabels() []string {
	labels := []string{}

	if c.levelKeySet {
		labels = append(labels, c.LevelKey)
	}

	if c.preprocessSet {
		addCaptureNames(c.preprocessParsed, &labels)
	}

	// all possible captures and `add`
	for _, pattern := range c.Patterns {
		if pattern.Discard {
			continue
		}

		patternLabels := []string{}
		addCaptureNames(pattern.regexParsed, &patternLabels)

		if pattern.Add != nil {
			patternLabels = append(patternLabels, keys(pattern.Add)...)
		}

		for _, l := range pattern.IgnoreMetricLabels {
			patternLabels = removeElement(patternLabels, l)
		}

		labels = append(labels, patternLabels...)
	}

	labels = unique(labels)
	labels = removeElement(labels, c.MessageKey) // would make stats useless

	return labels
}
