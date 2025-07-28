package mock

import (
	"net/http"
	"net/http/httptest"
	"sync"
	
	httpctx "github.com/yshengliao/gortex/transport/http"
)

// Context is a mock implementation of httpctx.Context for testing
type Context struct {
	httpctx.Context
	request  *http.Request
	response *httptest.ResponseRecorder
	params   map[string]string
	values   map[string]interface{}
	mu       sync.RWMutex
}

// NewContext creates a new mock context
func NewContext() *Context {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	
	return &Context{
		Context:  httpctx.NewDefaultContext(req, rec),
		request:  req,
		response: rec,
		params:   make(map[string]string),
		values:   make(map[string]interface{}),
	}
}

// NewContextWithRequest creates a new mock context with a specific request
func NewContextWithRequest(req *http.Request) *Context {
	rec := httptest.NewRecorder()
	
	return &Context{
		Context:  httpctx.NewDefaultContext(req, rec),
		request:  req,
		response: rec,
		params:   make(map[string]string),
		values:   make(map[string]interface{}),
	}
}

// SetParam sets a path parameter
func (c *Context) SetParam(name, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.params[name] = value
}

// SetValue sets a context value
func (c *Context) SetValue(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.values[key] = value
}

// Response returns the response recorder
func (c *Context) Response() *httptest.ResponseRecorder {
	return c.response
}

// AssertJSON asserts that the response contains the expected JSON
func (c *Context) AssertJSON(t interface{}, expectedStatus int, expectedBody string) {
	// This would be implemented with actual test assertion logic
	// For now, it's a placeholder
}