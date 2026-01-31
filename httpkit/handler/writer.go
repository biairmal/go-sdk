// Package handler provides an HTTP handler adapter that converts a function
// returning (any, error) into an http.HandlerFunc, and helpers to write
// success and error responses using the httpkit response envelope.
package handler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/biairmal/go-sdk/httpkit/response"
)

// WriteSuccessResponse writes a success response using the standard envelope.
func WriteSuccessResponse(w http.ResponseWriter, statusCode int, data any) {
	if statusCode == http.StatusNoContent {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	response.JSON(w, statusCode, response.BaseResponse[any]{
		Code:      "OK",
		Message:   "success",
		Timestamp: time.Now(),
		Data:      data,
	})
}

// WriteErrorResponse writes an error response using the standard envelope
// and ErrorPayload from the given error.
func WriteErrorResponse(w http.ResponseWriter, statusCode int, err any) {
	payload := response.ErrorFromErr(toError(err))
	response.JSON(w, statusCode, response.BaseResponse[any]{
		Code:      "ERROR",
		Message:   payload.Message,
		Timestamp: time.Now(),
		Error:     payload,
	})
}

func toError(v any) error {
	if v == nil {
		return nil
	}
	if err, ok := v.(error); ok {
		return err
	}
	// Panic or other value: wrap as string.
	return &stringError{s: stringOrSprint(v)}
}

type stringError struct{ s string }

func (e *stringError) Error() string { return e.s }

func stringOrSprint(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return fmtSprint(v)
}

func fmtSprint(v any) string {
	type stringer interface{ String() string }
	if s, ok := v.(stringer); ok {
		return s.String()
	}
	return fmt.Sprint(v)
}
