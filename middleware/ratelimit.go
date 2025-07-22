// Package middleware provides common middleware for the Gortex framework
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

// limiterEntry holds a rate limiter and its last access time
type limiterEntry struct {
	limiter    *rate.Limiter
	lastAccess time.Time
}

// MemoryStore is an in-memory rate limiter store
type MemoryStore struct {
	rate            int
	burst           int
	limiters        map[string]*limiterEntry
	mu              sync.RWMutex
	cleanup         *time.Ticker
	cleanupInterval time.Duration
	ttl             time.Duration
	stopped         chan struct{}
	stopOnce        sync.Once
}

// MemoryStoreConfig holds configuration for MemoryStore
type MemoryStoreConfig struct {
	Rate            int
	Burst           int
	CleanupInterval time.Duration
	TTL             time.Duration
}

// DefaultMemoryStoreConfig returns default configuration
func DefaultMemoryStoreConfig() *MemoryStoreConfig {
	return &MemoryStoreConfig{
		Rate:            10,
		Burst:           20,
		CleanupInterval: 1 * time.Minute,
		TTL:             10 * time.Minute,
	}
}

// NewMemoryStore creates a new in-memory rate limiter store
func NewMemoryStore(r int, b int) *MemoryStore {
	config := &MemoryStoreConfig{
		Rate:            r,
		Burst:           b,
		CleanupInterval: 1 * time.Minute,
		TTL:             10 * time.Minute,
	}
	return NewMemoryStoreWithConfig(config)
}

// NewMemoryStoreWithConfig creates a new in-memory rate limiter store with config
func NewMemoryStoreWithConfig(config *MemoryStoreConfig) *MemoryStore {
	store := &MemoryStore{
		rate:            config.Rate,
		burst:           config.Burst,
		limiters:        make(map[string]*limiterEntry),
		cleanup:         time.NewTicker(config.CleanupInterval),
		cleanupInterval: config.CleanupInterval,
		ttl:             config.TTL,
		stopped:         make(chan struct{}),
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
	now := time.Now()

	s.mu.Lock()
	entry, exists := s.limiters[key]
	if !exists {
		entry = &limiterEntry{
			limiter:    rate.NewLimiter(rate.Limit(s.rate), s.burst),
			lastAccess: now,
		}
		s.limiters[key] = entry
	} else {
		entry.lastAccess = now
	}
	s.mu.Unlock()

	return entry.limiter.AllowN(now, n)
}

// Reset resets the rate limiter for a key
func (s *MemoryStore) Reset(key string) {
	s.mu.Lock()
	delete(s.limiters, key)
	s.mu.Unlock()
}

// cleanupRoutine periodically cleans up unused limiters
func (s *MemoryStore) cleanupRoutine() {
	for {
		select {
		case <-s.cleanup.C:
			s.performCleanup()
		case <-s.stopped:
			return
		}
	}
}

// performCleanup removes expired limiters
func (s *MemoryStore) performCleanup() {
	now := time.Now()
	expiredKeys := make([]string, 0)

	s.mu.RLock()
	for key, entry := range s.limiters {
		if now.Sub(entry.lastAccess) > s.ttl {
			expiredKeys = append(expiredKeys, key)
		}
	}
	s.mu.RUnlock()

	// Remove expired entries
	if len(expiredKeys) > 0 {
		s.mu.Lock()
		for _, key := range expiredKeys {
			// Double-check the entry is still expired (it might have been accessed since we checked)
			if entry, exists := s.limiters[key]; exists && now.Sub(entry.lastAccess) > s.ttl {
				delete(s.limiters, key)
			}
		}
		s.mu.Unlock()
	}
}

// Size returns the current number of limiters in the store
func (s *MemoryStore) Size() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.limiters)
}

// Stop stops the cleanup routine
func (s *MemoryStore) Stop() {
	s.stopOnce.Do(func() {
		s.cleanup.Stop()
		close(s.stopped)
	})
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
		Rate:    rate,
		Burst:   burst,
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
