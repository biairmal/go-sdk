package redis

import (
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	tests := []struct {
		name    string
		checkFn func(*Config) interface{}
		wantVal interface{}
	}{
		{
			name: "mode is standalone",
			checkFn: func(c *Config) interface{} {
				return c.Mode
			},
			wantVal: ModeStandalone,
		},
		{
			name: "address is localhost:6379",
			checkFn: func(c *Config) interface{} {
				return c.Address
			},
			wantVal: "localhost:6379",
		},
		{
			name: "db is zero",
			checkFn: func(c *Config) interface{} {
				return c.DB
			},
			wantVal: 0,
		},
		{
			name: "username is default",
			checkFn: func(c *Config) interface{} {
				return c.Username
			},
			wantVal: "default",
		},
		{
			name: "pool size is 10",
			checkFn: func(c *Config) interface{} {
				return c.PoolSize
			},
			wantVal: 10,
		},
		{
			name: "min idle conns is 5",
			checkFn: func(c *Config) interface{} {
				return c.MinIdleConns
			},
			wantVal: 5,
		},
		{
			name: "max retries is 3",
			checkFn: func(c *Config) interface{} {
				return c.MaxRetries
			},
			wantVal: 3,
		},
		{
			name: "dial timeout is 5s",
			checkFn: func(c *Config) interface{} {
				return c.DialTimeout
			},
			wantVal: 5 * time.Second,
		},
		{
			name: "read timeout is 3s",
			checkFn: func(c *Config) interface{} {
				return c.ReadTimeout
			},
			wantVal: 3 * time.Second,
		},
		{
			name: "write timeout is 3s",
			checkFn: func(c *Config) interface{} {
				return c.WriteTimeout
			},
			wantVal: 3 * time.Second,
		},
		{
			name: "pool timeout is 4s",
			checkFn: func(c *Config) interface{} {
				return c.PoolTimeout
			},
			wantVal: 4 * time.Second,
		},
		{
			name: "max redirects is 8",
			checkFn: func(c *Config) interface{} {
				return c.MaxRedirects
			},
			wantVal: 8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			got := tt.checkFn(cfg)
			if got != tt.wantVal {
				t.Errorf("DefaultConfig().%s = %v, want %v", tt.name, got, tt.wantVal)
			}
		})
	}
}

func TestMode_constants(t *testing.T) {
	tests := []struct {
		name string
		m    Mode
		want string
	}{
		{"standalone", ModeStandalone, "standalone"},
		{"sentinel", ModeSentinel, "sentinel"},
		{"cluster", ModeCluster, "cluster"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := string(tt.m); got != tt.want {
				t.Errorf("Mode = %q, want %q", got, tt.want)
			}
		})
	}
}
