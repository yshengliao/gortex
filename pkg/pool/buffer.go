package pool

import (
	"bytes"
	"sync"
	"sync/atomic"
)

// BufferPool manages a pool of reusable byte buffers
type BufferPool struct {
	pool    *sync.Pool
	metrics *BufferMetrics
}

// BufferMetrics tracks buffer pool usage
type BufferMetrics struct {
	// Pool statistics
	TotalGet      int64
	TotalPut      int64
	TotalNew      int64
	CurrentActive int64
	
	// Size statistics
	TotalBytesAllocated int64
	LargestBuffer       int64
	
	// Reuse statistics
	ReuseRate float64 // Calculated on demand
}

// NewBufferPool creates a new buffer pool
func NewBufferPool() *BufferPool {
	bp := &BufferPool{
		metrics: &BufferMetrics{},
	}
	
	bp.pool = &sync.Pool{
		New: func() any {
			atomic.AddInt64(&bp.metrics.TotalNew, 1)
			buf := new(bytes.Buffer)
			// Pre-allocate some capacity to reduce allocations
			buf.Grow(1024)
			atomic.AddInt64(&bp.metrics.TotalBytesAllocated, 1024)
			return buf
		},
	}
	
	return bp
}

// Get retrieves a buffer from the pool
func (bp *BufferPool) Get() *bytes.Buffer {
	atomic.AddInt64(&bp.metrics.TotalGet, 1)
	atomic.AddInt64(&bp.metrics.CurrentActive, 1)
	
	buf := bp.pool.Get().(*bytes.Buffer)
	buf.Reset()
	return buf
}

// Put returns a buffer to the pool
func (bp *BufferPool) Put(buf *bytes.Buffer) {
	if buf == nil {
		return
	}
	
	atomic.AddInt64(&bp.metrics.TotalPut, 1)
	atomic.AddInt64(&bp.metrics.CurrentActive, -1)
	
	// Track largest buffer
	size := int64(buf.Cap())
	for {
		current := atomic.LoadInt64(&bp.metrics.LargestBuffer)
		if size <= current || atomic.CompareAndSwapInt64(&bp.metrics.LargestBuffer, current, size) {
			break
		}
	}
	
	// Don't pool extremely large buffers
	if buf.Cap() > 1024*1024 { // 1MB
		return
	}
	
	bp.pool.Put(buf)
}

// GetMetrics returns current pool metrics
func (bp *BufferPool) GetMetrics() BufferMetrics {
	metrics := BufferMetrics{
		TotalGet:            atomic.LoadInt64(&bp.metrics.TotalGet),
		TotalPut:            atomic.LoadInt64(&bp.metrics.TotalPut),
		TotalNew:            atomic.LoadInt64(&bp.metrics.TotalNew),
		CurrentActive:       atomic.LoadInt64(&bp.metrics.CurrentActive),
		TotalBytesAllocated: atomic.LoadInt64(&bp.metrics.TotalBytesAllocated),
		LargestBuffer:       atomic.LoadInt64(&bp.metrics.LargestBuffer),
	}
	
	// Calculate reuse rate
	if metrics.TotalGet > 0 {
		metrics.ReuseRate = float64(metrics.TotalGet-metrics.TotalNew) / float64(metrics.TotalGet)
	}
	
	return metrics
}

// Reset resets the metrics (useful for testing)
func (bp *BufferPool) ResetMetrics() {
	atomic.StoreInt64(&bp.metrics.TotalGet, 0)
	atomic.StoreInt64(&bp.metrics.TotalPut, 0)
	atomic.StoreInt64(&bp.metrics.TotalNew, 0)
	atomic.StoreInt64(&bp.metrics.CurrentActive, 0)
	atomic.StoreInt64(&bp.metrics.TotalBytesAllocated, 0)
	atomic.StoreInt64(&bp.metrics.LargestBuffer, 0)
}

// DefaultBufferPool is a global buffer pool instance
var DefaultBufferPool = NewBufferPool()

// GetBuffer is a convenience function to get a buffer from the default pool
func GetBuffer() *bytes.Buffer {
	return DefaultBufferPool.Get()
}

// PutBuffer is a convenience function to return a buffer to the default pool
func PutBuffer(buf *bytes.Buffer) {
	DefaultBufferPool.Put(buf)
}