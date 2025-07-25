// Package middleware provides common middleware for the Gortex framework
package middleware

import (
	"github.com/google/uuid"
	"github.com/yshengliao/gortex/context"
)

// RequestIDConfig defines the config for RequestID middleware.
type RequestIDConfig struct {
	// Skipper defines a function to skip middleware.
	Skipper func(context.Context) bool

	// Generator defines a function to generate an ID.
	// Optional. Defaults to UUID v4.
	Generator func() string

	// RequestIDHandler defines a function which is executed for a request id.
	RequestIDHandler func(context.Context, string)

	// TargetHeader defines the header name to look for existing request ID.
	// Optional. Defaults to X-Request-ID
	TargetHeader string
}

// DefaultSkipper returns false which processes the middleware for all requests.
func DefaultSkipper(context.Context) bool {
	return false
}

// DefaultRequestIDConfig is the default RequestID middleware config.
var DefaultRequestIDConfig = RequestIDConfig{
	Skipper:      DefaultSkipper,
	Generator:    generateRequestID,
	TargetHeader: "X-Request-ID",
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
func RequestID() MiddlewareFunc {
	return RequestIDWithConfig(DefaultRequestIDConfig)
}

// RequestIDWithConfig returns a RequestID middleware with config.
func RequestIDWithConfig(config RequestIDConfig) MiddlewareFunc {
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

	return func(next HandlerFunc) HandlerFunc {
		return func(c context.Context) error {
			if config.Skipper(c) {
				return next(c)
			}

			req := c.Request()
			res := c.Response()
			
			// Try to get request ID from header
			rid := req.Header.Get(config.TargetHeader)
			if rid == "" {
				// Generate new request ID
				rid = config.Generator()
			}

			// Set request ID in response header
			res.Header().Set(config.TargetHeader, rid)
			
			// Store request ID in context for later use
			c.Set("request_id", rid)

			// Call the handler if configured
			if config.RequestIDHandler != nil {
				config.RequestIDHandler(c, rid)
			}

			return next(c)
		}
	}
}

// GetRequestID retrieves the request ID from context
func GetRequestID(c context.Context) string {
	if rid := c.Get("request_id"); rid != nil {
		if ridStr, ok := rid.(string); ok {
			return ridStr
		}
	}
	return ""
}
