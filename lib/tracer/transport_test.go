package tracer

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/codes"
)

func TestTransport(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		wantErr bool
	}{
		{"200 ok", http.StatusOK, false},
		{"404 is not an error", http.StatusNotFound, false},
		{"503 is an error", http.StatusServiceUnavailable, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr, exp := newMemTracer(t)
			var gotTraceparent string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotTraceparent = r.Header.Get("traceparent")
				w.WriteHeader(tt.status)
			}))
			defer srv.Close()

			client := &http.Client{Transport: Transport(tr, nil)}
			req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/x?secret=1", http.NoBody)
			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("Do: %v", err)
			}
			resp.Body.Close()
			if req.Header.Get("traceparent") != "" {
				t.Error("caller's request was modified")
			}

			spans := flushed(t, tr, exp)
			if len(spans) != 1 {
				t.Fatalf("exported %d spans, want 1", len(spans))
			}
			s := spans[0]
			if !strings.Contains(gotTraceparent, s.SpanContext.TraceID().String()) ||
				!strings.Contains(gotTraceparent, s.SpanContext.SpanID().String()) {
				t.Errorf("server saw traceparent %q, want the client span %s", gotTraceparent, s.SpanContext.SpanID())
			}
			if want := "GET " + strings.TrimPrefix(srv.URL, "http://"); s.Name != want {
				t.Errorf("span name = %q, want %q", s.Name, want)
			}
			attrs := map[string]string{}
			for _, kv := range s.Attributes {
				attrs[string(kv.Key)] = kv.Value.String()
			}
			if attrs["http.response.status_code"] != strconv.Itoa(tt.status) ||
				attrs["http.request.method"] != "GET" || attrs["server.address"] != "127.0.0.1" {
				t.Errorf("attributes = %v", attrs)
			}
			for k, v := range attrs {
				if strings.Contains(v, "secret") {
					t.Errorf("attribute %s leaks the query string: %q", k, v)
				}
			}
			if gotErr := s.Status.Code == codes.Error; gotErr != tt.wantErr {
				t.Errorf("error span = %v, want %v", gotErr, tt.wantErr)
			}
		})
	}
}

func TestTransport_TransportErrorAndSpanName(t *testing.T) {
	tr, exp := newMemTracer(t)
	failing := roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, errors.New("dial refused") })
	rt := Transport(tr, failing, WithSpanName(func(r *http.Request) string { return "identity " + r.Method + " /v1/x" }))

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://identity/v1/x", http.NoBody)
	if _, err := rt.RoundTrip(req); err == nil {
		t.Fatal("RoundTrip() error = nil, want the transport error")
	}
	s := flushed(t, tr, exp)[0]
	if s.Name != "identity POST /v1/x" || s.Status.Code != codes.Error {
		t.Errorf("span %q status %v, want the custom name and an error", s.Name, s.Status.Code)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
