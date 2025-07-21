package observability_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yshengliao/gortex/observability"
)

func TestHealthChecker(t *testing.T) {
	// Create health checker with short intervals for testing
	checker := observability.NewHealthChecker(100*time.Millisecond, 50*time.Millisecond)
	defer checker.Stop()

	t.Run("RegisterAndCheck", func(t *testing.T) {
		// Register a healthy check
		checker.Register("database", func(ctx context.Context) observability.HealthCheckResult {
			return observability.HealthCheckResult{
				Status:  observability.HealthStatusHealthy,
				Message: "Database is healthy",
			}
		})

		// Register a degraded check
		checker.Register("cache", func(ctx context.Context) observability.HealthCheckResult {
			return observability.HealthCheckResult{
				Status:  observability.HealthStatusDegraded,
				Message: "Cache is slow",
				Details: map[string]interface{}{
					"latency_ms": 500,
				},
			}
		})

		// Perform health check
		ctx := context.Background()
		results := checker.Check(ctx)

		assert.Len(t, results, 2)
		assert.Equal(t, observability.HealthStatusHealthy, results["database"].Status)
		assert.Equal(t, observability.HealthStatusDegraded, results["cache"].Status)
	})

	t.Run("GetOverallStatus", func(t *testing.T) {
		// Clear existing checks
		checker.Unregister("database")
		checker.Unregister("cache")

		// All healthy
		checker.Register("service1", func(ctx context.Context) observability.HealthCheckResult {
			return observability.HealthCheckResult{Status: observability.HealthStatusHealthy}
		})
		checker.Register("service2", func(ctx context.Context) observability.HealthCheckResult {
			return observability.HealthCheckResult{Status: observability.HealthStatusHealthy}
		})

		checker.Check(context.Background())
		assert.Equal(t, observability.HealthStatusHealthy, checker.GetOverallStatus())

		// One degraded
		checker.Register("service3", func(ctx context.Context) observability.HealthCheckResult {
			return observability.HealthCheckResult{Status: observability.HealthStatusDegraded}
		})

		checker.Check(context.Background())
		assert.Equal(t, observability.HealthStatusDegraded, checker.GetOverallStatus())

		// One unhealthy
		checker.Register("service4", func(ctx context.Context) observability.HealthCheckResult {
			return observability.HealthCheckResult{Status: observability.HealthStatusUnhealthy}
		})

		checker.Check(context.Background())
		assert.Equal(t, observability.HealthStatusUnhealthy, checker.GetOverallStatus())
	})

	t.Run("Timeout", func(t *testing.T) {
		checker.Register("slow-check", func(ctx context.Context) observability.HealthCheckResult {
			select {
			case <-time.After(200 * time.Millisecond):
				return observability.HealthCheckResult{
					Status:  observability.HealthStatusHealthy,
					Message: "Should not reach here",
				}
			case <-ctx.Done():
				return observability.HealthCheckResult{
					Status:  observability.HealthStatusUnhealthy,
					Message: "Check timed out",
				}
			}
		})

		results := checker.Check(context.Background())
		// The check should timeout since we set timeout to 50ms
		assert.Equal(t, observability.HealthStatusUnhealthy, results["slow-check"].Status)
	})

	t.Run("GetCachedResults", func(t *testing.T) {
		// Clear and register new check
		checker.Unregister("slow-check")
		counter := 0
		checker.Register("counter", func(ctx context.Context) observability.HealthCheckResult {
			counter++
			return observability.HealthCheckResult{
				Status:  observability.HealthStatusHealthy,
				Message: "Check executed",
				Details: map[string]interface{}{
					"count": counter,
				},
			}
		})

		// First check
		checker.Check(context.Background())
		results1 := checker.GetResults()
		
		// Get cached results (should not increment counter)
		results2 := checker.GetResults()
		
		assert.Equal(t, results1["counter"].Details["count"], results2["counter"].Details["count"])
	})

	t.Run("BackgroundChecks", func(t *testing.T) {
		var updateCount int32
		checker.Register("background", func(ctx context.Context) observability.HealthCheckResult {
			atomic.AddInt32(&updateCount, 1)
			return observability.HealthCheckResult{
				Status: observability.HealthStatusHealthy,
			}
		})

		// Wait for background checks to run
		time.Sleep(250 * time.Millisecond)
		
		// Should have run at least 2 times (initial + periodic)
		count := atomic.LoadInt32(&updateCount)
		assert.GreaterOrEqual(t, count, int32(2))
	})
}

func TestDatabaseHealthCheck(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		check := observability.DatabaseHealthCheck(func(ctx context.Context) error {
			return nil
		})

		result := check(context.Background())
		assert.Equal(t, observability.HealthStatusHealthy, result.Status)
		assert.Contains(t, result.Message, "successful")
	})

	t.Run("Failure", func(t *testing.T) {
		testErr := errors.New("connection refused")
		check := observability.DatabaseHealthCheck(func(ctx context.Context) error {
			return testErr
		})

		result := check(context.Background())
		assert.Equal(t, observability.HealthStatusUnhealthy, result.Status)
		assert.Contains(t, result.Message, "failed")
		require.NotNil(t, result.Details)
		assert.Equal(t, testErr.Error(), result.Details["error"])
	})
}

func TestHTTPHealthCheck(t *testing.T) {
	// Create a test server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()
	
	t.Run("Success", func(t *testing.T) {
		check := observability.HTTPHealthCheck(ts.URL+"/health", 200)
		result := check(context.Background())
		
		assert.Equal(t, observability.HealthStatusHealthy, result.Status)
		assert.Equal(t, ts.URL+"/health", result.Details["url"])
		assert.Equal(t, 200, result.Details["status"])
	})
	
	t.Run("Wrong Status", func(t *testing.T) {
		check := observability.HTTPHealthCheck(ts.URL+"/notfound", 200)
		result := check(context.Background())
		
		assert.Equal(t, observability.HealthStatusUnhealthy, result.Status)
		assert.Equal(t, 404, result.Details["actual_status"])
	})
	
	t.Run("Invalid URL", func(t *testing.T) {
		check := observability.HTTPHealthCheck("http://invalid-domain-that-does-not-exist.com", 200)
		result := check(context.Background())
		
		assert.Equal(t, observability.HealthStatusUnhealthy, result.Status)
		assert.Contains(t, result.Message, "request failed")
	})
}

func TestMemoryHealthCheck(t *testing.T) {
	check := observability.MemoryHealthCheck(1024)
	result := check(context.Background())
	
	// Since this is a placeholder implementation, just verify it returns healthy
	assert.Equal(t, observability.HealthStatusHealthy, result.Status)
	assert.Equal(t, uint64(1024), result.Details["max_memory_mb"])
}