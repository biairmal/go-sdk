package middleware

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/biairmal/go-sdk/lib/logger"
)

// lineRecorder keeps the fields of every InfoWithContext call.
type lineRecorder struct {
	logger.Logger
	lines []map[string]any
}

func (l *lineRecorder) InfoWithContext(_ context.Context, _ string, fields ...logger.Field) {
	m := map[string]any{}
	for _, f := range fields {
		m[f.Key] = f.Value
	}
	l.lines = append(l.lines, m)
}

// serve runs one POST through Logging and returns what the handler read, the response body and the log lines.
func serve(t *testing.T, opts *LoggingOptions, reqBody, respBody string, hdr map[string]string) (
	handlerSaw, clientGot string, lines []map[string]any,
) {
	t.Helper()
	log := &lineRecorder{Logger: logger.NewNoOp()}
	h := Logging(log, opts)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		handlerSaw = string(b)
		_, _ = w.Write([]byte(respBody))
	}))
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(reqBody))
	req.RemoteAddr = "203.0.113.9:4444"
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return handlerSaw, rec.Body.String(), log.lines
}

func TestLogging_DefaultsLogNoBodies(t *testing.T) {
	saw, got, lines := serve(t, nil, `{"password":"hunter2"}`, `{"token":"abc"}`, nil)
	if saw != `{"password":"hunter2"}` || got != `{"token":"abc"}` {
		t.Fatalf("handler/client bodies changed: %q / %q", saw, got)
	}
	if len(lines) != 2 {
		t.Fatalf("lines = %d, want request and response", len(lines))
	}
	for _, l := range lines {
		if _, has := l["body"]; has {
			t.Errorf("default options logged a body: %v", l)
		}
	}
}

func TestLogging_IPIgnoresForgedHeaderByDefault(t *testing.T) {
	_, _, lines := serve(t, nil, "", "", map[string]string{"X-Forwarded-For": "1.2.3.4"})
	if got := lines[0]["ip"]; got != "203.0.113.9" {
		t.Errorf("ip = %v, want the peer address without a port", got)
	}
}

func TestLogging_BodyTruncatedButHandlerSeesAll(t *testing.T) {
	opts := &LoggingOptions{
		LogRequest: true, LogResponse: true, LogRequestBody: true, LogResponseBody: true,
		MaxBodyBytesForLogging: 5,
	}
	saw, got, lines := serve(t, opts, "hello world", "abcdefgh", nil)
	if saw != "hello world" {
		t.Errorf("handler read %q, want the whole body", saw)
	}
	if got != "abcdefgh" {
		t.Errorf("client got %q, want the whole response", got)
	}
	if lines[0]["body"] != "hello" {
		t.Errorf("logged request body = %v, want %q", lines[0]["body"], "hello")
	}
	if lines[1]["body"] != "abcde" {
		t.Errorf("logged response body = %v, want %q", lines[1]["body"], "abcde")
	}
}

func TestLogging_ZeroLimitIsBounded(t *testing.T) {
	opts := &LoggingOptions{LogRequest: true, LogRequestBody: true}
	big := strings.Repeat("x", 1<<20)
	saw, _, lines := serve(t, opts, big, "", nil)
	if len(saw) != len(big) {
		t.Errorf("handler read %d bytes, want %d", len(saw), len(big))
	}
	if n := len(lines[0]["body"].(string)); n != defaultMaxBodyBytesForLogging {
		t.Errorf("logged %d bytes, want %d", n, defaultMaxBodyBytesForLogging)
	}
}

// Metrics and Tracing build a responseCapture without max: they must not keep the body.
func TestResponseCapture_NoBufferWithoutMax(t *testing.T) {
	c := &responseCapture{ResponseWriter: httptest.NewRecorder(), status: http.StatusOK}
	_, _ = c.Write([]byte(strings.Repeat("x", 1000)))
	if c.buf.Len() != 0 {
		t.Errorf("buffered %d bytes, want 0", c.buf.Len())
	}
}
