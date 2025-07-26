package metrics

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestImprovedCollectorLRUCardinality(t *testing.T) {
	// Create collector with small cardinality limit for testing
	collector := NewImprovedCollectorWithCardinality(3)
	
	// Test that cardinality limit is respected
	t.Run("CardinacityLimit", func(t *testing.T) {
		// Add metrics up to the limit
		collector.RecordBusinessMetric("metric1", 1.0, nil)
		collector.RecordBusinessMetric("metric2", 2.0, nil)
		collector.RecordBusinessMetric("metric3", 3.0, nil)
		
		stats := collector.GetStats()
		cardinality := stats["cardinality"].(map[string]any)
		assert.Equal(t, 3, cardinality["current"])
		assert.Equal(t, 3, cardinality["max"])
		
		// Add one more metric, should trigger eviction
		collector.RecordBusinessMetric("metric4", 4.0, nil)
		
		stats = collector.GetStats()
		cardinality = stats["cardinality"].(map[string]any)
		assert.Equal(t, 3, cardinality["current"]) // Still at limit
		
		evictions := cardinality["evictions"].(EvictionStats)
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
	
	t.Run("MetricsWithTags", func(t *testing.T) {
		collector.Reset()
		
		// Test that metrics with tags are properly handled
		tags1 := map[string]string{"service": "api", "env": "prod"}
		tags2 := map[string]string{"service": "web", "env": "dev"}
		
		collector.RecordBusinessMetric("requests", 100.0, tags1)
		collector.RecordBusinessMetric("requests", 200.0, tags2)
		
		stats := collector.GetStats()
		business := stats["business"].(map[string]float64)
		
		// Should have two different metrics due to different tags
		assert.True(t, len(business) >= 2, "Should have at least 2 metrics with different tags")
		
		// Check that tag information is included in key
		found := false
		for key := range business {
			if key != "requests" && len(key) > len("requests") {
				found = true
				break
			}
		}
		assert.True(t, found, "Should have at least one metric with tag information in key")
	})
	
	t.Run("EvictionStatsTracking", func(t *testing.T) {
		collector.Reset()
		
		// Fill up to capacity
		for i := 0; i < 3; i++ {
			collector.RecordBusinessMetric(fmt.Sprintf("metric%d", i), float64(i), nil)
		}
		
		startTime := time.Now()
		
		// Trigger multiple evictions
		for i := 3; i < 8; i++ {
			collector.RecordBusinessMetric(fmt.Sprintf("metric%d", i), float64(i), nil)
		}
		
		evictionStats := collector.GetEvictionStats()
		assert.Equal(t, int64(5), evictionStats.TotalEvictions)
		assert.True(t, evictionStats.LastEvictionTime.After(startTime))
		assert.True(t, len(evictionStats.EvictedMetrics) <= 10, "Should keep max 10 evicted metrics")
	})
	
	t.Run("CardinalityInfo", func(t *testing.T) {
		collector.Reset()
		
		// Add some metrics
		collector.RecordBusinessMetric("metric1", 1.0, nil)
		collector.RecordBusinessMetric("metric2", 2.0, nil)
		
		info := collector.GetCardinalityInfo()
		assert.Equal(t, 2, info["current_metrics"])
		assert.Equal(t, 3, info["max_cardinality"])
		
		utilization := info["utilization"].(float64)
		assert.InDelta(t, 66.67, utilization, 0.1) // 2/3 * 100 ≈ 66.67%
	})
}

func TestImprovedCollectorDefaultCardinality(t *testing.T) {
	collector := NewImprovedCollector()
	
	info := collector.GetCardinalityInfo()
	assert.Equal(t, 10000, info["max_cardinality"], "Default cardinality should be 10000")
}

func TestImprovedCollectorZeroCardinality(t *testing.T) {
	// Test that zero cardinality defaults to 10000
	collector := NewImprovedCollectorWithCardinality(0)
	
	info := collector.GetCardinalityInfo()
	assert.Equal(t, 10000, info["max_cardinality"], "Zero cardinality should default to 10000")
}

func TestImprovedCollectorConcurrency(t *testing.T) {
	collector := NewImprovedCollectorWithCardinality(100)
	
	// Test concurrent metric recording
	t.Run("ConcurrentRecording", func(t *testing.T) {
		done := make(chan bool, 10)
		
		// Start multiple goroutines recording metrics
		for i := 0; i < 10; i++ {
			go func(id int) {
				defer func() { done <- true }()
				
				for j := 0; j < 50; j++ {
					metricName := fmt.Sprintf("metric_%d_%d", id, j)
					collector.RecordBusinessMetric(metricName, float64(j), nil)
					time.Sleep(time.Microsecond) // Small delay to create concurrency
				}
			}(i)
		}
		
		// Wait for all goroutines to complete
		for i := 0; i < 10; i++ {
			<-done
		}
		
		// Verify that cardinality limit is respected
		info := collector.GetCardinalityInfo()
		currentMetrics := info["current_metrics"].(int)
		maxCardinality := info["max_cardinality"].(int)
		
		assert.True(t, currentMetrics <= maxCardinality, 
			"Current metrics (%d) should not exceed max cardinality (%d)", 
			currentMetrics, maxCardinality)
		
		// Verify that evictions occurred
		evictionStats := collector.GetEvictionStats()
		assert.True(t, evictionStats.TotalEvictions > 0, "Should have evictions with 500 metrics and limit of 100")
	})
}

// Benchmark the LRU cache performance
func BenchmarkImprovedCollectorLRU(b *testing.B) {
	collector := NewImprovedCollectorWithCardinality(1000)
	
	b.Run("RecordBusinessMetric", func(b *testing.B) {
		b.ResetTimer()
		
		for i := 0; i < b.N; i++ {
			metricName := fmt.Sprintf("metric_%d", i%1500) // Cause some evictions
			collector.RecordBusinessMetric(metricName, float64(i), nil)
		}
	})
	
	b.Run("RecordBusinessMetricWithTags", func(b *testing.B) {
		tags := map[string]string{"service": "test"}
		b.ResetTimer()
		
		for i := 0; i < b.N; i++ {
			metricName := fmt.Sprintf("metric_%d", i%1500)
			collector.RecordBusinessMetric(metricName, float64(i), tags)
		}
	})
}