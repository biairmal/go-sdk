package auth

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Mode != ModeLocal {
		t.Errorf("Mode = %q, want %q", cfg.Mode, ModeLocal)
	}
	if !cfg.DefaultProtected {
		t.Error("DefaultProtected = false, want true")
	}
	if cfg.Local.Algorithm != AlgorithmHS256 {
		t.Errorf("Local.Algorithm = %q, want %q", cfg.Local.Algorithm, AlgorithmHS256)
	}
	if cfg.Remote.Forward.In != ForwardInHeader || cfg.Remote.Forward.Prefix != "Bearer " {
		t.Errorf("Remote.Forward = %+v, want header/Bearer default", cfg.Remote.Forward)
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{"local hs256 ok", Config{Mode: ModeLocal, Local: LocalConfig{Algorithm: AlgorithmHS256, HS256Secret: "s"}}, false},
		{"local hs256 missing secret", Config{Mode: ModeLocal, Local: LocalConfig{Algorithm: AlgorithmHS256}}, true},
		{"local rs256 both sources", Config{Mode: ModeLocal, Local: LocalConfig{
			Algorithm: AlgorithmRS256, RS256PublicKeyPath: "k.pem", JWKSURL: "https://x",
		}}, true},
		{"local rs256 neither source", Config{Mode: ModeLocal, Local: LocalConfig{Algorithm: AlgorithmRS256}}, true},
		{"local rs256 jwks only", Config{Mode: ModeLocal, Local: LocalConfig{
			Algorithm: AlgorithmRS256, JWKSURL: "https://x",
		}}, false},
		{"unknown mode", Config{Mode: "bogus"}, true},
		{"remote missing url", Config{Mode: ModeRemote}, true},
		{"remote bad forward.in", Config{Mode: ModeRemote, Remote: RemoteConfig{
			URL: "https://x", Forward: TokenForward{In: "cookie"},
		}}, true},
		{"remote ok", Config{Mode: ModeRemote, Remote: RemoteConfig{URL: "https://x"}}, false},
		{"negative cache ttl", Config{Mode: ModeLocal, Local: LocalConfig{
			Algorithm: AlgorithmHS256, HS256Secret: "s",
		}, CacheTTL: -time.Second}, true},
		{"rule without pattern", Config{Mode: ModeLocal, Local: LocalConfig{
			Algorithm: AlgorithmHS256, HS256Secret: "s",
		}, Rules: []Rule{{Public: true}}}, true},
		{"issuer validated when algorithm set", Config{Mode: ModeLocal, Local: LocalConfig{
			Algorithm: AlgorithmHS256, HS256Secret: "s",
		}, Issuer: IssuerConfig{Algorithm: AlgorithmHS256}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFromConfigLocalHS256(t *testing.T) {
	cfg := Config{Mode: ModeLocal, Local: LocalConfig{Algorithm: AlgorithmHS256, HS256Secret: "secret"}}
	v, err := FromConfig(&cfg)
	if err != nil {
		t.Fatalf("FromConfig: %v", err)
	}

	issuer, _ := NewHS256Issuer("secret")
	token, _ := issuer.Issue("u")
	if _, err := v.Validate(context.Background(), token); err != nil {
		t.Errorf("Validate: %v", err)
	}
}

func TestFromConfigWrapsCache(t *testing.T) {
	cfg := Config{
		Mode:     ModeLocal,
		Local:    LocalConfig{Algorithm: AlgorithmHS256, HS256Secret: "secret"},
		CacheTTL: time.Minute,
	}
	v, err := FromConfig(&cfg)
	if err != nil {
		t.Fatalf("FromConfig: %v", err)
	}
	if _, ok := v.(*cachedValidator); !ok {
		t.Errorf("FromConfig with cache_ttl>0 should return a *cachedValidator, got %T", v)
	}
}

func TestFromConfigFuncMode(t *testing.T) {
	cfg := Config{Mode: ModeFunc}
	if _, err := FromConfig(&cfg); err == nil {
		t.Error("expected FromConfig to reject mode func")
	}
}

func TestIssuerFromConfig(t *testing.T) {
	cfg := Config{Issuer: IssuerConfig{
		Algorithm: AlgorithmHS256, HS256Secret: "issuer-secret", Issuer: "svc", DefaultTTL: time.Hour,
	}}
	issuer, err := IssuerFromConfig(&cfg)
	if err != nil {
		t.Fatalf("IssuerFromConfig: %v", err)
	}
	if _, err := issuer.Issue("u"); err != nil {
		t.Errorf("Issue: %v", err)
	}
}

func TestIssuerFromConfigRS256(t *testing.T) {
	key := genRSAKey(t)
	der := x509.MarshalPKCS1PrivateKey(key)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: der})
	path := filepath.Join(t.TempDir(), "key.pem")
	if err := os.WriteFile(path, keyPEM, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg := Config{Issuer: IssuerConfig{Algorithm: AlgorithmRS256, RS256PrivateKeyPath: path, Issuer: "svc"}}
	issuer, err := IssuerFromConfig(&cfg)
	if err != nil {
		t.Fatalf("IssuerFromConfig: %v", err)
	}
	token, err := issuer.Issue("rs-user")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	v, _ := NewRS256(WithPublicKey(&key.PublicKey))
	if _, err := v.Validate(context.Background(), token); err != nil {
		t.Errorf("Validate issued RS256 token: %v", err)
	}
}

func TestIssuerFromConfigRequiresAlgorithm(t *testing.T) {
	cfg := Config{}
	if _, err := IssuerFromConfig(&cfg); err == nil {
		t.Error("expected error when issuer.algorithm is unset")
	}
}

func TestConfigPolicy(t *testing.T) {
	cfg := Config{DefaultProtected: true, Rules: []Rule{{Pattern: "/health", Public: true}}}
	pol := cfg.Policy()
	if pol.IsProtected("GET", "/health") {
		t.Error("expected /health to be public via policy")
	}
	if !pol.IsProtected("GET", "/api") {
		t.Error("expected /api to be protected by default")
	}
}
