package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/biairmal/go-sdk/lib/metrics"
)

// MetricsOptions controls the Metrics middleware's behavior. Nil means default.
type MetricsOptions struct {
	// PathNormalizer collapses a request's raw URL path into a low-cardinality
	// label value (e.g. "/orders/42" -> "/orders/{id}") before it's recorded
	// as the "path" label. Nil uses the raw path unmodified, which is
	// dangerous for path-parameterized routes — every distinct id mints a new
	// Prometheus time series. Supply a normalizer backed by your router's
	// matched route pattern in production.
	PathNormalizer func(r *http.Request) string
}

// Metrics returns a middleware that records the standard RED-method HTTP
// metrics via rec: metrics.HTTPRequestsInFlight (gauge, entry/exit),
// metrics.HTTPRequestsTotal (counter) and metrics.HTTPRequestDuration
// (histogram, seconds), both labeled by method, path, and status_code.
//
// Place Metrics outermost in the chain (before Recover) so panics are still
// counted: the in-flight gauge is decremented and the request/duration
// counters recorded via defer regardless of how the handler chain exits.
func Metrics(rec metrics.Recorder, opts *MetricsOptions) func(http.Handler) http.Handler {
	normalize := rawPath
	if opts != nil && opts.PathNormalizer != nil {
		normalize = opts.PathNormalizer
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := normalize(r)

			rec.GaugeAdd(metrics.HTTPRequestsInFlight, 1, nil)
			defer rec.GaugeAdd(metrics.HTTPRequestsInFlight, -1, nil)

			start := time.Now()
			capture := &responseCapture{ResponseWriter: w, status: http.StatusOK}
			defer recordHTTPMetrics(rec, r.Method, path, capture, start)

			next.ServeHTTP(capture, r)
		})
	}
}

// recordHTTPMetrics records the completed-request counter and duration
// histogram. Deferred so it still fires when the handler panics.
func recordHTTPMetrics(rec metrics.Recorder, method, path string, capture *responseCapture, start time.Time) {
	labels := metrics.Labels{"method": method, "path": path, "status_code": strconv.Itoa(capture.status)}
	rec.CounterInc(metrics.HTTPRequestsTotal, labels)
	rec.HistogramObserve(metrics.HTTPRequestDuration, time.Since(start).Seconds(), labels)
}

// rawPath is the default PathNormalizer: the request's URL path, unmodified.
func rawPath(r *http.Request) string {
	path, _, _ := requestMeta(r)
	return path
}
