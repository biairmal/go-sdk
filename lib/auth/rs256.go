package auth

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"

	"github.com/biairmal/go-sdk/lib/errorz"
	"github.com/golang-jwt/jwt/v5"
)

// NewRS256 returns a Validator that verifies RS256-signed JWTs. Exactly one of
// WithPublicKey (static key) or WithJWKSURL (keys fetched by kid) must be set.
func NewRS256(opts ...Option) (Validator, error) {
	s := applyOptions(opts...)
	hasKey := s.publicKey != nil
	hasJWKS := s.jwksURL != ""
	if hasKey == hasJWKS {
		return nil, errorz.BadRequest().
			WithMessage("auth: RS256 requires exactly one of WithPublicKey or WithJWKSURL")
	}

	var factory keyfuncFactory
	if hasKey {
		key := s.publicKey
		factory = func(context.Context) jwt.Keyfunc {
			return func(*jwt.Token) (any, error) { return key, nil }
		}
	} else {
		cache := newJWKSCache(s.jwksURL, s.httpClient)
		factory = func(ctx context.Context) jwt.Keyfunc {
			return func(t *jwt.Token) (any, error) { return cache.keyForToken(ctx, t) }
		}
	}
	return newJWTValidator(AlgorithmRS256, factory, s), nil
}

// parseRSAPublicKeyPEM parses an RSA public key from PEM bytes, accepting both
// PKIX ("PUBLIC KEY") and PKCS#1 ("RSA PUBLIC KEY") encodings.
func parseRSAPublicKeyPEM(pemBytes []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errorz.BadRequest().WithMessage("auth: no PEM block found in RSA public key")
	}
	if key, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
		rsaKey, ok := key.(*rsa.PublicKey)
		if !ok {
			return nil, errorz.BadRequest().WithMessage("auth: PEM public key is not RSA")
		}
		return rsaKey, nil
	}
	if key, err := x509.ParsePKCS1PublicKey(block.Bytes); err == nil {
		return key, nil
	}
	return nil, errorz.BadRequest().WithMessage("auth: unsupported RSA public key PEM format")
}

// parseRSAPrivateKeyPEM parses an RSA private key from PEM bytes, accepting both
// PKCS#1 ("RSA PRIVATE KEY") and PKCS#8 ("PRIVATE KEY") encodings.
func parseRSAPrivateKeyPEM(pemBytes []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errorz.BadRequest().WithMessage("auth: no PEM block found in RSA private key")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, errorz.BadRequest().WithMessage("auth: PEM private key is not RSA")
		}
		return rsaKey, nil
	}
	return nil, errorz.BadRequest().WithMessage("auth: unsupported RSA private key PEM format")
}
