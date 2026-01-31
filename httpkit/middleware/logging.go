package middleware

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/biairmal/go-sdk/logger"
)

// LoggingOptions controls what the logging middleware logs.
// Nil means default: log request and response with full info including bodies.
type LoggingOptions struct {
	// LogRequest logs the incoming request (path, IP, method, optional body).
	LogRequest bool
	// LogResponse logs the completed response (path, IP, method, status, duration, optional body).
	LogResponse bool
	// LogRequestBody includes the request body in the request log when present.
	LogRequestBody bool
	// LogResponseBody includes the response body in the response log when present.
	LogResponseBody bool
	// MaxBodyBytesForLogging limits how many bytes of request/response body are logged.
	// Zero means no limit. For example 4096 logs the first 4KB only.
	MaxBodyBytesForLogging int
}

func defaultLoggingOptions() *LoggingOptions {
	return &LoggingOptions{
		LogRequest:      true,
		LogResponse:     true,
		LogRequestBody:  true,
		LogResponseBody: true,
	}
}

// Logging returns a middleware that logs requests and responses using the given logger.
// If opts is nil, defaults are used (log request and response with path, IP, method, body).
func Logging(log logger.Logger, opts *LoggingOptions) func(http.Handler) http.Handler {
	if opts == nil {
		opts = defaultLoggingOptions()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			path, clientIPAddr, method := requestMeta(r)
			reqBody := maybeReadRequestBody(r, opts)
			maybeLogRequest(log, r, opts, path, clientIPAddr, method, reqBody)

			var capture *responseCapture
			if opts.LogResponse {
				capture = &responseCapture{ResponseWriter: w, status: http.StatusOK}
				w = capture
			}
			next.ServeHTTP(w, r)
			maybeLogResponse(log, r, opts, path, clientIPAddr, method, start, capture)
		})
	}
}

func requestMeta(r *http.Request) (path, clientIPAddr, method string) {
	path = r.URL.Path
	if path == "" {
		path = r.URL.RequestURI()
	}
	return path, clientIP(r), r.Method
}

func maybeReadRequestBody(r *http.Request, opts *LoggingOptions) []byte {
	if !opts.LogRequest || !opts.LogRequestBody || r.Body == nil {
		return nil
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		body = nil
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	return truncateForLog(body, opts.MaxBodyBytesForLogging)
}

func maybeLogRequest(
	log logger.Logger, r *http.Request, opts *LoggingOptions,
	path, clientIPAddr, method string, reqBody []byte,
) {
	if !opts.LogRequest {
		return
	}
	fields := []logger.Field{
		logger.F("path", path),
		logger.F("ip", clientIPAddr),
		logger.F("method", method),
	}
	if len(reqBody) > 0 {
		fields = append(fields, logger.F("body", string(reqBody)))
	}
	log.InfoWithContext(r.Context(), "http request", fields...)
}

func maybeLogResponse(
	log logger.Logger, r *http.Request, opts *LoggingOptions,
	path, clientIPAddr, method string, start time.Time, capture *responseCapture,
) {
	if !opts.LogResponse || capture == nil {
		return
	}
	fields := []logger.Field{
		logger.F("path", path),
		logger.F("ip", clientIPAddr),
		logger.F("method", method),
		logger.F("status", capture.status),
		logger.F("duration_ms", time.Since(start).Milliseconds()),
	}
	if opts.LogResponseBody && capture.buf.Len() > 0 {
		body := truncateForLog(capture.buf.Bytes(), opts.MaxBodyBytesForLogging)
		fields = append(fields, logger.F("body", string(body)))
	}
	log.InfoWithContext(r.Context(), "http response", fields...)
}

func clientIP(r *http.Request) string {
	if s := r.Header.Get("X-Forwarded-For"); s != "" {
		if i := strings.Index(s, ","); i >= 0 {
			return strings.TrimSpace(s[:i])
		}
		return strings.TrimSpace(s)
	}
	if s := r.Header.Get("X-Real-IP"); s != "" {
		return s
	}
	return r.RemoteAddr
}

func truncateForLog(b []byte, limit int) []byte {
	if limit <= 0 || len(b) <= limit {
		return b
	}
	return b[:limit]
}

type responseCapture struct {
	http.ResponseWriter
	status int
	buf    bytes.Buffer
	wrote  bool
}

func (c *responseCapture) WriteHeader(code int) {
	if !c.wrote {
		c.status = code
		c.wrote = true
		c.ResponseWriter.WriteHeader(code)
	}
}

func (c *responseCapture) Write(p []byte) (n int, err error) {
	if !c.wrote {
		c.WriteHeader(http.StatusOK)
	}
	c.buf.Write(p)
	return c.ResponseWriter.Write(p)
}

// Unwrap allows middleware to expose the underlying ResponseWriter for optional checks.
func (c *responseCapture) Unwrap() http.ResponseWriter {
	return c.ResponseWriter
}
