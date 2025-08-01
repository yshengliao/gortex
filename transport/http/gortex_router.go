// Package http provides the core routing functionality for Gortex framework
package http

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
)

// GortexRouter defines the main routing interface for Gortex framework
type GortexRouter interface {
	// HTTP method handlers
	GET(path string, h HandlerFunc, m ...MiddlewareFunc)
	POST(path string, h HandlerFunc, m ...MiddlewareFunc)
	PUT(path string, h HandlerFunc, m ...MiddlewareFunc)
	DELETE(path string, h HandlerFunc, m ...MiddlewareFunc)
	PATCH(path string, h HandlerFunc, m ...MiddlewareFunc)
	HEAD(path string, h HandlerFunc, m ...MiddlewareFunc)
	OPTIONS(path string, h HandlerFunc, m ...MiddlewareFunc)

	// Route grouping
	Group(prefix string, m ...MiddlewareFunc) GortexRouter

	// Global middleware
	Use(m ...MiddlewareFunc)

	// HTTP handler integration
	ServeHTTP(w http.ResponseWriter, r *http.Request)
}

// gortexRouter implements GortexRouter interface
type gortexRouter struct {
	trees       map[string]*routeNode
	middlewares []MiddlewareFunc
	prefix      string
	parent      *gortexRouter
	mu          sync.RWMutex
}

// routeNode represents a node in the route tree
type routeNode struct {
	path        string
	handler     HandlerFunc
	middlewares []MiddlewareFunc
	children    map[string]*routeNode
	paramChild  *routeNode
	wildChild   *routeNode
	isParam     bool
	isWild      bool
	paramName   string
}

// NewGortexRouter creates a new Gortex router instance
func NewGortexRouter() GortexRouter {
	return &gortexRouter{
		trees:       make(map[string]*routeNode),
		middlewares: make([]MiddlewareFunc, 0),
	}
}

// GET registers a GET route
func (r *gortexRouter) GET(path string, h HandlerFunc, m ...MiddlewareFunc) {
	r.addRoute("GET", path, h, m...)
}

// POST registers a POST route
func (r *gortexRouter) POST(path string, h HandlerFunc, m ...MiddlewareFunc) {
	r.addRoute("POST", path, h, m...)
}

// PUT registers a PUT route
func (r *gortexRouter) PUT(path string, h HandlerFunc, m ...MiddlewareFunc) {
	r.addRoute("PUT", path, h, m...)
}

// DELETE registers a DELETE route
func (r *gortexRouter) DELETE(path string, h HandlerFunc, m ...MiddlewareFunc) {
	r.addRoute("DELETE", path, h, m...)
}

// PATCH registers a PATCH route
func (r *gortexRouter) PATCH(path string, h HandlerFunc, m ...MiddlewareFunc) {
	r.addRoute("PATCH", path, h, m...)
}

// HEAD registers a HEAD route
func (r *gortexRouter) HEAD(path string, h HandlerFunc, m ...MiddlewareFunc) {
	r.addRoute("HEAD", path, h, m...)
}

// OPTIONS registers an OPTIONS route
func (r *gortexRouter) OPTIONS(path string, h HandlerFunc, m ...MiddlewareFunc) {
	r.addRoute("OPTIONS", path, h, m...)
}

// Group creates a new route group
func (r *gortexRouter) Group(prefix string, m ...MiddlewareFunc) GortexRouter {
	return &gortexRouter{
		trees:       r.trees,
		middlewares: append(r.middlewares, m...),
		prefix:      r.prefix + prefix,
		parent:      r,
	}
}

// Use adds global middleware
func (r *gortexRouter) Use(m ...MiddlewareFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.middlewares = append(r.middlewares, m...)
}

// addRoute adds a route to the router
func (r *gortexRouter) addRoute(method, path string, h HandlerFunc, m ...MiddlewareFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()

	fullPath := r.prefix + path

	if r.trees[method] == nil {
		r.trees[method] = &routeNode{
			children: make(map[string]*routeNode),
		}
	}

	// Combine all middleware: global + group + route-specific
	allMiddlewares := make([]MiddlewareFunc, 0)
	allMiddlewares = append(allMiddlewares, r.middlewares...)
	allMiddlewares = append(allMiddlewares, m...)

	// Create final handler by applying middleware chain
	finalHandler := h
	for i := len(allMiddlewares) - 1; i >= 0; i-- {
		finalHandler = allMiddlewares[i](finalHandler)
	}

	r.addToTree(r.trees[method], fullPath, finalHandler, allMiddlewares)
}

// addToTree adds a route to the route tree
func (r *gortexRouter) addToTree(root *routeNode, path string, handler HandlerFunc, middlewares []MiddlewareFunc) {
	// Special case for root path
	if path == "/" || path == "" {
		root.handler = handler
		root.middlewares = middlewares
		return
	}
	
	segments := strings.Split(strings.Trim(path, "/"), "/")
	current := root

	for _, segment := range segments {
		if segment == "" {
			continue
		}

		if strings.HasPrefix(segment, ":") {
			// Parameter route
			paramName := segment[1:]
			if current.paramChild == nil {
				current.paramChild = &routeNode{
					children:  make(map[string]*routeNode),
					isParam:   true,
					paramName: paramName,
				}
			}
			current = current.paramChild
		} else if strings.HasPrefix(segment, "*") {
			// Wildcard route
			if current.wildChild == nil {
				current.wildChild = &routeNode{
					children: make(map[string]*routeNode),
					isWild:   true,
				}
			}
			current = current.wildChild
		} else {
			// Static route
			if current.children[segment] == nil {
				current.children[segment] = &routeNode{
					children: make(map[string]*routeNode),
					path:     segment,
				}
			}
			current = current.children[segment]
		}
	}

	current.handler = handler
	current.middlewares = middlewares
}

// ServeHTTP implements the http.Handler interface
func (r *gortexRouter) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// Use context pool for better performance
	ctx := AcquireContext(req, w)
	defer ReleaseContext(ctx)
	
	// Set the request path
	ctx.SetPath(req.URL.Path)

	if handler, params := r.findRoute(req.Method, req.URL.Path); handler != nil {
		// Set path parameters efficiently
		if len(params) > 0 {
			SetParams(ctx, params)
		}

		if err := handler(ctx); err != nil {
			// Handle error - check if it's an HTTPError
			if he, ok := err.(*HTTPError); ok {
				// Write the proper HTTP error response
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(he.Code)
				response := map[string]interface{}{
					"message": he.Message,
				}
				if encErr := json.NewEncoder(w).Encode(response); encErr != nil {
					// Fall back to plain text if JSON encoding fails
					http.Error(w, he.Error(), he.Code)
				}
			} else {
				// Generic error handling
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		}
	} else {
		http.NotFound(w, req)
	}
}

// findRoute finds a route handler for the given method and path
func (r *gortexRouter) findRoute(method, path string) (HandlerFunc, map[string]string) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	root := r.trees[method]
	if root == nil {
		return nil, nil
	}

	segments := strings.Split(strings.Trim(path, "/"), "/")
	params := make(map[string]string)

	handler, _ := r.searchTree(root, segments, 0, params)
	return handler, params
}

// searchTree recursively searches the route tree
func (r *gortexRouter) searchTree(node *routeNode, segments []string, index int, params map[string]string) (HandlerFunc, []MiddlewareFunc) {
	// Check if we've processed all segments or if this is root path
	if index >= len(segments) || (len(segments) == 1 && segments[0] == "") {
		if node.handler != nil {
			return node.handler, node.middlewares
		}
		return nil, nil
	}

	segment := segments[index]

	// Try static route first
	if child, exists := node.children[segment]; exists {
		if handler, middlewares := r.searchTree(child, segments, index+1, params); handler != nil {
			return handler, middlewares
		}
	}

	// Try parameter route
	if node.paramChild != nil {
		params[node.paramChild.paramName] = segment
		if handler, middlewares := r.searchTree(node.paramChild, segments, index+1, params); handler != nil {
			return handler, middlewares
		}
		delete(params, node.paramChild.paramName)
	}

	// Try wildcard route
	if node.wildChild != nil {
		// Capture all remaining segments as the wildcard value
		remainingPath := strings.Join(segments[index:], "/")
		params["*"] = remainingPath
		if handler, middlewares := r.searchTree(node.wildChild, segments, len(segments), params); handler != nil {
			return handler, middlewares
		}
		delete(params, "*")
	}

	return nil, nil
}
