package handler

import (
	"errors"
	"net/http"

	"github.com/biairmal/go-sdk/errorz"
)

var defaultCodeToStatus = map[string]int{
	errorz.CodeNotFound:             http.StatusNotFound,
	errorz.CodeBadRequest:           http.StatusBadRequest,
	errorz.CodeInternal:             http.StatusInternalServerError,
	errorz.CodeUnauthorized:         http.StatusUnauthorized,
	errorz.CodeForbidden:            http.StatusForbidden,
	errorz.CodeTooManyRequests:      http.StatusTooManyRequests,
	errorz.CodeBadGateway:           http.StatusBadGateway,
	errorz.CodeServiceUnavailable:   http.StatusServiceUnavailable,
	errorz.CodeUnprocessableEntity:  http.StatusUnprocessableEntity,
	errorz.CodeConflict:             http.StatusConflict,
	errorz.CodePreconditionFailed:   http.StatusPreconditionFailed,
	errorz.CodePreconditionRequired: http.StatusPreconditionRequired,
	errorz.CodePreconditionNotMet:   http.StatusPreconditionFailed,
}

// StatusCodeFromError returns the HTTP status code for the given error.
// If the error is a *errorz.Error, its Code is looked up in the default map.
// Otherwise it returns http.StatusInternalServerError.
func StatusCodeFromError(err error) int {
	if err == nil {
		return http.StatusOK
	}
	var errz *errorz.Error
	if errors.As(err, &errz) && errz != nil && errz.Code != "" {
		if status, ok := defaultCodeToStatus[errz.Code]; ok {
			return status
		}
	}
	return http.StatusInternalServerError
}
