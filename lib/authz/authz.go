// Package authz answers "may this caller do X?" from the claims that
// lib/auth put on the request context. Permissions come from one of three
// sources: a token claim, a static role map in config, or a role map fetched
// from the identity service and cached per tenant. Grants may use the
// wildcards "*" and "prefix:*"; checks are always exact keys. A caller with
// the super admin claim is allowed everything.
//
// Example usage:
//
//	az, err := authz.New(&authz.Config{Source: authz.SourceConfig, Roles: map[string][]string{
//		"admin": {"*"}, "member": {"notes:read", "notes:write"},
//	}})
//	if err := az.Require(ctx, "notes:write"); err != nil {
//		return nil, err // 401 without claims, 403 without the permission
//	}
package authz

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -destination=../../mocks/authz/mock_authz.go -package=mockauthz github.com/biairmal/go-sdk/lib/authz Authorizer,ScopeResolver

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/biairmal/go-sdk/lib/auth"
	"github.com/biairmal/go-sdk/lib/errorz"
	"github.com/biairmal/go-sdk/lib/logger"
)

// Authorizer checks the caller's permissions.
type Authorizer interface {
	// Can reports whether the caller holds perm. Any error counts as a denial.
	Can(ctx context.Context, perm string) bool
	// Require returns nil when the caller holds perm, an errorz Unauthorized
	// error when no claims are in ctx, an errorz Forbidden error when perm is
	// missing, or the source's error when permissions cannot be resolved.
	Require(ctx context.Context, perm string) error
	// Permissions returns the caller's effective grants (["*"] for the super
	// admin), or nil when they cannot be resolved.
	Permissions(ctx context.Context) []string
	// RequireIn is Require for one resource: tenant-wide grants are checked
	// first, then the caller's roles in scope (from the ScopeResolver) mapped
	// through the same source. Without a resolver it behaves like Require.
	RequireIn(ctx context.Context, scope Scope, perm string) error
}

// Scope names one resource, e.g. {Type: "event", ID: "…"}.
type Scope struct {
	Type string
	ID   string
}

// ScopeResolver returns a user's roles on one resource. It is implemented by
// the resource service that owns the resource (e.g. over a staff table).
type ScopeResolver interface {
	RolesIn(ctx context.Context, scope Scope, userID string) ([]string, error)
}

// source resolves permissions for one configured Config.Source.
type source interface {
	// callerPerms returns the grants of the authenticated caller.
	callerPerms(ctx context.Context, c auth.Claims, tenantID string) ([]string, error)
	// rolePerms maps role names in a tenant to grants.
	rolePerms(ctx context.Context, tenantID string, roles []string) ([]string, error)
}

// Option configures New with optional, non-serializable behavior.
type Option func(*authorizer)

// WithHTTPClient sets the HTTP client used by the identity source. Nil is ignored.
func WithHTTPClient(c *http.Client) Option {
	return func(a *authorizer) {
		if c != nil {
			a.httpClient = c
		}
	}
}

// WithLogger sets a logger for identity-source refresh failures. Nil-safe.
func WithLogger(l logger.Logger) Option {
	return func(a *authorizer) { a.log = l }
}

// WithScopeResolver enables RequireIn for resource-scoped roles. Nil is ignored.
func WithScopeResolver(r ScopeResolver) Option {
	return func(a *authorizer) {
		if r != nil {
			a.resolver = r
		}
	}
}

// WithServiceToken sets a func returning the bearer token sent to the identity
// service (its internal endpoints require a service token). Nil is ignored.
func WithServiceToken(fn func(ctx context.Context) (string, error)) Option {
	return func(a *authorizer) {
		if fn != nil {
			a.serviceToken = fn
		}
	}
}

// WithClock overrides the time source used for cache expiry. Intended for
// tests; nil is ignored.
func WithClock(now func() time.Time) Option {
	return func(a *authorizer) {
		if now != nil {
			a.now = now
		}
	}
}

// authorizer is the Authorizer built by New.
type authorizer struct {
	cfg          Config
	src          source
	resolver     ScopeResolver
	httpClient   *http.Client
	serviceToken func(ctx context.Context) (string, error)
	log          logger.Logger
	now          func() time.Time
}

// New builds an Authorizer for cfg (nil means DefaultConfig). Zero-valued fields are filled from
// DefaultConfig before validation.
func New(cfg *Config, opts ...Option) (Authorizer, error) {
	c := DefaultConfig()
	if cfg != nil {
		c = withDefaults(*cfg)
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	a := &authorizer{cfg: c, now: time.Now}
	for _, opt := range opts {
		if opt != nil {
			opt(a)
		}
	}
	switch c.Source {
	case SourceToken:
		a.src = tokenSource{claim: c.PermsClaim, roles: c.Roles}
	case SourceConfig:
		a.src = configSource{roles: c.Roles}
	default:
		a.src = newIdentitySource(a)
	}
	return a, nil
}

// Can reports whether the caller holds perm.
func (a *authorizer) Can(ctx context.Context, perm string) bool {
	return a.Require(ctx, perm) == nil
}

// Require returns nil when the caller holds perm.
func (a *authorizer) Require(ctx context.Context, perm string) error {
	grants, err := a.effective(ctx)
	if err != nil {
		return err
	}
	if !Matches(grants, perm) {
		return forbidden(perm)
	}
	return nil
}

// Permissions returns the caller's effective grants.
func (a *authorizer) Permissions(ctx context.Context) []string {
	grants, err := a.effective(ctx)
	if err != nil {
		return nil
	}
	return grants
}

// RequireIn checks tenant-wide grants first, then roles in scope.
func (a *authorizer) RequireIn(ctx context.Context, scope Scope, perm string) error {
	err := a.Require(ctx, perm)
	if err == nil || a.resolver == nil || !errors.Is(err, errorz.ErrForbidden) {
		return err
	}
	c, _ := auth.ClaimsFromContext(ctx) // present: Require got past the claims check
	roles, rerr := a.resolver.RolesIn(ctx, scope, c.Subject())
	if rerr != nil {
		return errorz.Wrap(rerr).WithCode(errorz.CodeInternal).WithMessage("authz: resolve scoped roles")
	}
	if len(roles) == 0 {
		return err
	}
	tenant, _ := tenantFromClaims(ctx, a.cfg.TenantClaim)
	grants, gerr := a.src.rolePerms(ctx, tenant, roles)
	if gerr != nil {
		return gerr
	}
	if !Matches(grants, perm) {
		return err
	}
	return nil
}

// effective resolves the caller's grants, memoised per request when the
// context carries a memo (see WithMemo).
func (a *authorizer) effective(ctx context.Context) ([]string, error) {
	c, ok := auth.ClaimsFromContext(ctx)
	if !ok || c == nil {
		return nil, errorz.Unauthorized().WithMessage("authz: no authenticated caller")
	}
	if superFromClaims(ctx, a.cfg.SuperClaim) {
		return []string{"*"}, nil
	}
	m := memoFrom(ctx)
	if m != nil {
		m.mu.Lock()
		defer m.mu.Unlock()
		if grants, hit := m.perms[a]; hit {
			return grants, nil
		}
	}
	tenant, _ := tenantFromClaims(ctx, a.cfg.TenantClaim)
	grants, err := a.src.callerPerms(ctx, c, tenant)
	if err != nil {
		return nil, err
	}
	if m != nil {
		m.perms[a] = grants
	}
	return grants, nil
}

// forbidden builds the error for a missing permission.
func forbidden(perm string) error {
	return errorz.Forbidden().WithMessage("authz: missing permission "+perm).WithMeta("permission", perm)
}
