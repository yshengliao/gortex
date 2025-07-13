package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yshengliao/gortex/middleware"
)

func TestMemoryStore(t *testing.T) {
	store := middleware.NewMemoryStore(10, 20)
	defer store.Stop()

	t.Run("Allow", func(t *testing.T) {
		// Should allow initial requests
		for i := 0; i < 10; i++ {
			assert.True(t, store.Allow("test-key"))
		}
	})

	t.Run("AllowN", func(t *testing.T) {
		// Should allow burst
		assert.True(t, store.AllowN("burst-key", 5))
		assert.True(t, store.AllowN("burst-key", 5))
		
		// Should not allow exceeding burst
		assert.False(t, store.AllowN("burst-key", 15))
	})

	t.Run("Reset", func(t *testing.T) {
		key := "reset-key"
		
		// Use up the limit
		for i := 0; i < 20; i++ {
			store.Allow(key)
		}
		assert.False(t, store.Allow(key))
		
		// Reset and try again
		store.Reset(key)
		assert.True(t, store.Allow(key))
	})

	t.Run("ConcurrentAccess", func(t *testing.T) {
		var wg sync.WaitGroup
		key := "concurrent-key"
		
		// Make concurrent requests
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				store.Allow(key)
			}()
		}
		
		wg.Wait()
		// Should handle concurrent access without panic
	})
}

func TestRateLimitMiddleware(t *testing.T) {
	e := echo.New()
	
	t.Run("DefaultConfig", func(t *testing.T) {
		handler := func(c echo.Context) error {
			return c.String(200, "OK")
		}
		
		// Apply rate limit middleware
		h := middleware.RateLimitMiddleware(nil)(handler)
		
		// Create request
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		
		// Should allow request
		err := h(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("CustomConfig", func(t *testing.T) {
		config := &middleware.RateLimitConfig{
			Rate:  1,
			Burst: 2,
			KeyFunc: func(c echo.Context) string {
				return "fixed-key"
			},
			ErrorHandler: func(c echo.Context) error {
				return c.JSON(429, map[string]string{"error": "too many requests"})
			},
		}
		
		handler := func(c echo.Context) error {
			return c.String(200, "OK")
		}
		
		h := middleware.RateLimitMiddleware(config)(handler)
		
		// First two requests should pass (burst)
		for i := 0; i < 2; i++ {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			
			err := h(c)
			assert.NoError(t, err)
			assert.Equal(t, http.StatusOK, rec.Code)
		}
		
		// Third request should be rate limited
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		
		err := h(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	})

	t.Run("SkipFunc", func(t *testing.T) {
		config := &middleware.RateLimitConfig{
			Rate:  1,
			Burst: 1,
			KeyFunc: func(c echo.Context) string {
				return "test"
			},
			SkipFunc: func(c echo.Context) bool {
				return c.Request().Header.Get("X-Skip-Limit") == "true"
			},
		}
		
		handler := func(c echo.Context) error {
			return c.String(200, "OK")
		}
		
		h := middleware.RateLimitMiddleware(config)(handler)
		
		// Make multiple requests with skip header
		for i := 0; i < 5; i++ {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("X-Skip-Limit", "true")
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			
			err := h(c)
			assert.NoError(t, err)
			assert.Equal(t, http.StatusOK, rec.Code)
		}
	})
}

func TestRateLimitByIP(t *testing.T) {
	e := echo.New()
	handler := func(c echo.Context) error {
		return c.String(200, "OK")
	}
	
	h := middleware.RateLimitByIP(2, 4)(handler)
	
	// Test different IPs
	ips := []string{"192.168.1.1:12345", "192.168.1.2:12345"}
	
	for _, ip := range ips {
		// Each IP should get its own limit
		for i := 0; i < 4; i++ {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = ip
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			
			err := h(c)
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, rec.Code)
		}
	}
}

func TestRateLimitByPath(t *testing.T) {
	e := echo.New()
	handler := func(c echo.Context) error {
		return c.String(200, "OK")
	}
	
	h := middleware.RateLimitByPath(1, 2)(handler)
	
	paths := []string{"/api/users", "/api/posts"}
	
	for _, path := range paths {
		// Each path should get its own limit
		for i := 0; i < 2; i++ {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.RemoteAddr = "192.168.1.1:12345"
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			c.SetPath(path)
			
			err := h(c)
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, rec.Code)
		}
		
		// Third request should fail
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.RemoteAddr = "192.168.1.1:12345"
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPath(path)
		
		err := h(c)
		require.Error(t, err)
		httpErr, ok := err.(*echo.HTTPError)
		require.True(t, ok)
		assert.Equal(t, http.StatusTooManyRequests, httpErr.Code)
	}
}

func TestRateLimitByUser(t *testing.T) {
	e := echo.New()
	handler := func(c echo.Context) error {
		return c.String(200, "OK")
	}
	
	getUserID := func(c echo.Context) string {
		return c.Request().Header.Get("X-User-ID")
	}
	
	h := middleware.RateLimitByUser(1, 2, getUserID)(handler)
	
	users := []string{"user1", "user2"}
	
	for _, userID := range users {
		// Each user should get their own limit
		for i := 0; i < 2; i++ {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("X-User-ID", userID)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			
			err := h(c)
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, rec.Code)
		}
	}
}

func TestCustomRateLimitError(t *testing.T) {
	errorHandler := middleware.CustomRateLimitError("Please wait before trying again", 60)
	
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	
	err := errorHandler(c)
	require.Error(t, err)
	
	httpErr, ok := err.(*echo.HTTPError)
	require.True(t, ok)
	assert.Equal(t, http.StatusTooManyRequests, httpErr.Code)
	
	// Check response headers
	assert.Equal(t, "60", rec.Header().Get("Retry-After"))
}

func BenchmarkRateLimiter(b *testing.B) {
	store := middleware.NewMemoryStore(1000, 2000)
	defer store.Stop()
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := "key" + string(rune(i%100))
			store.Allow(key)
			i++
		}
	})
}