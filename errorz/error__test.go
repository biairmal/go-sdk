package errorz

import (
	"errors"
	"testing"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name          string
		message       string
		wantMessage   string
		wantSourceSys string
		wantCode      string
		wantErr       error
		wantMeta      map[string]any
	}{
		{
			name:          "creates error with message",
			message:       "test error",
			wantMessage:   "test error",
			wantSourceSys: DefaultSourceSystem,
			wantCode:      "",
			wantErr:       nil,
			wantMeta:      nil,
		},
		{
			name:          "creates error with empty message",
			message:       "",
			wantMessage:   "",
			wantSourceSys: DefaultSourceSystem,
			wantCode:      "",
			wantErr:       nil,
			wantMeta:      nil,
		},
		{
			name:          "creates error with special characters",
			message:       "error: invalid input @#$%",
			wantMessage:   "error: invalid input @#$%",
			wantSourceSys: DefaultSourceSystem,
			wantCode:      "",
			wantErr:       nil,
			wantMeta:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := New(tt.message)
			if got.Message != tt.wantMessage {
				t.Errorf("New().Message = %v, want %v", got.Message, tt.wantMessage)
			}
			if got.SourceSystem != tt.wantSourceSys {
				t.Errorf("New().SourceSystem = %v, want %v", got.SourceSystem, tt.wantSourceSys)
			}
			if got.Code != tt.wantCode {
				t.Errorf("New().Code = %v, want %v", got.Code, tt.wantCode)
			}
			if !errors.Is(got.Err, tt.wantErr) && (got.Err != nil || tt.wantErr != nil) {
				t.Errorf("New().Err = %v, want %v", got.Err, tt.wantErr)
			}
			if got.Meta == nil && tt.wantMeta != nil {
				t.Errorf("New().Meta = nil, want %v", tt.wantMeta)
			}
		})
	}
}

func TestWrap(t *testing.T) {
	standardErr := errors.New("standard error")
	innerErr := errors.New("inner error")

	tests := []struct {
		name          string
		err           error
		wantSourceSys string
		wantErr       error
		wantMessage   string
		wantCode      string
	}{
		{
			name:          "wraps standard error",
			err:           standardErr,
			wantSourceSys: DefaultSourceSystem,
			wantErr:       standardErr,
			wantMessage:   "",
			wantCode:      "",
		},
		{
			name:          "wraps nil error",
			err:           nil,
			wantSourceSys: DefaultSourceSystem,
			wantErr:       nil,
			wantMessage:   "",
			wantCode:      "",
		},
		{
			name:          "wraps wrapped error",
			err:           innerErr,
			wantSourceSys: DefaultSourceSystem,
			wantErr:       innerErr,
			wantMessage:   "",
			wantCode:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Wrap(tt.err)
			if got.SourceSystem != tt.wantSourceSys {
				t.Errorf("Wrap().SourceSystem = %v, want %v", got.SourceSystem, tt.wantSourceSys)
			}
			if tt.err == nil {
				if got.Err != nil {
					t.Errorf("Wrap().Err = %v, want nil", got.Err)
				}
			} else {
				//nolint:errorlint // Direct comparison is valid here since we're using the same error instance
				if got.Err != tt.wantErr {
					t.Errorf("Wrap().Err = %v, want %v", got.Err, tt.wantErr)
				}
			}
			if got.Message != tt.wantMessage {
				t.Errorf("Wrap().Message = %v, want %v", got.Message, tt.wantMessage)
			}
			if got.Code != tt.wantCode {
				t.Errorf("Wrap().Code = %v, want %v", got.Code, tt.wantCode)
			}
		})
	}
}

func TestError_Error(t *testing.T) {
	tests := []struct {
		name   string
		errorz *Error
		want   string
	}{
		{
			name:   "returns message",
			errorz: New("test message"),
			want:   "SourceSystem: application, Message: test message",
		},
		{
			name:   "returns empty message",
			errorz: New(""),
			want:   "SourceSystem: application",
		},
		{
			name:   "returns updated message",
			errorz: New("original").WithMessage("updated"),
			want:   "SourceSystem: application, Message: updated",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.errorz.Error()
			if got != tt.want {
				t.Errorf("Error.Error() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestError_Unwrap(t *testing.T) {
	innerErr := errors.New("inner error")
	wrappedErr := errors.New("wrapped error")

	tests := []struct {
		name   string
		errorz *Error
		want   error
	}{
		{
			name:   "unwraps wrapped error",
			errorz: Wrap(innerErr),
			want:   innerErr,
		},
		{
			name:   "returns nil for non-wrapped error",
			errorz: New("test"),
			want:   nil,
		},
		{
			name:   "unwraps nested error",
			errorz: Wrap(wrappedErr),
			want:   wrappedErr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.errorz.Unwrap()
			if tt.want == nil {
				if got != nil {
					t.Errorf("Error.Unwrap() = %v, want nil", got)
				}
			} else if !errors.Is(got, tt.want) {
				t.Errorf("Error.Unwrap() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestError_Is(t *testing.T) {
	targetErr := errors.New("target error")
	otherErr := errors.New("other error")

	tests := []struct {
		name   string
		errorz *Error
		target error
		want   bool
	}{
		{
			name:   "matches wrapped error",
			errorz: Wrap(targetErr),
			target: targetErr,
			want:   true,
		},
		{
			name:   "does not match different error",
			errorz: Wrap(targetErr),
			target: otherErr,
			want:   false,
		},
		{
			name:   "returns false for non-wrapped error",
			errorz: New("test"),
			target: targetErr,
			want:   false,
		},
		{
			name:   "returns false for nil target",
			errorz: Wrap(targetErr),
			target: nil,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.errorz.Is(tt.target)
			if got != tt.want {
				t.Errorf("Error.Is() = %v, want %v", got, tt.want)
			}
		})
	}
}

//nolint:dupl // Test structure is intentionally similar to other With* method tests
func TestError_WithCode(t *testing.T) {
	tests := []struct {
		name      string
		errorz    *Error
		code      string
		wantCode  string
		wantChain bool
	}{
		{
			name:      "sets error code",
			errorz:    New("test"),
			code:      "ERR001",
			wantCode:  "ERR001",
			wantChain: true,
		},
		{
			name:      "sets empty code",
			errorz:    New("test"),
			code:      "",
			wantCode:  "",
			wantChain: true,
		},
		{
			name:      "overwrites existing code",
			errorz:    New("test").WithCode("OLD"),
			code:      "NEW",
			wantCode:  "NEW",
			wantChain: true,
		},
		{
			name:      "allows method chaining",
			errorz:    New("test"),
			code:      "CHAIN",
			wantCode:  "CHAIN",
			wantChain: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.errorz.WithCode(tt.code)
			if got.Code != tt.wantCode {
				t.Errorf("Error.WithCode().Code = %v, want %v", got.Code, tt.wantCode)
			}
			if tt.wantChain && got != tt.errorz {
				t.Errorf("Error.WithCode() should return same instance for chaining")
			}
		})
	}
}

func TestError_WithMessage(t *testing.T) {
	tests := []struct {
		name        string
		errorz      *Error
		message     string
		wantMessage string
		wantChain   bool
	}{
		{
			name:        "sets error message",
			errorz:      New("original"),
			message:     "updated",
			wantMessage: "updated",
			wantChain:   true,
		},
		{
			name:        "sets empty message",
			errorz:      New("original"),
			message:     "",
			wantMessage: "",
			wantChain:   true,
		},
		{
			name:        "overwrites existing message",
			errorz:      New("old"),
			message:     "new",
			wantMessage: "new",
			wantChain:   true,
		},
		{
			name:        "allows method chaining",
			errorz:      New("test"),
			message:     "chained",
			wantMessage: "chained",
			wantChain:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.errorz.WithMessage(tt.message)
			if got.Message != tt.wantMessage {
				t.Errorf("Error.WithMessage().Message = %v, want %v", got.Message, tt.wantMessage)
			}
			if tt.wantChain && got != tt.errorz {
				t.Errorf("Error.WithMessage() should return same instance for chaining")
			}
		})
	}
}

//nolint:dupl // Test structure is intentionally similar to other With* method tests
func TestError_WithSourceSystem(t *testing.T) {
	tests := []struct {
		name          string
		errorz        *Error
		sourceSystem  string
		wantSourceSys string
		wantChain     bool
	}{
		{
			name:          "sets source system",
			errorz:        New("test"),
			sourceSystem:  "custom-system",
			wantSourceSys: "custom-system",
			wantChain:     true,
		},
		{
			name:          "sets empty source system",
			errorz:        New("test"),
			sourceSystem:  "",
			wantSourceSys: "",
			wantChain:     true,
		},
		{
			name:          "overwrites existing source system",
			errorz:        New("test").WithSourceSystem("old"),
			sourceSystem:  "new",
			wantSourceSys: "new",
			wantChain:     true,
		},
		{
			name:          "allows method chaining",
			errorz:        New("test"),
			sourceSystem:  "chained",
			wantSourceSys: "chained",
			wantChain:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.errorz.WithSourceSystem(tt.sourceSystem)
			if got.SourceSystem != tt.wantSourceSys {
				t.Errorf("Error.WithSourceSystem().SourceSystem = %v, want %v", got.SourceSystem, tt.wantSourceSys)
			}
			if tt.wantChain && got != tt.errorz {
				t.Errorf("Error.WithSourceSystem() should return same instance for chaining")
			}
		})
	}
}

func TestError_WithMeta(t *testing.T) {
	tests := []struct {
		name      string
		errorz    *Error
		key       string
		value     any
		wantMeta  map[string]any
		wantChain bool
	}{
		{
			name:      "adds metadata to empty meta",
			errorz:    New("test"),
			key:       "key1",
			value:     "value1",
			wantMeta:  map[string]any{"key1": "value1"},
			wantChain: true,
		},
		{
			name:      "adds multiple metadata entries",
			errorz:    New("test").WithMeta("key1", "value1"),
			key:       "key2",
			value:     "value2",
			wantMeta:  map[string]any{"key1": "value1", "key2": "value2"},
			wantChain: true,
		},
		{
			name:      "overwrites existing metadata key",
			errorz:    New("test").WithMeta("key1", "old"),
			key:       "key1",
			value:     "new",
			wantMeta:  map[string]any{"key1": "new"},
			wantChain: true,
		},
		{
			name:      "adds integer metadata",
			errorz:    New("test"),
			key:       "count",
			value:     42,
			wantMeta:  map[string]any{"count": 42},
			wantChain: true,
		},
		{
			name:      "adds boolean metadata",
			errorz:    New("test"),
			key:       "enabled",
			value:     true,
			wantMeta:  map[string]any{"enabled": true},
			wantChain: true,
		},
		{
			name:      "adds nil metadata",
			errorz:    New("test"),
			key:       "nilkey",
			value:     nil,
			wantMeta:  map[string]any{"nilkey": nil},
			wantChain: true,
		},
		{
			name:      "allows method chaining",
			errorz:    New("test"),
			key:       "chain",
			value:     "test",
			wantMeta:  map[string]any{"chain": "test"},
			wantChain: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.errorz.WithMeta(tt.key, tt.value)
			if got.Meta == nil {
				t.Errorf("Error.WithMeta().Meta = nil, want %v", tt.wantMeta)
				return
			}
			if len(got.Meta) != len(tt.wantMeta) {
				t.Errorf("Error.WithMeta().Meta length = %v, want %v", len(got.Meta), len(tt.wantMeta))
			}
			for k, v := range tt.wantMeta {
				if got.Meta[k] != v {
					t.Errorf("Error.WithMeta().Meta[%v] = %v, want %v", k, got.Meta[k], v)
				}
			}
			if tt.wantChain && got != tt.errorz {
				t.Errorf("Error.WithMeta() should return same instance for chaining")
			}
		})
	}
}

func TestPredefinedErrors(t *testing.T) {
	tests := []struct {
		name          string
		err           *Error
		wantCode      string
		wantMessage   string
		wantSourceSys string
		sentinel      error
	}{
		{name: "NotFound", err: NotFound(), wantCode: CodeNotFound, wantMessage: "not found", wantSourceSys: DefaultSourceSystem, sentinel: ErrNotFound},
		{name: "BadRequest", err: BadRequest(), wantCode: CodeBadRequest, wantMessage: "bad request", wantSourceSys: DefaultSourceSystem, sentinel: ErrBadRequest},
		{name: "Internal", err: Internal(), wantCode: CodeInternal, wantMessage: "internal server error", wantSourceSys: DefaultSourceSystem, sentinel: ErrInternal},
		{name: "Unauthorized", err: Unauthorized(), wantCode: CodeUnauthorized, wantMessage: "unauthorized", wantSourceSys: DefaultSourceSystem, sentinel: ErrUnauthorized},
		{name: "Forbidden", err: Forbidden(), wantCode: CodeForbidden, wantMessage: "forbidden", wantSourceSys: DefaultSourceSystem, sentinel: ErrForbidden},
		{name: "TooManyRequests", err: TooManyRequests(), wantCode: CodeTooManyRequests, wantMessage: "too many requests", wantSourceSys: DefaultSourceSystem, sentinel: ErrTooManyRequests},
		{name: "BadGateway", err: BadGateway(), wantCode: CodeBadGateway, wantMessage: "bad gateway", wantSourceSys: DefaultSourceSystem, sentinel: ErrBadGateway},
		{name: "ServiceUnavailable", err: ServiceUnavailable(), wantCode: CodeServiceUnavailable, wantMessage: "service unavailable", wantSourceSys: DefaultSourceSystem, sentinel: ErrServiceUnavailable},
		{name: "UnprocessableEntity", err: UnprocessableEntity(), wantCode: CodeUnprocessableEntity, wantMessage: "unprocessable entity", wantSourceSys: DefaultSourceSystem, sentinel: ErrUnprocessableEntity},
		{name: "Conflict", err: Conflict(), wantCode: CodeConflict, wantMessage: "conflict", wantSourceSys: DefaultSourceSystem, sentinel: ErrConflict},
		{name: "PreconditionFailed", err: PreconditionFailed(), wantCode: CodePreconditionFailed, wantMessage: "precondition failed", wantSourceSys: DefaultSourceSystem, sentinel: ErrPreconditionFailed},
		{name: "PreconditionRequired", err: PreconditionRequired(), wantCode: CodePreconditionRequired, wantMessage: "precondition required", wantSourceSys: DefaultSourceSystem, sentinel: ErrPreconditionRequired},
		{name: "PreconditionNotMet", err: PreconditionNotMet(), wantCode: CodePreconditionNotMet, wantMessage: "precondition not met", wantSourceSys: DefaultSourceSystem, sentinel: ErrPreconditionNotMet},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				t.Errorf("%s() = nil, want non-nil", tt.name)
				return
			}
			if tt.err.Code != tt.wantCode {
				t.Errorf("%s().Code = %v, want %v", tt.name, tt.err.Code, tt.wantCode)
			}
			if tt.err.Message != tt.wantMessage {
				t.Errorf("%s().Message = %v, want %v", tt.name, tt.err.Message, tt.wantMessage)
			}
			if tt.err.SourceSystem != tt.wantSourceSys {
				t.Errorf("%s().SourceSystem = %v, want %v", tt.name, tt.err.SourceSystem, tt.wantSourceSys)
			}
			if !errors.Is(tt.err, tt.sentinel) {
				t.Errorf("errors.Is(%s(), sentinel) = false, want true", tt.name)
			}
		})
	}
}

func TestPredefinedErrors_constructorReturnsNewInstance(t *testing.T) {
	err1 := NotFound().WithCode("CUSTOM_001")
	err2 := NotFound()
	if err2.Code != CodeNotFound {
		t.Errorf("second NotFound() has Code = %v, want %v (constructor must return new instance)", err2.Code, CodeNotFound)
	}
	if err1.Code != "CUSTOM_001" {
		t.Errorf("first NotFound().WithCode(...) has Code = %v, want CUSTOM_001", err1.Code)
	}
}

func TestError_MethodChaining(t *testing.T) {
	tests := []struct {
		name          string
		errorz        *Error
		wantMessage   string
		wantCode      string
		wantSourceSys string
		wantMeta      map[string]any
	}{
		{
			name: "chains all methods",
			errorz: New("test").
				WithCode("ERR001").
				WithMessage("chained message").
				WithSourceSystem("custom").
				WithMeta("key1", "value1").
				WithMeta("key2", 42),
			wantMessage:   "chained message",
			wantCode:      "ERR001",
			wantSourceSys: "custom",
			wantMeta:      map[string]any{"key1": "value1", "key2": 42},
		},
		{
			name: "chains with wrap",
			errorz: Wrap(errors.New("inner")).
				WithCode("WRAP001").
				WithMessage("wrapped").
				WithSourceSystem("wrapper").
				WithMeta("wrapped", true),
			wantMessage:   "wrapped",
			wantCode:      "WRAP001",
			wantSourceSys: "wrapper",
			wantMeta:      map[string]any{"wrapped": true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.errorz.Message != tt.wantMessage {
				t.Errorf("chained Message = %v, want %v", tt.errorz.Message, tt.wantMessage)
			}
			if tt.errorz.Code != tt.wantCode {
				t.Errorf("chained Code = %v, want %v", tt.errorz.Code, tt.wantCode)
			}
			if tt.errorz.SourceSystem != tt.wantSourceSys {
				t.Errorf("chained SourceSystem = %v, want %v", tt.errorz.SourceSystem, tt.wantSourceSys)
			}
			if tt.wantMeta != nil {
				if tt.errorz.Meta == nil {
					t.Errorf("chained Meta = nil, want %v", tt.wantMeta)
					return
				}
				for k, v := range tt.wantMeta {
					if tt.errorz.Meta[k] != v {
						t.Errorf("chained Meta[%v] = %v, want %v", k, tt.errorz.Meta[k], v)
					}
				}
			}
		})
	}
}
