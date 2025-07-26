package app

import (
	"fmt"
	"reflect"
	"sync"
)

// Context is a simple dependency injection container using generics
type Context struct {
	mu       sync.RWMutex
	services map[reflect.Type]any
}

// NewContext creates a new DI context
func NewContext() *Context {
	return &Context{
		services: make(map[reflect.Type]any),
	}
}

// Register adds a service to the context
func Register[T any](ctx *Context, service T) {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	
	// Get the actual type, not the generic type parameter
	actualType := reflect.TypeOf(service)
	ctx.services[actualType] = service
}

// Get retrieves a service from the context
func Get[T any](ctx *Context) (T, error) {
	ctx.mu.RLock()
	defer ctx.mu.RUnlock()
	
	var zero T
	t := reflect.TypeOf((*T)(nil)).Elem()
	
	if service, ok := ctx.services[t]; ok {
		if s, ok := service.(T); ok {
			return s, nil
		}
	}
	
	return zero, fmt.Errorf("service %v not found", t)
}

// MustGet retrieves a service from the context, panics if not found
func MustGet[T any](ctx *Context) T {
	service, err := Get[T](ctx)
	if err != nil {
		panic(err)
	}
	return service
}