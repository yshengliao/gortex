package tracing

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSpanStatus_String(t *testing.T) {
	tests := []struct {
		status   SpanStatus
		expected string
	}{
		{SpanStatusUnset, "UNSET"},
		{SpanStatusOK, "OK"},
		{SpanStatusError, "ERROR"},
		{SpanStatusDEBUG, "DEBUG"},
		{SpanStatusINFO, "INFO"},
		{SpanStatusNOTICE, "NOTICE"},
		{SpanStatusWARN, "WARN"},
		{SpanStatusERROR, "ERROR"},
		{SpanStatusCRITICAL, "CRITICAL"},
		{SpanStatusALERT, "ALERT"},
		{SpanStatusEMERGENCY, "EMERGENCY"},
		{SpanStatus(999), "UNKNOWN(999)"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.status.String())
		})
	}
}

func TestSpanStatus_IsMoreSevere(t *testing.T) {
	tests := []struct {
		name     string
		status   SpanStatus
		other    SpanStatus
		expected bool
	}{
		{"DEBUG less than INFO", SpanStatusDEBUG, SpanStatusINFO, false},
		{"WARN less than ERROR", SpanStatusWARN, SpanStatusERROR, false},
		{"ERROR less than CRITICAL", SpanStatusERROR, SpanStatusCRITICAL, false},
		{"EMERGENCY more than ALERT", SpanStatusEMERGENCY, SpanStatusALERT, true},
		{"CRITICAL more than WARN", SpanStatusCRITICAL, SpanStatusWARN, true},
		{"Same severity", SpanStatusERROR, SpanStatusERROR, false},
		{"Legacy Error maps to ERROR", SpanStatusError, SpanStatusERROR, false},
		{"OK less than DEBUG", SpanStatusOK, SpanStatusDEBUG, false},
		{"Unset least severe", SpanStatusUnset, SpanStatusDEBUG, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.status.IsMoreSevere(tt.other))
		})
	}
}

func TestEnhancedSpan_LogEvent(t *testing.T) {
	span := &Span{
		SpanID:    "test-span",
		TraceID:   "test-trace",
		Operation: "test-operation",
		StartTime: time.Now(),
		Tags:      make(map[string]string),
		Events:    make([]Event, 0),
	}

	enhancedSpan := &EnhancedSpan{Span: span}

	// Log events with different severities
	enhancedSpan.LogEvent(SpanStatusDEBUG, "Debug message", map[string]any{"key": "value"})
	enhancedSpan.LogEvent(SpanStatusINFO, "Info message", nil)
	enhancedSpan.LogEvent(SpanStatusWARN, "Warning message", map[string]any{"count": 10})

	assert.Len(t, enhancedSpan.Events, 3)

	// Check first event
	assert.Equal(t, SpanStatusDEBUG, enhancedSpan.Events[0].Severity)
	assert.Equal(t, "Debug message", enhancedSpan.Events[0].Message)
	assert.Equal(t, "value", enhancedSpan.Events[0].Fields["key"])

	// Check second event
	assert.Equal(t, SpanStatusINFO, enhancedSpan.Events[1].Severity)
	assert.Equal(t, "Info message", enhancedSpan.Events[1].Message)
	assert.Nil(t, enhancedSpan.Events[1].Fields)

	// Check third event
	assert.Equal(t, SpanStatusWARN, enhancedSpan.Events[2].Severity)
	assert.Equal(t, "Warning message", enhancedSpan.Events[2].Message)
	assert.Equal(t, 10, enhancedSpan.Events[2].Fields["count"])
}

func TestEnhancedSpan_SetError(t *testing.T) {
	span := &Span{
		SpanID:    "test-span",
		TraceID:   "test-trace",
		Operation: "test-operation",
		StartTime: time.Now(),
		Tags:      make(map[string]string),
		Events:    make([]Event, 0),
		Status:    SpanStatusOK,
	}

	enhancedSpan := &EnhancedSpan{Span: span}

	testErr := errors.New("test error")
	enhancedSpan.SetError(testErr)

	// Check that error was set
	assert.Equal(t, testErr, enhancedSpan.Error)
	assert.Equal(t, SpanStatusERROR, enhancedSpan.Status)

	// Check that error event was logged
	require.Len(t, enhancedSpan.Events, 1)
	assert.Equal(t, SpanStatusERROR, enhancedSpan.Events[0].Severity)
	assert.Equal(t, "Error occurred", enhancedSpan.Events[0].Message)
	assert.Equal(t, "test error", enhancedSpan.Events[0].Fields["error"])
}

func TestEnhancedSpan_AddTags(t *testing.T) {
	span := &Span{
		SpanID:    "test-span",
		TraceID:   "test-trace",
		Operation: "test-operation",
		StartTime: time.Now(),
		Tags:      make(map[string]string),
	}

	enhancedSpan := &EnhancedSpan{Span: span}

	// Add initial tags
	enhancedSpan.AddTags(map[string]string{
		"service": "auth",
		"version": "1.0",
	})

	assert.Equal(t, "auth", enhancedSpan.Tags["service"])
	assert.Equal(t, "1.0", enhancedSpan.Tags["version"])

	// Add more tags
	enhancedSpan.AddTags(map[string]string{
		"environment": "production",
		"version":     "2.0", // Override existing
	})

	assert.Equal(t, "auth", enhancedSpan.Tags["service"])
	assert.Equal(t, "2.0", enhancedSpan.Tags["version"])
	assert.Equal(t, "production", enhancedSpan.Tags["environment"])
}

func TestSimpleTracer_StartEnhancedSpan(t *testing.T) {
	tracer := NewSimpleTracer()
	ctx := context.Background()

	// Start enhanced span
	ctx, enhancedSpan := tracer.StartEnhancedSpan(ctx, "test-operation")

	assert.NotNil(t, enhancedSpan)
	assert.NotNil(t, enhancedSpan.Span)
	assert.Equal(t, "test-operation", enhancedSpan.Operation)
	assert.NotEmpty(t, enhancedSpan.TraceID)
	assert.NotEmpty(t, enhancedSpan.SpanID)
	assert.Empty(t, enhancedSpan.ParentID)

	// Verify context contains both regular and enhanced span
	assert.Equal(t, enhancedSpan.Span, SpanFromContext(ctx))
	assert.Equal(t, enhancedSpan, EnhancedSpanFromContext(ctx))

	// Start child enhanced span
	childCtx, childSpan := tracer.StartEnhancedSpan(ctx, "child-operation")

	assert.NotNil(t, childSpan)
	assert.Equal(t, enhancedSpan.TraceID, childSpan.TraceID)
	assert.Equal(t, enhancedSpan.SpanID, childSpan.ParentID)
	assert.NotEqual(t, enhancedSpan.SpanID, childSpan.SpanID)

	// Verify child context
	assert.Equal(t, childSpan.Span, SpanFromContext(childCtx))
	assert.Equal(t, childSpan, EnhancedSpanFromContext(childCtx))
}

func TestEnhancedSpan_NilSafety(t *testing.T) {
	// Test nil span safety
	enhancedSpan := &EnhancedSpan{Span: nil}

	// These should not panic
	enhancedSpan.LogEvent(SpanStatusINFO, "test", nil)
	enhancedSpan.SetError(errors.New("test"))
	enhancedSpan.AddTags(map[string]string{"key": "value"})
	enhancedSpan.SetStatus(SpanStatusOK)
}

func TestEnhancedSpan_CompleteWorkflow(t *testing.T) {
	tracer := NewSimpleTracer()
	ctx := context.Background()

	// Simulate a complete workflow
	ctx, span := tracer.StartEnhancedSpan(ctx, "process-request")

	// Add initial tags
	span.AddTags(map[string]string{
		"service":    "api",
		"endpoint":   "/users",
		"method":     "GET",
	})

	// Log some debug info
	span.LogEvent(SpanStatusDEBUG, "Starting request processing", map[string]any{
		"user_id": 123,
	})

	// Simulate some work
	span.LogEvent(SpanStatusINFO, "User authenticated", nil)
	
	// Simulate a warning
	span.LogEvent(SpanStatusWARN, "Cache miss", map[string]any{
		"cache_key": "user:123",
	})

	// Simulate an error
	err := errors.New("database connection failed")
	span.SetError(err)

	// Finish the span
	tracer.FinishSpan(span.Span)

	// Verify the span state
	assert.Equal(t, SpanStatusERROR, span.Status)
	assert.Equal(t, err, span.Error)
	assert.Len(t, span.Events, 4) // 3 manual + 1 from SetError
	assert.NotZero(t, span.EndTime)
}