# Tracing Migration Guide

This guide helps you migrate from the basic tracing implementation to the enhanced tracing system with 8 severity levels and OpenTelemetry support.

## Overview of Changes

The enhanced tracing system is **fully backward compatible**. Your existing code will continue to work without modifications. The enhancements are opt-in features that you can adopt gradually.

### New Features

1. **8 Severity Levels**: DEBUG, INFO, NOTICE, WARN, ERROR, CRITICAL, ALERT, EMERGENCY
2. **Enhanced Span Interface**: `LogEvent()` and `SetError()` methods
3. **OpenTelemetry Integration**: Bidirectional adapter for OTLP export
4. **Automatic Middleware Injection**: TracingMiddleware auto-configured with `WithTracer()`
5. **Context Integration**: Access spans via `Context.Span()`

## API Changes Reference

### Status Constants

| Old API | New API | Notes |
|---------|---------|-------|
| `SpanStatusUnset` | `SpanStatusUnset` | No change |
| `SpanStatusOK` | `SpanStatusOK` | No change |
| `SpanStatusError` | `SpanStatusError` | Maps to `SpanStatusERROR` (50) |
| - | `SpanStatusDEBUG` (10) | New |
| - | `SpanStatusINFO` (20) | New |
| - | `SpanStatusNOTICE` (30) | New |
| - | `SpanStatusWARN` (40) | New |
| - | `SpanStatusERROR` (50) | New (equivalent to old Error) |
| - | `SpanStatusCRITICAL` (60) | New |
| - | `SpanStatusALERT` (70) | New |
| - | `SpanStatusEMERGENCY` (80) | New |

### Tracer Interface

The base `Tracer` interface remains unchanged:

```go
type Tracer interface {
    StartSpan(ctx context.Context, operation string) (context.Context, *Span)
    FinishSpan(span *Span)
    AddTags(span *Span, tags map[string]string)
    SetStatus(span *Span, status SpanStatus)
}
```

New `EnhancedTracer` interface (optional):

```go
type EnhancedTracer interface {
    Tracer
    StartEnhancedSpan(ctx context.Context, operation string) (context.Context, *EnhancedSpan)
}
```

### Context Interface

New method added:

```go
type Context interface {
    // ... existing methods ...
    Span() interface{} // Returns current span from context
}
```

## Migration Steps

### Step 1: Update Imports (No Changes Needed)

Your existing imports remain the same:

```go
import (
    "github.com/yshengliao/gortex/observability/tracing"
)
```

### Step 2: Enable Enhanced Tracing (Optional)

#### Option A: Continue Using Basic Tracing

No changes needed. Your existing code continues to work:

```go
tracer := tracing.NewSimpleTracer()
ctx, span := tracer.StartSpan(ctx, "operation")
defer tracer.FinishSpan(span)
```

#### Option B: Upgrade to Enhanced Tracing

Cast your tracer and spans to use new features:

```go
// If using SimpleTracer, it now implements EnhancedTracer
tracer := tracing.NewSimpleTracer()

// Option 1: Use enhanced spans directly
if enhancedTracer, ok := tracer.(tracing.EnhancedTracer); ok {
    ctx, enhancedSpan := enhancedTracer.StartEnhancedSpan(ctx, "operation")
    defer tracer.FinishSpan(enhancedSpan.Span)
    
    // Use enhanced features
    enhancedSpan.LogEvent(tracing.SpanStatusINFO, "Processing", nil)
    enhancedSpan.SetError(err)
}

// Option 2: Cast regular spans
ctx, span := tracer.StartSpan(ctx, "operation")
if enhancedSpan, ok := span.(*tracing.EnhancedSpan); ok {
    enhancedSpan.LogEvent(tracing.SpanStatusWARN, "Slow operation", nil)
}
```

### Step 3: Update Middleware Configuration

#### Old Way (Manual Middleware Registration):

```go
app.Use(tracing.TracingMiddleware(tracer))
```

#### New Way (Automatic with WithTracer):

```go
app, _ := app.NewApp(
    app.WithTracer(tracer), // Automatically injects TracingMiddleware
)
```

Both approaches work, but the new way is cleaner.

### Step 4: Access Spans in Handlers

#### Old Way:

```go
// Manual span management
func (h *Handler) GET(c Context) error {
    ctx, span := h.tracer.StartSpan(c.Request().Context(), "handler")
    defer h.tracer.FinishSpan(span)
    // ...
}
```

#### New Way:

```go
func (h *Handler) GET(c Context) error {
    // Access auto-created span
    if span := c.Span(); span != nil {
        if enhancedSpan, ok := span.(*tracing.EnhancedSpan); ok {
            enhancedSpan.LogEvent(tracing.SpanStatusINFO, "Handler called", nil)
        }
    }
    // ...
}
```

## Code Examples

### Example 1: Minimal Changes (Backward Compatible)

```go
// This code works exactly as before
func ProcessRequest(ctx context.Context, tracer tracing.Tracer) error {
    ctx, span := tracer.StartSpan(ctx, "process_request")
    defer tracer.FinishSpan(span)
    
    tracer.AddTags(span, map[string]string{
        "request.id": "123",
    })
    
    if err := doWork(ctx); err != nil {
        tracer.SetStatus(span, tracing.SpanStatusError)
        return err
    }
    
    tracer.SetStatus(span, tracing.SpanStatusOK)
    return nil
}
```

### Example 2: Using Enhanced Features

```go
func ProcessRequestEnhanced(ctx context.Context, tracer tracing.Tracer) error {
    // Start enhanced span if supported
    var enhancedSpan *tracing.EnhancedSpan
    if et, ok := tracer.(tracing.EnhancedTracer); ok {
        ctx, enhancedSpan = et.StartEnhancedSpan(ctx, "process_request")
        defer tracer.FinishSpan(enhancedSpan.Span)
    } else {
        // Fallback to regular span
        ctx, span := tracer.StartSpan(ctx, "process_request")
        defer tracer.FinishSpan(span)
    }
    
    // Log events if enhanced span available
    if enhancedSpan != nil {
        enhancedSpan.LogEvent(tracing.SpanStatusDEBUG, "Starting processing", map[string]any{
            "timestamp": time.Now(),
        })
    }
    
    if err := doWork(ctx); err != nil {
        if enhancedSpan != nil {
            enhancedSpan.SetError(err) // Automatically sets status and logs
        }
        return err
    }
    
    if enhancedSpan != nil {
        enhancedSpan.LogEvent(tracing.SpanStatusINFO, "Processing completed", nil)
    }
    return nil
}
```

### Example 3: Handler Migration

```go
// Old handler
type OldHandler struct {
    tracer tracing.Tracer
}

func (h *OldHandler) GET(c Context) error {
    // Manual span creation
    ctx, span := h.tracer.StartSpan(c.Request().Context(), "handler")
    defer h.tracer.FinishSpan(span)
    
    // Do work...
    return c.JSON(200, result)
}

// New handler (no tracer needed)
type NewHandler struct{}

func (h *NewHandler) GET(c Context) error {
    // Access auto-created span
    if span := c.Span(); span != nil {
        if es, ok := span.(*tracing.EnhancedSpan); ok {
            es.LogEvent(tracing.SpanStatusINFO, "Processing request", map[string]any{
                "path": c.Path(),
                "method": c.Request().Method,
            })
        }
    }
    
    // Do work...
    return c.JSON(200, result)
}
```

## Configuration Updates

### Old Configuration

```go
// Manual tracer setup
tracer := tracing.NewSimpleTracer()
app := app.NewApp()
app.Use(tracing.TracingMiddleware(tracer))
```

### New Configuration

```go
// Automatic middleware injection
tracer := tracing.NewSimpleTracer()
app, _ := app.NewApp(
    app.WithTracer(tracer), // TracingMiddleware auto-injected
)
```

### YAML Configuration (New)

```yaml
tracing:
  enabled: true
  service_name: my-service
  sampling_rate: 1.0
  
  # OpenTelemetry export
  otlp:
    endpoint: localhost:4317
    insecure: true
    
  # Or Jaeger export
  jaeger:
    endpoint: http://localhost:14268/api/traces
```

## OpenTelemetry Integration

To export traces to OpenTelemetry:

```go
import (
    "github.com/yshengliao/gortex/observability/otel"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
)

// Create OTLP exporter
exporter, _ := otlptracegrpc.New(ctx,
    otlptracegrpc.WithEndpoint("localhost:4317"),
    otlptracegrpc.WithInsecure(),
)

// Create tracer provider
provider := trace.NewTracerProvider(
    trace.WithBatcher(exporter),
)
otel.SetTracerProvider(provider)

// Create Gortex adapter
otelTracer := provider.Tracer("gortex")
adapter := otel.NewOTelTracerAdapter(
    tracing.NewSimpleTracer(),
    otelTracer,
)

// Use adapter as your tracer
app, _ := app.NewApp(
    app.WithTracer(adapter),
)
```

## Common Patterns

### Pattern 1: Conditional Enhancement

```go
// Works with both basic and enhanced tracers
func FlexibleTracing(ctx context.Context, tracer tracing.Tracer) {
    ctx, span := tracer.StartSpan(ctx, "operation")
    defer tracer.FinishSpan(span)
    
    // Try to use enhanced features if available
    if es, ok := span.(*tracing.EnhancedSpan); ok {
        es.LogEvent(tracing.SpanStatusDEBUG, "Enhanced features available", nil)
    }
    
    // Regular tracing still works
    tracer.AddTags(span, map[string]string{"key": "value"})
}
```

### Pattern 2: Error Handling

```go
// Old way
if err != nil {
    tracer.SetStatus(span, tracing.SpanStatusError)
    tracer.AddTags(span, map[string]string{"error": err.Error()})
}

// New way (with enhanced span)
if err != nil {
    enhancedSpan.SetError(err) // Does everything automatically
}
```

### Pattern 3: Severity-Based Logging

```go
func LogBySeverity(span *tracing.EnhancedSpan, level string, msg string) {
    switch level {
    case "debug":
        span.LogEvent(tracing.SpanStatusDEBUG, msg, nil)
    case "info":
        span.LogEvent(tracing.SpanStatusINFO, msg, nil)
    case "warn":
        span.LogEvent(tracing.SpanStatusWARN, msg, nil)
    case "error":
        span.LogEvent(tracing.SpanStatusERROR, msg, nil)
    default:
        span.LogEvent(tracing.SpanStatusINFO, msg, nil)
    }
}
```

## Testing Considerations

### Testing with Enhanced Tracing

```go
func TestWithEnhancedTracing(t *testing.T) {
    // Create test tracer
    tracer := tracing.NewSimpleTracer()
    
    // Create app with tracer
    app, _ := app.NewApp(
        app.WithTracer(tracer),
        app.WithHandlers(handlers),
    )
    
    // Make test request
    req := httptest.NewRequest("GET", "/test", nil)
    rec := httptest.NewRecorder()
    
    app.ServeHTTP(rec, req)
    
    // Verify trace ID in response
    assert.NotEmpty(t, rec.Header().Get("X-Trace-ID"))
}
```

### Mocking Enhanced Spans

```go
type MockEnhancedSpan struct {
    *tracing.Span
    Events []tracing.Event
}

func (m *MockEnhancedSpan) LogEvent(severity tracing.SpanStatus, msg string, fields map[string]any) {
    m.Events = append(m.Events, tracing.Event{
        Timestamp: time.Now(),
        Severity:  severity,
        Message:   msg,
        Fields:    fields,
    })
}

func (m *MockEnhancedSpan) SetError(err error) {
    m.Error = err
    m.Status = tracing.SpanStatusERROR
}
```

## Troubleshooting

### Issue: Spans not appearing in handlers

**Solution**: Ensure `WithTracer()` is configured:

```go
app, _ := app.NewApp(
    app.WithTracer(tracer), // Required for auto-injection
)
```

### Issue: Enhanced features not available

**Solution**: Check tracer type:

```go
if _, ok := tracer.(tracing.EnhancedTracer); !ok {
    log.Println("Tracer doesn't support enhanced features")
}
```

### Issue: Type assertion failures

**Solution**: Always check type assertions:

```go
if span := c.Span(); span != nil {
    // Safe type assertion
    if es, ok := span.(*tracing.EnhancedSpan); ok {
        es.LogEvent(...)
    }
}
```

## Best Practices

1. **Gradual Migration**: Start with backward-compatible changes
2. **Type Safety**: Always check type assertions
3. **Severity Levels**: Use appropriate levels for your use case
4. **Performance**: Enhanced features have minimal overhead
5. **Testing**: Test both basic and enhanced tracing paths

## Summary

The enhanced tracing system provides powerful new features while maintaining full backward compatibility. You can:

- Continue using existing code without changes
- Gradually adopt enhanced features
- Mix basic and enhanced tracing in the same application
- Export to OpenTelemetry and other APM systems

No breaking changes means zero risk in upgrading!