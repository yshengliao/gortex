package middleware

import (
	"net/http/httptest"
	"testing"

	"golang.org/x/time/rate"
	httpctx "github.com/yshengliao/gortex/transport/http"
)

func TestGortexRateLimit(t *testing.T) {
	// Create a test handler
	handler := func(c Context) error {
		return c.String(200, "success")
	}

	// Create rate limit config with very low limits for testing
	config := &GortexRateLimitConfig{
		Rate:  1, // 1 request per second
		Burst: 1, // burst of 1
		KeyFunc: func(c Context) string {
			return "test-key" // Fixed key for testing
		},
	}

	// Create middleware
	middleware := GortexRateLimitWithConfig(config)
	wrappedHandler := middleware(handler)

	// First request should succeed
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	ctx := context.NewContext(req, w)

	err := wrappedHandler(ctx)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if w.Code != 200 {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Second request should be rate limited
	req = httptest.NewRequest("GET", "/test", nil)
	w = httptest.NewRecorder()
	ctx = context.NewContext(req, w)

	err = wrappedHandler(ctx)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if w.Code != 429 {
		t.Errorf("Expected status 429 (rate limited), got %d", w.Code)
	}
}

func TestGortexRateLimitByIP(t *testing.T) {
	handler := func(c Context) error {
		return c.String(200, "success")
	}

	config := &GortexRateLimitConfig{
		Rate:    1,
		Burst:   1,
		KeyFunc: RateLimitByIP(),
	}

	middleware := GortexRateLimitWithConfig(config)
	wrappedHandler := middleware(handler)

	// Test with different IPs
	req1 := httptest.NewRequest("GET", "/test", nil)
	req1.RemoteAddr = "192.168.1.1:12345"
	w1 := httptest.NewRecorder()
	ctx1 := context.NewContext(req1, w1)

	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.RemoteAddr = "192.168.1.2:12346"
	w2 := httptest.NewRecorder()
	ctx2 := context.NewContext(req2, w2)

	// Both requests should succeed (different IPs)
	err := wrappedHandler(ctx1)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if w1.Code != 200 {
		t.Errorf("Expected status 200, got %d", w1.Code)
	}

	err = wrappedHandler(ctx2)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if w2.Code != 200 {
		t.Errorf("Expected status 200, got %d", w2.Code)
	}
}

func TestGortexRateLimitSkip(t *testing.T) {
	handler := func(c Context) error {
		return c.String(200, "success")
	}

	config := &GortexRateLimitConfig{
		Rate:  1,
		Burst: 1,
		KeyFunc: func(c Context) string {
			return "test-key"
		},
		SkipFunc: func(c Context) bool {
			return c.Request().URL.Path == "/skip"
		},
	}

	middleware := GortexRateLimitWithConfig(config)
	wrappedHandler := middleware(handler)

	// First request to non-skipped path should succeed
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	ctx := context.NewContext(req, w)

	err := wrappedHandler(ctx)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if w.Code != 200 {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Second request to non-skipped path should be rate limited
	req = httptest.NewRequest("GET", "/test", nil)
	w = httptest.NewRecorder()
	ctx = context.NewContext(req, w)

	err = wrappedHandler(ctx)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if w.Code != 429 {
		t.Errorf("Expected status 429, got %d", w.Code)
	}

	// Request to skipped path should always succeed
	req = httptest.NewRequest("GET", "/skip", nil)
	w = httptest.NewRecorder()
	ctx = context.NewContext(req, w)

	err = wrappedHandler(ctx)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if w.Code != 200 {
		t.Errorf("Expected status 200 for skipped path, got %d", w.Code)
	}
}

func TestMemoryRateLimiter(t *testing.T) {
	limiter := NewMemoryRateLimiter()
	limiter.SetRate(rate.Limit(2), 2) // 2 requests per second, burst of 2

	key := "test-key"

	// First two requests should be allowed (burst)
	if !limiter.Allow(key) {
		t.Error("First request should be allowed")
	}
	if !limiter.Allow(key) {
		t.Error("Second request should be allowed")
	}

	// Third request should be denied (exceeds burst)
	if limiter.Allow(key) {
		t.Error("Third request should be denied")
	}

	// Test reset
	limiter.Reset(key)
	if !limiter.Allow(key) {
		t.Error("Request should be allowed after reset")
	}
}

func TestRateLimitByUser(t *testing.T) {
	keyFunc := RateLimitByUser("user_id")

	// Create context with user ID
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	ctx := context.NewContext(req, w)
	ctx.Set("user_id", "user123")

	key := keyFunc(ctx)
	expected := "user:user123"
	if key != expected {
		t.Errorf("Expected key %s, got %s", expected, key)
	}

	// Test fallback to IP when no user ID
	ctx2 := context.NewContext(req, w)
	key2 := keyFunc(ctx2)
	if key2 == expected {
		t.Error("Should fallback to IP when no user ID")
	}
}

func TestRateLimitByHeader(t *testing.T) {
	keyFunc := RateLimitByHeader("X-API-Key")

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", "api123")
	w := httptest.NewRecorder()
	ctx := context.NewContext(req, w)

	key := keyFunc(ctx)
	expected := "header:X-API-Key:api123"
	if key != expected {
		t.Errorf("Expected key %s, got %s", expected, key)
	}

	// Test fallback when header missing
	req2 := httptest.NewRequest("GET", "/test", nil)
	ctx2 := context.NewContext(req2, w)
	key2 := keyFunc(ctx2)
	if key2 == expected {
		t.Error("Should fallback to IP when header missing")
	}
}
