// Package response provides a consistent response envelope and JSON writer
// for HTTP APIs. It defines BaseResponse, ErrorPayload, and JSON writing
// used by the httpkit handler and middlewares.
package response

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/biairmal/go-sdk/errorz"
)

// BaseResponse is the base response struct for all API responses.
// Use Data for success and Error for error responses; keep the other field nil/zero.
type BaseResponse[T any] struct {
	Code      string    `json:"code,omitempty"`
	Message   string    `json:"message,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Data      T         `json:"data,omitempty"`
	Error     any       `json:"error,omitempty"`
}

// ErrorPayload is the normalised error shape for JSON responses.
// It is populated from errorz.Error when present, or from a generic message for other errors.
type ErrorPayload struct {
	Code         string         `json:"code"`
	Message      string         `json:"message"`
	SourceSystem string         `json:"source_system,omitempty"`
	Meta         map[string]any `json:"meta,omitempty"`
	Details      string         `json:"details,omitempty"`
}

// ErrorFromErr builds an ErrorPayload from an error.
// If the error is a *errorz.Error, Code, Message, SourceSystem, and Meta are copied.
// Otherwise a generic payload with code "ERR_INTERNAL" and the error string as message is returned.
func ErrorFromErr(err error) ErrorPayload {
	if err == nil {
		return ErrorPayload{Code: "ERR_INTERNAL", Message: "unknown error"}
	}
	var errz *errorz.Error
	if errors.As(err, &errz) && errz != nil {
		return ErrorPayload{
			Code:         nonEmpty(errz.Code, "ERR_INTERNAL"),
			Message:      nonEmpty(errz.Message, errz.Error()),
			SourceSystem: errz.SourceSystem,
			Meta:         errz.Meta,
		}
	}
	return ErrorPayload{
		Code:    "ERR_INTERNAL",
		Message: err.Error(),
	}
}

func nonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// JSON writes the response data to the response writer.
// It sets the Content-Type header to application/json and writes the data.
func JSON(w http.ResponseWriter, statusCode int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if data == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(data); err != nil {
		// Header already written; cannot send another status. Log or ignore.
		_ = err
	}
}
