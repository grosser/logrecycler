package main

import (
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"regexp"
	"strconv"
	"time"
)

type Pattern struct {
	Regex       string
	regexParsed *regexp.Regexp
	Discard     bool
	Add         map[string]string
	Level       string
	levelSet    bool
	MetricLabels *[]string `yaml:"metricLabels"`
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

func NewConfig(path string) *Config {
	// read config
	var config Config
	content, err := ioutil.ReadFile(path)
	check(err)

	err = yaml.UnmarshalStrict(content, &config)
	check(err)

	// we always need a message key
	if config.MessageKey == "" {
		config.MessageKey = "message"
	}

	// optimizations to avoid doing multiple times
	for i := range config.Patterns {
		config.Patterns[i].regexParsed =
			helpfulMustCompile(config.Patterns[i].Regex, "patterns["+strconv.Itoa(i)+"].regex")
		config.Patterns[i].levelSet = (config.Patterns[i].Level != "")
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

	return &config
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

		if pattern.MetricLabels == nil {
			addCaptureNames(pattern.regexParsed, &labels)

			if pattern.Add != nil {
				labels = append(labels, keys(pattern.Add)...)
			}
		} else {
			labels = append(labels, *pattern.MetricLabels...)
		}
	}

	labels = unique(labels)
	labels = removeElement(labels, c.MessageKey) // would make stats useless

	return labels
}
