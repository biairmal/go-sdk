package middleware

import (
	"net/http"
	"strings"

	"github.com/biairmal/go-sdk/auth"
	"github.com/biairmal/go-sdk/errorz"
	"github.com/biairmal/go-sdk/httpkit/handler"
)

// AuthorizationHeader is the request header the bearer token is read from.
const AuthorizationHeader = "Authorization"

// bearerPrefix is the (case-insensitive) scheme prefix on the Authorization header.
const bearerPrefix = "bearer "

// authOptions holds the configured Auth middleware settings.
type authOptions struct {
	policy auth.Policy
}

// AuthOption configures the Auth middleware.
type AuthOption func(*authOptions)

// WithPolicy sets the route policy deciding which requests require
// authentication. Without it, every request is protected.
func WithPolicy(p auth.Policy) AuthOption {
	return func(o *authOptions) { o.policy = p }
}

// Auth returns a middleware that authenticates requests with v. Public routes
// (per the policy) pass through untouched; protected routes require a valid
// bearer token, and on success the validated claims are stored in the request
// context (auth.ContextWithClaims, which also publishes the subject as user_id).
//
// Failures are written as the standard error envelope: 401 for a missing or
// invalid token, or whatever status the validator's error maps to (e.g. 502
// when a remote identity service is unreachable). A nil validator on a
// protected route fails closed with 500.
func Auth(v auth.Validator, opts ...AuthOption) func(http.Handler) http.Handler {
	options := authOptions{policy: auth.Policy{DefaultProtected: true}}
	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !options.policy.IsProtected(r.Method, r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}
			if v == nil {
				handler.WriteErrorResponse(w, http.StatusInternalServerError,
					errorz.Internal().WithMessage("auth: no validator configured"))
				return
			}
			token, ok := bearerToken(r)
			if !ok {
				handler.WriteErrorResponse(w, http.StatusUnauthorized,
					errorz.Unauthorized().WithMessage("missing bearer token"))
				return
			}
			claims, err := v.Validate(r.Context(), token)
			if err != nil {
				handler.WriteErrorResponse(w, handler.StatusCodeFromError(err), err)
				return
			}
			ctx := auth.ContextWithClaims(r.Context(), claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// bearerToken extracts the token from the Authorization header, requiring the
// "Bearer" scheme (case-insensitive). It returns false when absent or malformed.
func bearerToken(r *http.Request) (string, bool) {
	h := r.Header.Get(AuthorizationHeader)
	if len(h) < len(bearerPrefix) || !strings.EqualFold(h[:len(bearerPrefix)], bearerPrefix) {
		return "", false
	}
	token := strings.TrimSpace(h[len(bearerPrefix):])
	if token == "" {
		return "", false
	}
	return token, true
}
