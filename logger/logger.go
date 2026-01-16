// Package logger provides a structured logging interface with support for
// multiple backends, context extraction, and file rotation.
//
// The package defines a Logger interface that can be implemented by various
// logging backends. Currently, it provides:
//   - zerolog implementation (NewZerolog) for production use
//   - no-op implementation (NewNoOp) for testing or disabling logging
//
// Example usage:
//
//	log := logger.NewZerolog(&logger.Options{
//		Level:  logger.LevelInfo,
//		Output: logger.OutputStdout,
//		Format: logger.FormatText,
//	})
//
//	log.Info("Application started", logger.F("port", 8080))
//
// With context:
//
//	ctx := context.WithValue(context.Background(), "request_id", "req-123")
//	log.InfoWithContext(ctx, "Request processed", logger.F("status", "success"))
//
// For testing (no console output):
//
//	log := logger.NewNoOp()
//	log.Info("This won't be logged") // No output
package logger

import (
	"context"
)

// Level represents the logging level.
// Logs at or above the configured level will be written.
type Level string

const (
	LevelDebug Level = "debug" // Debug level for detailed diagnostic information
	LevelInfo  Level = "info"  // Info level for general informational messages
	LevelWarn  Level = "warn"  // Warn level for warning messages
	LevelError Level = "error" // Error level for error messages
	LevelFatal Level = "fatal" // Fatal level for critical errors that cause the program to exit
	LevelPanic Level = "panic" // Panic level for panic messages that cause the program to panic
)

// Output represents the output destination for log messages.
type Output string

const (
	OutputStdout Output = "stdout" // Write logs to standard output
	OutputStderr Output = "stderr" // Write logs to standard error
	OutputFile   Output = "file"   // Write logs to a file with rotation support
)

// Format represents the output format for log messages.
type Format string

const (
	FormatJSON Format = "json" // JSON format for structured logging (machine-readable)
	FormatText Format = "text" // Text format with color for human-readable console output
)

// RotationConfig configures file rotation settings for log files.
// This is only used when Output is set to OutputFile.
//
// File rotation automatically rotates log files when they reach MaxSize.
// Old log files are retained according to MaxBackups and MaxAge settings.
type RotationConfig struct {
	// Filename is the path to the log file. Backup files are stored in the same directory.
	// Defaults to "app.log" if empty.
	Filename string

	// MaxSize is the maximum size in megabytes before rotation occurs.
	// Defaults to 100 MB if zero.
	MaxSize int

	// MaxBackups is the maximum number of rotated log files to retain.
	// Zero means retain all backup files (subject to MaxAge).
	MaxBackups int

	// MaxAge is the maximum number of days to retain rotated log files.
	// Files older than MaxAge days are automatically deleted.
	// Zero means no age-based deletion.
	MaxAge int

	// Compress enables gzip compression for rotated log files.
	// Compressed files have a .gz extension.
	Compress bool

	// LocalTime uses local timezone for backup file timestamps.
	// If false, UTC timezone is used.
	LocalTime bool
}

// Options configures the logger behavior.
// All fields are optional and have sensible defaults.
type Options struct {
	// Level sets the minimum logging level. Messages below this level are ignored.
	// Defaults to LevelInfo if not specified.
	Level Level

	// Output specifies where log messages are written.
	// Defaults to OutputStdout if not specified.
	Output Output

	// Format specifies the output format (JSON or text).
	// Defaults to FormatText if not specified.
	// Note: File output always uses JSON format regardless of this setting.
	Format Format

	// Rotation configures file rotation when Output is OutputFile.
	// If nil, default rotation settings are used.
	Rotation *RotationConfig

	// ContextExtractor extracts fields from context.Context for automatic inclusion in logs.
	// If nil, a default extractor is used that extracts request_id, user_id, and trace_id.
	ContextExtractor ContextExtractor
}

// Field represents a single structured log field with a key-value pair.
// Fields are used to add structured data to log messages.
type Field struct {
	Key   string // The field name
	Value any    // The field value (any type)
}

// F creates a new Field with the given key and value.
// This is a convenience function for creating Field structs.
//
// Example:
//
//	logger.F("user_id", 123)
//	logger.F("ip", "192.168.1.1")
func F(key string, value any) Field {
	return Field{Key: key, Value: value}
}

// ContextExtractor extracts fields from context.Context for automatic inclusion in log messages.
// This allows custom extraction of context values such as request IDs, user IDs, trace IDs, etc.
//
// The extracted fields are automatically added to all log messages when using
// the *WithContext methods.
//
// Example:
//
//	extractor := func(ctx context.Context) []logger.Field {
//		var fields []logger.Field
//		if reqID := ctx.Value("request_id"); reqID != nil {
//			fields = append(fields, logger.F("request_id", reqID))
//		}
//		return fields
//	}
type ContextExtractor func(ctx context.Context) []Field

// Logger defines the contract for logging operations.
// All logger implementations must satisfy this interface.
//
// The interface provides methods for logging at different levels (Debug, Info, Warn, Error, Fatal, Panic)
// with support for structured fields and context-aware logging.
type Logger interface {
	// Debug logs a debug-level message with optional structured fields.
	Debug(msg string, fields ...Field)

	// Info logs an info-level message with optional structured fields.
	Info(msg string, fields ...Field)

	// Warn logs a warning-level message with optional structured fields.
	Warn(msg string, fields ...Field)

	// Error logs an error-level message with optional structured fields.
	Error(msg string, fields ...Field)

	// Fatal logs a fatal-level message with optional structured fields and exits the program.
	Fatal(msg string, fields ...Field)

	// Panic logs a panic-level message with optional structured fields and panics.
	Panic(msg string, fields ...Field)

	// Debugf logs a formatted debug-level message.
	Debugf(format string, args ...any)

	// Infof logs a formatted info-level message.
	Infof(format string, args ...any)

	// Warnf logs a formatted warning-level message.
	Warnf(format string, args ...any)

	// Errorf logs a formatted error-level message.
	Errorf(format string, args ...any)

	// Fatalf logs a formatted fatal-level message and exits the program.
	Fatalf(format string, args ...any)

	// Panicf logs a formatted panic-level message and panics.
	Panicf(format string, args ...any)

	// DebugWithContext logs a debug-level message with context-extracted fields and optional additional fields.
	DebugWithContext(ctx context.Context, msg string, fields ...Field)

	// InfoWithContext logs an info-level message with context-extracted fields and optional additional fields.
	InfoWithContext(ctx context.Context, msg string, fields ...Field)

	// WarnWithContext logs a warning-level message with context-extracted fields and optional additional fields.
	WarnWithContext(ctx context.Context, msg string, fields ...Field)

	// ErrorWithContext logs an error-level message with context-extracted fields and optional additional fields.
	ErrorWithContext(ctx context.Context, msg string, fields ...Field)

	// FatalWithContext logs a fatal-level message with context-extracted fields and optional
	// additional fields, then exits.
	FatalWithContext(ctx context.Context, msg string, fields ...Field)

	// PanicWithContext logs a panic-level message with context-extracted fields and optional
	// additional fields, then panics.
	PanicWithContext(ctx context.Context, msg string, fields ...Field)

	// DebugfWithContext logs a formatted debug-level message with context-extracted fields.
	DebugfWithContext(ctx context.Context, format string, args ...any)

	// InfofWithContext logs a formatted info-level message with context-extracted fields.
	InfofWithContext(ctx context.Context, format string, args ...any)

	// WarnfWithContext logs a formatted warning-level message with context-extracted fields.
	WarnfWithContext(ctx context.Context, format string, args ...any)

	// ErrorfWithContext logs a formatted error-level message with context-extracted fields.
	ErrorfWithContext(ctx context.Context, format string, args ...any)

	// FatalfWithContext logs a formatted fatal-level message with context-extracted fields and exits.
	FatalfWithContext(ctx context.Context, format string, args ...any)

	// PanicfWithContext logs a formatted panic-level message with context-extracted fields and panics.
	PanicfWithContext(ctx context.Context, format string, args ...any)
}
