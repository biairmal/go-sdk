package sqlkit

import (
	"fmt"
	"net/url"
	"time"
)

// Config is the main configuration struct for sqlkit.
type Config struct {
	Leader    DBConfig     // Leader (write) database configuration
	Followers []DBConfig   // Follower (read) database configurations (optional)
	Pool      PoolConfig   // Connection pool settings
	Health    HealthConfig // Health check settings
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	if c == nil {
		return fmt.Errorf("%w: config is required", ErrInvalidConfig)
	}
	if c.Leader.Driver == "" {
		return fmt.Errorf("%w: leader driver is required", ErrInvalidConfig)
	}
	if c.Leader.Host == "" {
		return fmt.Errorf("%w: leader host is required", ErrInvalidConfig)
	}
	if c.Leader.Database == "" {
		return fmt.Errorf("%w: leader database is required", ErrInvalidConfig)
	}
	return nil
}

// DBConfig is the configuration for a single database connection.
type DBConfig struct {
	Driver         string        // Database driver: "postgres", "mysql", "sqlite3"
	Host           string        // Database host
	Port           int           // Database port
	Database       string        // Database name
	Username       string        // Database username
	Password       string        // Database password
	SSLMode        string        // SSL mode: "disable", "require", "verify-ca", "verify-full" (postgres)
	ConnectTimeout time.Duration // Connection timeout (default: 5s)
	MaxRetries     int           // Maximum connection retry attempts (default: 3)
}

// DSN generates a database-specific connection string.
// Supports PostgreSQL and MySQL at minimum.
// Handles URL encoding for special characters in password.
func (c *DBConfig) DSN() string {
	// URL encode password to handle special characters
	encodedPassword := url.QueryEscape(c.Password)

	switch c.Driver {
	case "postgres":
		timeoutSeconds := int(c.ConnectTimeout.Seconds())
		if timeoutSeconds == 0 {
			timeoutSeconds = 5 // default
		}
		return fmt.Sprintf(
			"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s connect_timeout=%d",
			c.Host, c.Port, c.Username, encodedPassword, c.Database, c.SSLMode, timeoutSeconds)
	case "mysql":
		timeoutStr := c.ConnectTimeout.String()
		if timeoutStr == "0s" {
			timeoutStr = "5s" // default
		}
		return fmt.Sprintf(
			"%s:%s@tcp(%s:%d)/%s?parseTime=true&timeout=%s",
			c.Username, encodedPassword, c.Host, c.Port, c.Database, timeoutStr)
	case "sqlite3":
		timeoutMs := int(c.ConnectTimeout.Milliseconds())
		if timeoutMs == 0 {
			timeoutMs = 5000 // default 5 seconds
		}
		return fmt.Sprintf(
			"file:%s?mode=rwc&cache=shared&_busy_timeout=%d",
			c.Database, timeoutMs)
	default:
		timeoutSeconds := int(c.ConnectTimeout.Seconds())
		if timeoutSeconds == 0 {
			timeoutSeconds = 5 // default
		}
		return fmt.Sprintf(
			"driver=%s;host=%s;port=%d;database=%s;user id=%s;password=%s;connect timeout=%d",
			c.Driver, c.Host, c.Port, c.Database, c.Username, encodedPassword, timeoutSeconds)
	}
}

// PoolConfig is the connection pool configuration.
type PoolConfig struct {
	MaxOpenConns    int           // Maximum open connections (default: 25)
	MaxIdleConns    int           // Maximum idle connections (default: 5)
	ConnMaxLifetime time.Duration // Maximum connection lifetime (default: 5m)
	ConnMaxIdleTime time.Duration // Maximum connection idle time (default: 1m)
}

// DefaultPoolConfig returns a PoolConfig with default values.
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 1 * time.Minute,
	}
}

// HealthConfig is the health check configuration.
type HealthConfig struct {
	Enabled       bool          // Enable health checks (default: true)
	CheckInterval time.Duration // Health check interval (default: 30s)
	Timeout       time.Duration // Health check timeout (default: 5s)
}

// DefaultHealthConfig returns a HealthConfig with default values.
func DefaultHealthConfig() HealthConfig {
	return HealthConfig{
		Enabled:       true,
		CheckInterval: 30 * time.Second,
		Timeout:       5 * time.Second,
	}
}
