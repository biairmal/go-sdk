package validator

import pgvalidator "github.com/go-playground/validator/v10"

// Option configures a Validator built by New. Options are for non-serializable
// behavior (funcs) that can't live in Config/YAML; they must be nil/zero-safe.
type Option func(*playgroundValidator)

// WithCustomValidation registers a custom validation rule under tag at
// construction time, equivalent to calling Register after New. A blank tag or
// nil fn is ignored.
func WithCustomValidation(tag string, fn func(fl FieldLevel) bool) Option {
	return func(v *playgroundValidator) {
		if tag == "" || fn == nil {
			return
		}
		if err := v.validate.RegisterValidation(tag, func(fl pgvalidator.FieldLevel) bool {
			return fn(fl)
		}); err != nil {
			return
		}
	}
}
