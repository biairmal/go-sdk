package tracer

import (
	"fmt"
	"net/http"

	"go.opentelemetry.io/otel/propagation"
)

// transport is an http.RoundTripper that wraps each call in a client span.
type transport struct {
	tracer   Tracer
	base     http.RoundTripper
	spanName func(*http.Request) string
}

// TransportOption configures Transport.
type TransportOption func(*transport)

// WithSpanName sets how a request's client span is named, e.g.
// "identity GET /v1/internal/roles/{id}". Use a route pattern, never the raw
// path, so ids do not end up in span names. The default is "METHOD host".
func WithSpanName(fn func(*http.Request) string) TransportOption {
	return func(t *transport) {
		if fn != nil {
			t.spanName = fn
		}
	}
}

// Transport returns an http.RoundTripper that starts a client span per request
// and injects the W3C traceparent header, so the called service joins the
// caller's trace. A nil base means http.DefaultTransport.
//
// The span carries http.request.method, server.address and
// http.response.status_code; a transport error or a 5xx marks it failed, a 4xx
// does not. Headers, query strings and bodies are never recorded. The span ends
// when the response headers arrive, not when the body has been read.
//
//	client := &http.Client{Timeout: 10 * time.Second, Transport: tracer.Transport(tr, nil)}
func Transport(tr Tracer, base http.RoundTripper, opts ...TransportOption) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	t := &transport{
		tracer:   tr,
		base:     base,
		spanName: func(r *http.Request) string { return r.Method + " " + r.URL.Host },
	}
	for _, opt := range opts {
		if opt != nil {
			opt(t)
		}
	}
	return t
}

// RoundTrip sends req inside a client span.
func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx, span := t.tracer.Start(req.Context(), t.spanName(req), WithSpanKind(SpanKindClient),
		WithAttributes(map[string]any{"http.request.method": req.Method, "server.address": req.URL.Hostname()}))
	defer span.End()

	// A RoundTripper must not modify the caller's request, so the header goes on a clone.
	out := req.Clone(ctx)
	propagation.TraceContext{}.Inject(ctx, propagation.HeaderCarrier(out.Header))

	resp, err := t.base.RoundTrip(out)
	if err != nil {
		span.SetError(err)
		return nil, err
	}
	span.SetAttributes(map[string]any{"http.response.status_code": resp.StatusCode})
	if resp.StatusCode >= http.StatusInternalServerError {
		span.SetError(fmt.Errorf("http %d", resp.StatusCode))
	}
	return resp, nil
}
