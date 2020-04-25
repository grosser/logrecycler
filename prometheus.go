package main

import (
	"context"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
)

type Prometheus struct {
	Port   string
	Labels []string
	Metric *prometheus.CounterVec
	server *http.Server
}

func (p *Prometheus) Start() {
	// build new empty registry without go spam
	// https://stackoverflow.com/questions/35117993/how-to-disable-go-collector-metrics-in-prometheus-client-golang
	r := prometheus.NewRegistry()
	p.Metric = promauto.With(r).NewCounterVec(prometheus.CounterOpts{
		Name: "logs_total",
		Help: "Total number of logs received",
	}, p.Labels)
	handler := promhttp.HandlerFor(r, promhttp.HandlerOpts{})

	// serve metrics
	p.server = &http.Server{Addr: "0.0.0.0:" + p.Port, Handler: handler}
	go p.server.ListenAndServe()
}

func (p *Prometheus) Stop() {
	p.server.Shutdown(context.TODO())
}

func (p *Prometheus) Inc(values map[string]string) {
	p.Metric.WithLabelValues(p.labelValues(values)...).Inc()
}

// build values array in correct order to avoid overhead from prometheus validation code + blowing up on missing labels
func (p *Prometheus) labelValues(labelMap map[string]string) []string {
	values := make([]string, len(p.Labels))

	for i, label := range p.Labels {
		if value, found := labelMap[label]; found {
			values[i] = value
		} else {
			values[i] = ""
		}
	}
	return values
}
