package observability

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestSafeHealthChecker tests the thread-safe implementation
func TestSafeHealthChecker(t *testing.T) {
	t.Run("BasicFunctionality", func(t *testing.T) {
		checker := NewSafeHealthChecker(100*time.Millisecond, 50*time.Millisecond)
		defer checker.Stop()

		// Register checks
		checker.Register("test1", func(ctx context.Context) HealthCheckResult {
			return HealthCheckResult{
				Status:  HealthStatusHealthy,
				Message: "Test 1 OK",
			}
		})

		checker.Register("test2", func(ctx context.Context) HealthCheckResult {
			return HealthCheckResult{
				Status:  HealthStatusDegraded,
				Message: "Test 2 Degraded",
			}
		})

		// Perform check
		results := checker.Check(context.Background())
		assert.Len(t, results, 2)
		assert.Equal(t, HealthStatusHealthy, results["test1"].Status)
		assert.Equal(t, HealthStatusDegraded, results["test2"].Status)

		// Check overall status
		status := checker.GetOverallStatus()
		assert.Equal(t, HealthStatusDegraded, status)
	})

	t.Run("ConcurrentOperations", func(t *testing.T) {
		checker := NewSafeHealthChecker(50*time.Millisecond, 1*time.Second)
		defer checker.Stop()

		var wg sync.WaitGroup
		numGoroutines := 100

		// Concurrent registrations
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				name := string(rune('A' + (id % 26)))
				checker.Register(name, func(ctx context.Context) HealthCheckResult {
					return HealthCheckResult{
						Status: HealthStatusHealthy,
						Details: map[string]interface{}{
							"id": id,
						},
					}
				})
			}(i)
		}

		// Concurrent checks
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				checker.Check(context.Background())
			}()
		}

		// Concurrent result retrievals
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				checker.GetResults()
				checker.GetOverallStatus()
			}()
		}

		wg.Wait()

		// Verify some checks exist
		results := checker.GetResults()
		assert.Greater(t, len(results), 0)
	})

	t.Run("BackgroundChecksThreadSafe", func(t *testing.T) {
		checker := NewSafeHealthChecker(50*time.Millisecond, 1*time.Second)
		defer checker.Stop()

		var counter int64

		checker.Register("counter", func(ctx context.Context) HealthCheckResult {
			atomic.AddInt64(&counter, 1)
			return HealthCheckResult{
				Status: HealthStatusHealthy,
				Details: map[string]interface{}{
					"count": atomic.LoadInt64(&counter),
				},
			}
		})

		// Wait for background checks
		time.Sleep(250 * time.Millisecond)

		count := atomic.LoadInt64(&counter)
		assert.GreaterOrEqual(t, count, int64(2))
	})

	t.Run("StopSafety", func(t *testing.T) {
		checker := NewSafeHealthChecker(50*time.Millisecond, 1*time.Second)

		// Register a check
		checker.Register("test", func(ctx context.Context) HealthCheckResult {
			return HealthCheckResult{Status: HealthStatusHealthy}
		})

		// Multiple stops should not panic
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				checker.Stop()
			}()
		}
		wg.Wait()

		// Operations after stop should not panic
		checker.Register("after-stop", func(ctx context.Context) HealthCheckResult {
			return HealthCheckResult{Status: HealthStatusHealthy}
		})
		
		results := checker.Check(context.Background())
		assert.Empty(t, results)
	})

	t.Run("ConcurrentUnregister", func(t *testing.T) {
		checker := NewSafeHealthChecker(100*time.Millisecond, 1*time.Second)
		defer checker.Stop()

		// Register many checks
		for i := 0; i < 100; i++ {
			name := string(rune('A' + (i % 26))) + string(rune('0' + (i / 26)))
			checker.Register(name, func(ctx context.Context) HealthCheckResult {
				return HealthCheckResult{Status: HealthStatusHealthy}
			})
		}

		var wg sync.WaitGroup

		// Concurrent unregister
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				name := string(rune('A' + (id % 26))) + string(rune('0' + (id / 26)))
				checker.Unregister(name)
			}(i)
		}

		// Concurrent check during unregister
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				checker.Check(context.Background())
			}()
		}

		wg.Wait()

		// Should have some checks remaining
		results := checker.GetResults()
		assert.Greater(t, len(results), 0)
		assert.Less(t, len(results), 100)
	})
}

// TestSafeHealthCheckerRaceDetection runs with race detector
func TestSafeHealthCheckerRaceDetection(t *testing.T) {
	// This test is specifically designed to trigger race conditions if they exist
	checker := NewSafeHealthChecker(10*time.Millisecond, 100*time.Millisecond)
	defer checker.Stop()

	done := make(chan struct{})
	var wg sync.WaitGroup

	// Goroutine 1: Continuously register/unregister
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			select {
			case <-done:
				return
			default:
				name := "check" + string(rune(i%10))
				if i%2 == 0 {
					checker.Register(name, func(ctx context.Context) HealthCheckResult {
						return HealthCheckResult{Status: HealthStatusHealthy}
					})
				} else {
					checker.Unregister(name)
				}
			}
		}
	}()

	// Goroutine 2: Continuously check
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			select {
			case <-done:
				return
			default:
				checker.Check(context.Background())
			}
		}
	}()

	// Goroutine 3: Continuously get results
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			select {
			case <-done:
				return
			default:
				checker.GetResults()
				checker.GetOverallStatus()
			}
		}
	}()

	// Let it run for a bit
	time.Sleep(100 * time.Millisecond)
	close(done)
	wg.Wait()
}