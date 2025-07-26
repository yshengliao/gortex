package observability

import (
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestImprovedCollector(t *testing.T) {
	collector := NewImprovedCollector()

	t.Run("RecordHTTPRequest", func(t *testing.T) {
		collector.RecordHTTPRequest("GET", "/test", 200, 100*time.Millisecond)
		collector.RecordHTTPRequest("POST", "/api", 404, 50*time.Millisecond)

		stats := collector.GetHTTPStats()
		assert.Equal(t, int64(2), stats.TotalRequests)
		assert.Equal(t, int64(1), stats.RequestsByStatus[200])
		assert.Equal(t, int64(1), stats.RequestsByStatus[404])
		assert.Equal(t, int64(1), stats.RequestsByMethod["GET"])
		assert.Equal(t, int64(1), stats.RequestsByMethod["POST"])
	})

	t.Run("RecordWebSocketConnection", func(t *testing.T) {
		collector.Reset()
		
		collector.RecordWebSocketConnection(true)
		collector.RecordWebSocketConnection(true)
		collector.RecordWebSocketConnection(false)

		stats := collector.GetWebSocketStats()
		assert.Equal(t, int64(1), stats.ActiveConnections)
	})

	t.Run("RecordWebSocketMessage", func(t *testing.T) {
		collector.Reset()
		
		collector.RecordWebSocketMessage("inbound", "text", 100)
		collector.RecordWebSocketMessage("outbound", "binary", 200)

		stats := collector.GetWebSocketStats()
		assert.Equal(t, int64(2), stats.TotalMessages)
		assert.Equal(t, int64(1), stats.MessagesByType["inbound_text"])
		assert.Equal(t, int64(1), stats.MessagesByType["outbound_binary"])
	})

	t.Run("RecordBusinessMetric", func(t *testing.T) {
		collector.Reset()
		
		collector.RecordBusinessMetric("cpu.usage", 85.5, nil)
		collector.RecordBusinessMetric("disk.free", 1024.0, nil)

		stats := collector.GetStats()
		business := stats["business"].(map[string]float64)
		assert.Equal(t, 85.5, business["cpu.usage"])
		assert.Equal(t, 1024.0, business["disk.free"])
	})

	t.Run("RecordSystemMetrics", func(t *testing.T) {
		collector.Reset()
		
		collector.RecordGoroutines(42)
		collector.RecordMemoryUsage(1024*1024*100) // 100MB

		stats := collector.GetSystemStats()
		assert.Equal(t, 42, stats.GoroutineCount)
		assert.Equal(t, uint64(1024*1024*100), stats.MemoryUsage)
	})

	t.Run("GetStats", func(t *testing.T) {
		collector.Reset()
		collector.RecordHTTPRequest("GET", "/", 200, 10*time.Millisecond)
		collector.RecordWebSocketConnection(true)
		collector.RecordBusinessMetric("test.metric", 123.45, nil)
		
		stats := collector.GetStats()
		
		assert.Contains(t, stats, "http")
		assert.Contains(t, stats, "websocket") 
		assert.Contains(t, stats, "system")
		assert.Contains(t, stats, "business")
		assert.Contains(t, stats, "timestamp")
	})

	t.Run("Reset", func(t *testing.T) {
		// Add some metrics
		collector.RecordHTTPRequest("GET", "/", 200, 10*time.Millisecond)
		collector.RecordWebSocketConnection(true)
		collector.RecordBusinessMetric("test", 123, nil)
		collector.RecordGoroutines(10)
		collector.RecordMemoryUsage(1024)

		// Reset
		collector.Reset()

		// Check all metrics are cleared
		httpStats := collector.GetHTTPStats()
		wsStats := collector.GetWebSocketStats()
		sysStats := collector.GetSystemStats()
		stats := collector.GetStats()
		
		assert.Equal(t, int64(0), httpStats.TotalRequests)
		assert.Equal(t, int64(0), wsStats.ActiveConnections)
		assert.Equal(t, int64(0), wsStats.TotalMessages)
		assert.Equal(t, 0, sysStats.GoroutineCount)
		assert.Equal(t, uint64(0), sysStats.MemoryUsage)
		
		business := stats["business"].(map[string]float64)
		assert.Len(t, business, 0)
	})
}

// TestImprovedCollector_ConcurrentAccess tests that ImprovedCollector is thread-safe
func TestImprovedCollector_ConcurrentAccess(t *testing.T) {
	collector := NewImprovedCollector()
	
	const numGoroutines = 100
	const numOperations = 100
	
	var wg sync.WaitGroup
	start := time.Now()
	
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			for j := 0; j < numOperations; j++ {
				// Simulate different operations
				collector.RecordHTTPRequest("GET", "/test", 200, time.Millisecond)
				collector.RecordWebSocketConnection(true)
				collector.RecordWebSocketMessage("inbound", "text", 100)
				collector.RecordBusinessMetric("test.metric", float64(id*j), nil)
				collector.RecordGoroutines(runtime.NumGoroutine())
				
				// Read operations
				collector.GetHTTPStats()
				collector.GetWebSocketStats()
				collector.GetSystemStats()
				collector.GetStats()
			}
		}(i)
	}
	
	wg.Wait()
	duration := time.Since(start)
	
	t.Logf("併發測試完成: %d 操作在 %v 內完成", numGoroutines*numOperations*9, duration)
	t.Logf("平均每個操作: %v", duration/(numGoroutines*numOperations*9))
	
	// Verify some metrics were recorded
	httpStats := collector.GetHTTPStats()
	assert.Greater(t, httpStats.TotalRequests, int64(0))
	
	wsStats := collector.GetWebSocketStats()
	assert.Greater(t, wsStats.ActiveConnections, int64(0))
}

// TestImprovedCollector_MemoryStability tests that ImprovedCollector doesn't leak memory
func TestImprovedCollector_MemoryStability(t *testing.T) {
	collector := NewImprovedCollector()
	
	// Record initial state
	initialHTTPStats := collector.GetHTTPStats()
	
	// Simulate heavy usage - key insight: ImprovedCollector aggregates rather than stores raw events
	for i := 0; i < 10000; i++ {
		collector.RecordHTTPRequest("GET", "/test", 200, time.Millisecond)
		collector.RecordWebSocketConnection(i%2 == 0)
		collector.RecordWebSocketMessage("inbound", "text", 100)
		collector.RecordBusinessMetric("metric", float64(i), nil) // This overwrites previous value
		collector.RecordGoroutines(10)
		collector.RecordMemoryUsage(1024)
		
		// Periodically read stats
		if i%100 == 0 {
			collector.GetStats()
		}
	}
	
	// Check that metrics were recorded but memory structure is bounded
	finalHTTPStats := collector.GetHTTPStats()
	
	// HTTP requests should be counted
	assert.Greater(t, finalHTTPStats.TotalRequests, initialHTTPStats.TotalRequests)
	
	// Business metrics map should only contain the latest value (not growing unboundedly)
	stats := collector.GetStats()
	business := stats["business"].(map[string]float64)
	
	// Only one business metric should exist (latest value), not 10000 entries
	assert.Len(t, business, 1)
	assert.Equal(t, float64(9999), business["metric"]) // Latest value
	
	t.Logf("Final business metrics count: %d (expected: 1)", len(business))
	t.Logf("Final HTTP request count: %d", finalHTTPStats.TotalRequests)
	
	// The key improvement: maps don't grow with request count
	// StatusCode and Method maps should have limited entries based on unique values, not total requests
	assert.LessOrEqual(t, len(finalHTTPStats.RequestsByStatus), 5) // Should be small
	assert.LessOrEqual(t, len(finalHTTPStats.RequestsByMethod), 5) // Should be small
}

// Benchmark ImprovedCollector
func BenchmarkImprovedCollector_RecordHTTPRequest(b *testing.B) {
	collector := NewImprovedCollector()
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			collector.RecordHTTPRequest("GET", "/test", 200, time.Millisecond)
		}
	})
}