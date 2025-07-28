package http

import (
	"net/http"
	"sync"
)

// contextPool is a pool of contexts to reduce allocations
var contextPool = sync.Pool{
	New: func() interface{} {
		return &DefaultContext{
			store: make(Map),
		}
	},
}

// AcquireContext gets a context from the pool
func AcquireContext(r *http.Request, w http.ResponseWriter) Context {
	c := contextPool.Get().(*DefaultContext)
	c.Reset(r, w)
	return c
}

// ReleaseContext returns a context to the pool
func ReleaseContext(c Context) {
	if dc, ok := c.(*DefaultContext); ok {
		// Clean up the context before returning to pool
		dc.request = nil
		dc.response = nil
		dc.path = ""
		// Reset params instead of nil
		if dc.params != nil {
			dc.params.reset()
		}
		dc.handler = nil
		dc.logger = nil
		dc.echo = nil
		dc.stdContext = nil
		
		// Clear the store but keep the map allocated
		if dc.store != nil {
			for k := range dc.store {
				delete(dc.store, k)
			}
		}
		
		contextPool.Put(dc)
	}
}