package middleware

import (
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	
	"github.com/yshengliao/gortex/core/types"
)

// testContext is a minimal implementation of types.Context for testing
type testContext struct {
	request  *http.Request
	response types.ResponseWriter
	values   map[string]interface{}
	params   map[string]string
}

// newTestContext creates a new test context
func newTestContext(req *http.Request, resp http.ResponseWriter) *testContext {
	return &testContext{
		request:  req,
		response: NewTestResponseWriter(resp),
		values:   make(map[string]interface{}),
		params:   make(map[string]string),
	}
}

// Request returns the underlying HTTP request
func (c *testContext) Request() *http.Request {
	return c.request
}

// SetRequest sets the HTTP request
func (c *testContext) SetRequest(r *http.Request) {
	c.request = r
}

// Response returns the response writer
func (c *testContext) Response() types.ResponseWriter {
	return c.response
}

// IsTLS returns true if the request is using TLS
func (c *testContext) IsTLS() bool {
	return c.request.TLS != nil
}

// IsWebSocket returns true if the request is a WebSocket upgrade
func (c *testContext) IsWebSocket() bool {
	return false
}

// Scheme returns the HTTP protocol scheme
func (c *testContext) Scheme() string {
	if c.IsTLS() {
		return "https"
	}
	return "http"
}

// RealIP returns the client's real IP address
func (c *testContext) RealIP() string {
	return c.request.RemoteAddr
}

// Path returns the request path
func (c *testContext) Path() string {
	return c.request.URL.Path
}

// SetPath sets the request path
func (c *testContext) SetPath(p string) {
	c.request.URL.Path = p
}

// Param returns path parameter by name
func (c *testContext) Param(name string) string {
	return c.params[name]
}

// Params returns all path parameters
func (c *testContext) Params() url.Values {
	values := make(url.Values)
	for k, v := range c.params {
		values.Set(k, v)
	}
	return values
}

// ParamNames returns all path parameter names
func (c *testContext) ParamNames() []string {
	names := make([]string, 0, len(c.params))
	for k := range c.params {
		names = append(names, k)
	}
	return names
}

// SetParamNames sets the parameter names
func (c *testContext) SetParamNames(names ...string) {
	// Not implemented for test
}

// SetParamValues sets the parameter values
func (c *testContext) SetParamValues(values ...string) {
	// Not implemented for test
}

// QueryParam returns query parameter by name
func (c *testContext) QueryParam(name string) string {
	return c.request.URL.Query().Get(name)
}

// QueryParams returns all query parameters
func (c *testContext) QueryParams() url.Values {
	return c.request.URL.Query()
}

// QueryString returns the URL query string
func (c *testContext) QueryString() string {
	return c.request.URL.RawQuery
}

// FormValue returns form value by name
func (c *testContext) FormValue(name string) string {
	return c.request.FormValue(name)
}

// FormParams returns all form parameters
func (c *testContext) FormParams() (url.Values, error) {
	if err := c.request.ParseForm(); err != nil {
		return nil, err
	}
	return c.request.Form, nil
}

// FormFile returns the multipart form file for the provided name
func (c *testContext) FormFile(name string) (*multipart.FileHeader, error) {
	return nil, http.ErrNotMultipart
}

// MultipartForm returns the multipart form
func (c *testContext) MultipartForm() (*multipart.Form, error) {
	return nil, http.ErrNotMultipart
}

// Cookie returns the named cookie
func (c *testContext) Cookie(name string) (*http.Cookie, error) {
	return c.request.Cookie(name)
}

// SetCookie adds a Set-Cookie header
func (c *testContext) SetCookie(cookie *http.Cookie) {
	http.SetCookie(c.response, cookie)
}

// Cookies returns all cookies
func (c *testContext) Cookies() []*http.Cookie {
	return c.request.Cookies()
}

// Get retrieves data from the context
func (c *testContext) Get(key string) interface{} {
	return c.values[key]
}

// Set saves data in the context
func (c *testContext) Set(key string, val interface{}) {
	c.values[key] = val
}

// Bind binds the request body to an interface
func (c *testContext) Bind(i interface{}) error {
	return nil
}

// Validate validates an interface
func (c *testContext) Validate(i interface{}) error {
	return nil
}

// Render renders a template with data
func (c *testContext) Render(code int, name string, data interface{}) error {
	return nil
}

// HTML sends an HTTP response with HTML
func (c *testContext) HTML(code int, html string) error {
	c.response.Header().Set("Content-Type", "text/html; charset=utf-8")
	c.response.WriteHeader(code)
	_, err := c.response.Write([]byte(html))
	return err
}

// HTMLBlob sends an HTTP response with HTML blob
func (c *testContext) HTMLBlob(code int, b []byte) error {
	c.response.Header().Set("Content-Type", "text/html; charset=utf-8")
	c.response.WriteHeader(code)
	_, err := c.response.Write(b)
	return err
}

// String sends an HTTP response with string
func (c *testContext) String(code int, s string) error {
	c.response.Header().Set("Content-Type", "text/plain; charset=utf-8")
	c.response.WriteHeader(code)
	_, err := c.response.Write([]byte(s))
	return err
}

// JSON sends an HTTP response with JSON
func (c *testContext) JSON(code int, i interface{}) error {
	c.response.Header().Set("Content-Type", "application/json")
	c.response.WriteHeader(code)
	// Simple implementation for testing
	return nil
}

// JSONPretty sends an HTTP response with pretty JSON
func (c *testContext) JSONPretty(code int, i interface{}, indent string) error {
	return c.JSON(code, i)
}

// JSONBlob sends an HTTP response with JSON blob
func (c *testContext) JSONBlob(code int, b []byte) error {
	c.response.Header().Set("Content-Type", "application/json")
	c.response.WriteHeader(code)
	_, err := c.response.Write(b)
	return err
}

// JSONP sends an HTTP response with JSONP
func (c *testContext) JSONP(code int, callback string, i interface{}) error {
	return c.JSON(code, i)
}

// JSONPBlob sends an HTTP response with JSONP blob
func (c *testContext) JSONPBlob(code int, callback string, b []byte) error {
	return c.JSONBlob(code, b)
}

// XML sends an HTTP response with XML
func (c *testContext) XML(code int, i interface{}) error {
	c.response.Header().Set("Content-Type", "application/xml")
	c.response.WriteHeader(code)
	return nil
}

// XMLPretty sends an HTTP response with pretty XML
func (c *testContext) XMLPretty(code int, i interface{}, indent string) error {
	return c.XML(code, i)
}

// XMLBlob sends an HTTP response with XML blob
func (c *testContext) XMLBlob(code int, b []byte) error {
	c.response.Header().Set("Content-Type", "application/xml")
	c.response.WriteHeader(code)
	_, err := c.response.Write(b)
	return err
}

// Blob sends an HTTP response with blob
func (c *testContext) Blob(code int, contentType string, b []byte) error {
	c.response.Header().Set("Content-Type", contentType)
	c.response.WriteHeader(code)
	_, err := c.response.Write(b)
	return err
}

// Stream sends an HTTP response with stream
func (c *testContext) Stream(code int, contentType string, r io.Reader) error {
	c.response.Header().Set("Content-Type", contentType)
	c.response.WriteHeader(code)
	_, err := io.Copy(c.response, r)
	return err
}

// File sends a file as the response
func (c *testContext) File(file string) error {
	http.ServeFile(c.response, c.request, file)
	return nil
}

// Attachment sends a file as attachment
func (c *testContext) Attachment(file string, name string) error {
	return c.File(file)
}

// Inline sends a file inline
func (c *testContext) Inline(file string, name string) error {
	return c.File(file)
}

// NoContent sends a response with no body
func (c *testContext) NoContent(code int) error {
	c.response.WriteHeader(code)
	return nil
}

// Redirect redirects the request
func (c *testContext) Redirect(code int, url string) error {
	http.Redirect(c.response, c.request, url, code)
	return nil
}

// Error invokes the registered error handler
func (c *testContext) Error(err error) {
	// Not implemented for test
}

// Handler returns the registered handler
func (c *testContext) Handler() types.HandlerFunc {
	return nil
}

// SetHandler sets the matched handler
func (c *testContext) SetHandler(h types.HandlerFunc) {
	// Not implemented for test
}

// Logger returns the logger instance
func (c *testContext) Logger() interface{} {
	return nil
}

// SetLogger sets the logger instance
func (c *testContext) SetLogger(l interface{}) {
	// Not implemented for test
}

// Echo returns the context
func (c *testContext) Echo() interface{} {
	return nil
}

// Reset resets the context
func (c *testContext) Reset(r *http.Request, w http.ResponseWriter) {
	c.request = r
	c.response = NewTestResponseWriter(w)
	c.values = make(map[string]interface{})
}

// Context returns the standard context
func (c *testContext) Context() context.Context {
	return c.request.Context()
}

// Span returns the current trace span from context
func (c *testContext) Span() interface{} {
	if span, ok := c.values["enhanced_span"]; ok {
		return span
	}
	if span, ok := c.values["span"]; ok {
		return span
	}
	return nil
}

// testResponseWriter is a minimal implementation of types.ResponseWriter
type testResponseWriter struct {
	http.ResponseWriter
	status int
	size   int64
}

// NewTestResponseWriter creates a new test response writer
func NewTestResponseWriter(w http.ResponseWriter) types.ResponseWriter {
	return &testResponseWriter{
		ResponseWriter: w,
		status:         http.StatusOK,
	}
}

// Status returns the status code
func (w *testResponseWriter) Status() int {
	return w.status
}

// Size returns the size of response
func (w *testResponseWriter) Size() int64 {
	return w.size
}

// Written returns whether response was written
func (w *testResponseWriter) Written() bool {
	return w.status != 0
}

// WriteHeader writes the header
func (w *testResponseWriter) WriteHeader(code int) {
	if !w.Written() {
		w.status = code
		w.ResponseWriter.WriteHeader(code)
	}
}

// Write writes the data
func (w *testResponseWriter) Write(b []byte) (int, error) {
	if !w.Written() {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(b)
	w.size += int64(n)
	return n, err
}

// Flush flushes the response
func (w *testResponseWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}