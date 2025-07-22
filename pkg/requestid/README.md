# Request ID Package

The `requestid` package provides utilities for working with request IDs throughout the Gortex application. It enables distributed tracing and request correlation across services.

## Features

- **Context Management**: Store and retrieve request IDs from standard Go contexts
- **Echo Integration**: Seamless integration with Echo framework contexts
- **HTTP Propagation**: Automatic request ID propagation to outgoing HTTP requests
- **Logger Integration**: Automatic inclusion of request IDs in log entries
- **High Performance**: Optimized for minimal overhead (~15ns for context operations)

## Installation

This package is included with the Gortex framework. No separate installation required.

## Usage

### Extracting Request IDs

```go
// From Echo context (checks multiple sources in priority order)
requestID := requestid.FromEchoContext(c)

// From standard Go context
requestID := requestid.FromContext(ctx)

// From HTTP request header
requestID := requestid.GetHeader(req)
```

### Storing Request IDs

```go
// Add to standard context
ctx = requestid.WithContext(ctx, "my-request-id")

// Add from Echo context to standard context
ctx = requestid.WithEchoContext(ctx, echoContext)

// Add to HTTP request header
requestid.SetHeader(req, "my-request-id")
```

### Logger Integration

```go
// Create logger with request ID field
logger := requestid.Logger(baseLogger, "request-id-123")

// Create logger from Echo context
logger := requestid.LoggerFromEcho(baseLogger, c)

// Create logger from standard context
logger := requestid.LoggerFromContext(baseLogger, ctx)
```

### HTTP Client with Automatic Propagation

```go
// Create HTTP client that propagates request IDs
ctx := requestid.WithContext(context.Background(), "request-id-123")
client := requestid.NewHTTPClient(http.DefaultClient, ctx)

// All requests will automatically include the X-Request-ID header
resp, err := client.Get("https://api.example.com/data")

// Also supports other HTTP methods
resp, err := client.Post("https://api.example.com/data", "application/json", body)
```

### Manual Propagation

```go
// Propagate from Echo context to outgoing request
requestid.PropagateToRequest(echoContext, outgoingRequest)

// Propagate from standard context to outgoing request
requestid.PropagateFromContext(ctx, outgoingRequest)
```

## Priority Order

When extracting request IDs from Echo contexts, the following priority order is used:

1. Context value (set by middleware via `c.Set("request_id", value)`)
2. Response header (set by middleware)
3. Request header (from incoming request)

## Performance

The package is designed for high performance with minimal allocations:

- `FromEchoContext`: ~15ns per operation
- `WithContext`: ~33ns per operation
- `Logger`: ~46ns per operation

## Best Practices

1. **Always propagate request IDs** to external service calls for end-to-end tracing
2. **Include request IDs in logs** for easier debugging and correlation
3. **Use the HTTPClient wrapper** when making multiple external calls
4. **Preserve incoming request IDs** rather than generating new ones

## Example: Complete Request Flow

```go
func (h *Handler) ProcessOrder(c echo.Context) error {
    // Get logger with request ID
    logger := requestid.LoggerFromEcho(h.Logger, c)
    logger.Info("Processing order")
    
    // Create context for external calls
    ctx := requestid.WithEchoContext(context.Background(), c)
    
    // Make external API call with request ID propagation
    client := requestid.NewHTTPClient(http.DefaultClient, ctx)
    resp, err := client.Post("https://payment.api/charge", "application/json", paymentData)
    if err != nil {
        logger.Error("Payment failed", zap.Error(err))
        return err
    }
    
    // Request ID is automatically included in all logs and external calls
    logger.Info("Order processed successfully")
    return c.JSON(200, result)
}
```

## Thread Safety

All functions in this package are thread-safe and can be used concurrently.