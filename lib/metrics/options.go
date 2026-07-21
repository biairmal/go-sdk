package metrics

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/biairmal/go-sdk/lib/logger"
)

// Option configures NewPrometheus with optional, non-serializable dependencies.
type Option func(*prometheusRecorder)

// WithRegisterer sets the Prometheus registerer metrics are registered
// against. Nil-safe: a nil registerer is ignored, leaving the default
// (prometheus.DefaultRegisterer). Tests typically pass an isolated
// prometheus.NewRegistry() so parallel tests don't collide on metric names.
func WithRegisterer(r prometheus.Registerer) Option {
	return func(rec *prometheusRecorder) {
		if r != nil {
			rec.registerer = r
		}
	}
}

// WithLogger sets an optional logger for internal diagnostics (e.g. a metric
// name reused with a different label set). Nil-safe: a nil logger keeps the
// recorder silent.
func WithLogger(log logger.Logger) Option {
	return func(rec *prometheusRecorder) {
		rec.log = log
	}
}
