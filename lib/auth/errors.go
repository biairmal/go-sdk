package auth

import (
	"fmt"

	"github.com/biairmal/go-sdk/lib/errorz"
)

// Token error sentinels. Each wraps errorz.ErrUnauthorized so that both
// errors.Is(err, auth.ErrInvalidToken) and errors.Is(err, errorz.ErrUnauthorized)
// hold, and so the coded *errorz.Error produced by unauthorized maps to HTTP 401
// via httpkit's StatusCodeFromError.
var (
	// ErrMissingToken indicates no bearer token was presented on a protected route.
	ErrMissingToken = fmt.Errorf("auth: missing bearer token: %w", errorz.ErrUnauthorized)
	// ErrInvalidToken indicates the token failed signature, structure, or claim checks.
	ErrInvalidToken = fmt.Errorf("auth: invalid token: %w", errorz.ErrUnauthorized)
	// ErrTokenExpired indicates the token's exp claim is in the past (beyond leeway).
	ErrTokenExpired = fmt.Errorf("auth: token expired: %w", errorz.ErrUnauthorized)
	// ErrTokenInactive indicates a remote validator reported the token as inactive.
	ErrTokenInactive = fmt.Errorf("auth: token inactive: %w", errorz.ErrUnauthorized)
	// ErrUnknownKeyID indicates the token's kid was not found in the JWKS after refresh.
	ErrUnknownKeyID = fmt.Errorf("auth: unknown key id: %w", errorz.ErrUnauthorized)
)

// unauthorized wraps a token sentinel into a coded *errorz.Error (returned as
// error) so callers can both errors.Is against the sentinel and have the error
// resolve to a 401 at the HTTP edge.
func unauthorized(sentinel error) error {
	return errorz.Wrap(sentinel).WithCode(errorz.CodeUnauthorized).WithMessage(sentinel.Error())
}
