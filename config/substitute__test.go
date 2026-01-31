package config

import (
	"os"
	"testing"
)

func TestSubstituteEnv(t *testing.T) {
	tests := []struct {
		name     string
		env      map[string]string
		input    string
		expected string
	}{
		{
			name:     "no substitution",
			input:    "foo: bar",
			expected: "foo: bar",
		},
		{
			name:     "simple substitution",
			env:      map[string]string{"VAR": "value"},
			input:    "key: ${VAR}",
			expected: "key: value",
		},
		{
			name:     "substitution with default when unset",
			input:    "key: ${MISSING:default_value}",
			expected: "key: default_value",
		},
		{
			name:     "substitution with default when empty",
			env:      map[string]string{"EMPTY": ""},
			input:    "key: ${EMPTY:fallback}",
			expected: "key: fallback",
		},
		{
			name:     "set value overrides default",
			env:      map[string]string{"SET": "actual"},
			input:    "key: ${SET:default}",
			expected: "key: actual",
		},
		{
			name:     "multiple substitutions",
			env:      map[string]string{"A": "1", "B": "2"},
			input:    "a: ${A} b: ${B}",
			expected: "a: 1 b: 2",
		},
		{
			name:     "mixed literal and substitution",
			env:      map[string]string{"HOST": "localhost"},
			input:    "url: http://${HOST}:8080",
			expected: "url: http://localhost:8080",
		},
		{
			name:     "empty default",
			input:    "key: ${NONE:}",
			expected: "key: ",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			got := string(SubstituteEnv([]byte(tt.input)))
			if got != tt.expected {
				t.Errorf("SubstituteEnv(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestSubstituteEnv_restoresEnv(t *testing.T) {
	os.Setenv("TEST_VAR", "original")
	defer os.Unsetenv("TEST_VAR")
	SubstituteEnv([]byte("${TEST_VAR}"))
	if os.Getenv("TEST_VAR") != "original" {
		t.Error("SubstituteEnv must not modify environment")
	}
}
