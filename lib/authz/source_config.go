package authz

import (
	"context"

	"github.com/biairmal/go-sdk/lib/auth"
)

// configSource maps the caller's roles to grants with a static role map.
type configSource struct {
	roles map[string][]string
}

// callerPerms maps the caller's role claim.
func (s configSource) callerPerms(_ context.Context, c auth.Claims, _ string) ([]string, error) {
	return mapRoles(s.roles, c.Roles()), nil
}

// rolePerms maps roles with the static role map.
func (s configSource) rolePerms(_ context.Context, _ string, roles []string) ([]string, error) {
	return mapRoles(s.roles, roles), nil
}

// mapRoles returns the union of the grants of roles in m. Unknown roles grant nothing.
func mapRoles(m map[string][]string, roles []string) []string {
	var out []string
	for _, r := range roles {
		out = append(out, m[r]...)
	}
	return out
}
