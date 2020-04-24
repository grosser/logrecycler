package main

import (
	"bufio"
	"context"
	"fmt"
	"github.com/DataDog/datadog-go/statsd"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	yaml "gopkg.in/yaml.v2"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"time"
)

type Pattern struct {
	Regex       string
	regexParsed *regexp.Regexp
	Add         map[string]string
	Level       string
	levelSet    bool
}

type Config struct {
	PrometheusPort         string `yaml:"prometheus_port"`
	prometheus             bool
	prometheusMetricLabels []string
	StatsdAddress          string `yaml:"statsd_address"`
	StatsdMetric           string `yaml:"statsd_metric"`
	statsd                 bool
	TimestampKey           string `yaml:"timestamp_key"`
	timestampKeySet        bool
	LevelKey               string `yaml:"level_key"`
	levelKeySet            bool
	MessageKey             string `yaml:"message_key"`
	Patterns               []Pattern
	Preprocess             string `yaml:"preprocess"`
	preprocessSet          bool
	preprocessParsed       *regexp.Regexp
}

func main() {
	parseFlags()

	config := readConfig()

	var metric *prometheus.CounterVec
	var stats *statsd.Client
	var srv *http.Server
	var err error

	if config.prometheus {
		// build new empty registry without go spam
		// https://stackoverflow.com/questions/35117993/how-to-disable-go-collector-metrics-in-prometheus-client-golang
		r := prometheus.NewRegistry()
		metric = promauto.With(r).NewCounterVec(prometheus.CounterOpts{
			Name: "logs_total",
			Help: "Total number of logs received",
		}, prometheusMetricLabels(config))
		handler := promhttp.HandlerFor(r, promhttp.HandlerOpts{})

		// serve metrics
		srv = &http.Server{Addr: "0.0.0.0:" + config.PrometheusPort, Handler: handler}
		go srv.ListenAndServe()
		defer srv.Shutdown(context.TODO())
	}

	if config.statsd {
		stats, err = statsd.New(config.StatsdAddress)
		check(err)
		defer stats.Close()
	}

	// read logs from stdin
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		log := processLine(line, config, metric, stats)
		fmt.Println(log.ToJson())
	}
}

// parse flags ... so we fail on unknown flags and users can call `-help`
// TODO: use a real flag library that supports not failing on --help ... not builtin flag
func parseFlags() {
	if len(os.Args) == 1 {
		return
	}
	fmt.Fprintf(os.Stderr, "Usage:\npipe logs to logrecycler\nconfigure with logrecycler.yaml\n") // untested section
	if len(os.Args) == 2 && (os.Args[1] == "-help" || os.Args[1] == "--help") {
		// untested section
		os.Exit(0)
	} else {
		// untested section
		os.Exit(2)
	}
}

// https://www.golangprograms.com/remove-duplicate-values-from-slice.html
func unique(input []string) []string {
	keys := make(map[string]bool)
	unique := []string{}
	for _, entry := range input {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			unique = append(unique, entry)
		}
	}
	return unique
}

// remove element N times while preserving order
func removeElement(haystack []string, needle string) []string {
	clean := []string{}
	for _, item := range haystack {
		if item != needle {
			clean = append(clean, item)
		}
	}
	return clean
}

// https://stackoverflow.com/questions/21362950/getting-a-slice-of-keys-from-a-map
func keys(mymap map[string]string) []string {
	keys := make([]string, 0, len(mymap))
	for k := range mymap {
		keys = append(keys, k)
	}
	return keys
}

func readConfig() *Config {
	// read config
	var config Config
	content, err := ioutil.ReadFile("logrecycler.yaml")
	check(err)

	err = yaml.Unmarshal(content, &config)
	check(err)

	// we always need a message key
	if config.MessageKey == "" {
		config.MessageKey = "message"
	}

	// optimizations to avoid doing multiple times
	for i := range config.Patterns {
		config.Patterns[i].regexParsed = regexp.MustCompile(config.Patterns[i].Regex)
		config.Patterns[i].levelSet = (config.Patterns[i].Level != "")
	}
	config.timestampKeySet = (config.TimestampKey != "")
	config.levelKeySet = (config.LevelKey != "")
	config.prometheus = (config.PrometheusPort != "")
	config.statsd = (config.StatsdAddress != "")
	config.preprocessSet = (config.Preprocess != "")
	if config.preprocessSet {
		config.preprocessParsed = regexp.MustCompile(config.Preprocess)
	}

	// store all possible labels
	config.prometheusMetricLabels = prometheusMetricLabels(&config)

	return &config
}

// all labels that could ever be used by the given config
func prometheusMetricLabels(config *Config) []string {
	labels := []string{}

	if config.levelKeySet {
		labels = append(labels, config.LevelKey)
	}

	if config.preprocessSet {
		labels = prometheusAddCaptures(config.preprocessParsed, labels)
	}

	// all possible captures and `add`
	for _, pattern := range config.Patterns {
		labels = prometheusAddCaptures(pattern.regexParsed, labels)

		if pattern.Add != nil {
			labels = append(labels, keys(pattern.Add)...)
		}
	}

	labels = unique(labels)
	labels = removeElement(labels, config.MessageKey) // would make stats useless

	return labels
}

func prometheusAddCaptures(re *regexp.Regexp, labels []string) []string {
	for _, name := range re.SubexpNames() {
		if name != "" {
			labels = append(labels, name)
		}
	}
	return labels
}

// build values array in correct order to avoid overhead from prometheus validation code + blowing up on missing labels
func prometheusLabelValues(labelMap *map[string]string, config *Config) []string {
	values := make([]string, len(config.prometheusMetricLabels))

	for i, label := range config.prometheusMetricLabels {
		if value, found := (*labelMap)[label]; found {
			values[i] = value
		} else {
			values[i] = ""
		}
	}
	return values
}

// send everything except message
func statsdTags(m *map[string]string, config *Config) []string {
	tags := []string{}
	for k, v := range *m {
		if k != config.MessageKey {
			tags = append(tags, k+":"+v)
		}
	}

	return tags
}

func check(e error) {
	if e != nil {
		panic(e) // untested section
	}
}

func processLine(line string, config *Config, metric *prometheus.CounterVec, stats *statsd.Client) *OrderedMap {
	// build log line ... sets the json key order too
	log := NewOrderedMap()
	if config.timestampKeySet {
		log.Set(config.TimestampKey, time.Now().Format(time.RFC3339))
	}
	if config.levelKeySet {
		log.Set(config.LevelKey, "INFO")
	}
	log.Set(config.MessageKey, line)

	// preprocess the log line for general purpose cleanup
	if config.preprocessSet {
		if match := config.preprocessParsed.FindStringSubmatch(line); match != nil {
			storeCaptures(config.preprocessParsed, log, match)
		}
	}

	// apply pattern rules if any
	for _, pattern := range config.Patterns {
		if match := pattern.regexParsed.FindStringSubmatch(log.values[config.MessageKey]); match != nil {
			// set level
			if pattern.levelSet {
				log.Set(config.LevelKey, pattern.Level)
			}

			// log named captures
			storeCaptures(pattern.regexParsed, log, match)

			// log additional fields
			for k, v := range pattern.Add {
				log.Set(k, v)
			}

			break // a line can only match one pattern
		}
	}
	if config.prometheus {
		metric.WithLabelValues(prometheusLabelValues(&log.values, config)...).Inc()
	}
	if config.statsd {
		stats.Incr(config.StatsdMetric, statsdTags(&log.values, config), 1)
	}
	return log
}

func storeCaptures(re *regexp.Regexp, log *OrderedMap, match []string) {
	for i, name := range re.SubexpNames() {
		if i != 0 && name != "" {
			log.Set(name, match[i])
		}
	}
}
