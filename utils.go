package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"syscall"
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

// split an array of strings when a given delimiter is found
func splitArrayOn(arr []string, delimiter string) ([]string, []string) {
	for i, item := range arr {
		if item == delimiter {
			return arr[0:i], arr[i+1:]
		}
	}
	return arr, nil
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
		_, _ = fmt.Fprintf(os.Stderr, "Error: regular expression from "+location+": "+err.Error())
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
func isPipingToStdin() bool {
	stat, _ := os.Stdin.Stat()
	return (stat.Mode() & os.ModeCharDevice) == 0
}

// ReaderChannel is an io.Reader used to stream command output to the log processor
type ReaderChannel struct {
	channel chan ([]byte)
}

func (b ReaderChannel) Read(p []byte) (n int, err error) {
	msg, open := <-b.channel
	if open {
		copy(p, msg)
		return len(msg), nil
	} else {
		return 0, io.EOF
	}
}

// executeCommand executes a shell command and returns a reader from the combined stdout and stderr + exit code channel
func executeCommand(command []string) (io.Reader, chan (int), error) {
	cmd := exec.Command(command[0], command[1:]...)
	exit := make(chan int)
	//output := ReaderChannel{make(chan []byte)}

	// Send all output into a pipe
	//pipeReader, pipeWriter, err := os.Pipe()
	//if err != nil {
	//	// untested section
	//	return nil, nil, err
	//}
	//cmd.Stdout = pipeWriter
	//cmd.Stderr = pipeWriter

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		// untested section
		return nil, nil, err
	}

	// Pass on any signal, so the logrecycler behaves like the command it wraps
	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGUSR1, syscall.SIGUSR2, syscall.SIGHUP)
	go func() {
		s, open := <-signalChannel
		if open {
			fmt.Println("PASSING signal:", s)
			// untested section
			_ = cmd.Process.Signal(s)
		}
	}()

	// Stream the output to the buffer, to be consumed by the log parser
	//go func() {
	//	scanner := bufio.NewScanner(pipeReader)
	//	for scanner.Scan() {
	//		output.channel <- append(scanner.Bytes(), '\n')
	//	}
	//	close(output.channel)
	//}()

	// Wait for the command to finish and store the exit code
	go func() {
		_ = cmd.Wait()
		//_ = pipeReader.Close() // make scanner stop
		//_ = pipeWriter.Close()
		close(signalChannel) // make sure exiting the program does not re-signal ourselves
		exit <- cmd.ProcessState.ExitCode()
	}()

	return stdoutPipe, exit, nil
}
