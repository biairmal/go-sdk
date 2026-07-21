package auth

import (
	"errors"
	"testing"

	"github.com/biairmal/go-sdk/lib/errorz"
)

func TestUnauthorizedWrapsSentinels(t *testing.T) {
	sentinels := []error{ErrMissingToken, ErrInvalidToken, ErrTokenExpired, ErrTokenInactive, ErrUnknownKeyID}
	for _, sentinel := range sentinels {
		err := unauthorized(sentinel)

		if !errors.Is(err, sentinel) {
			t.Errorf("unauthorized(%v): errors.Is(sentinel) = false, want true", sentinel)
		}
		if !errors.Is(err, errorz.ErrUnauthorized) {
			t.Errorf("unauthorized(%v): errors.Is(errorz.ErrUnauthorized) = false, want true", sentinel)
		}

		var ez *errorz.Error
		if !errors.As(err, &ez) {
			t.Fatalf("unauthorized(%v): errors.As(*errorz.Error) = false", sentinel)
		}
		if ez.Code != errorz.CodeUnauthorized {
			t.Errorf("unauthorized(%v): code = %q, want %q", sentinel, ez.Code, errorz.CodeUnauthorized)
		}
	}
}
