package middleware

import (
	"bytes"
	"io"
	"net/http"
	"time"

	"github.com/biairmal/go-sdk/lib/logger"
)

// defaultMaxBodyBytesForLogging is the most body bytes the middleware keeps for one log line when
// LoggingOptions.MaxBodyBytesForLogging is not set.
const defaultMaxBodyBytesForLogging = 4096

// LoggingOptions controls what the logging middleware logs.
// Nil means default: log the request and the response line (path, ip, method, status, duration) and no bodies.
type LoggingOptions struct {
	// LogRequest logs the incoming request (path, IP, method, optional body).
	LogRequest bool
	// LogResponse logs the completed response (path, IP, method, status, duration, optional body).
	LogResponse bool
	// LogRequestBody includes the request body in the request log when present. Off by default: bodies carry
	// passwords, tokens and personal data.
	LogRequestBody bool
	// LogResponseBody includes the response body in the response log when present. Off by default.
	LogResponseBody bool
	// MaxBodyBytesForLogging is the most bytes of one body that are read for logging; the rest is not kept and the
	// handler still sees the whole request body. Zero or negative means 4096.
	MaxBodyBytesForLogging int
	// IPResolver finds the client IP for the "ip" field. Nil trusts no proxy: the TCP peer address is logged and
	// X-Forwarded-For is ignored. See IPResolver.
	IPResolver *IPResolver
}

func defaultLoggingOptions() *LoggingOptions {
	return &LoggingOptions{LogRequest: true, LogResponse: true}
}

// bodyLimit returns the number of body bytes to keep for one log line.
func (o *LoggingOptions) bodyLimit() int {
	if o.MaxBodyBytesForLogging > 0 {
		return o.MaxBodyBytesForLogging
	}
	return defaultMaxBodyBytesForLogging
}

// Logging returns a middleware that logs requests and responses using the given logger.
// If opts is nil, defaults are used: request and response lines, no bodies.
func Logging(log logger.Logger, opts *LoggingOptions) func(http.Handler) http.Handler {
	if opts == nil {
		opts = defaultLoggingOptions()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			path, clientIPAddr, method := requestPath(r), opts.IPResolver.ClientIP(r), r.Method
			reqBody := maybeReadRequestBody(r, opts)
			maybeLogRequest(log, r, opts, path, clientIPAddr, method, reqBody)

			var capture *responseCapture
			if opts.LogResponse {
				capture = &responseCapture{ResponseWriter: w, status: http.StatusOK}
				if opts.LogResponseBody {
					capture.max = opts.bodyLimit()
				}
				w = capture
			}
			next.ServeHTTP(w, r)
			maybeLogResponse(log, r, opts, path, clientIPAddr, method, start, capture)
		})
	}
}

// requestPath returns the URL path of r, or its request URI when the path is empty.
func requestPath(r *http.Request) string {
	if r.URL.Path == "" {
		return r.URL.RequestURI()
	}
	return r.URL.Path
}

// maybeReadRequestBody reads at most the log limit from the start of the request body and puts those bytes back in
// front of the unread rest, so the handler sees the whole body and the middleware never holds more than the limit.
func maybeReadRequestBody(r *http.Request, opts *LoggingOptions) []byte {
	if !opts.LogRequest || !opts.LogRequestBody || r.Body == nil || r.Body == http.NoBody {
		return nil
	}
	head, err := io.ReadAll(io.LimitReader(r.Body, int64(opts.bodyLimit())))
	r.Body = struct {
		io.Reader
		io.Closer
	}{Reader: io.MultiReader(bytes.NewReader(head), r.Body), Closer: r.Body}
	if err != nil {
		return nil // the handler hits the same error when it reads the rest
	}
	return head
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
		fields = append(fields, logger.F("body", capture.buf.String()))
	}
	log.InfoWithContext(r.Context(), "http response", fields...)
}

// responseCapture records the status code of a response and, when max > 0, the first max bytes of its body.
// Metrics and Tracing only need the status, so they leave max at 0 and nothing is copied.
type responseCapture struct {
	http.ResponseWriter
	status int
	buf    bytes.Buffer
	max    int
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
	if room := c.max - c.buf.Len(); room > 0 {
		c.buf.Write(p[:min(room, len(p))])
	}
	return c.ResponseWriter.Write(p)
}

// Unwrap allows middleware to expose the underlying ResponseWriter for optional checks.
func (c *responseCapture) Unwrap() http.ResponseWriter {
	return c.ResponseWriter
}
