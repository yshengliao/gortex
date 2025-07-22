package pool

import (
	"reflect"
	"sync"
	"sync/atomic"
)

// ObjectPool is a generic object pool
type ObjectPool[T any] struct {
	pool    *sync.Pool
	new     func() T
	reset   func(*T)
	metrics *ObjectMetrics
}

// ObjectMetrics tracks object pool usage
type ObjectMetrics struct {
	TotalGet      int64
	TotalPut      int64
	TotalNew      int64
	CurrentActive int64
}

// NewObjectPool creates a new object pool
func NewObjectPool[T any](new func() T, reset func(*T)) *ObjectPool[T] {
	p := &ObjectPool[T]{
		new:     new,
		reset:   reset,
		metrics: &ObjectMetrics{},
	}
	
	p.pool = &sync.Pool{
		New: func() interface{} {
			atomic.AddInt64(&p.metrics.TotalNew, 1)
			return p.new()
		},
	}
	
	return p
}

// Get retrieves an object from the pool
func (p *ObjectPool[T]) Get() T {
	atomic.AddInt64(&p.metrics.TotalGet, 1)
	atomic.AddInt64(&p.metrics.CurrentActive, 1)
	
	obj := p.pool.Get().(T)
	return obj
}

// Put returns an object to the pool
func (p *ObjectPool[T]) Put(obj T) {
	atomic.AddInt64(&p.metrics.TotalPut, 1)
	atomic.AddInt64(&p.metrics.CurrentActive, -1)
	
	// Reset object if reset function provided
	if p.reset != nil {
		p.reset(&obj)
	}
	
	p.pool.Put(obj)
}

// GetMetrics returns current pool metrics
func (p *ObjectPool[T]) GetMetrics() ObjectMetrics {
	return ObjectMetrics{
		TotalGet:      atomic.LoadInt64(&p.metrics.TotalGet),
		TotalPut:      atomic.LoadInt64(&p.metrics.TotalPut),
		TotalNew:      atomic.LoadInt64(&p.metrics.TotalNew),
		CurrentActive: atomic.LoadInt64(&p.metrics.CurrentActive),
	}
}

// StructPool manages a pool of struct pointers with automatic reset
type StructPool struct {
	pool       *sync.Pool
	structType reflect.Type
	zeroValue  interface{}
	metrics    *ObjectMetrics
}

// NewStructPool creates a pool for a specific struct type
func NewStructPool(example interface{}) *StructPool {
	structType := reflect.TypeOf(example)
	if structType.Kind() == reflect.Ptr {
		structType = structType.Elem()
	}
	
	sp := &StructPool{
		structType: structType,
		zeroValue:  reflect.Zero(structType).Interface(),
		metrics:    &ObjectMetrics{},
	}
	
	sp.pool = &sync.Pool{
		New: func() interface{} {
			atomic.AddInt64(&sp.metrics.TotalNew, 1)
			return reflect.New(sp.structType).Interface()
		},
	}
	
	return sp
}

// Get retrieves a struct pointer from the pool
func (sp *StructPool) Get() interface{} {
	atomic.AddInt64(&sp.metrics.TotalGet, 1)
	atomic.AddInt64(&sp.metrics.CurrentActive, 1)
	
	return sp.pool.Get()
}

// Put returns a struct pointer to the pool and resets it
func (sp *StructPool) Put(obj interface{}) {
	if obj == nil {
		return
	}
	
	atomic.AddInt64(&sp.metrics.TotalPut, 1)
	atomic.AddInt64(&sp.metrics.CurrentActive, -1)
	
	// Reset struct to zero value
	v := reflect.ValueOf(obj)
	if v.Kind() == reflect.Ptr && !v.IsNil() {
		v.Elem().Set(reflect.Zero(sp.structType))
	}
	
	sp.pool.Put(obj)
}

// GetMetrics returns current pool metrics
func (sp *StructPool) GetMetrics() ObjectMetrics {
	return ObjectMetrics{
		TotalGet:      atomic.LoadInt64(&sp.metrics.TotalGet),
		TotalPut:      atomic.LoadInt64(&sp.metrics.TotalPut),
		TotalNew:      atomic.LoadInt64(&sp.metrics.TotalNew),
		CurrentActive: atomic.LoadInt64(&sp.metrics.CurrentActive),
	}
}