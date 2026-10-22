package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

func TestIssuerWritesKeyID(t *testing.T) {
	tests := []struct {
		name    string
		kid     string
		wantKID any
	}{
		{name: "kid set", kid: "2026-10-a", wantKID: "2026-10-a"},
		{name: "blank kid ignored", kid: "", wantKID: nil},
	}
	key := genRSAKey(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issuer, err := NewRS256Issuer(key, WithKeyID(tt.kid))
			if err != nil {
				t.Fatalf("NewRS256Issuer: %v", err)
			}
			token, err := issuer.Issue("user-1")
			if err != nil {
				t.Fatalf("Issue: %v", err)
			}
			if got := headerOf(t, token)["kid"]; got != tt.wantKID {
				t.Errorf("kid = %v, want %v", got, tt.wantKID)
			}
		})
	}
}

func TestJWKSHandlerServesKeys(t *testing.T) {
	key := genRSAKey(t)
	h := JWKSHandler(func() []PublicKey {
		return []PublicKey{{KID: "a", Key: &key.PublicKey}, {KID: "nil-skipped"}}
	})
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, "/.well-known/jwks.json", http.NoBody))

	if got := rec.Header().Get("Cache-Control"); got != "public, max-age=300" {
		t.Errorf("Cache-Control = %q", got)
	}
	var set jwkSet
	if err := json.Unmarshal(rec.Body.Bytes(), &set); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(set.Keys) != 1 {
		t.Fatalf("keys = %d, want 1", len(set.Keys))
	}
	k := set.Keys[0]
	if k.Kty != "RSA" || k.Kid != "a" || k.Use != "sig" || k.Alg != "RS256" {
		t.Errorf("unexpected jwk header fields: %+v", k)
	}
	pub, err := jwkToRSA(&k)
	if err != nil || pub.N.Cmp(key.N) != 0 || pub.E != key.E {
		t.Errorf("jwk does not round-trip to the public key (err=%v)", err)
	}
}

func TestJWKSHandlerRoundTrip(t *testing.T) {
	key := genRSAKey(t)
	var fetches int32
	jwks := JWKSHandler(func() []PublicKey { return []PublicKey{{KID: "k1", Key: &key.PublicKey}} })
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&fetches, 1)
		jwks(w, r)
	}))
	t.Cleanup(srv.Close)

	v, err := NewRS256(WithJWKSURL(srv.URL))
	if err != nil {
		t.Fatalf("NewRS256: %v", err)
	}

	issuer, _ := NewRS256Issuer(key, WithKeyID("k1"))
	token, _ := issuer.Issue("user-1")
	claims, err := v.Validate(context.Background(), token)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if claims.Subject() != "user-1" {
		t.Errorf("subject = %q, want user-1", claims.Subject())
	}

	// An unknown kid inside the refresh cooldown is rejected without a refetch.
	other, _ := NewRS256Issuer(key, WithKeyID("k2"))
	token2, _ := other.Issue("user-1")
	if _, err := v.Validate(context.Background(), token2); !errors.Is(err, ErrUnknownKeyID) {
		t.Errorf("unknown kid err = %v, want ErrUnknownKeyID", err)
	}
	if got := atomic.LoadInt32(&fetches); got != 1 {
		t.Errorf("jwks fetched %d times, want 1 (cooldown)", got)
	}
}

func headerOf(t *testing.T, token string) map[string]any {
	t.Helper()
	tok, _, err := jwt.NewParser().ParseUnverified(token, jwt.MapClaims{})
	if err != nil {
		t.Fatalf("ParseUnverified: %v", err)
	}
	return tok.Header
}
