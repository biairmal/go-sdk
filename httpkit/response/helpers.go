package response

import "net/http"

// OK returns a successful response with the given data (HTTP 200).
func OK(data any) *Success {
	return &Success{
		HTTPStatusCode: http.StatusOK,
		Data:           data,
	}
}

// Created returns a successful response with the given data (HTTP 201).
func Created(data any) *Success {
	return &Success{
		HTTPStatusCode: http.StatusCreated,
		Data:           data,
	}
}

// NoContent returns a successful response with no content (HTTP 204).
func NoContent() *Success {
	return &Success{
		HTTPStatusCode: http.StatusNoContent,
		Data:           nil,
	}
}
