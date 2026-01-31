package handler

import (
	"errors"
	"net/http"
	"testing"

	"github.com/biairmal/go-sdk/errorz"
)

func TestStatusCodeFromError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode int
	}{
		{"nil error", nil, http.StatusOK},
		{"non-errorz error", errors.New("generic"), http.StatusInternalServerError},
		{"errorz NotFound", errorz.NotFound(), http.StatusNotFound},
		{"errorz BadRequest", errorz.BadRequest(), http.StatusBadRequest},
		{"errorz Internal", errorz.Internal(), http.StatusInternalServerError},
		{"errorz Unauthorized", errorz.Unauthorized(), http.StatusUnauthorized},
		{"errorz Forbidden", errorz.Forbidden(), http.StatusForbidden},
		{"errorz UnprocessableEntity", errorz.UnprocessableEntity(), http.StatusUnprocessableEntity},
		{"errorz with unknown code", errorz.New("x").WithCode("UNKNOWN"), http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StatusCodeFromError(tt.err)
			if got != tt.wantCode {
				t.Errorf("StatusCodeFromError() = %v, want %v", got, tt.wantCode)
			}
		})
	}
}
