// Package middleware provides the core middleware interface and utilities for Gortex framework
package middleware

import (
	"github.com/yshengliao/gortex/core/types"
)

// MiddlewareFunc is an alias to types.MiddlewareFunc
type MiddlewareFunc = types.MiddlewareFunc

// HandlerFunc is an alias to types.HandlerFunc for convenience
type HandlerFunc = types.HandlerFunc

// Context is an alias to types.Context for convenience
type Context = types.Context

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
		h = func(ctx Context) error {
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
