// Package client provides a thin HTTP client that decodes responses
// into the httpkit response envelope (BaseResponse[T]).
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/biairmal/go-sdk/httpkit/response"
)

// Client wraps *http.Client and provides Do, Get, and Post helpers
// that decode the response body into response.BaseResponse[T].
type Client struct {
	HTTPClient *http.Client
}

// New returns a Client using the given *http.Client.
// If c is nil, http.DefaultClient is used.
func New(c *http.Client) *Client {
	if c == nil {
		c = http.DefaultClient
	}
	return &Client{HTTPClient: c}
}

// Do sends the request and decodes the response body into BaseResponse[T].
// The returned status code and body are from the HTTP response.
// If the response body is not valid JSON or does not match BaseResponse[T],
// Result is zero and Err may be set (caller can still use RawBody or StatusCode).
func Do[T any](ctx context.Context, c *Client, req *http.Request) (
	result response.BaseResponse[T], statusCode int, rawBody []byte, err error,
) {
	if c == nil {
		c = New(nil)
	}
	req = req.WithContext(ctx)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return result, 0, nil, err
	}
	defer resp.Body.Close()
	rawBody, err = io.ReadAll(resp.Body)
	if err != nil {
		return result, resp.StatusCode, rawBody, err
	}
	statusCode = resp.StatusCode
	if len(rawBody) == 0 {
		return result, statusCode, rawBody, nil
	}
	if err := json.Unmarshal(rawBody, &result); err != nil {
		return result, statusCode, rawBody, err
	}
	return result, statusCode, rawBody, nil
}

// Get builds a GET request to url and calls Do.
func Get[T any](ctx context.Context, c *Client, url string) (
	result response.BaseResponse[T], statusCode int, rawBody []byte, err error,
) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		var zero response.BaseResponse[T]
		return zero, 0, nil, err
	}
	return Do[T](ctx, c, req)
}

// Post builds a POST request to url with body and calls Do.
func Post[T any](ctx context.Context, c *Client, url string, body any) (
	result response.BaseResponse[T], statusCode int, rawBody []byte, err error,
) {
	var bodyReader io.Reader = http.NoBody
	if body != nil {
		b, marshalErr := json.Marshal(body)
		if marshalErr != nil {
			var zero response.BaseResponse[T]
			return zero, 0, nil, marshalErr
		}
		bodyReader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bodyReader)
	if err != nil {
		var zero response.BaseResponse[T]
		return zero, 0, nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return Do[T](ctx, c, req)
}
