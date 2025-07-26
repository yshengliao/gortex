package metrics

import (
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestShardedCollectorBasicFunctionality(t *testing.T) {
	collector := NewShardedCollectorWithCardinality(48) // 16 shards * 3 per shard

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

	t.Run("CardinalityLimit", func(t *testing.T) {
		collector.Reset()
		
		// Add more metrics than the limit to test eviction
		for i := 0; i < 60; i++ {
			collector.RecordBusinessMetric(fmt.Sprintf("metric_%d", i), float64(i), nil)
		}
		
		info := collector.GetCardinalityInfo()
		currentMetrics := info["current_metrics"].(int64)
		maxCardinality := int64(info["max_cardinality"].(int))
		
		assert.True(t, currentMetrics <= maxCardinality,
			"Current metrics (%d) should not exceed max cardinality (%d)",
			currentMetrics, maxCardinality)
		
		evictions := info["evictions"].(EvictionStats)
		assert.True(t, evictions.TotalEvictions > 0, "Should have evictions")
	})
}

func TestShardedCollectorConcurrency(t *testing.T) {
	collector := NewShardedCollectorWithCardinality(160) // Allow for more metrics in concurrent test
	
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
					metricName := fmt.Sprintf("worker_%d_metric_%d", workerID, j%20)
					tags := map[string]string{"worker": fmt.Sprintf("%d", workerID)}
					collector.RecordBusinessMetric(metricName, float64(j), tags)
				}
			}(i)
		}
		
		close(start)
		wg.Wait()
		
		// Verify no data races occurred
		stats := collector.GetStats()
		assert.NotNil(t, stats["business"])
		
		info := collector.GetCardinalityInfo()
		currentMetrics := info["current_metrics"].(int64)
		maxCardinality := int64(info["max_cardinality"].(int))
		
		assert.True(t, currentMetrics <= maxCardinality,
			"Current metrics (%d) should not exceed max cardinality (%d)",
			currentMetrics, maxCardinality)
	})

	t.Run("HighContentionSameKeys", func(t *testing.T) {
		collector.Reset()
		numWorkers := runtime.NumCPU() * 2
		opsPerWorker := 100
		
		var wg sync.WaitGroup
		wg.Add(numWorkers)
		
		start := make(chan struct{})
		for i := 0; i < numWorkers; i++ {
			go func(workerID int) {
				defer wg.Done()
				<-start
				
				for j := 0; j < opsPerWorker; j++ {
					// Use same metric names to test lock contention on same shards
					metricName := fmt.Sprintf("shared_metric_%d", j%5)
					collector.RecordBusinessMetric(metricName, float64(workerID*1000+j), nil)
				}
			}(i)
		}
		
		close(start)
		wg.Wait()
		
		// Verify final state
		stats := collector.GetStats()
		business := stats["business"].(map[string]float64)
		
		// Should have at most 5 different metrics
		assert.True(t, len(business) <= 5, "Should have at most 5 different metrics")
		
		// Each metric should have a value (last writer wins)
		for i := 0; i < 5; i++ {
			metricName := fmt.Sprintf("shared_metric_%d", i)
			if val, exists := business[metricName]; exists {
				assert.True(t, val >= 0, "Metric value should be non-negative")
			}
		}
	})
}

func TestShardedCollectorSharding(t *testing.T) {
	collector := NewShardedCollectorWithCardinality(160)
	
	// Test that different metric names get distributed across shards
	metricNames := []string{
		"metric_a", "metric_b", "metric_c", "metric_d", "metric_e",
		"user_count", "order_total", "response_time", "error_rate", "cache_hits",
	}
	
	shardDistribution := make(map[int]int)
	
	for _, name := range metricNames {
		shardIndex := collector.hashKey(name)
		shardDistribution[shardIndex]++
		collector.RecordBusinessMetric(name, 1.0, nil)
	}
	
	// Verify metrics are distributed across multiple shards
	assert.True(t, len(shardDistribution) > 1, "Metrics should be distributed across multiple shards")
	
	// Verify all metrics are recorded
	stats := collector.GetStats()
	business := stats["business"].(map[string]float64)
	assert.Equal(t, len(metricNames), len(business), "All metrics should be recorded")
}

// BenchmarkShardedCollectorConcurrency tests sharded collector performance
func BenchmarkShardedCollectorConcurrency(b *testing.B) {
	collector := NewShardedCollectorWithCardinality(1000)
	
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
	
	b.Run("HighContentionSameShards", func(b *testing.B) {
		// Test worst case: all workers hitting the same few shards
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
					// Force all workers to use same set of metric names (same shards)
					metricName := fmt.Sprintf("contended_metric_%d", j%3)
					collector.RecordBusinessMetric(metricName, float64(j), nil)
				}
			}(i)
		}
		
		close(start)
		wg.Wait()
	})
}