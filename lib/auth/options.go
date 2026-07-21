package auth

import (
	"crypto/rsa"
	"net/http"
	"time"
)

// settings holds the non-serializable knobs shared by the local (HS256/RS256)
// and remote validator constructors. Serializable equivalents live in Config
// and are mapped onto these by FromConfig.
type settings struct {
	// expectedIssuer, when set, requires the token's "iss" claim to match.
	expectedIssuer string
	// expectedAudience, when set, requires the token's "aud" claim to contain it.
	expectedAudience string
	// leeway is the clock-skew tolerance applied to time-based claims.
	leeway time.Duration
	// publicKey is the static RS256 verification key (RS256 static-key mode).
	publicKey *rsa.PublicKey
	// jwksURL is the JWKS endpoint used to fetch RS256 keys by kid.
	jwksURL string
	// httpClient is used for JWKS and remote-validator requests.
	httpClient *http.Client
}

// Option configures a Validator constructor with optional, non-serializable
// behavior. Every Option is nil/zero-safe: a blank or nil argument is ignored.
type Option func(*settings)

// WithExpectedIssuer requires the token's "iss" claim to equal iss.
func WithExpectedIssuer(iss string) Option {
	return func(s *settings) {
		if iss != "" {
			s.expectedIssuer = iss
		}
	}
}

// WithExpectedAudience requires the token's "aud" claim to contain aud.
func WithExpectedAudience(aud string) Option {
	return func(s *settings) {
		if aud != "" {
			s.expectedAudience = aud
		}
	}
}

// WithLeeway sets the clock-skew tolerance for time-based claim checks
// (exp/nbf/iat). A negative duration is ignored; the default is 0.
func WithLeeway(d time.Duration) Option {
	return func(s *settings) {
		if d > 0 {
			s.leeway = d
		}
	}
}

// WithPublicKey sets the static RS256 verification key. Mutually exclusive with
// WithJWKSURL. A nil key is ignored.
func WithPublicKey(pub *rsa.PublicKey) Option {
	return func(s *settings) {
		if pub != nil {
			s.publicKey = pub
		}
	}
}

// WithJWKSURL configures RS256 verification to fetch keys by kid from a JWKS
// endpoint. Mutually exclusive with WithPublicKey. A blank URL is ignored.
func WithJWKSURL(url string) Option {
	return func(s *settings) {
		if url != "" {
			s.jwksURL = url
		}
	}
}

// WithHTTPClient sets the HTTP client used for JWKS and remote-validator
// requests. A nil client is ignored (a default client is used instead).
func WithHTTPClient(c *http.Client) Option {
	return func(s *settings) {
		if c != nil {
			s.httpClient = c
		}
	}
}

// applyOptions folds opts onto a zero settings value, skipping nil options.
func applyOptions(opts ...Option) settings {
	var s settings
	for _, opt := range opts {
		if opt != nil {
			opt(&s)
		}
	}
	return s
}
