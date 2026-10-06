// Package metrics implements Mockly's optional Prometheus /metrics endpoint
// (see api.metrics.enabled in the management API config). Each Registry is
// independent (backed by its own prometheus.Registry, not the global
// DefaultRegisterer) so multiple Mockly instances — or parallel tests — never
// collide on metric registration.
package metrics

import (
	"net/http"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Registry collects the metrics exposed on /metrics.
type Registry struct {
	reg *prometheus.Registry

	httpRequestsTotal   *prometheus.CounterVec
	httpRequestDuration *prometheus.HistogramVec
}

// New creates a Registry with all Mockly metrics registered. activeMocks is
// called on every scrape to populate the mockly_active_mocks gauge; pass nil
// to always report 0 (e.g. when the HTTP protocol server is disabled).
func New(activeMocks func() float64) *Registry {
	reg := prometheus.NewRegistry()

	httpRequestsTotal := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "mockly_http_requests_total",
		Help: "Total number of HTTP mock requests handled, labeled by matched mock ID, method, and response status.",
	}, []string{"mock_id", "method", "status"})

	httpRequestDuration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "mockly_http_request_duration_seconds",
		Help:    "HTTP mock request handling duration in seconds, labeled by matched mock ID.",
		Buckets: prometheus.DefBuckets,
	}, []string{"mock_id"})

	if activeMocks == nil {
		activeMocks = func() float64 { return 0 }
	}
	activeMocksGauge := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "mockly_active_mocks",
		Help: "Number of currently configured HTTP mocks.",
	}, activeMocks)

	reg.MustRegister(httpRequestsTotal, httpRequestDuration, activeMocksGauge)

	return &Registry{
		reg:                 reg,
		httpRequestsTotal:   httpRequestsTotal,
		httpRequestDuration: httpRequestDuration,
	}
}

// ObserveHTTPRequest records one completed HTTP mock request. mockID should
// be the ID of the mock that matched, or "" for unmatched requests (recorded
// under the "unmatched" label value).
func (r *Registry) ObserveHTTPRequest(mockID, method string, status int, durationSeconds float64) {
	if mockID == "" {
		mockID = "unmatched"
	}
	r.httpRequestsTotal.WithLabelValues(mockID, method, strconv.Itoa(status)).Inc()
	r.httpRequestDuration.WithLabelValues(mockID).Observe(durationSeconds)
}

// Handler returns the Prometheus scrape handler (text exposition format).
func (r *Registry) Handler() http.Handler {
	return promhttp.HandlerFor(r.reg, promhttp.HandlerOpts{})
}
