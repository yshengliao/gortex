package router

import (
	"net/http"
	"sync"
)

// router implements the Router interface
type router struct {
	trees       map[string]*node // method -> root node
	groups      []*routeGroup
	middlewares []Middleware
	mu          sync.RWMutex
}

// NewRouter creates a new router instance
func NewRouter() Router {
	return &router{
		trees:       make(map[string]*node),
		groups:      make([]*routeGroup, 0),
		middlewares: make([]Middleware, 0),
	}
}

// routeGroup represents a group of routes with common prefix and middleware
type routeGroup struct {
	prefix      string
	middlewares []Middleware
	router      *router
}

// GET registers a GET route
func (r *router) GET(path string, h HandlerFunc, m ...Middleware) {
	r.addRoute("GET", path, h, m...)
}

// POST registers a POST route
func (r *router) POST(path string, h HandlerFunc, m ...Middleware) {
	r.addRoute("POST", path, h, m...)
}

// PUT registers a PUT route
func (r *router) PUT(path string, h HandlerFunc, m ...Middleware) {
	r.addRoute("PUT", path, h, m...)
}

// DELETE registers a DELETE route
func (r *router) DELETE(path string, h HandlerFunc, m ...Middleware) {
	r.addRoute("DELETE", path, h, m...)
}

// PATCH registers a PATCH route
func (r *router) PATCH(path string, h HandlerFunc, m ...Middleware) {
	r.addRoute("PATCH", path, h, m...)
}

// HEAD registers a HEAD route
func (r *router) HEAD(path string, h HandlerFunc, m ...Middleware) {
	r.addRoute("HEAD", path, h, m...)
}

// OPTIONS registers an OPTIONS route
func (r *router) OPTIONS(path string, h HandlerFunc, m ...Middleware) {
	r.addRoute("OPTIONS", path, h, m...)
}

// Group creates a new route group with a prefix
func (r *router) Group(prefix string, m ...Middleware) Router {
	group := &routeGroup{
		prefix:      prefix,
		middlewares: m,
		router:      r,
	}
	r.groups = append(r.groups, group)
	return group
}

// Use adds global middleware
func (r *router) Use(m ...Middleware) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.middlewares = append(r.middlewares, m...)
}

// addRoute adds a route to the router
func (r *router) addRoute(method, path string, h HandlerFunc, m ...Middleware) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if r.trees[method] == nil {
		r.trees[method] = &node{}
	}
	
	// Apply middleware chain
	handler := h
	// Apply route-specific middleware in reverse order
	for i := len(m) - 1; i >= 0; i-- {
		handler = m[i](handler)
	}
	// Apply global middleware in reverse order
	for i := len(r.middlewares) - 1; i >= 0; i-- {
		handler = r.middlewares[i](handler)
	}
	
	r.trees[method].insertRoute(path, handler)
}

// ServeHTTP implements http.Handler interface
func (r *router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	method := req.Method
	path := req.URL.Path
	
	if root, ok := r.trees[method]; ok {
		params := make(map[string]string)
		if handler := root.search(path, params); handler != nil {
			ctx := &httpContext{
				request:  req,
				response: w,
				params:   params,
				values:   make(map[string]interface{}),
			}
			if err := handler(ctx); err != nil {
				// Handle error
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
	}
	
	// Not found
	http.NotFound(w, req)
}

// Route group implementation

// GET registers a GET route in the group
func (g *routeGroup) GET(path string, h HandlerFunc, m ...Middleware) {
	g.router.addRoute("GET", g.prefix+path, h, append(g.middlewares, m...)...)
}

// POST registers a POST route in the group
func (g *routeGroup) POST(path string, h HandlerFunc, m ...Middleware) {
	g.router.addRoute("POST", g.prefix+path, h, append(g.middlewares, m...)...)
}

// PUT registers a PUT route in the group
func (g *routeGroup) PUT(path string, h HandlerFunc, m ...Middleware) {
	g.router.addRoute("PUT", g.prefix+path, h, append(g.middlewares, m...)...)
}

// DELETE registers a DELETE route in the group
func (g *routeGroup) DELETE(path string, h HandlerFunc, m ...Middleware) {
	g.router.addRoute("DELETE", g.prefix+path, h, append(g.middlewares, m...)...)
}

// PATCH registers a PATCH route in the group
func (g *routeGroup) PATCH(path string, h HandlerFunc, m ...Middleware) {
	g.router.addRoute("PATCH", g.prefix+path, h, append(g.middlewares, m...)...)
}

// HEAD registers a HEAD route in the group
func (g *routeGroup) HEAD(path string, h HandlerFunc, m ...Middleware) {
	g.router.addRoute("HEAD", g.prefix+path, h, append(g.middlewares, m...)...)
}

// OPTIONS registers an OPTIONS route in the group
func (g *routeGroup) OPTIONS(path string, h HandlerFunc, m ...Middleware) {
	g.router.addRoute("OPTIONS", g.prefix+path, h, append(g.middlewares, m...)...)
}

// Group creates a sub-group
func (g *routeGroup) Group(prefix string, m ...Middleware) Router {
	return g.router.Group(g.prefix+prefix, append(g.middlewares, m...)...)
}

// Use adds middleware to the group
func (g *routeGroup) Use(m ...Middleware) {
	g.middlewares = append(g.middlewares, m...)
}