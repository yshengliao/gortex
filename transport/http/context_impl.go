package http

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// httpContext implements the Context interface
type httpContext struct {
	request     *http.Request
	response    http.ResponseWriter
	params      map[string]string
	values      map[string]interface{}
	written     bool
}

// NewContext creates a new HTTP context
func NewContext(w http.ResponseWriter, r *http.Request) Context {
	return &httpContext{
		request:  r,
		response: w,
		params:   make(map[string]string),
		values:   make(map[string]interface{}),
	}
}

// Param returns path parameter by name
func (c *httpContext) Param(name string) string {
	if val, ok := c.params[name]; ok {
		return val
	}
	return ""
}

// QueryParam returns query parameter by name
func (c *httpContext) QueryParam(name string) string {
	return c.request.URL.Query().Get(name)
}

// Bind binds the request body to the given interface
func (c *httpContext) Bind(i interface{}) error {
	contentType := c.request.Header.Get("Content-Type")
	
	switch {
	case strings.HasPrefix(contentType, "application/json"):
		return c.bindJSON(i)
	case strings.HasPrefix(contentType, "application/x-www-form-urlencoded"):
		return c.bindForm(i)
	case strings.HasPrefix(contentType, "multipart/form-data"):
		return c.bindMultipartForm(i)
	default:
		// Default to JSON
		return c.bindJSON(i)
	}
}

// bindJSON binds JSON request body
func (c *httpContext) bindJSON(i interface{}) error {
	body, err := io.ReadAll(c.request.Body)
	if err != nil {
		return err
	}
	defer c.request.Body.Close()
	
	if len(body) == 0 {
		return nil
	}
	
	return json.Unmarshal(body, i)
}

// bindForm binds form data
func (c *httpContext) bindForm(i interface{}) error {
	if err := c.request.ParseForm(); err != nil {
		return err
	}
	
	// Simple implementation - can be enhanced with reflection
	// For now, just store the form values
	formData := make(map[string]string)
	for key, values := range c.request.Form {
		if len(values) > 0 {
			formData[key] = values[0]
		}
	}
	
	// Convert to JSON for simplicity
	jsonData, err := json.Marshal(formData)
	if err != nil {
		return err
	}
	
	return json.Unmarshal(jsonData, i)
}

// bindMultipartForm binds multipart form data
func (c *httpContext) bindMultipartForm(i interface{}) error {
	if err := c.request.ParseMultipartForm(32 << 20); // 32 MB
		err != nil {
		return err
	}
	
	// Similar to form binding
	formData := make(map[string]string)
	if c.request.MultipartForm != nil && c.request.MultipartForm.Value != nil {
		for key, values := range c.request.MultipartForm.Value {
			if len(values) > 0 {
				formData[key] = values[0]
			}
		}
	}
	
	jsonData, err := json.Marshal(formData)
	if err != nil {
		return err
	}
	
	return json.Unmarshal(jsonData, i)
}

// JSON sends a JSON response with status code
func (c *httpContext) JSON(code int, i interface{}) error {
	c.response.Header().Set("Content-Type", "application/json")
	c.response.WriteHeader(code)
	c.written = true
	
	encoder := json.NewEncoder(c.response)
	return encoder.Encode(i)
}

// String sends a string response with status code
func (c *httpContext) String(code int, s string) error {
	c.response.Header().Set("Content-Type", "text/plain; charset=utf-8")
	c.response.WriteHeader(code)
	c.written = true
	
	_, err := c.response.Write([]byte(s))
	return err
}

// Get retrieves data from context
func (c *httpContext) Get(key string) interface{} {
	if val, ok := c.values[key]; ok {
		return val
	}
	return nil
}

// Set saves data in context
func (c *httpContext) Set(key string, val interface{}) {
	c.values[key] = val
}

// Request returns the *http.Request
func (c *httpContext) Request() interface{} {
	return c.request
}

// Response returns the http.ResponseWriter
func (c *httpContext) Response() interface{} {
	return c.response
}

// SetParam sets a path parameter
func (c *httpContext) SetParam(name, value string) {
	c.params[name] = value
}

// QueryParams returns all query parameters
func (c *httpContext) QueryParams() url.Values {
	return c.request.URL.Query()
}

// IsWritten returns true if response has been written
func (c *httpContext) IsWritten() bool {
	return c.written
}