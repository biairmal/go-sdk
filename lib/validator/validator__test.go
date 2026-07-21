package validator

import (
	"errors"
	"testing"

	"github.com/biairmal/go-sdk/lib/errorz"
)

type createUser struct {
	Email string `json:"email" validate:"required,email"`
	Age   int    `json:"age"   validate:"gte=0,lte=130"`
}

func TestNew_DefaultsTagName(t *testing.T) {
	v := New(Config{})

	if err := v.ValidateStruct(createUser{Email: "a@b.com", Age: 30}); err != nil {
		t.Fatalf("ValidateStruct() with valid struct = %v, want nil", err)
	}
}

func TestPlaygroundValidator_ValidateStruct(t *testing.T) {
	tests := []struct {
		name       string
		cfg        Config
		input      any
		wantErr    bool
		wantFields []string
	}{
		{
			name:  "valid struct passes",
			cfg:   Config{TagName: "validate"},
			input: createUser{Email: "a@b.com", Age: 30},
		},
		{
			name:       "missing required field fails",
			cfg:        Config{TagName: "validate"},
			input:      createUser{Email: "", Age: 30},
			wantErr:    true,
			wantFields: []string{"Email"},
		},
		{
			name:       "out of range field fails",
			cfg:        Config{TagName: "validate"},
			input:      createUser{Email: "a@b.com", Age: 200},
			wantErr:    true,
			wantFields: []string{"Age"},
		},
		{
			name:       "field name tag uses json name",
			cfg:        Config{TagName: "validate", FieldNameTag: "json"},
			input:      createUser{Email: "", Age: 30},
			wantErr:    true,
			wantFields: []string{"email"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := New(tt.cfg)
			err := v.ValidateStruct(tt.input)

			if !tt.wantErr {
				if err != nil {
					t.Fatalf("ValidateStruct() = %v, want nil", err)
				}
				return
			}

			var errz *errorz.Error
			if !errors.As(err, &errz) {
				t.Fatalf("ValidateStruct() error is not *errorz.Error: %v", err)
			}
			if errz.Code != errorz.CodeBadRequest {
				t.Errorf("ValidateStruct() error code = %q, want %q", errz.Code, errorz.CodeBadRequest)
			}

			fields, ok := errz.Meta["fields"].(map[string]string)
			if !ok {
				t.Fatalf("ValidateStruct() error meta[fields] missing or wrong type: %v", errz.Meta)
			}
			for _, want := range tt.wantFields {
				if _, ok := fields[want]; !ok {
					t.Errorf("ValidateStruct() fields = %v, want key %q", fields, want)
				}
			}
		})
	}
}

func TestPlaygroundValidator_ValidateStruct_NonStruct(t *testing.T) {
	v := New(DefaultConfig())

	err := v.ValidateStruct("not a struct")
	if err == nil {
		t.Fatal("ValidateStruct(non-struct) = nil, want error")
	}

	var errz *errorz.Error
	if !errors.As(err, &errz) {
		t.Fatalf("ValidateStruct() error is not *errorz.Error: %v", err)
	}
	if errz.Code != errorz.CodeInternal {
		t.Errorf("ValidateStruct() error code = %q, want %q", errz.Code, errorz.CodeInternal)
	}
}

func TestPlaygroundValidator_ValidateVar(t *testing.T) {
	tests := []struct {
		name    string
		field   any
		tag     string
		wantErr bool
	}{
		{name: "valid email passes", field: "a@b.com", tag: "required,email"},
		{name: "invalid email fails", field: "not-an-email", tag: "required,email", wantErr: true},
		{name: "empty required value fails", field: "", tag: "required", wantErr: true},
	}

	v := New(DefaultConfig())

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateVar(tt.field, tt.tag)

			if !tt.wantErr {
				if err != nil {
					t.Fatalf("ValidateVar() = %v, want nil", err)
				}
				return
			}

			var errz *errorz.Error
			if !errors.As(err, &errz) {
				t.Fatalf("ValidateVar() error is not *errorz.Error: %v", err)
			}
			if errz.Code != errorz.CodeBadRequest {
				t.Errorf("ValidateVar() error code = %q, want %q", errz.Code, errorz.CodeBadRequest)
			}
		})
	}
}

func TestPlaygroundValidator_Register(t *testing.T) {
	type payload struct {
		Code string `validate:"required,is-cool"`
	}

	v := New(DefaultConfig())
	if err := v.Register("is-cool", func(fl FieldLevel) bool {
		return fl.Field().String() == "cool"
	}); err != nil {
		t.Fatalf("Register() = %v, want nil", err)
	}

	if err := v.ValidateStruct(payload{Code: "cool"}); err != nil {
		t.Errorf("ValidateStruct() with matching custom rule = %v, want nil", err)
	}
	if err := v.ValidateStruct(payload{Code: "not-cool"}); err == nil {
		t.Error("ValidateStruct() with failing custom rule = nil, want error")
	}
}

func TestWithCustomValidation(t *testing.T) {
	type payload struct {
		Code string `validate:"required,is-cool"`
	}

	v := New(DefaultConfig(), WithCustomValidation("is-cool", func(fl FieldLevel) bool {
		return fl.Field().String() == "cool"
	}))

	if err := v.ValidateStruct(payload{Code: "cool"}); err != nil {
		t.Errorf("ValidateStruct() with matching custom rule = %v, want nil", err)
	}
	if err := v.ValidateStruct(payload{Code: "not-cool"}); err == nil {
		t.Error("ValidateStruct() with failing custom rule = nil, want error")
	}
}

func TestWithCustomValidation_NilSafe(t *testing.T) {
	// Blank tag and nil fn must be ignored, not panic.
	v := New(DefaultConfig(), WithCustomValidation("", nil), WithCustomValidation("tag", nil))
	if err := v.ValidateStruct(createUser{Email: "a@b.com", Age: 30}); err != nil {
		t.Fatalf("ValidateStruct() = %v, want nil", err)
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{name: "default config is valid", cfg: DefaultConfig()},
		{name: "empty config is valid", cfg: Config{}},
		{name: "whitespace tag name is invalid", cfg: Config{TagName: "bad tag"}, wantErr: true},
		{name: "whitespace field name tag is invalid", cfg: Config{FieldNameTag: "bad tag"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr && err == nil {
				t.Error("Validate() = nil, want error")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Validate() = %v, want nil", err)
			}
		})
	}
}

func TestNew_InvalidConfigFallsBackToDefault(t *testing.T) {
	v := New(Config{TagName: "bad tag"})
	if err := v.ValidateStruct(createUser{Email: "a@b.com", Age: 30}); err != nil {
		t.Fatalf("ValidateStruct() = %v, want nil", err)
	}
}
