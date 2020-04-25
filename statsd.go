package main

import (
	"github.com/DataDog/datadog-go/statsd"
)

type Statsd struct {
	Address string
	Metric  string
	client  *statsd.Client
}

func (s *Statsd) Start() {
	var err error
	s.client, err = statsd.New(s.Address)
	check(err)
}

func (s *Statsd) Stop() {
	s.client.Close()
}

// send everything except message
func (s *Statsd) tags(m map[string]string) *[]string {
	tags := []string{}
	for k, v := range m {
		tags = append(tags, k+":"+v)
	}

	return &tags
}

func (s *Statsd) Inc(m map[string]string) {
	s.client.Incr(s.Metric, *s.tags(m), 1)
}
