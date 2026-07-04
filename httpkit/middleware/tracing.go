package middleware

import (
	"fmt"
	"net/http"

	"go.opentelemetry.io/otel/propagation"

	"github.com/biairmal/go-sdk/ctxkit"
	"github.com/biairmal/go-sdk/tracer"
)

// tracePropagator extracts/injects W3C trace context (traceparent/tracestate).
// It is stateless, so a single instance is safe to share across requests.
var tracePropagator = propagation.TraceContext{}

// Tracing returns a middleware that starts a server span for each request. It
// extracts any inbound W3C trace context (traceparent/tracestate header) so
// the span joins the caller's trace, and publishes the resulting trace id via
// ctxkit.WithTraceID so it appears on every log line for this request.
//
// On a 5xx response the span is marked as failed. Place Tracing after
// RequestID/Correlation and before Logging in the chain so the trace id is
// available to the request logger.
func Tracing(t tracer.Tracer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := tracePropagator.Extract(r.Context(), propagation.HeaderCarrier(r.Header))

			path, _, method := requestMeta(r)
			ctx, span := t.Start(ctx, method+" "+path, tracer.WithSpanKind(tracer.SpanKindServer))
			defer span.End()

			ctx = ctxkit.WithTraceID(ctx, span.SpanContext().TraceID)

			capture := &responseCapture{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(capture, r.WithContext(ctx))

			if capture.status >= http.StatusInternalServerError {
				span.SetError(fmt.Errorf("http %d", capture.status))
			}
		})
	}
}
