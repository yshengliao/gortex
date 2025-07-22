package pool

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

type TestObject struct {
	ID    int
	Name  string
	Data  []byte
}

func TestObjectPool(t *testing.T) {
	// Create pool with new and reset functions
	pool := NewObjectPool(
		func() *TestObject {
			return &TestObject{
				Data: make([]byte, 0, 1024),
			}
		},
		func(obj **TestObject) {
			(*obj).ID = 0
			(*obj).Name = ""
			(*obj).Data = (*obj).Data[:0]
		},
	)
	
	// Get object
	obj := pool.Get()
	assert.NotNil(t, obj)
	assert.Equal(t, 0, obj.ID)
	assert.Equal(t, "", obj.Name)
	
	// Use object
	obj.ID = 123
	obj.Name = "test"
	obj.Data = append(obj.Data, []byte("hello")...)
	
	// Return to pool
	pool.Put(obj)
	
	// Get another object (should be reset)
	obj2 := pool.Get()
	assert.NotNil(t, obj2)
	assert.Equal(t, 0, obj2.ID)
	assert.Equal(t, "", obj2.Name)
	assert.Equal(t, 0, len(obj2.Data))
	assert.Equal(t, 1024, cap(obj2.Data)) // Capacity preserved
	
	pool.Put(obj2)
	
	// Check metrics
	metrics := pool.GetMetrics()
	assert.Equal(t, int64(2), metrics.TotalGet)
	assert.Equal(t, int64(2), metrics.TotalPut)
	assert.Equal(t, int64(0), metrics.CurrentActive)
}

func TestObjectPoolNoReset(t *testing.T) {
	// Pool without reset function
	pool := NewObjectPool(
		func() *TestObject {
			return &TestObject{}
		},
		nil, // No reset
	)
	
	obj := pool.Get()
	obj.ID = 999
	pool.Put(obj)
	
	// Object not reset
	obj2 := pool.Get()
	// May or may not be the same object
	pool.Put(obj2)
}

func TestStructPool(t *testing.T) {
	// Create pool for TestObject
	pool := NewStructPool(&TestObject{})
	
	// Get object
	obj := pool.Get().(*TestObject)
	assert.NotNil(t, obj)
	assert.Equal(t, 0, obj.ID)
	assert.Equal(t, "", obj.Name)
	assert.Nil(t, obj.Data)
	
	// Use object
	obj.ID = 456
	obj.Name = "struct pool test"
	obj.Data = []byte("data")
	
	// Return to pool
	pool.Put(obj)
	
	// Get another object (should be reset)
	obj2 := pool.Get().(*TestObject)
	assert.NotNil(t, obj2)
	assert.Equal(t, 0, obj2.ID)
	assert.Equal(t, "", obj2.Name)
	assert.Nil(t, obj2.Data)
	
	pool.Put(obj2)
	
	// Check metrics
	metrics := pool.GetMetrics()
	assert.Equal(t, int64(2), metrics.TotalGet)
	assert.Equal(t, int64(2), metrics.TotalPut)
}

func TestStructPoolNilHandling(t *testing.T) {
	pool := NewStructPool(&TestObject{})
	
	// Put nil should not panic
	assert.NotPanics(t, func() {
		pool.Put(nil)
	})
	
	// Put non-pointer should not panic
	assert.NotPanics(t, func() {
		pool.Put(TestObject{})
	})
}

func TestObjectPoolConcurrency(t *testing.T) {
	pool := NewObjectPool(
		func() *TestObject {
			return &TestObject{
				Data: make([]byte, 0, 256),
			}
		},
		func(obj **TestObject) {
			(*obj).ID = 0
			(*obj).Name = ""
			(*obj).Data = (*obj).Data[:0]
		},
	)
	
	var wg sync.WaitGroup
	numGoroutines := 50
	numOperations := 100
	
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			for j := 0; j < numOperations; j++ {
				obj := pool.Get()
				obj.ID = id*1000 + j
				obj.Name = "concurrent"
				obj.Data = append(obj.Data, byte(id), byte(j))
				pool.Put(obj)
			}
		}(i)
	}
	
	wg.Wait()
	
	metrics := pool.GetMetrics()
	expectedOps := int64(numGoroutines * numOperations)
	assert.Equal(t, expectedOps, metrics.TotalGet)
	assert.Equal(t, expectedOps, metrics.TotalPut)
	assert.Equal(t, int64(0), metrics.CurrentActive)
}

// Complex object for benchmarking
type ComplexObject struct {
	ID       int64
	Name     string
	Tags     []string
	Metadata map[string]interface{}
	Buffer   []byte
}

func BenchmarkObjectPool(b *testing.B) {
	pool := NewObjectPool(
		func() *ComplexObject {
			return &ComplexObject{
				Tags:     make([]string, 0, 10),
				Metadata: make(map[string]interface{}),
				Buffer:   make([]byte, 0, 1024),
			}
		},
		func(obj **ComplexObject) {
			(*obj).ID = 0
			(*obj).Name = ""
			(*obj).Tags = (*obj).Tags[:0]
			// Clear map
			for k := range (*obj).Metadata {
				delete((*obj).Metadata, k)
			}
			(*obj).Buffer = (*obj).Buffer[:0]
		},
	)
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		obj := pool.Get()
		obj.ID = int64(i)
		obj.Name = "benchmark"
		obj.Tags = append(obj.Tags, "tag1", "tag2")
		obj.Metadata["key"] = "value"
		obj.Buffer = append(obj.Buffer, []byte("data")...)
		pool.Put(obj)
	}
}

func BenchmarkStructPool(b *testing.B) {
	pool := NewStructPool(&ComplexObject{})
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		obj := pool.Get().(*ComplexObject)
		obj.ID = int64(i)
		obj.Name = "benchmark"
		pool.Put(obj)
	}
}

func BenchmarkNoPool(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		obj := &ComplexObject{
			Tags:     make([]string, 0, 10),
			Metadata: make(map[string]interface{}),
			Buffer:   make([]byte, 0, 1024),
		}
		obj.ID = int64(i)
		obj.Name = "benchmark"
	}
}