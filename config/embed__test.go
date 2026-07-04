package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/biairmal/go-sdk/config"
	"github.com/biairmal/go-sdk/logger"
	"github.com/biairmal/go-sdk/redis"
	"github.com/biairmal/go-sdk/sqlkit"
)

// appConfig mirrors how a consuming microservice embeds SDK config structs into
// its own config and loads them all from one YAML file.
type appConfig struct {
	DB     sqlkit.Config  `mapstructure:"db"`
	Redis  redis.Config   `mapstructure:"redis"`
	Logger logger.Options `mapstructure:"logger"`
}

const embedYAML = `
db:
  leader:
    driver: postgres
    host: db.internal
    port: 5432
    database: orders
    username: app
    password: secret
    ssl_mode: require
    connect_timeout: 5s
    max_retries: 3
  followers:
    - driver: postgres
      host: replica1.internal
      port: 5432
      database: orders
  pool:
    max_open_conns: 25
    max_idle_conns: 5
    conn_max_lifetime: 5m
    conn_max_idle_time: 1m
  health:
    enabled: true
    check_interval: 30s
    timeout: 5s
redis:
  mode: sentinel
  master_name: mymaster
  sentinel_addrs:
    - sentinel1:26379
    - sentinel2:26379
  db: 0
  dial_timeout: 5s
  pool_size: 10
logger:
  level: debug
  output: stdout
  format: json
  rotation:
    filename: app.log
    max_size: 100
    compress: true
`

// TestEmbeddedSDKConfigRoundTrip proves that SDK config structs embed into an app
// config and populate from snake_case YAML via config.Load — including nested
// structs, time.Duration strings, and slices — with no custom decode wiring.
func TestEmbeddedSDKConfigRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(embedYAML), 0o600); err != nil {
		t.Fatal(err)
	}

	var cfg appConfig
	if err := config.Load(&cfg, config.Files(path)); err != nil {
		t.Fatalf("config.Load = %v", err)
	}

	checks := []struct {
		name string
		got  any
		want any
	}{
		// sqlkit — nested struct + snake_case + duration
		{"db.leader.driver", cfg.DB.Leader.Driver, "postgres"},
		{"db.leader.host", cfg.DB.Leader.Host, "db.internal"},
		{"db.leader.port", cfg.DB.Leader.Port, 5432},
		{"db.leader.ssl_mode", cfg.DB.Leader.SSLMode, "require"},
		{"db.leader.connect_timeout", cfg.DB.Leader.ConnectTimeout, 5 * time.Second},
		{"db.leader.max_retries", cfg.DB.Leader.MaxRetries, 3},
		// sqlkit — slice of structs
		{"db.followers len", len(cfg.DB.Followers), 1},
		{"db.followers[0].host", firstFollowerHost(cfg.DB.Followers), "replica1.internal"},
		// sqlkit — pool + health (multi-word snake_case + durations)
		{"db.pool.max_open_conns", cfg.DB.Pool.MaxOpenConns, 25},
		{"db.pool.conn_max_lifetime", cfg.DB.Pool.ConnMaxLifetime, 5 * time.Minute},
		{"db.health.enabled", cfg.DB.Health.Enabled, true},
		{"db.health.check_interval", cfg.DB.Health.CheckInterval, 30 * time.Second},
		// redis — enum, slice, duration
		{"redis.mode", cfg.Redis.Mode, redis.ModeSentinel},
		{"redis.master_name", cfg.Redis.MasterName, "mymaster"},
		{"redis.sentinel_addrs len", len(cfg.Redis.SentinelAddrs), 2},
		{"redis.sentinel_addrs[0]", firstOrEmpty(cfg.Redis.SentinelAddrs), "sentinel1:26379"},
		{"redis.dial_timeout", cfg.Redis.DialTimeout, 5 * time.Second},
		{"redis.pool_size", cfg.Redis.PoolSize, 10},
		// logger — string enums + nested rotation
		{"logger.level", cfg.Logger.Level, logger.LevelDebug},
		{"logger.output", cfg.Logger.Output, logger.OutputStdout},
		{"logger.format", cfg.Logger.Format, logger.FormatJSON},
		{"logger.rotation.filename", rotationFilename(cfg.Logger.Rotation), "app.log"},
		{"logger.rotation.max_size", rotationMaxSize(cfg.Logger.Rotation), 100},
		{"logger.rotation.compress", rotationCompress(cfg.Logger.Rotation), true},
	}

	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			if c.got != c.want {
				t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
			}
		})
	}
}

func firstFollowerHost(fs []sqlkit.DBConfig) string {
	if len(fs) == 0 {
		return ""
	}
	return fs[0].Host
}

func firstOrEmpty(s []string) string {
	if len(s) == 0 {
		return ""
	}
	return s[0]
}

func rotationFilename(r *logger.RotationConfig) string {
	if r == nil {
		return ""
	}
	return r.Filename
}

func rotationMaxSize(r *logger.RotationConfig) int {
	if r == nil {
		return 0
	}
	return r.MaxSize
}

func rotationCompress(r *logger.RotationConfig) bool {
	if r == nil {
		return false
	}
	return r.Compress
}
