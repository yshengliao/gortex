package metrics

import (
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"
)

// BenchmarkCollectorConcurrency tests current locking performance
func BenchmarkCollectorConcurrency(b *testing.B) {
	collector := NewImprovedCollectorWithCardinality(1000)
	
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

// TestContention measures lock contention under high load
func TestCollectorContention(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping contention test in short mode")
	}
	
	collector := NewImprovedCollectorWithCardinality(1000)
	numWorkers := runtime.NumCPU() * 2
	opsPerWorker := 1000
	
	start := time.Now()
	var wg sync.WaitGroup
	wg.Add(numWorkers)
	
	startSignal := make(chan struct{})
	for i := 0; i < numWorkers; i++ {
		go func(workerID int) {
			defer wg.Done()
			<-startSignal
			
			for j := 0; j < opsPerWorker; j++ {
				switch j % 4 {
				case 0:
					collector.RecordBusinessMetric(fmt.Sprintf("metric_%d_%d", workerID, j), float64(j), nil)
				case 1:
					collector.RecordHTTPRequest("GET", "/test", 200, time.Millisecond)
				case 2:
					collector.GetStats()
				case 3:
					collector.GetHTTPStats()
				}
			}
		}(i)
	}
	
	close(startSignal)
	wg.Wait()
	
	duration := time.Since(start)
	totalOps := numWorkers * opsPerWorker
	opsPerSecond := float64(totalOps) / duration.Seconds()
	
	t.Logf("Contention test: %d workers, %d ops each, %.2f ops/sec", 
		numWorkers, opsPerWorker, opsPerSecond)
	
	// Verify data integrity
	stats := collector.GetStats()
	t.Logf("Final stats: %+v", stats)
}