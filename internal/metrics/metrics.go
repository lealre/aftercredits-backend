// Package metrics exposes the process's Prometheus metrics and the HTTP
// middleware that feeds them.
//
// The registry is deliberately not the Prometheus global default: what this
// process exposes should be readable from this package alone, and tests must
// be able to build several registries in one process without colliding on
// duplicate registration.
package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// namespace prefixes every metric this package defines, so nothing collides
// with the go_* and process_* series the runtime and process collectors own, or
// with anything a future exporter adds.
const namespace = "aftercredits"

// Metrics holds the registry this process serves and the HTTP instruments the
// middleware records into.
type Metrics struct {
	registry *prometheus.Registry
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
	inFlight prometheus.Gauge
}

// New builds the registry this process serves: Go runtime metrics, process
// metrics, the HTTP instruments, and — when pool is non-nil — the database
// connection-pool collector.
//
// pool is nil for callers with no pool to report on, which registers no pool
// series rather than panicking.
func New(pool PoolStatsFunc) *Metrics {
	registry := prometheus.NewRegistry()

	// Runtime and process metrics come from these two collectors alone. They
	// are what make memory and goroutine counts available with no
	// instrumentation of our own.
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	m := &Metrics{
		registry: registry,
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "http",
			Name:      "requests_total",
			Help:      "Total HTTP requests handled, by method, matched route and response status.",
		}, []string{"method", "route", "status"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "http",
			Name:      "request_duration_seconds",
			Help:      "HTTP request latency in seconds, by method and matched route.",
		}, []string{"method", "route"}),
		inFlight: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "http",
			Name:      "requests_in_flight",
			Help:      "HTTP requests currently being served.",
		}),
	}
	registry.MustRegister(m.requests, m.duration, m.inFlight)

	if pool != nil {
		registerPool(registry, pool)
	}

	return m
}

// Handler serves the registry in the Prometheus text exposition format.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}
