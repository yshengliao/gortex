package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/yshengliao/gortex/pkg/requestid"
	"github.com/yshengliao/gortex/pkg/router"
)

// RequestIDConfig contains configuration for the request ID middleware
type RequestIDConfig struct {
	// Header is the name of the header to read/write the request ID
	Header string
	// Generator is a function that generates a new request ID
	Generator func() string
	// SkipPaths is a list of paths to skip
	SkipPaths []string
}

// DefaultRequestIDConfig returns the default configuration
func DefaultRequestIDConfig() *RequestIDConfig {
	return &RequestIDConfig{
		Header: requestid.HeaderXRequestID,
		Generator: func() string {
			return uuid.New().String()
		},
		SkipPaths: []string{},
	}
}

// RequestID returns a middleware that adds a request ID to each request
func RequestID() router.Middleware {
	return RequestIDWithConfig(DefaultRequestIDConfig())
}

// RequestIDWithConfig returns a middleware with custom configuration
func RequestIDWithConfig(config *RequestIDConfig) router.Middleware {
	// Apply defaults
	if config == nil {
		config = DefaultRequestIDConfig()
	}
	if config.Header == "" {
		config.Header = requestid.HeaderXRequestID
	}
	if config.Generator == nil {
		config.Generator = func() string {
			return uuid.New().String()
		}
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

			// Get or generate request ID
			id := req.Header.Get(config.Header)
			if id == "" {
				id = config.Generator()
				req.Header.Set(config.Header, id)
			}

			// Store in request context using standard context
			ctx := context.WithValue(req.Context(), "request_id", id)
			newReq := req.WithContext(ctx)
			
			// Update the request in context if possible
			if setter, ok := c.(interface{ SetRequest(*http.Request) }); ok {
				setter.SetRequest(newReq)
			}

			// Set in router context for easy access
			c.Set("request_id", id)

			// Also set response header
			if resp, ok := c.Response().(http.ResponseWriter); ok {
				resp.Header().Set(config.Header, id)
			}

			return next(c)
		}
	}
}