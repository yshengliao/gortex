// Package middleware provides common middleware for the Gortex framework
package middleware

import (
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// RequestIDConfig defines the config for RequestID middleware.
type RequestIDConfig struct {
	// Skipper defines a function to skip middleware.
	Skipper middleware.Skipper

	// Generator defines a function to generate an ID.
	// Optional. Defaults to UUID v4.
	Generator func() string

	// RequestIDHandler defines a function which is executed for a request id.
	RequestIDHandler func(echo.Context, string)

	// TargetHeader defines the header name to look for existing request ID.
	// Optional. Defaults to X-Request-ID
	TargetHeader string
}

// DefaultRequestIDConfig is the default RequestID middleware config.
var DefaultRequestIDConfig = RequestIDConfig{
	Skipper:      middleware.DefaultSkipper,
	Generator:    generateRequestID,
	TargetHeader: echo.HeaderXRequestID,
}

// generateRequestID generates a new request ID using UUID v4
func generateRequestID() string {
	return uuid.New().String()
}

// RequestID returns a middleware that generates a unique request ID for each request.
// The middleware will:
// 1. Check if a request ID already exists in the incoming request headers
// 2. Generate a new UUID v4 if no existing ID is found
// 3. Set the request ID in both the context and response headers
// 4. Make the request ID available throughout the request lifecycle
func RequestID() echo.MiddlewareFunc {
	return RequestIDWithConfig(DefaultRequestIDConfig)
}

// RequestIDWithConfig returns a RequestID middleware with config.
func RequestIDWithConfig(config RequestIDConfig) echo.MiddlewareFunc {
	// Defaults
	if config.Skipper == nil {
		config.Skipper = DefaultRequestIDConfig.Skipper
	}
	if config.Generator == nil {
		config.Generator = DefaultRequestIDConfig.Generator
	}
	if config.TargetHeader == "" {
		config.TargetHeader = DefaultRequestIDConfig.TargetHeader
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if config.Skipper(c) {
				return next(c)
			}

			req := c.Request()
			res := c.Response()

			// Check if request ID already exists in the request headers
			rid := req.Header.Get(config.TargetHeader)
			if rid == "" {
				// Generate new request ID
				rid = config.Generator()
			}

			// Set request ID in response header
			res.Header().Set(config.TargetHeader, rid)

			// Store request ID in context for easy access
			c.Set("request_id", rid)

			// Execute handler if configured
			if config.RequestIDHandler != nil {
				config.RequestIDHandler(c, rid)
			}

			return next(c)
		}
	}
}