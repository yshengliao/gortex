package context

import (
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	
	"github.com/labstack/echo/v4"
)

// EchoAdapter adapts Gortex Context to Echo Context
type EchoAdapter struct {
	Context
	echoCtx echo.Context
}

// NewEchoAdapter creates a new adapter
func NewEchoAdapter(ctx Context, echoCtx echo.Context) *EchoAdapter {
	return &EchoAdapter{
		Context: ctx,
		echoCtx: echoCtx,
	}
}

// Echo returns the underlying echo.Context
func (a *EchoAdapter) Echo() interface{} {
	return a.echoCtx
}

// AdaptGortexToEcho adapts a Gortex handler to Echo handler
func AdaptGortexToEcho(h HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		// Create Gortex context from Echo context
		ctx := &EchoContextAdapter{echoCtx: c}
		return h(ctx)
	}
}

// AdaptEchoToGortex adapts an Echo handler to Gortex handler
func AdaptEchoToGortex(h echo.HandlerFunc) HandlerFunc {
	return func(c Context) error {
		// If the context already has an echo context, use it
		if adapter, ok := c.(*EchoAdapter); ok {
			return h(adapter.echoCtx)
		}
		
		// Otherwise, create an Echo context adapter
		echoCtx := NewGortexContextAdapter(c)
		return h(echoCtx)
	}
}

// EchoContextAdapter adapts Echo Context to Gortex Context
type EchoContextAdapter struct {
	echoCtx echo.Context
}

// Implement all Context methods by delegating to Echo Context
func (e *EchoContextAdapter) Request() *http.Request {
	return e.echoCtx.Request()
}

func (e *EchoContextAdapter) SetRequest(r *http.Request) {
	e.echoCtx.SetRequest(r)
}

func (e *EchoContextAdapter) Response() ResponseWriter {
	// Wrap Echo's response in our ResponseWriter
	return NewResponseWriter(e.echoCtx.Response())
}

func (e *EchoContextAdapter) IsTLS() bool {
	return e.echoCtx.IsTLS()
}

func (e *EchoContextAdapter) IsWebSocket() bool {
	return e.echoCtx.IsWebSocket()
}

func (e *EchoContextAdapter) Scheme() string {
	return e.echoCtx.Scheme()
}

func (e *EchoContextAdapter) RealIP() string {
	return e.echoCtx.RealIP()
}

func (e *EchoContextAdapter) Path() string {
	return e.echoCtx.Path()
}

func (e *EchoContextAdapter) SetPath(p string) {
	e.echoCtx.SetPath(p)
}

func (e *EchoContextAdapter) Param(name string) string {
	return e.echoCtx.Param(name)
}

func (e *EchoContextAdapter) ParamNames() []string {
	return e.echoCtx.ParamNames()
}

func (e *EchoContextAdapter) SetParamNames(names ...string) {
	e.echoCtx.SetParamNames(names...)
}

func (e *EchoContextAdapter) ParamValues() []string {
	return e.echoCtx.ParamValues()
}

func (e *EchoContextAdapter) SetParamValues(values ...string) {
	e.echoCtx.SetParamValues(values...)
}

func (e *EchoContextAdapter) QueryParam(name string) string {
	return e.echoCtx.QueryParam(name)
}

func (e *EchoContextAdapter) QueryParams() url.Values {
	return e.echoCtx.QueryParams()
}

func (e *EchoContextAdapter) QueryString() string {
	return e.echoCtx.QueryString()
}

func (e *EchoContextAdapter) FormValue(name string) string {
	return e.echoCtx.FormValue(name)
}

func (e *EchoContextAdapter) FormParams() (url.Values, error) {
	return e.echoCtx.FormParams()
}

func (e *EchoContextAdapter) FormFile(name string) (*multipart.FileHeader, error) {
	return e.echoCtx.FormFile(name)
}

func (e *EchoContextAdapter) MultipartForm() (*multipart.Form, error) {
	return e.echoCtx.MultipartForm()
}

func (e *EchoContextAdapter) Cookie(name string) (*http.Cookie, error) {
	return e.echoCtx.Cookie(name)
}

func (e *EchoContextAdapter) SetCookie(cookie *http.Cookie) {
	e.echoCtx.SetCookie(cookie)
}

func (e *EchoContextAdapter) Cookies() []*http.Cookie {
	return e.echoCtx.Cookies()
}

func (e *EchoContextAdapter) Get(key string) interface{} {
	return e.echoCtx.Get(key)
}

func (e *EchoContextAdapter) Set(key string, val interface{}) {
	e.echoCtx.Set(key, val)
}

func (e *EchoContextAdapter) Bind(i interface{}) error {
	return e.echoCtx.Bind(i)
}

func (e *EchoContextAdapter) Validate(i interface{}) error {
	return e.echoCtx.Validate(i)
}

func (e *EchoContextAdapter) JSON(code int, i interface{}) error {
	return e.echoCtx.JSON(code, i)
}

func (e *EchoContextAdapter) JSONBlob(code int, b []byte) error {
	return e.echoCtx.JSONBlob(code, b)
}

func (e *EchoContextAdapter) JSONPretty(code int, i interface{}, indent string) error {
	return e.echoCtx.JSONPretty(code, i, indent)
}

func (e *EchoContextAdapter) JSONByte(code int, b []byte) error {
	return e.echoCtx.JSONBlob(code, b)
}

func (e *EchoContextAdapter) JSONP(code int, callback string, i interface{}) error {
	return e.echoCtx.JSONP(code, callback, i)
}

func (e *EchoContextAdapter) JSONPBlob(code int, callback string, b []byte) error {
	return e.echoCtx.JSONPBlob(code, callback, b)
}

func (e *EchoContextAdapter) XML(code int, i interface{}) error {
	return e.echoCtx.XML(code, i)
}

func (e *EchoContextAdapter) XMLBlob(code int, b []byte) error {
	return e.echoCtx.XMLBlob(code, b)
}

func (e *EchoContextAdapter) XMLPretty(code int, i interface{}, indent string) error {
	return e.echoCtx.XMLPretty(code, i, indent)
}

func (e *EchoContextAdapter) Blob(code int, contentType string, b []byte) error {
	return e.echoCtx.Blob(code, contentType, b)
}

func (e *EchoContextAdapter) Stream(code int, contentType string, r io.Reader) error {
	return e.echoCtx.Stream(code, contentType, r)
}

func (e *EchoContextAdapter) File(file string) error {
	return e.echoCtx.File(file)
}

func (e *EchoContextAdapter) Inline(file, name string) error {
	return e.echoCtx.Inline(file, name)
}

func (e *EchoContextAdapter) Attachment(file, name string) error {
	return e.echoCtx.Attachment(file, name)
}

func (e *EchoContextAdapter) NoContent(code int) error {
	return e.echoCtx.NoContent(code)
}

func (e *EchoContextAdapter) String(code int, s string) error {
	return e.echoCtx.String(code, s)
}

func (e *EchoContextAdapter) HTML(code int, html string) error {
	return e.echoCtx.HTML(code, html)
}

func (e *EchoContextAdapter) HTMLBlob(code int, b []byte) error {
	return e.echoCtx.HTMLBlob(code, b)
}

func (e *EchoContextAdapter) Redirect(code int, url string) error {
	return e.echoCtx.Redirect(code, url)
}

func (e *EchoContextAdapter) Error(err error) {
	e.echoCtx.Error(err)
}

func (e *EchoContextAdapter) Handler() HandlerFunc {
	// Convert Echo handler to Gortex handler
	echoHandler := e.echoCtx.Handler()
	if echoHandler == nil {
		return nil
	}
	return AdaptEchoToGortex(echoHandler)
}

func (e *EchoContextAdapter) SetHandler(h HandlerFunc) {
	// Convert Gortex handler to Echo handler
	e.echoCtx.SetHandler(AdaptGortexToEcho(h))
}

func (e *EchoContextAdapter) Logger() Logger {
	// Adapt Echo logger to our Logger interface
	return &echoLoggerAdapter{logger: e.echoCtx.Logger()}
}

func (e *EchoContextAdapter) SetLogger(l Logger) {
	// This is a no-op for now as Echo doesn't support setting logger on context
}

func (e *EchoContextAdapter) Echo() interface{} {
	return e.echoCtx
}

func (e *EchoContextAdapter) Reset(r *http.Request, w http.ResponseWriter) {
	e.echoCtx.Reset(r, w)
}

func (e *EchoContextAdapter) StdContext() context.Context {
	return e.echoCtx.Request().Context()
}

func (e *EchoContextAdapter) SetStdContext(ctx context.Context) {
	e.echoCtx.SetRequest(e.echoCtx.Request().WithContext(ctx))
}

// echoLoggerAdapter adapts Echo's logger to our Logger interface
type echoLoggerAdapter struct {
	logger echo.Logger
}

func (l *echoLoggerAdapter) Debug(msg string, keysAndValues ...interface{}) {
	l.logger.Debug(msg)
}

func (l *echoLoggerAdapter) Info(msg string, keysAndValues ...interface{}) {
	l.logger.Info(msg)
}

func (l *echoLoggerAdapter) Warn(msg string, keysAndValues ...interface{}) {
	l.logger.Warn(msg)
}

func (l *echoLoggerAdapter) Error(msg string, keysAndValues ...interface{}) {
	l.logger.Error(msg)
}