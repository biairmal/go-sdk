// Package config provides a Viper-based configuration loader with support for
// JSON/YAML files, in-file environment variable substitution, and .env loading.
//
// Example usage:
//
//	type AppConfig struct {
//	    Handler HandlerOptions `mapstructure:"handler"`
//	    Domain  DomainOptions  `mapstructure:"domain"`
//	}
//
//	var cfg AppConfig
//	err := config.Load(&cfg,
//	    config.EnvFile(".env"),
//	    config.Files("config.yaml"),
//	)
package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// readFileAndSubstitute reads path, substitutes env vars in content, and returns
// the data plus the config type extension (e.g. "yaml", "json").
func readFileAndSubstitute(path string) (data []byte, ext string, err error) {
	data, err = os.ReadFile(path)
	if err != nil {
		return nil, "", fmt.Errorf("config: read file %q: %w", path, err)
	}
	data = SubstituteEnv(data)
	ext = strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
	if ext == "yml" {
		ext = "yaml"
	}
	return data, ext, nil
}

// applyConfigToViper either reads the first config or merges subsequent ones.
func applyConfigToViper(v *viper.Viper, data []byte, path string, initial bool) error {
	if initial {
		if err := v.ReadConfig(bytes.NewReader(data)); err != nil {
			return fmt.Errorf("config: read config %q: %w", path, err)
		}
		return nil
	}
	if err := v.MergeConfig(bytes.NewReader(data)); err != nil {
		return fmt.Errorf("config: merge config %q: %w", path, err)
	}
	return nil
}

// Load populates dst from config files and environment. Dst must be a pointer
// to a struct (possibly nested). Options control .env path and config file
// paths. Pipeline: load .env (if EnvFile set) → create Viper with AutomaticEnv
// → for each file (read → substitute ${VAR} and ${VAR:default} → ReadConfig
// or MergeConfig) → Unmarshal into dst.
//
// Config files are merged in order; later files override overlapping keys.
// Nested structs are supported via mapstructure tags (see package README).
func Load(dst interface{}, opts ...Option) error {
	o := &options{}
	for _, fn := range opts {
		fn(o)
	}

	if o.envFile != "" {
		if err := LoadEnvFileOptional(o.envFile); err != nil {
			return fmt.Errorf("config: load env file %q: %w", o.envFile, err)
		}
	}

	v := viper.New()
	v.AutomaticEnv()

	if len(o.files) == 0 {
		return v.Unmarshal(dst)
	}

	for i, path := range o.files {
		data, ext, err := readFileAndSubstitute(path)
		if err != nil {
			return err
		}
		v.SetConfigType(ext)
		if err := applyConfigToViper(v, data, path, i == 0); err != nil {
			return err
		}
	}

	if err := v.Unmarshal(dst); err != nil {
		return fmt.Errorf("config: unmarshal: %w", err)
	}
	return nil
}
