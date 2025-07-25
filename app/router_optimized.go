package app

import (
	"reflect"
	"sync"

	"github.com/labstack/echo/v4"
	"github.com/yshengliao/gortex/pkg/pool"
	"go.uber.org/zap"
)

// OptimizedRouter provides optimized routing with caching and pooling
type OptimizedRouter struct {
	e              *echo.Echo
	ctx            *Context
	logger         *zap.Logger
	handlerCache   *HandlerCache
	contextPool    sync.Pool
}

// NewOptimizedRouter creates an optimized router
func NewOptimizedRouter(e *echo.Echo, ctx *Context, logger *zap.Logger) *OptimizedRouter {
	return &OptimizedRouter{
		e:            e,
		ctx:          ctx,
		logger:       logger,
		handlerCache: handlerCache,
		contextPool: sync.Pool{
			New: func() interface{} {
				return &RouteContext{
					params: make(map[string]string, 4),
				}
			},
		},
	}
}

// RouteContext is a pooled context for route processing
type RouteContext struct {
	params map[string]string
	path   string
	method string
}

// Reset resets the route context for reuse
func (rc *RouteContext) Reset() {
	rc.path = ""
	rc.method = ""
	for k := range rc.params {
		delete(rc.params, k)
	}
}

// RegisterHandler registers a handler with optimizations
func (r *OptimizedRouter) RegisterHandler(basePath string, handler interface{}, isWebSocket bool) error {
	handlerValue := reflect.ValueOf(handler)
	handlerType := handlerValue.Type()
	
	// Get cached methods
	methods := r.handlerCache.GetHandlerMethods(handlerType)
	
	// For WebSocket handlers
	if isWebSocket {
		return r.registerWebSocketHandler(basePath, handler, methods)
	}
	
	// Register HTTP methods
	return r.registerHTTPMethods(basePath, handler, handlerValue, methods)
}

// registerWebSocketHandler registers WebSocket handlers
func (r *OptimizedRouter) registerWebSocketHandler(basePath string, handler interface{}, methods map[string]HandlerMethod) error {
	// Try direct interface assertion first
	if echoHandler, ok := handler.(echo.HandlerFunc); ok {
		r.e.GET(basePath, echoHandler)
		return nil
	}
	
	// Look for WebSocket methods in cache
	wsMethodNames := []string{"WebSocket", "WS", "Hijack", "HandleConnection"}
	for _, name := range wsMethodNames {
		if method, exists := methods[name]; exists && method.IsValid {
			handlerValue := reflect.ValueOf(handler)
			methodFunc := handlerValue.Method(method.Method.Index).Interface()
			if echoFunc, ok := methodFunc.(func(echo.Context) error); ok {
				r.e.GET(basePath, echoFunc)
				return nil
			}
		}
	}
	
	return nil
}

// registerHTTPMethods registers HTTP method handlers
func (r *OptimizedRouter) registerHTTPMethods(basePath string, handler interface{}, handlerValue reflect.Value, methods map[string]HandlerMethod) error {
	// Pre-allocate slice for routes
	routes := make([]RouteInfo, 0, len(methods))
	
	// Standard HTTP methods
	httpMethods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
	
	// Register standard HTTP methods
	for _, httpMethod := range httpMethods {
		if method, exists := methods[httpMethod]; exists && method.IsValid {
			methodFunc := handlerValue.MethodByName(httpMethod).Interface()
			if echoFunc, ok := methodFunc.(func(echo.Context) error); ok {
				routes = append(routes, RouteInfo{
					Method:  httpMethod,
					Path:    basePath,
					Handler: echoFunc,
				})
			}
		}
	}
	
	// Register custom methods
	for name, method := range methods {
		// Skip standard HTTP methods
		isStandard := false
		for _, std := range httpMethods {
			if name == std {
				isStandard = true
				break
			}
		}
		if isStandard {
			continue
		}
		
		if method.IsValid {
			fullPath := basePath + method.Path
			methodFunc := handlerValue.Method(method.Method.Index).Interface()
			if echoFunc, ok := methodFunc.(func(echo.Context) error); ok {
				routes = append(routes, RouteInfo{
					Method:  method.HTTPMethod,
					Path:    fullPath,
					Handler: echoFunc,
				})
			}
		}
	}
	
	// Register all routes
	for _, route := range routes {
		switch route.Method {
		case "GET":
			r.e.GET(route.Path, route.Handler)
		case "POST":
			r.e.POST(route.Path, route.Handler)
		case "PUT":
			r.e.PUT(route.Path, route.Handler)
		case "DELETE":
			r.e.DELETE(route.Path, route.Handler)
		case "PATCH":
			r.e.PATCH(route.Path, route.Handler)
		case "HEAD":
			r.e.HEAD(route.Path, route.Handler)
		case "OPTIONS":
			r.e.OPTIONS(route.Path, route.Handler)
		}
		
		if r.logger != nil {
			r.logger.Debug("Registered optimized route",
				zap.String("method", route.Method),
				zap.String("path", route.Path))
		}
	}
	
	return nil
}

// OptimizedHandlerFunc wraps a handler with pooling optimizations
func (r *OptimizedRouter) OptimizedHandlerFunc(handler echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		// Get pooled buffers for response
		buf := pool.GetBuffer()
		defer pool.PutBuffer(buf)
		
		// Execute handler
		return handler(c)
	}
}