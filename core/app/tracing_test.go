package app

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yshengliao/gortex/observability/tracing"
	httpctx "github.com/yshengliao/gortex/transport/http"
	"go.uber.org/zap"
)

func TestAppWithTracer(t *testing.T) {
	// Create a simple tracer
	tracer := tracing.NewSimpleTracer()
	
	// Create app with tracer
	app, err := NewApp(
		WithTracer(tracer),
		WithLogger(zap.NewNop()),
	)
	require.NoError(t, err)
	assert.NotNil(t, app.tracer)
	
	// Define a test handler
	testHandler := func(c httpctx.Context) error {
		// Check that span is available in context
		span := c.Span()
		assert.NotNil(t, span, "Span should be available in context")
		
		// If it's an enhanced span, log an event
		if enhancedSpan, ok := span.(*tracing.EnhancedSpan); ok {
			enhancedSpan.LogEvent(tracing.SpanStatusINFO, "Handler executed", nil)
		}
		
		return c.String(200, "OK")
	}
	
	// Register route
	app.router.GET("/test", testHandler)
	
	// Make test request
	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	
	app.router.ServeHTTP(rec, req)
	
	// Check response
	assert.Equal(t, 200, rec.Code)
	assert.Equal(t, "OK", rec.Body.String())
	
	// Check that trace ID was set in response header
	assert.NotEmpty(t, rec.Header().Get("X-Trace-ID"))
}

// Test handler types
type TestTracingHandlers struct {
	Home  *HomeTracingHandler  `url:"/"`
	Users *UsersTracingHandler `url:"/users/:id"`
}

type HomeTracingHandler struct{}
func (h *HomeTracingHandler) GET(c httpctx.Context) error {
	// Access span from context
	span := c.Span()
	if enhancedSpan, ok := span.(*tracing.EnhancedSpan); ok {
		enhancedSpan.LogEvent(tracing.SpanStatusDEBUG, "Home handler accessed", map[string]any{
			"user_agent": c.Request().UserAgent(),
		})
	}
	return c.JSON(200, map[string]string{"message": "Home"})
}

type UsersTracingHandler struct{}
func (h *UsersTracingHandler) GET(c httpctx.Context) error {
	userID := c.Param("id")
	
	// Access span and log event
	span := c.Span()
	if enhancedSpan, ok := span.(*tracing.EnhancedSpan); ok {
		enhancedSpan.LogEvent(tracing.SpanStatusINFO, "User accessed", map[string]any{
			"user_id": userID,
		})
	}
	
	return c.JSON(200, map[string]string{"id": userID})
}

func TestTracingMiddlewareIntegration(t *testing.T) {
	
	// Create app with tracer
	tracer := tracing.NewSimpleTracer()
	handlers := &TestTracingHandlers{
		Home:  &HomeTracingHandler{},
		Users: &UsersTracingHandler{},
	}
	
	app, err := NewApp(
		WithTracer(tracer),
		WithHandlers(handlers),
		WithLogger(zap.NewNop()),
	)
	require.NoError(t, err)
	
	// Test home endpoint
	t.Run("Home endpoint", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("User-Agent", "TestAgent/1.0")
		rec := httptest.NewRecorder()
		
		app.router.ServeHTTP(rec, req)
		
		assert.Equal(t, 200, rec.Code)
		assert.NotEmpty(t, rec.Header().Get("X-Trace-ID"))
		assert.Contains(t, rec.Header().Get("Content-Type"), "application/json")
	})
	
	// Test users endpoint
	t.Run("Users endpoint", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/users/123", nil)
		rec := httptest.NewRecorder()
		
		app.router.ServeHTTP(rec, req)
		
		assert.Equal(t, 200, rec.Code)
		assert.NotEmpty(t, rec.Header().Get("X-Trace-ID"))
		assert.Contains(t, rec.Body.String(), `"id":"123"`)
	})
	
	// Test error handling
	t.Run("Error handling", func(t *testing.T) {
		// Add error handler
		errorHandler := func(c httpctx.Context) error {
			return httpctx.NewHTTPError(500, "Internal error")
		}
		
		app.router.GET("/error", errorHandler)
		
		req := httptest.NewRequest("GET", "/error", nil)
		rec := httptest.NewRecorder()
		
		app.router.ServeHTTP(rec, req)
		
		// The error should be handled by the framework
		assert.NotEmpty(t, rec.Header().Get("X-Trace-ID"))
	})
}

func TestWithTracerValidation(t *testing.T) {
	// Test nil tracer
	app := &App{}
	err := WithTracer(nil)(app)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tracer cannot be nil")
	
	// Test valid tracer
	tracer := tracing.NewSimpleTracer()
	err = WithTracer(tracer)(app)
	assert.NoError(t, err)
	assert.Equal(t, tracer, app.tracer)
}