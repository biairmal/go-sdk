package httpkit

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestReadiness_nilCheck(t *testing.T) {
	h := Readiness(nil)
	req := httptest.NewRequest(http.MethodGet, "/ready", http.NoBody)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %v, want 200", w.Code)
	}
}

func TestReadiness_ok(t *testing.T) {
	h := Readiness(func(_ context.Context) error { return nil })
	req := httptest.NewRequest(http.MethodGet, "/ready", http.NoBody)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %v, want 200", w.Code)
	}
}

func TestReadiness_fail(t *testing.T) {
	h := Readiness(func(_ context.Context) error { return errors.New("db down") })
	req := httptest.NewRequest(http.MethodGet, "/ready", http.NoBody)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %v, want 503", w.Code)
	}
}
