package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
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
	name  string
}

func (s *fakeSpan) End()               { s.ended = true }
func (s *fakeSpan) SetError(err error) { s.err = err }
func (s *fakeSpan) SetAttributes(attrs map[string]any) {
	if s.attrs == nil {
		s.attrs = map[string]any{}
	}
	for k, v := range attrs {
		s.attrs[k] = v
	}
}
func (s *fakeSpan) SpanContext() tracer.SpanContext { return s.sc }
func (s *fakeSpan) SetName(name string)             { s.name = name }

// fakeTracer is a hand-rolled tracer.Tracer test double. Start records the
// span it returns so the test can inspect it after the handler runs.
type fakeTracer struct {
	started *fakeSpan
}

func (f *fakeTracer) Start(
	ctx context.Context, name string, _ ...tracer.SpanOption,
) (context.Context, tracer.Span) {
	f.started = &fakeSpan{sc: tracer.SpanContext{TraceID: "fake-trace-id"}, name: name}
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

// routeKey stands in for a router (chi) that puts a route context on the request
// before its middleware runs and fills the pattern in while routing.
type routeKey struct{}

type routeHolder struct{ pattern string }

func TestTracing_RouteFunc(t *testing.T) {
	routeOf := func(r *http.Request) string {
		h, ok := r.Context().Value(routeKey{}).(*routeHolder)
		if !ok {
			return ""
		}
		return h.pattern
	}
	tests := []struct {
		name      string
		opts      []TracingOption
		pattern   string
		wantName  string
		wantRoute any
	}{
		{
			"pattern matched",
			[]TracingOption{WithRouteFunc(routeOf)},
			"/v1/notes/{id}", "GET /v1/notes/{id}", "/v1/notes/{id}",
		},
		{"unknown route keeps raw path", []TracingOption{WithRouteFunc(routeOf)}, "", "GET /v1/notes/5f0c", nil},
		{"no route func keeps raw path", nil, "/v1/notes/{id}", "GET /v1/notes/5f0c", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ft := &fakeTracer{}
			inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if h, ok := r.Context().Value(routeKey{}).(*routeHolder); ok {
					h.pattern = tt.pattern
				}
				w.WriteHeader(http.StatusTeapot)
			})
			router := func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), routeKey{}, &routeHolder{})))
				})
			}
			req := httptest.NewRequest(http.MethodGet, "/v1/notes/5f0c", http.NoBody)
			router(Tracing(ft, tt.opts...)(inner)).ServeHTTP(httptest.NewRecorder(), req)

			if ft.started.name != tt.wantName {
				t.Errorf("span name = %q, want %q", ft.started.name, tt.wantName)
			}
			if got := ft.started.attrs["http.route"]; got != tt.wantRoute {
				t.Errorf("http.route = %v, want %v", got, tt.wantRoute)
			}
			want := map[string]any{
				"http.request.method": "GET", "url.path": "/v1/notes/5f0c", "http.response.status_code": http.StatusTeapot,
			}
			for k, v := range want {
				if ft.started.attrs[k] != v {
					t.Errorf("%s = %v, want %v", k, ft.started.attrs[k], v)
				}
			}
		})
	}
}

func TestTracing_RouteFuncServeMuxPattern(t *testing.T) {
	ft := &fakeTracer{}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/notes/{id}", func(http.ResponseWriter, *http.Request) {})
	h := Tracing(ft, WithRouteFunc(func(r *http.Request) string {
		return strings.TrimPrefix(r.Pattern, r.Method+" ")
	}))(mux)
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/v1/notes/5f0c", http.NoBody))

	if want := "GET /v1/notes/{id}"; ft.started.name != want {
		t.Errorf("span name = %q, want %q", ft.started.name, want)
	}
}
