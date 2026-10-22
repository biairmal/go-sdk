package authz

import (
	"context"

	"github.com/biairmal/go-sdk/lib/auth"
)

// tokenSource reads the caller's grants from a token claim. Scoped roles have
// no claim to read from, so they are mapped with the optional static role map.
type tokenSource struct {
	claim string
	roles map[string][]string
}

// callerPerms returns the string values of the perms claim.
func (s tokenSource) callerPerms(_ context.Context, c auth.Claims, _ string) ([]string, error) {
	v, ok := c.Get(s.claim)
	if !ok {
		return nil, nil
	}
	return stringSlice(v), nil
}

// rolePerms maps roles with the static role map (empty unless configured).
func (s tokenSource) rolePerms(_ context.Context, _ string, roles []string) ([]string, error) {
	return mapRoles(s.roles, roles), nil
}

// stringSlice coerces a decoded JSON claim into []string, skipping non-strings.
func stringSlice(v any) []string {
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
