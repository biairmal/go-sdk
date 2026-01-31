// Package Error provides a custom error type with enhanced metadata capabilities.
// It extends the standard error interface with support for error codes, source system
// identification, and arbitrary metadata. The package implements the error wrapping
// and unwrapping interfaces defined in the errors package, enabling seamless integration
// with Go's error handling mechanisms.
//
// Example usage:
//
//	// Create a new error
//	err := Error.New("resource not found").
//		WithCode("ERR_NOT_FOUND").
//		WithSourceSystem("user-service").
//		WithMeta("resource_id", 12345)
//
//	// Wrap an existing error
//	wrappedErr := Error.Wrap(errors.New("database connection failed")).
//		WithCode("ERR_DB_CONN").
//		WithMessage("failed to connect to database")
//
//	// Use predefined errors
//	if resource == nil {
//		return Error.ErrNotFound.WithCode("RESOURCE_001")
//	}
package errorz

import (
	"errors"
	"fmt"
)

// Predefined error variables for common HTTP and application error scenarios.
// These errors can be used directly or extended with additional metadata using
// the With* methods.
var (
	// ErrNotFound represents a "not found" error (HTTP 404 equivalent).
	ErrNotFound = New("not found")

	// ErrBadRequest represents a "bad request" error (HTTP 400 equivalent).
	ErrBadRequest = New("bad request")

	// ErrInternal represents an "internal server error" (HTTP 500 equivalent).
	ErrInternal = New("internal server error")

	// ErrUnauthorized represents an "unauthorized" error (HTTP 401 equivalent).
	ErrUnauthorized = New("unauthorized")

	// ErrForbidden represents a "forbidden" error (HTTP 403 equivalent).
	ErrForbidden = New("forbidden")

	// ErrTooManyRequests represents a "too many requests" error (HTTP 429 equivalent).
	ErrTooManyRequests = New("too many requests")

	// ErrBadGateway represents a "bad gateway" error (HTTP 502 equivalent).
	ErrBadGateway = New("bad gateway")

	// ErrServiceUnavailable represents a "service unavailable" error (HTTP 503 equivalent).
	ErrServiceUnavailable = New("service unavailable")

	// ErrUnprocessableEntity represents an "unprocessable entity" error (HTTP 422 equivalent).
	ErrUnprocessableEntity = New("unprocessable entity")

	// ErrConflict represents a "conflict" error (HTTP 409 equivalent).
	ErrConflict = New("conflict")

	// ErrPreconditionFailed represents a "precondition failed" error (HTTP 412 equivalent).
	ErrPreconditionFailed = New("precondition failed")

	// ErrPreconditionRequired represents a "precondition required" error (HTTP 428 equivalent).
	ErrPreconditionRequired = New("precondition required")

	// ErrPreconditionNotMet represents a "precondition not met" error.
	ErrPreconditionNotMet = New("precondition not met")
)

// DefaultSourceSystem is the default value used for the SourceSystem field
// when creating new Error instances via New or Wrap.
var DefaultSourceSystem = "application"

// Error represents a custom error type with additional metadata capabilities.
// It implements the error interface and supports error wrapping/unwrapping
// as defined in the errors package.
//
// The Error type provides:
//   - Code: A machine-readable error code for programmatic error handling
//   - Message: A human-readable error message
//   - SourceSystem: The system or service that generated the error
//   - Err: The underlying error that was wrapped (if any)
//   - Meta: Arbitrary key-value metadata for additional context
//
// All With* methods return the receiver to enable method chaining.
type Error struct {
	// Code is a machine-readable error code that can be used for
	// programmatic error handling and logging.
	Code string

	// Message is the human-readable error message returned by Error().
	Message string

	// SourceSystem identifies the system or service that generated the error.
	// This is useful for distributed systems where errors may originate from
	// multiple services.
	SourceSystem string

	// Err is the underlying error that was wrapped, if any.
	// This field is set when using Wrap() and can be accessed via Unwrap().
	Err error

	// Meta contains arbitrary key-value metadata that provides additional
	// context about the error. Common use cases include request IDs, user IDs,
	// timestamps, or other contextual information.
	Meta map[string]any
}

// Error returns a string representation of the error.
// The string includes the error code, source system, message, metadata, and original error.
// The string is formatted as:
// "Code: <code>, SourceSystem: <sourceSystem>, Message: <message>, Meta: <meta>, Original Error: <originalError>"
// If the error code, source system, message, or metadata is not set, it is not included in the string.
// If the original error is not set, it is not included in the string.
func (e *Error) Error() string {
	var message string

	if e.Code != "" {
		message += fmt.Sprintf("Code: %s, ", e.Code)
	}
	if e.SourceSystem != "" {
		message += fmt.Sprintf("SourceSystem: %s, ", e.SourceSystem)
	}
	if e.Message != "" {
		message += fmt.Sprintf("Message: %s, ", e.Message)
	}
	if len(e.Meta) > 0 {
		message += fmt.Sprintf("Meta: %v", e.Meta)
	}
	if e.Err != nil {
		message += fmt.Sprintf(", Original Error: %v", e.Err.Error())
	}

	return message
}

// Unwrap returns the underlying error that was wrapped, if any.
// This method implements the Unwrap interface defined in the errors package,
// enabling the use of errors.Is() and errors.As() with Error instances.
//
// If the Error was created via New() or does not wrap an error, Unwrap returns nil.
func (e *Error) Unwrap() error {
	return e.Err
}

// Wrap wraps an existing error into an Error instance.
// The wrapped error can be accessed later via Unwrap() or checked using Is().
//
// The resulting Error will have:
//   - Err set to the provided error
//   - SourceSystem set to DefaultSourceSystem
//   - Empty Message and Code fields (can be set using With* methods)
//
// Example:
//
//	err := errors.New("database connection failed")
//	wrapped := Error.Wrap(err).
//		WithCode("DB_CONN_ERR").
//		WithMessage("failed to connect to database")
func Wrap(err error) *Error {
	return &Error{
		Err:          err,
		SourceSystem: DefaultSourceSystem,
	}
}

// Is checks if the Error wraps an error that matches the target error.
// This method implements the Is interface defined in the errors package,
// enabling the use of errors.Is() with Error instances.
//
// The method uses errors.Is() to check if the wrapped error (Err) matches
// the target error, supporting error wrapping chains.
//
// If the Error does not wrap an error, Is returns false.
func (e *Error) Is(target error) bool {
	return errors.Is(e.Err, target)
}

// New creates a new Error instance with the specified message.
// The resulting Error will have:
//   - Message set to the provided message
//   - SourceSystem set to DefaultSourceSystem
//   - Empty Code and Err fields (can be set using With* methods)
//
// Example:
//
//	err := Error.New("resource not found").
//		WithCode("RESOURCE_001").
//		WithSourceSystem("user-service")
func New(message string) *Error {
	return &Error{
		Message:      message,
		SourceSystem: DefaultSourceSystem,
	}
}

// WithCode sets the error code and returns the receiver for method chaining.
// The error code is a machine-readable identifier that can be used for
// programmatic error handling, logging, or API responses.
//
// Example:
//
//	err := Error.New("validation failed").WithCode("VALIDATION_001")
func (e *Error) WithCode(code string) *Error {
	e.Code = code
	return e
}

// WithMessage sets the error message and returns the receiver for method chaining.
// The message is returned by the Error() method and should be human-readable.
//
// Example:
//
//	err := Error.New("original message").WithMessage("updated message")
func (e *Error) WithMessage(message string) *Error {
	e.Message = message
	return e
}

// WithSourceSystem sets the source system identifier and returns the receiver
// for method chaining. The source system identifies which system or service
// generated the error, which is particularly useful in distributed architectures.
//
// Example:
//
//	err := Error.New("error occurred").
//		WithSourceSystem("payment-service")
func (e *Error) WithSourceSystem(sourceSystem string) *Error {
	e.SourceSystem = sourceSystem
	return e
}

// WithMeta adds a key-value pair to the metadata map and returns the receiver
// for method chaining. If the Meta map is nil, it is initialized automatically.
//
// The metadata can contain any type of value (any) and is useful for storing
// contextual information such as request IDs, user IDs, timestamps, or other
// relevant data for debugging and logging.
//
// If a key already exists in the metadata map, its value will be overwritten.
//
// Example:
//
//	err := Error.New("operation failed").
//		WithMeta("request_id", "abc123").
//		WithMeta("user_id", 456).
//		WithMeta("timestamp", time.Now())
func (e *Error) WithMeta(key string, value any) *Error {
	if e.Meta == nil {
		e.Meta = make(map[string]any)
	}
	e.Meta[key] = value
	return e
}
