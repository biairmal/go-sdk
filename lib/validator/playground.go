package validator

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	pgvalidator "github.com/go-playground/validator/v10"

	"github.com/biairmal/go-sdk/lib/errorz"
)

// playgroundValidator implements Validator using go-playground/validator/v10.
type playgroundValidator struct {
	validate *pgvalidator.Validate
}

// newPlayground builds a playgroundValidator from cfg and opts.
func newPlayground(cfg Config, opts ...Option) *playgroundValidator {
	cfg = withDefaults(cfg)
	if err := cfg.Validate(); err != nil {
		cfg = DefaultConfig()
	}

	validate := pgvalidator.New()
	validate.SetTagName(cfg.TagName)
	if cfg.FieldNameTag != "" {
		validate.RegisterTagNameFunc(fieldNameFunc(cfg.FieldNameTag))
	}

	v := &playgroundValidator{validate: validate}
	for _, opt := range opts {
		if opt != nil {
			opt(v)
		}
	}
	return v
}

// fieldNameFunc builds a pgvalidator tag-name func that reads the field name
// from tagName (e.g. "json") instead of the Go struct field name.
func fieldNameFunc(tagName string) func(reflect.StructField) string {
	return func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get(tagName), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	}
}

// ValidateStruct validates s using its struct tags.
func (v *playgroundValidator) ValidateStruct(s any) error {
	return translateErr(v.validate.Struct(s), "validator: struct validation failed")
}

// ValidateVar validates a single value against a tag expression.
func (v *playgroundValidator) ValidateVar(field any, tag string) error {
	return translateErr(v.validate.Var(field, tag), "validator: var validation failed")
}

// Register adds a custom validation function under tag.
func (v *playgroundValidator) Register(tag string, fn func(fl FieldLevel) bool) error {
	if err := v.validate.RegisterValidation(tag, func(fl pgvalidator.FieldLevel) bool {
		return fn(fl)
	}); err != nil {
		return errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("validator: register custom validation")
	}
	return nil
}

// translateErr maps a go-playground/validator error into an errorz error.
// Field validation failures become a CodeBadRequest error with per-field
// messages under "fields"; anything else (e.g. a non-struct target) becomes
// a CodeInternal error wrapping the cause.
func translateErr(err error, internalMsg string) error {
	if err == nil {
		return nil
	}

	var verrs pgvalidator.ValidationErrors
	if errors.As(err, &verrs) {
		fields := make(map[string]string, len(verrs))
		for _, fe := range verrs {
			name := fe.Field()
			if name == "" {
				name = "value"
			}
			fields[name] = fieldMessage(fe)
		}
		return errorz.BadRequest().WithMessage("validation failed").WithMeta("fields", fields)
	}

	return errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage(internalMsg)
}

// fieldMessage renders a human-readable message for a single field error.
func fieldMessage(fe pgvalidator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return fmt.Sprintf("%s is required", fe.Field())
	case "email":
		return fmt.Sprintf("%s must be a valid email address", fe.Field())
	case "gte":
		return fmt.Sprintf("%s must be greater than or equal to %s", fe.Field(), fe.Param())
	case "lte":
		return fmt.Sprintf("%s must be less than or equal to %s", fe.Field(), fe.Param())
	case "gt":
		return fmt.Sprintf("%s must be greater than %s", fe.Field(), fe.Param())
	case "lt":
		return fmt.Sprintf("%s must be less than %s", fe.Field(), fe.Param())
	case "min":
		return fmt.Sprintf("%s must be at least %s", fe.Field(), fe.Param())
	case "max":
		return fmt.Sprintf("%s must be at most %s", fe.Field(), fe.Param())
	case "len":
		return fmt.Sprintf("%s must have length %s", fe.Field(), fe.Param())
	case "oneof":
		return fmt.Sprintf("%s must be one of [%s]", fe.Field(), fe.Param())
	default:
		if fe.Param() != "" {
			return fmt.Sprintf("%s failed on the '%s=%s' rule", fe.Field(), fe.Tag(), fe.Param())
		}
		return fmt.Sprintf("%s failed on the '%s' rule", fe.Field(), fe.Tag())
	}
}
