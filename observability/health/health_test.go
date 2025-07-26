package health_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yshengliao/gortex/observability/health"
)

func TestHealthChecker(t *testing.T) {
	// Create health checker with short intervals for testing
	checker := health.NewHealthChecker(100*time.Millisecond, 50*time.Millisecond)
	defer checker.Stop()

	t.Run("RegisterAndCheck", func(t *testing.T) {
		// Register a healthy check
		checker.Register("database", func(ctx context.Context) health.HealthCheckResult {
			return health.HealthCheckResult{
				Status:  health.HealthStatusHealthy,
				Message: "Database is healthy",
			}
		})

		// Register a degraded check
		checker.Register("cache", func(ctx context.Context) health.HealthCheckResult {
			return health.HealthCheckResult{
				Status:  health.HealthStatusDegraded,
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
		assert.Equal(t, health.HealthStatusHealthy, results["database"].Status)
		assert.Equal(t, health.HealthStatusDegraded, results["cache"].Status)
	})

	t.Run("GetOverallStatus", func(t *testing.T) {
		// Clear existing checks
		checker.Unregister("database")
		checker.Unregister("cache")

		// All healthy
		checker.Register("service1", func(ctx context.Context) health.HealthCheckResult {
			return health.HealthCheckResult{Status: health.HealthStatusHealthy}
		})
		checker.Register("service2", func(ctx context.Context) health.HealthCheckResult {
			return health.HealthCheckResult{Status: health.HealthStatusHealthy}
		})

		checker.Check(context.Background())
		assert.Equal(t, health.HealthStatusHealthy, checker.GetOverallStatus())

		// One degraded
		checker.Register("service3", func(ctx context.Context) health.HealthCheckResult {
			return health.HealthCheckResult{Status: health.HealthStatusDegraded}
		})

		checker.Check(context.Background())
		assert.Equal(t, health.HealthStatusDegraded, checker.GetOverallStatus())

		// One unhealthy
		checker.Register("service4", func(ctx context.Context) health.HealthCheckResult {
			return health.HealthCheckResult{Status: health.HealthStatusUnhealthy}
		})

		checker.Check(context.Background())
		assert.Equal(t, health.HealthStatusUnhealthy, checker.GetOverallStatus())
	})

	t.Run("Timeout", func(t *testing.T) {
		checker.Register("slow-check", func(ctx context.Context) health.HealthCheckResult {
			select {
			case <-time.After(200 * time.Millisecond):
				return health.HealthCheckResult{
					Status:  health.HealthStatusHealthy,
					Message: "Should not reach here",
				}
			case <-ctx.Done():
				return health.HealthCheckResult{
					Status:  health.HealthStatusUnhealthy,
					Message: "Check timed out",
				}
			}
		})

		results := checker.Check(context.Background())
		// The check should timeout since we set timeout to 50ms
		assert.Equal(t, health.HealthStatusUnhealthy, results["slow-check"].Status)
	})

	t.Run("GetCachedResults", func(t *testing.T) {
		// Clear and register new check
		checker.Unregister("slow-check")
		counter := 0
		checker.Register("counter", func(ctx context.Context) health.HealthCheckResult {
			counter++
			return health.HealthCheckResult{
				Status:  health.HealthStatusHealthy,
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
		checker.Register("background", func(ctx context.Context) health.HealthCheckResult {
			atomic.AddInt32(&updateCount, 1)
			return health.HealthCheckResult{
				Status: health.HealthStatusHealthy,
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
		check := health.DatabaseHealthCheck(func(ctx context.Context) error {
			return nil
		})

		result := check(context.Background())
		assert.Equal(t, health.HealthStatusHealthy, result.Status)
		assert.Contains(t, result.Message, "successful")
	})

	t.Run("Failure", func(t *testing.T) {
		testErr := errors.New("connection refused")
		check := health.DatabaseHealthCheck(func(ctx context.Context) error {
			return testErr
		})

		result := check(context.Background())
		assert.Equal(t, health.HealthStatusUnhealthy, result.Status)
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
		check := health.HTTPHealthCheck(ts.URL+"/health", 200)
		result := check(context.Background())

		assert.Equal(t, health.HealthStatusHealthy, result.Status)
		assert.Equal(t, ts.URL+"/health", result.Details["url"])
		assert.Equal(t, 200, result.Details["status"])
	})

	t.Run("Wrong Status", func(t *testing.T) {
		check := health.HTTPHealthCheck(ts.URL+"/notfound", 200)
		result := check(context.Background())

		assert.Equal(t, health.HealthStatusUnhealthy, result.Status)
		assert.Equal(t, 404, result.Details["actual_status"])
	})

	t.Run("Invalid URL", func(t *testing.T) {
		check := health.HTTPHealthCheck("http://invalid-domain-that-does-not-exist.com", 200)
		result := check(context.Background())

		assert.Equal(t, health.HealthStatusUnhealthy, result.Status)
		if !strings.Contains(result.Message, "request failed") {
			assert.Contains(t, result.Message, "Unexpected status code")
		}
	})
}

func TestMemoryHealthCheck(t *testing.T) {
	check := health.MemoryHealthCheck(1024)
	result := check(context.Background())

	// Since this is a placeholder implementation, just verify it returns healthy
	assert.Equal(t, health.HealthStatusHealthy, result.Status)
	assert.Equal(t, uint64(1024), result.Details["max_memory_mb"])
}
