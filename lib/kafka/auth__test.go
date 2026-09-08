package kafka

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/segmentio/kafka-go/sasl/plain"
	"github.com/segmentio/kafka-go/sasl/scram"
)

// selfSignedCertPEM generates a throwaway self-signed cert/key pair for
// exercising buildTLSConfig's file-loading paths — content doesn't matter,
// only that it's valid PEM x509.
func selfSignedCertPEM(t *testing.T) (certPEM, keyPEM []byte) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "kafka-test"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("CreateCertificate() error = %v", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("MarshalECPrivateKey() error = %v", err)
	}

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	return certPEM, keyPEM
}

// writeTestCert writes a standalone self-signed cert (its matching key is
// discarded) — enough for buildTLSConfig's ca_file parse path, which never
// needs a private key.
func writeTestCert(t *testing.T, dir, name string) string {
	t.Helper()
	certPEM, _ := selfSignedCertPEM(t)
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, certPEM, 0o600); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, err)
	}
	return path
}

// writeTestCertKeyPair writes a cert and its matching private key from the
// same generated pair, since tls.LoadX509KeyPair requires them to match.
func writeTestCertKeyPair(t *testing.T, dir string) (certFile, keyFile string) {
	t.Helper()
	certPEM, keyPEM := selfSignedCertPEM(t)
	certFile = filepath.Join(dir, "client-cert.pem")
	keyFile = filepath.Join(dir, "client-key.pem")
	if err := os.WriteFile(certFile, certPEM, 0o600); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", certFile, err)
	}
	if err := os.WriteFile(keyFile, keyPEM, 0o600); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", keyFile, err)
	}
	return certFile, keyFile
}

func TestBuildSASLMechanism(t *testing.T) {
	tests := []struct {
		name    string
		cfg     AuthConfig
		want    string // sasl.Mechanism.Name(), "" for nil
		wantErr bool
	}{
		{name: "none", cfg: AuthConfig{Mechanism: MechanismNone}, want: ""},
		{name: "mtls", cfg: AuthConfig{Mechanism: MechanismMTLS}, want: ""},
		{
			name: "plain",
			cfg:  AuthConfig{Mechanism: MechanismPlain, Username: "u", Password: "p"},
			want: plain.Mechanism{}.Name(),
		},
		{
			name: "scram-sha256",
			cfg:  AuthConfig{Mechanism: MechanismScramSHA256, Username: "u", Password: "p"},
			want: scram.SHA256.Name(),
		},
		{
			name: "scram-sha512",
			cfg:  AuthConfig{Mechanism: MechanismScramSHA512, Username: "u", Password: "p"},
			want: scram.SHA512.Name(),
		},
		{name: "unknown", cfg: AuthConfig{Mechanism: "bogus"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := buildSASLMechanism(&tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("buildSASLMechanism() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if tt.want == "" {
				if m != nil {
					t.Errorf("buildSASLMechanism() = %v, want nil", m)
				}
				return
			}
			if m == nil || m.Name() != tt.want {
				t.Errorf("buildSASLMechanism().Name() = %v, want %v", m, tt.want)
			}
		})
	}
}

func TestBuildTLSConfig(t *testing.T) {
	dir := t.TempDir()
	caFile := writeTestCert(t, dir, "ca.pem")
	certFile, keyFile := writeTestCertKeyPair(t, dir)

	t.Run("no tls needed", func(t *testing.T) {
		cfg, err := buildTLSConfig(&AuthConfig{Mechanism: MechanismPlain})
		if err != nil {
			t.Fatalf("buildTLSConfig() error = %v, want nil", err)
		}
		if cfg != nil {
			t.Errorf("buildTLSConfig() = %v, want nil", cfg)
		}
	})

	t.Run("sasl over tls with a private ca", func(t *testing.T) {
		cfg, err := buildTLSConfig(&AuthConfig{Mechanism: MechanismPlain, TLS: TLSConfig{CAFile: caFile}})
		if err != nil {
			t.Fatalf("buildTLSConfig() error = %v, want nil", err)
		}
		if cfg == nil || cfg.RootCAs == nil {
			t.Fatal("buildTLSConfig() did not set RootCAs")
		}
	})

	t.Run("mtls loads client cert and key", func(t *testing.T) {
		cfg, err := buildTLSConfig(&AuthConfig{
			Mechanism: MechanismMTLS,
			TLS:       TLSConfig{CertFile: certFile, KeyFile: keyFile, CAFile: caFile},
		})
		if err != nil {
			t.Fatalf("buildTLSConfig() error = %v, want nil", err)
		}
		if cfg == nil || len(cfg.Certificates) != 1 {
			t.Fatal("buildTLSConfig() did not load a client certificate")
		}
	})

	t.Run("mtls with missing cert file errors", func(t *testing.T) {
		_, err := buildTLSConfig(&AuthConfig{
			Mechanism: MechanismMTLS,
			TLS:       TLSConfig{CertFile: "/nonexistent/cert.pem", KeyFile: "/nonexistent/key.pem"},
		})
		if err == nil {
			t.Fatal("buildTLSConfig() error = nil, want non-nil for missing cert files")
		}
	})

	t.Run("bad ca file errors", func(t *testing.T) {
		badCA := filepath.Join(dir, "bad-ca.pem")
		if err := os.WriteFile(badCA, []byte("not a cert"), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := buildTLSConfig(&AuthConfig{Mechanism: MechanismPlain, TLS: TLSConfig{CAFile: badCA}})
		if err == nil {
			t.Fatal("buildTLSConfig() error = nil, want non-nil for an invalid ca_file")
		}
	})
}
