package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"os"
	"strconv"
	"sync"
	"time"
)

const Version = "master" // dynamically set by release action

type StreamLine struct {
	index int
	line  string
}

func main() {
	set, command := parseFlags()

	// prevent unsupported dual/no-input usage
	if isPipingToStdin() == (len(command) != 0) {
		// untested section
		set.Usage()
		os.Exit(2)
	}

	config, err := NewConfig("logrecycler.yaml")
	if err != nil {
		// untested section
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err.Error())
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

	var streams []io.Reader
	var exit chan (int)

	if len(command) != 0 {
		// read from command
		streams, exit, err = executeCommand(command)
		if err != nil {
			// untested section
			_, _ = fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(2)
		}
	} else {
		// read from stdin
		streams = []io.Reader{os.Stdin}
	}

	// process the stream line by line
	lines := combineStreams(streams)
	for l := range lines {
		processLine(l, config)
	}

	// exit with the exit code of the command
	if exit != nil {
		exitCode := <-exit
		if exitCode != 0 {
			// untested section
			os.Exit(exitCode)
		}
	}
}

func combineStreams(streams []io.Reader) chan StreamLine {
	lines := make(chan StreamLine)

	var wg sync.WaitGroup
	for i, stream := range streams {
		wg.Add(1)
		go func(idx int, r io.Reader) {
			defer wg.Done()
			scanner := bufio.NewScanner(r)
			for scanner.Scan() {
				lines <- StreamLine{idx, scanner.Text()}
			}
		}(i, stream)
	}

	go func() {
		wg.Wait()
		close(lines)
	}()

	return lines
}

// parse flags ... so we fail on unknown flags and users can call `-help`
// TODO: return errors so we can test this method
func parseFlags() (*flag.FlagSet, []string) {
	programName, args := os.Args[0], os.Args[1:]
	args, command := splitArrayOn(args, "--")

	set := flag.NewFlagSet(programName, flag.ContinueOnError)

	set.Usage = func() { // untested section
		_, _ = fmt.Fprintf(
			os.Stderr,
			"logrecycler "+Version+"\n"+
				"pipe logs to logrecycler to convert them into json logs with custom tags\n"+
				"alternatively tell it what command to execute with `-- command`\n"+
				"configure with logrecycler.yaml\n"+
				"for more info see https://github.com/grosser/logrecycler\n",
		)
		set.PrintDefaults()
	}
	version := set.Bool("version", false, "Show version")
	help := set.Bool("help", false, "Show this")

	if err := set.Parse(args); err != nil { // untested section
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

	return set, command
}

// everything in here needs to be extra efficient
func processLine(line StreamLine, config *Config) {
	// build log line ... sets the json key order too
	log := NewOrderedMap()
	if config.timestampKeySet {
		log.Set(config.TimestampKey, time.Now().Format(timeFormat))
	}
	if config.levelKeySet {
		log.Set(config.LevelKey, "INFO")
	}
	log.Set(config.MessageKey, line.line)

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

	// parse our json
	if config.jsonSet {
		message := log.values[config.MessageKey]
		messageLen := len(message)
		if messageLen != 0 && message[0] == '{' && message[messageLen-1] == '}' {
			captureJson(config, log)
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

	// write to where the line came from
	out := os.Stdout
	if line.index == 1 {
		out = os.Stderr
	}
	_, _ = fmt.Fprintln(out, log.ToJson())

	// remove keys nobody should be using as metrics, but can get set accidentally via captures
	delete(log.values, config.MessageKey)
	if config.timestampKeySet {
		delete(log.values, config.TimestampKey)
	}

	// remove not explicitly allowed labels
	if config.AllowMetricLabels != nil {
		previous := log.values
		log.values = map[string]string{}
		for _, l := range config.AllowMetricLabels {
			if previousValue, previousSet := previous[l]; previousSet {
				log.values[l] = previousValue
			}
		}
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

// TODO: this should ideally keep the ordering of the json keys
func captureJson(config *Config, log *OrderedMap) {
	jsonMap := make(map[string](interface{}))
	err := json.Unmarshal([]byte(log.values[config.MessageKey]), &jsonMap)
	if err != nil {
		return
	}

	// we split up the message, so discard it
	// TODO: deal with json that did not have a message by cleanly removing it
	log.values[config.MessageKey] = ""

	for k, v := range jsonMap {
		// TODO: allow parsing through any json type by not using Sprintf
		log.Set(k, fmt.Sprintf("%v", v))
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
