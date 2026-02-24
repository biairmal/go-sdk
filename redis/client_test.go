package redis

import (
	"crypto/tls"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewClient_InvalidConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr string
	}{
		{
			name: "unsupported mode",
			cfg: &Config{
				Mode:    Mode("invalid"),
				Address: "localhost:6379",
			},
			wantErr: "unsupported redis mode",
		},
		{
			name: "sentinel mode missing master name",
			cfg: &Config{
				Mode:          ModeSentinel,
				SentinelAddrs: []string{"127.0.0.1:26379"},
			},
			wantErr: "sentinel mode requires master name",
		},
		{
			name: "sentinel mode empty sentinel addrs",
			cfg: &Config{
				Mode:          ModeSentinel,
				MasterName:    "mymaster",
				SentinelAddrs: []string{},
			},
			wantErr: "sentinel mode requires master name and sentinel addresses",
		},
		{
			name: "cluster mode empty cluster addrs",
			cfg: &Config{
				Mode:         ModeCluster,
				ClusterAddrs: nil,
			},
			wantErr: "cluster mode requires at least one cluster address",
		},
		{
			name: "cluster mode zero addrs",
			cfg: &Config{
				Mode:         ModeCluster,
				ClusterAddrs: []string{},
			},
			wantErr: "cluster mode requires at least one cluster address",
		},
		{
			name: "TLS enabled with missing cert file",
			cfg: &Config{
				Mode:       ModeStandalone,
				Address:    "localhost:6379",
				TLSEnabled: true,
				TLSCert:    filepath.Join(os.TempDir(), "nonexistent-cert.pem"),
				TLSKey:     filepath.Join(os.TempDir(), "nonexistent-key.pem"),
			},
			wantErr: "tls config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewClient(tt.cfg)
			if err == nil {
				t.Fatal("NewClient: expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("NewClient: error = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestCreateTLSConfig(t *testing.T) {
	dir := t.TempDir()
	missingPath := filepath.Join(dir, "missing.pem")

	tests := []struct {
		name    string
		cfg     *Config
		wantErr string
		check   func(*testing.T, *tls.Config)
	}{
		{
			name: "TLS enabled no cert no CA returns base config",
			cfg: &Config{
				TLSEnabled: true,
				TLSCert:    "",
				TLSKey:     "",
				TLSCA:      "",
			},
			wantErr: "",
			check: func(t *testing.T, c *tls.Config) {
				if c.MinVersion != tls.VersionTLS12 {
					t.Errorf("MinVersion = %x, want TLS 1.2", c.MinVersion)
				}
				if c.MaxVersion != tls.VersionTLS13 {
					t.Errorf("MaxVersion = %x, want TLS 1.3", c.MaxVersion)
				}
			},
		},
		{
			name: "TLS enabled with nonexistent cert key pair",
			cfg: &Config{
				TLSEnabled: true,
				TLSCert:    missingPath,
				TLSKey:     missingPath,
			},
			wantErr: "load client cert/key",
			check:   nil,
		},
		{
			name: "TLS enabled with nonexistent CA file",
			cfg: &Config{
				TLSEnabled: true,
				TLSCA:      missingPath,
			},
			wantErr: "read CA file",
			check:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := createTLSConfig(tt.cfg)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("createTLSConfig: expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("createTLSConfig: error = %v, want substring %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("createTLSConfig: unexpected error: %v", err)
			}
			if got == nil {
				t.Fatal("createTLSConfig: expected non-nil config")
			}
			if tt.check != nil {
				tt.check(t, got)
			}
		})
	}
}

func TestCreateTLSConfig_invalidCAPEM(t *testing.T) {
	dir := t.TempDir()
	invalidPath := filepath.Join(dir, "invalid.pem")
	if err := os.WriteFile(invalidPath, []byte("not a valid PEM"), 0o600); err != nil {
		t.Fatalf("write invalid CA file: %v", err)
	}
	_, err := createTLSConfig(&Config{TLSEnabled: true, TLSCA: invalidPath})
	if err == nil {
		t.Fatal("createTLSConfig: expected error for invalid CA PEM, got nil")
	}
	if !strings.Contains(err.Error(), "no valid CA certificates") {
		t.Errorf("createTLSConfig: error = %v, want substring 'no valid CA certificates'", err)
	}
}
