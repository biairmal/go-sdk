package auth

import (
	"crypto/rsa"
	"time"

	"github.com/biairmal/go-sdk/lib/errorz"
	"github.com/golang-jwt/jwt/v5"
)

// defaultIssuerTTL is the token lifetime used when none is configured.
const defaultIssuerTTL = 15 * time.Minute

// Issuer signs tokens for a subject. It is used monolith-side to mint tokens
// that the same process (or a peer service) later validates.
type Issuer interface {
	// Issue signs a token for subject, setting the registered claims
	// (sub/iss/aud/iat/exp) plus any caller claims from opts, and returns the
	// signed token string.
	Issue(subject string, opts ...IssueOption) (string, error)
}

// issuerSettings holds an Issuer's construction-time defaults.
type issuerSettings struct {
	name       string
	defaultTTL time.Duration
	audience   string
}

// IssuerOption configures an Issuer at construction time.
type IssuerOption func(*issuerSettings)

// WithIssuerName sets the "iss" claim written on every issued token.
func WithIssuerName(iss string) IssuerOption {
	return func(s *issuerSettings) {
		if iss != "" {
			s.name = iss
		}
	}
}

// WithDefaultTTL sets the default token lifetime. A non-positive duration is
// ignored; the package default is 15 minutes.
func WithDefaultTTL(d time.Duration) IssuerOption {
	return func(s *issuerSettings) {
		if d > 0 {
			s.defaultTTL = d
		}
	}
}

// WithDefaultAudience sets the default "aud" claim written on issued tokens.
func WithDefaultAudience(aud string) IssuerOption {
	return func(s *issuerSettings) {
		if aud != "" {
			s.audience = aud
		}
	}
}

// issueSettings holds the per-call overrides for a single Issue.
type issueSettings struct {
	ttl         time.Duration
	audience    string
	roles       []string
	issuedAt    time.Time
	issuedAtSet bool
	extra       map[string]any
}

// IssueOption overrides issuer defaults for a single Issue call.
type IssueOption func(*issueSettings)

// WithTTL overrides the token lifetime for this token.
func WithTTL(d time.Duration) IssueOption {
	return func(s *issueSettings) {
		if d > 0 {
			s.ttl = d
		}
	}
}

// WithAudience overrides the "aud" claim for this token.
func WithAudience(aud string) IssueOption {
	return func(s *issueSettings) {
		if aud != "" {
			s.audience = aud
		}
	}
}

// WithRoles sets the "roles" claim for this token.
func WithRoles(roles ...string) IssueOption {
	return func(s *issueSettings) {
		if len(roles) > 0 {
			s.roles = roles
		}
	}
}

// WithIssuedAt pins the "iat" claim (and thus the "exp" basis) to t, making
// token contents deterministic. Primarily for tests.
func WithIssuedAt(t time.Time) IssueOption {
	return func(s *issueSettings) {
		s.issuedAt = t
		s.issuedAtSet = true
	}
}

// WithExtraClaims adds arbitrary caller claims. Registered claims
// (sub/iss/aud/iat/exp) always win over extras and cannot be overridden here.
func WithExtraClaims(extra map[string]any) IssueOption {
	return func(s *issueSettings) {
		if len(extra) > 0 {
			s.extra = extra
		}
	}
}

// applyIssuerOptions folds construction-time opts onto a settings value seeded
// with the package default TTL.
func applyIssuerOptions(opts ...IssuerOption) issuerSettings {
	s := issuerSettings{defaultTTL: defaultIssuerTTL}
	for _, opt := range opts {
		if opt != nil {
			opt(&s)
		}
	}
	return s
}

// applyIssueOptions folds per-call opts onto a zero settings value.
func applyIssueOptions(opts ...IssueOption) issueSettings {
	var s issueSettings
	for _, opt := range opts {
		if opt != nil {
			opt(&s)
		}
	}
	return s
}

// jwtIssuer signs tokens with a fixed method and key.
type jwtIssuer struct {
	method   jwt.SigningMethod
	key      any // []byte for HS256, *rsa.PrivateKey for RS256
	settings issuerSettings
}

// NewHS256Issuer returns an Issuer that signs tokens with HS256 using secret.
func NewHS256Issuer(secret string, opts ...IssuerOption) (Issuer, error) {
	if secret == "" {
		return nil, errorz.BadRequest().WithMessage("auth: hs256 issuer secret is required")
	}
	return &jwtIssuer{
		method:   jwt.SigningMethodHS256,
		key:      []byte(secret),
		settings: applyIssuerOptions(opts...),
	}, nil
}

// NewRS256Issuer returns an Issuer that signs tokens with RS256 using key.
func NewRS256Issuer(key *rsa.PrivateKey, opts ...IssuerOption) (Issuer, error) {
	if key == nil {
		return nil, errorz.BadRequest().WithMessage("auth: rs256 issuer private key is required")
	}
	return &jwtIssuer{
		method:   jwt.SigningMethodRS256,
		key:      key,
		settings: applyIssuerOptions(opts...),
	}, nil
}

// Issue signs a token for subject. Registered claims (sub/iss/aud/iat/exp) are
// written last and always win over any WithExtraClaims values.
func (i *jwtIssuer) Issue(subject string, opts ...IssueOption) (string, error) {
	if subject == "" {
		return "", errorz.BadRequest().WithMessage("auth: issue subject is required")
	}
	cfg := applyIssueOptions(opts...)

	now := time.Now()
	if cfg.issuedAtSet {
		now = cfg.issuedAt
	}
	ttl := i.settings.defaultTTL
	if cfg.ttl > 0 {
		ttl = cfg.ttl
	}
	audience := i.settings.audience
	if cfg.audience != "" {
		audience = cfg.audience
	}

	claims := jwt.MapClaims{}
	for k, v := range cfg.extra {
		claims[k] = v
	}
	claims["sub"] = subject
	claims["iat"] = now.Unix()
	claims["exp"] = now.Add(ttl).Unix()
	if i.settings.name != "" {
		claims["iss"] = i.settings.name
	}
	if audience != "" {
		claims["aud"] = audience
	}
	if len(cfg.roles) > 0 {
		claims["roles"] = cfg.roles
	}

	signed, err := jwt.NewWithClaims(i.method, claims).SignedString(i.key)
	if err != nil {
		return "", errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("auth: sign token")
	}
	return signed, nil
}
