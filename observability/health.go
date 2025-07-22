package observability

import (
	"context"
	"net/http"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// HealthStatus represents the health status of a component
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusDegraded  HealthStatus = "degraded"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
)

// HealthCheck represents a health check function
type HealthCheck func(ctx context.Context) HealthCheckResult

// HealthCheckResult represents the result of a health check
type HealthCheckResult struct {
	Status      HealthStatus           `json:"status"`
	Message     string                 `json:"message,omitempty"`
	Details     map[string]any `json:"details,omitempty"`
	LastChecked time.Time              `json:"last_checked"`
	Duration    time.Duration          `json:"duration_ms"`
}

// HealthChecker manages health checks
type HealthChecker struct {
	checks   map[string]HealthCheck
	results  map[string]HealthCheckResult
	mu       sync.RWMutex
	interval time.Duration
	timeout  time.Duration
	stop     chan struct{}
	stopped  int32 // atomic flag to prevent multiple stops
	stopOnce sync.Once
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(interval, timeout time.Duration) *HealthChecker {
	hc := &HealthChecker{
		checks:   make(map[string]HealthCheck),
		results:  make(map[string]HealthCheckResult),
		interval: interval,
		timeout:  timeout,
		stop:     make(chan struct{}),
	}
	
	// Start background health checks
	go hc.runChecks()
	
	return hc
}

// Register registers a health check
func (hc *HealthChecker) Register(name string, check HealthCheck) {
	if atomic.LoadInt32(&hc.stopped) == 1 {
		return // Don't register if already stopped
	}
	
	hc.mu.Lock()
	defer hc.mu.Unlock()
	
	hc.checks[name] = check
}

// Unregister removes a health check
func (hc *HealthChecker) Unregister(name string) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	
	delete(hc.checks, name)
	delete(hc.results, name)
}

// Check performs all health checks and returns the results
func (hc *HealthChecker) Check(ctx context.Context) map[string]HealthCheckResult {
	hc.mu.RLock()
	checks := make(map[string]HealthCheck)
	for name, check := range hc.checks {
		checks[name] = check
	}
	hc.mu.RUnlock()
	
	results := make(map[string]HealthCheckResult)
	var wg sync.WaitGroup
	var mu sync.Mutex
	
	for name, check := range checks {
		wg.Add(1)
		go func(n string, c HealthCheck) {
			defer wg.Done()
			
			// Create timeout context
			checkCtx, cancel := context.WithTimeout(ctx, hc.timeout)
			defer cancel()
			
			start := time.Now()
			result := c(checkCtx)
			result.Duration = time.Since(start)
			result.LastChecked = time.Now()
			
			mu.Lock()
			results[n] = result
			mu.Unlock()
			
			// Update cached result
			hc.mu.Lock()
			hc.results[n] = result
			hc.mu.Unlock()
		}(name, check)
	}
	
	wg.Wait()
	return results
}

// GetResults returns cached health check results
func (hc *HealthChecker) GetResults() map[string]HealthCheckResult {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	
	results := make(map[string]HealthCheckResult)
	for name, result := range hc.results {
		results[name] = result
	}
	
	return results
}

// GetOverallStatus returns the overall health status
func (hc *HealthChecker) GetOverallStatus() HealthStatus {
	results := hc.GetResults()
	
	if len(results) == 0 {
		return HealthStatusHealthy
	}
	
	hasUnhealthy := false
	hasDegraded := false
	
	for _, result := range results {
		switch result.Status {
		case HealthStatusUnhealthy:
			hasUnhealthy = true
		case HealthStatusDegraded:
			hasDegraded = true
		}
	}
	
	if hasUnhealthy {
		return HealthStatusUnhealthy
	}
	if hasDegraded {
		return HealthStatusDegraded
	}
	
	return HealthStatusHealthy
}

// runChecks runs health checks periodically
func (hc *HealthChecker) runChecks() {
	ticker := time.NewTicker(hc.interval)
	defer ticker.Stop()
	
	// Run initial check
	hc.Check(context.Background())
	
	for {
		select {
		case <-ticker.C:
			if atomic.LoadInt32(&hc.stopped) == 1 {
				return
			}
			hc.Check(context.Background())
		case <-hc.stop:
			return
		}
	}
}

// Stop stops the health checker
func (hc *HealthChecker) Stop() {
	// Use sync.Once to ensure Stop is only called once
	hc.stopOnce.Do(func() {
		atomic.StoreInt32(&hc.stopped, 1)
		close(hc.stop)
	})
}

// Common health checks

// DatabaseHealthCheck creates a database health check
func DatabaseHealthCheck(ping func(ctx context.Context) error) HealthCheck {
	return func(ctx context.Context) HealthCheckResult {
		err := ping(ctx)
		if err != nil {
			return HealthCheckResult{
				Status:  HealthStatusUnhealthy,
				Message: "Database connection failed",
				Details: map[string]any{
					"error": err.Error(),
				},
			}
		}
		
		return HealthCheckResult{
			Status:  HealthStatusHealthy,
			Message: "Database connection successful",
		}
	}
}

// HTTPHealthCheck creates an HTTP endpoint health check
func HTTPHealthCheck(url string, expectedStatus int) HealthCheck {
	return func(ctx context.Context) HealthCheckResult {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return HealthCheckResult{
				Status:  HealthStatusUnhealthy,
				Message: "Failed to create request",
				Details: map[string]any{
					"error": err.Error(),
					"url":   url,
				},
			}
		}
		
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return HealthCheckResult{
				Status:  HealthStatusUnhealthy,
				Message: "HTTP request failed",
				Details: map[string]any{
					"error": err.Error(),
					"url":   url,
				},
			}
		}
		defer resp.Body.Close()
		
		if resp.StatusCode != expectedStatus {
			return HealthCheckResult{
				Status:  HealthStatusUnhealthy,
				Message: "Unexpected status code",
				Details: map[string]any{
					"url":             url,
					"expected_status": expectedStatus,
					"actual_status":   resp.StatusCode,
				},
			}
		}
		
		return HealthCheckResult{
			Status:  HealthStatusHealthy,
			Message: "HTTP endpoint reachable",
			Details: map[string]any{
				"url":    url,
				"status": resp.StatusCode,
			},
		}
	}
}

// MemoryHealthCheck creates a memory usage health check
func MemoryHealthCheck(maxMemoryMB uint64) HealthCheck {
	return func(ctx context.Context) HealthCheckResult {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		
		allocMB := m.Alloc / 1024 / 1024
		totalAllocMB := m.TotalAlloc / 1024 / 1024
		sysMB := m.Sys / 1024 / 1024
		
		status := HealthStatusHealthy
		message := "Memory usage within limits"
		
		if allocMB > maxMemoryMB {
			status = HealthStatusUnhealthy
			message = "Memory usage exceeds limit"
		} else if allocMB > maxMemoryMB*80/100 {
			status = HealthStatusDegraded
			message = "Memory usage approaching limit"
		}
		
		return HealthCheckResult{
			Status:  status,
			Message: message,
			Details: map[string]any{
				"allocated_mb":       allocMB,
				"total_allocated_mb": totalAllocMB,
				"system_mb":          sysMB,
				"max_memory_mb":      maxMemoryMB,
				"num_gc":             m.NumGC,
			},
		}
	}
}