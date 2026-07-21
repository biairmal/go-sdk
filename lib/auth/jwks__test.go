package auth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func jwkEntry(kid string, pub *rsa.PublicKey) string {
	n := base64.RawURLEncoding.EncodeToString(pub.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes())
	return fmt.Sprintf(`{"kty":"RSA","kid":%q,"n":%q,"e":%q}`, kid, n, e)
}

func jwksServer(t *testing.T, count *int32, entries ...string) *httptest.Server {
	t.Helper()
	body := `{"keys":[`
	for i, e := range entries {
		if i > 0 {
			body += ","
		}
		body += e
	}
	body += `]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if count != nil {
			atomic.AddInt32(count, 1)
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func signWithKID(t *testing.T, key *rsa.PrivateKey, kid string) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub": "jwks-user", "exp": time.Now().Add(time.Hour).Unix(),
	})
	tok.Header["kid"] = kid
	s, err := tok.SignedString(key)
	if err != nil {
		t.Fatalf("SignedString: %v", err)
	}
	return s
}

func TestJWKSValidateAndCache(t *testing.T) {
	key := genRSAKey(t)
	var count int32
	srv := jwksServer(t, &count, jwkEntry("key-1", &key.PublicKey))

	v, err := NewRS256(WithJWKSURL(srv.URL))
	if err != nil {
		t.Fatalf("NewRS256: %v", err)
	}
	token := signWithKID(t, key, "key-1")

	for i := 0; i < 3; i++ {
		claims, err := v.Validate(context.Background(), token)
		if err != nil {
			t.Fatalf("Validate #%d: %v", i, err)
		}
		if claims.Subject() != "jwks-user" {
			t.Errorf("Subject() = %q, want jwks-user", claims.Subject())
		}
	}
	if got := atomic.LoadInt32(&count); got != 1 {
		t.Errorf("jwks fetched %d times, want 1 (subsequent validations should hit cache)", got)
	}
}

func TestJWKSUnknownKID(t *testing.T) {
	key := genRSAKey(t)
	srv := jwksServer(t, nil, jwkEntry("key-1", &key.PublicKey))
	v, _ := NewRS256(WithJWKSURL(srv.URL))

	token := signWithKID(t, key, "missing-kid")
	_, err := v.Validate(context.Background(), token)
	if !errors.Is(err, ErrUnknownKeyID) {
		t.Errorf("Validate error = %v, want errors.Is ErrUnknownKeyID", err)
	}
}

func TestJWKSMalformedEntrySkipped(t *testing.T) {
	key := genRSAKey(t)
	bad := `{"kty":"RSA","kid":"bad","n":"!!!invalid","e":"AQAB"}`
	srv := jwksServer(t, nil, bad, jwkEntry("good", &key.PublicKey))

	v, _ := NewRS256(WithJWKSURL(srv.URL))
	token := signWithKID(t, key, "good")
	if _, err := v.Validate(context.Background(), token); err != nil {
		t.Errorf("expected good key to validate despite malformed sibling, got %v", err)
	}
}

func TestJWKSUnknownKIDRefetchAfterCooldown(t *testing.T) {
	key := genRSAKey(t)
	var count int32
	srv := jwksServer(t, &count, jwkEntry("key-1", &key.PublicKey))

	cache := newJWKSCache(srv.URL, nil)
	cache.refreshCooldown = 0 // allow immediate refetch on unknown kid

	if _, err := cache.key(context.Background(), "key-1"); err != nil {
		t.Fatalf("first key(): %v", err)
	}
	// Unknown kid with zero cooldown forces a second fetch.
	_, _ = cache.key(context.Background(), "unknown")
	if got := atomic.LoadInt32(&count); got != 2 {
		t.Errorf("jwks fetched %d times, want 2 (unknown kid past cooldown refetches)", got)
	}
}

func TestJWKSServesStaleOnRefreshFailure(t *testing.T) {
	key := genRSAKey(t)
	srv := jwksServer(t, nil, jwkEntry("key-1", &key.PublicKey))
	cache := newJWKSCache(srv.URL, nil)

	if _, err := cache.key(context.Background(), "key-1"); err != nil {
		t.Fatalf("prime cache: %v", err)
	}
	// Force staleness and break the endpoint.
	cache.ttl = 0
	srv.Close()

	if _, err := cache.key(context.Background(), "key-1"); err != nil {
		t.Errorf("expected stale key to be served on refresh failure, got %v", err)
	}
}

func TestJWKSConcurrentSingleFetch(t *testing.T) {
	key := genRSAKey(t)
	var count int32
	srv := jwksServer(t, &count, jwkEntry("key-1", &key.PublicKey))
	cache := newJWKSCache(srv.URL, nil)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = cache.key(context.Background(), "key-1")
		}()
	}
	wg.Wait()

	if got := atomic.LoadInt32(&count); got != 1 {
		t.Errorf("jwks fetched %d times under concurrency, want 1", got)
	}
}
