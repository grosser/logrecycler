package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

func main() {
	parseFlags()

	config := NewConfig("logrecycler.yaml")

	if config.Prometheus != nil {
		config.Prometheus.Start()
		defer config.Prometheus.Stop()
	}

	if config.Statsd != nil {
		config.Statsd.Start()
		defer config.Statsd.Stop()
	}

	// read logs from stdin
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		processLine(line, config)
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
	for _, pattern := range config.Patterns {
		if match := pattern.regexParsed.FindStringSubmatch(log.values[config.MessageKey]); match != nil {
			if pattern.Discard {
				return
			}

			// set level
			if pattern.levelSet {
				log.values[config.LevelKey] = pattern.Level
			}

			log.StoreNamedCaptures(pattern.regexParsed, &match)
			log.Merge(pattern.Add)

			break // a line can only match one pattern
		}
	}

	fmt.Println(log.ToJson())

	delete(log.values, config.MessageKey) // nobody should use message as label

	if config.Prometheus != nil {
		config.Prometheus.Inc(log.values)
	}
	if config.Statsd != nil {
		config.Statsd.Inc(log.values)
	}
}

func captureGlog(config *Config, match []string, log *OrderedMap) {
	// remove glog from message
	log.values[config.MessageKey] = strings.TrimLeft(log.values[config.MessageKey], match[0])

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
