package pool

import (
	"bytes"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBufferPool(t *testing.T) {
	pool := NewBufferPool()
	
	// Get and put buffers
	buf1 := pool.Get()
	assert.NotNil(t, buf1)
	assert.Equal(t, 0, buf1.Len())
	
	// Write some data
	buf1.WriteString("Hello, World!")
	assert.Equal(t, 13, buf1.Len())
	
	// Return to pool
	pool.Put(buf1)
	
	// Get another buffer (should be reset)
	buf2 := pool.Get()
	assert.NotNil(t, buf2)
	assert.Equal(t, 0, buf2.Len())
	
	// Check if it's the same buffer (reused)
	buf2.WriteString("Test")
	pool.Put(buf2)
	
	// Check metrics
	metrics := pool.GetMetrics()
	assert.Equal(t, int64(2), metrics.TotalGet)
	assert.Equal(t, int64(2), metrics.TotalPut)
	assert.True(t, metrics.TotalNew <= 2) // May reuse
	assert.Equal(t, int64(0), metrics.CurrentActive)
}

func TestBufferPoolLargeBuffer(t *testing.T) {
	pool := NewBufferPool()
	
	// Get a buffer and grow it beyond the limit
	buf := pool.Get()
	largeData := make([]byte, 2*1024*1024) // 2MB
	buf.Write(largeData)
	
	// Return to pool (should not be pooled)
	pool.Put(buf)
	
	// Get another buffer
	buf2 := pool.Get()
	assert.True(t, buf2.Cap() < 2*1024*1024) // Should be a new smaller buffer
	pool.Put(buf2)
	
	metrics := pool.GetMetrics()
	assert.Equal(t, int64(2), metrics.TotalGet)
	assert.Equal(t, int64(2), metrics.TotalPut)
	assert.True(t, metrics.LargestBuffer >= 2*1024*1024)
}

func TestBufferPoolConcurrency(t *testing.T) {
	pool := NewBufferPool()
	
	var wg sync.WaitGroup
	numGoroutines := 100
	numOperations := 1000
	
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			for j := 0; j < numOperations; j++ {
				buf := pool.Get()
				buf.WriteString("test data")
				pool.Put(buf)
			}
		}(i)
	}
	
	wg.Wait()
	
	metrics := pool.GetMetrics()
	expectedOps := int64(numGoroutines * numOperations)
	assert.Equal(t, expectedOps, metrics.TotalGet)
	assert.Equal(t, expectedOps, metrics.TotalPut)
	assert.Equal(t, int64(0), metrics.CurrentActive)
	
	// High reuse rate expected
	assert.True(t, metrics.ReuseRate > 0.9)
}

func TestBufferPoolNilHandling(t *testing.T) {
	pool := NewBufferPool()
	
	// Put nil should not panic
	assert.NotPanics(t, func() {
		pool.Put(nil)
	})
	
	metrics := pool.GetMetrics()
	assert.Equal(t, int64(0), metrics.TotalPut)
}

func TestDefaultBufferPool(t *testing.T) {
	// Reset metrics for clean test
	DefaultBufferPool.ResetMetrics()
	
	// Use convenience functions
	buf := GetBuffer()
	assert.NotNil(t, buf)
	
	buf.WriteString("test")
	PutBuffer(buf)
	
	metrics := DefaultBufferPool.GetMetrics()
	assert.Equal(t, int64(1), metrics.TotalGet)
	assert.Equal(t, int64(1), metrics.TotalPut)
}

func BenchmarkBufferPool(b *testing.B) {
	pool := NewBufferPool()
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		buf := pool.Get()
		buf.WriteString("benchmark test data")
		pool.Put(buf)
	}
}

func BenchmarkBufferPoolParallel(b *testing.B) {
	pool := NewBufferPool()
	
	b.ResetTimer()
	b.ReportAllocs()
	
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buf := pool.Get()
			buf.WriteString("benchmark test data")
			pool.Put(buf)
		}
	})
}

func BenchmarkBufferNoPool(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		buf := new(bytes.Buffer)
		buf.Grow(1024)
		buf.WriteString("benchmark test data")
	}
}