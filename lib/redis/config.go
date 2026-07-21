package redis

import "time"

type Mode string

const (
	ModeStandalone Mode = "standalone"
	ModeSentinel   Mode = "sentinel"
	ModeCluster    Mode = "cluster"
)

type Config struct {
	Mode Mode `mapstructure:"mode"`

	// Standalone configuration
	Address  string `mapstructure:"address"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"` // Note: Cluster mode doesn't support DB selection (always 0)

	// Sentinel configuration
	MasterName       string   `mapstructure:"master_name"`
	SentinelAddrs    []string `mapstructure:"sentinel_addrs"`
	SentinelPassword string   `mapstructure:"sentinel_password"` // Password for sentinel instances themselves

	// Cluster configuration
	ClusterAddrs []string `mapstructure:"cluster_addrs"`

	// TLS configuration (all modes)
	TLSEnabled bool   `mapstructure:"tls_enabled"`
	TLSCert    string `mapstructure:"tls_cert"`
	TLSKey     string `mapstructure:"tls_key"`
	TLSCA      string `mapstructure:"tls_ca"`

	// ACL authentication (Redis 6+)
	Username string `mapstructure:"username"` // default is "default"

	// Common settings
	PoolSize     int           `mapstructure:"pool_size"`
	MinIdleConns int           `mapstructure:"min_idle_conns"`
	MaxRetries   int           `mapstructure:"max_retries"`
	DialTimeout  time.Duration `mapstructure:"dial_timeout"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	PoolTimeout  time.Duration `mapstructure:"pool_timeout"`

	// Cluster specific
	ReadOnly       bool `mapstructure:"read_only"`        // Allow reads from replicas
	RouteByLatency bool `mapstructure:"route_by_latency"` // Route to lowest latency node
	RouteRandomly  bool `mapstructure:"route_randomly"`   // Distribute reads randomly
	MaxRedirects   int  `mapstructure:"max_redirects"`    // Max MOVED/ASK redirects
}

func DefaultConfig() *Config {
	return &Config{
		Mode:         ModeStandalone,
		Address:      "localhost:6379",
		DB:           0,
		Username:     "default",
		PoolSize:     10,
		MinIdleConns: 5,
		MaxRetries:   3,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolTimeout:  4 * time.Second,
		MaxRedirects: 8,
	}
}
