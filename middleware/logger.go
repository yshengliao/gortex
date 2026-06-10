package middleware

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// statusReader is the subset of the response writer the logger needs to read
// the real status that went on the wire. DefaultContext's tracked
// responseWriter implements it (see transport/http/response_writer.go), as
// does the framework's test response writer.
type statusReader interface {
	Status() int
	Written() bool
}

// LoggerConfig contains configuration for the logger middleware
type LoggerConfig struct {
	// Logger is the zap logger to use
	Logger *zap.Logger
	// SkipPaths is a list of paths to skip logging
	SkipPaths []string
	// LogRequestBody logs the request body (be careful with sensitive data)
	LogRequestBody bool
	// LogResponseBody is deprecated and has no effect.
	//
	// Capturing the response body requires swapping the context's response
	// writer for a body-recording wrapper, which the framework's pooled
	// DefaultContext does not support (it owns a single tracked writer for
	// the request's lifetime). The previous implementation relied on an
	// optional SetResponse hook that no context type implemented, so the
	// capture never ran and every request logged an empty body and a
	// hardcoded status 200. The dead path has been removed; the logger now
	// reads the real status from the tracked writer instead.
	//
	// Request-body redaction (LogRequestBody + BodyRedactor) is unaffected
	// and continues to mask sensitive JSON fields. To log response bodies,
	// wrap the response writer in your own middleware.
	LogResponseBody bool
	// BodyLogLimit is the maximum size of body to log
	BodyLogLimit int
	// TrustedProxies lists the CIDR ranges whose requests are allowed to
	// set X-Real-IP or X-Forwarded-For. If nil or empty the forwarding
	// headers are ignored entirely and the logger always reports the
	// direct peer address. This stops attackers from forging client IPs
	// by sending a proxy header through an internet-facing listener.
	TrustedProxies []*net.IPNet
	// BodyRedactor transforms captured request/response bodies before
	// they reach the log sink. When nil and body logging is enabled, the
	// middleware falls back to DefaultBodyRedactor so sensitive fields
	// such as passwords or API keys are not persisted by accident.
	// Explicitly set to a no-op (func(b []byte) []byte { return b }) to
	// opt out.
	BodyRedactor func([]byte) []byte
}

// DefaultLoggerConfig returns the default configuration
func DefaultLoggerConfig(logger *zap.Logger) *LoggerConfig {
	return &LoggerConfig{
		Logger:          logger,
		SkipPaths:       []string{"/health", "/metrics"},
		LogRequestBody:  false,
		LogResponseBody: false,
		BodyLogLimit:    1024, // 1KB
	}
}

// Logger returns a middleware that logs HTTP requests
func Logger(logger *zap.Logger) MiddlewareFunc {
	return LoggerWithConfig(DefaultLoggerConfig(logger))
}

// LoggerWithConfig returns a middleware with custom configuration
func LoggerWithConfig(config *LoggerConfig) MiddlewareFunc {
	// Apply defaults
	if config == nil {
		panic("LoggerConfig cannot be nil")
	}
	if config.Logger == nil {
		panic("Logger cannot be nil")
	}
	if config.BodyLogLimit == 0 {
		config.BodyLogLimit = 1024
	}
	if config.BodyRedactor == nil {
		config.BodyRedactor = DefaultBodyRedactor
	}

	return func(next HandlerFunc) HandlerFunc {
		return func(c Context) error {
			req := c.Request()

			// Skip if path is in skip list
			for _, skip := range config.SkipPaths {
				if req.URL.Path == skip {
					return next(c)
				}
			}

			// Start timer
			start := time.Now()

			// Get request ID
			requestID := c.Get("request_id")
			if requestID == nil {
				requestID = ""
			}

			// Log request body if enabled
			var requestBody []byte
			if config.LogRequestBody && req.Body != nil {
				requestBody, _ = io.ReadAll(req.Body)
				req.Body = io.NopCloser(bytes.NewReader(requestBody))
				if len(requestBody) > config.BodyLogLimit {
					requestBody = requestBody[:config.BodyLogLimit]
				}
			}

			// Process request. The error-handler middleware runs *inside* the
			// logger (it is registered after Logger, so the router wraps it
			// closer to the route handler), which means by the time next(c)
			// returns the real status has already been written to the
			// context's tracked response writer — both for handlers that write
			// a status directly and for errors the error handler converts.
			err := next(c)

			// Calculate latency
			latency := time.Since(start)

			// Read the real status from the tracked response writer. If the
			// writer does not expose Status()/Written() (a custom context),
			// fall back to 200 rather than guessing.
			status := http.StatusOK
			if sr, ok := c.Response().(statusReader); ok && sr.Written() {
				status = sr.Status()
			}

			// Build log fields
			fields := []zap.Field{
				zap.String("method", req.Method),
				zap.String("path", req.URL.Path),
				zap.Int("status", status),
				zap.Duration("latency", latency),
				zap.String("ip", clientIPFromRequest(req, config.TrustedProxies)),
				zap.String("user_agent", req.UserAgent()),
			}

			// comma-ok guards against a non-string value being stored under
			// "request_id" by other code; a bare assertion would panic here.
			if reqID, ok := requestID.(string); ok && reqID != "" {
				fields = append(fields, zap.String("request_id", reqID))
			}

			if config.LogRequestBody && len(requestBody) > 0 {
				fields = append(fields, zap.ByteString("request_body", config.BodyRedactor(requestBody)))
			}

			if err != nil {
				fields = append(fields, zap.Error(err))
			}

			// Log based on the real status code. A handler that returned an
			// error which the error handler then converted to a 4xx/5xx is
			// already covered by the status check, so the error alone no
			// longer forces an Error-level line for a 4xx response.
			switch {
			case status >= 500:
				config.Logger.Error("Request failed", fields...)
			case status >= 400:
				config.Logger.Warn("Request error", fields...)
			case err != nil:
				// The handler returned an error but nothing reached the wire
				// (no status written) — surface it loudly so it is not lost.
				config.Logger.Error("Request failed", fields...)
			default:
				config.Logger.Info("Request completed", fields...)
			}

			return err
		}
	}
}
