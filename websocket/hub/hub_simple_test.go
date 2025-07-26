package hub_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"github.com/yshengliao/gortex/websocket/hub"
)

// TestHub_ConcurrentOperations verifies the hub works correctly without mutex
func TestHub_ConcurrentOperations(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	h := hub.NewHub(logger)
	
	// Start hub
	go h.Run()
	defer h.Shutdown()
	
	// Wait for hub to start
	time.Sleep(10 * time.Millisecond)
	
	var wg sync.WaitGroup
	
	// Test concurrent broadcasts
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			msg := &hub.Message{
				Type: "test",
				Data: map[string]interface{}{
					"id": id,
				},
			}
			h.Broadcast(msg)
		}(i)
	}
	
	// Test concurrent client count checks
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = h.GetConnectedClients()
		}()
	}
	
	wg.Wait()
	
	// Should complete without panic or deadlock
	assert.Equal(t, 0, h.GetConnectedClients())
}

// TestHub_NoRaceCondition uses the race detector to verify thread safety
func TestHub_NoRaceCondition(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	h := hub.NewHub(logger)
	
	// Start hub
	go h.Run()
	defer h.Shutdown()
	
	// Stress test with concurrent operations
	var wg sync.WaitGroup
	
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			// Rapid fire operations
			for j := 0; j < 100; j++ {
				// Broadcast
				h.Broadcast(&hub.Message{
					Type: "test",
					Data: map[string]interface{}{
						"from": id,
						"seq":  j,
					},
				})
				
				// Send to user
				h.SendToUser("user123", &hub.Message{
					Type: "private",
					Data: map[string]interface{}{
						"from": id,
					},
				})
				
				// Get count
				_ = h.GetConnectedClients()
			}
		}(i)
	}
	
	wg.Wait()
}

// TestHub_Performance benchmarks operations without mutex overhead
func TestHub_Performance(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	h := hub.NewHub(logger)
	
	// Start hub
	go h.Run()
	defer h.Shutdown()
	
	// Benchmark broadcast operations
	start := time.Now()
	const numOps = 10000
	
	for i := 0; i < numOps; i++ {
		h.Broadcast(&hub.Message{
			Type: "perf",
			Data: map[string]interface{}{"i": i},
		})
	}
	
	elapsed := time.Since(start)
	opsPerSec := float64(numOps) / elapsed.Seconds()
	
	t.Logf("Operations: %d", numOps)
	t.Logf("Elapsed: %v", elapsed)
	t.Logf("Ops/sec: %.0f", opsPerSec)
	
	// Should be very fast without mutex overhead
	assert.Greater(t, opsPerSec, float64(100000), "Should handle > 100k ops/sec")
}

// TestHub_StressTest ensures no panics under heavy load
func TestHub_StressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}
	
	logger, _ := zap.NewDevelopment()
	h := hub.NewHub(logger)
	
	// Start hub
	go h.Run()
	defer h.Shutdown()
	
	var panics int32
	var wg sync.WaitGroup
	
	// Function to catch panics
	safeDo := func(fn func()) {
		defer func() {
			if r := recover(); r != nil {
				atomic.AddInt32(&panics, 1)
				t.Logf("Panic caught: %v", r)
			}
		}()
		fn()
	}
	
	// Hammer the hub with operations
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			for j := 0; j < 1000; j++ {
				safeDo(func() {
					h.Broadcast(&hub.Message{
						Type: "stress",
						Data: map[string]interface{}{
							"worker": id,
							"seq":    j,
						},
					})
				})
				
				safeDo(func() {
					_ = h.GetConnectedClients()
				})
				
				// Occasional targeted message
				if j%10 == 0 {
					safeDo(func() {
						h.SendToUser("stress-user", &hub.Message{
							Type: "targeted",
							Data: map[string]interface{}{
								"worker": id,
							},
						})
					})
				}
			}
		}(i)
	}
	
	wg.Wait()
	assert.Equal(t, int32(0), atomic.LoadInt32(&panics), "No panics should occur")
}

// TestHub_ShutdownSafety verifies graceful shutdown
func TestHub_ShutdownSafety(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	h := hub.NewHub(logger)
	
	// Start hub
	go h.Run()
	
	// Start some background operations
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 1000; i++ {
			h.Broadcast(&hub.Message{
				Type: "shutdown-test",
				Data: map[string]interface{}{"i": i},
			})
			time.Sleep(time.Microsecond)
		}
	}()
	
	// Shutdown while operations are ongoing
	time.Sleep(10 * time.Millisecond)
	h.Shutdown()
	
	// Wait for background operations to complete
	select {
	case <-done:
		// Good, operations completed
	case <-time.After(1 * time.Second):
		t.Fatal("Background operations did not complete")
	}
	
	// Further operations should not panic
	assert.NotPanics(t, func() {
		h.Broadcast(&hub.Message{Type: "after-shutdown"})
		_ = h.GetConnectedClients()
	})
}

// BenchmarkHub_Broadcast benchmarks broadcast performance
func BenchmarkHub_Broadcast(b *testing.B) {
	logger, _ := zap.NewDevelopment()
	h := hub.NewHub(logger)
	
	// Start hub
	go h.Run()
	defer h.Shutdown()
	
	msg := &hub.Message{
		Type: "benchmark",
		Data: map[string]interface{}{
			"content": "benchmark message",
		},
	}
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			h.Broadcast(msg)
		}
	})
}

// BenchmarkHub_GetConnectedClients benchmarks client count retrieval
func BenchmarkHub_GetConnectedClients(b *testing.B) {
	logger, _ := zap.NewDevelopment()
	h := hub.NewHub(logger)
	
	// Start hub
	go h.Run()
	defer h.Shutdown()
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = h.GetConnectedClients()
		}
	})
}