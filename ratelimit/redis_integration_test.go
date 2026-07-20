//go:build integration

package ratelimit

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/biairmal/go-sdk/redis"
)

// Integration test for the redis backend's Lua sliding-window script.
// Requires a running Redis instance (e.g. default localhost:6379).
// Run with: go test -tags=integration ./ratelimit/...

func TestIntegration_RedisLimiter_SlidingWindow(t *testing.T) {
	client, err := redis.NewClient(redis.DefaultConfig())
	if err != nil {
		t.Fatalf("Redis not available (required for integration test): %v", err)
	}
	defer func() { _ = client.Close() }()

	key := fmt.Sprintf("ratelimit-integration-test-%d", time.Now().UnixNano())
	l := NewRedis(client, Config{Burst: 3, Window: 200 * time.Millisecond})
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		res, err := l.Allow(ctx, key)
		if err != nil {
			t.Fatalf("Allow() call %d error = %v, want nil", i, err)
		}
		if !res.Allowed {
			t.Fatalf("Allow() call %d = denied, want allowed", i)
		}
	}

	res, err := l.Allow(ctx, key)
	if err != nil {
		t.Fatalf("Allow() error = %v, want nil", err)
	}
	if res.Allowed {
		t.Fatal("Allow() after burst exhausted = allowed, want denied")
	}
	if res.RetryAfter <= 0 {
		t.Errorf("RetryAfter = %v, want > 0", res.RetryAfter)
	}

	time.Sleep(250 * time.Millisecond)

	res, err = l.Allow(ctx, key)
	if err != nil {
		t.Fatalf("Allow() after window elapsed error = %v, want nil", err)
	}
	if !res.Allowed {
		t.Fatal("Allow() after window elapsed = denied, want allowed")
	}
}
