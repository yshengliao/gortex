package observability

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestHealthCheckerRaceConditions tests for race conditions in HealthChecker
func TestHealthCheckerRaceConditions(t *testing.T) {
	t.Run("ConcurrentRegisterAndCheck", func(t *testing.T) {
		checker := NewHealthChecker(50*time.Millisecond, 5*time.Second)
		defer checker.Stop()

		var wg sync.WaitGroup
		
		// Concurrently register health checks
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				name := string(rune('A' + id))
				checker.Register(name, func(ctx context.Context) HealthCheckResult {
					return HealthCheckResult{
						Status:  HealthStatusHealthy,
						Message: "Test check",
					}
				})
			}(i)
		}

		// Concurrently perform checks
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				checker.Check(context.Background())
			}()
		}

		// Concurrently get results
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				checker.GetResults()
				checker.GetOverallStatus()
			}()
		}

		wg.Wait()
		
		// Wait a bit for background processing
		time.Sleep(100 * time.Millisecond)
		
		// Verify checks are registered
		results := checker.GetResults()
		assert.Greater(t, len(results), 0)
	})

	t.Run("BackgroundCheckRaceCondition", func(t *testing.T) {
		checker := NewHealthChecker(50*time.Millisecond, 5*time.Second)
		defer checker.Stop()

		// Use atomic counter to avoid race condition
		var updateCount int64
		
		checker.Register("background", func(ctx context.Context) HealthCheckResult {
			atomic.AddInt64(&updateCount, 1)
			return HealthCheckResult{
				Status: HealthStatusHealthy,
			}
		})

		// Wait for background checks to run
		time.Sleep(250 * time.Millisecond)
		
		// Read with atomic operation
		count := atomic.LoadInt64(&updateCount)
		assert.GreaterOrEqual(t, count, int64(2))
	})

	t.Run("ConcurrentUnregister", func(t *testing.T) {
		checker := NewHealthChecker(100*time.Millisecond, 5*time.Second)
		defer checker.Stop()

		// Register multiple checks
		for i := 0; i < 20; i++ {
			name := string(rune('A' + i))
			checker.Register(name, func(ctx context.Context) HealthCheckResult {
				return HealthCheckResult{Status: HealthStatusHealthy}
			})
		}

		var wg sync.WaitGroup

		// Concurrently unregister
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				name := string(rune('A' + id))
				checker.Unregister(name)
			}(i)
		}

		// Concurrently check
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				checker.Check(context.Background())
			}()
		}

		wg.Wait()

		// Verify remaining checks
		results := checker.GetResults()
		assert.GreaterOrEqual(t, len(results), 10)
		assert.LessOrEqual(t, len(results), 20)
	})

	t.Run("StopDuringCheck", func(t *testing.T) {
		checker := NewHealthChecker(100*time.Millisecond, 5*time.Second)

		// Register a slow check
		checker.Register("slow", func(ctx context.Context) HealthCheckResult {
			select {
			case <-time.After(200 * time.Millisecond):
				return HealthCheckResult{Status: HealthStatusHealthy}
			case <-ctx.Done():
				return HealthCheckResult{
					Status:  HealthStatusUnhealthy,
					Message: "Check cancelled",
				}
			}
		})

		// Start a check in background
		go func() {
			checker.Check(context.Background())
		}()

		// Stop the checker while check is running
		time.Sleep(50 * time.Millisecond)
		checker.Stop()

		// Should not panic or deadlock
		time.Sleep(100 * time.Millisecond)
	})
}

// TestHealthCheckerStress performs stress testing for race conditions
func TestHealthCheckerStress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	checker := NewHealthChecker(10*time.Millisecond, 1*time.Second)
	defer checker.Stop()

	var wg sync.WaitGroup
	stopCh := make(chan struct{})

	// Continuously register/unregister checks
	wg.Add(1)
	go func() {
		defer wg.Done()
		counter := 0
		for {
			select {
			case <-stopCh:
				return
			default:
				name := string(rune('A' + (counter % 26)))
				if counter%2 == 0 {
					checker.Register(name, func(ctx context.Context) HealthCheckResult {
						return HealthCheckResult{Status: HealthStatusHealthy}
					})
				} else {
					checker.Unregister(name)
				}
				counter++
				time.Sleep(time.Millisecond)
			}
		}
	}()

	// Continuously perform checks
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stopCh:
				return
			default:
				checker.Check(context.Background())
				time.Sleep(5 * time.Millisecond)
			}
		}
	}()

	// Continuously get results
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stopCh:
				return
			default:
				checker.GetResults()
				checker.GetOverallStatus()
				time.Sleep(3 * time.Millisecond)
			}
		}
	}()

	// Run for 1 second
	time.Sleep(1 * time.Second)
	close(stopCh)
	wg.Wait()
}