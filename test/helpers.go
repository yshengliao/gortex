package test

import (
	"bytes"
	stdContext "context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/yshengliao/gortex/app"
	"github.com/yshengliao/gortex/context"
	"github.com/yshengliao/gortex/router"
)

// TestApp creates a test application with handlers
func TestApp(handlers interface{}) (*app.App, error) {
	cfg := &app.Config{}
	cfg.Server.Address = ":0"  // Random port
	cfg.Logger.Level = "error" // Quiet logging

	return app.NewApp(
		app.WithConfig(cfg),
		app.WithHandlers(handlers),
	)
}

// Request performs a test HTTP request
func Request(app *app.App, method, path string, body interface{}) *httptest.ResponseRecorder {
	var bodyReader io.Reader

	if body != nil {
		switch v := body.(type) {
		case string:
			bodyReader = strings.NewReader(v)
		case []byte:
			bodyReader = bytes.NewReader(v)
		case io.Reader:
			bodyReader = v
		default:
			data, _ := json.Marshal(body)
			bodyReader = bytes.NewReader(data)
		}
	}

	req := httptest.NewRequest(method, path, bodyReader)
	if body != nil && bodyReader != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	rec := httptest.NewRecorder()
	router := app.Router()
	router.ServeHTTP(rec, req)

	return rec
}

// RequestWithHeaders performs a test HTTP request with custom headers
func RequestWithHeaders(app *app.App, method, path string, headers map[string]string, body interface{}) *httptest.ResponseRecorder {
	var bodyReader io.Reader

	if body != nil {
		switch v := body.(type) {
		case string:
			bodyReader = strings.NewReader(v)
		case []byte:
			bodyReader = bytes.NewReader(v)
		case io.Reader:
			bodyReader = v
		default:
			data, _ := json.Marshal(body)
			bodyReader = bytes.NewReader(data)
		}
	}

	req := httptest.NewRequest(method, path, bodyReader)

	// Set headers
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	// Set content type if body exists and not already set
	if body != nil && bodyReader != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	rec := httptest.NewRecorder()
	router := app.Router()
	router.ServeHTTP(rec, req)

	return rec
}

// JSONResponse parses JSON response body
func JSONResponse(rec *httptest.ResponseRecorder, v interface{}) error {
	return json.NewDecoder(rec.Body).Decode(v)
}

// MockContext creates a mock context for unit testing
type MockContext struct {
	req         *http.Request
	res         context.ResponseWriter
	params      map[string]string
	paramNames  []string
	paramValues []string
	store       map[string]interface{}
	path        string
	handler     context.HandlerFunc
	logger      context.Logger
	recorder    *httptest.ResponseRecorder
}

// NewMockContext creates a new mock context
func NewMockContext(method, path string) *MockContext {
	req := httptest.NewRequest(method, path, nil)
	rec := httptest.NewRecorder()

	return &MockContext{
		req:      req,
		res:      context.NewResponseWriter(rec),
		params:   make(map[string]string),
		store:    make(map[string]interface{}),
		path:     path,
		recorder: rec,
	}
}

// WithJSON sets JSON body
func (c *MockContext) WithJSON(data interface{}) *MockContext {
	body, _ := json.Marshal(data)
	c.req = httptest.NewRequest(c.req.Method, c.req.URL.String(), bytes.NewReader(body))
	c.req.Header.Set("Content-Type", "application/json")
	return c
}

// WithQuery adds query parameters
func (c *MockContext) WithQuery(key, value string) *MockContext {
	q := c.req.URL.Query()
	q.Add(key, value)
	c.req.URL.RawQuery = q.Encode()
	return c
}

// WithParam sets path parameters
func (c *MockContext) WithParam(key, value string) *MockContext {
	c.params[key] = value
	return c
}

// WithHeader sets a header
func (c *MockContext) WithHeader(key, value string) *MockContext {
	c.req.Header.Set(key, value)
	return c
}

// Build creates a Gortex context from the mock
func (c *MockContext) Build() context.Context {
	return c
}

// Response returns the response writer
func (c *MockContext) Response() context.ResponseWriter {
	return c.res
}

// ResponseRecorder returns the httptest.ResponseRecorder for assertions
func (c *MockContext) ResponseRecorder() *httptest.ResponseRecorder {
	return c.recorder
}

// SimpleRouter creates a simple router for testing
func SimpleRouter() router.GortexRouter {
	return router.NewGortexRouter()
}

// Request returns the HTTP request
func (c *MockContext) Request() *http.Request {
	return c.req
}

// SetRequest sets the HTTP request
func (c *MockContext) SetRequest(r *http.Request) {
	c.req = r
}


// IsTLS returns true if the request is using TLS
func (c *MockContext) IsTLS() bool {
	return c.req.TLS != nil
}

// IsWebSocket returns true if the request is a WebSocket upgrade
func (c *MockContext) IsWebSocket() bool {
	upgrade := c.req.Header.Get("Upgrade")
	return strings.EqualFold(upgrade, "websocket")
}

// Scheme returns the HTTP protocol scheme
func (c *MockContext) Scheme() string {
	if c.IsTLS() {
		return "https"
	}
	return "http"
}

// RealIP returns the client's real IP address
func (c *MockContext) RealIP() string {
	return "127.0.0.1"
}

// Path returns the request URL path
func (c *MockContext) Path() string {
	return c.path
}

// SetPath sets the request URL path
func (c *MockContext) SetPath(p string) {
	c.path = p
}

// Param returns path parameter by name
func (c *MockContext) Param(name string) string {
	return c.params[name]
}

// ParamNames returns all path parameter names
func (c *MockContext) ParamNames() []string {
	return c.paramNames
}

// SetParamNames sets path parameter names
func (c *MockContext) SetParamNames(names ...string) {
	c.paramNames = names
}

// ParamValues returns all path parameter values
func (c *MockContext) ParamValues() []string {
	return c.paramValues
}

// SetParamValues sets path parameter values
func (c *MockContext) SetParamValues(values ...string) {
	c.paramValues = values
}

// QueryParam returns query parameter by name
func (c *MockContext) QueryParam(name string) string {
	return c.req.URL.Query().Get(name)
}

// QueryParams returns all query parameters
func (c *MockContext) QueryParams() url.Values {
	return c.req.URL.Query()
}

// QueryString returns the URL query string
func (c *MockContext) QueryString() string {
	return c.req.URL.RawQuery
}

// FormValue returns form value by name
func (c *MockContext) FormValue(name string) string {
	return c.req.FormValue(name)
}

// FormParams returns all form parameters
func (c *MockContext) FormParams() (url.Values, error) {
	if err := c.req.ParseForm(); err != nil {
		return nil, err
	}
	return c.req.Form, nil
}

// FormFile returns uploaded file by name
func (c *MockContext) FormFile(name string) (*multipart.FileHeader, error) {
	_, fh, err := c.req.FormFile(name)
	return fh, err
}

// MultipartForm returns multipart form
func (c *MockContext) MultipartForm() (*multipart.Form, error) {
	if err := c.req.ParseMultipartForm(32 << 20); err != nil {
		return nil, err
	}
	return c.req.MultipartForm, nil
}

// Cookie returns cookie by name
func (c *MockContext) Cookie(name string) (*http.Cookie, error) {
	return c.req.Cookie(name)
}

// SetCookie sets a cookie
func (c *MockContext) SetCookie(cookie *http.Cookie) {
	http.SetCookie(c.res, cookie)
}

// Cookies returns all cookies
func (c *MockContext) Cookies() []*http.Cookie {
	return c.req.Cookies()
}

// Get retrieves data from context
func (c *MockContext) Get(key string) interface{} {
	return c.store[key]
}

// Set saves data in context
func (c *MockContext) Set(key string, val interface{}) {
	c.store[key] = val
}

// Bind binds request data to interface
func (c *MockContext) Bind(i interface{}) error {
	if c.req.ContentLength == 0 {
		return nil
	}

	contentType := c.req.Header.Get("Content-Type")
	switch {
	case strings.HasPrefix(contentType, "application/json"):
		return json.NewDecoder(c.req.Body).Decode(i)
	case strings.HasPrefix(contentType, "application/xml"):
		return xml.NewDecoder(c.req.Body).Decode(i)
	default:
		return fmt.Errorf("unsupported content type: %s", contentType)
	}
}

// Validate validates the provided interface
func (c *MockContext) Validate(i interface{}) error {
	return nil // Mock validation
}

// JSON sends a JSON response
func (c *MockContext) JSON(code int, i interface{}) error {
	c.res.Header().Set("Content-Type", "application/json")
	c.res.WriteHeader(code)
	return json.NewEncoder(c.res).Encode(i)
}

// JSONPretty sends a pretty JSON response
func (c *MockContext) JSONPretty(code int, i interface{}, indent string) error {
	c.res.Header().Set("Content-Type", "application/json")
	c.res.WriteHeader(code)
	enc := json.NewEncoder(c.res)
	enc.SetIndent("", indent)
	return enc.Encode(i)
}

// JSONBlob sends a JSON blob response
func (c *MockContext) JSONBlob(code int, b []byte) error {
	return c.Blob(code, "application/json", b)
}

// JSONByte sends a JSON byte response
func (c *MockContext) JSONByte(code int, b []byte) error {
	return c.JSONBlob(code, b)
}

// JSONP sends a JSONP response
func (c *MockContext) JSONP(code int, callback string, i interface{}) error {
	c.res.Header().Set("Content-Type", "application/javascript")
	c.res.WriteHeader(code)

	if _, err := c.res.Write([]byte(callback + "(")); err != nil {
		return err
	}
	if err := json.NewEncoder(c.res).Encode(i); err != nil {
		return err
	}
	_, err := c.res.Write([]byte(");"))
	return err
}

// JSONPBlob sends a JSONP blob response
func (c *MockContext) JSONPBlob(code int, callback string, b []byte) error {
	c.res.Header().Set("Content-Type", "application/javascript")
	c.res.WriteHeader(code)

	if _, err := c.res.Write([]byte(callback + "(")); err != nil {
		return err
	}
	if _, err := c.res.Write(b); err != nil {
		return err
	}
	_, err := c.res.Write([]byte(");"))
	return err
}

// XML sends an XML response
func (c *MockContext) XML(code int, i interface{}) error {
	c.res.Header().Set("Content-Type", "application/xml")
	c.res.WriteHeader(code)
	return xml.NewEncoder(c.res).Encode(i)
}

// XMLPretty sends a pretty XML response
func (c *MockContext) XMLPretty(code int, i interface{}, indent string) error {
	c.res.Header().Set("Content-Type", "application/xml")
	c.res.WriteHeader(code)
	enc := xml.NewEncoder(c.res)
	enc.Indent("", indent)
	return enc.Encode(i)
}

// XMLBlob sends an XML blob response
func (c *MockContext) XMLBlob(code int, b []byte) error {
	return c.Blob(code, "application/xml", b)
}

// HTML sends an HTML response
func (c *MockContext) HTML(code int, html string) error {
	return c.HTMLBlob(code, []byte(html))
}

// HTMLBlob sends an HTML blob response
func (c *MockContext) HTMLBlob(code int, b []byte) error {
	return c.Blob(code, "text/html", b)
}

// String sends a string response
func (c *MockContext) String(code int, s string) error {
	return c.Blob(code, "text/plain", []byte(s))
}

// Blob sends a blob response
func (c *MockContext) Blob(code int, contentType string, b []byte) error {
	c.res.Header().Set("Content-Type", contentType)
	c.res.WriteHeader(code)
	_, err := c.res.Write(b)
	return err
}

// Stream sends a streaming response
func (c *MockContext) Stream(code int, contentType string, r io.Reader) error {
	c.res.Header().Set("Content-Type", contentType)
	c.res.WriteHeader(code)
	_, err := io.Copy(c.res, r)
	return err
}

// File sends a file
func (c *MockContext) File(file string) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return err
	}

	http.ServeContent(c.res, c.req, fi.Name(), fi.ModTime(), f)
	return nil
}

// Attachment sends a response as attachment
func (c *MockContext) Attachment(file, name string) error {
	c.res.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name))
	return c.File(file)
}

// Inline sends a response as inline
func (c *MockContext) Inline(file, name string) error {
	c.res.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", name))
	return c.File(file)
}

// NoContent sends a no content response
func (c *MockContext) NoContent(code int) error {
	c.res.WriteHeader(code)
	return nil
}

// Redirect redirects the request
func (c *MockContext) Redirect(code int, url string) error {
	if code < 300 || code > 308 {
		return fmt.Errorf("invalid redirect status code")
	}
	c.res.Header().Set("Location", url)
	c.res.WriteHeader(code)
	return nil
}

// Error writes an error
func (c *MockContext) Error(err error) {
	c.res.WriteHeader(http.StatusInternalServerError)
	c.res.Write([]byte(err.Error()))
}

// Handler returns the handler
func (c *MockContext) Handler() context.HandlerFunc {
	return c.handler
}

// SetHandler sets the handler
func (c *MockContext) SetHandler(h context.HandlerFunc) {
	c.handler = h
}

// Logger returns the logger
func (c *MockContext) Logger() context.Logger {
	return c.logger
}

// SetLogger sets the logger
func (c *MockContext) SetLogger(l context.Logger) {
	c.logger = l
}

// Echo returns the echo instance (for compatibility)
func (c *MockContext) Echo() interface{} {
	return nil
}

// Reset resets the context
func (c *MockContext) Reset(r *http.Request, w http.ResponseWriter) {
	c.req = r
	c.res = context.NewResponseWriter(w)
	c.recorder = httptest.NewRecorder()
	c.path = ""
	c.params = make(map[string]string)
	c.paramNames = nil
	c.paramValues = nil
	c.handler = nil
	c.store = make(map[string]interface{})
}

// StdContext returns the standard context.Context
func (c *MockContext) StdContext() stdContext.Context {
	return c.req.Context()
}

// SetStdContext sets the standard context.Context
func (c *MockContext) SetStdContext(ctx stdContext.Context) {
	c.req = c.req.WithContext(ctx)
}

// ParamInt returns path parameter as int with default value
func (c *MockContext) ParamInt(name string, defaultValue int) int {
	param := c.Param(name)
	if param == "" {
		return defaultValue
	}
	if value, err := strconv.Atoi(param); err == nil {
		return value
	}
	return defaultValue
}

// QueryInt returns query parameter as int with default value
func (c *MockContext) QueryInt(name string, defaultValue int) int {
	query := c.QueryParam(name)
	if query == "" {
		return defaultValue
	}
	if value, err := strconv.Atoi(query); err == nil {
		return value
	}
	return defaultValue
}

// QueryBool returns query parameter as bool with default value
func (c *MockContext) QueryBool(name string, defaultValue bool) bool {
	query := c.QueryParam(name)
	if query == "" {
		return defaultValue
	}
	if value, err := strconv.ParseBool(query); err == nil {
		return value
	}
	return defaultValue
}

// OK sends a successful response with data (200 OK)
func (c *MockContext) OK(data interface{}) error {
	return c.JSON(http.StatusOK, data)
}

// Created sends a created response with data (201 Created)
func (c *MockContext) Created(data interface{}) error {
	return c.JSON(http.StatusCreated, data)
}

// NoContent204 sends a no content response (204 No Content)
func (c *MockContext) NoContent204() error {
	return c.NoContent(http.StatusNoContent)
}

// BadRequest sends a bad request response with message (400 Bad Request)
func (c *MockContext) BadRequest(message string) error {
	return c.JSON(http.StatusBadRequest, map[string]string{
		"error": message,
	})
}
