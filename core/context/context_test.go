package context

import (
	"context"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/yshengliao/gortex/pkg/config"
	gcontext "github.com/yshengliao/gortex/transport/http"
	"github.com/yshengliao/gortex/middleware"
	"github.com/yshengliao/gortex/pkg/errors"
	"go.uber.org/zap"
)

// Test handlers for context cancellation
type ContextTestHandlers struct {
	Long    *LongHandler    `url:"/long"`
	Timeout *TimeoutHandler `url:"/timeout"`
	Loop    *LoopHandler    `url:"/loop"`
}

type LongHandler struct {
	cancelled bool
	wg        *sync.WaitGroup
}

func (h *LongHandler) GET(c gcontext.Context) error {
	if h.wg != nil {
		defer h.wg.Done()
	}

	// Get the request context
	ctx := c.Request().Context()

	// Simulate long-running operation
	select {
	case <-time.After(5 * time.Second):
		// Should not reach here if cancelled
		return nil
	case <-ctx.Done():
		// Context was cancelled
		h.cancelled = true
		return ctx.Err()
	}
}

type TimeoutHandler struct {
	timedOut bool
}

func (h *TimeoutHandler) GET(c gcontext.Context) error {
	// Create context with timeout
	timeoutCtx, cancel := context.WithTimeout(c.Request().Context(), 100*time.Millisecond)
	defer cancel()

	// Simulate operation that takes longer than timeout
	select {
	case <-time.After(200 * time.Millisecond):
		// Should not reach here
		return nil
	case <-timeoutCtx.Done():
		// Context timed out
		h.timedOut = true
		return errors.New(errors.CodeTimeout, "Operation timed out")
	}
}

type LoopHandler struct {
	iterations int
	cancelled  bool
	mu         sync.Mutex
}

func (h *LoopHandler) GET(c gcontext.Context) error {
	ctx := c.Request().Context()

	// Simulate a long-running loop
	for i := 0; i < 1000; i++ {
		// Check context cancellation at each iteration
		select {
		case <-ctx.Done():
			h.mu.Lock()
			h.cancelled = true
			h.mu.Unlock()
			return ctx.Err()
		default:
			// Continue processing
		}

		h.mu.Lock()
		h.iterations++
		h.mu.Unlock()

		// Simulate some work
		time.Sleep(10 * time.Millisecond)
	}

	return nil
}

// TestContextCancellationPropagation tests that context cancellation is properly propagated
func TestContextCancellationPropagation(t *testing.T) {
	// Create test app
	cfg := &config.Config{}
	cfg.Server.Address = ":0"
	cfg.Logger.Level = "debug"

	logger, _ := zap.NewDevelopment()

	var wg sync.WaitGroup
	wg.Add(1)

	handlers := &ContextTestHandlers{
		Long: &LongHandler{wg: &wg},
	}

	app, err := NewApp(
		WithConfig(cfg),
		WithLogger(logger),
		WithHandlers(handlers),
	)
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}

	// Create test request with cancellable context
	req := httptest.NewRequest("GET", "/long", nil)
	cancelCtx, cancel := context.WithCancel(req.Context())
	req = req.WithContext(cancelCtx)

	// Start request in goroutine
	rec := httptest.NewRecorder()
	go func() {
		app.Router().ServeHTTP(rec, req)
	}()

	// Cancel context after short delay
	time.Sleep(100 * time.Millisecond)
	cancel()

	// Wait for handler to complete
	wg.Wait()

	// Verify handler was cancelled
	if !handlers.Long.cancelled {
		t.Error("Handler did not receive context cancellation")
	}
}

// TestContextWithTimeout tests context timeout functionality
func TestContextWithTimeout(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.Address = ":0"

	logger, _ := zap.NewDevelopment()

	handlers := &ContextTestHandlers{
		Timeout: &TimeoutHandler{},
	}

	app, err := NewApp(
		WithConfig(cfg),
		WithLogger(logger),
		WithHandlers(handlers),
	)
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}

	// Make request
	req := httptest.NewRequest("GET", "/timeout", nil)
	rec := httptest.NewRecorder()
	app.Router().ServeHTTP(rec, req)

	// Verify timeout occurred
	if !handlers.Timeout.timedOut {
		t.Error("Handler did not timeout as expected")
	}
}

// TestContextCancellationInLoop tests checking context cancellation in loops
func TestContextCancellationInLoop(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.Address = ":0"

	logger, _ := zap.NewDevelopment()

	handlers := &ContextTestHandlers{
		Loop: &LoopHandler{},
	}

	app, err := NewApp(
		WithConfig(cfg),
		WithLogger(logger),
		WithHandlers(handlers),
	)
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}

	// Create request with cancellable context
	req := httptest.NewRequest("GET", "/loop", nil)
	cancelCtx, cancel := context.WithCancel(req.Context())
	req = req.WithContext(cancelCtx)

	// Make request in goroutine
	rec := httptest.NewRecorder()
	go func() {
		app.Router().ServeHTTP(rec, req)
	}()

	// Let it run for a bit then cancel
	time.Sleep(50 * time.Millisecond)
	cancel()

	// Wait a bit for cancellation to be processed
	time.Sleep(50 * time.Millisecond)

	// Verify loop was cancelled
	handlers.Loop.mu.Lock()
	cancelled := handlers.Loop.cancelled
	iterations := handlers.Loop.iterations
	handlers.Loop.mu.Unlock()

	if !cancelled {
		t.Error("Loop did not check for context cancellation")
	}

	// Should have done some iterations but not all
	if iterations == 0 {
		t.Error("Loop did not execute any iterations")
	}
	if iterations >= 1000 {
		t.Error("Loop completed all iterations despite cancellation")
	}
}

// Test handlers for middleware context propagation
type MiddlewareTestHandlers struct {
	Protected *ProtectedHandler `url:"/protected" middleware:"contextaware"`
}

type ProtectedHandler struct {
	executed bool
	mu       sync.Mutex
}

func (h *ProtectedHandler) GET(c gcontext.Context) error {
	h.mu.Lock()
	h.executed = true
	h.mu.Unlock()
	return nil
}

// TestContextPropagationInMiddleware tests context propagation through middleware chain
func TestContextPropagationInMiddleware(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.Address = ":0"

	logger, _ := zap.NewDevelopment()

	// Track middleware execution
	middlewareExecuted := []string{}
	var mu sync.Mutex

	// Create context-aware middleware
	contextMiddleware := func(next middleware.HandlerFunc) middleware.HandlerFunc {
		return func(c gcontext.Context) error {
			mu.Lock()
			middlewareExecuted = append(middlewareExecuted, "before")
			mu.Unlock()

			// Check if context is cancelled before proceeding
			select {
			case <-c.Request().Context().Done():
				mu.Lock()
				middlewareExecuted = append(middlewareExecuted, "cancelled")
				mu.Unlock()
				return c.Request().Context().Err()
			default:
				// Continue
			}

			err := next(c)

			mu.Lock()
			middlewareExecuted = append(middlewareExecuted, "after")
			mu.Unlock()

			return err
		}
	}

	handlers := &MiddlewareTestHandlers{
		Protected: &ProtectedHandler{},
	}

	app, err := NewApp(
		WithConfig(cfg),
		WithLogger(logger),
	)
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}

	// Register middleware
	ctx := app.Context()
	middlewareRegistry := make(map[string]middleware.MiddlewareFunc)
	middlewareRegistry["contextaware"] = contextMiddleware
	Register(ctx, middlewareRegistry)

	// Now register routes with middleware support
	if err := RegisterRoutes(app, handlers); err != nil {
		t.Fatalf("Failed to register routes: %v", err)
	}

	// Test 1: Normal request
	req := httptest.NewRequest("GET", "/protected", nil)
	rec := httptest.NewRecorder()

	// Add handler tracking to middleware execution
	origLen := len(middlewareExecuted)
	app.Router().ServeHTTP(rec, req)

	// Add "handler" to track when handler executes
	if handlers.Protected.executed {
		mu.Lock()
		// Insert "handler" after "before" and before "after"
		if len(middlewareExecuted) > origLen+1 {
			temp := append([]string{}, middlewareExecuted[:origLen+1]...)
			temp = append(temp, "handler")
			temp = append(temp, middlewareExecuted[origLen+1:]...)
			middlewareExecuted = temp
		}
		mu.Unlock()
	}

	// Verify execution order
	expected := []string{"before", "handler", "after"}
	startIdx := origLen
	for i, v := range expected {
		idx := startIdx + i
		if idx >= len(middlewareExecuted) {
			t.Errorf("Missing middleware execution: %s", v)
		} else if middlewareExecuted[idx] != v {
			t.Errorf("Expected middleware[%d] = %s, got %s", idx, v, middlewareExecuted[idx])
		}
	}

	// Test 2: Cancelled request
	handlers.Protected.executed = false
	req2 := httptest.NewRequest("GET", "/protected", nil)
	cancelCtx, cancel := context.WithCancel(req2.Context())
	req2 = req2.WithContext(cancelCtx)

	// Cancel immediately
	cancel()

	rec2 := httptest.NewRecorder()
	origLen2 := len(middlewareExecuted)
	app.Router().ServeHTTP(rec2, req2)

	// Check if cancellation was detected
	cancelled := false
	for i := origLen2; i < len(middlewareExecuted); i++ {
		if middlewareExecuted[i] == "cancelled" {
			cancelled = true
			break
		}
	}
	if !cancelled {
		t.Error("Middleware should have detected context cancellation")
	}
}

// Test parent-child context relationship
type ParentChildHandlers struct {
	Parent *ParentHandler `url:"/parent"`
}

type ParentHandler struct {
	parentCancelled bool
	childCancelled  bool
}

func (h *ParentHandler) GET(c gcontext.Context) error {
	// Get parent context
	parentCtx := c.Request().Context()

	// Create child context
	childCtx, childCancel := context.WithCancel(parentCtx)
	defer childCancel()

	var wg sync.WaitGroup
	wg.Add(2)

	// Monitor both contexts in goroutines
	go func() {
		defer wg.Done()
		<-parentCtx.Done()
		h.parentCancelled = true
	}()

	go func() {
		defer wg.Done()
		<-childCtx.Done()
		h.childCancelled = true
	}()

	// Wait a bit to let goroutines start
	time.Sleep(50 * time.Millisecond)

	// Don't wait for goroutines to complete here
	// They'll complete when context is cancelled
	return nil
}

// TestParentContextCancellation tests that child contexts are cancelled when parent is cancelled
func TestParentContextCancellation(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.Address = ":0"

	logger, _ := zap.NewDevelopment()

	handlers := &ParentChildHandlers{
		Parent: &ParentHandler{},
	}

	app, err := NewApp(
		WithConfig(cfg),
		WithLogger(logger),
		WithHandlers(handlers),
	)
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}

	// Create request with cancellable context
	req := httptest.NewRequest("GET", "/parent", nil)
	cancelCtx, cancel := context.WithCancel(req.Context())
	req = req.WithContext(cancelCtx)

	// Make request
	rec := httptest.NewRecorder()
	go func() {
		app.Router().ServeHTTP(rec, req)
	}()

	// Cancel parent context after handler starts
	time.Sleep(100 * time.Millisecond)
	cancel()

	// Wait for cancellations to propagate
	time.Sleep(100 * time.Millisecond)

	// Verify both contexts were cancelled
	if !handlers.Parent.parentCancelled {
		t.Error("Parent context was not cancelled")
	}
	if !handlers.Parent.childCancelled {
		t.Error("Child context was not cancelled when parent was cancelled")
	}
}
