package http

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"io/fs"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
)

// DefaultMaxMultipartBytes is the default memory cap that
// (*DefaultContext).MultipartForm and FormFile apply when parsing
// multipart bodies. Bytes beyond this budget spill to tmp files. Larger
// values let bigger payloads stay in RAM (cheaper to consume) at the
// cost of higher memory pressure per in-flight request; smaller values
// are safer but may slow file uploads.
const DefaultMaxMultipartBytes int64 = 32 << 20 // 32 MiB

// DefaultMaxBodyBytes is the maximum number of bytes that Bind() will
// read from a request body when decoding JSON or XML. Requests whose
// bodies exceed this limit are rejected with an error. The value matches
// the cap applied by core/context.ParameterBinder so both code paths
// enforce the same policy.
const DefaultMaxBodyBytes int64 = 1 << 20 // 1 MiB

// maxMultipartBytesOverride holds a process-wide override set by
// SetDefaultMaxMultipartBytes. Stored atomically so app startup and
// request handling can race freely. A value of 0 (the zero-initialised
// state) means "use DefaultMaxMultipartBytes".
var maxMultipartBytesOverride atomic.Int64

// SetDefaultMaxMultipartBytes changes the multipart in-memory cap used by
// every subsequent MultipartForm / FormFile call. Pass a value <= 0 to
// restore DefaultMaxMultipartBytes. Typical callers wire this from
// application configuration during startup.
func SetDefaultMaxMultipartBytes(n int64) {
	if n <= 0 {
		maxMultipartBytesOverride.Store(0)
		return
	}
	maxMultipartBytesOverride.Store(n)
}

// effectiveMaxMultipartBytes resolves the current cap.
func effectiveMaxMultipartBytes() int64 {
	if v := maxMultipartBytesOverride.Load(); v > 0 {
		return v
	}
	return DefaultMaxMultipartBytes
}

// compile time check to ensure DefaultContext implements Context
var _ Context = (*DefaultContext)(nil)

// DefaultContext is the default implementation of Context interface
type DefaultContext struct {
	request     *http.Request
	response    ResponseWriter
	rw          responseWriter   // embedded value; same lifetime as the pooled context
	path        string
	params      *smartParams // Use smart params for better performance
	handler     HandlerFunc
	store       Map
	lock        sync.RWMutex
	logger      interface{}
	stdContext  context.Context
}

// NewDefaultContext creates a new DefaultContext
func NewDefaultContext(r *http.Request, w http.ResponseWriter) Context {
	dc := &DefaultContext{
		request:    r,
		params:     newSmartParams(),
		store:      make(Map),
		stdContext: r.Context(),
	}
	dc.rw.reset(w)
	dc.response = &dc.rw
	return dc
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

// RealIP returns the client's real IP address.
//
// SECURITY WARNING: This method unconditionally trusts the X-Forwarded-For
// and X-Real-IP headers. In environments where clients can set these headers
// directly (no trusted reverse proxy in front of the server), the returned
// value can be spoofed by any client. Do not use as a sole security control
// (e.g., rate-limit key, IP allow-list) without ensuring that only a trusted
// reverse proxy can reach the server and that the proxy strips client-supplied
// forwarding headers before adding its own.
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
	if c.request.MultipartForm == nil {
		if err := c.request.ParseMultipartForm(effectiveMaxMultipartBytes()); err != nil {
			return nil, err
		}
	}
	_, fh, err := c.request.FormFile(name)
	return fh, err
}

// MultipartForm returns multipart form
func (c *DefaultContext) MultipartForm() (*multipart.Form, error) {
	err := c.request.ParseMultipartForm(effectiveMaxMultipartBytes())
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

// Bind binds request body to interface.
//
// The request body is capped at DefaultMaxBodyBytes (1 MiB) via
// http.MaxBytesReader to prevent oversized-payload DoS attacks. Requests
// whose body exceeds the limit will cause Decode to return an error
// containing "http: request body too large".
func (c *DefaultContext) Bind(i interface{}) error {
	// Cap body size before any decoding to prevent DoS via oversized payloads.
	limited := http.MaxBytesReader(c.response, c.request.Body, DefaultMaxBodyBytes)
	contentType := c.request.Header.Get(HeaderContentType)
	switch {
	case strings.HasPrefix(contentType, MIMEApplicationJSON):
		return json.NewDecoder(limited).Decode(i)
	case strings.HasPrefix(contentType, MIMEApplicationXML), strings.HasPrefix(contentType, MIMETextXML):
		return xml.NewDecoder(limited).Decode(i)
	default:
		return ErrUnsupportedMediaType
	}
}

// Validate validates the bound struct.
//
// This method is a placeholder — no validator is registered by default.
// It always returns ErrValidatorNotRegistered to fail loudly rather than
// silently passing unvalidated data to the application.
// Integrate a validator (e.g. github.com/go-playground/validator) and
// call it here, or wrap this context to provide a custom implementation.
func (c *DefaultContext) Validate(i interface{}) error {
	return ErrValidatorNotRegistered
}

// Render renders a template with data.
//
// This method is a placeholder — no template engine is registered by default.
// Integrate a template engine and override this method, or wrap this context
// to provide a custom implementation.
func (c *DefaultContext) Render(code int, name string, data interface{}) error {
	return fmt.Errorf("Render is not implemented: register a template engine before calling Render")
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

// File sends a file as the response.
//
// The supplied path is treated as server-trusted. To defend against
// accidental path-traversal when callers forward user input, the path
// is cleaned and any ".." segments are rejected. For user-supplied
// filenames, prefer FileFS with an explicit root.
func (c *DefaultContext) File(file string) error {
	cleaned, err := safeServerPath(file)
	if err != nil {
		return err
	}

	f, err := os.Open(cleaned)
	if err != nil {
		return err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return err
	}

	if fi.IsDir() {
		indexPath := filepath.Join(cleaned, "index.html")
		f, err = os.Open(indexPath)
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

// FileFS serves a file from the given filesystem root. The name is
// validated via fs.ValidPath, which rejects absolute paths, ".."
// segments, and other escapes, making this the safe choice for
// serving user-supplied filenames.
func (c *DefaultContext) FileFS(fsys fs.FS, name string) error {
	if !fs.ValidPath(name) {
		return ErrUnsafeFilePath
	}

	f, err := fsys.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return err
	}

	if fi.IsDir() {
		indexName := path.Join(name, "index.html")
		if !fs.ValidPath(indexName) {
			return ErrUnsafeFilePath
		}
		f, err = fsys.Open(indexName)
		if err != nil {
			return err
		}
		defer f.Close()
		fi, err = f.Stat()
		if err != nil {
			return err
		}
	}

	rs, ok := f.(io.ReadSeeker)
	if !ok {
		b, err := io.ReadAll(f)
		if err != nil {
			return err
		}
		c.writeContentType(http.DetectContentType(b))
		_, err = c.response.Write(b)
		return err
	}
	http.ServeContent(c.response, c.request, fi.Name(), fi.ModTime(), rs)
	return nil
}

// safeServerPath cleans a server-trusted file path and rejects it if
// it contains any ".." traversal segments after cleaning.
func safeServerPath(file string) (string, error) {
	if file == "" {
		return "", ErrUnsafeFilePath
	}
	cleaned := filepath.Clean(file)
	for _, seg := range strings.Split(cleaned, string(filepath.Separator)) {
		if seg == ".." {
			return "", ErrUnsafeFilePath
		}
	}
	return cleaned, nil
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

// Redirect redirects the request.
//
// For safety, only same-origin paths are accepted by default: the URL
// must start with "/" and must not start with "//" (protocol-relative).
// Callers that legitimately need to redirect to an external host
// should write the Location header and status code directly.
func (c *DefaultContext) Redirect(code int, target string) error {
	if code < 300 || code > 308 {
		return ErrInvalidRedirectCode
	}
	if !isSafeRedirectTarget(target) {
		return ErrUnsafeRedirectURL
	}
	c.response.Header().Set(HeaderLocation, target)
	c.response.WriteHeader(code)
	return nil
}

// isSafeRedirectTarget returns true when target is a relative path
// that cannot be coerced into an off-site navigation.
func isSafeRedirectTarget(target string) bool {
	if target == "" {
		return false
	}
	if strings.HasPrefix(target, "//") {
		return false
	}
	if !strings.HasPrefix(target, "/") {
		return false
	}
	// Reject control characters that could break out of the Location header.
	for _, r := range target {
		if r == '\r' || r == '\n' || r == 0 {
			return false
		}
	}
	return true
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

// Reset resets the context
func (c *DefaultContext) Reset(r *http.Request, w http.ResponseWriter) {
	c.request = r
	c.rw.reset(w)
	c.response = &c.rw
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

// Span returns the current trace span from context
func (c *DefaultContext) Span() interface{} {
	// Try to get enhanced span first
	if span := c.Get("enhanced_span"); span != nil {
		return span
	}
	// Fall back to regular span
	if span := c.Get("span"); span != nil {
		return span
	}
	return nil
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