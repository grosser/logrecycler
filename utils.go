package main

import (
	"fmt"
	"os"
	"regexp"
)

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

func check(e error) {
	if e != nil {
		panic(e) // untested section
	}
}

func helpfulMustCompile(expr string, location string) *regexp.Regexp {
	compiled, err := regexp.Compile(expr)
	if err != nil {
		// untested section
		fmt.Fprintf(os.Stderr, "Error: regular expression from "+location+": "+err.Error())
		os.Exit(1)
	}
	return compiled
}

func addCaptureNames(re *regexp.Regexp, labels *[]string) {
	for _, name := range re.SubexpNames() {
		if name != "" {
			*labels = append(*labels, name)
		}
	}
}

// https://stackoverflow.com/questions/39993688/are-golang-slices-passed-by-value
func pipingToStding() bool {
	stat, _ := os.Stdin.Stat()
	return (stat.Mode() & os.ModeCharDevice) == 0
}
