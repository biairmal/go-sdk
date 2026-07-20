// Package auth provides monolith-first authentication building blocks: JWT
// issuing and validation (HS256, RS256, and RS256 via JWKS), a remote
// (network) validator with fully-mappable claims, a config-driven route
// policy, and an optional TTL validation cache.
//
// The auth.Validator interface is the seam that lets an application run as a
// monolith today (in-process issue + validate) and split into services later
// by configuration alone:
//
//	local              → NewHS256 / NewRS256(WithPublicKey) — no network hop
//	local + jwks_url    → NewRS256(WithJWKSURL) — cached key fetch
//	remote             → NewRemote(...) — call an identity service
//	func               → ValidatorFunc — custom in-code verification
//
// Validation failures are returned as *errorz.Error with code
// errorz.CodeUnauthorized so they flow through httpkit as clean 401 responses.
// The HTTP adapter lives in httpkit/middleware.Auth; on success it publishes the
// authenticated subject via ctxkit.WithUserID and stores the full Claims in the
// request context (auth.ClaimsFromContext).
package auth

import "context"

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -destination=../mocks/auth/mock_auth.go -package=mockauth github.com/biairmal/go-sdk/auth Validator,Claims,Issuer

// Validator verifies a bearer token and returns its claims. Implementations
// return an error wrapping one of this package's token sentinels (all of which
// wrap errorz.ErrUnauthorized) on any authentication failure, or an errorz
// error with a non-401 code (e.g. CodeBadGateway) when the failure is an
// infrastructure problem rather than a bad credential.
type Validator interface {
	// Validate verifies token and returns its claims, or an error on failure.
	Validate(ctx context.Context, token string) (Claims, error)
}

// ValidatorFunc adapts an ordinary function to the Validator interface. It is
// the backing type for auth.mode "func": custom in-process verification.
type ValidatorFunc func(ctx context.Context, token string) (Claims, error)

// Validate calls f. It satisfies the Validator interface.
func (f ValidatorFunc) Validate(ctx context.Context, token string) (Claims, error) {
	return f(ctx, token)
}
