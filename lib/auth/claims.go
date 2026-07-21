package auth

import (
	"context"
	"strings"

	"github.com/biairmal/go-sdk/lib/ctxkit"
)

// Claims is the read-only view of a validated token's contents. Backends
// (local JWT, remote) build a Claims and hand it back from Validate; handlers
// read it via ClaimsFromContext.
type Claims interface {
	// Subject returns the authenticated principal (the "sub" claim, or the
	// mapped subject field for a remote validator).
	Subject() string
	// Roles returns the caller's roles, or nil when none are present.
	Roles() []string
	// Get resolves a dotted path (e.g. "org.id") into the raw claim set,
	// returning (value, true) on a hit or (nil, false) on any miss.
	Get(key string) (any, bool)
	// Raw returns the underlying claim map. Callers must not mutate it.
	Raw() map[string]any
}

// mapClaims is the default Claims implementation backed by a decoded claim map.
type mapClaims struct {
	raw     map[string]any
	subject string
	roles   []string
}

// Subject returns the resolved subject.
func (c mapClaims) Subject() string { return c.subject }

// Roles returns the resolved roles.
func (c mapClaims) Roles() []string { return c.roles }

// Raw returns the underlying claim map.
func (c mapClaims) Raw() map[string]any { return c.raw }

// Get resolves a dotted path into the raw claim map.
func (c mapClaims) Get(key string) (any, bool) { return getPath(c.raw, key) }

// NewClaims builds Claims over raw, reading the subject from "sub" and roles
// from "roles". It is exported so ValidatorFunc implementations can produce
// Claims without depending on the concrete type.
func NewClaims(raw map[string]any) Claims {
	var roles []string
	if v, ok := raw["roles"]; ok {
		roles = toStringSlice(v)
	}
	return mapClaims{raw: raw, subject: asString(raw["sub"]), roles: roles}
}

// asString returns v as a string, or "" when v is absent or not a string.
func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// newMappedClaims builds Claims with an explicitly mapped subject and roles,
// used by the remote validator where those fields live at configurable paths.
func newMappedClaims(raw map[string]any, subject string, roles []string) Claims {
	return mapClaims{raw: raw, subject: subject, roles: roles}
}

// getPath walks a dotted path (e.g. "a.b.c") through nested map[string]any
// values. It returns (nil, false) on an empty path, a missing key, or a
// non-map intermediate value.
func getPath(m map[string]any, path string) (any, bool) {
	if path == "" {
		return nil, false
	}
	var cur any = m
	for part := range strings.SplitSeq(path, ".") {
		asMap, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = asMap[part]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

// toStringSlice coerces a claim value into a []string, accepting either a
// []string or a []any of strings (as produced by JSON decoding). Any other
// type yields nil.
func toStringSlice(v any) []string {
	switch vv := v.(type) {
	case []string:
		return vv
	case []any:
		out := make([]string, 0, len(vv))
		for _, item := range vv {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

// claimsCtxKey is the private context key under which Claims are stored.
type claimsCtxKey struct{}

// ContextWithClaims returns a copy of ctx carrying c, and also publishes the
// subject via ctxkit.WithUserID so it surfaces on logs. A nil Claims returns
// ctx unchanged. The HTTP middleware calls this on successful validation.
func ContextWithClaims(ctx context.Context, c Claims) context.Context {
	if c == nil {
		return ctx
	}
	ctx = context.WithValue(ctx, claimsCtxKey{}, c)
	return ctxkit.WithUserID(ctx, c.Subject())
}

// ClaimsFromContext returns the Claims stored in ctx and whether they were present.
func ClaimsFromContext(ctx context.Context) (Claims, bool) {
	c, ok := ctx.Value(claimsCtxKey{}).(Claims)
	return c, ok
}

// SubjectFromContext returns the authenticated subject stored in ctx, or "" if absent.
func SubjectFromContext(ctx context.Context) string {
	if c, ok := ClaimsFromContext(ctx); ok {
		return c.Subject()
	}
	return ""
}
