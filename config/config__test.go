package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoad_noFiles(t *testing.T) {
	var dst struct {
		Port int `mapstructure:"port"`
	}
	err := Load(&dst)
	if err != nil {
		t.Fatalf("Load(&dst) = %v", err)
	}
	if dst.Port != 0 {
		t.Errorf("dst.Port = %d, want 0", dst.Port)
	}
}

func TestLoad_singleFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := "port: 8080\nname: test\n"
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	var dst struct {
		Port int    `mapstructure:"port"`
		Name string `mapstructure:"name"`
	}
	err := Load(&dst, Files(path))
	if err != nil {
		t.Fatalf("Load = %v", err)
	}
	if dst.Port != 8080 {
		t.Errorf("port = %d, want 8080", dst.Port)
	}
	if dst.Name != "test" {
		t.Errorf("name = %q, want test", dst.Name)
	}
}

func TestLoad_envSubstitution(t *testing.T) {
	t.Setenv("DB_URL", "postgres://localhost/db")
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := "database_url: ${DB_URL}\n"
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	var dst struct {
		DatabaseURL string `mapstructure:"database_url"`
	}
	err := Load(&dst, Files(path))
	if err != nil {
		t.Fatalf("Load = %v", err)
	}
	if dst.DatabaseURL != "postgres://localhost/db" {
		t.Errorf("database_url = %q, want postgres://localhost/db", dst.DatabaseURL)
	}
}

func TestLoad_envSubstitutionWithDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := "database_url: ${MISSING_VAR:postgres://default/host}\n"
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	var dst struct {
		DatabaseURL string `mapstructure:"database_url"`
	}
	err := Load(&dst, Files(path))
	if err != nil {
		t.Fatalf("Load = %v", err)
	}
	if dst.DatabaseURL != "postgres://default/host" {
		t.Errorf("database_url = %q, want postgres://default/host", dst.DatabaseURL)
	}
}

func TestLoad_mergeMultipleFiles(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "base.yaml")
	overlay := filepath.Join(dir, "overlay.yaml")
	if err := os.WriteFile(base, []byte("port: 8080\nname: base\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(overlay, []byte("name: overlay\n"), 0600); err != nil {
		t.Fatal(err)
	}

	var dst struct {
		Port int    `mapstructure:"port"`
		Name string `mapstructure:"name"`
	}
	err := Load(&dst, Files(base, overlay))
	if err != nil {
		t.Fatalf("Load = %v", err)
	}
	if dst.Port != 8080 {
		t.Errorf("port = %d, want 8080 (from base)", dst.Port)
	}
	if dst.Name != "overlay" {
		t.Errorf("name = %q, want overlay (from overlay)", dst.Name)
	}
}

func TestLoad_nestedStruct(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `
handler:
  port: 8080
  read_timeout: 60s
domain:
  name: example.com
  tls: true
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	type HandlerOptions struct {
		Port        int           `mapstructure:"port"`
		ReadTimeout time.Duration `mapstructure:"read_timeout"`
	}
	type DomainOptions struct {
		Name string `mapstructure:"name"`
		TLS  bool   `mapstructure:"tls"`
	}
	var dst struct {
		Handler HandlerOptions `mapstructure:"handler"`
		Domain  DomainOptions `mapstructure:"domain"`
	}
	err := Load(&dst, Files(path))
	if err != nil {
		t.Fatalf("Load = %v", err)
	}
	if dst.Handler.Port != 8080 {
		t.Errorf("handler.port = %d, want 8080", dst.Handler.Port)
	}
	if dst.Handler.ReadTimeout != 60*time.Second {
		t.Errorf("handler.read_timeout = %v, want 60s", dst.Handler.ReadTimeout)
	}
	if dst.Domain.Name != "example.com" {
		t.Errorf("domain.name = %q, want example.com", dst.Domain.Name)
	}
	if !dst.Domain.TLS {
		t.Error("domain.tls = false, want true")
	}
}

func TestLoad_envFileBeforeSubstitution(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(envPath, []byte("INJECTED=from_env\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("value: ${INJECTED}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	defer os.Unsetenv("INJECTED")

	var dst struct {
		Value string `mapstructure:"value"`
	}
	err := Load(&dst, EnvFile(envPath), Files(configPath))
	if err != nil {
		t.Fatalf("Load = %v", err)
	}
	if dst.Value != "from_env" {
		t.Errorf("value = %q, want from_env (loaded from .env)", dst.Value)
	}
}

func TestLoad_missingFile(t *testing.T) {
	var dst struct{}
	err := Load(&dst, Files("nonexistent.yaml"))
	if err == nil {
		t.Error("Load with missing file = nil, want error")
	}
}

func TestLoad_jsonFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	content := `{"port": 9000, "name": "json"}`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	var dst struct {
		Port int    `mapstructure:"port"`
		Name string `mapstructure:"name"`
	}
	err := Load(&dst, Files(path))
	if err != nil {
		t.Fatalf("Load = %v", err)
	}
	if dst.Port != 9000 || dst.Name != "json" {
		t.Errorf("port=%d name=%q, want 9000 json", dst.Port, dst.Name)
	}
}
