// Package contextutil provides helper functions for working with context.Context
package contextutil

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// DoWithContext executes a function with automatic context cancellation handling
func DoWithContext(ctx context.Context, fn func(context.Context) error) error {
	// Check if context is already cancelled
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Create channel for function result
	type result struct {
		err error
	}
	done := make(chan result, 1)

	// Run function in goroutine
	go func() {
		done <- result{err: fn(ctx)}
	}()

	// Wait for function to complete or context to cancel
	select {
	case <-ctx.Done():
		return ctx.Err()
	case res := <-done:
		return res.err
	}
}

// RetryOptions configures retry behavior
type RetryOptions struct {
	MaxAttempts int
	Delay       time.Duration
	BackoffFunc func(attempt int, delay time.Duration) time.Duration
}

// DefaultRetryOptions returns sensible default retry options
func DefaultRetryOptions() RetryOptions {
	return RetryOptions{
		MaxAttempts: 3,
		Delay:       100 * time.Millisecond,
		BackoffFunc: func(attempt int, delay time.Duration) time.Duration {
			// Exponential backoff with jitter
			return delay * time.Duration(attempt)
		},
	}
}

// RetryWithContext executes a function with retry logic and context awareness
func RetryWithContext(ctx context.Context, fn func(context.Context) error, opts RetryOptions) error {
	if opts.MaxAttempts <= 0 {
		opts.MaxAttempts = 1
	}
	if opts.BackoffFunc == nil {
		opts.BackoffFunc = DefaultRetryOptions().BackoffFunc
	}

	var lastErr error
	delay := opts.Delay

	for attempt := 1; attempt <= opts.MaxAttempts; attempt++ {
		// Check context before each attempt
		select {
		case <-ctx.Done():
			if lastErr != nil {
				return fmt.Errorf("context cancelled after %d attempts: %w (last error: %v)", attempt-1, ctx.Err(), lastErr)
			}
			return ctx.Err()
		default:
		}

		// Try the function
		err := fn(ctx)
		if err == nil {
			return nil
		}

		lastErr = err

		// Don't retry if context is cancelled
		if ctx.Err() != nil {
			return fmt.Errorf("context error during attempt %d: %w", attempt, ctx.Err())
		}

		// Don't wait after the last attempt
		if attempt < opts.MaxAttempts {
			// Calculate next delay
			if opts.BackoffFunc != nil {
				delay = opts.BackoffFunc(attempt, opts.Delay)
			}

			// Wait with context awareness
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return fmt.Errorf("context cancelled while waiting for retry: %w", ctx.Err())
			case <-timer.C:
				// Continue to next attempt
			}
		}
	}

	return fmt.Errorf("failed after %d attempts: %w", opts.MaxAttempts, lastErr)
}

// ParallelWithContext executes multiple functions in parallel with context awareness
func ParallelWithContext(ctx context.Context, fns ...func(context.Context) error) error {
	if len(fns) == 0 {
		return nil
	}

	// Create error channel and wait group
	errCh := make(chan error, len(fns))
	var wg sync.WaitGroup

	// Create cancellable context for all operations
	parallelCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Execute all functions
	for i, fn := range fns {
		if fn == nil {
			continue
		}

		wg.Add(1)
		go func(idx int, f func(context.Context) error) {
			defer wg.Done()

			// Check if context is already cancelled
			select {
			case <-parallelCtx.Done():
				errCh <- fmt.Errorf("function %d cancelled before start: %w", idx, parallelCtx.Err())
				return
			default:
			}

			// Execute function
			if err := f(parallelCtx); err != nil {
				// Cancel all other operations on first error
				cancel()
				errCh <- fmt.Errorf("function %d failed: %w", idx, err)
			}
		}(i, fn)
	}

	// Wait for all to complete
	go func() {
		wg.Wait()
		close(errCh)
	}()

	// Collect errors
	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}

	// Return first error if any
	if len(errs) > 0 {
		return errs[0]
	}

	// Check if context was cancelled
	if ctx.Err() != nil {
		return ctx.Err()
	}

	return nil
}

// ForEachWithContext processes items with context awareness and early termination
func ForEachWithContext[T any](ctx context.Context, items []T, fn func(context.Context, T) error) error {
	for i, item := range items {
		// Check context before each iteration
		select {
		case <-ctx.Done():
			return fmt.Errorf("cancelled at item %d: %w", i, ctx.Err())
		default:
		}

		// Process item
		if err := fn(ctx, item); err != nil {
			return fmt.Errorf("error processing item %d: %w", i, err)
		}
	}
	return nil
}

// TimeoutFunc wraps a function with a timeout
func TimeoutFunc(timeout time.Duration, fn func(context.Context) error) func(context.Context) error {
	return func(ctx context.Context) error {
		timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		return fn(timeoutCtx)
	}
}

// Race executes multiple functions and returns when the first one completes successfully
func Race(ctx context.Context, fns ...func(context.Context) error) error {
	if len(fns) == 0 {
		return fmt.Errorf("no functions provided")
	}

	type result struct {
		err   error
		index int
	}

	resultCh := make(chan result, len(fns))
	raceCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Start all functions
	for i, fn := range fns {
		if fn == nil {
			continue
		}

		go func(idx int, f func(context.Context) error) {
			err := f(raceCtx)
			select {
			case resultCh <- result{err: err, index: idx}:
			case <-raceCtx.Done():
				// Context cancelled, don't send result
			}
		}(i, fn)
	}

	// Wait for first result
	select {
	case <-ctx.Done():
		return ctx.Err()
	case res := <-resultCh:
		if res.err == nil {
			// First success, cancel others
			cancel()
			return nil
		}
		// Continue waiting for a successful result
		completed := 1
		for completed < len(fns) {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case res := <-resultCh:
				completed++
				if res.err == nil {
					cancel()
					return nil
				}
			}
		}
		// All failed
		return fmt.Errorf("all functions failed")
	}
}