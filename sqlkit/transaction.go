package sqlkit

import (
	"context"
	"database/sql"
	"fmt"
)

// txKey is an empty struct used as context key for transaction injection.
type txKey struct{}

// TxFunc is a function type for transaction execution.
type TxFunc func(ctx context.Context) error

// InjectTx injects a transaction into the context.
// Use case: Called internally by WithTransaction.
func InjectTx(ctx context.Context, tx *sql.Tx) context.Context {
	return context.WithValue(ctx, txKey{}, tx)
}

// ExtractTx extracts a transaction from the context if present.
// Use case: Called by repositories to detect if they're in a transaction.
// Returns the transaction and true if found, nil and false otherwise.
func ExtractTx(ctx context.Context) (*sql.Tx, bool) {
	tx, ok := ctx.Value(txKey{}).(*sql.Tx)
	return tx, ok
}

// WithTransaction executes a function within a transaction with default options.
// Begins transaction on leader with default options.
// Injects transaction into context.
// If function returns error: rollback and return error.
// If function panics: rollback, then re-panic.
// If function succeeds: commit and return nil.
func (db *DB) WithTransaction(ctx context.Context, fn TxFunc) error {
	return db.WithTransactionOptions(ctx, nil, fn)
}

// WithTransactionOptions executes a function within a transaction with custom options.
// Same as WithTransaction but uses provided options.
func (db *DB) WithTransactionOptions(ctx context.Context, opts *sql.TxOptions, fn TxFunc) error {
	// Check if already in a transaction
	if _, ok := ExtractTx(ctx); ok {
		return fmt.Errorf("sqlkit: nested transaction detected")
	}

	// Begin transaction on leader
	tx, err := db.Leader().BeginTx(ctx, opts)
	if err != nil {
		return fmt.Errorf("sqlkit: failed to begin transaction: %w", err)
	}

	// Inject transaction into context
	txCtx := InjectTx(ctx, tx)

	// Execute function with panic recovery
	var fnErr error
	panicked := true
	defer func() {
		switch {
		case panicked:
			// Rollback on panic
			if rbErr := tx.Rollback(); rbErr != nil {
				// Combine panic and rollback error if possible
				// Re-panic with original panic value
				panic(fmt.Errorf("sqlkit: transaction panic and rollback failed: %w", rbErr))
			}
		case fnErr != nil:
			// Rollback on function error
			if rbErr := tx.Rollback(); rbErr != nil {
				fnErr = fmt.Errorf("sqlkit: transaction error: %w, rollback error: %w", fnErr, rbErr)
			}
		default:
			// Commit on success
			if commitErr := tx.Commit(); commitErr != nil {
				fnErr = fmt.Errorf("sqlkit: commit failed: %w", commitErr)
			}
		}
	}()

	// Execute function
	fnErr = fn(txCtx)
	panicked = false

	return fnErr
}

// WithReadOnlyTransaction executes a read-only transaction on a follower.
// Uses follower, not leader.
// Still requires commit (even for read-only).
// Automatically falls back to leader if no healthy followers.
func (db *DB) WithReadOnlyTransaction(ctx context.Context, fn TxFunc) error {
	opts := &sql.TxOptions{
		ReadOnly: true,
	}

	// Check if already in a transaction
	if _, ok := ExtractTx(ctx); ok {
		return fmt.Errorf("sqlkit: nested transaction detected")
	}

	// Begin transaction on follower (falls back to leader if no healthy followers)
	followerDB := db.Follower()
	tx, err := followerDB.BeginTx(ctx, opts)
	if err != nil {
		return fmt.Errorf("sqlkit: failed to begin read-only transaction: %w", err)
	}

	// Inject transaction into context
	txCtx := InjectTx(ctx, tx)

	// Execute function with panic recovery
	var fnErr error
	panicked := true
	defer func() {
		switch {
		case panicked:
			// Rollback on panic
			if rbErr := tx.Rollback(); rbErr != nil {
				panic(fmt.Errorf("sqlkit: read-only transaction panic and rollback failed: %w", rbErr))
			}
		case fnErr != nil:
			// Rollback on function error
			if rbErr := tx.Rollback(); rbErr != nil {
				fnErr = fmt.Errorf("sqlkit: read-only transaction error: %w, rollback error: %w", fnErr, rbErr)
			}
		default:
			// Commit on success (required even for read-only)
			if commitErr := tx.Commit(); commitErr != nil {
				fnErr = fmt.Errorf("sqlkit: read-only transaction commit failed: %w", commitErr)
			}
		}
	}()

	// Execute function
	fnErr = fn(txCtx)
	panicked = false

	return fnErr
}
