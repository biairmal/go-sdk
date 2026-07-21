package auth

import (
	"os"
	"time"

	"github.com/biairmal/go-sdk/lib/errorz"
)

// Validator modes.
const (
	ModeLocal  = "local"
	ModeRemote = "remote"
	ModeFunc   = "func"
)

// Signing algorithms.
const (
	AlgorithmHS256 = "HS256"
	AlgorithmRS256 = "RS256"
)

// Config is the YAML-able configuration for the auth package. It selects a
// validation mode and carries the settings for whichever backend that mode
// uses, plus the optional token Issuer and route Policy.
type Config struct {
	// Mode selects the validator: "local", "remote", or "func".
	Mode string `mapstructure:"mode"`
	// DefaultProtected is the route-policy default when no rule matches.
	DefaultProtected bool `mapstructure:"default_protected"`
	// Rules are the route-policy rules (first match wins).
	Rules []Rule `mapstructure:"rules"`
	// Local configures the in-process JWT validator (mode "local").
	Local LocalConfig `mapstructure:"local"`
	// Issuer configures token issuing (optional; validated only when its
	// Algorithm is set).
	Issuer IssuerConfig `mapstructure:"issuer"`
	// Remote configures the network validator (mode "remote").
	Remote RemoteConfig `mapstructure:"remote"`
	// CacheTTL, when > 0, wraps the validator in a TTL cache of successful
	// validations.
	CacheTTL time.Duration `mapstructure:"cache_ttl"`
}

// LocalConfig configures the in-process JWT validator.
type LocalConfig struct {
	// Algorithm is "HS256" or "RS256".
	Algorithm string `mapstructure:"algorithm"`
	// HS256Secret is the shared secret for HS256.
	HS256Secret string `mapstructure:"hs256_secret"`
	// RS256PublicKeyPath is a PEM file with the RS256 verification key.
	RS256PublicKeyPath string `mapstructure:"rs256_public_key_path"`
	// JWKSURL fetches RS256 keys by kid from a JWKS endpoint (alternative to
	// RS256PublicKeyPath).
	JWKSURL string `mapstructure:"jwks_url"`
	// Issuer, when set, is the required "iss" claim.
	Issuer string `mapstructure:"issuer"`
	// Audience, when set, is the required "aud" claim.
	Audience string `mapstructure:"audience"`
}

// IssuerConfig configures token issuing.
type IssuerConfig struct {
	// Algorithm is "HS256" or "RS256". Empty disables issuing.
	Algorithm string `mapstructure:"algorithm"`
	// HS256Secret is the signing secret for HS256.
	HS256Secret string `mapstructure:"hs256_secret"`
	// RS256PrivateKeyPath is a PEM file with the RS256 signing key.
	RS256PrivateKeyPath string `mapstructure:"rs256_private_key_path"`
	// DefaultTTL is the default token lifetime.
	DefaultTTL time.Duration `mapstructure:"default_ttl"`
	// Issuer is the "iss" claim written on issued tokens.
	Issuer string `mapstructure:"issuer"`
	// Audience is the default "aud" claim written on issued tokens.
	Audience string `mapstructure:"audience"`
}

// DefaultConfig returns a Config with sane defaults: local HS256 validation,
// everything protected by default, a 15-minute HS256 issuer, and remote
// defaults matching a bearer-header verification call.
func DefaultConfig() Config {
	return Config{
		Mode:             ModeLocal,
		DefaultProtected: true,
		Local:            LocalConfig{Algorithm: AlgorithmHS256},
		Issuer:           IssuerConfig{DefaultTTL: defaultIssuerTTL},
		Remote: RemoteConfig{
			Method:  defaultRemoteMethod,
			Timeout: defaultRemoteTimeout,
			Forward: TokenForward{In: ForwardInHeader, Name: defaultForwardName, Prefix: defaultForwardPrefix},
			Mapping: ClaimsMapping{SubjectField: defaultSubjectField, RolesField: defaultRolesField},
		},
	}
}

// Policy returns the route policy derived from the config's DefaultProtected
// and Rules fields.
func (c *Config) Policy() Policy {
	return Policy{DefaultProtected: c.DefaultProtected, Rules: c.Rules}
}

// Validate checks that Config is internally consistent for its selected mode.
// The Issuer section is validated only when its Algorithm is set, so
// validation-only consumers need not configure issuing.
func (c *Config) Validate() error {
	switch c.Mode {
	case ModeLocal:
		if err := validateLocal(&c.Local); err != nil {
			return err
		}
	case ModeRemote:
		if err := validateRemote(&c.Remote); err != nil {
			return err
		}
	case ModeFunc:
		// No serializable settings; the ValidatorFunc is supplied in code.
	default:
		return errorz.BadRequest().WithMessage("auth: mode must be one of local, remote, func")
	}
	if c.CacheTTL < 0 {
		return errorz.BadRequest().WithMessage("auth: cache_ttl must not be negative")
	}
	if err := validateRules(c.Rules); err != nil {
		return err
	}
	if c.Issuer.Algorithm != "" {
		return validateIssuer(&c.Issuer)
	}
	return nil
}

// validateLocal checks the in-process validator settings.
func validateLocal(l *LocalConfig) error {
	switch l.Algorithm {
	case AlgorithmHS256:
		if l.HS256Secret == "" {
			return errorz.BadRequest().WithMessage("auth: local.hs256_secret is required for HS256")
		}
	case AlgorithmRS256:
		hasKey := l.RS256PublicKeyPath != ""
		hasJWKS := l.JWKSURL != ""
		if hasKey == hasJWKS {
			return errorz.BadRequest().
				WithMessage("auth: local RS256 requires exactly one of rs256_public_key_path or jwks_url")
		}
	default:
		return errorz.BadRequest().WithMessage("auth: local.algorithm must be HS256 or RS256")
	}
	return nil
}

// validateRemote checks the network validator settings.
func validateRemote(r *RemoteConfig) error {
	if r.URL == "" {
		return errorz.BadRequest().WithMessage("auth: remote.url is required for remote mode")
	}
	switch r.Forward.In {
	case "", ForwardInHeader, ForwardInBody, ForwardInQuery:
		// "" is allowed and defaults to header via withDefaults.
	default:
		return errorz.BadRequest().WithMessage("auth: remote.forward.in must be header, body, or query")
	}
	return nil
}

// validateIssuer checks the token-issuer settings.
func validateIssuer(i *IssuerConfig) error {
	switch i.Algorithm {
	case AlgorithmHS256:
		if i.HS256Secret == "" {
			return errorz.BadRequest().WithMessage("auth: issuer.hs256_secret is required for HS256")
		}
	case AlgorithmRS256:
		if i.RS256PrivateKeyPath == "" {
			return errorz.BadRequest().WithMessage("auth: issuer.rs256_private_key_path is required for RS256")
		}
	default:
		return errorz.BadRequest().WithMessage("auth: issuer.algorithm must be HS256 or RS256")
	}
	return nil
}

// validateRules checks that every route rule has a pattern.
func validateRules(rules []Rule) error {
	for _, r := range rules {
		if r.Pattern == "" {
			return errorz.BadRequest().WithMessage("auth: every rule must have a pattern")
		}
	}
	return nil
}

// withDefaults fills zero-valued fields of cfg from DefaultConfig() so callers
// (and YAML) only need to override what differs. Used by the FromConfig
// factories.
func withDefaults(cfg *Config) {
	def := DefaultConfig()
	if cfg.Mode == "" {
		cfg.Mode = def.Mode
	}
	if cfg.Local.Algorithm == "" {
		cfg.Local.Algorithm = def.Local.Algorithm
	}
	if cfg.Issuer.DefaultTTL == 0 {
		cfg.Issuer.DefaultTTL = def.Issuer.DefaultTTL
	}
	remoteWithDefaults(&cfg.Remote)
}

// FromConfig builds a Validator from cfg, selecting the backend by cfg.Mode and
// wrapping it in a TTL cache when cfg.CacheTTL > 0. Mode "func" is rejected: a
// ValidatorFunc must be constructed in code. Opts pass non-serializable
// dependencies (e.g. a custom HTTP client) through to the backend.
func FromConfig(cfg *Config, opts ...Option) (Validator, error) {
	withDefaults(cfg)
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	var (
		v   Validator
		err error
	)
	switch cfg.Mode {
	case ModeLocal:
		v, err = buildLocal(&cfg.Local, opts...)
	case ModeRemote:
		v, err = NewRemote(&cfg.Remote, opts...)
	case ModeFunc:
		return nil, errorz.BadRequest().WithMessage("auth: mode func requires constructing a ValidatorFunc in code")
	default:
		return nil, errorz.BadRequest().WithMessage("auth: unknown mode")
	}
	if err != nil {
		return nil, err
	}
	return NewCached(v, cfg.CacheTTL), nil
}

// buildLocal constructs the in-process JWT validator from the local config,
// threading the expected issuer/audience through as options.
func buildLocal(l *LocalConfig, opts ...Option) (Validator, error) {
	expect := make([]Option, 0, len(opts)+2)
	if l.Issuer != "" {
		expect = append(expect, WithExpectedIssuer(l.Issuer))
	}
	if l.Audience != "" {
		expect = append(expect, WithExpectedAudience(l.Audience))
	}
	expect = append(expect, opts...)

	switch l.Algorithm {
	case AlgorithmHS256:
		return NewHS256(l.HS256Secret, expect...)
	case AlgorithmRS256:
		return buildLocalRS256(l, expect)
	default:
		return nil, errorz.BadRequest().WithMessage("auth: local.algorithm must be HS256 or RS256")
	}
}

// buildLocalRS256 builds an RS256 validator from either a JWKS URL or a PEM
// public-key file.
func buildLocalRS256(l *LocalConfig, opts []Option) (Validator, error) {
	if l.JWKSURL != "" {
		return NewRS256(append(opts, WithJWKSURL(l.JWKSURL))...)
	}
	pemBytes, err := os.ReadFile(l.RS256PublicKeyPath)
	if err != nil {
		return nil, errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("auth: read rs256 public key")
	}
	pub, err := parseRSAPublicKeyPEM(pemBytes)
	if err != nil {
		return nil, err
	}
	return NewRS256(append(opts, WithPublicKey(pub))...)
}

// IssuerFromConfig builds a token Issuer from cfg.Issuer. The issuer section
// must set an Algorithm.
func IssuerFromConfig(cfg *Config, opts ...IssuerOption) (Issuer, error) {
	ic := &cfg.Issuer
	if ic.Algorithm == "" {
		return nil, errorz.BadRequest().WithMessage("auth: issuer.algorithm is required to build an Issuer")
	}
	if err := validateIssuer(ic); err != nil {
		return nil, err
	}

	base := make([]IssuerOption, 0, len(opts)+3)
	if ic.Issuer != "" {
		base = append(base, WithIssuerName(ic.Issuer))
	}
	if ic.Audience != "" {
		base = append(base, WithDefaultAudience(ic.Audience))
	}
	if ic.DefaultTTL > 0 {
		base = append(base, WithDefaultTTL(ic.DefaultTTL))
	}
	base = append(base, opts...)

	switch ic.Algorithm {
	case AlgorithmHS256:
		return NewHS256Issuer(ic.HS256Secret, base...)
	case AlgorithmRS256:
		return buildRS256Issuer(ic, base)
	default:
		return nil, errorz.BadRequest().WithMessage("auth: issuer.algorithm must be HS256 or RS256")
	}
}

// buildRS256Issuer reads the PEM private key and builds an RS256 issuer.
func buildRS256Issuer(ic *IssuerConfig, opts []IssuerOption) (Issuer, error) {
	pemBytes, err := os.ReadFile(ic.RS256PrivateKeyPath)
	if err != nil {
		return nil, errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("auth: read rs256 private key")
	}
	key, err := parseRSAPrivateKeyPEM(pemBytes)
	if err != nil {
		return nil, err
	}
	return NewRS256Issuer(key, opts...)
}

// remoteWithDefaults fills the remote sub-config's zero fields from the defaults.
func remoteWithDefaults(r *RemoteConfig) {
	def := DefaultConfig().Remote
	if r.Method == "" {
		r.Method = def.Method
	}
	if r.Timeout == 0 {
		r.Timeout = def.Timeout
	}
	if r.Forward.In == "" {
		// A fully-unset forward block defaults to a bearer Authorization header.
		// An explicit In (query/body) keeps its own name and no bearer prefix.
		r.Forward.In = def.Forward.In
		if r.Forward.Name == "" {
			r.Forward.Name = def.Forward.Name
		}
		if r.Forward.Prefix == "" {
			r.Forward.Prefix = def.Forward.Prefix
		}
	}
	if r.Mapping.SubjectField == "" {
		r.Mapping.SubjectField = def.Mapping.SubjectField
	}
	if r.Mapping.RolesField == "" {
		r.Mapping.RolesField = def.Mapping.RolesField
	}
}
