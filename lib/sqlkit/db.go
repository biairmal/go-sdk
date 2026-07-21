package sqlkit

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"
)

// DB is the main database wrapper that manages leader and follower connections.
type DB struct {
	// Private fields
	leader    *sql.DB
	followers []*sql.DB
	config    Config
	driver    string

	// Round-robin for follower selection
	followerIdx int
	followerMu  sync.Mutex

	// Health tracking
	healthMu          sync.RWMutex
	leaderHealth      ConnectionHealth
	followerHealthMap map[int]ConnectionHealth

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
}

// New creates and initializes a new DB instance.
// Validates configuration (at least leader must be configured).
// Initializes leader connection.
// Initializes follower connections (non-blocking if followers fail).
// Configures connection pools for all connections.
// Starts health check goroutine if enabled.
// Returns error only if leader connection fails.
func New(ctx context.Context, cfg *Config) (*DB, error) {
	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidConfig, err)
	}

	// Apply defaults
	if cfg.Pool.MaxOpenConns == 0 {
		cfg.Pool = DefaultPoolConfig()
	}
	if cfg.Health.CheckInterval == 0 {
		cfg.Health = DefaultHealthConfig()
	}

	// Create context with cancellation for health checks
	ctxWithCancel, cancel := context.WithCancel(ctx)

	db := &DB{
		config:            *cfg,
		driver:            cfg.Leader.Driver,
		followerHealthMap: make(map[int]ConnectionHealth),
		leaderHealth:      ConnectionHealth{Healthy: false},
		ctx:               ctxWithCancel,
		cancel:            cancel,
	}

	// Initialize leader connection (required)
	if err := db.initLeader(); err != nil {
		cancel()
		return nil, fmt.Errorf("sqlkit: failed to initialize leader: %w", err)
	}

	// Initialize follower connections (optional, non-blocking)
	db.initFollowers()

	// Start health check goroutine if enabled
	if cfg.Health.Enabled {
		go db.runHealthChecks()
	}

	return db, nil
}

// Leader returns the leader (write) database connection.
// Thread-safe.
// Always returns non-nil if DB was successfully created.
// Use cases: Write operations (INSERT, UPDATE, DELETE), transactions that modify data,
// operations requiring strong consistency.
func (db *DB) Leader() *sql.DB {
	return db.leader
}

// Follower returns a follower (read) database connection using round-robin load balancing.
// If no followers configured, returns leader.
// Uses round-robin to select next follower.
// Checks if selected follower is healthy.
// If unhealthy, tries next follower (up to len(followers) attempts).
// If all followers unhealthy, falls back to leader.
// Thread-safe.
// Use cases: Read operations (SELECT), analytics queries, report generation,
// any operation that can tolerate eventual consistency.
func (db *DB) Follower() *sql.DB {
	// If no followers configured, return leader
	if len(db.followers) == 0 {
		return db.leader
	}

	db.followerMu.Lock()
	defer db.followerMu.Unlock()

	// Try to find a healthy follower using round-robin
	attempts := len(db.followers)
	startIdx := db.followerIdx

	for i := 0; i < attempts; i++ {
		idx := (startIdx + i) % len(db.followers)
		db.followerIdx = (idx + 1) % len(db.followers) // Advance for next call

		// Check if follower is healthy
		db.healthMu.RLock()
		followerHealth, ok := db.followerHealthMap[idx]
		healthy := ok && followerHealth.Healthy
		db.healthMu.RUnlock()

		if healthy && db.followers[idx] != nil {
			return db.followers[idx]
		}
	}

	// All followers unhealthy, fall back to leader
	return db.leader
}

// Driver returns the database driver name.
// Returns: "postgres", "mysql", "sqlite3", etc.
func (db *DB) Driver() string {
	return db.driver
}

// Close closes all database connections and stops health checks.
// Cancels context (stops health checks).
// Closes leader connection.
// Closes all follower connections.
// Collects and returns any errors.
// Thread-safe.
func (db *DB) Close() error {
	var errs []error

	// Cancel context (stops health checks)
	if db.cancel != nil {
		db.cancel()
	}

	// Close leader connection
	if db.leader != nil {
		if err := db.leader.Close(); err != nil {
			errs = append(errs, fmt.Errorf("leader close error: %w", err))
		}
	}

	// Close all follower connections
	for i, follower := range db.followers {
		if follower != nil {
			if err := follower.Close(); err != nil {
				errs = append(errs, fmt.Errorf("follower %d close error: %w", i, err))
			}
		}
	}

	// Return combined error if any
	if len(errs) > 0 {
		return fmt.Errorf("sqlkit: errors during close: %v", errs)
	}

	return nil
}

// initLeader initializes leader database connection.
// Opens connection using driver and DSN.
// Pings to verify connectivity.
// Configures connection pool.
// Sets leaderHealthy = true.
// Returns error on failure.
func (db *DB) initLeader() error {
	conn, err := db.connect(&db.config.Leader)
	if err != nil {
		return err
	}

	db.leader = conn
	db.healthMu.Lock()
	db.leaderHealth = ConnectionHealth{
		Healthy:   true,
		LastCheck: time.Now(),
	}
	db.healthMu.Unlock()

	return nil
}

// initFollowers initializes follower database connections.
// Iterates through follower configs.
// For each follower, attempts connection.
// If connection fails, logs warning but continues (don't fail).
// Adds successful connections to followers slice.
// Initializes health map for each follower.
// Never returns error (followers are optional).
func (db *DB) initFollowers() {
	if len(db.config.Followers) == 0 {
		db.followers = []*sql.DB{}
		return
	}

	db.followers = make([]*sql.DB, 0, len(db.config.Followers))

	for i, followerConfig := range db.config.Followers {
		conn, err := db.connect(&followerConfig)
		if err != nil {
			log.Printf("sqlkit: warning: failed to connect to follower %d: %v", i, err)
			// Continue to next follower
			continue
		}

		idx := len(db.followers)
		db.followers = append(db.followers, conn)
		db.healthMu.Lock()
		db.followerHealthMap[idx] = ConnectionHealth{
			Healthy:   true,
			LastCheck: time.Now(),
		}
		db.healthMu.Unlock()
	}
}

// connect creates a database connection from config.
// Calls sql.Open(cfg.Driver, cfg.DSN()).
// Creates context with ConnectTimeout.
// Pings database to verify connection.
// Configures connection pool settings.
// Returns connection or error.
// Must validate connection before returning.
// Should retry on transient errors (up to MaxRetries).
// Closes connection on validation failure.
func (db *DB) connect(cfg *DBConfig) (*sql.DB, error) {
	if cfg == nil {
		return nil, fmt.Errorf("%w: config is required", ErrInvalidConfig)
	}
	// Set defaults
	connectTimeout := cfg.ConnectTimeout
	if connectTimeout == 0 {
		connectTimeout = 5 * time.Second
	}
	maxRetries := cfg.MaxRetries
	if maxRetries == 0 {
		maxRetries = 3
	}

	var conn *sql.DB
	var err error

	// Retry connection up to MaxRetries times
	for attempt := 0; attempt < maxRetries; attempt++ {
		conn, err = sql.Open(db.driver, cfg.DSN())
		if err != nil {
			if attempt < maxRetries-1 {
				time.Sleep(time.Duration(attempt+1) * 100 * time.Millisecond) // Exponential backoff
				continue
			}
			return nil, fmt.Errorf("sqlkit: failed to open connection after %d attempts: %w", maxRetries, err)
		}

		// Ping with timeout to verify connection
		pingCtx, cancel := context.WithTimeout(context.Background(), connectTimeout)
		err = conn.PingContext(pingCtx)
		cancel()

		if err != nil {
			conn.Close() // Close failed connection
			if attempt < maxRetries-1 {
				time.Sleep(time.Duration(attempt+1) * 100 * time.Millisecond)
				continue
			}
			return nil, fmt.Errorf("sqlkit: failed to ping connection after %d attempts: %w", maxRetries, err)
		}

		// Connection successful, configure pool
		pool := db.config.Pool
		if pool.MaxOpenConns == 0 {
			pool = DefaultPoolConfig()
		}

		conn.SetMaxOpenConns(pool.MaxOpenConns)
		conn.SetMaxIdleConns(pool.MaxIdleConns)
		conn.SetConnMaxLifetime(pool.ConnMaxLifetime)
		conn.SetConnMaxIdleTime(pool.ConnMaxIdleTime)

		return conn, nil
	}

	return nil, fmt.Errorf("sqlkit: connection failed after %d retries", maxRetries)
}
