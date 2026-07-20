package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func parseUnverified(t *testing.T, token string) jwt.MapClaims {
	t.Helper()
	claims := jwt.MapClaims{}
	_, _, err := jwt.NewParser().ParseUnverified(token, claims)
	if err != nil {
		t.Fatalf("ParseUnverified: %v", err)
	}
	return claims
}

func TestIssuerClaimContents(t *testing.T) {
	issuer, err := NewHS256Issuer("secret",
		WithIssuerName("my-iss"), WithDefaultAudience("my-aud"), WithDefaultTTL(time.Hour))
	if err != nil {
		t.Fatalf("NewHS256Issuer: %v", err)
	}
	iat := time.Unix(1_700_000_000, 0)
	token, err := issuer.Issue("subject-1",
		WithRoles("admin", "ops"),
		WithIssuedAt(iat),
		WithExtraClaims(map[string]any{"tenant": "acme"}),
	)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	claims := parseUnverified(t, token)
	if claims["sub"] != "subject-1" {
		t.Errorf("sub = %v, want subject-1", claims["sub"])
	}
	if claims["iss"] != "my-iss" {
		t.Errorf("iss = %v, want my-iss", claims["iss"])
	}
	if claims["aud"] != "my-aud" {
		t.Errorf("aud = %v, want my-aud", claims["aud"])
	}
	if claims["tenant"] != "acme" {
		t.Errorf("tenant = %v, want acme", claims["tenant"])
	}
	if int64(claims["iat"].(float64)) != iat.Unix() {
		t.Errorf("iat = %v, want %d", claims["iat"], iat.Unix())
	}
	if int64(claims["exp"].(float64)) != iat.Add(time.Hour).Unix() {
		t.Errorf("exp = %v, want %d", claims["exp"], iat.Add(time.Hour).Unix())
	}
}

func TestIssuerRegisteredClaimsWinOverExtras(t *testing.T) {
	issuer, _ := NewHS256Issuer("secret", WithIssuerName("real-iss"))
	token, _ := issuer.Issue("real-sub", WithExtraClaims(map[string]any{"sub": "spoofed", "iss": "spoofed"}))

	claims := parseUnverified(t, token)
	if claims["sub"] != "real-sub" {
		t.Errorf("sub = %v, want real-sub (registered claim must win)", claims["sub"])
	}
	if claims["iss"] != "real-iss" {
		t.Errorf("iss = %v, want real-iss (registered claim must win)", claims["iss"])
	}
}

func TestIssuerEmptySubject(t *testing.T) {
	issuer, _ := NewHS256Issuer("secret")
	if _, err := issuer.Issue(""); err == nil {
		t.Error("expected error for empty subject")
	}
}

func TestNewHS256IssuerEmptySecret(t *testing.T) {
	if _, err := NewHS256Issuer(""); err == nil {
		t.Error("expected error for empty issuer secret")
	}
}

func TestNewRS256IssuerNilKey(t *testing.T) {
	if _, err := NewRS256Issuer(nil); err == nil {
		t.Error("expected error for nil private key")
	}
}
