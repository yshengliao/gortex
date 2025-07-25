package router

// Context defines the interface for handling HTTP requests
type Context interface {
	// Request data access
	Param(name string) string      // Path parameters
	QueryParam(name string) string // Query parameters
	Bind(interface{}) error        // Request body binding
	
	// Response methods
	JSON(code int, i interface{}) error
	String(code int, s string) error
	
	// Context value storage
	Get(key string) interface{}
	Set(key string, val interface{})
	
	// Request/Response access
	Request() interface{}  // Returns *http.Request
	Response() interface{} // Returns http.ResponseWriter
}

// HandlerFunc defines the handler function type
type HandlerFunc func(Context) error

// Middleware defines the middleware function type
type Middleware func(HandlerFunc) HandlerFunc