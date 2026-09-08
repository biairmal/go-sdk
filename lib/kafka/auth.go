package kafka

import (
	"crypto/tls"
	"crypto/x509"
	"os"

	"github.com/segmentio/kafka-go/sasl"
	"github.com/segmentio/kafka-go/sasl/plain"
	"github.com/segmentio/kafka-go/sasl/scram"

	"github.com/biairmal/go-sdk/lib/errorz"
)

// buildSASLMechanism resolves cfg.Mechanism to a sasl.Mechanism. Returns nil
// for "none" and "mtls" — mTLS authenticates via the TLS handshake, not SASL.
func buildSASLMechanism(cfg *AuthConfig) (sasl.Mechanism, error) {
	switch cfg.Mechanism {
	case MechanismNone, MechanismMTLS:
		return nil, nil
	case MechanismPlain:
		return plain.Mechanism{Username: cfg.Username, Password: cfg.Password}, nil
	case MechanismScramSHA256:
		return scram.Mechanism(scram.SHA256, cfg.Username, cfg.Password)
	case MechanismScramSHA512:
		return scram.Mechanism(scram.SHA512, cfg.Username, cfg.Password)
	default:
		return nil, errorz.BadRequest().WithMessage("kafka: unknown auth mechanism " + cfg.Mechanism)
	}
}

// buildTLSConfig builds a *tls.Config when cfg.Mechanism is "mtls" (client
// cert + optional CA), or when a ca_file is set alongside a SASL mechanism
// (SASL-over-TLS with a private CA). Returns (nil, nil) when neither applies
// — the connection then uses whatever TLS default the caller's Dial does
// (plaintext, for the stdlib dialer).
func buildTLSConfig(cfg *AuthConfig) (*tls.Config, error) {
	if cfg.Mechanism != MechanismMTLS && cfg.TLS.CAFile == "" {
		return nil, nil
	}

	tlsCfg := &tls.Config{MinVersion: tls.VersionTLS12}

	if cfg.TLS.CAFile != "" {
		caPEM, err := os.ReadFile(cfg.TLS.CAFile)
		if err != nil {
			return nil, errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("kafka: failed to read ca_file")
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caPEM) {
			return nil, errorz.BadRequest().WithMessage("kafka: ca_file contains no valid certificates")
		}
		tlsCfg.RootCAs = pool
	}

	if cfg.Mechanism == MechanismMTLS {
		cert, err := tls.LoadX509KeyPair(cfg.TLS.CertFile, cfg.TLS.KeyFile)
		if err != nil {
			return nil, errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("kafka: failed to load client cert/key")
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}

	return tlsCfg, nil
}
