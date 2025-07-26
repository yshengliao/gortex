package middleware

import (
	"bytes"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"
	"github.com/yshengliao/gortex/http/router"
)

// LoggerConfig contains configuration for the logger middleware
type LoggerConfig struct {
	// Logger is the zap logger to use
	Logger *zap.Logger
	// SkipPaths is a list of paths to skip logging
	SkipPaths []string
	// LogRequestBody logs the request body (be careful with sensitive data)
	LogRequestBody bool
	// LogResponseBody logs the response body (be careful with sensitive data)
	LogResponseBody bool
	// BodyLogLimit is the maximum size of body to log
	BodyLogLimit int
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
func Logger(logger *zap.Logger) router.Middleware {
	return LoggerWithConfig(DefaultLoggerConfig(logger))
}

// LoggerWithConfig returns a middleware with custom configuration
func LoggerWithConfig(config *LoggerConfig) router.Middleware {
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

	return func(next router.HandlerFunc) router.HandlerFunc {
		return func(c router.Context) error {
			req, ok := c.Request().(*http.Request)
			if !ok {
				return next(c)
			}

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

			// Create response writer wrapper to capture status code
			resp, ok := c.Response().(http.ResponseWriter)
			if !ok {
				return next(c)
			}
			
			rw := &responseWriter{
				ResponseWriter: resp,
				statusCode:     http.StatusOK,
			}

			// Replace response writer in context if possible
			if setter, ok := c.(interface{ SetResponse(http.ResponseWriter) }); ok {
				setter.SetResponse(rw)
			}

			// Process request
			err := next(c)

			// Calculate latency
			latency := time.Since(start)

			// Build log fields
			fields := []zap.Field{
				zap.String("method", req.Method),
				zap.String("path", req.URL.Path),
				zap.Int("status", rw.statusCode),
				zap.Duration("latency", latency),
				zap.String("ip", getClientIP(req)),
				zap.String("user_agent", req.UserAgent()),
			}

			if requestID != "" {
				fields = append(fields, zap.String("request_id", requestID.(string)))
			}

			if config.LogRequestBody && len(requestBody) > 0 {
				fields = append(fields, zap.ByteString("request_body", requestBody))
			}

			if config.LogResponseBody && len(rw.body) > 0 {
				body := rw.body
				if len(body) > config.BodyLogLimit {
					body = body[:config.BodyLogLimit]
				}
				fields = append(fields, zap.ByteString("response_body", body))
			}

			if err != nil {
				fields = append(fields, zap.Error(err))
			}

			// Log based on status code
			if err != nil || rw.statusCode >= 500 {
				config.Logger.Error("Request failed", fields...)
			} else if rw.statusCode >= 400 {
				config.Logger.Warn("Request error", fields...)
			} else {
				config.Logger.Info("Request completed", fields...)
			}

			return err
		}
	}
}

// responseWriter wraps http.ResponseWriter to capture status code and body
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	body       []byte
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	// Capture body for logging (be careful with memory usage)
	if len(rw.body) < 1024 { // Limit to 1KB
		rw.body = append(rw.body, b...)
	}
	return rw.ResponseWriter.Write(b)
}

// getClientIP gets the client IP address
func getClientIP(req *http.Request) string {
	// Check X-Real-IP header
	if ip := req.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	
	// Check X-Forwarded-For header
	if ip := req.Header.Get("X-Forwarded-For"); ip != "" {
		// Take the first IP if there are multiple
		if idx := bytes.IndexByte([]byte(ip), ','); idx >= 0 {
			return ip[:idx]
		}
		return ip
	}
	
	// Fall back to RemoteAddr
	return req.RemoteAddr
}