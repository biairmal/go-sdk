package ratelimit

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/biairmal/go-sdk/lib/errorz"
	"github.com/biairmal/go-sdk/lib/redis"
)

// slidingWindowScript implements an accurate sliding-window log: it trims
// entries older than the window, counts what's left, and — only if under
// limit — admits the request by recording it. Trim, count, and admit run
// atomically in one round trip, so concurrent callers never race past limit.
//
// KEYS[1] = rate limit key (a Redis sorted set)
// ARGV[1] = now, in unix milliseconds
// ARGV[2] = window length, in milliseconds
// ARGV[3] = limit (max entries allowed within the window)
// ARGV[4] = member id for this request (must be unique per call)
//
// Returns {allowed (1/0), remaining, retry_after_ms}.
const slidingWindowScript = `
local key = KEYS[1]
local now = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local limit = tonumber(ARGV[3])
local member = ARGV[4]

redis.call('ZREMRANGEBYSCORE', key, '-inf', now - window)
local count = redis.call('ZCARD', key)

if count < limit then
    redis.call('ZADD', key, now, member)
    redis.call('PEXPIRE', key, window)
    return {1, limit - count - 1, 0}
end

local oldest = redis.call('ZRANGE', key, 0, 0, 'WITHSCORES')
local retryAfter = window
if oldest[2] ~= nil then
    retryAfter = (tonumber(oldest[2]) + window) - now
    if retryAfter < 0 then
        retryAfter = 0
    end
end
return {0, 0, retryAfter}
`

// redisLimiter implements Limiter with a Redis-backed sliding-window log,
// shared across every process pointed at the same Redis.
type redisLimiter struct {
	client redis.Client
	limit  int
	window time.Duration
}

// NewRedis builds a Limiter backed by Redis: cfg.Burst requests are allowed
// per cfg.Window, counted with a sliding-window log shared across every
// process hitting the same Redis. Use this over NewInMemory when replicas
// behind a load balancer must enforce one combined limit instead of an
// independent one each.
func NewRedis(client redis.Client, cfg Config) Limiter {
	cfg = withDefaults(cfg)
	return &redisLimiter{client: client, limit: cfg.Burst, window: cfg.Window}
}

// Allow reports whether key may proceed, evaluated atomically in Redis.
func (l *redisLimiter) Allow(ctx context.Context, key string) (Result, error) {
	member, err := randomMember()
	if err != nil {
		return Result{}, errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("ratelimit: generate member")
	}

	now := time.Now().UnixMilli()
	windowMs := l.window.Milliseconds()
	res, err := l.client.Eval(ctx, slidingWindowScript, []string{key}, now, windowMs, l.limit, member)
	if err != nil {
		return Result{}, errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("ratelimit: redis eval")
	}

	allowed, remaining, retryAfterMs, err := parseSlidingWindowResult(res)
	if err != nil {
		return Result{}, err
	}
	return Result{
		Allowed:    allowed,
		Limit:      l.limit,
		Remaining:  remaining,
		RetryAfter: time.Duration(retryAfterMs) * time.Millisecond,
	}, nil
}

// randomMember returns a random hex string, unique enough to disambiguate
// same-millisecond requests as distinct sorted-set members.
func randomMember() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// parseSlidingWindowResult decodes the {allowed, remaining, retry_after_ms}
// array returned by slidingWindowScript.
func parseSlidingWindowResult(res interface{}) (allowed bool, remaining int, retryAfterMs int64, err error) {
	arr, ok := res.([]interface{})
	if !ok || len(arr) != 3 {
		return false, 0, 0, errorz.Internal().WithMessage("ratelimit: unexpected redis eval result shape")
	}
	allowedN, ok1 := toInt64(arr[0])
	remainingN, ok2 := toInt64(arr[1])
	retryAfterN, ok3 := toInt64(arr[2])
	if !ok1 || !ok2 || !ok3 {
		return false, 0, 0, errorz.Internal().WithMessage("ratelimit: unexpected redis eval result types")
	}
	return allowedN == 1, int(remainingN), retryAfterN, nil
}

// toInt64 converts a Redis/Lua-returned numeric value to int64.
func toInt64(v interface{}) (int64, bool) {
	switch n := v.(type) {
	case int64:
		return n, true
	case int:
		return int64(n), true
	default:
		return 0, false
	}
}
