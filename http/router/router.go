package router

// Router defines the routing interface
type Router interface {
	// Route registration methods
	GET(path string, h HandlerFunc, m ...Middleware)
	POST(path string, h HandlerFunc, m ...Middleware)
	PUT(path string, h HandlerFunc, m ...Middleware)
	DELETE(path string, h HandlerFunc, m ...Middleware)
	PATCH(path string, h HandlerFunc, m ...Middleware)
	HEAD(path string, h HandlerFunc, m ...Middleware)
	OPTIONS(path string, h HandlerFunc, m ...Middleware)
	
	// Group creates a new route group
	Group(prefix string, m ...Middleware) Router
	
	// Use adds global middleware
	Use(m ...Middleware)
}

// RouterConfig contains router configuration
type RouterConfig struct {
	// Performance optimizations
	EnableCaching bool
	MaxRoutes     int
	
	// Development features
	DebugMode bool
}