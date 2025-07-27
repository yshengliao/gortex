package metrics

import (
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"
)

// BenchmarkMetricsCollectors provides comprehensive performance benchmarks
// for all collector implementations to track performance changes over time
func BenchmarkMetricsCollectors(b *testing.B) {
	collectors := map[string]func() interface {
		RecordBusinessMetric(name string, value float64, tags map[string]string)
		RecordHTTPRequest(method, path string, statusCode int, duration time.Duration)
		GetStats() map[string]any
		Reset()
	}{
		"ImprovedCollector": func() interface {
			RecordBusinessMetric(name string, value float64, tags map[string]string)
			RecordHTTPRequest(method, path string, statusCode int, duration time.Duration)
			GetStats() map[string]any
			Reset()
		} {
			return NewImprovedCollectorWithCardinality(1000)
		},
		"ShardedCollector": func() interface {
			RecordBusinessMetric(name string, value float64, tags map[string]string)
			RecordHTTPRequest(method, path string, statusCode int, duration time.Duration)
			GetStats() map[string]any
			Reset()
		} {
			return NewShardedCollectorWithCardinality(1000)
		},
	}

	for collectorName, factory := range collectors {
		b.Run(collectorName, func(b *testing.B) {
			benchmarkCollectorSuite(b, factory)
		})
	}
}

func benchmarkCollectorSuite(b *testing.B, factory func() interface {
	RecordBusinessMetric(name string, value float64, tags map[string]string)
	RecordHTTPRequest(method, path string, statusCode int, duration time.Duration)
	GetStats() map[string]any
	Reset()
}) {
	// Scenario 1: High-Concurrency Writes
	b.Run("HighConcurrencyWrites", func(b *testing.B) {
		collector := factory()
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
					metricName := fmt.Sprintf("worker_%d_metric_%d", workerID, j%100)
					tags := map[string]string{"worker": fmt.Sprintf("%d", workerID)}
					collector.RecordBusinessMetric(metricName, float64(j), tags)
				}
			}(i)
		}
		
		close(start)
		wg.Wait()
	})

	// Scenario 2: Different Tag Combinations (High Cardinality)
	b.Run("HighCardinalityTags", func(b *testing.B) {
		collector := factory()
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Create metrics with different tag combinations
			tags := map[string]string{
				"service":     fmt.Sprintf("service_%d", i%10),
				"version":     fmt.Sprintf("v%d", i%5),
				"environment": fmt.Sprintf("env_%d", i%3),
				"region":      fmt.Sprintf("region_%d", i%4),
			}
			collector.RecordBusinessMetric("api_requests", float64(i), tags)
		}
	})

	// Scenario 3: Mixed Read/Write Operations
	b.Run("MixedReadWrite", func(b *testing.B) {
		collector := factory()
		numWorkers := runtime.NumCPU()
		
		// Pre-populate with some data
		for i := 0; i < 100; i++ {
			collector.RecordBusinessMetric(fmt.Sprintf("metric_%d", i), float64(i), nil)
		}
		
		b.ResetTimer()
		var wg sync.WaitGroup
		wg.Add(numWorkers)
		
		start := make(chan struct{})
		for i := 0; i < numWorkers; i++ {
			go func(workerID int) {
				defer wg.Done()
				<-start
				
				for j := 0; j < b.N/numWorkers; j++ {
					if j%4 == 0 {
						// Read operation (25% of operations)
						collector.GetStats()
					} else {
						// Write operation (75% of operations)
						metricName := fmt.Sprintf("worker_%d_metric_%d", workerID, j%50)
						collector.RecordBusinessMetric(metricName, float64(j), nil)
					}
				}
			}(i)
		}
		
		close(start)
		wg.Wait()
	})

	// Scenario 4: HTTP Request Recording (Real-world usage)
	b.Run("HTTPRequestRecording", func(b *testing.B) {
		collector := factory()
		methods := []string{"GET", "POST", "PUT", "DELETE"}
		paths := []string{"/api/users", "/api/orders", "/api/products", "/health"}
		statusCodes := []int{200, 201, 400, 404, 500}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			method := methods[i%len(methods)]
			path := paths[i%len(paths)]
			statusCode := statusCodes[i%len(statusCodes)]
			duration := time.Duration(i%100) * time.Millisecond
			
			collector.RecordHTTPRequest(method, path, statusCode, duration)
		}
	})

	// Scenario 5: Bulk Metrics Recording
	b.Run("BulkMetricsRecording", func(b *testing.B) {
		collector := factory()
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Simulate bulk recording of related metrics
			userID := i % 1000
			tags := map[string]string{"user_id": fmt.Sprintf("%d", userID)}
			
			collector.RecordBusinessMetric("user_login_count", 1, tags)
			collector.RecordBusinessMetric("user_session_duration", float64(i%3600), tags)
			collector.RecordBusinessMetric("user_page_views", float64(i%50), tags)
		}
	})

	// Scenario 6: Single-Threaded Performance Baseline
	b.Run("SingleThreadedBaseline", func(b *testing.B) {
		collector := factory()
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			collector.RecordBusinessMetric(fmt.Sprintf("metric_%d", i%100), float64(i), nil)
		}
	})

	// Scenario 7: Aggregated Data Reads (Dashboard simulation)
	b.Run("AggregatedDataReads", func(b *testing.B) {
		collector := factory()
		
		// Pre-populate with realistic data
		for i := 0; i < 500; i++ {
			tags := map[string]string{
				"service": fmt.Sprintf("service_%d", i%5),
				"status":  fmt.Sprintf("status_%d", i%3),
			}
			collector.RecordBusinessMetric("requests_total", float64(i), tags)
			collector.RecordHTTPRequest("GET", "/api", 200, time.Millisecond)
		}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Simulate dashboard refreshing all metrics
			stats := collector.GetStats()
			_ = stats["business"]
			_ = stats["http"]
			_ = stats["cardinality"]
		}
	})

	// Scenario 8: Memory Pressure Simulation
	b.Run("MemoryPressure", func(b *testing.B) {
		collector := factory()
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Create unique metric names to force memory allocation
			metricName := fmt.Sprintf("unique_metric_%d_%d", i, time.Now().UnixNano())
			tags := map[string]string{
				"id":        fmt.Sprintf("%d", i),
				"timestamp": fmt.Sprintf("%d", time.Now().UnixNano()),
			}
			collector.RecordBusinessMetric(metricName, float64(i), tags)
		}
	})

	// Scenario 9: Time Series Simulation
	b.Run("TimeSeriesSimulation", func(b *testing.B) {
		collector := factory()
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Simulate time series data with consistent metric names but changing values
			timestamp := time.Now().Add(time.Duration(i) * time.Second)
			tags := map[string]string{
				"timestamp": fmt.Sprintf("%d", timestamp.Unix()),
				"host":      fmt.Sprintf("host_%d", i%10),
			}
			
			collector.RecordBusinessMetric("cpu_usage", float64(i%100), tags)
			collector.RecordBusinessMetric("memory_usage", float64((i*2)%100), tags)
			collector.RecordBusinessMetric("disk_usage", float64((i*3)%100), tags)
		}
	})

	// Scenario 10: Contentious Key Access
	b.Run("ContentiousKeyAccess", func(b *testing.B) {
		collector := factory()
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
					// All workers update the same few metrics (worst case contention)
					metricName := fmt.Sprintf("shared_metric_%d", j%3)
					collector.RecordBusinessMetric(metricName, float64(workerID*1000+j), nil)
				}
			}(i)
		}
		
		close(start)
		wg.Wait()
	})
}

// BenchmarkMetricsMemoryFootprint measures memory usage patterns
func BenchmarkMetricsMemoryFootprint(b *testing.B) {
	collectors := map[string]func() interface {
		RecordBusinessMetric(name string, value float64, tags map[string]string)
		Reset()
	}{
		"ImprovedCollector": func() interface {
			RecordBusinessMetric(name string, value float64, tags map[string]string)
			Reset()
		} {
			return NewImprovedCollectorWithCardinality(10000)
		},
		"ShardedCollector": func() interface {
			RecordBusinessMetric(name string, value float64, tags map[string]string)
			Reset()
		} {
			return NewShardedCollectorWithCardinality(10000)
		},
	}

	for collectorName, factory := range collectors {
		b.Run(collectorName, func(b *testing.B) {
			collector := factory()
			
			b.ReportAllocs()
			b.ResetTimer()
			
			for i := 0; i < b.N; i++ {
				metricName := fmt.Sprintf("metric_%d", i%1000)
				tags := map[string]string{"batch": fmt.Sprintf("%d", i/1000)}
				collector.RecordBusinessMetric(metricName, float64(i), tags)
			}
		})
	}
}

// BenchmarkCardinalityLimits tests behavior under cardinality pressure
func BenchmarkCardinalityLimits(b *testing.B) {
	cardinalityLimits := []int{100, 1000, 10000}
	
	for _, limit := range cardinalityLimits {
		b.Run(fmt.Sprintf("Limit_%d", limit), func(b *testing.B) {
			b.Run("ImprovedCollector", func(b *testing.B) {
				collector := NewImprovedCollectorWithCardinality(limit)
				
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					// Generate more unique metrics than the limit
					metricName := fmt.Sprintf("metric_%d", i)
					collector.RecordBusinessMetric(metricName, float64(i), nil)
				}
			})
			
			b.Run("ShardedCollector", func(b *testing.B) {
				collector := NewShardedCollectorWithCardinality(limit)
				
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					// Generate more unique metrics than the limit
					metricName := fmt.Sprintf("metric_%d", i)
					collector.RecordBusinessMetric(metricName, float64(i), nil)
				}
			})
		})
	}
}

// BenchmarkScalability tests performance scaling with worker count
func BenchmarkScalability(b *testing.B) {
	workerCounts := []int{1, 2, 4, 8, 16, 32}
	
	for _, workers := range workerCounts {
		if workers > runtime.NumCPU()*4 {
			continue // Skip unrealistic worker counts
		}
		
		b.Run(fmt.Sprintf("Workers_%d", workers), func(b *testing.B) {
			b.Run("ImprovedCollector", func(b *testing.B) {
				benchmarkScalabilityImpl(b, NewImprovedCollectorWithCardinality(1000), workers)
			})
			
			b.Run("ShardedCollector", func(b *testing.B) {
				benchmarkScalabilityImpl(b, NewShardedCollectorWithCardinality(1000), workers)
			})
		})
	}
}

func benchmarkScalabilityImpl(b *testing.B, collector interface {
	RecordBusinessMetric(name string, value float64, tags map[string]string)
}, workers int) {
	b.ResetTimer()
	
	var wg sync.WaitGroup
	wg.Add(workers)
	
	start := make(chan struct{})
	for i := 0; i < workers; i++ {
		go func(workerID int) {
			defer wg.Done()
			<-start
			
			for j := 0; j < b.N/workers; j++ {
				metricName := fmt.Sprintf("worker_%d_metric_%d", workerID, j%100)
				collector.RecordBusinessMetric(metricName, float64(j), nil)
			}
		}(i)
	}
	
	close(start)
	wg.Wait()
}