package main

import (
	"bufio"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"time"
)

const Version = "master" // dynamically set by release action

func main() {
	set := parseFlags()

	config, err := NewConfig("logrecycler.yaml")
	if err != nil {
		// untested section
		fmt.Fprintf(os.Stderr, "Error: %v\n", err.Error())
		os.Exit(2)
	}

	if config.Prometheus != nil {
		config.Prometheus.Start()
		defer config.Prometheus.Stop()
	}

	if config.Statsd != nil {
		config.Statsd.Start()
		defer config.Statsd.Stop()
	}

	rand.Seed(time.Now().UnixNano())

	// read logs from stdin
	if !pipingToStding() {
		// untested section
		set.Usage()
		os.Exit(2)
	}
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		processLine(line, config)
	}
}

// parse flags ... so we fail on unknown flags and users can call `-help`
// TODO: return errors so we can test this method
func parseFlags() *flag.FlagSet {
	set := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

	set.Usage = func() { // untested section
		fmt.Fprintf(
			os.Stderr,
			"logrecycler "+Version+"\n"+
				"pipe logs to logrecycler to convert them into json logs with custom tags\n"+
				"configure with logrecycler.yaml\n"+
				"for more info see https://github.com/grosser/logrecycler\n",
		)
		set.PrintDefaults()
	}
	version := set.Bool("version", false, "Show version")
	help := set.Bool("help", false, "Show this")

	if err := set.Parse(os.Args[1:]); err != nil { // untested section
		set.Usage()
		os.Exit(2)
	}

	if *version { // untested section
		fmt.Println(Version)
		os.Exit(0)
	}

	if *help { // untested section
		set.Usage()
		os.Exit(0)
	}

	if len(set.Args()) != 0 { // untested section
		set.Usage()
		os.Exit(2)
	}

	return set
}

// everything in here needs to be extra efficient
func processLine(line string, config *Config) {
	// build log line ... sets the json key order too
	log := NewOrderedMap()
	if config.timestampKeySet {
		log.Set(config.TimestampKey, time.Now().Format(timeFormat))
	}
	if config.levelKeySet {
		log.Set(config.LevelKey, "INFO")
	}
	log.Set(config.MessageKey, line)

	// preprocess the log line for general purpose cleanup
	if config.preprocessSet {
		if match := config.preprocessParsed.FindStringSubmatch(log.values[config.MessageKey]); match != nil {
			log.StoreNamedCaptures(config.preprocessParsed, &match)
		}
	}

	// parse out glog
	if config.glogSet {
		if match := glogRegex.FindStringSubmatch(log.values[config.MessageKey]); match != nil {
			captureGlog(config, match, log)
		}
	}

	// apply pattern rules if any
	var ignoreMetricLabels []string
	for _, pattern := range config.Patterns {
		if match := pattern.regexParsed.FindStringSubmatch(log.values[config.MessageKey]); match != nil {
			if pattern.Discard {
				return
			}

			if pattern.SampleRate != nil {
				if rand.Float32() > *pattern.SampleRate {
					return
				}
			}

			// set level
			if pattern.levelSet {
				log.values[config.LevelKey] = pattern.Level
			}

			log.StoreNamedCaptures(pattern.regexParsed, &match)
			log.Merge(pattern.Add)

			ignoreMetricLabels = pattern.IgnoreMetricLabels

			break // a line can only match one pattern
		}
	}

	fmt.Println(log.ToJson())

	// remove keys nobody should be using as metrics, but can get set accidentally via captures
	delete(log.values, config.MessageKey)
	if config.timestampKeySet {
		delete(log.values, config.TimestampKey)
	}

	// remove explicitly ignored labels
	for _, l := range ignoreMetricLabels {
		delete(log.values, l)
	}

	// report to metrics backends
	if config.Prometheus != nil {
		config.Prometheus.Inc(log.values)
	}
	if config.Statsd != nil {
		config.Statsd.Inc(log.values)
	}
}

func captureGlog(config *Config, match []string, log *OrderedMap) {
	// remove glog from message
	log.values[config.MessageKey] = log.values[config.MessageKey][len(match[0]):]

	// set level
	if config.levelKeySet {
		log.values[config.LevelKey] = glogLevels[match[1]]
	}

	// parse time
	if config.timestampKeySet {
		year := time.Now().Year()
		month, _ := strconv.Atoi(match[2])
		day, _ := strconv.Atoi(match[3])
		hour, _ := strconv.Atoi(match[4])
		min, _ := strconv.Atoi(match[5])
		sec, _ := strconv.Atoi(match[6])
		date := time.Date(year, time.Month(month), day, hour, min, sec, 0, time.UTC)
		log.values[config.TimestampKey] = date.Format(timeFormat)
	}
}
