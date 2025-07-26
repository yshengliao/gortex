// Package requestid provides utilities for working with request IDs throughout the application
package requestid

import (
	"context"
	"io"
	"net/http"

	gortexContext "github.com/yshengliao/gortex/http/context"
	"go.uber.org/zap"
)

// contextKey is a type for context keys to avoid collisions
type contextKey string

const (
	// RequestIDKey is the key used to store request ID in context
	RequestIDKey contextKey = "request_id"

	// HeaderXRequestID is the standard header name for request ID
	HeaderXRequestID = "X-Request-ID"
)

// FromGortexContext extracts the request ID from a Gortex context
func FromGortexContext(c gortexContext.Context) string {
	// First try to get from context value (set by our middleware)
	if rid, ok := c.Get("request_id").(string); ok && rid != "" {
		return rid
	}

	// Fallback to checking response header (set by middleware)
	if rid := c.Response().Header().Get(HeaderXRequestID); rid != "" {
		return rid
	}

	// Finally check request header
	if rid := c.Request().Header.Get(HeaderXRequestID); rid != "" {
		return rid
	}

	return ""
}

// FromContext extracts the request ID from a standard context
func FromContext(ctx context.Context) string {
	if rid, ok := ctx.Value(RequestIDKey).(string); ok {
		return rid
	}
	return ""
}

// WithContext adds the request ID to a standard context
func WithContext(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, RequestIDKey, requestID)
}

// WithGortexContext adds the request ID from Gortex context to a standard context
func WithGortexContext(ctx context.Context, c gortexContext.Context) context.Context {
	rid := FromGortexContext(c)
	if rid != "" {
		return WithContext(ctx, rid)
	}
	return ctx
}

// SetHeader sets the request ID header on an HTTP request
func SetHeader(req *http.Request, requestID string) {
	if requestID != "" {
		req.Header.Set(HeaderXRequestID, requestID)
	}
}

// GetHeader gets the request ID from an HTTP request header
func GetHeader(req *http.Request) string {
	return req.Header.Get(HeaderXRequestID)
}

// PropagateToRequest propagates the request ID from a Gortex context to an outgoing HTTP request
func PropagateToRequest(c gortexContext.Context, req *http.Request) {
	rid := FromGortexContext(c)
	if rid != "" {
		SetHeader(req, rid)
	}
}

// PropagateFromContext propagates the request ID from a context to an outgoing HTTP request
func PropagateFromContext(ctx context.Context, req *http.Request) {
	rid := FromContext(ctx)
	if rid != "" {
		SetHeader(req, rid)
	}
}

// Logger returns a logger with the request ID field added
func Logger(logger *zap.Logger, requestID string) *zap.Logger {
	if requestID != "" {
		return logger.With(zap.String("request_id", requestID))
	}
	return logger
}

// LoggerFromGortex returns a logger with the request ID from Gortex context
func LoggerFromGortex(logger *zap.Logger, c gortexContext.Context) *zap.Logger {
	return Logger(logger, FromGortexContext(c))
}

// LoggerFromContext returns a logger with the request ID from context
func LoggerFromContext(logger *zap.Logger, ctx context.Context) *zap.Logger {
	return Logger(logger, FromContext(ctx))
}

// HTTPClient wraps an HTTP client to automatically propagate request IDs
type HTTPClient struct {
	client *http.Client
	ctx    context.Context
}

// NewHTTPClient creates a new HTTP client that propagates request IDs from context
func NewHTTPClient(client *http.Client, ctx context.Context) *HTTPClient {
	if client == nil {
		client = http.DefaultClient
	}
	return &HTTPClient{
		client: client,
		ctx:    ctx,
	}
}

// Do executes an HTTP request with automatic request ID propagation
func (c *HTTPClient) Do(req *http.Request) (*http.Response, error) {
	PropagateFromContext(c.ctx, req)
	return c.client.Do(req)
}

// Get is a convenience method for GET requests with request ID propagation
func (c *HTTPClient) Get(url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(c.ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return c.Do(req)
}

// Post is a convenience method for POST requests with request ID propagation
func (c *HTTPClient) Post(url string, contentType string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(c.ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	return c.Do(req)
}