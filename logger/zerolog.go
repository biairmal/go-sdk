package logger

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/rs/zerolog"
	"gopkg.in/natefinch/lumberjack.v2"
)

// zerologLogger implements the Logger interface using rs/zerolog as the backend.
type zerologLogger struct {
	logger           zerolog.Logger
	contextExtractor ContextExtractor
	fileWriter       *lumberjack.Logger // Keep reference for cleanup if needed
}

// NewZerolog creates a new Logger instance using zerolog as the backend.
//
// If opts is nil, default options are used:
//   - Level: LevelInfo
//   - Output: OutputStdout
//   - Format: FormatText
//   - ContextExtractor: defaultContextExtractor (extracts request_id, user_id, trace_id)
//
// When Output is OutputFile, file rotation is automatically enabled with default settings
// unless Rotation is explicitly configured. File output always uses JSON format regardless
// of the Format setting.
//
// Example:
//
//	// Basic usage with defaults
//	log := logger.NewZerolog(nil)
//
//	// Custom configuration
//	log := logger.NewZerolog(&logger.Options{
//		Level:  logger.LevelDebug,
//		Output: logger.OutputFile,
//		Format: logger.FormatJSON,
//		Rotation: &logger.RotationConfig{
//			Filename:   "logs/app.log",
//			MaxSize:    100,
//			MaxBackups: 5,
//			MaxAge:     30,
//			Compress:   true,
//		},
//	})
func NewZerolog(opts *Options) Logger {
	if opts == nil {
		opts = &Options{
			Level:  LevelInfo,
			Output: OutputStdout,
			Format: FormatText,
		}
	}

	var writer io.Writer
	var fileWriter *lumberjack.Logger

	// Determine output writer based on Output setting
	switch opts.Output {
	case OutputFile:
		// File output with rotation
		rotation := opts.Rotation
		if rotation == nil {
			rotation = &RotationConfig{
				Filename:   "app.log",
				MaxSize:    100,
				MaxBackups: 5,
				MaxAge:     30,
				Compress:   true,
				LocalTime:  true,
			}
		}

		// Set defaults for rotation config
		if rotation.Filename == "" {
			rotation.Filename = "app.log"
		}
		if rotation.MaxSize == 0 {
			rotation.MaxSize = 100 // 100 MB default
		}

		fileWriter = &lumberjack.Logger{
			Filename:   rotation.Filename,
			MaxSize:    rotation.MaxSize,
			MaxBackups: rotation.MaxBackups,
			MaxAge:     rotation.MaxAge,
			Compress:   rotation.Compress,
			LocalTime:  rotation.LocalTime,
		}
		writer = fileWriter

	case OutputStderr:
		writer = os.Stderr

	default: // OutputStdout
		writer = os.Stdout
	}

	// Configure zerolog with appropriate writer
	var baseLogger zerolog.Logger
	if opts.Format == FormatJSON {
		baseLogger = zerolog.New(writer).With().Timestamp().Logger()
	} else {
		// For file output, always use JSON format for structured logging
		// For console output, use pretty console writer
		if opts.Output == OutputFile {
			baseLogger = zerolog.New(writer).With().Timestamp().Logger()
		} else {
			output := zerolog.ConsoleWriter{Out: writer, NoColor: false}
			baseLogger = zerolog.New(output).With().Timestamp().Logger()
		}
	}

	// Set log level
	level := parseZerologLevel(opts.Level)
	baseLogger = baseLogger.Level(level)

	// Set context extractor, default if not provided
	contextExtractor := opts.ContextExtractor
	if contextExtractor == nil {
		contextExtractor = defaultContextExtractor
	}

	return &zerologLogger{
		logger:           baseLogger,
		contextExtractor: contextExtractor,
		fileWriter:       fileWriter,
	}
}

// parseZerologLevel converts a Level to the corresponding zerolog.Level.
// Returns zerolog.InfoLevel for unknown levels.
func parseZerologLevel(level Level) zerolog.Level {
	switch level {
	case LevelDebug:
		return zerolog.DebugLevel
	case LevelInfo:
		return zerolog.InfoLevel
	case LevelWarn:
		return zerolog.WarnLevel
	case LevelError:
		return zerolog.ErrorLevel
	case LevelFatal:
		return zerolog.FatalLevel
	case LevelPanic:
		return zerolog.PanicLevel
	default:
		return zerolog.InfoLevel
	}
}

// defaultContextExtractor extracts common context values for logging.
// It extracts request_id, user_id, and trace_id from the context if present.
// This can be overridden by providing a custom ContextExtractor in Options.
func defaultContextExtractor(ctx context.Context) []Field {
	var fields []Field

	// Extract request ID if present
	if reqID := ctx.Value("request_id"); reqID != nil {
		fields = append(fields, Field{Key: "request_id", Value: reqID})
	}

	// Extract user ID if present
	if userID := ctx.Value("user_id"); userID != nil {
		fields = append(fields, Field{Key: "user_id", Value: userID})
	}

	// Extract trace ID if present
	if traceID := ctx.Value("trace_id"); traceID != nil {
		fields = append(fields, Field{Key: "trace_id", Value: traceID})
	}

	return fields
}

// addFields adds structured fields to a zerolog event from a variadic Field slice.
// If no fields are provided, the event is returned unchanged.
func addFields(event *zerolog.Event, fields ...Field) *zerolog.Event {
	if len(fields) == 0 {
		return event
	}

	for _, field := range fields {
		event = event.Interface(field.Key, field.Value)
	}

	return event
}

// addContextFields adds context-extracted fields to a zerolog event using the logger's ContextExtractor.
// If no ContextExtractor is configured, the event is returned unchanged.
func (l *zerologLogger) addContextFields(ctx context.Context, event *zerolog.Event) *zerolog.Event {
	if l.contextExtractor == nil {
		return event
	}

	fields := l.contextExtractor(ctx)
	return addFields(event, fields...)
}

// Debug logs a debug message.
func (l *zerologLogger) Debug(msg string, fields ...Field) {
	event := l.logger.Debug()
	event = addFields(event, fields...)
	event.Msg(msg)
}

// Info logs an info message.
func (l *zerologLogger) Info(msg string, fields ...Field) {
	event := l.logger.Info()
	event = addFields(event, fields...)
	event.Msg(msg)
}

// Warn logs a warning message.
func (l *zerologLogger) Warn(msg string, fields ...Field) {
	event := l.logger.Warn()
	event = addFields(event, fields...)
	event.Msg(msg)
}

// Error logs an error message.
func (l *zerologLogger) Error(msg string, fields ...Field) {
	event := l.logger.Error()
	event = addFields(event, fields...)
	event.Msg(msg)
}

// Fatal logs a fatal message and exits.
func (l *zerologLogger) Fatal(msg string, fields ...Field) {
	event := l.logger.Fatal()
	event = addFields(event, fields...)
	event.Msg(msg)
}

// Panic logs a panic message and panics.
func (l *zerologLogger) Panic(msg string, fields ...Field) {
	event := l.logger.Panic()
	event = addFields(event, fields...)
	event.Msg(msg)
}

// Debugf logs a formatted debug message.
func (l *zerologLogger) Debugf(format string, args ...any) {
	l.logger.Debug().Msg(fmt.Sprintf(format, args...))
}

// Infof logs a formatted info message.
func (l *zerologLogger) Infof(format string, args ...any) {
	l.logger.Info().Msg(fmt.Sprintf(format, args...))
}

// Warnf logs a formatted warning message.
func (l *zerologLogger) Warnf(format string, args ...any) {
	l.logger.Warn().Msg(fmt.Sprintf(format, args...))
}

// Errorf logs a formatted error message.
func (l *zerologLogger) Errorf(format string, args ...any) {
	l.logger.Error().Msg(fmt.Sprintf(format, args...))
}

// Fatalf logs a formatted fatal message and exits.
func (l *zerologLogger) Fatalf(format string, args ...any) {
	l.logger.Fatal().Msg(fmt.Sprintf(format, args...))
}

// Panicf logs a formatted panic message and panics.
func (l *zerologLogger) Panicf(format string, args ...any) {
	l.logger.Panic().Msg(fmt.Sprintf(format, args...))
}

// DebugWithContext logs a debug message with context.
func (l *zerologLogger) DebugWithContext(ctx context.Context, msg string, fields ...Field) {
	event := l.logger.Debug()
	event = l.addContextFields(ctx, event)
	event = addFields(event, fields...)
	event.Msg(msg)
}

// InfoWithContext logs an info message with context.
func (l *zerologLogger) InfoWithContext(ctx context.Context, msg string, fields ...Field) {
	event := l.logger.Info()
	event = l.addContextFields(ctx, event)
	event = addFields(event, fields...)
	event.Msg(msg)
}

// WarnWithContext logs a warning message with context.
func (l *zerologLogger) WarnWithContext(ctx context.Context, msg string, fields ...Field) {
	event := l.logger.Warn()
	event = l.addContextFields(ctx, event)
	event = addFields(event, fields...)
	event.Msg(msg)
}

// ErrorWithContext logs an error message with context.
func (l *zerologLogger) ErrorWithContext(ctx context.Context, msg string, fields ...Field) {
	event := l.logger.Error()
	event = l.addContextFields(ctx, event)
	event = addFields(event, fields...)
	event.Msg(msg)
}

// FatalWithContext logs a fatal message with context and exits.
func (l *zerologLogger) FatalWithContext(ctx context.Context, msg string, fields ...Field) {
	event := l.logger.Fatal()
	event = l.addContextFields(ctx, event)
	event = addFields(event, fields...)
	event.Msg(msg)
}

// PanicWithContext logs a panic message with context and panics.
func (l *zerologLogger) PanicWithContext(ctx context.Context, msg string, fields ...Field) {
	event := l.logger.Panic()
	event = l.addContextFields(ctx, event)
	event = addFields(event, fields...)
	event.Msg(msg)
}

// DebugfWithContext logs a formatted debug message with context.
func (l *zerologLogger) DebugfWithContext(ctx context.Context, format string, args ...any) {
	event := l.logger.Debug()
	event = l.addContextFields(ctx, event)
	event.Msg(fmt.Sprintf(format, args...))
}

// InfofWithContext logs a formatted info message with context.
func (l *zerologLogger) InfofWithContext(ctx context.Context, format string, args ...any) {
	event := l.logger.Info()
	event = l.addContextFields(ctx, event)
	event.Msg(fmt.Sprintf(format, args...))
}

// WarnfWithContext logs a formatted warning message with context.
func (l *zerologLogger) WarnfWithContext(ctx context.Context, format string, args ...any) {
	event := l.logger.Warn()
	event = l.addContextFields(ctx, event)
	event.Msg(fmt.Sprintf(format, args...))
}

// ErrorfWithContext logs a formatted error message with context.
func (l *zerologLogger) ErrorfWithContext(ctx context.Context, format string, args ...any) {
	event := l.logger.Error()
	event = l.addContextFields(ctx, event)
	event.Msg(fmt.Sprintf(format, args...))
}

// FatalfWithContext logs a formatted fatal message with context and exits.
func (l *zerologLogger) FatalfWithContext(ctx context.Context, format string, args ...any) {
	event := l.logger.Fatal()
	event = l.addContextFields(ctx, event)
	event.Msg(fmt.Sprintf(format, args...))
}

// PanicfWithContext logs a formatted panic message with context and panics.
func (l *zerologLogger) PanicfWithContext(ctx context.Context, format string, args ...any) {
	event := l.logger.Panic()
	event = l.addContextFields(ctx, event)
	event.Msg(fmt.Sprintf(format, args...))
}
