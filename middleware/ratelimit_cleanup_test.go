package middleware_test

import (
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/yshengliao/gortex/middleware"
)

func TestMemoryStore_Cleanup(t *testing.T) {
	// Create store with short TTL and cleanup interval for testing
	config := &middleware.MemoryStoreConfig{
		Rate:            10,
		Burst:           20,
		CleanupInterval: 100 * time.Millisecond,
		TTL:             200 * time.Millisecond,
	}
	store := middleware.NewMemoryStoreWithConfig(config)
	defer store.Stop()

	// Create some limiters
	keys := []string{"key1", "key2", "key3"}
	for _, key := range keys {
		assert.True(t, store.Allow(key))
	}

	// Verify all limiters exist
	assert.Equal(t, 3, store.Size())

	// Wait for TTL to expire
	time.Sleep(300 * time.Millisecond)

	// Wait for cleanup to run
	time.Sleep(150 * time.Millisecond)

	// All limiters should be cleaned up
	assert.Equal(t, 0, store.Size())

	// Create new limiter and access it to keep it alive
	assert.True(t, store.Allow("key4"))
	assert.Equal(t, 1, store.Size())

	// Keep accessing it to prevent cleanup
	for i := 0; i < 3; i++ {
		time.Sleep(150 * time.Millisecond)
		assert.True(t, store.Allow("key4"))
	}

	// It should still exist
	assert.Equal(t, 1, store.Size())
}

func TestMemoryStore_CleanupDoesNotAffectActiveKeys(t *testing.T) {
	config := &middleware.MemoryStoreConfig{
		Rate:            10,
		Burst:           20,
		CleanupInterval: 50 * time.Millisecond,
		TTL:             100 * time.Millisecond,
	}
	store := middleware.NewMemoryStoreWithConfig(config)
	defer store.Stop()

	// Create limiters
	activeKey := "active"
	inactiveKey := "inactive"

	assert.True(t, store.Allow(activeKey))
	assert.True(t, store.Allow(inactiveKey))
	assert.Equal(t, 2, store.Size())

	// Keep one key active
	ticker := time.NewTicker(30 * time.Millisecond)
	defer ticker.Stop()

	done := time.After(200 * time.Millisecond)
	for {
		select {
		case <-ticker.C:
			store.Allow(activeKey)
		case <-done:
			// Wait for final cleanup
			time.Sleep(60 * time.Millisecond)

			// Active key should remain, inactive should be cleaned
			assert.Equal(t, 1, store.Size())
			assert.True(t, store.Allow(activeKey))
			return
		}
	}
}

func TestMemoryStore_MemoryStability(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory stability test in short mode")
	}

	config := &middleware.MemoryStoreConfig{
		Rate:            100,
		Burst:           200,
		CleanupInterval: 100 * time.Millisecond,
		TTL:             500 * time.Millisecond,
	}
	store := middleware.NewMemoryStoreWithConfig(config)
	defer store.Stop()

	// Force GC and get baseline
	runtime.GC()
	runtime.GC()
	var baseline runtime.MemStats
	runtime.ReadMemStats(&baseline)

	// Simulate traffic with client churn
	clientID := 0
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	done := time.After(3 * time.Second)
	maxSize := 0

	for {
		select {
		case <-ticker.C:
			// Create new clients
			for i := 0; i < 10; i++ {
				clientID++
				key := fmt.Sprintf("client-%d", clientID)
				store.Allow(key)
			}

			// Track max size
			currentSize := store.Size()
			if currentSize > maxSize {
				maxSize = currentSize
			}

		case <-done:
			// Wait for final cleanup
			time.Sleep(1 * time.Second)

			// Check final state
			finalSize := store.Size()
			t.Logf("Created %d clients total", clientID)
			t.Logf("Max concurrent limiters: %d", maxSize)
			t.Logf("Final limiter count: %d", finalSize)

			// Memory should be stable
			runtime.GC()
			runtime.GC()
			var final runtime.MemStats
			runtime.ReadMemStats(&final)

			memGrowth := float64(final.Alloc-baseline.Alloc) / 1024 / 1024
			t.Logf("Memory growth: %.2f MB", memGrowth)

			// Final size should be much less than total created (due to cleanup)
			assert.Less(t, finalSize, clientID/10, "Most limiters should be cleaned up")

			// Memory growth should be minimal
			assert.Less(t, memGrowth, 10.0, "Memory growth should be controlled")
			return
		}
	}
}

func TestMemoryStore_ConcurrentCleanup(t *testing.T) {
	config := &middleware.MemoryStoreConfig{
		Rate:            10,
		Burst:           20,
		CleanupInterval: 10 * time.Millisecond,
		TTL:             50 * time.Millisecond,
	}
	store := middleware.NewMemoryStoreWithConfig(config)
	defer store.Stop()

	// Stress test with concurrent operations during cleanup
	var wg sync.WaitGroup
	stop := make(chan struct{})

	// Writer goroutines
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					key := fmt.Sprintf("writer-%d-%d", id, time.Now().UnixNano())
					store.Allow(key)
					time.Sleep(time.Millisecond)
				}
			}
		}(i)
	}

	// Reader goroutines
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			keys := make([]string, 10)
			for j := range keys {
				keys[j] = fmt.Sprintf("reader-%d-%d", id, j)
			}

			for {
				select {
				case <-stop:
					return
				default:
					for _, key := range keys {
						store.Allow(key)
					}
					time.Sleep(5 * time.Millisecond)
				}
			}
		}(i)
	}

	// Let it run
	time.Sleep(500 * time.Millisecond)

	// Stop all goroutines
	close(stop)
	wg.Wait()

	// Should not panic and size should be reasonable
	size := store.Size()
	t.Logf("Final size after concurrent operations: %d", size)
	assert.Less(t, size, 1000, "Size should be controlled by cleanup")
}

func TestMemoryStore_StopCleanup(t *testing.T) {
	config := &middleware.MemoryStoreConfig{
		Rate:            10,
		Burst:           20,
		CleanupInterval: 50 * time.Millisecond,
		TTL:             100 * time.Millisecond,
	}
	store := middleware.NewMemoryStoreWithConfig(config)

	// Create some limiters
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("key-%d", i)
		assert.True(t, store.Allow(key))
	}
	assert.Equal(t, 5, store.Size())

	// Stop the store
	store.Stop()

	// Wait beyond TTL
	time.Sleep(200 * time.Millisecond)

	// Size should remain the same (no cleanup after stop)
	assert.Equal(t, 5, store.Size())
}

func TestMemoryStore_StopTwice(t *testing.T) {
	config := &middleware.MemoryStoreConfig{
		Rate:            10,
		Burst:           20,
		CleanupInterval: 10 * time.Millisecond,
		TTL:             20 * time.Millisecond,
	}
	store := middleware.NewMemoryStoreWithConfig(config)

	// Calling Stop twice should not panic
	store.Stop()
	store.Stop()
}

func BenchmarkMemoryStore_WithCleanup(b *testing.B) {
	config := &middleware.MemoryStoreConfig{
		Rate:            1000,
		Burst:           2000,
		CleanupInterval: 100 * time.Millisecond,
		TTL:             1 * time.Second,
	}
	store := middleware.NewMemoryStoreWithConfig(config)
	defer store.Stop()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("bench-key-%d", i%10000)
			store.Allow(key)
			i++
		}
	})

	b.StopTimer()
	b.Logf("Final store size: %d", store.Size())
}
