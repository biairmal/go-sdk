package sqlkit

import (
	"database/sql"
	"errors"
)

var (
	// ErrNoConnection indicates no database connection is available.
	ErrNoConnection = errors.New("sqlkit: no database connection")

	// ErrLeaderUnhealthy indicates the leader database is unhealthy.
	ErrLeaderUnhealthy = errors.New("sqlkit: leader database unhealthy")

	// ErrAllFollowersDown indicates all follower databases are down.
	ErrAllFollowersDown = errors.New("sqlkit: all follower databases down")

	// ErrInvalidConfig indicates invalid configuration.
	ErrInvalidConfig = errors.New("sqlkit: invalid configuration")

	// ErrTransactionFailed indicates a transaction failed.
	ErrTransactionFailed = errors.New("sqlkit: transaction failed")
)

// IsNoRows checks if error is sql.ErrNoRows.
// Use case: Repository layer to distinguish "not found" from other errors.
func IsNoRows(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}
