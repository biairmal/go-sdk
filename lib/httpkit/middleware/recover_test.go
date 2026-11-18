package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/biairmal/go-sdk/lib/logger"
)

// errorRecorder records ErrorWithContext calls; every other method is a no-op.
type errorRecorder struct {
	logger.Logger
	msgs   []string
	fields [][]logger.Field
}

func (e *errorRecorder) ErrorWithContext(_ context.Context, msg string, fields ...logger.Field) {
	e.msgs = append(e.msgs, msg)
	e.fields = append(e.fields, fields)
}

func TestRecover(t *testing.T) {
	panicHandler := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		panic("test panic")
	})
	log := &errorRecorder{Logger: logger.NewNoOp()}
	h := Recover(log)(panicHandler)
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %v, want 500", w.Code)
	}
	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %v", w.Header().Get("Content-Type"))
	}
	if strings.Contains(w.Body.String(), "test panic") {
		t.Errorf("body leaks the panic value: %s", w.Body.String())
	}
	if len(log.msgs) != 1 {
		t.Fatalf("logged %d errors, want 1", len(log.msgs))
	}
	got := map[string]any{}
	for _, f := range log.fields[0] {
		got[f.Key] = f.Value
	}
	if got["panic"] != "test panic" || got["method"] != http.MethodGet || got["path"] != "/" {
		t.Errorf("fields = %v", got)
	}
	if s, _ := got["stack"].(string); !strings.Contains(s, "recover_test.go") {
		t.Errorf("stack does not point at the panic: %q", s)
	}
}

func TestRecover_nilLogger(t *testing.T) {
	h := Recover(nil)(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) { panic("boom") }))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", http.NoBody))
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %v, want 500", w.Code)
	}
}

func TestRecover_noPanic(t *testing.T) {
	okHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := Recover(nil)(okHandler)
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %v, want 200", w.Code)
	}
}
