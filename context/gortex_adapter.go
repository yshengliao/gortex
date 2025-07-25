package context

import (
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	
	"github.com/labstack/echo/v4"
)

// GortexContextAdapter adapts Gortex Context to Echo Context
// This allows Gortex Context to be used where Echo Context is expected
type GortexContextAdapter struct {
	ctx Context
}

// Ensure GortexContextAdapter implements echo.Context
var _ echo.Context = (*GortexContextAdapter)(nil)

// Request returns the HTTP request
func (g *GortexContextAdapter) Request() *http.Request {
	return g.ctx.Request()
}

// SetRequest sets the HTTP request
func (g *GortexContextAdapter) SetRequest(r *http.Request) {
	g.ctx.SetRequest(r)
}

// Response returns the response writer
func (g *GortexContextAdapter) Response() *echo.Response {
	// We need to convert our ResponseWriter to echo.Response
	// This is a simplified implementation
	resp := g.ctx.Response()
	return &echo.Response{
		Writer: resp,
		Status: resp.Status(),
		Size:   resp.Size(),
	}
}

// IsTLS returns true if the request is using TLS
func (g *GortexContextAdapter) IsTLS() bool {
	return g.ctx.IsTLS()
}

// IsWebSocket returns true if the request is a WebSocket upgrade
func (g *GortexContextAdapter) IsWebSocket() bool {
	return g.ctx.IsWebSocket()
}

// Scheme returns the HTTP protocol scheme
func (g *GortexContextAdapter) Scheme() string {
	return g.ctx.Scheme()
}

// RealIP returns the client's real IP
func (g *GortexContextAdapter) RealIP() string {
	return g.ctx.RealIP()
}

// Path returns the registered path
func (g *GortexContextAdapter) Path() string {
	return g.ctx.Path()
}

// SetPath sets the registered path
func (g *GortexContextAdapter) SetPath(p string) {
	g.ctx.SetPath(p)
}

// Param returns path parameter by name
func (g *GortexContextAdapter) Param(name string) string {
	return g.ctx.Param(name)
}

// ParamNames returns all path parameter names
func (g *GortexContextAdapter) ParamNames() []string {
	return g.ctx.ParamNames()
}

// SetParamNames sets path parameter names
func (g *GortexContextAdapter) SetParamNames(names ...string) {
	g.ctx.SetParamNames(names...)
}

// ParamValues returns all path parameter values
func (g *GortexContextAdapter) ParamValues() []string {
	return g.ctx.ParamValues()
}

// SetParamValues sets path parameter values
func (g *GortexContextAdapter) SetParamValues(values ...string) {
	g.ctx.SetParamValues(values...)
}

// QueryParam returns query parameter by name
func (g *GortexContextAdapter) QueryParam(name string) string {
	return g.ctx.QueryParam(name)
}

// QueryParams returns all query parameters
func (g *GortexContextAdapter) QueryParams() url.Values {
	return g.ctx.QueryParams()
}

// QueryString returns the URL query string
func (g *GortexContextAdapter) QueryString() string {
	return g.ctx.QueryString()
}

// FormValue returns form value by name
func (g *GortexContextAdapter) FormValue(name string) string {
	return g.ctx.FormValue(name)
}

// FormParams returns all form parameters
func (g *GortexContextAdapter) FormParams() (url.Values, error) {
	return g.ctx.FormParams()
}

// FormFile returns multipart form file by name
func (g *GortexContextAdapter) FormFile(name string) (*multipart.FileHeader, error) {
	return g.ctx.FormFile(name)
}

// MultipartForm returns multipart form
func (g *GortexContextAdapter) MultipartForm() (*multipart.Form, error) {
	return g.ctx.MultipartForm()
}

// Cookie returns cookie by name
func (g *GortexContextAdapter) Cookie(name string) (*http.Cookie, error) {
	return g.ctx.Cookie(name)
}

// SetCookie sets a cookie
func (g *GortexContextAdapter) SetCookie(cookie *http.Cookie) {
	g.ctx.SetCookie(cookie)
}

// Cookies returns all cookies
func (g *GortexContextAdapter) Cookies() []*http.Cookie {
	return g.ctx.Cookies()
}

// Get retrieves data from context
func (g *GortexContextAdapter) Get(key string) interface{} {
	return g.ctx.Get(key)
}

// Set saves data in context
func (g *GortexContextAdapter) Set(key string, val interface{}) {
	g.ctx.Set(key, val)
}

// Bind binds request body to interface
func (g *GortexContextAdapter) Bind(i interface{}) error {
	return g.ctx.Bind(i)
}

// Validate validates a struct
func (g *GortexContextAdapter) Validate(i interface{}) error {
	return g.ctx.Validate(i)
}

// JSON sends a JSON response
func (g *GortexContextAdapter) JSON(code int, i interface{}) error {
	return g.ctx.JSON(code, i)
}

// JSONBlob sends a JSON blob response
func (g *GortexContextAdapter) JSONBlob(code int, b []byte) error {
	return g.ctx.JSONBlob(code, b)
}

// JSONPretty sends a pretty-printed JSON response
func (g *GortexContextAdapter) JSONPretty(code int, i interface{}, indent string) error {
	return g.ctx.JSONPretty(code, i, indent)
}

// JSONP sends a JSONP response
func (g *GortexContextAdapter) JSONP(code int, callback string, i interface{}) error {
	return g.ctx.JSONP(code, callback, i)
}

// JSONPBlob sends a JSONP blob response
func (g *GortexContextAdapter) JSONPBlob(code int, callback string, b []byte) error {
	return g.ctx.JSONPBlob(code, callback, b)
}

// XML sends an XML response
func (g *GortexContextAdapter) XML(code int, i interface{}) error {
	return g.ctx.XML(code, i)
}

// XMLBlob sends an XML blob response
func (g *GortexContextAdapter) XMLBlob(code int, b []byte) error {
	return g.ctx.XMLBlob(code, b)
}

// XMLPretty sends a pretty-printed XML response
func (g *GortexContextAdapter) XMLPretty(code int, i interface{}, indent string) error {
	return g.ctx.XMLPretty(code, i, indent)
}

// Blob sends a blob response
func (g *GortexContextAdapter) Blob(code int, contentType string, b []byte) error {
	return g.ctx.Blob(code, contentType, b)
}

// Stream sends a streaming response
func (g *GortexContextAdapter) Stream(code int, contentType string, r io.Reader) error {
	return g.ctx.Stream(code, contentType, r)
}

// File sends a file response
func (g *GortexContextAdapter) File(file string) error {
	return g.ctx.File(file)
}

// Inline sends a file as inline
func (g *GortexContextAdapter) Inline(file, name string) error {
	return g.ctx.Inline(file, name)
}

// Attachment sends a file as attachment
func (g *GortexContextAdapter) Attachment(file, name string) error {
	return g.ctx.Attachment(file, name)
}

// NoContent sends a response with no body
func (g *GortexContextAdapter) NoContent(code int) error {
	return g.ctx.NoContent(code)
}

// String sends a string response
func (g *GortexContextAdapter) String(code int, s string) error {
	return g.ctx.String(code, s)
}

// HTML sends an HTML response
func (g *GortexContextAdapter) HTML(code int, html string) error {
	return g.ctx.HTML(code, html)
}

// HTMLBlob sends an HTML blob response
func (g *GortexContextAdapter) HTMLBlob(code int, b []byte) error {
	return g.ctx.HTMLBlob(code, b)
}

// Redirect redirects the request
func (g *GortexContextAdapter) Redirect(code int, url string) error {
	return g.ctx.Redirect(code, url)
}

// Error invokes the error handler
func (g *GortexContextAdapter) Error(err error) {
	g.ctx.Error(err)
}

// Handler returns the handler
func (g *GortexContextAdapter) Handler() echo.HandlerFunc {
	// Convert Gortex handler to Echo handler
	h := g.ctx.Handler()
	if h == nil {
		return nil
	}
	return AdaptGortexToEcho(h)
}

// SetHandler sets the handler
func (g *GortexContextAdapter) SetHandler(h echo.HandlerFunc) {
	// Convert Echo handler to Gortex handler
	g.ctx.SetHandler(AdaptEchoToGortex(h))
}

// Logger returns the logger
func (g *GortexContextAdapter) Logger() echo.Logger {
	// We need to adapt our Logger to Echo's Logger
	// This is a simplified implementation
	return echo.New().Logger
}

// SetLogger sets the logger
func (g *GortexContextAdapter) SetLogger(l echo.Logger) {
	// This is a no-op for now
}

// Echo returns the Echo instance
func (g *GortexContextAdapter) Echo() *echo.Echo {
	// Return the echo instance if available
	if echoInstance := g.ctx.Echo(); echoInstance != nil {
		if e, ok := echoInstance.(*echo.Echo); ok {
			return e
		}
	}
	// Otherwise return a default instance
	return echo.New()
}

// Reset resets the context
func (g *GortexContextAdapter) Reset(r *http.Request, w http.ResponseWriter) {
	g.ctx.Reset(r, w)
}

// Render renders a template with data
func (g *GortexContextAdapter) Render(code int, name string, data interface{}) error {
	// This is a simplified implementation
	// In a real implementation, you would use a template engine
	return g.ctx.HTML(code, "Template rendering not implemented")
}

// SetResponse sets the response
func (g *GortexContextAdapter) SetResponse(r *echo.Response) {
	// This is a no-op for now as our context doesn't expose direct response setting
	// The response is managed internally
}

// NewGortexContextAdapter creates a new adapter
func NewGortexContextAdapter(ctx Context) *GortexContextAdapter {
	return &GortexContextAdapter{ctx: ctx}
}