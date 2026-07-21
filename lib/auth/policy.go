package auth

import "strings"

// Rule declares whether requests matching a path pattern (and optional method)
// are public. Rules are evaluated in order and the first match wins.
type Rule struct {
	// Method, when non-empty, restricts the rule to that HTTP method
	// (case-insensitive). Empty matches any method.
	Method string `mapstructure:"method"`
	// Pattern is a path glob: a literal segment matches itself, "*" matches
	// exactly one segment, and "**" matches zero or more segments.
	Pattern string `mapstructure:"pattern"`
	// Public marks matching requests as not requiring authentication.
	Public bool `mapstructure:"public"`
}

// Policy decides whether a given request must be authenticated. It is built
// from configuration (Config.Policy) and passed to the HTTP middleware.
type Policy struct {
	// DefaultProtected is the verdict when no rule matches. When true, an
	// unmatched request requires authentication.
	DefaultProtected bool `mapstructure:"default_protected"`
	// Rules are evaluated in order; the first match decides.
	Rules []Rule `mapstructure:"rules"`
}

// IsProtected reports whether a request for method and path requires
// authentication. The first matching rule decides (protected = !rule.Public);
// with no match the result is DefaultProtected.
func (p Policy) IsProtected(method, path string) bool {
	for _, r := range p.Rules {
		if r.matches(method, path) {
			return !r.Public
		}
	}
	return p.DefaultProtected
}

// matches reports whether r applies to a request for method and path.
func (r Rule) matches(method, path string) bool {
	if r.Method != "" && !strings.EqualFold(r.Method, method) {
		return false
	}
	return matchPattern(r.Pattern, path)
}

// matchPattern reports whether path matches a glob pattern under the segment
// semantics documented on Rule.Pattern.
func matchPattern(pattern, path string) bool {
	return matchSegments(splitPath(pattern), splitPath(path))
}

// splitPath trims surrounding slashes and splits on "/", so "/foo/" and "/foo"
// normalize to the same segments and "/" yields no segments.
func splitPath(p string) []string {
	p = strings.Trim(p, "/")
	if p == "" {
		return nil
	}
	return strings.Split(p, "/")
}

// matchSegments matches pattern segments against path segments. "**" matches
// zero or more segments via backtracking; "*" matches exactly one; any other
// segment is a case-sensitive literal.
func matchSegments(pat, seg []string) bool {
	if len(pat) == 0 {
		return len(seg) == 0
	}
	switch pat[0] {
	case "**":
		// Consume zero segments, or one segment and retry the same "**".
		if matchSegments(pat[1:], seg) {
			return true
		}
		return len(seg) > 0 && matchSegments(pat, seg[1:])
	case "*":
		return len(seg) > 0 && matchSegments(pat[1:], seg[1:])
	default:
		return len(seg) > 0 && pat[0] == seg[0] && matchSegments(pat[1:], seg[1:])
	}
}
