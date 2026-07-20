package ratelimit

import (
	"context"
	"testing"
	"time"
)

func TestMemoryLimiter_AllowsUpToBurstThenDenies(t *testing.T) {
	l := NewInMemory(Config{Backend: BackendMemory, Rate: 1, Burst: 3, MaxKeys: 100})
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		res, err := l.Allow(ctx, "k")
		if err != nil {
			t.Fatalf("Allow() error = %v, want nil", err)
		}
		if !res.Allowed {
			t.Fatalf("Allow() call %d = denied, want allowed", i)
		}
		if res.Limit != 3 {
			t.Errorf("Limit = %d, want 3", res.Limit)
		}
	}

	res, err := l.Allow(ctx, "k")
	if err != nil {
		t.Fatalf("Allow() error = %v, want nil", err)
	}
	if res.Allowed {
		t.Fatal("Allow() after burst exhausted = allowed, want denied")
	}
	if res.RetryAfter <= 0 {
		t.Errorf("RetryAfter = %v, want > 0", res.RetryAfter)
	}
}

func TestMemoryLimiter_KeysAreIndependent(t *testing.T) {
	l := NewInMemory(Config{Backend: BackendMemory, Rate: 1, Burst: 1, MaxKeys: 100})
	ctx := context.Background()

	if res, err := l.Allow(ctx, "a"); err != nil || !res.Allowed {
		t.Fatalf("Allow(a) = %+v, %v, want allowed", res, err)
	}
	if res, err := l.Allow(ctx, "a"); err != nil || res.Allowed {
		t.Fatalf("Allow(a) second call = %+v, %v, want denied", res, err)
	}
	if res, err := l.Allow(ctx, "b"); err != nil || !res.Allowed {
		t.Fatalf("Allow(b) = %+v, %v, want allowed (independent bucket)", res, err)
	}
}

func TestMemoryLimiter_EvictsIdleKeys(t *testing.T) {
	l := NewInMemory(Config{Backend: BackendMemory, Rate: 1, Burst: 1, MaxKeys: 100}).(*memoryLimiter)
	ctx := context.Background()

	if _, err := l.Allow(ctx, "idle"); err != nil {
		t.Fatalf("Allow() error = %v, want nil", err)
	}

	l.mu.Lock()
	l.buckets["idle"].lastSeen = time.Now().Add(-idleTTL - time.Minute)
	l.evictIdleLocked(time.Now())
	_, stillPresent := l.buckets["idle"]
	l.mu.Unlock()

	if stillPresent {
		t.Error("evictIdleLocked() did not remove an idle bucket")
	}
}

func TestMemoryLimiter_EvictsAtMaxKeysCap(t *testing.T) {
	l := NewInMemory(Config{Backend: BackendMemory, Rate: 10, Burst: 10, MaxKeys: 2}).(*memoryLimiter)
	ctx := context.Background()

	if _, err := l.Allow(ctx, "k1"); err != nil {
		t.Fatalf("Allow(k1) error = %v", err)
	}
	if _, err := l.Allow(ctx, "k2"); err != nil {
		t.Fatalf("Allow(k2) error = %v", err)
	}

	l.mu.Lock()
	l.buckets["k1"].lastSeen = time.Now().Add(-idleTTL - time.Minute)
	l.mu.Unlock()

	if _, err := l.Allow(ctx, "k3"); err != nil {
		t.Fatalf("Allow(k3) error = %v", err)
	}

	l.mu.Lock()
	_, k1Present := l.buckets["k1"]
	_, k3Present := l.buckets["k3"]
	l.mu.Unlock()

	if k1Present {
		t.Error("expected idle k1 to be evicted once MaxKeys was reached")
	}
	if !k3Present {
		t.Error("expected k3 to be tracked after eviction made room")
	}
}
