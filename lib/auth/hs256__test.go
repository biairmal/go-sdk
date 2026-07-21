package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestNewHS256EmptySecret(t *testing.T) {
	if _, err := NewHS256(""); err == nil {
		t.Fatal("expected error for empty secret")
	}
}

func TestHS256RoundTrip(t *testing.T) {
	const secret = "test-secret-value"
	issuer, err := NewHS256Issuer(secret, WithIssuerName("issuer-x"), WithDefaultAudience("aud-x"))
	if err != nil {
		t.Fatalf("NewHS256Issuer: %v", err)
	}
	token, err := issuer.Issue("user-1", WithRoles("admin"))
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	validator, err := NewHS256(secret, WithExpectedIssuer("issuer-x"), WithExpectedAudience("aud-x"))
	if err != nil {
		t.Fatalf("NewHS256: %v", err)
	}
	claims, err := validator.Validate(context.Background(), token)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if claims.Subject() != "user-1" {
		t.Errorf("Subject() = %q, want user-1", claims.Subject())
	}
	if len(claims.Roles()) != 1 || claims.Roles()[0] != "admin" {
		t.Errorf("Roles() = %v, want [admin]", claims.Roles())
	}
}

func TestHS256ValidateErrors(t *testing.T) {
	const secret = "the-secret"
	issuer, _ := NewHS256Issuer(secret, WithIssuerName("iss"), WithDefaultAudience("aud"))

	tests := []struct {
		name    string
		token   func() string
		build   func() (Validator, error)
		wantErr error
	}{
		{
			name:    "wrong secret",
			token:   func() string { tok, _ := issuer.Issue("u"); return tok },
			build:   func() (Validator, error) { return NewHS256("different-secret") },
			wantErr: ErrInvalidToken,
		},
		{
			name: "expired token",
			token: func() string {
				tok, _ := issuer.Issue("u", WithTTL(time.Minute), WithIssuedAt(time.Now().Add(-2*time.Hour)))
				return tok
			},
			build:   func() (Validator, error) { return NewHS256(secret) },
			wantErr: ErrTokenExpired,
		},
		{
			name:    "issuer mismatch",
			token:   func() string { tok, _ := issuer.Issue("u"); return tok },
			build:   func() (Validator, error) { return NewHS256(secret, WithExpectedIssuer("other")) },
			wantErr: ErrInvalidToken,
		},
		{
			name:    "audience mismatch",
			token:   func() string { tok, _ := issuer.Issue("u"); return tok },
			build:   func() (Validator, error) { return NewHS256(secret, WithExpectedAudience("other")) },
			wantErr: ErrInvalidToken,
		},
		{
			name:    "garbage token",
			token:   func() string { return "not-a-jwt" },
			build:   func() (Validator, error) { return NewHS256(secret) },
			wantErr: ErrInvalidToken,
		},
		{
			name: "alg none rejected",
			token: func() string {
				tok := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{
					"sub": "u", "exp": time.Now().Add(time.Hour).Unix(),
				})
				s, _ := tok.SignedString(jwt.UnsafeAllowNoneSignatureType)
				return s
			},
			build:   func() (Validator, error) { return NewHS256(secret) },
			wantErr: ErrInvalidToken,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := tt.build()
			if err != nil {
				t.Fatalf("build validator: %v", err)
			}
			_, err = v.Validate(context.Background(), tt.token())
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Validate error = %v, want errors.Is %v", err, tt.wantErr)
			}
		})
	}
}

func TestHS256LeewayAcceptsRecentlyExpired(t *testing.T) {
	const secret = "leeway-secret"
	issuer, _ := NewHS256Issuer(secret)
	// Expired 10s ago.
	token, _ := issuer.Issue("u", WithTTL(time.Minute), WithIssuedAt(time.Now().Add(-time.Minute-10*time.Second)))

	v, _ := NewHS256(secret, WithLeeway(30*time.Second))
	if _, err := v.Validate(context.Background(), token); err != nil {
		t.Errorf("expected leeway to accept recently-expired token, got %v", err)
	}
}
