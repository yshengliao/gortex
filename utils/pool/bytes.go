package pool

import (
	"sync"
	"sync/atomic"
)

// ByteSlicePool manages pools of byte slices of different sizes
type ByteSlicePool struct {
	pools   []*sync.Pool
	sizes   []int
	metrics *ByteSliceMetrics
}

// ByteSliceMetrics tracks byte slice pool usage
type ByteSliceMetrics struct {
	// Per-size metrics
	SizeMetrics map[int]*SizeMetric
	mu          sync.RWMutex
}

// SizeMetric tracks metrics for a specific size
type SizeMetric struct {
	Size              int
	TotalGet          int64
	TotalPut          int64
	TotalNew          int64
	CurrentActive     int64
	TotalBytesWasted  int64 // When returned slice is smaller than pool size
}

// Common buffer sizes (powers of 2 for efficiency)
var defaultSizes = []int{
	512,        // 0.5KB
	1024,       // 1KB
	2048,       // 2KB
	4096,       // 4KB
	8192,       // 8KB
	16384,      // 16KB
	32768,      // 32KB
	65536,      // 64KB
	131072,     // 128KB
	262144,     // 256KB
	524288,     // 512KB
	1048576,    // 1MB
}

// NewByteSlicePool creates a new byte slice pool with default sizes
func NewByteSlicePool() *ByteSlicePool {
	return NewByteSlicePoolWithSizes(defaultSizes)
}

// NewByteSlicePoolWithSizes creates a new byte slice pool with custom sizes
func NewByteSlicePoolWithSizes(sizes []int) *ByteSlicePool {
	p := &ByteSlicePool{
		pools: make([]*sync.Pool, len(sizes)),
		sizes: make([]int, len(sizes)),
		metrics: &ByteSliceMetrics{
			SizeMetrics: make(map[int]*SizeMetric),
		},
	}
	
	// Copy and sort sizes
	copy(p.sizes, sizes)
	
	// Initialize pools and metrics
	for i, size := range p.sizes {
		size := size // Capture for closure
		idx := i     // Capture for closure
		
		p.metrics.SizeMetrics[size] = &SizeMetric{Size: size}
		
		p.pools[idx] = &sync.Pool{
			New: func() any {
				atomic.AddInt64(&p.metrics.SizeMetrics[size].TotalNew, 1)
				return make([]byte, size)
			},
		}
	}
	
	return p
}

// Get retrieves a byte slice that can hold at least n bytes
func (p *ByteSlicePool) Get(n int) []byte {
	if n <= 0 {
		return nil
	}
	
	// Find the appropriate pool
	idx := p.findPoolIndex(n)
	if idx < 0 {
		// Size too large, allocate directly
		return make([]byte, n)
	}
	
	size := p.sizes[idx]
	metric := p.metrics.SizeMetrics[size]
	
	atomic.AddInt64(&metric.TotalGet, 1)
	atomic.AddInt64(&metric.CurrentActive, 1)
	
	buf := p.pools[idx].Get().([]byte)
	return buf[:n] // Return slice of requested size
}

// Put returns a byte slice to the pool
func (p *ByteSlicePool) Put(buf []byte) {
	if buf == nil {
		return
	}
	
	size := cap(buf)
	
	// Find the matching pool
	idx := -1
	for i, poolSize := range p.sizes {
		if poolSize == size {
			idx = i
			break
		}
	}
	
	if idx < 0 {
		// Not from our pools
		return
	}
	
	metric := p.metrics.SizeMetrics[size]
	atomic.AddInt64(&metric.TotalPut, 1)
	atomic.AddInt64(&metric.CurrentActive, -1)
	
	// Track wasted bytes
	waste := int64(cap(buf) - len(buf))
	if waste > 0 {
		atomic.AddInt64(&metric.TotalBytesWasted, waste)
	}
	
	// Reset slice to full capacity before returning to pool
	buf = buf[:cap(buf)]
	p.pools[idx].Put(buf)
}

// GetExact retrieves a byte slice of exact size (may waste memory)
func (p *ByteSlicePool) GetExact(size int) []byte {
	// Find exact match
	for i, poolSize := range p.sizes {
		if poolSize == size {
			metric := p.metrics.SizeMetrics[size]
			atomic.AddInt64(&metric.TotalGet, 1)
			atomic.AddInt64(&metric.CurrentActive, 1)
			return p.pools[i].Get().([]byte)
		}
	}
	
	// No exact match, use Get
	return p.Get(size)
}

// findPoolIndex finds the smallest pool that can hold n bytes
func (p *ByteSlicePool) findPoolIndex(n int) int {
	for i, size := range p.sizes {
		if size >= n {
			return i
		}
	}
	return -1
}

// GetMetrics returns current pool metrics
func (p *ByteSlicePool) GetMetrics() map[int]SizeMetric {
	p.metrics.mu.RLock()
	defer p.metrics.mu.RUnlock()
	
	result := make(map[int]SizeMetric)
	for size, metric := range p.metrics.SizeMetrics {
		result[size] = SizeMetric{
			Size:             size,
			TotalGet:         atomic.LoadInt64(&metric.TotalGet),
			TotalPut:         atomic.LoadInt64(&metric.TotalPut),
			TotalNew:         atomic.LoadInt64(&metric.TotalNew),
			CurrentActive:    atomic.LoadInt64(&metric.CurrentActive),
			TotalBytesWasted: atomic.LoadInt64(&metric.TotalBytesWasted),
		}
	}
	
	return result
}

// DefaultByteSlicePool is a global byte slice pool instance
var DefaultByteSlicePool = NewByteSlicePool()

// GetBytes is a convenience function to get bytes from the default pool
func GetBytes(n int) []byte {
	return DefaultByteSlicePool.Get(n)
}

// PutBytes is a convenience function to return bytes to the default pool
func PutBytes(buf []byte) {
	DefaultByteSlicePool.Put(buf)
}