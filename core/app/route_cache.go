package app

import (
	"reflect"
	"sync"

	"github.com/yshengliao/gortex/core/handler"
	"github.com/yshengliao/gortex/transport/http"
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

	// Double-check after acquiring write lock
	if methods, exists := c.methods[t]; exists {
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

	// Check custom methods
	for i := 0; i < t.NumMethod(); i++ {
		method := t.Method(i)
		name := method.Name

		// Skip if already processed
		if _, exists := methods[name]; exists {
			continue
		}

		// Check if it's a valid handler
		if isValidGortexHandler(method) {
			httpMethod := "GET" // Default

			// Determine HTTP method from name
			prefixMap := map[string]string{
				"Post":   "POST",
				"Create": "POST",
				"Put":    "PUT",
				"Update": "PUT",
				"Delete": "DELETE",
				"Remove": "DELETE",
				"Patch":  "PATCH",
			}

			for prefix, method := range prefixMap {
				if len(name) > len(prefix) && name[:len(prefix)] == prefix {
					httpMethod = method
					break
				}
			}

			methods[name] = HandlerMethod{
				Name:       name,
				HTTPMethod: httpMethod,
				Path:       methodNameToPath(name),
				IsValid:    true,
				Method:     method,
			}
		}
	}

	return methods
}

// RouteInfo represents cached route information
type RouteInfo struct {
	Method  string
	Path    string
	Handler handler.HandlerFunc
}

// RouteCache caches compiled routes
type RouteCache struct {
	mu     sync.RWMutex
	routes map[string][]RouteInfo // key is struct type name
}

// Global route cache
var routeCache = &RouteCache{
	routes: make(map[string][]RouteInfo),
}

// GetRoutes returns cached routes for a handler type
func (c *RouteCache) GetRoutes(key string) ([]RouteInfo, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	routes, exists := c.routes[key]
	return routes, exists
}

// SetRoutes caches routes for a handler type
func (c *RouteCache) SetRoutes(key string, routes []RouteInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.routes[key] = routes
}

// ClearCache clears all caches (useful for testing)
func ClearCache() {
	handlerCache.mu.Lock()
	handlerCache.methods = make(map[reflect.Type]map[string]HandlerMethod)
	handlerCache.mu.Unlock()

	routeCache.mu.Lock()
	routeCache.routes = make(map[string][]RouteInfo)
	routeCache.mu.Unlock()
}
