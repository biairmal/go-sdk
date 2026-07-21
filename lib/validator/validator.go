// Package validator validates structs and single values against tag-driven
// rules, wrapping github.com/go-playground/validator/v10 behind an SDK-native
// interface so the backend stays swappable and consumers never import the
// third-party package directly.
//
// Validation failures are returned as *errorz.Error with code
// errorz.CodeBadRequest and a per-field message map under the "fields" meta
// key, so they flow through httpkit as clean 400 responses with no extra
// wiring.
//
// Example usage:
//
//	v := validator.New(validator.Config{TagName: "validate", FieldNameTag: "json"})
//
//	type CreateUser struct {
//		Email string `json:"email" validate:"required,email"`
//		Age   int    `json:"age"   validate:"gte=0,lte=130"`
//	}
//
//	if err := v.ValidateStruct(body); err != nil {
//		return nil, err // → 400 with per-field messages
//	}
package validator

import pgvalidator "github.com/go-playground/validator/v10"

// FieldLevel is a thin alias over the backend's field accessor so callers
// writing custom validations don't need to import go-playground/validator
// directly.
type FieldLevel = pgvalidator.FieldLevel

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -destination=../../mocks/validator/mock_validator.go -package=mockvalidator github.com/biairmal/go-sdk/lib/validator Validator

// Validator validates values against struct tags.
type Validator interface {
	// ValidateStruct validates s using its struct tags. Returns nil when
	// valid, or an *errorz.Error (code errorz.CodeBadRequest) whose Meta
	// carries per-field messages under "fields". Non-validation failures
	// (e.g. s is not a struct) are wrapped with errorz.CodeInternal instead.
	ValidateStruct(s any) error
	// ValidateVar validates a single value against a tag expression, e.g.
	// ValidateVar(email, "required,email"). Error shape matches ValidateStruct.
	ValidateVar(field any, tag string) error
	// Register adds a custom validation function under a tag name. It can
	// also be set at construction time via WithCustomValidation.
	Register(tag string, fn func(fl FieldLevel) bool) error
}

// New builds a Validator backed by go-playground/validator/v10. Cfg carries
// the YAML-able knobs; zero-valued fields fall back to DefaultConfig(). Opts
// register non-serializable custom-rule funcs.
func New(cfg Config, opts ...Option) Validator {
	return newPlayground(cfg, opts...)
}
