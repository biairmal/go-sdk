package auth

import (
	"context"

	"github.com/biairmal/go-sdk/lib/errorz"
	"github.com/golang-jwt/jwt/v5"
)

// NewHS256 returns a Validator that verifies HS256-signed JWTs using secret.
// Options set the expected issuer/audience and clock-skew leeway.
func NewHS256(secret string, opts ...Option) (Validator, error) {
	if secret == "" {
		return nil, errorz.BadRequest().WithMessage("auth: hs256 secret is required")
	}
	s := applyOptions(opts...)
	key := []byte(secret)
	factory := func(context.Context) jwt.Keyfunc {
		return func(*jwt.Token) (any, error) { return key, nil }
	}
	return newJWTValidator(AlgorithmHS256, factory, s), nil
}
