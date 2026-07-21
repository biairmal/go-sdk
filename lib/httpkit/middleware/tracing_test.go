package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/biairmal/go-sdk/lib/ctxkit"
	"github.com/biairmal/go-sdk/lib/tracer"
)

// fakeSpan is a hand-rolled tracer.Span test double that records calls so
// tests can assert on them without depending on the OTel SDK internals.
type fakeSpan struct {
	ended bool
	err   error
	attrs map[string]any
	sc    tracer.SpanContext
}

func (s *fakeSpan) End()                               { s.ended = true }
func (s *fakeSpan) SetError(err error)                 { s.err = err }
func (s *fakeSpan) SetAttributes(attrs map[string]any) { s.attrs = attrs }
func (s *fakeSpan) SpanContext() tracer.SpanContext    { return s.sc }

// fakeTracer is a hand-rolled tracer.Tracer test double. Start records the
// span it returns so the test can inspect it after the handler runs.
type fakeTracer struct {
	started *fakeSpan
}

func (f *fakeTracer) Start(
	ctx context.Context, _ string, _ ...tracer.SpanOption,
) (context.Context, tracer.Span) {
	f.started = &fakeSpan{sc: tracer.SpanContext{TraceID: "fake-trace-id"}}
	return ctx, f.started
}

func (f *fakeTracer) Shutdown(_ context.Context) error { return nil }

func TestTracing_PublishesTraceIDAndEndsSpan(t *testing.T) {
	ft := &fakeTracer{}
	var ctxTraceID string
	handler := Tracing(ft)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		ctxTraceID = ctxkit.TraceID(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/orders/42", http.NoBody)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if ctxTraceID != "fake-trace-id" {
		t.Errorf("ctxkit.TraceID() = %q, want %q", ctxTraceID, "fake-trace-id")
	}
	if !ft.started.ended {
		t.Error("span was not ended")
	}
}

func TestTracing_SetsErrorOn5xx(t *testing.T) {
	ft := &fakeTracer{}
	handler := Tracing(ft)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))

	req := httptest.NewRequest(http.MethodGet, "/boom", http.NoBody)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if ft.started.err == nil {
		t.Error("expected span error to be set on a 5xx response")
	}
}

func TestTracing_NoErrorOn2xx(t *testing.T) {
	ft := &fakeTracer{}
	handler := Tracing(ft)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/ok", http.NoBody)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if ft.started.err != nil {
		t.Errorf("expected no span error on a 2xx response, got %v", ft.started.err)
	}
}

func TestTracing_ExtractsInboundTraceparent(t *testing.T) {
	ft := &fakeTracer{}
	handler := Tracing(ft)(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if ft.started == nil {
		t.Fatal("span was not started")
	}
}
