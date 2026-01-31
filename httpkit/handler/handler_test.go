package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/biairmal/go-sdk/errorz"
	"github.com/biairmal/go-sdk/httpkit/response"
)

func TestHandle_success(t *testing.T) {
	h := Handle(func(_ *http.Request) (any, error) {
		return response.OK(map[string]string{"pong": "ok"}), nil
	})
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %v, want 200", w.Code)
	}
	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %v", w.Header().Get("Content-Type"))
	}
}

func TestHandle_error(t *testing.T) {
	h := Handle(func(_ *http.Request) (any, error) {
		return nil, errorz.NotFound()
	})
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %v, want 404", w.Code)
	}
}

func TestHandle_noContent(t *testing.T) {
	h := Handle(func(_ *http.Request) (any, error) {
		return response.NoContent(), nil
	})
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Errorf("status = %v, want 204", w.Code)
	}
	if w.Body.Len() != 0 {
		t.Errorf("body should be empty for 204, got %d bytes", w.Body.Len())
	}
}
