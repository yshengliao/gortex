# Gortex Tracing Example

This example demonstrates the enhanced tracing capabilities of the Gortex framework, including:

- 8 severity levels for span events
- Enhanced span interface with `LogEvent()` and `SetError()` methods
- Child span creation
- Error tracking and propagation
- Integration with Jaeger and OpenTelemetry

## Features Demonstrated

1. **Basic Tracing** - Automatic span creation for HTTP requests
2. **Child Spans** - Creating nested spans for database operations
3. **Event Logging** - Using all 8 severity levels (DEBUG to EMERGENCY)
4. **Error Tracking** - Automatic error capture and propagation
5. **Custom Attributes** - Adding metadata to spans
6. **Context Propagation** - Accessing spans from handlers

## Running the Example

### 1. Start the Tracing Infrastructure (Optional)

If you want to visualize traces, start Jaeger:

```bash
docker-compose up -d jaeger
```

Access Jaeger UI at: http://localhost:16686

### 2. Run the Example

```bash
go run main.go
```

The server will start on port 8084.

### 3. Test the Endpoints

```bash
# Basic tracing
curl http://localhost:8084/

# Child spans (database query simulation)
curl http://localhost:8084/users

# Success case
curl http://localhost:8084/users/123

# Error case (demonstrates error tracking)
curl http://localhost:8084/users/999

# Multiple severity levels
curl http://localhost:8084/products

# Critical error
curl -X POST http://localhost:8084/order

# All severity levels demonstration
curl http://localhost:8084/analytics
```

## Code Examples

### Accessing Span in Handler

```go
func (h *HomeHandler) GET(c httpctx.Context) error {
    // Get span from context
    if span := c.Span(); span != nil {
        if enhancedSpan, ok := span.(*tracing.EnhancedSpan); ok {
            // Log an event
            enhancedSpan.LogEvent(tracing.SpanStatusINFO, "Processing request", map[string]any{
                "user_id": "123",
            })
        }
    }
    return c.JSON(200, response)
}
```

### Creating Child Spans

```go
func (h *UsersHandler) GET(c httpctx.Context) error {
    // Create a child span
    ctx, dbSpan := h.tracer.StartSpan(c.Request().Context(), "database.query")
    defer h.tracer.FinishSpan(dbSpan)
    
    // Add tags
    h.tracer.AddTags(dbSpan, map[string]string{
        "db.type": "postgresql",
        "db.query": "SELECT * FROM users",
    })
    
    // Perform operation
    result, err := h.performQuery(ctx)
    if err != nil {
        if enhancedSpan, ok := dbSpan.(*tracing.EnhancedSpan); ok {
            enhancedSpan.SetError(err)
        }
        return err
    }
    
    return c.JSON(200, result)
}
```

### Using Severity Levels

```go
// DEBUG - Detailed information for debugging
enhancedSpan.LogEvent(tracing.SpanStatusDEBUG, "Variable state", map[string]any{"var": value})

// INFO - General informational messages
enhancedSpan.LogEvent(tracing.SpanStatusINFO, "User logged in", map[string]any{"user_id": id})

// NOTICE - Normal but significant events
enhancedSpan.LogEvent(tracing.SpanStatusNOTICE, "New feature accessed", nil)

// WARN - Warning messages
enhancedSpan.LogEvent(tracing.SpanStatusWARN, "Slow query", map[string]any{"duration_ms": 2000})

// ERROR - Error events that don't stop execution
enhancedSpan.LogEvent(tracing.SpanStatusERROR, "Retry failed", map[string]any{"attempt": 3})

// CRITICAL - Critical problems that need attention
enhancedSpan.LogEvent(tracing.SpanStatusCRITICAL, "Service degraded", nil)

// ALERT - Immediate action required
enhancedSpan.LogEvent(tracing.SpanStatusALERT, "Memory threshold exceeded", nil)

// EMERGENCY - System is unusable
enhancedSpan.LogEvent(tracing.SpanStatusEMERGENCY, "Database connection lost", nil)
```

## Configuration

### YAML Configuration

```yaml
tracing:
  enabled: true
  service_name: my-service
  sampling_rate: 1.0
  
  # For Jaeger
  jaeger:
    endpoint: http://localhost:14268/api/traces
    
  # For OTLP
  otlp:
    endpoint: localhost:4317
    insecure: true
```

### Programmatic Configuration

```go
// Create tracer
tracer := tracing.NewSimpleTracer()

// Create app with tracer
app, _ := app.NewApp(
    app.WithTracer(tracer), // Auto-injects TracingMiddleware
)
```

## Full Stack Example

For a complete observability stack with Jaeger, OpenTelemetry Collector, Prometheus, and Grafana:

```bash
# Start all services
docker-compose up -d

# Access UIs
# - Jaeger: http://localhost:16686
# - Prometheus: http://localhost:9090
# - Grafana: http://localhost:3000 (admin/admin)
```

## Production Considerations

1. **Sampling**: In production, use sampling to reduce overhead:
   ```yaml
   tracing:
     sampling_rate: 0.1  # 10% of requests
   ```

2. **Sensitive Data**: Avoid logging sensitive information in spans:
   ```go
   // Bad
   enhancedSpan.LogEvent(tracing.SpanStatusINFO, "User login", map[string]any{
       "password": password,  // Never log passwords!
   })
   
   // Good
   enhancedSpan.LogEvent(tracing.SpanStatusINFO, "User login", map[string]any{
       "user_id": userID,
       "method": "oauth2",
   })
   ```

3. **Performance**: Use appropriate severity levels:
   - DEBUG: Development only
   - INFO/NOTICE: Normal operations
   - WARN and above: Issues requiring attention

4. **Context Propagation**: Ensure trace headers are propagated:
   - W3C Trace Context: `traceparent`, `tracestate`
   - Jaeger: `uber-trace-id`
   - B3: `X-B3-TraceId`, `X-B3-SpanId`

## Troubleshooting

1. **No traces in Jaeger**: Check that Jaeger is running and accessible
2. **Missing spans**: Ensure `WithTracer()` is configured
3. **Performance impact**: Reduce sampling rate or use NoOpTracer in tests

## Next Steps

- Implement OpenTelemetry exporter for production use
- Add custom span processors for filtering/enrichment
- Integrate with your APM solution
- Configure alerts based on trace data