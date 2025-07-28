package contextutil

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestDoWithContext(t *testing.T) {
	t.Run("successful execution", func(t *testing.T) {
		ctx := context.Background()
		executed := false
		
		err := DoWithContext(ctx, func(ctx context.Context) error {
			executed = true
			return nil
		})
		
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if !executed {
			t.Error("function was not executed")
		}
	})

	t.Run("function returns error", func(t *testing.T) {
		ctx := context.Background()
		expectedErr := errors.New("test error")
		
		err := DoWithContext(ctx, func(ctx context.Context) error {
			return expectedErr
		})
		
		if err != expectedErr {
			t.Errorf("expected error %v, got %v", expectedErr, err)
		}
	})

	t.Run("context cancelled before execution", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately
		
		executed := false
		err := DoWithContext(ctx, func(ctx context.Context) error {
			executed = true
			return nil
		})
		
		if err != context.Canceled {
			t.Errorf("expected context.Canceled, got %v", err)
		}
		if executed {
			t.Error("function should not have been executed")
		}
	})

	t.Run("context cancelled during execution", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		
		err := DoWithContext(ctx, func(ctx context.Context) error {
			// Cancel context during execution
			go func() {
				time.Sleep(10 * time.Millisecond)
				cancel()
			}()
			
			// Simulate long operation
			time.Sleep(50 * time.Millisecond)
			return nil
		})
		
		if err != context.Canceled {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	})
}

func TestRetryWithContext(t *testing.T) {
	t.Run("succeeds on first attempt", func(t *testing.T) {
		ctx := context.Background()
		attempts := 0
		
		err := RetryWithContext(ctx, func(ctx context.Context) error {
			attempts++
			return nil
		}, DefaultRetryOptions())
		
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if attempts != 1 {
			t.Errorf("expected 1 attempt, got %d", attempts)
		}
	})

	t.Run("succeeds after retries", func(t *testing.T) {
		ctx := context.Background()
		attempts := 0
		
		err := RetryWithContext(ctx, func(ctx context.Context) error {
			attempts++
			if attempts < 3 {
				return errors.New("temporary error")
			}
			return nil
		}, RetryOptions{
			MaxAttempts: 3,
			Delay:       10 * time.Millisecond,
		})
		
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if attempts != 3 {
			t.Errorf("expected 3 attempts, got %d", attempts)
		}
	})

	t.Run("fails after max attempts", func(t *testing.T) {
		ctx := context.Background()
		attempts := 0
		
		err := RetryWithContext(ctx, func(ctx context.Context) error {
			attempts++
			return errors.New("persistent error")
		}, RetryOptions{
			MaxAttempts: 3,
			Delay:       10 * time.Millisecond,
		})
		
		if err == nil {
			t.Error("expected error, got nil")
		}
		if attempts != 3 {
			t.Errorf("expected 3 attempts, got %d", attempts)
		}
	})

	t.Run("context cancelled during retry", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		attempts := 0
		
		// Cancel after first attempt
		go func() {
			time.Sleep(20 * time.Millisecond)
			cancel()
		}()
		
		err := RetryWithContext(ctx, func(ctx context.Context) error {
			attempts++
			return errors.New("error")
		}, RetryOptions{
			MaxAttempts: 5,
			Delay:       30 * time.Millisecond,
		})
		
		if err == nil {
			t.Error("expected error due to cancellation")
		}
		if attempts >= 5 {
			t.Error("should have been cancelled before all attempts")
		}
	})

	t.Run("exponential backoff", func(t *testing.T) {
		ctx := context.Background()
		attempts := 0
		var delays []time.Duration
		lastTime := time.Now()
		
		err := RetryWithContext(ctx, func(ctx context.Context) error {
			attempts++
			if attempts > 1 {
				delays = append(delays, time.Since(lastTime))
			}
			lastTime = time.Now()
			if attempts < 3 {
				return errors.New("error")
			}
			return nil
		}, RetryOptions{
			MaxAttempts: 3,
			Delay:       10 * time.Millisecond,
			BackoffFunc: func(attempt int, delay time.Duration) time.Duration {
				return delay * time.Duration(attempt)
			},
		})
		
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		
		// Verify backoff worked (delays should increase)
		if len(delays) != 2 {
			t.Fatalf("expected 2 delays, got %d", len(delays))
		}
		if delays[1] <= delays[0] {
			t.Error("backoff did not increase delay")
		}
	})
}

func TestParallelWithContext(t *testing.T) {
	t.Run("all functions succeed", func(t *testing.T) {
		ctx := context.Background()
		var completed int32
		
		err := ParallelWithContext(ctx,
			func(ctx context.Context) error {
				atomic.AddInt32(&completed, 1)
				return nil
			},
			func(ctx context.Context) error {
				atomic.AddInt32(&completed, 1)
				return nil
			},
			func(ctx context.Context) error {
				atomic.AddInt32(&completed, 1)
				return nil
			},
		)
		
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if completed != 3 {
			t.Errorf("expected 3 completions, got %d", completed)
		}
	})

	t.Run("one function fails", func(t *testing.T) {
		ctx := context.Background()
		
		err := ParallelWithContext(ctx,
			func(ctx context.Context) error {
				return nil
			},
			func(ctx context.Context) error {
				return errors.New("function 2 error")
			},
			func(ctx context.Context) error {
				time.Sleep(50 * time.Millisecond)
				return nil
			},
		)
		
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("context cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		
		// Cancel after functions start
		go func() {
			time.Sleep(10 * time.Millisecond)
			cancel()
		}()
		
		err := ParallelWithContext(ctx,
			func(ctx context.Context) error {
				time.Sleep(100 * time.Millisecond)
				return nil
			},
			func(ctx context.Context) error {
				time.Sleep(100 * time.Millisecond)
				return nil
			},
		)
		
		if err == nil {
			t.Error("expected error due to cancellation")
		}
	})

	t.Run("empty functions", func(t *testing.T) {
		ctx := context.Background()
		err := ParallelWithContext(ctx)
		
		if err != nil {
			t.Errorf("expected no error for empty functions, got %v", err)
		}
	})

	t.Run("nil functions are skipped", func(t *testing.T) {
		ctx := context.Background()
		executed := false
		
		err := ParallelWithContext(ctx,
			nil,
			func(ctx context.Context) error {
				executed = true
				return nil
			},
			nil,
		)
		
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if !executed {
			t.Error("non-nil function should have been executed")
		}
	})
}

func TestForEachWithContext(t *testing.T) {
	t.Run("processes all items", func(t *testing.T) {
		ctx := context.Background()
		items := []int{1, 2, 3, 4, 5}
		var processed []int
		
		err := ForEachWithContext(ctx, items, func(ctx context.Context, item int) error {
			processed = append(processed, item)
			return nil
		})
		
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if len(processed) != len(items) {
			t.Errorf("expected %d processed items, got %d", len(items), len(processed))
		}
	})

	t.Run("stops on error", func(t *testing.T) {
		ctx := context.Background()
		items := []int{1, 2, 3, 4, 5}
		var processed []int
		
		err := ForEachWithContext(ctx, items, func(ctx context.Context, item int) error {
			processed = append(processed, item)
			if item == 3 {
				return errors.New("error at item 3")
			}
			return nil
		})
		
		if err == nil {
			t.Error("expected error, got nil")
		}
		if len(processed) != 3 {
			t.Errorf("expected 3 processed items before error, got %d", len(processed))
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		items := []int{1, 2, 3, 4, 5}
		var processed []int
		
		err := ForEachWithContext(ctx, items, func(ctx context.Context, item int) error {
			processed = append(processed, item)
			if item == 2 {
				cancel()
			}
			time.Sleep(10 * time.Millisecond)
			return nil
		})
		
		if err == nil {
			t.Error("expected error due to cancellation")
		}
		if len(processed) >= len(items) {
			t.Error("should not have processed all items")
		}
	})
}

func TestTimeoutFunc(t *testing.T) {
	t.Run("completes within timeout", func(t *testing.T) {
		ctx := context.Background()
		fn := TimeoutFunc(100*time.Millisecond, func(ctx context.Context) error {
			time.Sleep(10 * time.Millisecond)
			return nil
		})
		
		err := fn(ctx)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("times out", func(t *testing.T) {
		ctx := context.Background()
		fn := TimeoutFunc(10*time.Millisecond, func(ctx context.Context) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(100 * time.Millisecond):
				return nil
			}
		})
		
		err := fn(ctx)
		if err == nil {
			t.Error("expected timeout error, got nil")
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("expected DeadlineExceeded error, got %v", err)
		}
	})

	t.Run("inherits parent context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		
		fn := TimeoutFunc(100*time.Millisecond, func(ctx context.Context) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(10 * time.Millisecond):
				return nil
			}
		})
		
		err := fn(ctx)
		if err != context.Canceled {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	})
}

func TestRace(t *testing.T) {
	t.Run("first success wins", func(t *testing.T) {
		ctx := context.Background()
		
		err := Race(ctx,
			func(ctx context.Context) error {
				time.Sleep(100 * time.Millisecond)
				return nil
			},
			func(ctx context.Context) error {
				time.Sleep(10 * time.Millisecond)
				return nil // This should win
			},
			func(ctx context.Context) error {
				time.Sleep(50 * time.Millisecond)
				return nil
			},
		)
		
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("all fail", func(t *testing.T) {
		ctx := context.Background()
		
		err := Race(ctx,
			func(ctx context.Context) error {
				return errors.New("error 1")
			},
			func(ctx context.Context) error {
				return errors.New("error 2")
			},
			func(ctx context.Context) error {
				return errors.New("error 3")
			},
		)
		
		if err == nil {
			t.Error("expected error when all functions fail")
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		
		go func() {
			time.Sleep(10 * time.Millisecond)
			cancel()
		}()
		
		err := Race(ctx,
			func(ctx context.Context) error {
				time.Sleep(100 * time.Millisecond)
				return nil
			},
			func(ctx context.Context) error {
				time.Sleep(100 * time.Millisecond)
				return nil
			},
		)
		
		if err != context.Canceled {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	})

	t.Run("no functions provided", func(t *testing.T) {
		ctx := context.Background()
		err := Race(ctx)
		
		if err == nil {
			t.Error("expected error for no functions")
		}
	})

	t.Run("nil functions are skipped", func(t *testing.T) {
		ctx := context.Background()
		
		err := Race(ctx,
			nil,
			func(ctx context.Context) error {
				return nil
			},
			nil,
		)
		
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})
}