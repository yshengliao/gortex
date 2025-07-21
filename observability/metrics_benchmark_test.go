package observability_test

import (
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/yshengliao/gortex/observability"
)

// BenchmarkSimpleCollector_HTTPRequest 展示 SimpleCollector 的效能問題
func BenchmarkSimpleCollector_HTTPRequest(b *testing.B) {
	collector := observability.NewSimpleCollector()
	
	b.Run("Sequential", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			collector.RecordHTTPRequest("GET", "/api/test", 200, time.Millisecond)
		}
	})
	
	b.Run("Concurrent", func(b *testing.B) {
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				collector.RecordHTTPRequest("GET", "/api/test", 200, time.Millisecond)
			}
		})
	})
}

// TestSimpleCollector_MemoryLeak 展示記憶體洩漏問題
func TestSimpleCollector_MemoryLeak(t *testing.T) {
	collector := observability.NewSimpleCollector()
	
	// 記錄初始記憶體
	runtime.GC()
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)
	
	// 模擬高負載請求
	for i := 0; i < 10000; i++ {
		collector.RecordHTTPRequest("GET", "/api/test", 200, time.Millisecond)
		collector.RecordWebSocketMessage("inbound", "text", 1024)
		collector.RecordBusinessMetric("test.metric", float64(i), map[string]string{
			"id": string(rune(i)),
		})
	}
	
	// 記錄最終記憶體
	runtime.GC()
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)
	
	memoryGrowth := m2.Alloc - m1.Alloc
	t.Logf("記憶體增長: %d bytes", memoryGrowth)
	
	// 在實際場景中，這個增長會持續且無界限
	if memoryGrowth == 0 {
		t.Error("記憶體測量可能不準確，應該有記憶體增長")
	}
}

// TestSimpleCollector_ConcurrentAccess 展示併發存取下的鎖爭用
func TestSimpleCollector_ConcurrentAccess(t *testing.T) {
	collector := observability.NewSimpleCollector()
	
	const numGoroutines = 100
	const numOperations = 100
	
	var wg sync.WaitGroup
	start := make(chan struct{})
	
	// 啟動多個 goroutine
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			<-start // 等待開始信號，確保所有 goroutine 同時開始
			
			for j := 0; j < numOperations; j++ {
				collector.RecordHTTPRequest("GET", "/api/test", 200, time.Millisecond)
				collector.RecordWebSocketConnection(true)
				collector.RecordBusinessMetric("concurrent.test", float64(id*j), nil)
			}
		}(i)
	}
	
	startTime := time.Now()
	close(start) // 開始所有 goroutine
	wg.Wait()
	duration := time.Since(startTime)
	
	totalOps := numGoroutines * numOperations * 3 // 每個 goroutine 做 3 個操作
	t.Logf("併發測試完成: %d 操作在 %v 內完成", totalOps, duration)
	t.Logf("平均每個操作: %v", duration/time.Duration(totalOps))
	
	// 這個測試展示了在高併發下，所有操作都被序列化到一個全域鎖
	// 造成效能嚴重下降
}