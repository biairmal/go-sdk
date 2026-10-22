package authz

import (
	"context"
	"sync"

	"github.com/biairmal/go-sdk/lib/auth"
)

// TenantID returns the tenant claim ("tid") of the authenticated caller, and
// whether it was present and non-empty. It reads the default claim name; an
// Authorizer configured with another TenantClaim resolves tenants itself.
func TenantID(ctx context.Context) (string, bool) {
	return tenantFromClaims(ctx, DefaultConfig().TenantClaim)
}

// IsSuperAdmin reports whether the authenticated caller carries the default
// super admin claim ("sa": true).
func IsSuperAdmin(ctx context.Context) bool {
	return superFromClaims(ctx, DefaultConfig().SuperClaim)
}

// tenantFromClaims reads a string tenant claim from the context claims.
func tenantFromClaims(ctx context.Context, claim string) (string, bool) {
	c, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return "", false
	}
	v, ok := c.Get(claim)
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok && s != ""
}

// superFromClaims reads a boolean super admin claim from the context claims.
func superFromClaims(ctx context.Context, claim string) bool {
	c, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return false
	}
	v, ok := c.Get(claim)
	if !ok {
		return false
	}
	b, ok := v.(bool)
	return ok && b
}

// memoCtxKey is the private context key for the per-request memo.
type memoCtxKey struct{}

// memo caches each Authorizer's effective permissions for one request.
type memo struct {
	mu    sync.Mutex
	perms map[*authorizer][]string
}

// WithMemo returns a copy of ctx that memoises effective permissions for the
// rest of the request, so repeated Can/Require calls resolve them once. The
// RequirePermission middleware installs it; calling it again is a no-op.
func WithMemo(ctx context.Context) context.Context {
	if _, ok := ctx.Value(memoCtxKey{}).(*memo); ok {
		return ctx
	}
	return context.WithValue(ctx, memoCtxKey{}, &memo{perms: map[*authorizer][]string{}})
}

// memoFrom returns the request memo, or nil when none is installed.
func memoFrom(ctx context.Context) *memo {
	m, ok := ctx.Value(memoCtxKey{}).(*memo)
	if !ok {
		return nil
	}
	return m
}
