// Package middleware provides common middleware for the Gortex framework
package middleware

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Headers emitted by the rate-limit middleware on every pass-through
// request and on 429 responses. The names follow the de-facto convention
// used by GitHub, Twitter and others.
const (
	HeaderRateLimitLimit     = "X-RateLimit-Limit"
	HeaderRateLimitRemaining = "X-RateLimit-Remaining"
	HeaderRateLimitReset     = "X-RateLimit-Reset"
	HeaderRetryAfter         = "Retry-After"
)

// RateLimitStatuser is implemented by stores that can report how many
// requests are left in a bucket without consuming one. It is optional:
// stores that cannot supply the information simply won't produce the
// client-facing rate-limit headers.
type RateLimitStatuser interface {
	Status(key string) (limit int, remaining int, reset time.Time)
}

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
	KeyFunc func(c Context) string

	// ErrorHandler handles rate limit errors
	ErrorHandler func(c Context) error

	// SkipFunc determines if rate limiting should be skipped
	SkipFunc func(c Context) bool

	// Store is the rate limiter implementation
	Store RateLimiter
}

// DefaultGortexRateLimitConfig returns a default rate limit configuration
func DefaultGortexRateLimitConfig() *GortexRateLimitConfig {
	return &GortexRateLimitConfig{
		Rate:  10,
		Burst: 20,
		KeyFunc: func(c Context) string {
			// Use client IP as default rate-limit key.
			// SECURITY WARNING: c.RealIP() trusts X-Forwarded-For unconditionally.
			// This key can be spoofed if no trusted reverse proxy is configured.
			// Consider a custom KeyFunc (e.g., authenticated user ID) in production.
			return c.RealIP()
		},
		ErrorHandler: func(c Context) error {
			return c.JSON(http.StatusTooManyRequests, map[string]string{
				"error": "rate limit exceeded",
			})
		},
		SkipFunc: nil,
		Store:    NewMemoryRateLimiter(),
	}
}

// rateLimitEntry holds a rate limiter and its last-access timestamp for TTL tracking.
type rateLimitEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// MemoryRateLimiterOptions configures a MemoryRateLimiter.
type MemoryRateLimiterOptions struct {
	Rate  rate.Limit
	Burst int
	// TTL is the idle duration after which an entry is eligible for cleanup.
	// Defaults to 30 minutes.
	TTL time.Duration
	// CleanupInterval is how often the background goroutine scans for stale
	// entries. Defaults to 10 minutes.
	CleanupInterval time.Duration
}

// MemoryRateLimiter implements RateLimiter using in-memory storage with
// automatic TTL-based cleanup to prevent unbounded memory growth.
type MemoryRateLimiter struct {
	entries map[string]*rateLimitEntry
	mu      sync.RWMutex
	rate    rate.Limit
	burst   int

	ttl             time.Duration
	cleanupInterval time.Duration
	stopCh          chan struct{}
	stopOnce        sync.Once
}

// NewMemoryRateLimiter creates a new memory-based rate limiter with
// default settings (10 req/s, burst 20, 30 min TTL, 10 min cleanup interval).
func NewMemoryRateLimiter() *MemoryRateLimiter {
	return NewMemoryRateLimiterWithOptions(MemoryRateLimiterOptions{
		Rate:            rate.Limit(10),
		Burst:           20,
		TTL:             30 * time.Minute,
		CleanupInterval: 10 * time.Minute,
	})
}

// NewMemoryRateLimiterWithOptions creates a MemoryRateLimiter with the given
// options and starts the background cleanup goroutine.
func NewMemoryRateLimiterWithOptions(opts MemoryRateLimiterOptions) *MemoryRateLimiter {
	if opts.TTL <= 0 {
		opts.TTL = 30 * time.Minute
	}
	if opts.CleanupInterval <= 0 {
		opts.CleanupInterval = 10 * time.Minute
	}
	if opts.Burst <= 0 {
		opts.Burst = 20
	}

	m := &MemoryRateLimiter{
		entries:         make(map[string]*rateLimitEntry),
		rate:            opts.Rate,
		burst:           opts.Burst,
		ttl:             opts.TTL,
		cleanupInterval: opts.CleanupInterval,
		stopCh:          make(chan struct{}),
	}
	go m.runCleanup()
	return m
}

// runCleanup periodically removes entries that have been idle longer than TTL.
func (m *MemoryRateLimiter) runCleanup() {
	ticker := time.NewTicker(m.cleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			m.Cleanup()
		case <-m.stopCh:
			return
		}
	}
}

// Stop shuts down the background cleanup goroutine. Safe to call multiple times.
func (m *MemoryRateLimiter) Stop() {
	m.stopOnce.Do(func() { close(m.stopCh) })
}

// SetRate sets the rate and burst for new limiters.
func (m *MemoryRateLimiter) SetRate(r rate.Limit, burst int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rate = r
	m.burst = burst
}

// getEntry returns the entry for key, creating it if it does not exist.
// It updates lastSeen on every access.
func (m *MemoryRateLimiter) getEntry(key string) *rateLimitEntry {
	m.mu.Lock()
	defer m.mu.Unlock()

	e, exists := m.entries[key]
	if !exists {
		e = &rateLimitEntry{limiter: rate.NewLimiter(m.rate, m.burst)}
		m.entries[key] = e
	}
	e.lastSeen = time.Now()
	return e
}

// Allow checks if a request is allowed.
func (m *MemoryRateLimiter) Allow(key string) bool {
	return m.getEntry(key).limiter.Allow()
}

// AllowN checks if n requests are allowed.
func (m *MemoryRateLimiter) AllowN(key string, n int) bool {
	return m.getEntry(key).limiter.AllowN(time.Now(), n)
}

// Reset removes the rate limiter for a key, resetting its counter.
func (m *MemoryRateLimiter) Reset(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.entries, key)
}

// Status reports the current bucket state for the given key without
// consuming a token. limit is the configured burst, remaining is the
// rounded-down number of tokens currently available, and reset is when
// the bucket will next be fully refilled.
func (m *MemoryRateLimiter) Status(key string) (limit int, remaining int, reset time.Time) {
	entry := m.getEntry(key)

	m.mu.RLock()
	burst := m.burst
	r := m.rate
	m.mu.RUnlock()

	now := time.Now()
	tokens := entry.limiter.TokensAt(now)
	if tokens < 0 {
		tokens = 0
	}
	if tokens > float64(burst) {
		tokens = float64(burst)
	}

	remaining = int(math.Floor(tokens))
	limit = burst

	if r <= 0 || tokens >= float64(burst) {
		reset = now
		return
	}
	missing := float64(burst) - tokens
	seconds := missing / float64(r)
	reset = now.Add(time.Duration(seconds * float64(time.Second)))
	return
}

// Cleanup removes entries that have been idle longer than the configured TTL.
// It is called automatically by the background goroutine but can also be
// invoked manually.
func (m *MemoryRateLimiter) Cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := time.Now().Add(-m.ttl)
	for key, entry := range m.entries {
		if entry.lastSeen.Before(cutoff) {
			delete(m.entries, key)
		}
	}
}


// GortexRateLimit returns a rate limiting middleware for Gortex
func GortexRateLimit() MiddlewareFunc {
	return GortexRateLimitWithConfig(DefaultGortexRateLimitConfig())
}

// GortexRateLimitWithConfig returns a rate limiting middleware with custom config
func GortexRateLimitWithConfig(config *GortexRateLimitConfig) MiddlewareFunc {
	// Set defaults
	if config.KeyFunc == nil {
		config.KeyFunc = func(c Context) string {
			return c.RealIP()
		}
	}
	
	if config.ErrorHandler == nil {
		config.ErrorHandler = func(c Context) error {
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
		return func(c Context) error {
			// Skip if skip function returns true
			if config.SkipFunc != nil && config.SkipFunc(c) {
				return next(c)
			}

			// Get key for rate limiting
			key := config.KeyFunc(c)
			allowed := config.Store.Allow(key)
			applyRateLimitHeaders(c, config.Store, key, allowed)

			if !allowed {
				return config.ErrorHandler(c)
			}
			return next(c)
		}
	}
}

// applyRateLimitHeaders writes the RateLimit-Limit, RateLimit-Remaining,
// RateLimit-Reset and (on 429) Retry-After headers, provided the store
// supports status reporting. Called before the handler / error handler
// runs so clients always see a consistent view regardless of branch.
func applyRateLimitHeaders(c Context, store RateLimiter, key string, allowed bool) {
	statuser, ok := store.(RateLimitStatuser)
	if !ok {
		return
	}
	limit, remaining, reset := statuser.Status(key)

	h := c.Response().Header()
	h.Set(HeaderRateLimitLimit, strconv.Itoa(limit))
	h.Set(HeaderRateLimitRemaining, strconv.Itoa(remaining))
	h.Set(HeaderRateLimitReset, strconv.FormatInt(reset.Unix(), 10))

	if !allowed {
		// Retry-After is the minimum wait before the client should try
		// again. Express as whole seconds, rounded up and clamped to a
		// minimum of one second so clients don't hammer.
		wait := time.Until(reset)
		seconds := int64(math.Ceil(wait.Seconds()))
		if seconds < 1 {
			seconds = 1
		}
		h.Set(HeaderRetryAfter, strconv.FormatInt(seconds, 10))
	}
}

// GetRateLimitKey is a helper function to extract rate limit key from context
func GetRateLimitKey(c Context) string {
	if key := c.Get("rate_limit_key"); key != nil {
		if keyStr, ok := key.(string); ok {
			return keyStr
		}
	}
	return c.RealIP()
}

// Common key functions for rate limiting

// RateLimitByIP returns a key function that uses client IP
func RateLimitByIP() func(Context) string {
	return func(c Context) string {
		return c.RealIP()
	}
}

// RateLimitByUser returns a key function that uses user ID from context
func RateLimitByUser(userKey string) func(Context) string {
	return func(c Context) string {
		if userID := c.Get(userKey); userID != nil {
			return fmt.Sprintf("user:%v", userID)
		}
		return c.RealIP() // fallback to IP
	}
}

// RateLimitByHeader returns a key function that uses a specific header
func RateLimitByHeader(headerName string) func(Context) string {
	return func(c Context) string {
		if value := c.Request().Header.Get(headerName); value != "" {
			return fmt.Sprintf("header:%s:%s", headerName, value)
		}
		return c.RealIP() // fallback to IP
	}
}
