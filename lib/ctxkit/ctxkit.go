// Package ctxkit holds the canonical request-scoped context keys shared across
// the SDK and typed accessors for them.
//
// There are four canonical request-scoped values, each with exactly one writer
// path in the standard middleware chain:
//
//   - request_id     — per-request id, unique to this hop (middleware.RequestID)
//   - correlation_id — flows across services for one logical op (middleware.Correlation)
//   - trace_id       — OpenTelemetry trace id (middleware.Tracing)
//   - user_id        — authenticated subject (middleware.Auth)
//
// Producers store values with the With* setters; consumers read them with the
// matching getters. The logger surfaces all present values automatically when
// built with LoggerExtractor:
//
//	log := logger.NewZerolog(&logger.Options{ContextExtractor: ctxkit.LoggerExtractor()})
//
// Keys are unexported and typed, so values cannot collide with other packages'
// context keys and cannot be set except through this package.
package ctxkit

import "context"

// ctxKey is the private key type for all canonical context values. Using a
// dedicated unexported type guarantees no collision with keys from other packages.
type ctxKey int

const (
	keyRequestID ctxKey = iota
	keyCorrelationID
	keyTraceID
	keyUserID
)

// WithRequestID returns a copy of ctx carrying the request id.
// An empty id returns ctx unchanged (empty values are never stored).
func WithRequestID(ctx context.Context, id string) context.Context {
	return withValue(ctx, keyRequestID, id)
}

// RequestID returns the request id stored in ctx, or "" if absent.
func RequestID(ctx context.Context) string {
	return stringValue(ctx, keyRequestID)
}

// WithCorrelationID returns a copy of ctx carrying the correlation id.
// An empty id returns ctx unchanged.
func WithCorrelationID(ctx context.Context, id string) context.Context {
	return withValue(ctx, keyCorrelationID, id)
}

// CorrelationID returns the correlation id stored in ctx, or "" if absent.
func CorrelationID(ctx context.Context) string {
	return stringValue(ctx, keyCorrelationID)
}

// WithTraceID returns a copy of ctx carrying the trace id.
// An empty id returns ctx unchanged.
func WithTraceID(ctx context.Context, id string) context.Context {
	return withValue(ctx, keyTraceID, id)
}

// TraceID returns the trace id stored in ctx, or "" if absent.
func TraceID(ctx context.Context) string {
	return stringValue(ctx, keyTraceID)
}

// WithUserID returns a copy of ctx carrying the authenticated subject id.
// An empty id returns ctx unchanged.
func WithUserID(ctx context.Context, id string) context.Context {
	return withValue(ctx, keyUserID, id)
}

// UserID returns the authenticated subject id stored in ctx, or "" if absent.
func UserID(ctx context.Context) string {
	return stringValue(ctx, keyUserID)
}

// withValue stores a non-empty string under key; empty values are ignored so
// callers never overwrite an existing value with "" and getters stay simple.
func withValue(ctx context.Context, key ctxKey, value string) context.Context {
	if value == "" {
		return ctx
	}
	return context.WithValue(ctx, key, value)
}

// stringValue reads a string stored under key, returning "" if absent or of a
// different type.
func stringValue(ctx context.Context, key ctxKey) string {
	if v, ok := ctx.Value(key).(string); ok {
		return v
	}
	return ""
}
