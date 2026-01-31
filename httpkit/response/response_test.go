package response

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/biairmal/go-sdk/errorz"
)

func TestErrorFromErr(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode string
	}{
		{"nil", nil, "ERR_INTERNAL"},
		{"errorz NotFound", errorz.NotFound(), "ERR_NOT_FOUND"},
		{"errorz BadRequest", errorz.BadRequest(), "ERR_BAD_REQUEST"},
		{"plain error", errors.New("plain"), "ERR_INTERNAL"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ErrorFromErr(tt.err)
			if got.Code != tt.wantCode {
				t.Errorf("ErrorFromErr().Code = %v, want %v", got.Code, tt.wantCode)
			}
		})
	}
}

func TestJSON(t *testing.T) {
	w := httptest.NewRecorder()
	body := BaseResponse[any]{Code: "OK", Message: "ok", Data: "test"}
	JSON(w, http.StatusOK, body)
	if w.Code != http.StatusOK {
		t.Errorf("JSON status = %v, want 200", w.Code)
	}
	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %v, want application/json", w.Header().Get("Content-Type"))
	}
	if w.Body.Len() == 0 {
		t.Error("JSON body empty")
	}
}

func TestJSON_nilBody(t *testing.T) {
	w := httptest.NewRecorder()
	JSON(w, http.StatusNoContent, nil)
	if w.Code != http.StatusNoContent {
		t.Errorf("JSON status = %v, want 204", w.Code)
	}
	if w.Body.Len() != 0 {
		t.Errorf("JSON body should be empty, got %d bytes", w.Body.Len())
	}
}

func TestOK_Created_NoContent(t *testing.T) {
	if OK(nil).HTTPStatusCode != http.StatusOK {
		t.Error("OK status should be 200")
	}
	if Created(nil).HTTPStatusCode != http.StatusCreated {
		t.Error("Created status should be 201")
	}
	if NoContent().HTTPStatusCode != http.StatusNoContent {
		t.Error("NoContent status should be 204")
	}
}
