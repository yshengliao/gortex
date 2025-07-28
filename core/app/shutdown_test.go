package app

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestShutdownHooks(t *testing.T) {
	t.Run("hooks are called during shutdown", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		app, err := NewApp(
			WithLogger(logger),
			WithShutdownTimeout(5*time.Second),
		)
		require.NoError(t, err)

		var called atomic.Int32

		// Register multiple hooks
		app.OnShutdown(func(ctx context.Context) error {
			called.Add(1)
			return nil
		})

		app.RegisterShutdownHook(func(ctx context.Context) error {
			called.Add(1)
			time.Sleep(100 * time.Millisecond) // Simulate work
			return nil
		})

		app.OnShutdown(func(ctx context.Context) error {
			called.Add(1)
			return nil
		})

		// Shutdown
		ctx := context.Background()
		err = app.Shutdown(ctx)
		assert.NoError(t, err)
		assert.Equal(t, int32(3), called.Load())
	})

	t.Run("shutdown timeout is respected", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		app, err := NewApp(
			WithLogger(logger),
			WithShutdownTimeout(100*time.Millisecond),
		)
		require.NoError(t, err)

		// Register a slow hook
		app.OnShutdown(func(ctx context.Context) error {
			select {
			case <-time.After(1 * time.Second):
				return errors.New("should not reach here")
			case <-ctx.Done():
				return ctx.Err()
			}
		})

		// Shutdown with no deadline in context
		ctx := context.Background()
		start := time.Now()
		err = app.Shutdown(ctx)
		duration := time.Since(start)

		// Should timeout after configured duration
		assert.Error(t, err)
		assert.Less(t, duration, 200*time.Millisecond)
	})

	t.Run("context deadline overrides default timeout", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		app, err := NewApp(
			WithLogger(logger),
			WithShutdownTimeout(5*time.Second), // Long default timeout
		)
		require.NoError(t, err)

		// Register a hook that waits for context
		app.OnShutdown(func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		})

		// Shutdown with short deadline
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		start := time.Now()
		err = app.Shutdown(ctx)
		duration := time.Since(start)

		// Should use the context deadline, not default timeout
		assert.Error(t, err)
		assert.Less(t, duration, 100*time.Millisecond)
	})

	t.Run("hooks run in parallel", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		app, err := NewApp(
			WithLogger(logger),
			WithShutdownTimeout(1*time.Second),
		)
		require.NoError(t, err)

		var mu sync.Mutex
		var order []int

		// Register hooks that should run in parallel
		for i := 0; i < 3; i++ {
			idx := i
			app.OnShutdown(func(ctx context.Context) error {
				time.Sleep(50 * time.Millisecond) // Simulate work

				mu.Lock()
				order = append(order, idx)
				mu.Unlock()

				return nil
			})
		}

		// Shutdown
		start := time.Now()
		err = app.Shutdown(context.Background())
		duration := time.Since(start)

		assert.NoError(t, err)
		assert.Len(t, order, 3)
		// If they ran in parallel, total time should be ~50ms, not 150ms
		assert.Less(t, duration, 100*time.Millisecond)
	})

	t.Run("hook errors are collected", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		app, err := NewApp(
			WithLogger(logger),
		)
		require.NoError(t, err)

		// Register hooks with some failures
		app.OnShutdown(func(ctx context.Context) error {
			return errors.New("hook 1 failed")
		})

		app.OnShutdown(func(ctx context.Context) error {
			return nil // This one succeeds
		})

		app.OnShutdown(func(ctx context.Context) error {
			return errors.New("hook 3 failed")
		})

		// Shutdown
		err = app.Shutdown(context.Background())
		// Errors from hooks are returned
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "shutdown hooks failed")
		assert.Contains(t, err.Error(), "hook 1 failed")
		assert.Contains(t, err.Error(), "hook 3 failed")
	})

	t.Run("thread safety", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		app, err := NewApp(
			WithLogger(logger),
		)
		require.NoError(t, err)

		// Concurrently register hooks
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				app.OnShutdown(func(ctx context.Context) error {
					return nil
				})
			}()
		}
		wg.Wait()

		// Shutdown should work correctly
		err = app.Shutdown(context.Background())
		assert.NoError(t, err)
	})
}

func TestWithShutdownTimeout(t *testing.T) {
	t.Run("valid timeout", func(t *testing.T) {
		app, err := NewApp(
			WithShutdownTimeout(30 * time.Second),
		)
		require.NoError(t, err)
		assert.Equal(t, 30*time.Second, app.shutdownTimeout)
	})

	t.Run("invalid timeout", func(t *testing.T) {
		_, err := NewApp(
			WithShutdownTimeout(0),
		)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "shutdown timeout must be positive")

		_, err = NewApp(
			WithShutdownTimeout(-1 * time.Second),
		)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "shutdown timeout must be positive")
	})

	t.Run("default timeout", func(t *testing.T) {
		app, err := NewApp()
		require.NoError(t, err)
		assert.Equal(t, 30*time.Second, app.shutdownTimeout)
	})
}
