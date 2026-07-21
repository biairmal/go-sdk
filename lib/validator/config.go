package validator

import (
	"strings"

	"github.com/biairmal/go-sdk/lib/errorz"
)

// Config holds the YAML-able validator settings.
type Config struct {
	// TagName is the struct tag key rules are read from. Default "validate".
	TagName string `mapstructure:"tag_name"`
	// FieldNameTag, when set (e.g. "json"), makes error field names match that
	// tag instead of the Go field name. Default "" (Go field names).
	FieldNameTag string `mapstructure:"field_name_tag"`
}

// DefaultConfig returns a Config with sane defaults: rules are read from the
// "validate" tag and error field names use the Go struct field name.
func DefaultConfig() Config {
	return Config{TagName: "validate"}
}

// Validate checks that Config is well-formed. TagName and FieldNameTag, when
// set, are used as struct tag keys and so must not contain whitespace.
func (c Config) Validate() error {
	if strings.ContainsAny(c.TagName, " \t") {
		return errorz.BadRequest().WithMessage("validator: tag_name must not contain whitespace")
	}
	if strings.ContainsAny(c.FieldNameTag, " \t") {
		return errorz.BadRequest().WithMessage("validator: field_name_tag must not contain whitespace")
	}
	return nil
}

// withDefaults fills zero-valued fields of cfg from DefaultConfig() so
// callers (and YAML) only need to override what differs.
func withDefaults(cfg Config) Config {
	if cfg.TagName == "" {
		cfg.TagName = DefaultConfig().TagName
	}
	return cfg
}
