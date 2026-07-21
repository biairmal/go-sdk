package logger

import (
	"context"
)

// noopLogger implements the Logger interface with no operations.
// All logging methods are no-ops, making it suitable for testing
// or when logging needs to be completely disabled.
type noopLogger struct{}

// NewNoOp creates a new Logger that performs no operations.
// This is useful for testing to keep console output clean,
// or for disabling logging in specific environments.
//
// Example:
//
//	// In tests
//	log := logger.NewNoOp()
//	log.Info("This won't be logged") // No output
//
//	// In production code
//	var log logger.Logger
//	if cfg.DisableLogging {
//		log = logger.NewNoOp()
//	} else {
//		log = logger.NewZerolog(opts)
//	}
func NewNoOp() Logger {
	return &noopLogger{}
}

// Debug is a no-op.
func (n *noopLogger) Debug(_ string, _ ...Field) {}

// Info is a no-op.
func (n *noopLogger) Info(_ string, _ ...Field) {}

// Warn is a no-op.
func (n *noopLogger) Warn(_ string, _ ...Field) {}

// Error is a no-op.
func (n *noopLogger) Error(_ string, _ ...Field) {}

// Fatal is a no-op.
// Note: Unlike other implementations, this does not exit the program.
// If you need fatal behavior in tests, use a real logger implementation.
func (n *noopLogger) Fatal(_ string, _ ...Field) {}

// Panic is a no-op.
// Note: Unlike other implementations, this does not panic.
// If you need panic behavior in tests, use a real logger implementation.
func (n *noopLogger) Panic(_ string, _ ...Field) {}

// Debugf is a no-op.
func (n *noopLogger) Debugf(_ string, _ ...any) {}

// Infof is a no-op.
func (n *noopLogger) Infof(_ string, _ ...any) {}

// Warnf is a no-op.
func (n *noopLogger) Warnf(_ string, _ ...any) {}

// Errorf is a no-op.
func (n *noopLogger) Errorf(_ string, _ ...any) {}

// Fatalf is a no-op.
// Note: Unlike other implementations, this does not exit the program.
func (n *noopLogger) Fatalf(_ string, _ ...any) {}

// Panicf is a no-op.
// Note: Unlike other implementations, this does not panic.
func (n *noopLogger) Panicf(_ string, _ ...any) {}

// DebugWithContext is a no-op.
func (n *noopLogger) DebugWithContext(_ context.Context, _ string, _ ...Field) {}

// InfoWithContext is a no-op.
func (n *noopLogger) InfoWithContext(_ context.Context, _ string, _ ...Field) {}

// WarnWithContext is a no-op.
func (n *noopLogger) WarnWithContext(_ context.Context, _ string, _ ...Field) {}

// ErrorWithContext is a no-op.
func (n *noopLogger) ErrorWithContext(_ context.Context, _ string, _ ...Field) {}

// FatalWithContext is a no-op.
// Note: Unlike other implementations, this does not exit the program.
func (n *noopLogger) FatalWithContext(_ context.Context, _ string, _ ...Field) {}

// PanicWithContext is a no-op.
// Note: Unlike other implementations, this does not panic.
func (n *noopLogger) PanicWithContext(_ context.Context, _ string, _ ...Field) {}

// DebugfWithContext is a no-op.
func (n *noopLogger) DebugfWithContext(_ context.Context, _ string, _ ...any) {}

// InfofWithContext is a no-op.
func (n *noopLogger) InfofWithContext(_ context.Context, _ string, _ ...any) {}

// WarnfWithContext is a no-op.
func (n *noopLogger) WarnfWithContext(_ context.Context, _ string, _ ...any) {}

// ErrorfWithContext is a no-op.
func (n *noopLogger) ErrorfWithContext(_ context.Context, _ string, _ ...any) {}

// FatalfWithContext is a no-op.
// Note: Unlike other implementations, this does not exit the program.
func (n *noopLogger) FatalfWithContext(_ context.Context, _ string, _ ...any) {}

// PanicfWithContext is a no-op.
// Note: Unlike other implementations, this does not panic.
func (n *noopLogger) PanicfWithContext(_ context.Context, _ string, _ ...any) {}
