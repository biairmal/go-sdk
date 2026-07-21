package sqlkit

import (
	"context"
	"database/sql"
	"time"
)

// Health represents the overall health status of database connections.
type Health struct {
	Leader    ConnectionHealth   // Leader connection health
	Followers []ConnectionHealth // Follower connections health
}

// ConnectionHealth represents the health status of a single connection.
type ConnectionHealth struct {
	Healthy      bool          // Is connection healthy
	LastCheck    time.Time     // Last health check timestamp
	Error        string        // Error message if unhealthy (optional)
	ResponseTime time.Duration // Last ping response time
}

// GetHealth returns current health status of all connections.
// Thread-safe.
func (db *DB) GetHealth() Health {
	db.healthMu.RLock()
	defer db.healthMu.RUnlock()

	health := Health{
		Leader:    db.leaderHealth,
		Followers: make([]ConnectionHealth, len(db.followers)),
	}

	// Get follower health statuses
	for i := range db.followers {
		if followerHealth, ok := db.followerHealthMap[i]; ok {
			health.Followers[i] = followerHealth
		} else {
			health.Followers[i] = ConnectionHealth{Healthy: false}
		}
	}

	return health
}

// IsHealthy returns true if leader is healthy.
// Thread-safe.
func (db *DB) IsHealthy() bool {
	db.healthMu.RLock()
	defer db.healthMu.RUnlock()
	return db.leaderHealth.Healthy
}

// runHealthChecks is a background goroutine that performs periodic health checks.
// Should be started as goroutine in New().
// Must respect context cancellation.
func (db *DB) runHealthChecks() {
	ticker := time.NewTicker(db.config.Health.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-db.ctx.Done():
			return
		case <-ticker.C:
			db.checkHealth()
		}
	}
}

// checkHealth performs health check on all connections.
// Uses PingContext with timeout.
// Updates health atomically.
func (db *DB) checkHealth() {
	ctx, cancel := context.WithTimeout(db.ctx, db.config.Health.Timeout)
	defer cancel()

	now := time.Now()

	// Check leader
	start := time.Now()
	leaderHealthy := db.ping(ctx, db.leader)
	leaderResponseTime := time.Since(start)

	var leaderError string
	if !leaderHealthy {
		leaderError = "ping failed"
	}

	db.healthMu.Lock()
	db.leaderHealth = ConnectionHealth{
		Healthy:      leaderHealthy,
		LastCheck:    now,
		Error:        leaderError,
		ResponseTime: leaderResponseTime,
	}
	db.healthMu.Unlock()

	// Check followers
	db.healthMu.Lock()
	for i, follower := range db.followers {
		if follower == nil {
			db.followerHealthMap[i] = ConnectionHealth{
				Healthy:   false,
				LastCheck: now,
				Error:     "connection is nil",
			}
			continue
		}

		start := time.Now()
		followerHealthy := db.ping(ctx, follower)
		followerResponseTime := time.Since(start)

		var followerError string
		if !followerHealthy {
			followerError = "ping failed"
		}

		db.followerHealthMap[i] = ConnectionHealth{
			Healthy:      followerHealthy,
			LastCheck:    now,
			Error:        followerError,
			ResponseTime: followerResponseTime,
		}
	}
	db.healthMu.Unlock()
}

// ping pings a single connection to check health.
// Returns true if ping succeeds, false otherwise.
func (db *DB) ping(ctx context.Context, conn *sql.DB) bool {
	if conn == nil {
		return false
	}
	if err := conn.PingContext(ctx); err != nil {
		return false
	}
	return true
}
