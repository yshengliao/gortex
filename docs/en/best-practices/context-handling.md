# Context Handling Best Practices

## Overview

Context propagation is a critical aspect of building robust Go applications. In Gortex, proper context handling ensures graceful shutdowns, request cancellation, and timeout management. This guide covers best practices for using `context.Context` effectively throughout your Gortex applications.

## Table of Contents

1. [Context Lifecycle Management](#context-lifecycle-management)
2. [Cancellation Signal Propagation](#cancellation-signal-propagation)
3. [Common Context Leak Patterns](#common-context-leak-patterns)
4. [Working with Goroutines](#working-with-goroutines)
5. [Timeout Best Practices](#timeout-best-practices)
6. [Real-World Example: HTTP Request Tracing](#real-world-example-http-request-tracing)
7. [Performance Considerations](#performance-considerations)
8. [Troubleshooting](#troubleshooting)

## Context Lifecycle Management

### Understanding Context Hierarchy

Every context in Gortex follows a parent-child relationship. When a parent context is cancelled, all its children are automatically cancelled.

```go
// Example 1: Creating child contexts
func (h *UserHandler) GetUserWithDetails(c httpctx.Context) error {
    // Parent context from HTTP request
    parentCtx := c.Request().Context()
    
    // Create child context with timeout for database query
    dbCtx, cancel := context.WithTimeout(parentCtx, 5*time.Second)
    defer cancel() // Always defer cancel to avoid context leaks
    
    // Use the child context for database operations
    user, err := h.db.GetUser(dbCtx, c.Param("id"))
    if err != nil {
        return c.JSON(500, map[string]string{"error": "Database error"})
    }
    
    // Create another child context for external API call
    apiCtx, apiCancel := context.WithTimeout(parentCtx, 3*time.Second)
    defer apiCancel()
    
    details, err := h.api.GetUserDetails(apiCtx, user.ID)
    if err != nil {
        // Log error but don't fail the request
        h.logger.Warn("Failed to fetch user details", zap.Error(err))
    }
    
    return c.JSON(200, map[string]interface{}{
        "user":    user,
        "details": details,
    })
}
```

### Best Practices for Context Creation

1. **Always use request context as parent**: Start with the request context to ensure proper cancellation
2. **Set appropriate timeouts**: Different operations need different timeouts
3. **Always call cancel**: Use defer to ensure cancel is called

```go
// Example 2: Context creation patterns
func (h *Handler) ProcessRequest(c httpctx.Context) error {
    ctx := c.Request().Context()
    
    // Pattern 1: Short-lived operation
    shortCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
    defer cancel()
    
    // Pattern 2: Cancellable operation
    cancelCtx, cancel := context.WithCancel(ctx)
    defer cancel()
    
    // Pattern 3: Operation with deadline
    deadline := time.Now().Add(30 * time.Second)
    deadlineCtx, cancel := context.WithDeadline(ctx, deadline)
    defer cancel()
    
    return nil
}
```

## Cancellation Signal Propagation

### Checking for Context Cancellation

Always check for context cancellation in long-running operations:

```go
// Example 3: Proper cancellation checking
func (h *DataProcessor) ProcessLargeDataset(ctx context.Context, data []Item) error {
    for i, item := range data {
        // Check for cancellation at the start of each iteration
        select {
        case <-ctx.Done():
            return fmt.Errorf("processing cancelled at item %d: %w", i, ctx.Err())
        default:
            // Continue processing
        }
        
        // Process item
        if err := h.processItem(ctx, item); err != nil {
            return fmt.Errorf("failed to process item %d: %w", i, err)
        }
        
        // For very long operations, check more frequently
        if i%100 == 0 {
            select {
            case <-ctx.Done():
                return ctx.Err()
            default:
            }
        }
    }
    return nil
}
```

### Propagating Context Through Channels

When using channels, always include a context for cancellation:

```go
// Example 4: Context with channels
func (h *WorkerPool) ProcessJobs(ctx context.Context, jobs <-chan Job) error {
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case job, ok := <-jobs:
            if !ok {
                return nil // Channel closed
            }
            
            // Process job with context
            if err := h.processJob(ctx, job); err != nil {
                h.logger.Error("Job processing failed", 
                    zap.String("job_id", job.ID),
                    zap.Error(err))
            }
        }
    }
}
```

## Common Context Leak Patterns

### Pattern 1: Forgetting to Cancel

```go
// ❌ BAD: Context leak - cancel is never called
func BadExample(ctx context.Context) {
    newCtx, _ := context.WithTimeout(ctx, 5*time.Second)
    doSomething(newCtx)
    // Missing cancel() call - context will leak!
}

// ✅ GOOD: Proper cleanup
func GoodExample(ctx context.Context) {
    newCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel() // Always defer cancel immediately
    doSomething(newCtx)
}
```

### Pattern 2: Storing Context in Structs

```go
// ❌ BAD: Storing context in struct fields
type BadService struct {
    ctx context.Context // Don't do this!
}

// ✅ GOOD: Pass context as parameter
type GoodService struct {
    logger *zap.Logger
}

func (s *GoodService) DoWork(ctx context.Context) error {
    // Use the passed context
    return nil
}
```

### Pattern 3: Using Wrong Context

```go
// Example 5: Using the correct context
func (h *Handler) ComplexOperation(c httpctx.Context) error {
    // ❌ BAD: Using background context ignores cancellation
    badResult, err := h.service.Process(context.Background(), data)
    
    // ✅ GOOD: Using request context propagates cancellation
    ctx := c.Request().Context()
    goodResult, err := h.service.Process(ctx, data)
    
    if err != nil {
        return c.JSON(500, map[string]string{"error": err.Error()})
    }
    
    return c.JSON(200, goodResult)
}
```

## Working with Goroutines

### Passing Context to Goroutines

Always pass context to goroutines and handle cancellation:

```go
// Example 6: Context with goroutines
func (h *AsyncHandler) ProcessAsync(c httpctx.Context) error {
    ctx := c.Request().Context()
    results := make(chan Result, 10)
    errors := make(chan error, 10)
    
    // Start workers
    var wg sync.WaitGroup
    for i := 0; i < 5; i++ {
        wg.Add(1)
        go func(workerID int) {
            defer wg.Done()
            
            // Worker respects context cancellation
            for {
                select {
                case <-ctx.Done():
                    h.logger.Info("Worker shutting down", 
                        zap.Int("worker_id", workerID))
                    return
                    
                case job := <-h.jobQueue:
                    result, err := h.processJob(ctx, job)
                    if err != nil {
                        errors <- err
                        continue
                    }
                    results <- result
                }
            }
        }(i)
    }
    
    // Wait for completion or timeout
    done := make(chan struct{})
    go func() {
        wg.Wait()
        close(done)
    }()
    
    select {
    case <-done:
        return c.JSON(200, map[string]string{"status": "completed"})
    case <-ctx.Done():
        return c.JSON(408, map[string]string{"error": "request timeout"})
    }
}
```

### Graceful Goroutine Shutdown

```go
// Example 7: Graceful shutdown pattern
type Worker struct {
    logger *zap.Logger
    jobs   chan Job
}

func (w *Worker) Start(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            w.logger.Info("Worker received shutdown signal")
            w.cleanup()
            return
            
        case job := <-w.jobs:
            // Create job-specific timeout
            jobCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
            
            err := w.processJob(jobCtx, job)
            cancel() // Clean up job context
            
            if err != nil {
                if errors.Is(err, context.DeadlineExceeded) {
                    w.logger.Warn("Job timeout", zap.String("job_id", job.ID))
                } else {
                    w.logger.Error("Job failed", zap.Error(err))
                }
            }
        }
    }
}
```

## Timeout Best Practices

### Setting Appropriate Timeouts

Different operations require different timeout strategies:

```go
// Example 8: Timeout strategies
func (h *TimeoutHandler) DemoTimeouts(c httpctx.Context) error {
    parentCtx := c.Request().Context()
    
    // Fast cache lookup - short timeout
    cacheCtx, cancel := context.WithTimeout(parentCtx, 50*time.Millisecond)
    defer cancel()
    
    if cached, err := h.cache.Get(cacheCtx, "key"); err == nil {
        return c.JSON(200, cached)
    }
    
    // Database query - medium timeout
    dbCtx, cancel := context.WithTimeout(parentCtx, 2*time.Second)
    defer cancel()
    
    data, err := h.db.Query(dbCtx, "SELECT ...")
    if err != nil {
        return c.JSON(500, map[string]string{"error": "Database error"})
    }
    
    // External API call - longer timeout
    apiCtx, cancel := context.WithTimeout(parentCtx, 10*time.Second)
    defer cancel()
    
    enriched, err := h.api.EnrichData(apiCtx, data)
    if err != nil {
        // Don't fail if enrichment fails
        h.logger.Warn("Enrichment failed", zap.Error(err))
        enriched = data
    }
    
    // Async cache update - fire and forget with timeout
    go func() {
        cacheUpdateCtx, cancel := context.WithTimeout(
            context.Background(), // Use background for async operations
            5*time.Second,
        )
        defer cancel()
        
        if err := h.cache.Set(cacheUpdateCtx, "key", enriched); err != nil {
            h.logger.Error("Cache update failed", zap.Error(err))
        }
    }()
    
    return c.JSON(200, enriched)
}
```

### Handling Timeout Errors

```go
// Example 9: Proper timeout error handling
func (h *Handler) HandleTimeouts(ctx context.Context) error {
    result, err := h.service.SlowOperation(ctx)
    if err != nil {
        switch {
        case errors.Is(err, context.DeadlineExceeded):
            return fmt.Errorf("operation timed out after %v", h.timeout)
        case errors.Is(err, context.Canceled):
            return fmt.Errorf("operation was cancelled")
        default:
            return fmt.Errorf("operation failed: %w", err)
        }
    }
    return nil
}
```

## Real-World Example: HTTP Request Tracing

Here's a complete example showing context propagation through an HTTP request with tracing:

```go
// Example 10: Complete request tracing example
package handlers

import (
    "context"
    "database/sql"
    "time"
    
    "github.com/yshengliao/gortex/transport/http"
    "github.com/yshengliao/gortex/observability/tracing"
    "go.uber.org/zap"
)

type OrderHandler struct {
    db       *sql.DB
    logger   *zap.Logger
    tracer   tracing.EnhancedTracer
    cache    Cache
    payment  PaymentService
    shipping ShippingService
}

func (h *OrderHandler) CreateOrder(c http.Context) error {
    // Start with request context
    ctx := c.Request().Context()
    
    // Extract span from context if tracing is enabled
    span := c.Span()
    if span != nil {
        span.AddTags(map[string]interface{}{
            "handler": "CreateOrder",
            "user_id": c.Header("X-User-ID"),
        })
    }
    
    // Parse request
    var req CreateOrderRequest
    if err := c.Bind(&req); err != nil {
        if span != nil {
            span.SetError(err)
        }
        return c.JSON(400, map[string]string{"error": "Invalid request"})
    }
    
    // Validate with timeout
    validateCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
    defer cancel()
    
    if err := h.validateOrder(validateCtx, &req); err != nil {
        if span != nil {
            span.LogEvent(tracing.WARN, "Validation failed", map[string]any{
                "error": err.Error(),
            })
        }
        return c.JSON(400, map[string]string{"error": err.Error()})
    }
    
    // Begin transaction with context
    tx, err := h.db.BeginTx(ctx, nil)
    if err != nil {
        if span != nil {
            span.SetError(err)
        }
        return c.JSON(500, map[string]string{"error": "Database error"})
    }
    defer tx.Rollback()
    
    // Create order with timeout
    orderCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
    defer cancel()
    
    order, err := h.createOrderTx(orderCtx, tx, &req)
    if err != nil {
        if span != nil {
            span.SetError(err)
        }
        return c.JSON(500, map[string]string{"error": "Failed to create order"})
    }
    
    // Process payment asynchronously with separate context
    paymentCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()
    
    paymentResult := make(chan PaymentResult, 1)
    go func() {
        result, err := h.payment.Process(paymentCtx, order)
        paymentResult <- PaymentResult{Result: result, Error: err}
    }()
    
    // Wait for payment with timeout
    select {
    case <-ctx.Done():
        // Request cancelled, attempt to cancel payment
        go h.payment.Cancel(context.Background(), order.ID)
        return c.JSON(408, map[string]string{"error": "Request timeout"})
        
    case result := <-paymentResult:
        if result.Error != nil {
            if span != nil {
                span.LogEvent(tracing.ERROR, "Payment failed", map[string]any{
                    "order_id": order.ID,
                    "error":    result.Error.Error(),
                })
            }
            return c.JSON(402, map[string]string{"error": "Payment failed"})
        }
        
        // Update order status
        updateCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
        defer cancel()
        
        if err := h.updateOrderStatus(updateCtx, tx, order.ID, "paid"); err != nil {
            // Log but don't fail - can be retried
            h.logger.Error("Failed to update order status",
                zap.String("order_id", order.ID),
                zap.Error(err))
        }
    }
    
    // Commit transaction
    if err := tx.Commit(); err != nil {
        if span != nil {
            span.SetError(err)
        }
        return c.JSON(500, map[string]string{"error": "Failed to finalize order"})
    }
    
    // Async operations after response
    go func() {
        // Create new context for async operations
        asyncCtx, cancel := context.WithTimeout(
            context.Background(),
            5*time.Minute,
        )
        defer cancel()
        
        // Update cache
        cacheCtx, cacheCancel := context.WithTimeout(asyncCtx, 1*time.Second)
        h.cache.Set(cacheCtx, order.CacheKey(), order)
        cacheCancel()
        
        // Schedule shipping
        shipCtx, shipCancel := context.WithTimeout(asyncCtx, 2*time.Minute)
        h.shipping.Schedule(shipCtx, order)
        shipCancel()
    }()
    
    if span != nil {
        span.LogEvent(tracing.INFO, "Order created successfully", map[string]any{
            "order_id": order.ID,
            "total":    order.Total,
        })
    }
    
    return c.JSON(201, order)
}
```

## Performance Considerations

### Context Value Storage

Avoid storing large values in context:

```go
// ❌ BAD: Storing large objects in context
ctx = context.WithValue(ctx, "user", largeUserObject)

// ✅ GOOD: Store only identifiers
ctx = context.WithValue(ctx, "user_id", userID)
```

### Context Key Best Practices

```go
// Define typed keys to avoid collisions
type contextKey string

const (
    userIDKey     contextKey = "user_id"
    requestIDKey  contextKey = "request_id"
    spanKey       contextKey = "span"
)

// Helper functions for type safety
func WithUserID(ctx context.Context, userID string) context.Context {
    return context.WithValue(ctx, userIDKey, userID)
}

func GetUserID(ctx context.Context) (string, bool) {
    userID, ok := ctx.Value(userIDKey).(string)
    return userID, ok
}
```

## Troubleshooting

### Common Issues and Solutions

1. **Context deadline exceeded errors**
   - Check if timeouts are appropriate for the operation
   - Verify network latency and service response times
   - Consider implementing retry logic with backoff

2. **Goroutine leaks**
   - Always check context cancellation in loops
   - Use `defer cancel()` immediately after creating context
   - Monitor goroutine count in production

3. **Context not propagating**
   - Ensure you're passing the correct context
   - Don't use `context.Background()` in request handlers
   - Check middleware is properly propagating context

### Debugging Context Issues

```go
// Debug helper to trace context cancellation
func DebugContext(ctx context.Context, name string) {
    go func() {
        <-ctx.Done()
        fmt.Printf("Context %s cancelled: %v\n", name, ctx.Err())
    }()
}
```

### Testing Context Handling

```go
func TestContextCancellation(t *testing.T) {
    // Create cancellable context
    ctx, cancel := context.WithCancel(context.Background())
    
    // Start operation
    done := make(chan error, 1)
    go func() {
        done <- longRunningOperation(ctx)
    }()
    
    // Cancel after delay
    time.Sleep(100 * time.Millisecond)
    cancel()
    
    // Verify cancellation was handled
    err := <-done
    assert.ErrorIs(t, err, context.Canceled)
}
```

## Summary

Proper context handling is essential for building robust Gortex applications. Key takeaways:

1. Always propagate context from HTTP requests
2. Set appropriate timeouts for different operations
3. Check for cancellation in long-running operations
4. Clean up resources with `defer cancel()`
5. Don't store context in structs
6. Handle timeout and cancellation errors gracefully

By following these patterns, you'll build more reliable and responsive applications that handle failures gracefully and respect client timeouts.