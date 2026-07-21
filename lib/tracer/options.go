package tracer

import "github.com/biairmal/go-sdk/lib/logger"

// Option configures NewOTel with optional, non-serializable dependencies.
type Option func(*otelTracer)

// WithLogger sets an optional logger for internal diagnostics (exporter
// setup/shutdown failures). Nil-safe: a nil logger keeps the tracer silent.
func WithLogger(log logger.Logger) Option {
	return func(t *otelTracer) {
		t.log = log
	}
}
