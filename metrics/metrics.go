// Package metrics provides interface-first application metrics: a Recorder
// records counters, histograms, and gauges by name, a Prometheus backend
// exposes them for scraping, and a NoOp backend is available for tests and
// environments where metrics are disabled.
//
// httpkit/middleware.Metrics records the standard RED-method HTTP metrics
// (request count, duration, in-flight) for every request without coupling
// httpkit to Prometheus directly.
//
// Example usage:
//
//	rec, err := metrics.NewPrometheus(metrics.Config{Namespace: "orders"})
//	if err != nil {
//		return err
//	}
//
//	rec.CounterInc("orders_processed_total", metrics.Labels{"status": "ok"})
package metrics

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -destination=../mocks/metrics/mock_metrics.go -package=mockmetrics github.com/biairmal/go-sdk/metrics Recorder

// Standard HTTP metric names recorded by httpkit/middleware.Metrics.
const (
	// HTTPRequestsTotal is a counter of completed HTTP requests, labeled by
	// method, path, and status_code.
	HTTPRequestsTotal = "http_requests_total"
	// HTTPRequestDuration is a histogram of HTTP request durations in
	// seconds, labeled by method, path, and status_code.
	HTTPRequestDuration = "http_request_duration_seconds"
	// HTTPRequestsInFlight is a gauge of HTTP requests currently being served.
	HTTPRequestsInFlight = "http_requests_in_flight"
)

// Labels are key-value pairs attached to a metric observation. The set of
// keys used for a given metric name must stay consistent across calls; the
// Prometheus backend registers each metric's label set on first use.
type Labels map[string]string

// Recorder records application metrics by name. Implementations must be
// safe for concurrent use. Recording never fails the caller's operation:
// methods have no error return, and backend-level failures (e.g. a label-set
// mismatch on a reused metric name) are logged, not propagated.
type Recorder interface {
	// CounterInc increments the named counter by 1, creating it on first use
	// with the label keys present in labels.
	CounterInc(name string, labels Labels)
	// HistogramObserve records value for the named histogram, creating it on
	// first use with the label keys present in labels.
	HistogramObserve(name string, value float64, labels Labels)
	// GaugeAdd adds delta (which may be negative) to the named gauge,
	// creating it on first use with the label keys present in labels.
	GaugeAdd(name string, delta float64, labels Labels)
}
