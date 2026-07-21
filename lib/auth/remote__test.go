package auth

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/biairmal/go-sdk/lib/errorz"
)

func remoteServer(t *testing.T, h http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

func TestRemoteForwardHeader(t *testing.T) {
	var gotAuth string
	srv := remoteServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(map[string]any{"sub": "remote-user", "roles": []string{"admin"}})
	})

	v, err := NewRemote(&RemoteConfig{URL: srv.URL})
	if err != nil {
		t.Fatalf("NewRemote: %v", err)
	}
	claims, err := v.Validate(context.Background(), "tok-123")
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if gotAuth != "Bearer tok-123" {
		t.Errorf("forwarded Authorization = %q, want %q", gotAuth, "Bearer tok-123")
	}
	if claims.Subject() != "remote-user" {
		t.Errorf("Subject() = %q, want remote-user", claims.Subject())
	}
	if len(claims.Roles()) != 1 || claims.Roles()[0] != "admin" {
		t.Errorf("Roles() = %v, want [admin]", claims.Roles())
	}
}

func TestRemoteForwardQuery(t *testing.T) {
	var gotToken string
	srv := remoteServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotToken = r.URL.Query().Get("access_token")
		_ = json.NewEncoder(w).Encode(map[string]any{"sub": "u"})
	})

	v, _ := NewRemote(&RemoteConfig{
		URL:     srv.URL,
		Forward: TokenForward{In: ForwardInQuery, Name: "access_token"},
	})
	if _, err := v.Validate(context.Background(), "qtok"); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if gotToken != "qtok" {
		t.Errorf("forwarded query token = %q, want qtok", gotToken)
	}
}

func TestRemoteForwardBody(t *testing.T) {
	var gotBody map[string]string
	srv := remoteServer(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		_ = json.NewEncoder(w).Encode(map[string]any{"sub": "u"})
	})

	v, _ := NewRemote(&RemoteConfig{
		URL:     srv.URL,
		Forward: TokenForward{In: ForwardInBody, Name: "token"},
	})
	if _, err := v.Validate(context.Background(), "btok"); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if gotBody["token"] != "btok" {
		t.Errorf("forwarded body token = %q, want btok", gotBody["token"])
	}
}

func TestRemoteClaimsMappingNested(t *testing.T) {
	srv := remoteServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"user": map[string]any{"id": "nested-user", "perms": []any{"read", "write"}},
			},
		})
	})

	v, _ := NewRemote(&RemoteConfig{
		URL: srv.URL,
		Mapping: ClaimsMapping{
			ClaimsPath:   "data.user",
			SubjectField: "id",
			RolesField:   "perms",
		},
	})
	claims, err := v.Validate(context.Background(), "tok")
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if claims.Subject() != "nested-user" {
		t.Errorf("Subject() = %q, want nested-user", claims.Subject())
	}
	if len(claims.Roles()) != 2 {
		t.Errorf("Roles() = %v, want 2 entries", claims.Roles())
	}
}

func TestRemoteActiveFieldFalse(t *testing.T) {
	srv := remoteServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"sub": "u", "active": false})
	})
	v, _ := NewRemote(&RemoteConfig{
		URL:     srv.URL,
		Mapping: ClaimsMapping{ActiveField: "active"},
	})
	_, err := v.Validate(context.Background(), "tok")
	if !errors.Is(err, ErrTokenInactive) {
		t.Errorf("Validate error = %v, want errors.Is ErrTokenInactive", err)
	}
}

func TestRemoteStatusMapping(t *testing.T) {
	tests := []struct {
		name        string
		status      int
		body        string
		wantErrIs   error
		wantErrCode string
	}{
		{"unauthorized maps to invalid", http.StatusUnauthorized, ``, ErrInvalidToken, errorz.CodeUnauthorized},
		{"forbidden maps to invalid", http.StatusForbidden, ``, ErrInvalidToken, errorz.CodeUnauthorized},
		{"server error maps to bad gateway", http.StatusInternalServerError, ``, nil, errorz.CodeBadGateway},
		{"missing subject", http.StatusOK, `{"roles":["a"]}`, ErrInvalidToken, errorz.CodeUnauthorized},
		{"malformed json", http.StatusOK, `not json`, nil, errorz.CodeBadGateway},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := remoteServer(t, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			})
			v, _ := NewRemote(&RemoteConfig{URL: srv.URL})
			_, err := v.Validate(context.Background(), "tok")
			if err == nil {
				t.Fatal("expected error")
			}
			if tt.wantErrIs != nil && !errors.Is(err, tt.wantErrIs) {
				t.Errorf("error = %v, want errors.Is %v", err, tt.wantErrIs)
			}
			var ez *errorz.Error
			if errors.As(err, &ez) && ez.Code != tt.wantErrCode {
				t.Errorf("error code = %q, want %q", ez.Code, tt.wantErrCode)
			}
		})
	}
}

func TestNewRemoteRequiresURL(t *testing.T) {
	if _, err := NewRemote(&RemoteConfig{}); err == nil {
		t.Error("expected error for empty remote URL")
	}
}
