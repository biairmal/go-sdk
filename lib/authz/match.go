package authz

import "strings"

// Matches reports whether any grant in grants covers perm. A grant of "*"
// covers everything, a grant ending in ":*" covers every permission under that
// prefix ("notes:*" covers "notes:read" and "notes:read:any"), and any other
// grant must equal perm exactly. Wildcards are only honored in grants: a perm
// that is empty or contains "*" never matches.
func Matches(grants []string, perm string) bool {
	if perm == "" || strings.Contains(perm, "*") {
		return false
	}
	for _, g := range grants {
		if grantCovers(g, perm) {
			return true
		}
	}
	return false
}

// grantCovers reports whether a single grant covers perm.
func grantCovers(grant, perm string) bool {
	if grant == "*" || grant == perm {
		return true
	}
	if prefix, ok := strings.CutSuffix(grant, ":*"); ok {
		return strings.HasPrefix(perm, prefix+":")
	}
	return false
}
