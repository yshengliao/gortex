package metrics

import (
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestOptimizedCollectorBasicFunctionality(t *testing.T) {
	collector := NewOptimizedCollectorWithCardinality(3)

	t.Run("BusinessMetrics", func(t *testing.T) {
		collector.RecordBusinessMetric("test_metric", 42.0, nil)
		
		stats := collector.GetStats()
		business := stats["business"].(map[string]float64)
		assert.Equal(t, 42.0, business["test_metric"])
	})

	t.Run("HTTPMetrics", func(t *testing.T) {
		collector.RecordHTTPRequest("GET", "/api", 200, 100*time.Millisecond)
		
		httpStats := collector.GetHTTPStats()
		assert.Equal(t, int64(1), httpStats.TotalRequests)
		assert.Equal(t, int64(1), httpStats.RequestsByStatus[200])
		assert.Equal(t, int64(1), httpStats.RequestsByMethod["GET"])
	})

	t.Run("WebSocketMetrics", func(t *testing.T) {
		collector.RecordWebSocketConnection(true)
		collector.RecordWebSocketMessage("inbound", "text", 100)
		
		wsStats := collector.GetWebSocketStats()
		assert.Equal(t, int64(1), wsStats.ActiveConnections)
		assert.Equal(t, int64(1), wsStats.TotalMessages)
	})

	t.Run("SystemMetrics", func(t *testing.T) {
		collector.RecordGoroutines(10)
		collector.RecordMemoryUsage(1024)
		
		sysStats := collector.GetSystemStats()
		assert.Equal(t, 10, sysStats.GoroutineCount)
		assert.Equal(t, uint64(1024), sysStats.MemoryUsage)
	})
}

func TestOptimizedCollectorLRUCardinality(t *testing.T) {
	collector := NewOptimizedCollectorWithCardinality(3)

	t.Run("CardinalityLimit", func(t *testing.T) {
		// Add metrics up to the limit
		collector.RecordBusinessMetric("metric1", 1.0, nil)
		collector.RecordBusinessMetric("metric2", 2.0, nil)
		collector.RecordBusinessMetric("metric3", 3.0, nil)
		
		info := collector.GetCardinalityInfo()
		assert.Equal(t, int64(3), info["current_metrics"])
		assert.Equal(t, 3, info["max_cardinality"])
		
		// Add one more metric, should trigger eviction
		collector.RecordBusinessMetric("metric4", 4.0, nil)
		
		info = collector.GetCardinalityInfo()
		assert.Equal(t, int64(3), info["current_metrics"]) // Still at limit
		
		evictions := info["evictions"].(EvictionStats)
		assert.Equal(t, int64(1), evictions.TotalEvictions)
		assert.Contains(t, evictions.EvictedMetrics, "metric1") // LRU should be metric1
	})

	t.Run("LRUEvictionOrder", func(t *testing.T) {
		collector.Reset()
		
		// Add metrics in order
		collector.RecordBusinessMetric("a", 1.0, nil)
		collector.RecordBusinessMetric("b", 2.0, nil)
		collector.RecordBusinessMetric("c", 3.0, nil)
		
		// Access 'a' to make it most recently used
		collector.RecordBusinessMetric("a", 1.1, nil)
		
		// Add new metric, should evict 'b' (least recently used)
		collector.RecordBusinessMetric("d", 4.0, nil)
		
		stats := collector.GetStats()
		business := stats["business"].(map[string]float64)
		
		// Should have a, c, d (b should be evicted)
		assert.Contains(t, business, "a")
		assert.Contains(t, business, "c")
		assert.Contains(t, business, "d")
		assert.NotContains(t, business, "b")
		
		evictionStats := collector.GetEvictionStats()
		assert.Contains(t, evictionStats.EvictedMetrics, "b")
	})
}

func TestOptimizedCollectorConcurrency(t *testing.T) {
	collector := NewOptimizedCollectorWithCardinality(100)
	
	t.Run("ConcurrentBusinessMetrics", func(t *testing.T) {
		numWorkers := runtime.NumCPU()
		opsPerWorker := 100
		
		var wg sync.WaitGroup
		wg.Add(numWorkers)
		
		start := make(chan struct{})
		for i := 0; i < numWorkers; i++ {
			go func(workerID int) {
				defer wg.Done()
				<-start
				
				for j := 0; j < opsPerWorker; j++ {
					metricName := fmt.Sprintf("worker_%d_metric_%d", workerID, j)
					collector.RecordBusinessMetric(metricName, float64(j), nil)
				}
			}(i)
		}
		
		close(start)
		wg.Wait()
		
		// Verify cardinality limit is respected
		info := collector.GetCardinalityInfo()
		currentMetrics := info["current_metrics"].(int64)
		maxCardinality := int64(info["max_cardinality"].(int))
		
		assert.True(t, currentMetrics <= maxCardinality,
			"Current metrics (%d) should not exceed max cardinality (%d)",
			currentMetrics, maxCardinality)
	})

	t.Run("ConcurrentHTTPRequests", func(t *testing.T) {
		numWorkers := runtime.NumCPU()
		requestsPerWorker := 100
		
		var wg sync.WaitGroup
		wg.Add(numWorkers)
		
		start := make(chan struct{})
		for i := 0; i < numWorkers; i++ {
			go func(workerID int) {
				defer wg.Done()
				<-start
				
				for j := 0; j < requestsPerWorker; j++ {
					collector.RecordHTTPRequest("GET", "/test", 200, time.Millisecond)
				}
			}(i)
		}
		
		close(start)
		wg.Wait()
		
		httpStats := collector.GetHTTPStats()
		expectedTotal := int64(numWorkers * requestsPerWorker)
		assert.Equal(t, expectedTotal, httpStats.TotalRequests)
		assert.Equal(t, expectedTotal, httpStats.RequestsByStatus[200])
	})

	t.Run("MixedConcurrentOperations", func(t *testing.T) {
		numWorkers := runtime.NumCPU()
		opsPerWorker := 50
		
		var wg sync.WaitGroup
		wg.Add(numWorkers)
		
		start := make(chan struct{})
		for i := 0; i < numWorkers; i++ {
			go func(workerID int) {
				defer wg.Done()
				<-start
				
				for j := 0; j < opsPerWorker; j++ {
					switch j % 4 {
					case 0:
						collector.RecordBusinessMetric(fmt.Sprintf("metric_%d_%d", workerID, j), float64(j), nil)
					case 1:
						collector.RecordHTTPRequest("POST", "/api", 201, time.Millisecond)
					case 2:
						collector.GetStats()
					case 3:
						collector.GetCardinalityInfo()
					}
				}
			}(i)
		}
		
		close(start)
		wg.Wait()
		
		// Verify all operations completed without data races
		stats := collector.GetStats()
		assert.NotNil(t, stats)
		assert.NotNil(t, stats["business"])
		assert.NotNil(t, stats["http"])
		assert.NotNil(t, stats["cardinality"])
	})
}

func TestOptimizedCollectorReset(t *testing.T) {
	collector := NewOptimizedCollectorWithCardinality(10)
	
	// Add some data
	collector.RecordBusinessMetric("test", 1.0, nil)
	collector.RecordHTTPRequest("GET", "/test", 200, time.Millisecond)
	collector.RecordWebSocketConnection(true)
	
	// Verify data exists
	stats := collector.GetStats()
	assert.NotEmpty(t, stats["business"])
	
	// Reset and verify everything is cleared
	collector.Reset()
	
	stats = collector.GetStats()
	business := stats["business"].(map[string]float64)
	assert.Empty(t, business)
	
	info := collector.GetCardinalityInfo()
	assert.Equal(t, int64(0), info["current_metrics"])
	
	httpStats := collector.GetHTTPStats()
	assert.Equal(t, int64(0), httpStats.TotalRequests)
}

// BenchmarkOptimizedCollectorConcurrency compares performance with the original
func BenchmarkOptimizedCollectorConcurrency(b *testing.B) {
	collector := NewOptimizedCollectorWithCardinality(1000)
	
	b.Run("SingleThreaded", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			collector.RecordBusinessMetric(fmt.Sprintf("metric_%d", i%100), float64(i), nil)
		}
	})
	
	b.Run("HighConcurrencyWrites", func(b *testing.B) {
		numWorkers := runtime.NumCPU()
		b.ResetTimer()
		
		var wg sync.WaitGroup
		wg.Add(numWorkers)
		
		start := make(chan struct{})
		for i := 0; i < numWorkers; i++ {
			go func(workerID int) {
				defer wg.Done()
				<-start
				
				for j := 0; j < b.N/numWorkers; j++ {
					metricName := fmt.Sprintf("worker_%d_metric_%d", workerID, j%50)
					collector.RecordBusinessMetric(metricName, float64(j), nil)
				}
			}(i)
		}
		
		close(start)
		wg.Wait()
	})
	
	b.Run("MixedReadWrites", func(b *testing.B) {
		numWorkers := runtime.NumCPU()
		b.ResetTimer()
		
		var wg sync.WaitGroup
		wg.Add(numWorkers)
		
		start := make(chan struct{})
		for i := 0; i < numWorkers; i++ {
			go func(workerID int) {
				defer wg.Done()
				<-start
				
				for j := 0; j < b.N/numWorkers; j++ {
					if j%3 == 0 {
						// Read operation
						collector.GetStats()
					} else {
						// Write operation
						metricName := fmt.Sprintf("worker_%d_metric_%d", workerID, j%50)
						collector.RecordBusinessMetric(metricName, float64(j), nil)
					}
				}
			}(i)
		}
		
		close(start)
		wg.Wait()
	})
	
	b.Run("HTTPRequestRecording", func(b *testing.B) {
		numWorkers := runtime.NumCPU()
		b.ResetTimer()
		
		var wg sync.WaitGroup
		wg.Add(numWorkers)
		
		start := make(chan struct{})
		for i := 0; i < numWorkers; i++ {
			go func(workerID int) {
				defer wg.Done()
				<-start
				
				for j := 0; j < b.N/numWorkers; j++ {
					collector.RecordHTTPRequest("GET", "/api/test", 200, time.Millisecond)
				}
			}(i)
		}
		
		close(start)
		wg.Wait()
	})
}