package observability_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yshengliao/gortex/observability"
)

func TestSimpleTracer(t *testing.T) {
	tracer := observability.NewSimpleTracer()

	t.Run("StartAndFinishSpan", func(t *testing.T) {
		ctx := context.Background()
		ctx, span := tracer.StartSpan(ctx, "test-operation")
		
		assert.NotNil(t, span)
		assert.NotEmpty(t, span.TraceID)
		assert.NotEmpty(t, span.SpanID)
		assert.Equal(t, "test-operation", span.Operation)
		assert.Empty(t, span.ParentID)
		
		tracer.FinishSpan(span)
		assert.False(t, span.EndTime.IsZero())
	})

	t.Run("NestedSpans", func(t *testing.T) {
		ctx := context.Background()
		
		// Start parent span
		ctx, parentSpan := tracer.StartSpan(ctx, "parent-operation")
		
		// Start child span
		ctx, childSpan := tracer.StartSpan(ctx, "child-operation")
		
		assert.Equal(t, parentSpan.TraceID, childSpan.TraceID)
		assert.Equal(t, parentSpan.SpanID, childSpan.ParentID)
		
		tracer.FinishSpan(childSpan)
		tracer.FinishSpan(parentSpan)
	})

	t.Run("AddTags", func(t *testing.T) {
		ctx := context.Background()
		_, span := tracer.StartSpan(ctx, "tagged-operation")
		
		tracer.AddTags(span, map[string]string{
			"user.id": "123",
			"http.method": "GET",
		})
		
		assert.Equal(t, "123", span.Tags["user.id"])
		assert.Equal(t, "GET", span.Tags["http.method"])
		
		tracer.FinishSpan(span)
	})

	t.Run("SetStatus", func(t *testing.T) {
		ctx := context.Background()
		_, span := tracer.StartSpan(ctx, "status-operation")
		
		tracer.SetStatus(span, observability.SpanStatusError)
		assert.Equal(t, observability.SpanStatusError, span.Status)
		
		tracer.FinishSpan(span)
	})
}

func TestTracingMiddleware(t *testing.T) {
	e := echo.New()
	tracer := observability.NewSimpleTracer()
	
	// Add tracing middleware
	e.Use(observability.TracingMiddleware(tracer))
	
	// Add test handler that uses tracing
	e.GET("/test", func(c echo.Context) error {
		// Get span from context
		span := observability.SpanFromContext(c.Request().Context())
		assert.NotNil(t, span)
		
		return c.JSON(200, map[string]string{"trace_id": span.TraceID})
	})
	
	// Make request
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.NotEmpty(t, rec.Header().Get("X-Trace-ID"))
}

func TestTraceFunction(t *testing.T) {
	tracer := observability.NewSimpleTracer()
	ctx := context.Background()
	
	t.Run("Success", func(t *testing.T) {
		called := false
		err := observability.TraceFunction(ctx, tracer, "test-function", func(ctx context.Context) error {
			called = true
			span := observability.SpanFromContext(ctx)
			assert.NotNil(t, span)
			return nil
		})
		
		assert.NoError(t, err)
		assert.True(t, called)
	})
	
	t.Run("Error", func(t *testing.T) {
		testErr := assert.AnError
		err := observability.TraceFunction(ctx, tracer, "error-function", func(ctx context.Context) error {
			return testErr
		})
		
		assert.Equal(t, testErr, err)
	})
}

func TestNoOpTracer(t *testing.T) {
	tracer := &observability.NoOpTracer{}
	ctx := context.Background()
	
	// Ensure all methods can be called without panic
	newCtx, span := tracer.StartSpan(ctx, "test")
	assert.NotNil(t, newCtx)
	assert.NotNil(t, span)
	
	tracer.AddTags(span, map[string]string{"key": "value"})
	tracer.SetStatus(span, observability.SpanStatusOK)
	tracer.FinishSpan(span)
}

func TestContextWithSpan(t *testing.T) {
	ctx := context.Background()
	span := &observability.Span{
		TraceID: "test-trace",
		SpanID:  "test-span",
	}
	
	// Add span to context
	ctx = observability.ContextWithSpan(ctx, span)
	
	// Retrieve span from context
	retrieved := observability.SpanFromContext(ctx)
	require.NotNil(t, retrieved)
	assert.Equal(t, span.TraceID, retrieved.TraceID)
	assert.Equal(t, span.SpanID, retrieved.SpanID)
	
	// Test with empty context
	emptyCtx := context.Background()
	assert.Nil(t, observability.SpanFromContext(emptyCtx))
}