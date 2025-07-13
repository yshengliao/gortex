// Package middleware provides common middleware for the STMP framework
package middleware

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"golang.org/x/time/rate"
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
type RateLimitConfig struct {
	// Rate is the number of requests per second
	Rate int
	
	// Burst is the maximum burst size
	Burst int
	
	// KeyFunc extracts the key from the request
	KeyFunc func(c echo.Context) string
	
	// ErrorHandler handles rate limit errors
	ErrorHandler func(c echo.Context) error
	
	// SkipFunc determines if rate limiting should be skipped
	SkipFunc func(c echo.Context) bool
	
	// Store is the rate limiter implementation
	Store RateLimiter
}

// DefaultRateLimitConfig returns a default rate limit configuration
func DefaultRateLimitConfig() *RateLimitConfig {
	return &RateLimitConfig{
		Rate:  10,
		Burst: 20,
		KeyFunc: func(c echo.Context) string {
			return c.RealIP()
		},
		ErrorHandler: func(c echo.Context) error {
			return echo.NewHTTPError(http.StatusTooManyRequests, "rate limit exceeded")
		},
		SkipFunc: func(c echo.Context) bool {
			return false
		},
	}
}

// MemoryStore is an in-memory rate limiter store
type MemoryStore struct {
	rate     int
	burst    int
	limiters map[string]*rate.Limiter
	mu       sync.RWMutex
	cleanup  *time.Ticker
}

// NewMemoryStore creates a new in-memory rate limiter store
func NewMemoryStore(r int, b int) *MemoryStore {
	store := &MemoryStore{
		rate:     r,
		burst:    b,
		limiters: make(map[string]*rate.Limiter),
		cleanup:  time.NewTicker(1 * time.Minute),
	}
	
	// Start cleanup routine
	go store.cleanupRoutine()
	
	return store
}

// Allow checks if a request is allowed
func (s *MemoryStore) Allow(key string) bool {
	return s.AllowN(key, 1)
}

// AllowN checks if n requests are allowed
func (s *MemoryStore) AllowN(key string, n int) bool {
	s.mu.Lock()
	limiter, exists := s.limiters[key]
	if !exists {
		limiter = rate.NewLimiter(rate.Limit(s.rate), s.burst)
		s.limiters[key] = limiter
	}
	s.mu.Unlock()
	
	return limiter.AllowN(time.Now(), n)
}

// Reset resets the rate limiter for a key
func (s *MemoryStore) Reset(key string) {
	s.mu.Lock()
	delete(s.limiters, key)
	s.mu.Unlock()
}

// cleanupRoutine periodically cleans up unused limiters
func (s *MemoryStore) cleanupRoutine() {
	for range s.cleanup.C {
		s.mu.Lock()
		// In a production implementation, you'd track last access time
		// and remove limiters that haven't been used recently
		s.mu.Unlock()
	}
}

// Stop stops the cleanup routine
func (s *MemoryStore) Stop() {
	s.cleanup.Stop()
}

// RateLimitMiddleware creates a rate limiting middleware
func RateLimitMiddleware(config *RateLimitConfig) echo.MiddlewareFunc {
	if config == nil {
		config = DefaultRateLimitConfig()
	}
	
	if config.Store == nil {
		config.Store = NewMemoryStore(config.Rate, config.Burst)
	}
	
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Check if should skip
			if config.SkipFunc != nil && config.SkipFunc(c) {
				return next(c)
			}
			
			// Get key
			key := config.KeyFunc(c)
			
			// Check rate limit
			if !config.Store.Allow(key) {
				return config.ErrorHandler(c)
			}
			
			return next(c)
		}
	}
}

// RateLimitByIP creates a rate limiter by IP address
func RateLimitByIP(rate, burst int) echo.MiddlewareFunc {
	config := &RateLimitConfig{
		Rate:  rate,
		Burst: burst,
		KeyFunc: func(c echo.Context) string {
			return c.RealIP()
		},
		ErrorHandler: func(c echo.Context) error {
			return echo.NewHTTPError(http.StatusTooManyRequests, "rate limit exceeded")
		},
	}
	
	return RateLimitMiddleware(config)
}

// RateLimitByUser creates a rate limiter by user ID
func RateLimitByUser(rate, burst int, getUserID func(c echo.Context) string) echo.MiddlewareFunc {
	config := &RateLimitConfig{
		Rate:  rate,
		Burst: burst,
		KeyFunc: getUserID,
		ErrorHandler: func(c echo.Context) error {
			return echo.NewHTTPError(http.StatusTooManyRequests, "rate limit exceeded")
		},
	}
	
	return RateLimitMiddleware(config)
}

// RateLimitByPath creates a rate limiter by path
func RateLimitByPath(rate, burst int) echo.MiddlewareFunc {
	config := &RateLimitConfig{
		Rate:  rate,
		Burst: burst,
		KeyFunc: func(c echo.Context) string {
			return fmt.Sprintf("%s:%s", c.RealIP(), c.Path())
		},
		ErrorHandler: func(c echo.Context) error {
			return echo.NewHTTPError(http.StatusTooManyRequests, "rate limit exceeded")
		},
	}
	
	return RateLimitMiddleware(config)
}

// CustomRateLimitError creates a custom error handler for rate limiting
func CustomRateLimitError(message string, retryAfter int) func(c echo.Context) error {
	return func(c echo.Context) error {
		c.Response().Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
		return echo.NewHTTPError(http.StatusTooManyRequests, map[string]interface{}{
			"error":       "rate_limit_exceeded",
			"message":     message,
			"retry_after": retryAfter,
		})
	}
}