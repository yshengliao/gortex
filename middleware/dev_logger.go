package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

// DevLoggerConfig defines the config for development logger middleware
type DevLoggerConfig struct {
	// Logger instance to use
	Logger *zap.Logger

	// LogRequestBody logs request body content
	LogRequestBody bool

	// LogResponseBody logs response body content
	LogResponseBody bool

	// MaxBodySize limits the body size to log (default: 10KB)
	MaxBodySize int

	// SkipPaths to skip logging
	SkipPaths []string

	// SensitiveHeaders to mask in logs
	SensitiveHeaders []string
}

// DefaultDevLoggerConfig returns default config
var DefaultDevLoggerConfig = DevLoggerConfig{
	LogRequestBody:  true,
	LogResponseBody: true,
	MaxBodySize:     10 * 1024, // 10KB
	SensitiveHeaders: []string{
		"Authorization",
		"Cookie",
		"X-Api-Key",
		"X-Auth-Token",
	},
}

// DevLogger returns a development logger middleware
func DevLogger() echo.MiddlewareFunc {
	return DevLoggerWithConfig(DefaultDevLoggerConfig)
}

// DevLoggerWithConfig returns a development logger middleware with config
func DevLoggerWithConfig(config DevLoggerConfig) echo.MiddlewareFunc {
	// Defaults
	if config.Logger == nil {
		logger, _ := zap.NewDevelopment()
		config.Logger = logger
	}
	if config.MaxBodySize == 0 {
		config.MaxBodySize = DefaultDevLoggerConfig.MaxBodySize
	}
	if len(config.SensitiveHeaders) == 0 {
		config.SensitiveHeaders = DefaultDevLoggerConfig.SensitiveHeaders
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Skip if path is in skip list
			path := c.Request().URL.Path
			for _, skipPath := range config.SkipPaths {
				if strings.HasPrefix(path, skipPath) {
					return next(c)
				}
			}

			start := time.Now()
			
			// Capture request info
			reqInfo := captureRequestInfo(c, config)

			// Capture response using custom response writer
			resBody := new(bytes.Buffer)
			mw := io.MultiWriter(c.Response().Writer, resBody)
			writer := &responseWriter{
				Writer:         mw,
				ResponseWriter: c.Response().Writer,
			}
			c.Response().Writer = writer

			// Process request
			err := next(c)

			// Calculate duration
			duration := time.Since(start)

			// Log the request/response
			fields := []zap.Field{
				zap.String("method", c.Request().Method),
				zap.String("path", path),
				zap.Int("status", c.Response().Status),
				zap.Duration("duration", duration),
				zap.String("request_id", c.Response().Header().Get(echo.HeaderXRequestID)),
				zap.String("remote_ip", c.RealIP()),
				zap.String("user_agent", c.Request().UserAgent()),
			}

			// Add request headers (mask sensitive ones)
			headers := make(map[string]string)
			for key, values := range c.Request().Header {
				if isSensitiveHeader(key, config.SensitiveHeaders) {
					headers[key] = "[MASKED]"
				} else {
					headers[key] = strings.Join(values, ", ")
				}
			}
			fields = append(fields, zap.Any("request_headers", headers))

			// Add query parameters
			if c.QueryString() != "" {
				fields = append(fields, zap.String("query", c.QueryString()))
			}

			// Add request body if configured
			if config.LogRequestBody && reqInfo.body != "" {
				fields = append(fields, zap.String("request_body", reqInfo.body))
			}

			// Add response body if configured
			if config.LogResponseBody && resBody.Len() > 0 {
				respBodyStr := resBody.String()
				if len(respBodyStr) > config.MaxBodySize {
					respBodyStr = respBodyStr[:config.MaxBodySize] + "... [truncated]"
				}
				fields = append(fields, zap.String("response_body", respBodyStr))
			}

			// Add error if any
			if err != nil {
				fields = append(fields, zap.Error(err))
				config.Logger.Error("Request failed", fields...)
			} else if c.Response().Status >= 400 {
				config.Logger.Warn("Request completed with error status", fields...)
			} else {
				config.Logger.Info("Request completed", fields...)
			}

			return err
		}
	}
}

// requestInfo holds captured request information
type requestInfo struct {
	body string
}

// captureRequestInfo captures request information before processing
func captureRequestInfo(c echo.Context, config DevLoggerConfig) *requestInfo {
	info := &requestInfo{}

	if config.LogRequestBody && c.Request().Body != nil {
		// Read body
		bodyBytes, err := io.ReadAll(c.Request().Body)
		if err == nil {
			// Restore body for handler
			c.Request().Body = io.NopCloser(bytes.NewReader(bodyBytes))
			
			// Store body content
			if len(bodyBytes) > 0 {
				if len(bodyBytes) > config.MaxBodySize {
					info.body = string(bodyBytes[:config.MaxBodySize]) + "... [truncated]"
				} else {
					info.body = string(bodyBytes)
				}
				
				// Try to pretty print JSON
				if isJSON(c.Request().Header.Get(echo.HeaderContentType)) {
					var jsonData any
					if err := json.Unmarshal(bodyBytes, &jsonData); err == nil {
						prettyBytes, _ := json.MarshalIndent(jsonData, "", "  ")
						info.body = string(prettyBytes)
					}
				}
			}
		}
	}

	return info
}

// responseWriter wraps http.ResponseWriter to capture response body
type responseWriter struct {
	io.Writer
	http.ResponseWriter
}

func (w *responseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

// isSensitiveHeader checks if a header should be masked
func isSensitiveHeader(header string, sensitiveHeaders []string) bool {
	header = strings.ToLower(header)
	for _, sensitive := range sensitiveHeaders {
		if strings.ToLower(sensitive) == header {
			return true
		}
	}
	return false
}

// isJSON checks if content type is JSON
func isJSON(contentType string) bool {
	return strings.Contains(strings.ToLower(contentType), "application/json")
}