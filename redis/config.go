package redis

import "time"

type Mode string

const (
	ModeStandalone Mode = "standalone"
	ModeSentinel   Mode = "sentinel"
	ModeCluster    Mode = "cluster"
)

type Config struct {
	Mode Mode

	// Standalone configuration
	Address  string
	Password string
	DB       int // Note: Cluster mode doesn't support DB selection (always 0)

	// Sentinel configuration
	MasterName       string
	SentinelAddrs    []string
	SentinelPassword string // Password for sentinel instances themselves

	// Cluster configuration
	ClusterAddrs []string

	// TLS configuration (all modes)
	TLSEnabled bool
	TLSCert    string
	TLSKey     string
	TLSCA      string

	// ACL authentication (Redis 6+)
	Username string // default is "default"

	// Common settings
	PoolSize     int
	MinIdleConns int
	MaxRetries   int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	PoolTimeout  time.Duration

	// Cluster specific
	ReadOnly       bool // Allow reads from replicas
	RouteByLatency bool // Route to lowest latency node
	RouteRandomly  bool // Distribute reads randomly
	MaxRedirects   int  // Max MOVED/ASK redirects
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
