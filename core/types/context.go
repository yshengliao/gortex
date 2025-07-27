// Package types provides core type definitions for the Gortex framework
package types

import (
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
)

// Context represents the context of the current HTTP request.
// It provides methods to access request and response data.
type Context interface {
	// Request returns the underlying HTTP request
	Request() *http.Request
	
	// SetRequest sets the HTTP request
	SetRequest(r *http.Request)
	
	// Response returns the response writer
	Response() ResponseWriter
	
	// IsTLS returns true if the request is using TLS
	IsTLS() bool
	
	// IsWebSocket returns true if the request is a WebSocket upgrade
	IsWebSocket() bool
	
	// Scheme returns the HTTP protocol scheme (http or https)
	Scheme() string
	
	// RealIP returns the client's real IP address
	RealIP() string
	
	// Path returns the request path
	Path() string
	
	// SetPath sets the request path
	SetPath(p string)
	
	// Param returns path parameter by name
	Param(name string) string
	
	// Params returns all path parameters
	Params() url.Values
	
	// ParamNames returns all path parameter names
	ParamNames() []string
	
	// SetParamNames sets the parameter names
	SetParamNames(names ...string)
	
	// SetParamValues sets the parameter values
	SetParamValues(values ...string)
	
	// QueryParam returns query parameter by name
	QueryParam(name string) string
	
	// QueryParams returns all query parameters
	QueryParams() url.Values
	
	// QueryString returns the URL query string
	QueryString() string
	
	// FormValue returns form value by name
	FormValue(name string) string
	
	// FormParams returns all form parameters
	FormParams() (url.Values, error)
	
	// FormFile returns the multipart form file for the provided name
	FormFile(name string) (*multipart.FileHeader, error)
	
	// MultipartForm returns the multipart form
	MultipartForm() (*multipart.Form, error)
	
	// Cookie returns the named cookie
	Cookie(name string) (*http.Cookie, error)
	
	// SetCookie adds a Set-Cookie header
	SetCookie(cookie *http.Cookie)
	
	// Cookies returns all cookies
	Cookies() []*http.Cookie
	
	// Get retrieves data from the context
	Get(key string) interface{}
	
	// Set saves data in the context
	Set(key string, val interface{})
	
	// Bind binds the request body to an interface
	Bind(i interface{}) error
	
	// Validate validates an interface
	Validate(i interface{}) error
	
	// Render renders a template with data
	Render(code int, name string, data interface{}) error
	
	// HTML sends an HTTP response with HTML
	HTML(code int, html string) error
	
	// HTMLBlob sends an HTTP response with HTML blob
	HTMLBlob(code int, b []byte) error
	
	// String sends an HTTP response with string
	String(code int, s string) error
	
	// JSON sends an HTTP response with JSON
	JSON(code int, i interface{}) error
	
	// JSONPretty sends an HTTP response with pretty JSON
	JSONPretty(code int, i interface{}, indent string) error
	
	// JSONBlob sends an HTTP response with JSON blob
	JSONBlob(code int, b []byte) error
	
	// JSONP sends an HTTP response with JSONP
	JSONP(code int, callback string, i interface{}) error
	
	// JSONPBlob sends an HTTP response with JSONP blob
	JSONPBlob(code int, callback string, b []byte) error
	
	// XML sends an HTTP response with XML
	XML(code int, i interface{}) error
	
	// XMLPretty sends an HTTP response with pretty XML
	XMLPretty(code int, i interface{}, indent string) error
	
	// XMLBlob sends an HTTP response with XML blob
	XMLBlob(code int, b []byte) error
	
	// Blob sends an HTTP response with blob
	Blob(code int, contentType string, b []byte) error
	
	// Stream sends an HTTP response with stream
	Stream(code int, contentType string, r io.Reader) error
	
	// File sends a file as the response
	File(file string) error
	
	// Attachment sends a file as attachment
	Attachment(file string, name string) error
	
	// Inline sends a file inline
	Inline(file string, name string) error
	
	// NoContent sends a response with no body
	NoContent(code int) error
	
	// Redirect redirects the request
	Redirect(code int, url string) error
	
	// Error invokes the registered error handler
	Error(err error)
	
	// Handler returns the registered handler
	Handler() HandlerFunc
	
	// SetHandler sets the matched handler  
	SetHandler(h HandlerFunc)
	
	// Logger returns the logger instance
	Logger() interface{}
	
	// Set the logger instance
	SetLogger(l interface{})
	
	// Echo returns the context (for echo compatibility)
	Echo() interface{}
	
	// Reset resets the context
	Reset(r *http.Request, w http.ResponseWriter)
	
	// Context returns the standard context
	Context() context.Context
	
	// Span returns the current trace span from context
	Span() interface{}
}

// ResponseWriter wraps http.ResponseWriter
type ResponseWriter interface {
	http.ResponseWriter
	
	// Status returns the status code
	Status() int
	
	// Size returns the size of response
	Size() int64
	
	// Written returns whether response was written
	Written() bool
	
	// WriteHeader writes the header
	WriteHeader(code int)
	
	// Write writes the data
	Write(b []byte) (int, error)
	
	// Flush flushes the response
	Flush()
}

// HandlerFunc defines a function to serve HTTP requests
type HandlerFunc func(c Context) error

// MiddlewareFunc defines the middleware function type
type MiddlewareFunc func(next HandlerFunc) HandlerFunc