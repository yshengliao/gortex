// Package middleware provides common middleware for the Gortex framework
package middleware

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
	"github.com/yshengliao/gortex/context"
)

// RateLimiter defines the interface for rate limiting
type RateLimiter interface {
	// Allow checks if a request is allowed
	Allow(key string) bool

	// AllowN checks if n requests are allowed
	AllowN(key string, n int) bool

	// Reset resets the rate limiter for a key
	Reset(key string)
}

// RateLimitConfig holds rate limiting configuration
type GortexRateLimitConfig struct {
	// Rate is the number of requests per second
	Rate int

	// Burst is the maximum burst size
	Burst int

	// KeyFunc extracts the key from the request
	KeyFunc func(c context.Context) string

	// ErrorHandler handles rate limit errors
	ErrorHandler func(c context.Context) error

	// SkipFunc determines if rate limiting should be skipped
	SkipFunc func(c context.Context) bool

	// Store is the rate limiter implementation
	Store RateLimiter
}

// DefaultGortexRateLimitConfig returns a default rate limit configuration
func DefaultGortexRateLimitConfig() *GortexRateLimitConfig {
	return &GortexRateLimitConfig{
		Rate:  10,
		Burst: 20,
		KeyFunc: func(c context.Context) string {
			// Use client IP as default key
			return c.RealIP()
		},
		ErrorHandler: func(c context.Context) error {
			return c.JSON(http.StatusTooManyRequests, map[string]string{
				"error": "rate limit exceeded",
			})
		},
		SkipFunc: nil,
		Store:    NewMemoryRateLimiter(),
	}
}

// MemoryRateLimiter implements RateLimiter using in-memory storage
type MemoryRateLimiter struct {
	limiters map[string]*rate.Limiter
	mu       sync.RWMutex
	rate     rate.Limit
	burst    int
}

// NewMemoryRateLimiter creates a new memory-based rate limiter
func NewMemoryRateLimiter() *MemoryRateLimiter {
	return &MemoryRateLimiter{
		limiters: make(map[string]*rate.Limiter),
		rate:     rate.Limit(10), // 10 requests per second
		burst:    20,             // burst of 20
	}
}

// SetRate sets the rate and burst for new limiters
func (m *MemoryRateLimiter) SetRate(r rate.Limit, burst int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rate = r
	m.burst = burst
}

// getLimiter returns the rate limiter for a given key
func (m *MemoryRateLimiter) getLimiter(key string) *rate.Limiter {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	limiter, exists := m.limiters[key]
	if !exists {
		limiter = rate.NewLimiter(m.rate, m.burst)
		m.limiters[key] = limiter
	}
	
	return limiter
}

// Allow checks if a request is allowed
func (m *MemoryRateLimiter) Allow(key string) bool {
	return m.getLimiter(key).Allow()
}

// AllowN checks if n requests are allowed
func (m *MemoryRateLimiter) AllowN(key string, n int) bool {
	return m.getLimiter(key).AllowN(time.Now(), n)
}

// Reset resets the rate limiter for a key
func (m *MemoryRateLimiter) Reset(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.limiters, key)
}

// Cleanup removes old limiters (should be called periodically)
func (m *MemoryRateLimiter) Cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Simple cleanup: remove all limiters
	// In production, you might want more sophisticated cleanup logic
	m.limiters = make(map[string]*rate.Limiter)
}

// GortexRateLimit returns a rate limiting middleware for Gortex
func GortexRateLimit() MiddlewareFunc {
	return GortexRateLimitWithConfig(DefaultGortexRateLimitConfig())
}

// GortexRateLimitWithConfig returns a rate limiting middleware with custom config
func GortexRateLimitWithConfig(config *GortexRateLimitConfig) MiddlewareFunc {
	// Set defaults
	if config.KeyFunc == nil {
		config.KeyFunc = func(c context.Context) string {
			return c.RealIP()
		}
	}
	
	if config.ErrorHandler == nil {
		config.ErrorHandler = func(c context.Context) error {
			return c.JSON(http.StatusTooManyRequests, map[string]string{
				"error": "rate limit exceeded",
			})
		}
	}
	
	if config.Store == nil {
		store := NewMemoryRateLimiter()
		store.SetRate(rate.Limit(config.Rate), config.Burst)
		config.Store = store
	}

	return func(next HandlerFunc) HandlerFunc {
		return func(c context.Context) error {
			// Skip if skip function returns true
			if config.SkipFunc != nil && config.SkipFunc(c) {
				return next(c)
			}

			// Get key for rate limiting
			key := config.KeyFunc(c)
			
			// Check rate limit
			if !config.Store.Allow(key) {
				return config.ErrorHandler(c)
			}

			return next(c)
		}
	}
}

// GetRateLimitKey is a helper function to extract rate limit key from context
func GetRateLimitKey(c context.Context) string {
	if key := c.Get("rate_limit_key"); key != nil {
		if keyStr, ok := key.(string); ok {
			return keyStr
		}
	}
	return c.RealIP()
}

// Common key functions for rate limiting

// RateLimitByIP returns a key function that uses client IP
func RateLimitByIP() func(context.Context) string {
	return func(c context.Context) string {
		return c.RealIP()
	}
}

// RateLimitByUser returns a key function that uses user ID from context
func RateLimitByUser(userKey string) func(context.Context) string {
	return func(c context.Context) string {
		if userID := c.Get(userKey); userID != nil {
			return fmt.Sprintf("user:%v", userID)
		}
		return c.RealIP() // fallback to IP
	}
}

// RateLimitByHeader returns a key function that uses a specific header
func RateLimitByHeader(headerName string) func(context.Context) string {
	return func(c context.Context) string {
		if value := c.Request().Header.Get(headerName); value != "" {
			return fmt.Sprintf("header:%s:%s", headerName, value)
		}
		return c.RealIP() // fallback to IP
	}
}
