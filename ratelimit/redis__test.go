package ratelimit

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/biairmal/go-sdk/redis"
)

// fakeRedisClient is a minimal redis.Client test double. redisLimiter only
// calls Eval, so every other method is an unused stub satisfying the
// interface.
type fakeRedisClient struct {
	evalFunc func(ctx context.Context, script string, keys []string, args ...interface{}) (interface{}, error)
	lastKeys []string
	lastArgs []interface{}
}

var _ redis.Client = (*fakeRedisClient)(nil)

func (f *fakeRedisClient) Eval(
	ctx context.Context, script string, keys []string, args ...interface{},
) (interface{}, error) {
	f.lastKeys = keys
	f.lastArgs = args
	return f.evalFunc(ctx, script, keys, args...)
}

func (f *fakeRedisClient) Get(context.Context, string) (string, error) { return "", nil }
func (f *fakeRedisClient) Set(context.Context, string, interface{}, time.Duration) error {
	return nil
}
func (f *fakeRedisClient) Del(context.Context, ...string) error             { return nil }
func (f *fakeRedisClient) Exists(context.Context, ...string) (int64, error) { return 0, nil }
func (f *fakeRedisClient) HGet(context.Context, string, string) (string, error) {
	return "", nil
}
func (f *fakeRedisClient) HSet(context.Context, string, ...interface{}) error { return nil }
func (f *fakeRedisClient) HGetAll(context.Context, string) (map[string]string, error) {
	return nil, nil
}
func (f *fakeRedisClient) HDel(context.Context, string, ...string) error       { return nil }
func (f *fakeRedisClient) SAdd(context.Context, string, ...interface{}) error  { return nil }
func (f *fakeRedisClient) SMembers(context.Context, string) ([]string, error)  { return nil, nil }
func (f *fakeRedisClient) SRem(context.Context, string, ...interface{}) error  { return nil }
func (f *fakeRedisClient) LPush(context.Context, string, ...interface{}) error { return nil }
func (f *fakeRedisClient) RPush(context.Context, string, ...interface{}) error { return nil }
func (f *fakeRedisClient) LPop(context.Context, string) (string, error)        { return "", nil }
func (f *fakeRedisClient) RPop(context.Context, string) (string, error)        { return "", nil }
func (f *fakeRedisClient) LRange(context.Context, string, int64, int64) ([]string, error) {
	return nil, nil
}
func (f *fakeRedisClient) Expire(context.Context, string, time.Duration) error { return nil }
func (f *fakeRedisClient) TTL(context.Context, string) (time.Duration, error)  { return 0, nil }
func (f *fakeRedisClient) Incr(context.Context, string) (int64, error)         { return 0, nil }
func (f *fakeRedisClient) Decr(context.Context, string) (int64, error)         { return 0, nil }
func (f *fakeRedisClient) IncrBy(context.Context, string, int64) (int64, error) {
	return 0, nil
}
func (f *fakeRedisClient) Pipeline() redis.Pipeliner   { return nil }
func (f *fakeRedisClient) TxPipeline() redis.Pipeliner { return nil }
func (f *fakeRedisClient) Ping(context.Context) error  { return nil }
func (f *fakeRedisClient) Close() error                { return nil }

func TestRedisLimiter_Allow(t *testing.T) {
	t.Run("allowed", func(t *testing.T) {
		client := &fakeRedisClient{
			evalFunc: func(context.Context, string, []string, ...interface{}) (interface{}, error) {
				return []interface{}{int64(1), int64(4), int64(0)}, nil
			},
		}
		l := NewRedis(client, Config{Burst: 5, Window: time.Second})

		res, err := l.Allow(context.Background(), "k")
		if err != nil {
			t.Fatalf("Allow() error = %v, want nil", err)
		}
		if !res.Allowed || res.Limit != 5 || res.Remaining != 4 || res.RetryAfter != 0 {
			t.Errorf("Allow() = %+v, want {Allowed:true Limit:5 Remaining:4 RetryAfter:0}", res)
		}
		if client.lastKeys[0] != "k" {
			t.Errorf("Eval keys = %v, want [k]", client.lastKeys)
		}
	})

	t.Run("denied", func(t *testing.T) {
		client := &fakeRedisClient{
			evalFunc: func(context.Context, string, []string, ...interface{}) (interface{}, error) {
				return []interface{}{int64(0), int64(0), int64(250)}, nil
			},
		}
		l := NewRedis(client, Config{Burst: 5, Window: time.Second})

		res, err := l.Allow(context.Background(), "k")
		if err != nil {
			t.Fatalf("Allow() error = %v, want nil", err)
		}
		if res.Allowed || res.RetryAfter != 250*time.Millisecond {
			t.Errorf("Allow() = %+v, want denied with RetryAfter=250ms", res)
		}
	})

	t.Run("eval error is wrapped", func(t *testing.T) {
		client := &fakeRedisClient{
			evalFunc: func(context.Context, string, []string, ...interface{}) (interface{}, error) {
				return nil, errors.New("connection refused")
			},
		}
		l := NewRedis(client, Config{Burst: 5, Window: time.Second})

		if _, err := l.Allow(context.Background(), "k"); err == nil {
			t.Fatal("Allow() error = nil, want non-nil when the backend errors")
		}
	})

	t.Run("malformed result shape errors", func(t *testing.T) {
		client := &fakeRedisClient{
			evalFunc: func(context.Context, string, []string, ...interface{}) (interface{}, error) {
				return "not-an-array", nil
			},
		}
		l := NewRedis(client, Config{Burst: 5, Window: time.Second})

		if _, err := l.Allow(context.Background(), "k"); err == nil {
			t.Fatal("Allow() error = nil, want non-nil for a malformed eval result")
		}
	})
}
