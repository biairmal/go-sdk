package kafka

import (
	"github.com/biairmal/go-sdk/lib/errorz"
)

// SASL/TLS mechanism selectors for AuthConfig.Mechanism.
const (
	MechanismNone        = "none"
	MechanismPlain       = "plain"
	MechanismScramSHA256 = "scram-sha256"
	MechanismScramSHA512 = "scram-sha512"
	MechanismMTLS        = "mtls"
)

// Config holds broker connection settings, mapstructure-tagged for
// config.Load.
type Config struct {
	// Brokers is the list of "host:port" bootstrap addresses.
	Brokers []string `mapstructure:"brokers"`
	// Auth selects and configures the one active SASL/TLS mechanism.
	Auth AuthConfig `mapstructure:"auth"`
}

// AuthConfig selects one SASL/TLS mechanism. Only one is active per Config —
// same shape as auth.Config's mode switch, not a struct per mechanism.
type AuthConfig struct {
	// Mechanism selects how the client authenticates: "none", "plain",
	// "scram-sha256", "scram-sha512", or "mtls".
	Mechanism string `mapstructure:"mechanism"`
	// Username is required for "plain" and "scram-*".
	Username string `mapstructure:"username"`
	// Password is required for "plain" and "scram-*".
	Password string `mapstructure:"password"`
	// TLS is required for "mtls", and optional alongside SASL over TLS.
	TLS TLSConfig `mapstructure:"tls"`
}

// TLSConfig points at PEM-encoded certificate material on disk.
type TLSConfig struct {
	CertFile string `mapstructure:"cert_file"`
	KeyFile  string `mapstructure:"key_file"`
	CAFile   string `mapstructure:"ca_file"`
}

// DefaultConfig returns a Config with no brokers and no auth ("none").
// Brokers has no sane default — callers must set it — so DefaultConfig()
// alone does not pass Validate(), the same precedent as crypto.DefaultConfig.
func DefaultConfig() Config {
	return Config{Auth: AuthConfig{Mechanism: MechanismNone}}
}

// Validate checks that Config is well-formed for its selected mechanism.
func (c *Config) Validate() error {
	if len(c.Brokers) == 0 {
		return errorz.BadRequest().WithMessage("kafka: brokers must not be empty")
	}
	switch c.Auth.Mechanism {
	case MechanismNone:
	case MechanismPlain, MechanismScramSHA256, MechanismScramSHA512:
		if c.Auth.Username == "" || c.Auth.Password == "" {
			return errorz.BadRequest().WithMessage("kafka: username and password are required for " + c.Auth.Mechanism)
		}
	case MechanismMTLS:
		if c.Auth.TLS.CertFile == "" || c.Auth.TLS.KeyFile == "" || c.Auth.TLS.CAFile == "" {
			return errorz.BadRequest().WithMessage("kafka: cert_file, key_file and ca_file are required for mtls")
		}
	default:
		return errorz.BadRequest().WithMessage("kafka: unknown auth mechanism " + c.Auth.Mechanism)
	}
	return nil
}
