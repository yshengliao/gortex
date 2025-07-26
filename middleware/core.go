// Package middleware provides the core middleware interface and utilities for Gortex framework
package middleware

import "github.com/yshengliao/gortex/http/context"

// MiddlewareFunc defines the middleware function type for Gortex framework.
// It takes a HandlerFunc and returns a new HandlerFunc, allowing middleware to wrap handlers.
type MiddlewareFunc func(next HandlerFunc) HandlerFunc

// HandlerFunc defines a function to serve HTTP requests in Gortex framework.
// This is the same as app.HandlerFunc to ensure consistency across the framework.
type HandlerFunc func(c context.Context) error

// Chain represents a middleware chain that can be applied to handlers
type Chain struct {
	middlewares []MiddlewareFunc
}

// NewChain creates a new middleware chain
func NewChain(middlewares ...MiddlewareFunc) *Chain {
	return &Chain{
		middlewares: append([]MiddlewareFunc(nil), middlewares...),
	}
}

// Then chains the middleware and returns the final handler
func (c *Chain) Then(h HandlerFunc) HandlerFunc {
	if h == nil {
		h = func(ctx context.Context) error {
			return nil
		}
	}

	for i := len(c.middlewares) - 1; i >= 0; i-- {
		h = c.middlewares[i](h)
	}

	return h
}

// Append adds middleware to the chain
func (c *Chain) Append(middlewares ...MiddlewareFunc) *Chain {
	newChain := &Chain{
		middlewares: make([]MiddlewareFunc, len(c.middlewares)+len(middlewares)),
	}
	copy(newChain.middlewares, c.middlewares)
	copy(newChain.middlewares[len(c.middlewares):], middlewares)
	return newChain
}
