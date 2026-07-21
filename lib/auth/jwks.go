package auth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/biairmal/go-sdk/lib/errorz"
	"github.com/golang-jwt/jwt/v5"
)

const (
	// defaultJWKSTTL is how long a fetched key set is considered fresh.
	defaultJWKSTTL = 15 * time.Minute
	// jwksRefreshCooldown bounds how often an unknown kid triggers a refetch,
	// so a spray of bogus kids cannot hammer the JWKS endpoint.
	jwksRefreshCooldown = 30 * time.Second
	// maxJWKSBytes caps the JWKS response body read.
	maxJWKSBytes = 1 << 20
)

// jwksCache fetches and caches RSA verification keys from a JWKS endpoint,
// keyed by kid. It refreshes on TTL expiry and (rate-limited) on unknown kids,
// and serves the last good key set if a refresh fails.
type jwksCache struct {
	url             string
	client          *http.Client
	ttl             time.Duration
	refreshCooldown time.Duration

	mu        sync.RWMutex
	keys      map[string]*rsa.PublicKey
	fetchedAt time.Time
}

// newJWKSCache builds a cache for the given JWKS URL. A nil client falls back
// to a default client with a request timeout.
func newJWKSCache(url string, client *http.Client) *jwksCache {
	if client == nil {
		client = defaultHTTPClient()
	}
	return &jwksCache{
		url:             url,
		client:          client,
		ttl:             defaultJWKSTTL,
		refreshCooldown: jwksRefreshCooldown,
	}
}

// keyForToken resolves the verification key for a parsed token by its kid header.
func (c *jwksCache) keyForToken(ctx context.Context, t *jwt.Token) (any, error) {
	return c.key(ctx, asString(t.Header["kid"]))
}

// key returns the RSA public key for kid, refreshing the key set when it is
// stale or when kid is unknown (subject to the refresh cooldown).
func (c *jwksCache) key(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	c.mu.RLock()
	if k, ok := c.lookupFresh(kid); ok {
		c.mu.RUnlock()
		return k, nil
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check: another goroutine may have refreshed while we waited on
	// the write lock. This is the single-flight substitute (stdlib only).
	if k, ok := c.lookupFresh(kid); ok {
		return k, nil
	}
	if c.shouldFetchLocked(kid) {
		if err := c.fetchLocked(ctx); err != nil && len(c.keys) == 0 {
			return nil, err
		}
	}
	if k, ok := c.keys[kid]; ok {
		return k, nil
	}
	return nil, ErrUnknownKeyID
}

// lookupFresh returns the key for kid only when the cache is not stale.
func (c *jwksCache) lookupFresh(kid string) (*rsa.PublicKey, bool) {
	if c.staleLocked() {
		return nil, false
	}
	k, ok := c.keys[kid]
	return k, ok
}

// staleLocked reports whether the cached key set is empty or past its TTL.
func (c *jwksCache) staleLocked() bool {
	return c.keys == nil || time.Since(c.fetchedAt) >= c.ttl
}

// shouldFetchLocked decides whether to fetch: always when stale, and for an
// unknown kid only once the refresh cooldown has elapsed.
func (c *jwksCache) shouldFetchLocked(kid string) bool {
	if c.staleLocked() {
		return true
	}
	if _, ok := c.keys[kid]; !ok {
		return time.Since(c.fetchedAt) >= c.refreshCooldown
	}
	return false
}

// fetchLocked replaces the cached key set from the JWKS endpoint. On failure it
// leaves the existing keys untouched so callers can keep serving stale keys.
func (c *jwksCache) fetchLocked(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, http.NoBody)
	if err != nil {
		return errorz.Wrap(err).WithCode(errorz.CodeBadGateway).WithMessage("auth: build jwks request")
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return errorz.Wrap(err).WithCode(errorz.CodeBadGateway).WithMessage("auth: fetch jwks")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return errorz.BadGateway().WithMessage(fmt.Sprintf("auth: jwks endpoint returned %d", resp.StatusCode))
	}
	keys, err := parseJWKS(io.LimitReader(resp.Body, maxJWKSBytes))
	if err != nil {
		return err
	}
	c.keys = keys
	c.fetchedAt = time.Now()
	return nil
}

// jwk is a single RSA JSON Web Key.
type jwk struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// jwkSet is a JSON Web Key Set.
type jwkSet struct {
	Keys []jwk `json:"keys"`
}

// parseJWKS decodes a JWKS document into a kid→key map, skipping non-RSA and
// malformed entries. It errors only when no usable RSA key remains.
func parseJWKS(r io.Reader) (map[string]*rsa.PublicKey, error) {
	var set jwkSet
	if err := json.NewDecoder(r).Decode(&set); err != nil {
		return nil, errorz.Wrap(err).WithCode(errorz.CodeBadGateway).WithMessage("auth: decode jwks")
	}
	keys := make(map[string]*rsa.PublicKey, len(set.Keys))
	for _, k := range set.Keys {
		if k.Kty != "RSA" {
			continue
		}
		pub, err := jwkToRSA(k)
		if err != nil {
			continue
		}
		keys[k.Kid] = pub
	}
	if len(keys) == 0 {
		return nil, errorz.BadGateway().WithMessage("auth: jwks contained no usable RSA keys")
	}
	return keys, nil
}

// jwkToRSA converts a single RSA JWK (base64url n/e) into an *rsa.PublicKey.
func jwkToRSA(k jwk) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		return nil, err
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		return nil, err
	}
	e := new(big.Int).SetBytes(eBytes)
	if !e.IsInt64() || e.Int64() < 2 {
		return nil, fmt.Errorf("auth: invalid jwk exponent")
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(nBytes), E: int(e.Int64())}, nil
}
