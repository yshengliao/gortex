package otel

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yshengliao/gortex/observability/tracing"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func setupTest(t *testing.T) (*OTelTracerAdapter, *tracetest.SpanRecorder) {
	// Create a span recorder for testing
	spanRecorder := tracetest.NewSpanRecorder()
	
	// Create a trace provider with the span recorder
	tp := trace.NewTracerProvider(
		trace.WithSpanProcessor(spanRecorder),
	)
	
	// Set as global tracer provider
	otel.SetTracerProvider(tp)
	
	// Create Gortex tracer
	gortexTracer := tracing.NewSimpleTracer()
	
	// Create adapter
	adapter := NewOTelTracerAdapter(gortexTracer, "test-tracer")
	
	return adapter, spanRecorder
}

func TestOTelTracerAdapter_StartSpan(t *testing.T) {
	adapter, recorder := setupTest(t)
	ctx := context.Background()
	
	// Test with enhanced span for full functionality
	ctx, enhancedSpan := adapter.StartEnhancedSpan(ctx, "test-operation")
	require.NotNil(t, enhancedSpan)
	assert.Equal(t, "test-operation", enhancedSpan.Span.Operation)
	
	// Get the span adapter from context
	spanAdapter := SpanAdapterFromContext(ctx)
	require.NotNil(t, spanAdapter)
	
	// End the span properly
	spanAdapter.End()
	
	// Verify OpenTelemetry span was recorded
	spans := recorder.Ended()
	require.Len(t, spans, 1)
	assert.Equal(t, "test-operation", spans[0].Name())
}

func TestSpanAdapter_LogEvent(t *testing.T) {
	adapter, recorder := setupTest(t)
	ctx := context.Background()
	
	// Use StartSpanWithOptions to get SpanAdapter
	ctx, spanAdapter := adapter.StartSpanWithOptions(ctx, "test-operation")
	
	// Log events with different severities
	spanAdapter.LogEvent(tracing.SpanStatusDEBUG, "Debug message", map[string]any{
		"key": "value",
		"count": 10,
	})
	
	spanAdapter.LogEvent(tracing.SpanStatusERROR, "Error message", map[string]any{
		"error_code": "E001",
	})
	
	spanAdapter.End()
	
	// Verify Gortex span has events
	assert.Len(t, spanAdapter.gortexSpan.Events, 2)
	assert.Equal(t, tracing.SpanStatusDEBUG, spanAdapter.gortexSpan.Events[0].Severity)
	assert.Equal(t, "Debug message", spanAdapter.gortexSpan.Events[0].Message)
	
	// Verify OpenTelemetry span has events
	spans := recorder.Ended()
	require.Len(t, spans, 1)
	otelSpan := spans[0]
	
	events := otelSpan.Events()
	assert.Len(t, events, 2)
	
	// Check first event
	assert.Equal(t, "Debug message", events[0].Name)
	attrs := events[0].Attributes
	assert.Contains(t, attrs, attribute.String("severity", "DEBUG"))
	assert.Contains(t, attrs, attribute.Int("severity.number", 5))
	assert.Contains(t, attrs, attribute.String("event.name", "Debug message"))
	assert.Contains(t, attrs, attribute.String("key", "value"))
	assert.Contains(t, attrs, attribute.Int("count", 10))
}

func TestSpanAdapter_SetError(t *testing.T) {
	adapter, recorder := setupTest(t)
	ctx := context.Background()
	
	ctx, spanAdapter := adapter.StartSpanWithOptions(ctx, "test-operation")
	
	// Set an error
	testErr := errors.New("test error")
	spanAdapter.SetError(testErr)
	
	spanAdapter.End()
	
	// Verify Gortex span has error
	assert.Equal(t, testErr, spanAdapter.gortexSpan.Error)
	assert.Equal(t, tracing.SpanStatusERROR, spanAdapter.gortexSpan.Status)
	
	// Verify OpenTelemetry span has error
	spans := recorder.Ended()
	require.Len(t, spans, 1)
	otelSpan := spans[0]
	
	// Check status
	assert.Equal(t, "Error", otelSpan.Status().Code.String())
	assert.Equal(t, "test error", otelSpan.Status().Description)
	
	// Check error event
	events := otelSpan.Events()
	hasErrorEvent := false
	for _, event := range events {
		for _, attr := range event.Attributes {
			if attr.Key == "exception.message" && attr.Value.AsString() == "test error" {
				hasErrorEvent = true
				break
			}
		}
	}
	assert.True(t, hasErrorEvent, "Error event not found")
}

func TestSpanAdapter_AddTags(t *testing.T) {
	adapter, recorder := setupTest(t)
	ctx := context.Background()
	
	ctx, spanAdapter := adapter.StartSpanWithOptions(ctx, "test-operation")
	
	// Add tags
	tags := map[string]string{
		"service":     "auth",
		"version":     "1.0",
		"environment": "test",
	}
	spanAdapter.AddTags(tags)
	
	spanAdapter.End()
	
	// Verify Gortex span has tags
	for k, v := range tags {
		assert.Equal(t, v, spanAdapter.gortexSpan.Tags[k])
	}
	
	// Verify OpenTelemetry span has attributes
	spans := recorder.Ended()
	require.Len(t, spans, 1)
	otelSpan := spans[0]
	
	attrs := otelSpan.Attributes()
	assert.Contains(t, attrs, attribute.String("service", "auth"))
	assert.Contains(t, attrs, attribute.String("version", "1.0"))
	assert.Contains(t, attrs, attribute.String("environment", "test"))
}

func TestSpanAdapter_SetStatus(t *testing.T) {
	adapter, recorder := setupTest(t)
	
	tests := []struct {
		name           string
		status         tracing.SpanStatus
		expectedCode   string
		expectedSeverity string
	}{
		{"OK status", tracing.SpanStatusOK, "Ok", "OK"},
		{"ERROR status", tracing.SpanStatusERROR, "Error", "ERROR"},
		{"CRITICAL status", tracing.SpanStatusCRITICAL, "Error", "CRITICAL"},
		{"WARN status", tracing.SpanStatusWARN, "Unset", "WARN"},
		{"INFO status", tracing.SpanStatusINFO, "Unset", "INFO"},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			ctx, spanAdapter := adapter.StartSpanWithOptions(ctx, "test-operation")
			
			spanAdapter.SetStatus(tt.status)
			spanAdapter.End()
			
			// Verify Gortex span status
			assert.Equal(t, tt.status, spanAdapter.gortexSpan.Status)
			
			// Verify OpenTelemetry span status
			spans := recorder.Ended()
			lastSpan := spans[len(spans)-1]
			
			assert.Equal(t, tt.expectedCode, lastSpan.Status().Code.String())
			
			// Check severity attribute
			attrs := lastSpan.Attributes()
			assert.Contains(t, attrs, attribute.String("severity", tt.expectedSeverity))
		})
	}
}

func TestSeverityMapping(t *testing.T) {
	severityMap := initSeverityMap()
	
	// Test all severity levels are mapped
	statuses := []tracing.SpanStatus{
		tracing.SpanStatusUnset,
		tracing.SpanStatusOK,
		tracing.SpanStatusError,
		tracing.SpanStatusDEBUG,
		tracing.SpanStatusINFO,
		tracing.SpanStatusNOTICE,
		tracing.SpanStatusWARN,
		tracing.SpanStatusERROR,
		tracing.SpanStatusCRITICAL,
		tracing.SpanStatusALERT,
		tracing.SpanStatusEMERGENCY,
	}
	
	for _, status := range statuses {
		attr, ok := severityMap[status]
		assert.True(t, ok, "Status %v not mapped", status)
		assert.Equal(t, "severity", string(attr.Key))
	}
}

func TestSeverityToNumber(t *testing.T) {
	tests := []struct {
		status   tracing.SpanStatus
		expected int
	}{
		{tracing.SpanStatusUnset, 0},
		{tracing.SpanStatusOK, 1},
		{tracing.SpanStatusDEBUG, 5},
		{tracing.SpanStatusINFO, 9},
		{tracing.SpanStatusNOTICE, 10},
		{tracing.SpanStatusWARN, 13},
		{tracing.SpanStatusError, 17},
		{tracing.SpanStatusERROR, 17},
		{tracing.SpanStatusCRITICAL, 21},
		{tracing.SpanStatusALERT, 22},
		{tracing.SpanStatusEMERGENCY, 24},
	}
	
	for _, tt := range tests {
		t.Run(tt.status.String(), func(t *testing.T) {
			assert.Equal(t, tt.expected, severityToNumber(tt.status))
		})
	}
}

func TestHTTPMiddleware(t *testing.T) {
	adapter, recorder := setupTest(t)
	
	// Create a test handler
	handlerCalled := false
	handler := func(ctx context.Context) error {
		handlerCalled = true
		
		// Verify span is in context
		span := SpanAdapterFromContext(ctx)
		assert.NotNil(t, span)
		
		// Add some tags from within handler
		span.AddTags(map[string]string{
			"handler": "test",
		})
		
		return nil
	}
	
	// Wrap with middleware
	wrapped := adapter.HTTPMiddleware(handler)
	
	// Execute
	err := wrapped(context.Background())
	assert.NoError(t, err)
	assert.True(t, handlerCalled)
	
	// Verify span was created and ended
	spans := recorder.Ended()
	require.Len(t, spans, 1)
	
	otelSpan := spans[0]
	assert.Equal(t, "HTTP Request", otelSpan.Name())
	assert.Equal(t, "server", otelSpan.SpanKind().String())
	
	// Check attributes
	attrs := otelSpan.Attributes()
	assert.Contains(t, attrs, attribute.String("handler", "test"))
}