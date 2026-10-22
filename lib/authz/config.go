package authz

import (
	"time"

	"github.com/biairmal/go-sdk/lib/errorz"
)

// Permission sources for Config.Source.
const (
	// SourceToken reads the caller's permissions from a token claim.
	SourceToken = "token"
	// SourceConfig maps the caller's roles to permissions with Config.Roles.
	SourceConfig = "config"
	// SourceIdentity maps roles with a role map fetched from the identity service.
	SourceIdentity = "identity"
)

// Config holds the YAML-able authz settings.
type Config struct {
	// Source decides where permissions come from: "token", "config" or "identity".
	Source string `mapstructure:"source"`
	// Roles maps role names to granted permissions. Used by the "config" source,
	// and by the "token" source to resolve resource-scoped roles.
	Roles map[string][]string `mapstructure:"roles"`
	// IdentityURL is the identity service base URL. Required for the "identity" source.
	IdentityURL string `mapstructure:"identity_url"`
	// CacheTTL is how long a fetched role map stays fresh ("identity" source).
	CacheTTL time.Duration `mapstructure:"cache_ttl"`
	// PermsClaim is the token claim holding permissions ("token" source).
	PermsClaim string `mapstructure:"perms_claim"`
	// TenantClaim is the token claim holding the tenant ID.
	TenantClaim string `mapstructure:"tenant_claim"`
	// SuperClaim is the boolean token claim that grants every permission.
	SuperClaim string `mapstructure:"super_claim"`
}

// DefaultConfig returns the defaults: token source, 60s cache TTL and the
// claims perms, tid and sa.
func DefaultConfig() Config {
	return Config{
		Source:      SourceToken,
		CacheTTL:    60 * time.Second,
		PermsClaim:  "perms",
		TenantClaim: "tid",
		SuperClaim:  "sa",
	}
}

// Validate checks that Config is well-formed for its selected source.
func (c *Config) Validate() error {
	switch c.Source {
	case SourceToken, SourceConfig:
	case SourceIdentity:
		if c.IdentityURL == "" {
			return errorz.BadRequest().WithMessage("authz: identity_url is required for the identity source")
		}
	default:
		return errorz.BadRequest().WithMessage("authz: source must be token, config or identity")
	}
	if c.CacheTTL <= 0 {
		return errorz.BadRequest().WithMessage("authz: cache_ttl must be positive")
	}
	if c.PermsClaim == "" || c.TenantClaim == "" || c.SuperClaim == "" {
		return errorz.BadRequest().WithMessage("authz: perms_claim, tenant_claim and super_claim are required")
	}
	return nil
}

// withDefaults fills zero-valued fields of cfg from DefaultConfig().
func withDefaults(cfg Config) Config { //nolint:gocritic // works on a copy by design
	def := DefaultConfig()
	if cfg.Source == "" {
		cfg.Source = def.Source
	}
	if cfg.CacheTTL == 0 {
		cfg.CacheTTL = def.CacheTTL
	}
	if cfg.PermsClaim == "" {
		cfg.PermsClaim = def.PermsClaim
	}
	if cfg.TenantClaim == "" {
		cfg.TenantClaim = def.TenantClaim
	}
	if cfg.SuperClaim == "" {
		cfg.SuperClaim = def.SuperClaim
	}
	return cfg
}
