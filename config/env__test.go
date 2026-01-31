package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEnvFile_emptyPath(t *testing.T) {
	err := LoadEnvFile("")
	if err != nil {
		t.Errorf("LoadEnvFile(\"\") = %v, want nil", err)
	}
}

func TestLoadEnvFile_missingFile(t *testing.T) {
	err := LoadEnvFile("nonexistent.env")
	if err == nil {
		t.Error("LoadEnvFile(\"nonexistent.env\") = nil, want error")
	}
	if !os.IsNotExist(err) {
		t.Errorf("LoadEnvFile: want os.IsNotExist error, got %v", err)
	}
}

func TestLoadEnvFile_optionalMissingFile(t *testing.T) {
	err := LoadEnvFileOptional("nonexistent.env")
	if err != nil {
		t.Errorf("LoadEnvFileOptional(\"nonexistent.env\") = %v, want nil", err)
	}
}

func TestLoadEnvFile_loadsAndSetsEnv(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	content := "TEST_KEY=test_value\n"
	if err := os.WriteFile(envPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	defer os.Unsetenv("TEST_KEY")

	err := LoadEnvFile(envPath)
	if err != nil {
		t.Fatalf("LoadEnvFile(%q) = %v", envPath, err)
	}
	if got := os.Getenv("TEST_KEY"); got != "test_value" {
		t.Errorf("after LoadEnvFile, TEST_KEY = %q, want test_value", got)
	}
}
