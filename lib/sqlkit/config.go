package sqlkit

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Config is the main configuration struct for sqlkit.
type Config struct {
	Leader    DBConfig     `mapstructure:"leader"`    // Leader (write) database configuration
	Followers []DBConfig   `mapstructure:"followers"` // Follower (read) database configurations (optional)
	Pool      PoolConfig   `mapstructure:"pool"`      // Connection pool settings
	Health    HealthConfig `mapstructure:"health"`    // Health check settings
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
	if !validSSLMode(c.Leader.SSLMode) {
		return fmt.Errorf("%w: leader ssl_mode %q must be one of %s",
			ErrInvalidConfig, c.Leader.SSLMode, sslModeList)
	}
	for i := range c.Followers {
		if !validSSLMode(c.Followers[i].SSLMode) {
			return fmt.Errorf("%w: follower %d ssl_mode %q must be one of %s",
				ErrInvalidConfig, i, c.Followers[i].SSLMode, sslModeList)
		}
	}
	return nil
}

// DBConfig is the configuration for a single database connection.
type DBConfig struct {
	Driver         string        `mapstructure:"driver"`          // Database driver: "postgres", "mysql", "sqlite3"
	Host           string        `mapstructure:"host"`            // Database host
	Port           int           `mapstructure:"port"`            // Database port
	Database       string        `mapstructure:"database"`        // Database name
	Username       string        `mapstructure:"username"`        // Database username
	Password       string        `mapstructure:"password"`        // Database password
	SSLMode        string        `mapstructure:"ssl_mode"`        // SSL mode: disable|require|verify-ca|verify-full
	ConnectTimeout time.Duration `mapstructure:"connect_timeout"` // Connection timeout (default: 5s)
	MaxRetries     int           `mapstructure:"max_retries"`     // Maximum connection retry attempts (default: 3)
}

// SSL modes accepted in DBConfig.SSLMode. They are the libpq names; for MySQL they map to the driver's tls
// parameter (see mysqlTLS).
const (
	SSLModeDisable    = "disable"
	SSLModeAllow      = "allow"
	SSLModePrefer     = "prefer"
	SSLModeRequire    = "require"
	SSLModeVerifyCA   = "verify-ca"
	SSLModeVerifyFull = "verify-full"
)

// sslModeList names the accepted modes in error messages.
const sslModeList = "disable, allow, prefer, require, verify-ca or verify-full"

// validSSLMode reports whether mode is empty (driver default) or one of the SSLMode constants.
func validSSLMode(mode string) bool {
	switch mode {
	case "", SSLModeDisable, SSLModeAllow, SSLModePrefer, SSLModeRequire, SSLModeVerifyCA, SSLModeVerifyFull:
		return true
	}
	return false
}

// libpqQuote returns v as a single-quoted libpq connection value, escaping backslashes and quotes. lib/pq and
// pgx read the key=value DSN literally (they do not URL-decode), so a value with a space, quote or backslash
// is only safe when quoted like this.
func libpqQuote(v string) string {
	return "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(v) + "'"
}

// mysqlTLS maps an SSLMode onto go-sql-driver's tls parameter. "" leaves the driver default.
func mysqlTLS(mode string) string {
	switch mode {
	case SSLModeDisable:
		return "false"
	case SSLModeAllow, SSLModePrefer:
		return "preferred"
	case SSLModeRequire:
		return "skip-verify" // encrypted, certificate not checked: the libpq meaning of "require"
	case SSLModeVerifyCA, SSLModeVerifyFull:
		return "true" // encrypted, certificate chain and host name checked
	}
	return ""
}

// DSN generates a database-specific connection string.
// Supports PostgreSQL and MySQL at minimum.
//
// The password is written as the driver expects it, never URL-encoded: PostgreSQL values are libpq-quoted
// and the MySQL password is passed as is.
func (c *DBConfig) DSN() string {
	switch c.Driver {
	case "postgres":
		timeoutSeconds := int(c.ConnectTimeout.Seconds())
		if timeoutSeconds == 0 {
			timeoutSeconds = 5 // default
		}
		parts := []string{
			"host=" + libpqQuote(c.Host),
			"port=" + strconv.Itoa(c.Port),
			"user=" + libpqQuote(c.Username),
			"password=" + libpqQuote(c.Password),
			"dbname=" + libpqQuote(c.Database),
		}
		if c.SSLMode != "" {
			parts = append(parts, "sslmode="+libpqQuote(c.SSLMode))
		}
		parts = append(parts, "connect_timeout="+strconv.Itoa(timeoutSeconds))
		return strings.Join(parts, " ")
	case "mysql":
		timeoutStr := c.ConnectTimeout.String()
		if timeoutStr == "0s" {
			timeoutStr = "5s" // default
		}
		dsn := fmt.Sprintf(
			"%s:%s@tcp(%s:%d)/%s?parseTime=true&timeout=%s",
			c.Username, c.Password, c.Host, c.Port, c.Database, timeoutStr)
		if tls := mysqlTLS(c.SSLMode); tls != "" {
			dsn += "&tls=" + tls
		}
		return dsn
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
			c.Driver, c.Host, c.Port, c.Database, c.Username, url.QueryEscape(c.Password), timeoutSeconds)
	}
}

// PoolConfig is the connection pool configuration.
type PoolConfig struct {
	MaxOpenConns    int           `mapstructure:"max_open_conns"`     // Maximum open connections (default: 25)
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`     // Maximum idle connections (default: 5)
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`  // Maximum connection lifetime (default: 5m)
	ConnMaxIdleTime time.Duration `mapstructure:"conn_max_idle_time"` // Maximum connection idle time (default: 1m)
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
	Enabled       bool          `mapstructure:"enabled"`        // Enable health checks (default: true)
	CheckInterval time.Duration `mapstructure:"check_interval"` // Health check interval (default: 30s)
	Timeout       time.Duration `mapstructure:"timeout"`        // Health check timeout (default: 5s)
}

// DefaultHealthConfig returns a HealthConfig with default values.
func DefaultHealthConfig() HealthConfig {
	return HealthConfig{
		Enabled:       true,
		CheckInterval: 30 * time.Second,
		Timeout:       5 * time.Second,
	}
}
