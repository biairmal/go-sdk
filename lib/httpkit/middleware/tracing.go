package middleware

import (
	"fmt"
	"net/http"

	"go.opentelemetry.io/otel/propagation"

	"github.com/biairmal/go-sdk/lib/ctxkit"
	"github.com/biairmal/go-sdk/lib/httpkit/response"
	"github.com/biairmal/go-sdk/lib/tracer"
)

// tracePropagator extracts/injects W3C trace context (traceparent/tracestate).
// It is stateless, so a single instance is safe to share across requests.
var tracePropagator = propagation.TraceContext{}

// TraceIDHeader is the response header Tracing sets to the request's trace id.
const TraceIDHeader = response.TraceIDHeader

// Tracing returns a middleware that starts a server span for each request. It
// extracts any inbound W3C trace context (traceparent/tracestate header) so
// the span joins the caller's trace, and publishes the resulting trace id via
// ctxkit.WithTraceID so it appears on every log line for this request, and sets
// it as the TraceIDHeader response header (omitted when the tracer gives no id,
// as the NoOp tracer does) so a client can look the trace up.
//
// The span is named "METHOD /raw/path" and carries http.request.method, url.path
// and http.response.status_code. Pass WithRouteFunc to name it by route pattern
// instead, which keeps span names low-cardinality.
//
// On a 5xx response the span is marked as failed. Place Tracing after
// RequestID/Correlation and before Logging in the chain so the trace id is
// available to the request logger.
func Tracing(t tracer.Tracer, opts ...TracingOption) func(http.Handler) http.Handler {
	var o tracingOptions
	for _, opt := range opts {
		if opt != nil {
			opt(&o)
		}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := tracePropagator.Extract(r.Context(), propagation.HeaderCarrier(r.Header))

			path, _, method := requestMeta(r)
			ctx, span := t.Start(ctx, method+" "+path, tracer.WithSpanKind(tracer.SpanKindServer))
			defer span.End()

			traceID := span.SpanContext().TraceID
			ctx = ctxkit.WithTraceID(ctx, traceID)
			if traceID != "" {
				w.Header().Set(TraceIDHeader, traceID)
			}

			capture := &responseCapture{ResponseWriter: w, status: http.StatusOK}
			req := r.WithContext(ctx)
			next.ServeHTTP(capture, req)

			attrs := map[string]any{
				"http.request.method":       method,
				"url.path":                  path,
				"http.response.status_code": capture.status,
			}
			if o.route != nil {
				if route := o.route(req); route != "" {
					span.SetName(method + " " + route)
					attrs["http.route"] = route
				}
			}
			span.SetAttributes(attrs)

			if capture.status >= http.StatusInternalServerError {
				span.SetError(fmt.Errorf("http %d", capture.status))
			}
		})
	}
}

type tracingOptions struct {
	route func(*http.Request) string
}

// TracingOption configures Tracing.
type TracingOption func(*tracingOptions)

// WithRouteFunc sets the function that returns the matched route pattern
// (e.g. "/v1/notes/{id}"). It is called after the handler returns, so a router
// that fills in its match while routing works; with chi, mount Tracing with
// r.Use and pass:
//
//	func(r *http.Request) string { return chi.RouteContext(r.Context()).RoutePattern() }
//
// With a net/http ServeMux wrapped by Tracing, read r.Pattern (minus any "METHOD " prefix):
// the function gets the same *http.Request the mux saw.
//
// When it returns "" (an unknown route, a 404), the span keeps the raw path.
func WithRouteFunc(fn func(*http.Request) string) TracingOption {
	return func(o *tracingOptions) { o.route = fn }
}
