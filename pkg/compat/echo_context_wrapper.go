package compat

import (
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	
	"github.com/labstack/echo/v4"
	"github.com/yshengliao/gortex/pkg/router"
)

// echoContextWrapper implements echo.Context interface wrapping router.Context
type echoContextWrapper struct {
	router.Context
	request     *http.Request
	response    *echo.Response
	path        string
	paramNames  []string
	paramValues []string
	values      map[string]interface{}
}

// newEchoContextWrapper creates a new Echo context wrapper
func newEchoContextWrapper(c router.Context) *echoContextWrapper {
	req, _ := c.Request().(*http.Request)
	resp, _ := c.Response().(http.ResponseWriter)
	
	return &echoContextWrapper{
		Context:     c,
		request:     req,
		response:    echo.NewResponse(resp, echo.New()),
		values:      make(map[string]interface{}),
		paramNames:  []string{},
		paramValues: []string{},
	}
}

// Request returns `*http.Request`.
func (e *echoContextWrapper) Request() *http.Request {
	return e.request
}

// SetRequest sets `*http.Request`.
func (e *echoContextWrapper) SetRequest(r *http.Request) {
	e.request = r
}

// Response returns `*Response`.
func (e *echoContextWrapper) Response() *echo.Response {
	return e.response
}

// SetResponse sets `*Response`.
func (e *echoContextWrapper) SetResponse(r *echo.Response) {
	e.response = r
}

// IsTLS returns true if HTTP connection is TLS otherwise false.
func (e *echoContextWrapper) IsTLS() bool {
	return e.request.TLS != nil
}

// IsWebSocket returns true if HTTP connection is WebSocket otherwise false.
func (e *echoContextWrapper) IsWebSocket() bool {
	upgrade := e.request.Header.Get("Upgrade")
	return upgrade == "websocket"
}

// Scheme returns the HTTP protocol scheme, `http` or `https`.
func (e *echoContextWrapper) Scheme() string {
	if e.IsTLS() {
		return "https"
	}
	return "http"
}

// RealIP returns the client's network address.
func (e *echoContextWrapper) RealIP() string {
	if ip := e.request.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	if ip := e.request.Header.Get("X-Forwarded-For"); ip != "" {
		return ip
	}
	return e.request.RemoteAddr
}

// Path returns the registered path for the handler.
func (e *echoContextWrapper) Path() string {
	return e.path
}

// SetPath sets the registered path for the handler.
func (e *echoContextWrapper) SetPath(p string) {
	e.path = p
}

// Param returns path parameter by name.
func (e *echoContextWrapper) Param(name string) string {
	return e.Context.Param(name)
}

// ParamNames returns path parameter names.
func (e *echoContextWrapper) ParamNames() []string {
	return e.paramNames
}

// SetParamNames sets path parameter names.
func (e *echoContextWrapper) SetParamNames(names ...string) {
	e.paramNames = names
}

// ParamValues returns path parameter values.
func (e *echoContextWrapper) ParamValues() []string {
	return e.paramValues
}

// SetParamValues sets path parameter values.
func (e *echoContextWrapper) SetParamValues(values ...string) {
	e.paramValues = values
}

// QueryParam returns the query param for the provided name.
func (e *echoContextWrapper) QueryParam(name string) string {
	return e.Context.QueryParam(name)
}

// QueryParams returns the query parameters as `url.Values`.
func (e *echoContextWrapper) QueryParams() url.Values {
	return e.request.URL.Query()
}

// QueryString returns the URL query string.
func (e *echoContextWrapper) QueryString() string {
	return e.request.URL.RawQuery
}

// FormValue returns the form field value for the provided name.
func (e *echoContextWrapper) FormValue(name string) string {
	return e.request.FormValue(name)
}

// FormParams returns the form parameters as `url.Values`.
func (e *echoContextWrapper) FormParams() (url.Values, error) {
	if err := e.request.ParseForm(); err != nil {
		return nil, err
	}
	return e.request.Form, nil
}

// FormFile returns the multipart form file for the provided name.
func (e *echoContextWrapper) FormFile(name string) (*multipart.FileHeader, error) {
	_, fh, err := e.request.FormFile(name)
	return fh, err
}

// MultipartForm returns the multipart form.
func (e *echoContextWrapper) MultipartForm() (*multipart.Form, error) {
	if err := e.request.ParseMultipartForm(32 << 20); err != nil {
		return nil, err
	}
	return e.request.MultipartForm, nil
}

// Cookie returns the named cookie provided in the request.
func (e *echoContextWrapper) Cookie(name string) (*http.Cookie, error) {
	return e.request.Cookie(name)
}

// SetCookie adds a `Set-Cookie` header in HTTP response.
func (e *echoContextWrapper) SetCookie(cookie *http.Cookie) {
	http.SetCookie(e.response.Writer, cookie)
}

// Cookies returns the HTTP cookies sent with the request.
func (e *echoContextWrapper) Cookies() []*http.Cookie {
	return e.request.Cookies()
}

// Get retrieves data from the context.
func (e *echoContextWrapper) Get(key string) interface{} {
	if val, ok := e.values[key]; ok {
		return val
	}
	return e.Context.Get(key)
}

// Set saves data in the context.
func (e *echoContextWrapper) Set(key string, val interface{}) {
	e.values[key] = val
	e.Context.Set(key, val)
}

// Bind binds the request body into provided type `i`.
func (e *echoContextWrapper) Bind(i interface{}) error {
	return e.Context.Bind(i)
}

// Validate validates provided `i`.
func (e *echoContextWrapper) Validate(i interface{}) error {
	// Implement validation if needed
	return nil
}

// Render renders a template with data and sends a text/html response.
func (e *echoContextWrapper) Render(code int, name string, data interface{}) error {
	// Not implemented - would require template engine
	return echo.ErrRendererNotRegistered
}

// HTML sends an HTTP response with status code.
func (e *echoContextWrapper) HTML(code int, html string) error {
	e.response.Header().Set("Content-Type", "text/html; charset=utf-8")
	e.response.WriteHeader(code)
	_, err := e.response.Write([]byte(html))
	return err
}

// HTMLBlob sends an HTTP blob response with status code.
func (e *echoContextWrapper) HTMLBlob(code int, b []byte) error {
	e.response.Header().Set("Content-Type", "text/html; charset=utf-8")
	e.response.WriteHeader(code)
	_, err := e.response.Write(b)
	return err
}

// String sends a string response with status code.
func (e *echoContextWrapper) String(code int, s string) error {
	return e.Context.String(code, s)
}

// JSON sends a JSON response with status code.
func (e *echoContextWrapper) JSON(code int, i interface{}) error {
	return e.Context.JSON(code, i)
}

// JSONPretty sends a pretty-print JSON response.
func (e *echoContextWrapper) JSONPretty(code int, i interface{}, indent string) error {
	// Delegate to regular JSON for now
	return e.JSON(code, i)
}

// JSONBlob sends a JSON blob response with status code.
func (e *echoContextWrapper) JSONBlob(code int, b []byte) error {
	e.response.Header().Set("Content-Type", "application/json")
	e.response.WriteHeader(code)
	_, err := e.response.Write(b)
	return err
}

// JSONP sends a JSONP response with status code.
func (e *echoContextWrapper) JSONP(code int, callback string, i interface{}) error {
	// Not commonly used, minimal implementation
	return e.JSON(code, i)
}

// JSONPBlob sends a JSONP blob response with status code.
func (e *echoContextWrapper) JSONPBlob(code int, callback string, b []byte) error {
	// Not commonly used, minimal implementation
	return e.JSONBlob(code, b)
}

// XML sends an XML response with status code.
func (e *echoContextWrapper) XML(code int, i interface{}) error {
	// Minimal implementation
	e.response.Header().Set("Content-Type", "application/xml")
	e.response.WriteHeader(code)
	return nil
}

// XMLPretty sends a pretty-print XML response.
func (e *echoContextWrapper) XMLPretty(code int, i interface{}, indent string) error {
	return e.XML(code, i)
}

// XMLBlob sends an XML blob response with status code.
func (e *echoContextWrapper) XMLBlob(code int, b []byte) error {
	e.response.Header().Set("Content-Type", "application/xml")
	e.response.WriteHeader(code)
	_, err := e.response.Write(b)
	return err
}

// Blob sends a blob response with status code and content type.
func (e *echoContextWrapper) Blob(code int, contentType string, b []byte) error {
	e.response.Header().Set("Content-Type", contentType)
	e.response.WriteHeader(code)
	_, err := e.response.Write(b)
	return err
}

// Stream sends a streaming response with status code and content type.
func (e *echoContextWrapper) Stream(code int, contentType string, r io.Reader) error {
	e.response.Header().Set("Content-Type", contentType)
	e.response.WriteHeader(code)
	_, err := io.Copy(e.response.Writer, r)
	return err
}

// File sends a response with the content of the file.
func (e *echoContextWrapper) File(file string) error {
	http.ServeFile(e.response.Writer, e.request, file)
	return nil
}

// Attachment sends a response as attachment.
func (e *echoContextWrapper) Attachment(file, name string) error {
	e.response.Header().Set("Content-Disposition", "attachment; filename=\""+name+"\"")
	return e.File(file)
}

// Inline sends a response as inline.
func (e *echoContextWrapper) Inline(file, name string) error {
	e.response.Header().Set("Content-Disposition", "inline; filename=\""+name+"\"")
	return e.File(file)
}

// NoContent sends a response with no body and a status code.
func (e *echoContextWrapper) NoContent(code int) error {
	e.response.WriteHeader(code)
	return nil
}

// Redirect redirects the request to a provided URL with status code.
func (e *echoContextWrapper) Redirect(code int, url string) error {
	http.Redirect(e.response.Writer, e.request, url, code)
	return nil
}

// Error invokes the registered HTTP error handler.
func (e *echoContextWrapper) Error(err error) {
	// Let the framework handle the error
	e.Context.Set("error", err)
}

// Handler returns the matched handler by router.
func (e *echoContextWrapper) Handler() echo.HandlerFunc {
	// Return a dummy handler
	return func(c echo.Context) error {
		return nil
	}
}

// SetHandler sets the matched handler by router.
func (e *echoContextWrapper) SetHandler(h echo.HandlerFunc) {
	// No-op for now
}

// Logger returns the logger instance.
func (e *echoContextWrapper) Logger() echo.Logger {
	// Return a minimal logger
	return echo.New().Logger
}

// Set returns the logger instance.
func (e *echoContextWrapper) SetLogger(l echo.Logger) {
	// No-op for now
}

// Echo returns the `Echo` instance.
func (e *echoContextWrapper) Echo() *echo.Echo {
	// Return a dummy echo instance
	return echo.New()
}

// Reset resets the context after request completes.
func (e *echoContextWrapper) Reset(r *http.Request, w http.ResponseWriter) {
	e.request = r
	e.response = echo.NewResponse(w, echo.New())
	e.values = make(map[string]interface{})
	e.paramNames = []string{}
	e.paramValues = []string{}
	e.path = ""
}