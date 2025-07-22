package pool

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestByteSlicePool(t *testing.T) {
	pool := NewByteSlicePool()
	
	// Test various sizes
	sizes := []int{100, 500, 1000, 5000, 10000}
	
	for _, size := range sizes {
		buf := pool.Get(size)
		assert.NotNil(t, buf)
		assert.Equal(t, size, len(buf))
		assert.True(t, cap(buf) >= size)
		
		// Write some data
		for i := 0; i < size; i++ {
			buf[i] = byte(i % 256)
		}
		
		pool.Put(buf)
	}
	
	// Check metrics
	metrics := pool.GetMetrics()
	assert.NotEmpty(t, metrics)
}

func TestByteSlicePoolExactSize(t *testing.T) {
	sizes := []int{512, 1024, 2048}
	pool := NewByteSlicePoolWithSizes(sizes)
	
	// Get exact size
	buf := pool.GetExact(1024)
	assert.NotNil(t, buf)
	assert.Equal(t, 1024, len(buf))
	assert.Equal(t, 1024, cap(buf))
	
	pool.Put(buf)
	
	// Get non-exact size
	buf2 := pool.GetExact(1000)
	assert.NotNil(t, buf2)
	assert.Equal(t, 1000, len(buf2))
	assert.True(t, cap(buf2) >= 1000)
}

func TestByteSlicePoolLargeSize(t *testing.T) {
	pool := NewByteSlicePool()
	
	// Request larger than largest pool size
	largeSize := 10 * 1024 * 1024 // 10MB
	buf := pool.Get(largeSize)
	assert.NotNil(t, buf)
	assert.Equal(t, largeSize, len(buf))
	
	// Put won't add to pool
	pool.Put(buf)
	
	metrics := pool.GetMetrics()
	// No metrics for this size
	for _, metric := range metrics {
		if metric.Size > 1024*1024 {
			assert.Equal(t, int64(0), metric.TotalGet)
		}
	}
}

func TestByteSlicePoolWastedBytes(t *testing.T) {
	pool := NewByteSlicePool()
	
	// Get 100 bytes (will get from 512 pool)
	buf := pool.Get(100)
	assert.Equal(t, 100, len(buf))
	assert.True(t, cap(buf) >= 512)
	
	// Return with original length
	pool.Put(buf)
	
	// Check wasted bytes
	metrics := pool.GetMetrics()
	if metric, ok := metrics[512]; ok {
		assert.True(t, metric.TotalBytesWasted > 0)
	}
}

func TestByteSlicePoolConcurrency(t *testing.T) {
	pool := NewByteSlicePool()
	
	var wg sync.WaitGroup
	numGoroutines := 50
	numOperations := 100
	
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			for j := 0; j < numOperations; j++ {
				size := 100 + (j % 1000)
				buf := pool.Get(size)
				
				// Use the buffer
				for k := 0; k < len(buf); k++ {
					buf[k] = byte(k)
				}
				
				pool.Put(buf)
			}
		}(i)
	}
	
	wg.Wait()
	
	// Verify no active buffers
	metrics := pool.GetMetrics()
	for _, metric := range metrics {
		assert.Equal(t, int64(0), metric.CurrentActive)
		assert.Equal(t, metric.TotalGet, metric.TotalPut)
	}
}

func TestDefaultByteSlicePool(t *testing.T) {
	// Use convenience functions
	buf := GetBytes(256)
	assert.NotNil(t, buf)
	assert.Equal(t, 256, len(buf))
	
	PutBytes(buf)
	
	// Verify it's working
	metrics := DefaultByteSlicePool.GetMetrics()
	assert.NotEmpty(t, metrics)
}

func BenchmarkByteSlicePool(b *testing.B) {
	pool := NewByteSlicePool()
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		buf := pool.Get(1024)
		// Simulate some work
		for j := 0; j < len(buf); j++ {
			buf[j] = byte(j)
		}
		pool.Put(buf)
	}
}

func BenchmarkByteSlicePoolVariedSizes(b *testing.B) {
	pool := NewByteSlicePool()
	sizes := []int{64, 256, 1024, 4096, 16384}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		size := sizes[i%len(sizes)]
		buf := pool.Get(size)
		pool.Put(buf)
	}
}

func BenchmarkByteSliceNoPool(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		buf := make([]byte, 1024)
		// Simulate some work
		for j := 0; j < len(buf); j++ {
			buf[j] = byte(j)
		}
	}
}