# Errorz Package

A custom error type for Go that provides enhanced metadata capabilities, error codes, source system identification, and seamless integration with Go's standard error handling mechanisms.

## Overview

The errorz package extends the standard error interface with support for structured error information, making it suitable for distributed systems, API development, and applications that require rich error context. It implements the error wrapping and unwrapping interfaces defined in the `errors` package, enabling seamless integration with Go's error handling mechanisms.

The package provides a fluent API for building errors with method chaining, predefined error variables for common scenarios, and support for arbitrary metadata. It is designed to be type-safe, performant, and easy to integrate into existing Go applications.

## Features

### Core Capabilities

- **Error Codes**: Machine-readable error codes for programmatic error handling and logging
- **Source System Identification**: Track which system or service generated the error, useful for distributed architectures
- **Error Wrapping**: Wrap existing errors while preserving the original error chain
- **Standard Error Interface**: Full compatibility with Go's `errors` package (`errors.Is`, `errors.As`, `errors.Unwrap`)
- **Arbitrary Metadata**: Key-value metadata support for additional contextual information
- **Method Chaining**: Fluent API with `With*` methods that return the receiver for chaining
- **Predefined Errors**: Common HTTP and application error scenarios available as package variables

### Predefined Errors

The package includes predefined error variables for common scenarios:

- `ErrNotFound` (HTTP 404 equivalent)
- `ErrBadRequest` (HTTP 400 equivalent)
- `ErrInternal` (HTTP 500 equivalent)
- `ErrUnauthorized` (HTTP 401 equivalent)
- `ErrForbidden` (HTTP 403 equivalent)
- `ErrTooManyRequests` (HTTP 429 equivalent)
- `ErrBadGateway` (HTTP 502 equivalent)
- `ErrServiceUnavailable` (HTTP 503 equivalent)
- `ErrUnprocessableEntity` (HTTP 422 equivalent)
- `ErrConflict` (HTTP 409 equivalent)
- `ErrPreconditionFailed` (HTTP 412 equivalent)
- `ErrPreconditionRequired` (HTTP 428 equivalent)
- `ErrPreconditionNotMet`

## Limitations

### General Limitations

1. **Error() Method**: The `Error()` method only returns the `Message` field. Error codes, source system, and metadata are not included in the string representation. To access these fields, you must access the struct fields directly.

2. **No Automatic Serialisation**: When errors are printed or logged, metadata and error codes are not automatically included in the output. You must explicitly format the error to include this information.

3. **No HTTP Status Code Mapping**: The package does not provide built-in mapping between error codes and HTTP status codes. This mapping must be implemented separately in your application layer.

4. **No Error Code Validation**: The package does not validate or enforce any format for error codes. It is the application's responsibility to maintain consistent error code conventions.

5. **Global DefaultSourceSystem**: The `DefaultSourceSystem` variable is global and shared across all package instances. Changing it affects all new errors created after the change.

6. **Metadata Overwrites**: Calling `WithMeta()` with an existing key overwrites the previous value without warning. There is no mechanism to merge or append metadata values.

7. **No Error Aggregation**: The package does not provide built-in support for aggregating multiple errors or creating error collections.

8. **Nil Error Handling**: Wrapping a `nil` error with `Wrap()` creates a valid `Errorz` instance with a `nil` `Err` field. This may not always be the desired behaviour.

## Usage

### Installation

```bash
go get github.com/biairmal/go-sdk/errorz
```

### Basic Usage

#### Creating New Errors

```go
package main

import (
    "github.com/biairmal/go-sdk/errorz"
)

func main() {
    // Create a simple error
    err := errorz.New("resource not found")
    
    // Create error with code and metadata
    err := errorz.New("validation failed").
        WithCode("VALIDATION_001").
        WithSourceSystem("user-service").
        WithMeta("field", "email").
        WithMeta("value", "invalid@")
}
```

#### Wrapping Existing Errors

```go
import (
    "errors"
    "github.com/biairmal/go-sdk/errorz"
)

func processData() error {
    data, err := fetchData()
    if err != nil {
        return errorz.Wrap(err).
            WithCode("DATA_FETCH_ERR").
            WithMessage("failed to fetch data from external service").
            WithSourceSystem("data-service").
            WithMeta("endpoint", "https://api.example.com/data")
    }
    // ...
}
```

#### Using Predefined Errors

```go
func findUser(id int) (*User, error) {
    user, err := db.GetUser(id)
    if err != nil {
        return nil, errorz.ErrNotFound.
            WithCode("USER_001").
            WithMessage("user not found").
            WithMeta("user_id", id)
    }
    return user, nil
}
```

### Error Handling

#### Checking Error Types

```go
import (
    "errors"
    "github.com/biairmal/go-sdk/errorz"
)

func handleError(err error) {
    // Check if error is a specific Errorz instance
    var errz *errorz.Errorz
    if errors.As(err, &errz) {
        fmt.Printf("Error Code: %s\n", errz.Code)
        fmt.Printf("Source System: %s\n", errz.SourceSystem)
        fmt.Printf("Metadata: %+v\n", errz.Meta)
    }
    
    // Check if error wraps a specific error
    if errors.Is(err, sql.ErrNoRows) {
        // Handle database not found error
    }
}
```

#### Using errors.Is with Errorz

```go
targetErr := errors.New("target error")
wrappedErr := errorz.Wrap(targetErr)

if errors.Is(wrappedErr, targetErr) {
    // This will be true
}

// Errorz also implements Is method directly
if wrappedErr.Is(targetErr) {
    // This will also be true
}
```

### Method Chaining

All `With*` methods return the receiver, enabling fluent method chaining:

```go
err := errorz.New("operation failed").
    WithCode("OP_001").
    WithMessage("detailed error message").
    WithSourceSystem("payment-service").
    WithMeta("request_id", "req-123").
    WithMeta("user_id", 456).
    WithMeta("amount", 100.50).
    WithMeta("timestamp", time.Now())
```

### Metadata Usage

```go
// Add single metadata entry
err := errorz.New("error").WithMeta("key", "value")

// Add multiple metadata entries
err := errorz.New("error").
    WithMeta("request_id", "abc-123").
    WithMeta("user_id", 789).
    WithMeta("ip_address", "192.168.1.1").
    WithMeta("retry_count", 3)

// Overwrite existing metadata
err := errorz.New("error").
    WithMeta("count", 1).
    WithMeta("count", 2) // count is now 2
```

### Custom Source System

```go
// Set default source system for all new errors
errorz.DefaultSourceSystem = "my-application"

// Override for specific error
err := errorz.New("error").
    WithSourceSystem("custom-service")
```

## Examples

### HTTP Handler with Error Handling

```go
package main

import (
    "encoding/json"
    "net/http"
    "github.com/biairmal/go-sdk/errorz"
)

func getUserHandler(w http.ResponseWriter, r *http.Request) {
    userID := r.URL.Query().Get("id")
    if userID == "" {
        err := errorz.ErrBadRequest.
            WithCode("MISSING_USER_ID").
            WithMessage("user ID is required")
        writeErrorResponse(w, err, http.StatusBadRequest)
        return
    }
    
    user, err := findUser(userID)
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            err := errorz.ErrNotFound.
                WithCode("USER_NOT_FOUND").
                WithMessage("user not found").
                WithMeta("user_id", userID)
            writeErrorResponse(w, err, http.StatusNotFound)
            return
        }
        
        err := errorz.ErrInternal.
            WithCode("DB_ERROR").
            WithMessage("database error occurred")
        writeErrorResponse(w, err, http.StatusInternalServerError)
        return
    }
    
    json.NewEncoder(w).Encode(user)
}

func writeErrorResponse(w http.ResponseWriter, err *errorz.Errorz, statusCode int) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(statusCode)
    
    response := map[string]interface{}{
        "error": map[string]interface{}{
            "code":    err.Code,
            "message": err.Message,
            "source":  err.SourceSystem,
        },
    }
    
    if err.Meta != nil && len(err.Meta) > 0 {
        response["error"].(map[string]interface{})["meta"] = err.Meta
    }
    
    json.NewEncoder(w).Encode(response)
}
```

### Service Layer with Error Propagation

```go
package service

import (
    "context"
    "errors"
    "github.com/biairmal/go-sdk/errorz"
)

type UserService struct {
    repo UserRepository
}

func (s *UserService) CreateUser(ctx context.Context, user *User) error {
    // Validate user
    if user.Email == "" {
        return errorz.ErrBadRequest.
            WithCode("VALIDATION_EMAIL_REQUIRED").
            WithMessage("email is required").
            WithSourceSystem("user-service")
    }
    
    // Check if user exists
    existing, err := s.repo.FindByEmail(ctx, user.Email)
    if err != nil && !errors.Is(err, sql.ErrNoRows) {
        return errorz.Wrap(err).
            WithCode("DB_QUERY_ERROR").
            WithMessage("failed to check existing user").
            WithSourceSystem("user-service").
            WithMeta("email", user.Email)
    }
    
    if existing != nil {
        return errorz.ErrConflict.
            WithCode("USER_ALREADY_EXISTS").
            WithMessage("user with this email already exists").
            WithSourceSystem("user-service").
            WithMeta("email", user.Email)
    }
    
    // Create user
    if err := s.repo.Create(ctx, user); err != nil {
        return errorz.Wrap(err).
            WithCode("DB_CREATE_ERROR").
            WithMessage("failed to create user").
            WithSourceSystem("user-service").
            WithMeta("email", user.Email)
    }
    
    return nil
}
```

### Error Logging with Metadata

```go
package main

import (
    "github.com/biairmal/go-sdk/errorz"
    "github.com/biairmal/go-sdk/logger"
)

func handleError(log logger.Logger, err error) {
    var errz *errorz.Errorz
    if errors.As(err, &errz) {
        log.ErrorWithContext(ctx, "Error occurred",
            logger.F("error_code", errz.Code),
            logger.F("error_message", errz.Message),
            logger.F("source_system", errz.SourceSystem),
            logger.F("metadata", errz.Meta),
        )
        
        // Also log wrapped error if present
        if errz.Err != nil {
            log.ErrorWithContext(ctx, "Wrapped error",
                logger.F("wrapped_error", errz.Err.Error()),
            )
        }
    } else {
        // Handle standard errors
        log.ErrorWithContext(ctx, "Standard error",
            logger.F("error", err.Error()),
        )
    }
}
```

### Testing Error Handling

```go
package service_test

import (
    "errors"
    "testing"
    "github.com/biairmal/go-sdk/errorz"
)

func TestService_HandleError(t *testing.T) {
    targetErr := errors.New("target error")
    wrappedErr := errorz.Wrap(targetErr).
        WithCode("TEST_001").
        WithMessage("test error")
    
    // Test error wrapping
    if !errors.Is(wrappedErr, targetErr) {
        t.Error("wrapped error should match target error")
    }
    
    // Test error code
    if wrappedErr.Code != "TEST_001" {
        t.Errorf("expected code TEST_001, got %s", wrappedErr.Code)
    }
    
    // Test metadata
    err := errorz.New("test").
        WithMeta("key", "value")
    
    if err.Meta["key"] != "value" {
        t.Error("metadata not set correctly")
    }
}
```

## API Reference

### Types

#### Errorz

```go
type Errorz struct {
    Code         string
    Message      string
    SourceSystem string
    Err          error
    Meta         map[string]any
}
```

The main error type that implements the `error` interface and supports error wrapping.

### Functions

#### New

```go
func New(message string) *Errorz
```

Creates a new `Errorz` instance with the specified message. The `SourceSystem` is set to `DefaultSourceSystem`.

#### Wrap

```go
func Wrap(err error) *Errorz
```

Wraps an existing error into an `Errorz` instance. The wrapped error can be accessed via `Unwrap()` or checked using `errors.Is()`.

### Methods

#### Error

```go
func (e *Errorz) Error() string
```

Returns the error message, implementing the `error` interface.

#### Unwrap

```go
func (e *Errorz) Unwrap() error
```

Returns the underlying error that was wrapped, if any. Implements the `Unwrap` interface for `errors.Is()` and `errors.As()`.

#### Is

```go
func (e *Errorz) Is(target error) bool
```

Checks if the `Errorz` wraps an error that matches the target error. Implements the `Is` interface for `errors.Is()`.

#### WithCode

```go
func (e *Errorz) WithCode(code string) *Errorz
```

Sets the error code and returns the receiver for method chaining.

#### WithMessage

```go
func (e *Errorz) WithMessage(message string) *Errorz
```

Sets the error message and returns the receiver for method chaining.

#### WithSourceSystem

```go
func (e *Errorz) WithSourceSystem(sourceSystem string) *Errorz
```

Sets the source system identifier and returns the receiver for method chaining.

#### WithMeta

```go
func (e *Errorz) WithMeta(key string, value any) *Errorz
```

Adds a key-value pair to the metadata map and returns the receiver for method chaining. Initialises the `Meta` map if it is `nil`.

### Variables

#### Predefined Errors

- `ErrNotFound`
- `ErrBadRequest`
- `ErrInternal`
- `ErrUnauthorized`
- `ErrForbidden`
- `ErrTooManyRequests`
- `ErrBadGateway`
- `ErrServiceUnavailable`
- `ErrUnprocessableEntity`
- `ErrConflict`
- `ErrPreconditionFailed`
- `ErrPreconditionRequired`
- `ErrPreconditionNotMet`

#### DefaultSourceSystem

```go
var DefaultSourceSystem = "application"
```

The default value used for the `SourceSystem` field when creating new `Errorz` instances.

## Dependencies

- Standard library `errors` package

## License

See the main repository license file.
