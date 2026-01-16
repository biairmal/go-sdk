# Logger Package

A structured logging package for Go that provides a unified interface for logging operations with support for multiple backends, context extraction, and file rotation.

## Overview

The logger package offers a flexible and extensible logging solution built on top of the `rs/zerolog` library. It provides a clean interface abstraction that allows for easy switching between different logging implementations, making it suitable for both production applications and testing scenarios.

The package supports structured logging with key-value fields, context-aware logging for distributed systems, and automatic file rotation for production deployments. It is designed to be performant, type-safe, and easy to integrate into existing Go applications.

## Features

### Core Capabilities

- **Multiple Log Levels**: Support for Debug, Info, Warn, Error, Fatal, and Panic levels
- **Structured Logging**: Key-value field support for rich, queryable log entries
- **Context-Aware Logging**: Automatic extraction of context values (request_id, user_id, trace_id) for distributed tracing
- **Multiple Output Destinations**: Support for stdout, stderr, and file output
- **File Rotation**: Automatic log file rotation with configurable size, retention, and compression
- **Format Support**: JSON format for machine-readable logs and text format with colour for human-readable console output
- **Formatted Logging**: Support for printf-style formatted messages alongside structured fields
- **Zero Dependencies Interface**: Clean interface design allows for custom implementations

### Implementations

- **Zerolog Backend**: Production-ready implementation using `rs/zerolog` with full feature support
- **No-Op Logger**: Testing-friendly implementation that discards all log output

## Limitations

### General Limitations

1. **File Output Format**: When using file output (`OutputFile`), logs are always written in JSON format regardless of the `Format` setting. The `Format` option only affects console output (stdout/stderr).

2. **No-Op Behaviour**: The no-op logger implementation does not exit the program on `Fatal` calls or panic on `Panic` calls. This is intentional for testing purposes but may not reflect production behaviour.

3. **Context Extraction**: The default context extractor only extracts `request_id`, `user_id`, and `trace_id` from context. Custom extractors must be provided for additional context values.

4. **Single Output Destination**: Each logger instance can only write to one output destination at a time. Multiple destinations require multiple logger instances.

5. **No Log Sampling**: The package does not provide built-in log sampling or rate limiting capabilities.

6. **Synchronous Logging**: All logging operations are synchronous. High-throughput scenarios may benefit from buffering or asynchronous logging, which is not provided out of the box.

7. **No Log Aggregation**: The package does not include built-in support for sending logs to external aggregation services (e.g., ELK, Splunk, Datadog). Integration must be implemented separately.

## Usage

### Installation

```bash
go get github.com/biairmal/go-sdk/logger
```

### Basic Usage

#### Simple Logger with Defaults

```go
package main

import (
    "github.com/biairmal/go-sdk/logger"
)

func main() {
    // Create logger with default settings (Info level, stdout, text format)
    log := logger.NewZerolog(nil)
    
    log.Info("Application started")
    log.Info("User logged in", logger.F("user_id", 123), logger.F("ip", "192.168.1.1"))
}
```

#### Custom Configuration

```go
log := logger.NewZerolog(&logger.Options{
    Level:  logger.LevelDebug,
    Output: logger.OutputStdout,
    Format: logger.FormatJSON,
})
```

#### File Output with Rotation

```go
log := logger.NewZerolog(&logger.Options{
    Level:  logger.LevelInfo,
    Output: logger.OutputFile,
    Rotation: &logger.RotationConfig{
        Filename:   "logs/app.log",
        MaxSize:    100,      // 100 MB
        MaxBackups: 5,        // Keep 5 backup files
        MaxAge:     30,       // Keep files for 30 days
        Compress:   true,     // Compress rotated files
        LocalTime:  true,     // Use local timezone
    },
})
```

### Log Levels

```go
log.Debug("Debug message", logger.F("key", "value"))
log.Info("Info message", logger.F("key", "value"))
log.Warn("Warning message", logger.F("key", "value"))
log.Error("Error message", logger.F("key", "value"))
log.Fatal("Fatal message", logger.F("key", "value")) // Exits program
log.Panic("Panic message", logger.F("key", "value")) // Panics
```

### Formatted Logging

```go
log.Debugf("User %s logged in from %s", username, ip)
log.Infof("Processing %d items", count)
log.Warnf("Rate limit approaching: %d%%", percentage)
log.Errorf("Failed to connect: %v", err)
log.Fatalf("Critical error: %s", message) // Exits program
log.Panicf("Panic: %s", message) // Panics
```

### Context-Aware Logging

```go
import (
    "context"
    "github.com/biairmal/go-sdk/logger"
)

// Set context values
ctx := context.WithValue(context.Background(), "request_id", "req-123")
ctx = context.WithValue(ctx, "user_id", 456)
ctx = context.WithValue(ctx, "trace_id", "trace-789")

// Log with context (automatically includes request_id, user_id, trace_id)
log.InfoWithContext(ctx, "Request processed", logger.F("status", "success"))

// Formatted logging with context
log.InfofWithContext(ctx, "User %s performed action", username)
```

### Custom Context Extractor

```go
customExtractor := func(ctx context.Context) []logger.Field {
    var fields []logger.Field
    
    if reqID := ctx.Value("request_id"); reqID != nil {
        fields = append(fields, logger.F("request_id", reqID))
    }
    
    if sessionID := ctx.Value("session_id"); sessionID != nil {
        fields = append(fields, logger.F("session_id", sessionID))
    }
    
    return fields
}

log := logger.NewZerolog(&logger.Options{
    Level:            logger.LevelInfo,
    Output:           logger.OutputStdout,
    Format:           logger.FormatJSON,
    ContextExtractor: customExtractor,
})
```

### Testing with No-Op Logger

```go
func TestMyFunction(t *testing.T) {
    // Use no-op logger to suppress log output during tests
    log := logger.NewNoOp()
    
    // Your test code here
    // All log calls will be silently ignored
    log.Info("This won't appear in test output")
}
```

### Structured Fields

```go
// Single field
log.Info("Event occurred", logger.F("event_type", "user_action"))

// Multiple fields
log.Info("Request completed",
    logger.F("method", "GET"),
    logger.F("path", "/api/users"),
    logger.F("status_code", 200),
    logger.F("duration_ms", 45),
)

// Nested structures (serialised as JSON in output)
log.Info("User data",
    logger.F("user", map[string]interface{}{
        "id":    123,
        "name":  "John Doe",
        "email": "john@example.com",
    }),
)
```

## Configuration Options

### Log Levels

- `LevelDebug`: Detailed diagnostic information
- `LevelInfo`: General informational messages (default)
- `LevelWarn`: Warning messages
- `LevelError`: Error messages
- `LevelFatal`: Critical errors that cause program exit
- `LevelPanic`: Panic messages that cause program panic

### Output Destinations

- `OutputStdout`: Standard output (default)
- `OutputStderr`: Standard error
- `OutputFile`: File output with rotation support

### Output Formats

- `FormatText`: Human-readable text format with colour (default for console)
- `FormatJSON`: Machine-readable JSON format (always used for file output)

### Rotation Configuration

- `Filename`: Path to log file (default: "app.log")
- `MaxSize`: Maximum file size in MB before rotation (default: 100 MB)
- `MaxBackups`: Maximum number of rotated files to keep (default: 5, 0 = unlimited)
- `MaxAge`: Maximum days to retain rotated files (default: 30, 0 = no age limit)
- `Compress`: Enable gzip compression for rotated files (default: false)
- `LocalTime`: Use local timezone for timestamps (default: false, uses UTC)

## Examples

### HTTP Server with Request Logging

```go
package main

import (
    "context"
    "net/http"
    "github.com/biairmal/go-sdk/logger"
)

var log logger.Logger

func init() {
    log = logger.NewZerolog(&logger.Options{
        Level:  logger.LevelInfo,
        Output: logger.OutputStdout,
        Format: logger.FormatJSON,
    })
}

func handler(w http.ResponseWriter, r *http.Request) {
    ctx := context.WithValue(r.Context(), "request_id", generateRequestID())
    ctx = context.WithValue(ctx, "ip", r.RemoteAddr)
    
    log.InfoWithContext(ctx, "Request received",
        logger.F("method", r.Method),
        logger.F("path", r.URL.Path),
    )
    
    // Process request...
    
    log.InfoWithContext(ctx, "Request completed",
        logger.F("status", 200),
    )
}
```

### Application Initialization

```go
func initLogger(cfg *Config) logger.Logger {
    opts := &logger.Options{
        Level: parseLogLevel(cfg.LogLevel),
    }
    
    switch cfg.LogOutput {
    case "file":
        opts.Output = logger.OutputFile
        opts.Rotation = &logger.RotationConfig{
            Filename:   cfg.LogFile,
            MaxSize:    cfg.LogMaxSize,
            MaxBackups: cfg.LogMaxBackups,
            MaxAge:     cfg.LogMaxAge,
            Compress:   cfg.LogCompress,
        }
    case "stderr":
        opts.Output = logger.OutputStderr
    default:
        opts.Output = logger.OutputStdout
    }
    
    if cfg.LogFormat == "json" {
        opts.Format = logger.FormatJSON
    } else {
        opts.Format = logger.FormatText
    }
    
    return logger.NewZerolog(opts)
}
```

## Dependencies

- `github.com/rs/zerolog`: Structured logging library
- `gopkg.in/natefinch/lumberjack.v2`: Log file rotation

## License

See the main repository license file.
