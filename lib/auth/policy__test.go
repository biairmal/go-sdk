package auth

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
)

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
	policy := Policy{DefaultPublic: true}
	if policy.IsProtected("GET", "/anything") {
		t.Error("expected default-public policy to leave requests unprotected")
	}
}

// The zero Policy must protect everything: forgetting a field may never open the API.
func TestPolicyZeroValueIsProtected(t *testing.T) {
	var policy Policy
	if !policy.IsProtected("GET", "/anything") {
		t.Error("zero Policy leaves an unmatched route public, want protected")
	}
	withRules := Policy{Rules: []Rule{{Pattern: "/health", Public: true}}}
	if !withRules.IsProtected("GET", "/orders") {
		t.Error("Policy with only rules leaves an unmatched route public, want protected")
	}
	if withRules.IsProtected("GET", "/health") {
		t.Error("rule for /health did not make it public")
	}
}

// A YAML file that omits default_protected is the case that used to fail open.
func TestConfigPolicyFromYAML_DefaultProtected(t *testing.T) {
	tests := []struct {
		name          string
		yaml          string
		wantProtected bool
	}{
		{name: "key missing", yaml: "mode: local\n", wantProtected: true},
		{name: "explicit true", yaml: "default_protected: true\n", wantProtected: true},
		{name: "explicit false", yaml: "default_protected: false\n", wantProtected: false},
		{name: "rules but no key", yaml: "rules:\n  - pattern: /health\n    public: true\n", wantProtected: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := viper.New()
			v.SetConfigType("yaml")
			if err := v.ReadConfig(strings.NewReader(tt.yaml)); err != nil {
				t.Fatal(err)
			}
			var cfg Config
			if err := v.Unmarshal(&cfg); err != nil {
				t.Fatal(err)
			}
			if got := cfg.Policy().IsProtected("GET", "/unmatched"); got != tt.wantProtected {
				t.Errorf("IsProtected = %v, want %v", got, tt.wantProtected)
			}
		})
	}
}
