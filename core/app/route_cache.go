package app

import (
	"reflect"
	"sync"
)

// HandlerCache caches reflection results for handlers
type HandlerCache struct {
	mu      sync.RWMutex
	methods map[reflect.Type]map[string]HandlerMethod
}

// HandlerMethod represents a cached handler method
type HandlerMethod struct {
	Name       string
	HTTPMethod string
	Path       string
	IsValid    bool
	Method     reflect.Method
}

// Global handler cache
var handlerCache = &HandlerCache{
	methods: make(map[reflect.Type]map[string]HandlerMethod),
}

// GetHandlerMethods returns cached handler methods for a type
func (c *HandlerCache) GetHandlerMethods(t reflect.Type) map[string]HandlerMethod {
	c.mu.RLock()
	methods, exists := c.methods[t]
	c.mu.RUnlock()

	if exists {
		return methods
	}

	// Build cache
	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock (reuse outer vars to avoid shadowing)
	if methods, exists = c.methods[t]; exists {
		return methods
	}

	methods = c.buildMethodCache(t)
	c.methods[t] = methods
	return methods
}

// buildMethodCache builds the method cache for a type
func (c *HandlerCache) buildMethodCache(t reflect.Type) map[string]HandlerMethod {
	methods := make(map[string]HandlerMethod)

	// HTTP methods to check
	httpMethods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}

	// Check standard HTTP methods
	for _, httpMethod := range httpMethods {
		if method, exists := t.MethodByName(httpMethod); exists {
			if isValidGortexHandler(method) {
				methods[httpMethod] = HandlerMethod{
					Name:       httpMethod,
					HTTPMethod: httpMethod,
					Path:       "",
					IsValid:    true,
					Method:     method,
				}
			}
		}
	}

	// Check custom methods. All non-standard method names are registered as POST
	// by the runtime (registerCustomMethodWithMiddleware always calls r.POST).
	for i := 0; i < t.NumMethod(); i++ {
		method := t.Method(i)
		name := method.Name

		// Skip if already processed
		if _, exists := methods[name]; exists {
			continue
		}

		// Check if it's a valid handler
		if isValidGortexHandler(method) {
			methods[name] = HandlerMethod{
				Name:       name,
				HTTPMethod: "POST", // Custom methods always map to POST at runtime.
				Path:       methodNameToPath(name),
				IsValid:    true,
				Method:     method,
			}
		}
	}

	return methods
}

// ClearCache clears the handler method cache (useful for testing).
func ClearCache() {
	handlerCache.mu.Lock()
	handlerCache.methods = make(map[reflect.Type]map[string]HandlerMethod)
	handlerCache.mu.Unlock()
}
