package health

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// SafeHealthChecker is a thread-safe implementation of health checker
type SafeHealthChecker struct {
	checks   sync.Map // map[string]HealthCheck
	results  sync.Map // map[string]HealthCheckResult
	interval time.Duration
	timeout  time.Duration
	stopCh   chan struct{}
	stopped  int32 // atomic flag
	wg       sync.WaitGroup
}

// NewSafeHealthChecker creates a new thread-safe health checker
func NewSafeHealthChecker(interval, timeout time.Duration) *SafeHealthChecker {
	hc := &SafeHealthChecker{
		interval: interval,
		timeout:  timeout,
		stopCh:   make(chan struct{}),
	}
	
	// Start background health checks
	hc.wg.Add(1)
	go hc.runChecks()
	
	return hc
}

// Register registers a health check
func (hc *SafeHealthChecker) Register(name string, check HealthCheck) {
	if atomic.LoadInt32(&hc.stopped) == 1 {
		return
	}
	hc.checks.Store(name, check)
}

// Unregister removes a health check
func (hc *SafeHealthChecker) Unregister(name string) {
	hc.checks.Delete(name)
	hc.results.Delete(name)
}

// Check performs all health checks and returns the results
func (hc *SafeHealthChecker) Check(ctx context.Context) map[string]HealthCheckResult {
	if atomic.LoadInt32(&hc.stopped) == 1 {
		return make(map[string]HealthCheckResult)
	}

	results := make(map[string]HealthCheckResult)
	var wg sync.WaitGroup
	var mu sync.Mutex
	
	// Iterate over all registered checks
	hc.checks.Range(func(key, value any) bool {
		name := key.(string)
		check := value.(HealthCheck)
		
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
			hc.results.Store(n, result)
		}(name, check)
		
		return true
	})
	
	wg.Wait()
	return results
}

// GetResults returns cached health check results
func (hc *SafeHealthChecker) GetResults() map[string]HealthCheckResult {
	results := make(map[string]HealthCheckResult)
	
	hc.results.Range(func(key, value any) bool {
		name := key.(string)
		result := value.(HealthCheckResult)
		results[name] = result
		return true
	})
	
	return results
}

// GetOverallStatus returns the overall health status
func (hc *SafeHealthChecker) GetOverallStatus() HealthStatus {
	hasUnhealthy := false
	hasDegraded := false
	hasAny := false
	
	hc.results.Range(func(key, value any) bool {
		hasAny = true
		result := value.(HealthCheckResult)
		
		switch result.Status {
		case HealthStatusUnhealthy:
			hasUnhealthy = true
		case HealthStatusDegraded:
			hasDegraded = true
		}
		
		return true
	})
	
	if !hasAny {
		return HealthStatusHealthy
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
func (hc *SafeHealthChecker) runChecks() {
	defer hc.wg.Done()
	
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
		case <-hc.stopCh:
			return
		}
	}
}

// Stop stops the health checker
func (hc *SafeHealthChecker) Stop() {
	if !atomic.CompareAndSwapInt32(&hc.stopped, 0, 1) {
		return // Already stopped
	}
	
	close(hc.stopCh)
	hc.wg.Wait()
}

// Alternative fix for the original HealthChecker
// This demonstrates minimal changes needed to fix race conditions

// FixedHealthChecker wraps the original with proper synchronization
type FixedHealthChecker struct {
	*HealthChecker
	stopOnce sync.Once
}

// NewFixedHealthChecker creates a race-free wrapper
func NewFixedHealthChecker(interval, timeout time.Duration) *FixedHealthChecker {
	return &FixedHealthChecker{
		HealthChecker: NewHealthChecker(interval, timeout),
	}
}

// Stop ensures single execution
func (fhc *FixedHealthChecker) Stop() {
	fhc.stopOnce.Do(func() {
		if fhc.stop != nil {
			close(fhc.stop)
		}
	})
}