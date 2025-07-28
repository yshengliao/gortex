package http

// Router defines the routing interface
type Router interface {
	// Route registration methods
	GET(path string, h HandlerFunc, m ...MiddlewareFunc)
	POST(path string, h HandlerFunc, m ...MiddlewareFunc)
	PUT(path string, h HandlerFunc, m ...MiddlewareFunc)
	DELETE(path string, h HandlerFunc, m ...MiddlewareFunc)
	PATCH(path string, h HandlerFunc, m ...MiddlewareFunc)
	HEAD(path string, h HandlerFunc, m ...MiddlewareFunc)
	OPTIONS(path string, h HandlerFunc, m ...MiddlewareFunc)
	
	// Group creates a new route group
	Group(prefix string, m ...MiddlewareFunc) Router
	
	// Use adds global middleware
	Use(m ...MiddlewareFunc)
}

// RouterConfig contains router configuration
type RouterConfig struct {
	// Performance optimizations
	EnableCaching bool
	MaxRoutes     int
	
	// Development features
	DebugMode bool
}