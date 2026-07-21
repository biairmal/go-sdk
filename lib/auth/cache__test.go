package auth

import (
	"context"
	"crypto/sha256"
	"testing"
	"time"
)

type countingValidator struct {
	calls  int
	claims Claims
	err    error
}

func (c *countingValidator) Validate(context.Context, string) (Claims, error) {
	c.calls++
	return c.claims, c.err
}

func TestNewCachedDisabled(t *testing.T) {
	inner := &countingValidator{}
	if got := NewCached(inner, 0); got != Validator(inner) {
		t.Error("NewCached with ttl<=0 should return the inner validator unchanged")
	}
}

func TestCacheHit(t *testing.T) {
	inner := &countingValidator{claims: NewClaims(map[string]any{"sub": "u"})}
	v := NewCached(inner, time.Minute)

	for i := 0; i < 3; i++ {
		if _, err := v.Validate(context.Background(), "tok"); err != nil {
			t.Fatalf("Validate #%d: %v", i, err)
		}
	}
	if inner.calls != 1 {
		t.Errorf("inner called %d times, want 1 (result should be cached)", inner.calls)
	}
}

func TestCacheDistinctTokens(t *testing.T) {
	inner := &countingValidator{claims: NewClaims(map[string]any{"sub": "u"})}
	v := NewCached(inner, time.Minute)

	_, _ = v.Validate(context.Background(), "tok-a")
	_, _ = v.Validate(context.Background(), "tok-b")
	if inner.calls != 2 {
		t.Errorf("inner called %d times, want 2 (distinct tokens)", inner.calls)
	}
}

func TestCacheErrorsNotCached(t *testing.T) {
	inner := &countingValidator{err: unauthorized(ErrInvalidToken)}
	v := NewCached(inner, time.Minute)

	_, _ = v.Validate(context.Background(), "tok")
	_, _ = v.Validate(context.Background(), "tok")
	if inner.calls != 2 {
		t.Errorf("inner called %d times, want 2 (errors must not be cached)", inner.calls)
	}
}

func TestCacheExpiry(t *testing.T) {
	inner := &countingValidator{claims: NewClaims(map[string]any{"sub": "u"})}
	v := NewCached(inner, time.Minute).(*cachedValidator)

	if _, err := v.Validate(context.Background(), "tok"); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	// Force the stored entry to be expired, then revalidate.
	key := sha256.Sum256([]byte("tok"))
	v.mu.Lock()
	v.entries[key] = cacheEntry{claims: inner.claims, expiresAt: time.Now().Add(-time.Second)}
	v.mu.Unlock()

	if _, err := v.Validate(context.Background(), "tok"); err != nil {
		t.Fatalf("Validate after expiry: %v", err)
	}
	if inner.calls != 2 {
		t.Errorf("inner called %d times, want 2 (expired entry must revalidate)", inner.calls)
	}
}

func TestCacheEntryCappedByTokenExp(t *testing.T) {
	// Token exp is sooner than the cache ttl, so the entry must expire at exp.
	exp := time.Now().Add(500 * time.Millisecond)
	inner := &countingValidator{claims: NewClaims(map[string]any{"sub": "u", "exp": float64(exp.Unix())})}
	v := NewCached(inner, time.Hour).(*cachedValidator)

	if _, err := v.Validate(context.Background(), "tok"); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	key := sha256.Sum256([]byte("tok"))
	v.mu.RLock()
	entry := v.entries[key]
	v.mu.RUnlock()

	if entry.expiresAt.After(exp.Add(time.Second)) {
		t.Errorf("entry expiry %v exceeds token exp %v; must be capped by exp", entry.expiresAt, exp)
	}
}

func TestClaimsExpiry(t *testing.T) {
	ts := time.Unix(1_700_000_000, 0)
	tests := []struct {
		name string
		exp  any
		ok   bool
	}{
		{"float64", float64(ts.Unix()), true},
		{"int64", ts.Unix(), true},
		{"missing", nil, false},
		{"non-numeric", "soon", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := map[string]any{"sub": "u"}
			if tt.exp != nil {
				raw["exp"] = tt.exp
			}
			got, ok := claimsExpiry(NewClaims(raw))
			if ok != tt.ok {
				t.Fatalf("claimsExpiry ok = %v, want %v", ok, tt.ok)
			}
			if ok && !got.Equal(ts) {
				t.Errorf("claimsExpiry = %v, want %v", got, ts)
			}
		})
	}
}
