package ratelimit

import (
	"context"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// idleTTL is how long an in-memory bucket may go unused before it's eligible
// for eviction, bounding memory under a churning key space (e.g. per-IP).
const idleTTL = 10 * time.Minute

// sweepEvery triggers an opportunistic idle-eviction sweep every N Allow
// calls, so memory stays bounded without a background goroutine to manage.
const sweepEvery = 128

// bucketEntry is one key's token bucket plus its last-access time, used for
// idle eviction.
type bucketEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// memoryLimiter implements Limiter with a per-key token bucket
// (golang.org/x/time/rate), scoped to this process only.
type memoryLimiter struct {
	rate    rate.Limit
	burst   int
	maxKeys int

	mu      sync.Mutex
	buckets map[string]*bucketEntry
	calls   uint64
}

// NewInMemory builds a Limiter backed by an in-process token bucket per key.
// Suitable for a single monolith instance: the limit is per-process, not
// shared across replicas behind a load balancer. Idle keys are evicted
// opportunistically (every sweepEvery calls, and when MaxKeys is reached) so
// memory stays bounded.
func NewInMemory(cfg Config) Limiter {
	cfg = withDefaults(cfg)
	return &memoryLimiter{
		rate:    rate.Limit(cfg.Rate),
		burst:   cfg.Burst,
		maxKeys: cfg.MaxKeys,
		buckets: make(map[string]*bucketEntry),
	}
}

// Allow reports whether key may proceed, consuming a token if so.
func (l *memoryLimiter) Allow(_ context.Context, key string) (Result, error) {
	entry := l.entryFor(key)

	now := time.Now()
	reservation := entry.limiter.ReserveN(now, 1)
	if !reservation.OK() {
		// Burst is smaller than the requested 1 token; never satisfiable.
		return Result{Allowed: false, Limit: l.burst}, nil
	}
	if delay := reservation.DelayFrom(now); delay > 0 {
		reservation.Cancel()
		return Result{Allowed: false, Limit: l.burst, RetryAfter: delay}, nil
	}
	return Result{Allowed: true, Limit: l.burst, Remaining: remainingTokens(entry.limiter, l.burst)}, nil
}

// entryFor returns the bucket for key, creating it on first use, and
// refreshes its last-seen time. It also performs opportunistic idle
// eviction so the map doesn't grow unbounded.
func (l *memoryLimiter) entryFor(key string) *bucketEntry {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	l.calls++
	if l.calls%sweepEvery == 0 {
		l.evictIdleLocked(now)
	}

	entry, ok := l.buckets[key]
	if !ok {
		if l.maxKeys > 0 && len(l.buckets) >= l.maxKeys {
			l.evictIdleLocked(now)
		}
		entry = &bucketEntry{limiter: rate.NewLimiter(l.rate, l.burst)}
		l.buckets[key] = entry
	}
	entry.lastSeen = now
	return entry
}

// evictIdleLocked removes buckets unused for longer than idleTTL. Caller
// must hold l.mu.
func (l *memoryLimiter) evictIdleLocked(now time.Time) {
	for k, e := range l.buckets {
		if now.Sub(e.lastSeen) > idleTTL {
			delete(l.buckets, k)
		}
	}
}

// remainingTokens returns lim's currently available tokens, clamped to
// [0, burst].
func remainingTokens(lim *rate.Limiter, burst int) int {
	tokens := int(lim.Tokens())
	if tokens < 0 {
		return 0
	}
	if tokens > burst {
		return burst
	}
	return tokens
}
