package auth

import (
	"context"
	"errors"

	"github.com/golang-jwt/jwt/v5"
)

// keyfuncFactory produces a jwt.Keyfunc bound to a request context, so that
// key resolution which performs I/O (JWKS fetch) can honor the caller's
// deadline. Static-key backends ignore the context.
type keyfuncFactory func(ctx context.Context) jwt.Keyfunc

// jwtValidator is the shared local-JWT validation core used by the HS256 and
// RS256 backends. It pins the accepted signing algorithm and applies the
// configured issuer/audience/leeway expectations.
type jwtValidator struct {
	parser     *jwt.Parser
	newKeyfunc keyfuncFactory
}

// newJWTValidator builds a validator that accepts only alg-signed tokens and
// enforces the expectations carried in s.
func newJWTValidator(alg string, factory keyfuncFactory, s settings) *jwtValidator {
	opts := []jwt.ParserOption{
		jwt.WithValidMethods([]string{alg}),
		jwt.WithExpirationRequired(),
	}
	if s.leeway > 0 {
		opts = append(opts, jwt.WithLeeway(s.leeway))
	}
	if s.expectedIssuer != "" {
		opts = append(opts, jwt.WithIssuer(s.expectedIssuer))
	}
	if s.expectedAudience != "" {
		opts = append(opts, jwt.WithAudience(s.expectedAudience))
	}
	return &jwtValidator{parser: jwt.NewParser(opts...), newKeyfunc: factory}
}

// Validate parses and verifies token, returning its claims. It satisfies the
// Validator interface.
func (v *jwtValidator) Validate(ctx context.Context, token string) (Claims, error) {
	claims := jwt.MapClaims{}
	parsed, err := v.parser.ParseWithClaims(token, claims, v.newKeyfunc(ctx))
	if err != nil {
		return nil, mapJWTError(err)
	}
	if !parsed.Valid {
		return nil, unauthorized(ErrInvalidToken)
	}
	return NewClaims(map[string]any(claims)), nil
}

// mapJWTError translates a golang-jwt parse error into a package sentinel.
// Key-resolution errors (e.g. an unknown JWKS kid) are already package errors
// and are surfaced unchanged; everything else collapses to expired or invalid.
func mapJWTError(err error) error {
	switch {
	case errors.Is(err, ErrUnknownKeyID):
		return unauthorized(ErrUnknownKeyID)
	case errors.Is(err, jwt.ErrTokenExpired):
		return unauthorized(ErrTokenExpired)
	default:
		return unauthorized(ErrInvalidToken)
	}
}
