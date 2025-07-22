package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestRequestID(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// Test with default configuration
	mw := RequestID()
	handler := mw(func(c echo.Context) error {
		// Check that request ID is set in context
		requestID := c.Get("request_id").(string)
		assert.NotEmpty(t, requestID)
		
		// Check UUID format (8-4-4-4-12)
		assert.Regexp(t, `^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$`, requestID)
		
		return c.String(http.StatusOK, "OK")
	})

	err := handler(c)
	assert.NoError(t, err)

	// Check response header
	assert.NotEmpty(t, rec.Header().Get(echo.HeaderXRequestID))
}

func TestRequestIDWithExistingID(t *testing.T) {
	e := echo.New()
	existingID := "existing-request-id-123"
	
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(echo.HeaderXRequestID, existingID)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	mw := RequestID()
	handler := mw(func(c echo.Context) error {
		// Check that existing request ID is preserved
		requestID := c.Get("request_id").(string)
		assert.Equal(t, existingID, requestID)
		return c.String(http.StatusOK, "OK")
	})

	err := handler(c)
	assert.NoError(t, err)

	// Check response header has the same ID
	assert.Equal(t, existingID, rec.Header().Get(echo.HeaderXRequestID))
}

func TestRequestIDWithCustomGenerator(t *testing.T) {
	e := echo.New()
	customID := "custom-generated-id"
	
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	config := RequestIDConfig{
		Generator: func() string {
			return customID
		},
	}
	
	mw := RequestIDWithConfig(config)
	handler := mw(func(c echo.Context) error {
		// Check custom generated ID
		requestID := c.Get("request_id").(string)
		assert.Equal(t, customID, requestID)
		return c.String(http.StatusOK, "OK")
	})

	err := handler(c)
	assert.NoError(t, err)

	// Check response header
	assert.Equal(t, customID, rec.Header().Get(echo.HeaderXRequestID))
}

func TestRequestIDWithHandler(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handlerCalled := false
	capturedID := ""
	
	config := RequestIDConfig{
		RequestIDHandler: func(c echo.Context, requestID string) {
			handlerCalled = true
			capturedID = requestID
		},
	}
	
	mw := RequestIDWithConfig(config)
	handler := mw(func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	err := handler(c)
	assert.NoError(t, err)
	
	assert.True(t, handlerCalled)
	assert.NotEmpty(t, capturedID)
	assert.Equal(t, capturedID, rec.Header().Get(echo.HeaderXRequestID))
}

func TestRequestIDWithCustomHeader(t *testing.T) {
	e := echo.New()
	customHeader := "X-Trace-ID"
	existingID := "trace-123"
	
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(customHeader, existingID)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	config := RequestIDConfig{
		TargetHeader: customHeader,
	}
	
	mw := RequestIDWithConfig(config)
	handler := mw(func(c echo.Context) error {
		// Check that existing ID from custom header is used
		requestID := c.Get("request_id").(string)
		assert.Equal(t, existingID, requestID)
		return c.String(http.StatusOK, "OK")
	})

	err := handler(c)
	assert.NoError(t, err)

	// Check response uses custom header
	assert.Equal(t, existingID, rec.Header().Get(customHeader))
}

func TestRequestIDWithSkipper(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/skip", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	config := RequestIDConfig{
		Skipper: func(c echo.Context) bool {
			return c.Request().URL.Path == "/skip"
		},
	}
	
	mw := RequestIDWithConfig(config)
	handler := mw(func(c echo.Context) error {
		// Check that request ID is NOT set when skipped
		requestID := c.Get("request_id")
		assert.Nil(t, requestID)
		return c.String(http.StatusOK, "OK")
	})

	err := handler(c)
	assert.NoError(t, err)

	// Check no request ID header when skipped
	assert.Empty(t, rec.Header().Get(echo.HeaderXRequestID))
}

// Benchmark the request ID generation
func BenchmarkRequestID(b *testing.B) {
	e := echo.New()
	mw := RequestID()
	handler := mw(func(c echo.Context) error {
		return nil
	})

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			handler(c)
		}
	})
}

// Benchmark with existing request ID (no generation needed)
func BenchmarkRequestIDWithExisting(b *testing.B) {
	e := echo.New()
	mw := RequestID()
	handler := mw(func(c echo.Context) error {
		return nil
	})

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set(echo.HeaderXRequestID, "existing-id-123")
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			handler(c)
		}
	})
}