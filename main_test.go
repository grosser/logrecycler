package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("main", func() {
	BeforeSuite(func() {
		os.Args = []string{"logrecycler"} // make flag parsing not crash
	})

	It("works with empty config", func() {
		withConfig("", func() {
			Expect(parse("hi")).To(Equal(`{"message":"hi"}`))
		})
	})

	It("can configure messageKey", func() {
		withConfig("---\nmessageKey: msg", func() {
			Expect(parse("hi")).To(Equal(`{"msg":"hi"}`))
		})
	})

	It("can configure timestampKey", func() {
		withConfig("---\ntimestampKey: ts", func() {
			Expect(parse("hi")).To(ContainSubstring(`{"ts":"`))
		})
	})

	It("can set level", func() {
		withConfig("---\nlevelKey: severity", func() {
			Expect(parse("hi")).To(Equal(`{"severity":"INFO","message":"hi"}`))
		})
	})

	It("can match patterns", func() {
		withConfig("---\npatterns:\n- regex: hi\n  add:\n    foo: bar", func() {
			Expect(parse("hi")).To(Equal(`{"message":"hi","foo":"bar"}`))
		})
	})

	It("only matches a single pattern", func() {
		withConfig("---\npatterns:\n- regex: hi\n  add:\n    foo: bar\n- regex: hello\n  add:\n    bar: baz\n- regex: hell\n  add:\n    oh: no", func() {
			Expect(parse("hello")).To(Equal(`{"message":"hello","bar":"baz"}`))
		})
	})

	It("can change level from patterns", func() {
		withConfig("---\nlevelKey: level\npatterns:\n- regex: hi\n  level: WARN", func() {
			Expect(parse("hi")).To(Equal(`{"level":"WARN","message":"hi"}`))
		})
	})

	It("can change message from match", func() {
		withConfig("---\npatterns:\n- regex: (?P<message>hi) \\S+", func() {
			Expect(parse("hi foo")).To(Equal(`{"message":"hi"}`))
		})
	})

	It("ignores unnamed captures", func() {
		withConfig("---\npatterns:\n- regex: (hi)", func() {
			Expect(parse("hi there")).To(Equal(`{"message":"hi there"}`))
		})
	})

	It("can add named captures", func() {
		withConfig("---\npatterns:\n- regex: hi (?P<name>\\S+)", func() {
			Expect(parse("hi foo")).To(Equal(`{"message":"hi foo","name":"foo"}`))
		})
	})

	It("ignores unnamed captures", func() {
		withConfig("---\npatterns:\n- regex: (h)(?P<ii>i) (?P<name>\\S+)", func() {
			Expect(parse("hi foo")).To(Equal(`{"message":"hi foo","ii":"i","name":"foo"}`))
		})
	})

	It("can discard", func() {
		withConfig("---\npatterns:\n- regex: hi\n  discard: true", func() {
			Expect(parse("hi foo")).To(Equal(``))
		})
	})

	It("can sample", func() {
		withConfig("---\npatterns:\n- regex: hi\n  sampleRate: 1.0", func() {
			Expect(parse("hi foo")).To(Equal(`{"message":"hi foo"}`))
		})
	})

	It("ignores when sample rate is zero", func() {
		withConfig("---\npatterns:\n- regex: hi\n  sampleRate: 0.0", func() {
			Expect(parse("hi foo")).To(Equal(""))
		})
	})

	It("can log complex messages", func() {
		withConfig("", func() {
			Expect(parse("hi\"foo")).To(Equal(`{"message":"hi\"foo"}`))
		})
	})

	It("can call a command", func() {
		withConfig("", func() {
			Expect(runWithCommand("echo", "hi\"foo")).To(Equal(`{"message":"hi\"foo"}`))
		})
	})

	It("can call a command that produces stdout and stderr", func() {
		withConfig("", func() {
			err := captureStderr(func() {
				out := runWithCommand("sh", "-c", "echo OUT; echo ERR >&2")
				Expect(out).To(Equal(`{"message":"OUT"}`))
			})
			Expect(strings.TrimRight(err, "\n")).To(Equal(`{"message":"ERR"}`))
		})
	})

	Context("Glog", func() {
		It("parses simple", func() {
			withConfig("---\nglog: simple", func() {
				Expect(parse("I0203 02:03:04.12345    123 foo.go:123] hi")).
					To(Equal(`{"message":"hi"}`))
			})
		})

		It("parses level", func() {
			withConfig("---\nglog: simple\nlevelKey: lvl", func() {
				Expect(parse("I0203 02:03:04.12345    123 foo.go:123] hi")).
					To(Equal(`{"lvl":"INFO","message":"hi"}`))
			})
		})

		It("parses time", func() {
			withConfig("---\nglog: simple\ntimestampKey: ts", func() {
				Expect(parse("I0203 02:03:04.12345     123 foo.go:123] hi")).
					To(Equal(`{"ts":"` + fmt.Sprint(time.Now().Year()) + `-02-03T02:03:04Z","message":"hi"}`))
			})
		})
	})

	Context("Json", func() {
		It("parses simple", func() {
			withConfig("---\njson: simple", func() {
				Expect(parse("{\"message\":\"hi\"}")).
					To(Equal(`{"message":"hi"}`))
			})
		})

		It("can add non-strings", func() {
			withConfig("---\njson: simple", func() {
				Expect(parse("{\"foo\":123}")).
					To(Equal(`{"message":"","foo":"123"}`))
			})
		})

		It("ignores invalid", func() {
			withConfig("---\njson: simple", func() {
				Expect(parse("{}}}")).
					To(Equal(`{"message":"{}}}"}`))
			})
		})
	})

	Context("preprocess", func() {
		It("Ignores non-matching", func() {
			withConfig("---\npreprocess: (?P<greeting>oops) (?P<message>.*)\npatterns:\n- regex: (?P<rest>.*)", func() {
				Expect(parse("hi foo")).To(Equal(`{"message":"hi foo","rest":"hi foo"}`))
			})
		})

		It("can modify and add via preprocess", func() {
			withConfig("---\npreprocess: (?P<greeting>hi) (?P<message>.*)\npatterns:\n- regex: (?P<rest>.*)", func() {
				Expect(parse("hi foo")).To(Equal(`{"message":"foo","greeting":"hi","rest":"foo"}`))
			})
		})
	})

	Context("prometheus metrics", func() {
		It("opens server at requested port", func() {
			port := randomPort()
			withConfig("---\nprometheus:\n  port: "+port, func() {
				Expect(prometheusMetrics(port)).To(Equal("# HELP logs_total Total number of logs received\n# TYPE logs_total counter\nlogs_total 1\n"))
			})
		})

		It("reports level", func() {
			port := randomPort()
			withConfig("---\nprometheus:\n  port: "+port+"\nlevelKey: lvl", func() {
				Expect(prometheusMetrics(port)).To(Equal("# HELP logs_total Total number of logs received\n# TYPE logs_total counter\nlogs_total{lvl=\"INFO\"} 1\n"))
			})
		})

		It("reports added fields", func() {
			port := randomPort()
			withConfig("---\nprometheus:\n  port: "+port+"\npatterns:\n- regex: hi\n  add:\n    foo: bar", func() {
				Expect(prometheusMetrics(port)).To(Equal("# HELP logs_total Total number of logs received\n# TYPE logs_total counter\nlogs_total{foo=\"bar\"} 1\n"))
			})
		})

		It("ignores labels from discarded fields", func() {
			port := randomPort()
			withConfig("---\nprometheus:\n  port: "+port+"\npatterns:\n- regex: hi\n  add:\n    foo: bar\n- regex: ''\n  add:\n    bar: foo\n  discard: true", func() {
				Expect(prometheusMetrics(port)).To(Equal("# HELP logs_total Total number of logs received\n# TYPE logs_total counter\nlogs_total{foo=\"bar\"} 1\n"))
			})
		})

		It("ignores labels when not in ignoreMetricLabels", func() {
			port := randomPort()
			withConfig("---\nprometheus:\n  port: "+port+"\npatterns:\n- regex: hi\n  add:\n    foo: bar\n    bar: baz\n  ignoreMetricLabels: [\"bar\"]", func() {
				Expect(prometheusMetrics(port)).To(Equal("# HELP logs_total Total number of logs received\n# TYPE logs_total counter\nlogs_total{foo=\"bar\"} 1\n"))
			})
		})

		It("reports captures", func() {
			port := randomPort()
			withConfig("---\nprometheus:\n  port: "+port+"\npatterns:\n- regex: h(?P<name>i)", func() {
				Expect(prometheusMetrics(port)).To(Equal("# HELP logs_total Total number of logs received\n# TYPE logs_total counter\nlogs_total{name=\"i\"} 1\n"))
			})
		})

		It("does not report missing fields", func() {
			port := randomPort()
			withConfig("---\nprometheus:\n  port: "+port+"\npatterns:\n- regex: nope\n  add:\n    foo: bar", func() {
				Expect(prometheusMetrics(port)).To(Equal("# HELP logs_total Total number of logs received\n# TYPE logs_total counter\nlogs_total{foo=\"\"} 1\n"))
			})
		})

		It("can report from preprocess", func() {
			port := randomPort()
			withConfig("---\nprometheus:\n  port: "+port+"\npreprocess: h(?P<ii>i)", func() {
				Expect(prometheusMetrics(port)).To(Equal("# HELP logs_total Total number of logs received\n# TYPE logs_total counter\nlogs_total{ii=\"i\"} 1\n"))
			})
		})
	})

	Context("statsd metrics", func() {
		It("reports", func() {
			received := receiveUdp(func() {
				withConfig("---\nstatsd:\n  address: 0.0.0.0:8125\n  metric: foo.logs", func() {
					parse("hi foo")
				})
			})
			Expect(received).To(Equal("foo.logs:1|c"))
		})

		It("reports additions", func() {
			received := receiveUdp(func() {
				withConfig("---\nstatsd:\n  address: 0.0.0.0:8125\n  metric: foo.logs\npatterns:\n- regex: hi\n  add:\n    foo: bar", func() {
					parse("hi foo")
				})
			})
			Expect(received).To(Equal("foo.logs:1|c|#foo:bar"))
		})

		It("reports preprocess", func() {
			received := receiveUdp(func() {
				withConfig("---\nstatsd:\n  address: 0.0.0.0:8125\n  metric: foo.logs\npreprocess: hi (?P<name>.*)", func() {
					parse("hi foo")
				})
			})
			Expect(received).To(Equal("foo.logs:1|c|#name:foo"))
		})

		It("does not report message override", func() {
			received := receiveUdp(func() {
				withConfig("---\nstatsd:\n  address: 0.0.0.0:8125\n  metric: foo.logs\npatterns:\n- regex: hi\n  add:\n    message: bar", func() {
					parse("hi foo")
				})
			})
			Expect(received).To(Equal("foo.logs:1|c"))
		})

		It("does not report timestamps", func() {
			received := receiveUdp(func() {
				withConfig("---\nstatsd:\n  address: 0.0.0.0:8125\n  metric: foo.logs\ntimestampKey: ts", func() {
					Expect(parse("hi")).To(ContainSubstring(`{"ts":"`))
				})
			})
			Expect(received).To(Equal("foo.logs:1|c"))
		})

		It("ignores when AllowMetricLabels is set", func() {
			received := receiveUdp(func() {
				withConfig("---\nstatsd:\n  address: 0.0.0.0:8125\n  metric: foo.logs\njson: simple\nallowMetricLabels: [this]", func() {
					output := parse(`{"this":"1","that":"2"}`)
					Expect(output).To(ContainSubstring(`{"message":"","t`)) // order of this and that is not guaranteed
					Expect(output).To(ContainSubstring(`"this":"1"`))
					Expect(output).To(ContainSubstring(`"that":"2"`))
				})
			})
			Expect(received).To(Equal("foo.logs:1|c|#this:1"))
		})
	})
})

// ports are not freed fast enough when running on travis, so instead of waiting use a random port
func randomPort() string {
	return strconv.Itoa(rand.Intn(5000) + 1000)
}

func parse(input string) (output string) {
	withStdin(input, false, func() {
		output = captureStdout(func() { main() })
		output = strings.TrimRight(output, "\n")
	})
	return
}

func runWithCommand(input ...string) (out string) {
	before := os.Args
	os.Args = append([]string{"foo", "--"}, input...)
	defer func() { os.Args = before }()
	out = captureStdout(func() { main() })
	out = strings.TrimRight(out, "\n")
	return
}

func prometheusMetrics(port string) string {
	out := "ERROR"
	withStdin("hi\n", true, func() {
		go captureStdout(func() { main() }) // finished when stdin closes
		time.Sleep(10 * time.Millisecond)   // works locally without, but travis needs it
		out = request("http://0.0.0.0:" + port + "/metrics")
	})
	return out
}

func receiveUdp(fn func()) string {
	pc, err := net.ListenUDP("udp", &net.UDPAddr{IP: []byte{0, 0, 0, 0}, Port: 8125, Zone: ""})
	Expect(err).To(BeNil())
	defer pc.Close()

	fn()

	deadline := time.Now().Add(1 * time.Second)
	err = pc.SetReadDeadline(deadline)
	Expect(err).To(BeNil())

	buf := make([]byte, 1024)
	n, _, err := pc.ReadFromUDP(buf)
	Expect(err).To(BeNil())

	return string(buf[0:n])
}

func withStdin(input string, open bool, fn func()) {
	old := os.Stdin // keep backup of the real
	r, w, _ := os.Pipe()
	w.WriteString(input)
	if !open {
		w.Close()
	}
	os.Stdin = r
	fn()
	if open {
		w.Close()
	}
	os.Stdin = old
}

// https://stackoverflow.com/questions/10473800/in-go-how-do-i-capture-stdout-of-a-function-into-a-string
func captureStdout(fn func()) (captured string) {
	old := os.Stdout // keep backup of the real
	r, w, _ := os.Pipe()
	os.Stdout = w

	fn()

	outC := make(chan string)
	// copy the output in a separate goroutine so printing can't block indefinitely
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, r)
		outC <- buf.String()
	}()

	// back to normal state
	w.Close()
	os.Stdout = old // restoring the real
	captured = <-outC
	return
}

func captureStderr(fn func()) (captured string) {
	old := os.Stderr // keep backup of the real
	r, w, _ := os.Pipe()
	os.Stderr = w

	fn()

	outC := make(chan string)
	// copy the output in a separate goroutine so printing can't block indefinitely
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, r)
		outC <- buf.String()
	}()

	// back to normal state
	w.Close()
	os.Stderr = old // restoring the real
	captured = <-outC
	return
}

func withConfig(config string, fn func()) {
	orig, _ := os.ReadFile("logrecycler.yaml")
	err := os.WriteFile("logrecycler.yaml", []byte(config), 0644)
	Expect(err).To(BeNil())
	defer os.WriteFile("logrecycler.yaml", orig, 0644)
	fn()
}

func request(url string) string {
	// untested section
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	Expect(err).To(BeNil())

	response, err := client.Do(req)
	Expect(err).To(BeNil())

	body, err := ioutil.ReadAll(response.Body)
	Expect(err).To(BeNil())

	return string(body)
}

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Example")
}
