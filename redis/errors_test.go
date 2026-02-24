package redis

import (
	"errors"
	"testing"
)

func TestErrorSentinels(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		wantMsg string
	}{
		{
			name:    "ErrKeyNotFound has expected message",
			err:     ErrKeyNotFound,
			wantMsg: "redis: key not found",
		},
		{
			name:    "ErrNilValue has expected message",
			err:     ErrNilValue,
			wantMsg: "redis: nil value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				t.Fatal("test error sentinel must be non-nil")
			}
			if tt.err.Error() != tt.wantMsg {
				t.Errorf("err.Error() = %q, want %q", tt.err.Error(), tt.wantMsg)
			}
		})
	}
}

func TestErrorSentinels_are_usable_with_errors_Is(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		target error
		want   bool
	}{
		{"ErrKeyNotFound is ErrKeyNotFound", ErrKeyNotFound, ErrKeyNotFound, true},
		{"ErrNilValue is ErrNilValue", ErrNilValue, ErrNilValue, true},
		{"ErrKeyNotFound is not ErrNilValue", ErrKeyNotFound, ErrNilValue, false},
		{"ErrNilValue is not ErrKeyNotFound", ErrNilValue, ErrKeyNotFound, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := errors.Is(tt.err, tt.target)
			if got != tt.want {
				t.Errorf("errors.Is(%v, %v) = %v, want %v", tt.err, tt.target, got, tt.want)
			}
		})
	}
}
