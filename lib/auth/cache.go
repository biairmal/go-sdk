package auth

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"sync"
	"time"
)

// maxCacheEntries bounds the validation cache; on reaching it, expired entries
// are swept before inserting a new one.
const maxCacheEntries = 10000

// cachedValidator wraps a Validator with a TTL cache of successful validations,
// keyed by a hash of the token so raw tokens are never stored as map keys.
// Only successes are cached; errors are always revalidated.
type cachedValidator struct {
	inner Validator
	ttl   time.Duration

	mu      sync.RWMutex
	entries map[[32]byte]cacheEntry
}

// cacheEntry is a cached validation result and its expiry.
type cacheEntry struct {
	claims    Claims
	expiresAt time.Time
}

// NewCached wraps v in a TTL cache of successful validations. A non-positive
// ttl returns v unchanged. Cached entries never outlive the token's own exp.
//
// Trade-off: a cached token is accepted until the entry expires even if it was
// revoked upstream, so keep ttl short relative to token lifetime.
func NewCached(v Validator, ttl time.Duration) Validator {
	if ttl <= 0 {
		return v
	}
	return &cachedValidator{
		inner:   v,
		ttl:     ttl,
		entries: make(map[[32]byte]cacheEntry),
	}
}

// Validate returns a cached result when present and unexpired, otherwise
// delegates to the inner validator and caches a successful result.
func (c *cachedValidator) Validate(ctx context.Context, token string) (Claims, error) {
	key := sha256.Sum256([]byte(token))
	now := time.Now()

	c.mu.RLock()
	entry, ok := c.entries[key]
	c.mu.RUnlock()
	if ok && now.Before(entry.expiresAt) {
		return entry.claims, nil
	}

	claims, err := c.inner.Validate(ctx, token)
	if err != nil {
		return nil, err
	}
	c.store(key, claims, now)
	return claims, nil
}

// store records claims under key, capping the entry's expiry at the token's exp.
func (c *cachedValidator) store(key [32]byte, claims Claims, now time.Time) {
	expiresAt := now.Add(c.ttl)
	if exp, ok := claimsExpiry(claims); ok && exp.Before(expiresAt) {
		expiresAt = exp
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.entries) >= maxCacheEntries {
		c.sweepLocked(now)
	}
	c.entries[key] = cacheEntry{claims: claims, expiresAt: expiresAt}
}

// sweepLocked removes expired entries.
func (c *cachedValidator) sweepLocked(now time.Time) {
	for k, e := range c.entries {
		if !now.Before(e.expiresAt) {
			delete(c.entries, k)
		}
	}
}

// claimsExpiry extracts the token's exp claim as a time, handling the numeric
// forms produced by JSON decoding.
func claimsExpiry(claims Claims) (time.Time, bool) {
	v, ok := claims.Get("exp")
	if !ok {
		return time.Time{}, false
	}
	switch n := v.(type) {
	case float64:
		return time.Unix(int64(n), 0), true
	case int64:
		return time.Unix(n, 0), true
	case int:
		return time.Unix(int64(n), 0), true
	case json.Number:
		if i, err := n.Int64(); err == nil {
			return time.Unix(i, 0), true
		}
	}
	return time.Time{}, false
}
