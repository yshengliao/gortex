package http

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// compile time check to ensure DefaultContext implements Context
var _ Context = (*DefaultContext)(nil)

// DefaultContext is the default implementation of Context interface
type DefaultContext struct {
	request     *http.Request
	response    ResponseWriter
	path        string
	params      *smartParams // Use smart params for better performance
	handler     HandlerFunc
	store       Map
	lock        sync.RWMutex
	logger      interface{}
	echo        interface{} // For compatibility
	stdContext  context.Context
}

// NewDefaultContext creates a new DefaultContext
func NewDefaultContext(r *http.Request, w http.ResponseWriter) Context {
	return &DefaultContext{
		request:    r,
		response:   NewResponseWriter(w),
		params:     newSmartParams(),
		store:      make(Map),
		stdContext: r.Context(),
	}
}

// Request returns the HTTP request
func (c *DefaultContext) Request() *http.Request {
	return c.request
}

// SetRequest sets the HTTP request
func (c *DefaultContext) SetRequest(r *http.Request) {
	c.request = r
}

// Response returns the response writer
func (c *DefaultContext) Response() ResponseWriter {
	return c.response
}

// IsTLS returns true if the request is using TLS
func (c *DefaultContext) IsTLS() bool {
	return c.request.TLS != nil
}

// IsWebSocket returns true if the request is a WebSocket upgrade
func (c *DefaultContext) IsWebSocket() bool {
	upgrade := c.request.Header.Get(HeaderUpgrade)
	return strings.ToLower(upgrade) == "websocket"
}

// Scheme returns the HTTP protocol scheme
func (c *DefaultContext) Scheme() string {
	if c.IsTLS() {
		return "https"
	}
	if scheme := c.request.Header.Get(HeaderXForwardedProto); scheme != "" {
		return scheme
	}
	if scheme := c.request.Header.Get(HeaderXForwardedProtocol); scheme != "" {
		return scheme
	}
	if ssl := c.request.Header.Get(HeaderXForwardedSsl); ssl == "on" {
		return "https"
	}
	if scheme := c.request.Header.Get(HeaderXUrlScheme); scheme != "" {
		return scheme
	}
	return "http"
}

// RealIP returns the client's real IP address
func (c *DefaultContext) RealIP() string {
	if ip := c.request.Header.Get(HeaderXForwardedFor); ip != "" {
		i := strings.IndexAny(ip, ",")
		if i > 0 {
			return strings.TrimSpace(ip[:i])
		}
		return ip
	}
	if ip := c.request.Header.Get(HeaderXRealIP); ip != "" {
		return ip
	}
	ra, _, _ := net.SplitHostPort(c.request.RemoteAddr)
	return ra
}

// Path returns the registered path
func (c *DefaultContext) Path() string {
	return c.path
}

// SetPath sets the registered path
func (c *DefaultContext) SetPath(p string) {
	c.path = p
}

// Param returns path parameter by name
func (c *DefaultContext) Param(name string) string {
	if c.params == nil {
		return ""
	}
	return c.params.get(name)
}

// Params returns all path parameters
func (c *DefaultContext) Params() url.Values {
	values := make(url.Values)
	if c.params != nil {
		// Convert smartParams to url.Values
		// First, add values from the fixed arrays
		for i := 0; i < c.params.count && i < 4; i++ {
			values.Set(c.params.keys[i], c.params.vals[i])
		}
		// Then add any overflow values
		if c.params.overflow != nil {
			for k, v := range c.params.overflow {
				values.Set(k, v)
			}
		}
	}
	return values
}

// ParamNames returns all path parameter names
func (c *DefaultContext) ParamNames() []string {
	if c.params == nil {
		return nil
	}
	return c.params.names()
}

// SetParamNames sets path parameter names
func (c *DefaultContext) SetParamNames(names ...string) {
	if c.params == nil {
		c.params = newSmartParams()
	}
	// Reset and set new names
	c.params.reset()
	for _, name := range names {
		c.params.set(name, "")
	}
}

// ParamValues returns all path parameter values
func (c *DefaultContext) ParamValues() []string {
	if c.params == nil {
		return nil
	}
	return c.params.values()
}

// SetParamValues sets path parameter values
func (c *DefaultContext) SetParamValues(values ...string) {
	if c.params == nil {
		c.params = newSmartParams()
	}
	// Set values for existing names
	for i, value := range values {
		name, _ := c.params.getByIndex(i)
		if name != "" {
			c.params.setByIndex(i, name, value)
		}
	}
}

// QueryParam returns query parameter by name
func (c *DefaultContext) QueryParam(name string) string {
	return c.request.URL.Query().Get(name)
}

// QueryParams returns all query parameters
func (c *DefaultContext) QueryParams() url.Values {
	return c.request.URL.Query()
}

// QueryString returns the URL query string
func (c *DefaultContext) QueryString() string {
	return c.request.URL.RawQuery
}

// FormValue returns form value by name
func (c *DefaultContext) FormValue(name string) string {
	return c.request.FormValue(name)
}

// FormParams returns all form parameters
func (c *DefaultContext) FormParams() (url.Values, error) {
	if err := c.request.ParseForm(); err != nil {
		return nil, err
	}
	return c.request.Form, nil
}

// FormFile returns multipart form file by name
func (c *DefaultContext) FormFile(name string) (*multipart.FileHeader, error) {
	_, fh, err := c.request.FormFile(name)
	return fh, err
}

// MultipartForm returns multipart form
func (c *DefaultContext) MultipartForm() (*multipart.Form, error) {
	err := c.request.ParseMultipartForm(32 << 20) // 32 MB
	return c.request.MultipartForm, err
}

// Cookie returns cookie by name
func (c *DefaultContext) Cookie(name string) (*http.Cookie, error) {
	return c.request.Cookie(name)
}

// SetCookie sets a cookie
func (c *DefaultContext) SetCookie(cookie *http.Cookie) {
	http.SetCookie(c.response, cookie)
}

// Cookies returns all cookies
func (c *DefaultContext) Cookies() []*http.Cookie {
	return c.request.Cookies()
}

// Get retrieves data from context
func (c *DefaultContext) Get(key string) interface{} {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.store[key]
}

// Set saves data in context
func (c *DefaultContext) Set(key string, val interface{}) {
	c.lock.Lock()
	defer c.lock.Unlock()
	if c.store == nil {
		c.store = make(Map)
	}
	c.store[key] = val
}

// Bind binds request body to interface
func (c *DefaultContext) Bind(i interface{}) error {
	// This is a simplified implementation
	// In production, you would use a proper binding library
	contentType := c.request.Header.Get(HeaderContentType)
	switch {
	case strings.HasPrefix(contentType, MIMEApplicationJSON):
		return json.NewDecoder(c.request.Body).Decode(i)
	case strings.HasPrefix(contentType, MIMEApplicationXML), strings.HasPrefix(contentType, MIMETextXML):
		return xml.NewDecoder(c.request.Body).Decode(i)
	default:
		return ErrUnsupportedMediaType
	}
}

// Validate validates the bound struct
func (c *DefaultContext) Validate(i interface{}) error {
	// This should be implemented with a validator
	// For now, return nil
	return nil
}

// Render renders a template with data
func (c *DefaultContext) Render(code int, name string, data interface{}) error {
	// This should be implemented with a template engine
	// For now, return an error
	return fmt.Errorf("render not implemented")
}

// JSON sends a JSON response with status code
func (c *DefaultContext) JSON(code int, i interface{}) error {
	c.writeContentType(MIMEApplicationJSONCharsetUTF8)
	c.response.WriteHeader(code)
	encoder := json.NewEncoder(c.response)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(i)
}

// JSONBlob sends a JSON blob response with status code
func (c *DefaultContext) JSONBlob(code int, b []byte) error {
	return c.Blob(code, MIMEApplicationJSONCharsetUTF8, b)
}

// JSONPretty sends a pretty-printed JSON response
func (c *DefaultContext) JSONPretty(code int, i interface{}, indent string) error {
	c.writeContentType(MIMEApplicationJSONCharsetUTF8)
	c.response.WriteHeader(code)
	encoder := json.NewEncoder(c.response)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", indent)
	return encoder.Encode(i)
}

// JSONByte sends a JSON byte response
func (c *DefaultContext) JSONByte(code int, b []byte) error {
	return c.JSONBlob(code, b)
}

// JSONP sends a JSONP response
func (c *DefaultContext) JSONP(code int, callback string, i interface{}) error {
	b, err := json.Marshal(i)
	if err != nil {
		return err
	}
	return c.JSONPBlob(code, callback, b)
}

// JSONPBlob sends a JSONP blob response
func (c *DefaultContext) JSONPBlob(code int, callback string, b []byte) error {
	c.writeContentType(MIMEApplicationJavaScriptCharsetUTF8)
	c.response.WriteHeader(code)
	if _, err := c.response.Write([]byte(callback + "(")); err != nil {
		return err
	}
	if _, err := c.response.Write(b); err != nil {
		return err
	}
	_, err := c.response.Write([]byte(");"))
	return err
}

// XML sends an XML response with status code
func (c *DefaultContext) XML(code int, i interface{}) error {
	c.writeContentType(MIMEApplicationXMLCharsetUTF8)
	c.response.WriteHeader(code)
	encoder := xml.NewEncoder(c.response)
	return encoder.Encode(i)
}

// XMLBlob sends an XML blob response
func (c *DefaultContext) XMLBlob(code int, b []byte) error {
	return c.Blob(code, MIMEApplicationXMLCharsetUTF8, b)
}

// XMLPretty sends a pretty-printed XML response
func (c *DefaultContext) XMLPretty(code int, i interface{}, indent string) error {
	c.writeContentType(MIMEApplicationXMLCharsetUTF8)
	c.response.WriteHeader(code)
	encoder := xml.NewEncoder(c.response)
	encoder.Indent("", indent)
	return encoder.Encode(i)
}

// Blob sends a blob response with content type
func (c *DefaultContext) Blob(code int, contentType string, b []byte) error {
	c.writeContentType(contentType)
	c.response.WriteHeader(code)
	_, err := c.response.Write(b)
	return err
}

// Stream sends a streaming response with content type
func (c *DefaultContext) Stream(code int, contentType string, r io.Reader) error {
	c.writeContentType(contentType)
	c.response.WriteHeader(code)
	_, err := io.Copy(c.response, r)
	return err
}

// File sends a file as response
func (c *DefaultContext) File(file string) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()
	
	fi, err := f.Stat()
	if err != nil {
		return err
	}
	
	if fi.IsDir() {
		file = filepath.Join(file, "index.html")
		f, err = os.Open(file)
		if err != nil {
			return err
		}
		defer f.Close()
		fi, err = f.Stat()
		if err != nil {
			return err
		}
	}
	
	http.ServeContent(c.response, c.request, fi.Name(), fi.ModTime(), f)
	return nil
}

// Inline sends a file as inline
func (c *DefaultContext) Inline(file, name string) error {
	return c.contentDisposition(file, name, "inline")
}

// Attachment sends a file as attachment
func (c *DefaultContext) Attachment(file, name string) error {
	return c.contentDisposition(file, name, "attachment")
}

// NoContent sends a response with no body
func (c *DefaultContext) NoContent(code int) error {
	c.response.WriteHeader(code)
	return nil
}

// String sends a string response
func (c *DefaultContext) String(code int, s string) error {
	return c.Blob(code, MIMETextPlainCharsetUTF8, []byte(s))
}

// HTML sends an HTML response
func (c *DefaultContext) HTML(code int, html string) error {
	return c.HTMLBlob(code, []byte(html))
}

// HTMLBlob sends an HTML blob response
func (c *DefaultContext) HTMLBlob(code int, b []byte) error {
	return c.Blob(code, MIMETextHTMLCharsetUTF8, b)
}

// Redirect redirects the request
func (c *DefaultContext) Redirect(code int, url string) error {
	if code < 300 || code > 308 {
		return ErrInvalidRedirectCode
	}
	c.response.Header().Set(HeaderLocation, url)
	c.response.WriteHeader(code)
	return nil
}

// Error invokes the registered error handler
func (c *DefaultContext) Error(err error) {
	// This should be handled by the framework's error handler
	// For now, just write the error
	if httpErr, ok := err.(*HTTPError); ok {
		c.String(httpErr.Code, httpErr.Error())
	} else {
		c.String(http.StatusInternalServerError, err.Error())
	}
}

// Handler returns the handler
func (c *DefaultContext) Handler() HandlerFunc {
	return c.handler
}

// SetHandler sets the handler
func (c *DefaultContext) SetHandler(h HandlerFunc) {
	c.handler = h
}

// Logger returns the logger
func (c *DefaultContext) Logger() interface{} {
	return c.logger
}

// SetLogger sets the logger
func (c *DefaultContext) SetLogger(l interface{}) {
	c.logger = l
}

// Echo returns the Echo instance (for compatibility)
func (c *DefaultContext) Echo() interface{} {
	return c.echo
}

// Reset resets the context
func (c *DefaultContext) Reset(r *http.Request, w http.ResponseWriter) {
	c.request = r
	c.response = NewResponseWriter(w)
	c.path = ""
	// Reset params instead of allocating new
	if c.params == nil {
		c.params = newSmartParams()
	} else {
		c.params.reset()
	}
	c.handler = nil
	// Keep the store allocated but clear it
	if c.store == nil {
		c.store = make(Map)
	} else {
		for k := range c.store {
			delete(c.store, k)
		}
	}
	c.stdContext = r.Context()
}

// Context returns the standard context.Context
func (c *DefaultContext) Context() context.Context {
	if c.stdContext == nil {
		c.stdContext = c.request.Context()
	}
	return c.stdContext
}

// StdContext returns the standard context.Context
func (c *DefaultContext) StdContext() context.Context {
	return c.Context()
}

// SetStdContext sets the standard context.Context
func (c *DefaultContext) SetStdContext(ctx context.Context) {
	c.stdContext = ctx
	c.request = c.request.WithContext(ctx)
}

// writeContentType writes the content type header
func (c *DefaultContext) writeContentType(value string) {
	header := c.response.Header()
	if header.Get(HeaderContentType) == "" {
		header.Set(HeaderContentType, value)
	}
}

// contentDisposition sends a file with content disposition header
func (c *DefaultContext) contentDisposition(file, name, dispositionType string) error {
	c.response.Header().Set(HeaderContentDisposition, fmt.Sprintf("%s; filename=%q", dispositionType, name))
	return c.File(file)
}