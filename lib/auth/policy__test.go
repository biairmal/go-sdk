package auth

import "testing"

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		path    string
		want    bool
	}{
		{"literal match", "/api/users", "/api/users", true},
		{"literal mismatch", "/api/users", "/api/orders", false},
		{"literal case sensitive", "/API/Users", "/api/users", false},
		{"trailing slash normalized", "/health/", "/health", true},
		{"star matches one segment", "/api/*", "/api/users", true},
		{"star does not match zero segments", "/api/*", "/api", false},
		{"star does not match two segments", "/api/*", "/api/users/1", false},
		{"star mid-pattern", "/api/*/edit", "/api/users/edit", true},
		{"doublestar matches zero segments", "/swagger/**", "/swagger", true},
		{"doublestar matches trailing slash", "/swagger/**", "/swagger/", true},
		{"doublestar matches one segment", "/swagger/**", "/swagger/index.html", true},
		{"doublestar matches many segments", "/swagger/**", "/swagger/a/b/c", true},
		{"doublestar mismatch prefix", "/swagger/**", "/api/users", false},
		{"root doublestar matches all", "/**", "/anything/here", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchPattern(tt.pattern, tt.path); got != tt.want {
				t.Errorf("matchPattern(%q, %q) = %v, want %v", tt.pattern, tt.path, got, tt.want)
			}
		})
	}
}

func TestPolicyIsProtected(t *testing.T) {
	policy := Policy{
		DefaultProtected: true,
		Rules: []Rule{
			{Pattern: "/health", Public: true},
			{Pattern: "/swagger/**", Public: true},
			{Method: "POST", Pattern: "/api/auth/login", Public: true},
			{Pattern: "/api/**", Public: false},
		},
	}

	tests := []struct {
		name   string
		method string
		path   string
		want   bool
	}{
		{"public health", "GET", "/health", false},
		{"public swagger subtree", "GET", "/swagger/index.html", false},
		{"method-specific public login", "POST", "/api/auth/login", false},
		{"method mismatch falls through to api rule", "GET", "/api/auth/login", true},
		{"explicit protected api", "GET", "/api/users", true},
		{"first match wins (login before api)", "POST", "/api/auth/login", false},
		{"unmatched uses default protected", "GET", "/random", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := policy.IsProtected(tt.method, tt.path); got != tt.want {
				t.Errorf("IsProtected(%q, %q) = %v, want %v", tt.method, tt.path, got, tt.want)
			}
		})
	}
}

func TestPolicyDefaultPublic(t *testing.T) {
	policy := Policy{DefaultProtected: false}
	if policy.IsProtected("GET", "/anything") {
		t.Error("expected default-public policy to leave requests unprotected")
	}
}
