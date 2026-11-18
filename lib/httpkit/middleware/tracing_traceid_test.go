package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/biairmal/go-sdk/lib/errorz"
	"github.com/biairmal/go-sdk/lib/httpkit/handler"
	"github.com/biairmal/go-sdk/lib/httpkit/response"
	"github.com/biairmal/go-sdk/lib/tracer"
)

// newTestOTel builds a real OTel tracer. The exporter dials lazily, so no collector is needed.
func newTestOTel(t *testing.T) tracer.Tracer {
	t.Helper()
	tr, err := tracer.NewOTel(tracer.Config{ServiceName: "test", Endpoint: "127.0.0.1:1", Insecure: true})
	if err != nil {
		t.Fatalf("NewOTel: %v", err)
	}
	return tr
}

func TestTracing_TraceIDInResponse(t *testing.T) {
	tests := []struct {
		name     string
		h        handler.Func
		status   int
		wantBody bool
	}{
		{"200", func(*http.Request) (any, error) { return "ok", nil }, http.StatusOK, true},
		{"201", func(*http.Request) (any, error) { return response.Created("x"), nil }, http.StatusCreated, true},
		{"204", func(*http.Request) (any, error) { return response.NoContent(), nil }, http.StatusNoContent, false},
		{"404", func(*http.Request) (any, error) { return nil, errorz.NotFound() }, http.StatusNotFound, true},
		{"500", func(*http.Request) (any, error) { return nil, errorz.Internal() }, http.StatusInternalServerError, true},
	}
	backends := []struct {
		name   string
		tr     tracer.Tracer
		wantID bool
	}{
		{"otel", newTestOTel(t), true},
		{"noop", tracer.NewNoOp(), false},
	}
	for _, b := range backends {
		for _, tt := range tests {
			t.Run(b.name+"/"+tt.name, func(t *testing.T) {
				w := httptest.NewRecorder()
				Tracing(b.tr)(handler.Handle(tt.h)).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", http.NoBody))

				if w.Code != tt.status {
					t.Fatalf("status = %d, want %d", w.Code, tt.status)
				}
				id, present := w.Header()[TraceIDHeader]
				if present != b.wantID {
					t.Fatalf("header present = %v (%q), want %v", present, id, b.wantID)
				}
				if !tt.wantBody {
					return
				}
				var body map[string]any
				if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
					t.Fatalf("decode body: %v", err)
				}
				got, inBody := body["trace_id"]
				if inBody != b.wantID {
					t.Fatalf("trace_id in body = %v (%v), want %v", inBody, got, b.wantID)
				}
				if b.wantID && got != w.Header().Get(TraceIDHeader) {
					t.Errorf("body trace_id %v != header %q", got, w.Header().Get(TraceIDHeader))
				}
			})
		}
	}
}

func TestTracing_TraceIDFollowsInboundTraceparent(t *testing.T) {
	const callerTraceID = "4bf92f3577b34da6a3ce929d0e0e4736"
	req := httptest.NewRequest(http.MethodGet, "/x", http.NoBody)
	req.Header.Set("traceparent", "00-"+callerTraceID+"-00f067aa0ba902b7-01")
	w := httptest.NewRecorder()
	Tracing(newTestOTel(t))(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).ServeHTTP(w, req)

	if got := w.Header().Get(TraceIDHeader); got != callerTraceID {
		t.Errorf("%s = %q, want the caller's %q", TraceIDHeader, got, callerTraceID)
	}
}
