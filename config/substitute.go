// Package config provides a Viper-based configuration loader with support for
// JSON/YAML files, in-file environment variable substitution, and .env loading.
package config

import (
	"os"
	"regexp"
)

// envSubstRegex matches ${VAR} or ${VAR:default_value}.
// Group 1: variable name; Group 2 (optional): default value.
var envSubstRegex = regexp.MustCompile(`\$\{([^}:]+)(?::([^}]*))?\}`)

// SubstituteEnv replaces ${VAR} and ${VAR:default_value} in b with values from
// the environment. For ${VAR}, the result is os.Getenv("VAR"). For
// ${VAR:default_value}, the default is used when VAR is unset or empty.
// The returned slice is a new allocation; b is not modified.
func SubstituteEnv(b []byte) []byte {
	return envSubstRegex.ReplaceAllFunc(b, func(match []byte) []byte {
		submatches := envSubstRegex.FindSubmatch(match)
		if len(submatches) < 2 {
			return match
		}
		name := string(submatches[1])
		val := os.Getenv(name)
		if len(submatches) == 3 && (val == "") {
			val = string(submatches[2])
		}
		return []byte(val)
	})
}
