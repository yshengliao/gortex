package middleware_test

import (
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/yshengliao/gortex/middleware"
)

func TestMemoryStore_MemoryLeak(t *testing.T) {
	// This test demonstrates the memory leak issue
	// The MemoryStore creates limiters but never removes them
	store := middleware.NewMemoryStore(10, 20)
	defer store.Stop()

	// Force garbage collection and get initial memory stats
	runtime.GC()
	runtime.GC()
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	// Create many unique keys to simulate different clients
	numKeys := 10000
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("client-%d", i)
		store.Allow(key)
	}

	// Force garbage collection and get memory stats after creating limiters
	runtime.GC()
	runtime.GC()
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)

	// Calculate memory growth
	memoryGrowth := m2.Alloc - m1.Alloc
	avgMemoryPerLimiter := float64(memoryGrowth) / float64(numKeys)

	t.Logf("Memory growth after creating %d limiters: %d bytes", numKeys, memoryGrowth)
	t.Logf("Average memory per limiter: %.2f bytes", avgMemoryPerLimiter)
	t.Logf("Projected memory for 1M limiters: %.2f MB", avgMemoryPerLimiter*1000000/1024/1024)

	// The memory should not grow indefinitely
	// Each limiter takes some memory, and without cleanup, this grows forever
	assert.Greater(t, memoryGrowth, uint64(numKeys*50), "Memory should grow with each new limiter")
}

func TestMemoryStore_CleanupFixed(t *testing.T) {
	// This test verifies that limiters ARE now cleaned up properly
	config := &middleware.MemoryStoreConfig{
		Rate:            1,
		Burst:           1,
		CleanupInterval: 100 * time.Millisecond,
		TTL:             200 * time.Millisecond,
	}
	store := middleware.NewMemoryStoreWithConfig(config)
	defer store.Stop()

	// Create limiters for different keys
	keys := []string{"key1", "key2", "key3"}
	for _, key := range keys {
		// Use up the burst limit
		store.Allow(key)
		assert.False(t, store.Allow(key), "Second request should be rate limited")
	}

	// Wait for TTL to expire and cleanup to run
	time.Sleep(350 * time.Millisecond)

	// Try the same keys again - they should now be allowed (cleaned up)
	for _, key := range keys {
		assert.True(t, store.Allow(key), "Key should be allowed after cleanup")
	}
}

func TestMemoryStore_LongRunningSimulation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long-running test in short mode")
	}

	store := middleware.NewMemoryStore(10, 20)
	defer store.Stop()

	// Simulate a service that receives requests from many different IPs
	// over an extended period
	duration := 10 * time.Second
	start := time.Now()
	clientCounter := 0

	// Track memory growth over time
	initialMem := getMemoryUsage()
	t.Logf("Initial memory usage: %.2f MB", initialMem)

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	done := time.After(duration)
	for {
		select {
		case <-done:
			finalMem := getMemoryUsage()
			memGrowth := finalMem - initialMem
			t.Logf("Final memory usage: %.2f MB", finalMem)
			t.Logf("Memory growth: %.2f MB", memGrowth)
			t.Logf("Total unique clients: %d", clientCounter)
			t.Logf("Average memory per client: %.2f KB", memGrowth*1024/float64(clientCounter))

			// Memory should not grow indefinitely
			// Without cleanup, each new client adds to memory usage
			assert.Greater(t, memGrowth, 0.1, "Memory should grow without cleanup")
			return

		case <-ticker.C:
			// Simulate new clients every second
			for i := 0; i < 100; i++ {
				clientCounter++
				key := fmt.Sprintf("client-%d-%d", time.Now().Unix(), clientCounter)
				store.Allow(key)
			}
			
			currentMem := getMemoryUsage()
			elapsed := time.Since(start).Seconds()
			t.Logf("[%.0fs] Memory: %.2f MB, Clients: %d", elapsed, currentMem, clientCounter)
		}
	}
}

func getMemoryUsage() float64 {
	runtime.GC()
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return float64(m.Alloc) / 1024 / 1024
}

func BenchmarkMemoryStore_MemoryUsage(b *testing.B) {
	store := middleware.NewMemoryStore(100, 200)
	defer store.Stop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("bench-key-%d", i)
		store.Allow(key)
	}

	b.StopTimer()
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	b.ReportMetric(float64(m.Alloc)/float64(b.N), "bytes/op")
}