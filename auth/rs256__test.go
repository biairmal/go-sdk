package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"
)

func genRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	return key
}

func TestRS256RoundTrip(t *testing.T) {
	key := genRSAKey(t)
	issuer, err := NewRS256Issuer(key, WithIssuerName("rs-iss"))
	if err != nil {
		t.Fatalf("NewRS256Issuer: %v", err)
	}
	token, err := issuer.Issue("rs-user", WithRoles("admin"))
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	validator, err := NewRS256(WithPublicKey(&key.PublicKey), WithExpectedIssuer("rs-iss"))
	if err != nil {
		t.Fatalf("NewRS256: %v", err)
	}
	claims, err := validator.Validate(context.Background(), token)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if claims.Subject() != "rs-user" {
		t.Errorf("Subject() = %q, want rs-user", claims.Subject())
	}
}

func TestNewRS256KeySourceValidation(t *testing.T) {
	key := genRSAKey(t)
	tests := []struct {
		name    string
		opts    []Option
		wantErr bool
	}{
		{"neither source", nil, true},
		{"both sources", []Option{WithPublicKey(&key.PublicKey), WithJWKSURL("https://x/jwks")}, true},
		{"only public key", []Option{WithPublicKey(&key.PublicKey)}, false},
		{"only jwks", []Option{WithJWKSURL("https://x/jwks")}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewRS256(tt.opts...)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewRS256 err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseRSAPublicKeyPEM(t *testing.T) {
	key := genRSAKey(t)
	pkix, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatalf("MarshalPKIXPublicKey: %v", err)
	}
	pkixPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pkix})
	pkcs1PEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PUBLIC KEY", Bytes: x509.MarshalPKCS1PublicKey(&key.PublicKey)})

	tests := []struct {
		name    string
		pem     []byte
		wantErr bool
	}{
		{"PKIX", pkixPEM, false},
		{"PKCS1", pkcs1PEM, false},
		{"garbage", []byte("not a pem"), true},
		{"empty", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseRSAPublicKeyPEM(tt.pem)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseRSAPublicKeyPEM err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseRSAPrivateKeyPEM(t *testing.T) {
	key := genRSAKey(t)
	pkcs8, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("MarshalPKCS8PrivateKey: %v", err)
	}
	pkcs8PEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8})
	pkcs1PEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})

	tests := []struct {
		name    string
		pem     []byte
		wantErr bool
	}{
		{"PKCS8", pkcs8PEM, false},
		{"PKCS1", pkcs1PEM, false},
		{"garbage", []byte("not a pem"), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseRSAPrivateKeyPEM(tt.pem)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseRSAPrivateKeyPEM err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
