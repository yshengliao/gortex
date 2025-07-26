// Package context provides the core HTTP context interface for Gortex framework
package context

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
	
	// Path returns the request URL path
	Path() string
	
	// SetPath sets the request URL path
	SetPath(p string)
	
	// Param returns path parameter by name
	Param(name string) string
	
	// ParamNames returns all path parameter names
	ParamNames() []string
	
	// SetParamNames sets path parameter names
	SetParamNames(names ...string)
	
	// ParamValues returns all path parameter values
	ParamValues() []string
	
	// SetParamValues sets path parameter values
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
	
	// FormFile returns multipart form file by name
	FormFile(name string) (*multipart.FileHeader, error)
	
	// MultipartForm returns multipart form
	MultipartForm() (*multipart.Form, error)
	
	// Cookie returns cookie by name
	Cookie(name string) (*http.Cookie, error)
	
	// SetCookie sets a cookie
	SetCookie(cookie *http.Cookie)
	
	// Cookies returns all cookies
	Cookies() []*http.Cookie
	
	// Get retrieves data from context
	Get(key string) interface{}
	
	// Set saves data in context
	Set(key string, val interface{})
	
	// Bind binds request body to interface
	Bind(i interface{}) error
	
	// Validate validates the bound struct
	Validate(i interface{}) error
	
	// JSON sends a JSON response with status code
	JSON(code int, i interface{}) error
	
	// JSONBlob sends a JSON blob response with status code
	JSONBlob(code int, b []byte) error
	
	// JSONPretty sends a pretty-printed JSON response
	JSONPretty(code int, i interface{}, indent string) error
	
	// JSONByte sends a JSON byte response (for pre-marshaled data)
	JSONByte(code int, b []byte) error
	
	// JSONP sends a JSONP response
	JSONP(code int, callback string, i interface{}) error
	
	// JSONPBlob sends a JSONP blob response
	JSONPBlob(code int, callback string, b []byte) error
	
	// XML sends an XML response with status code
	XML(code int, i interface{}) error
	
	// XMLBlob sends an XML blob response
	XMLBlob(code int, b []byte) error
	
	// XMLPretty sends a pretty-printed XML response
	XMLPretty(code int, i interface{}, indent string) error
	
	// Blob sends a blob response with content type
	Blob(code int, contentType string, b []byte) error
	
	// Stream sends a streaming response with content type
	Stream(code int, contentType string, r io.Reader) error
	
	// File sends a file as response
	File(file string) error
	
	// Inline sends a file as inline
	Inline(file, name string) error
	
	// Attachment sends a file as attachment
	Attachment(file, name string) error
	
	// NoContent sends a response with no body
	NoContent(code int) error
	
	// String sends a string response
	String(code int, s string) error
	
	// HTML sends an HTML response
	HTML(code int, html string) error
	
	// HTMLBlob sends an HTML blob response
	HTMLBlob(code int, b []byte) error
	
	// Redirect redirects the request
	Redirect(code int, url string) error
	
	// Error invokes the registered error handler
	Error(err error)
	
	// Handler returns the handler
	Handler() HandlerFunc
	
	// SetHandler sets the handler
	SetHandler(h HandlerFunc)
	
	// Logger returns the logger
	Logger() Logger
	
	// SetLogger sets the logger
	SetLogger(l Logger)
	
	// Echo returns the Echo instance (for compatibility)
	// This will be removed in future versions
	Echo() interface{}
	
	// Reset resets the context
	Reset(r *http.Request, w http.ResponseWriter)
	
	// StdContext returns the standard context.Context
	StdContext() context.Context
	
	// SetStdContext sets the standard context.Context
	SetStdContext(ctx context.Context)
	
	// Helper methods for better developer experience
	
	// ParamInt returns path parameter as int with default value
	ParamInt(name string, defaultValue int) int
	
	// QueryInt returns query parameter as int with default value
	QueryInt(name string, defaultValue int) int
	
	// QueryBool returns query parameter as bool with default value
	QueryBool(name string, defaultValue bool) bool
	
	// OK sends a successful response with data (200 OK)
	OK(data interface{}) error
	
	// Created sends a created response with data (201 Created)
	Created(data interface{}) error
	
	// NoContent204 sends a no content response (204 No Content)
	NoContent204() error
	
	// BadRequest sends a bad request response with message (400 Bad Request)
	BadRequest(message string) error
}

// HandlerFunc defines a function to serve HTTP requests
type HandlerFunc func(Context) error

// MiddlewareFunc defines a function to process middleware
type MiddlewareFunc func(HandlerFunc) HandlerFunc

// ResponseWriter extends http.ResponseWriter
type ResponseWriter interface {
	http.ResponseWriter
	// Status returns the status code
	Status() int
	// Size returns the response size
	Size() int64
	// Written returns true if response was written
	Written() bool
	// Before allows adding functions to be called before writing
	Before(func())
	// After allows adding functions to be called after writing
	After(func())
}

// Logger interface for logging
type Logger interface {
	Debug(msg string, keysAndValues ...interface{})
	Info(msg string, keysAndValues ...interface{})
	Warn(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
}

// Map is a generic map type alias
type Map map[string]interface{}

// HTTPError represents an HTTP error
type HTTPError struct {
	Code     int         `json:"-"`
	Message  interface{} `json:"message"`
	Internal error       `json:"-"`
}

// NewHTTPError creates a new HTTPError
func NewHTTPError(code int, message ...interface{}) *HTTPError {
	he := &HTTPError{Code: code}
	if len(message) > 0 {
		he.Message = message[0]
	} else {
		he.Message = http.StatusText(code)
	}
	return he
}

// Error implements the error interface
func (he *HTTPError) Error() string {
	if he.Internal != nil {
		return he.Internal.Error()
	}
	return he.Message.(string)
}

// Unwrap returns the internal error
func (he *HTTPError) Unwrap() error {
	return he.Internal
}

// WithInternal sets the internal error
func (he *HTTPError) WithInternal(err error) *HTTPError {
	he.Internal = err
	return he
}

// Common errors
var (
	ErrUnsupportedMediaType        = NewHTTPError(http.StatusUnsupportedMediaType)
	ErrNotFound                    = NewHTTPError(http.StatusNotFound)
	ErrUnauthorized                = NewHTTPError(http.StatusUnauthorized)
	ErrForbidden                   = NewHTTPError(http.StatusForbidden)
	ErrMethodNotAllowed            = NewHTTPError(http.StatusMethodNotAllowed)
	ErrStatusRequestEntityTooLarge = NewHTTPError(http.StatusRequestEntityTooLarge)
	ErrBadRequest                  = NewHTTPError(http.StatusBadRequest)
	ErrBadGateway                  = NewHTTPError(http.StatusBadGateway)
	ErrInternalServerError         = NewHTTPError(http.StatusInternalServerError)
	ErrRequestTimeout              = NewHTTPError(http.StatusRequestTimeout)
	ErrServiceUnavailable          = NewHTTPError(http.StatusServiceUnavailable)
	ErrValidatorNotRegistered      = NewHTTPError(http.StatusInternalServerError, "validator not registered")
	ErrInvalidRedirectCode         = NewHTTPError(http.StatusInternalServerError, "invalid redirect status code")
)

// MIME types
const (
	MIMEApplicationJSON                  = "application/json"
	MIMEApplicationJSONCharsetUTF8       = MIMEApplicationJSON + "; " + charsetUTF8
	MIMEApplicationJavaScript            = "application/javascript"
	MIMEApplicationJavaScriptCharsetUTF8 = MIMEApplicationJavaScript + "; " + charsetUTF8
	MIMEApplicationXML                   = "application/xml"
	MIMEApplicationXMLCharsetUTF8        = MIMEApplicationXML + "; " + charsetUTF8
	MIMETextXML                          = "text/xml"
	MIMETextXMLCharsetUTF8               = MIMETextXML + "; " + charsetUTF8
	MIMEApplicationForm                  = "application/x-www-form-urlencoded"
	MIMEApplicationProtobuf              = "application/protobuf"
	MIMEApplicationMsgpack               = "application/msgpack"
	MIMETextHTML                         = "text/html"
	MIMETextHTMLCharsetUTF8              = MIMETextHTML + "; " + charsetUTF8
	MIMETextPlain                        = "text/plain"
	MIMETextPlainCharsetUTF8             = MIMETextPlain + "; " + charsetUTF8
	MIMEMultipartForm                    = "multipart/form-data"
	MIMEOctetStream                      = "application/octet-stream"
)

// Charset
const charsetUTF8 = "charset=UTF-8"

// Headers
const (
	HeaderAccept              = "Accept"
	HeaderAcceptEncoding      = "Accept-Encoding"
	HeaderAllow               = "Allow"
	HeaderAuthorization       = "Authorization"
	HeaderContentDisposition  = "Content-Disposition"
	HeaderContentEncoding     = "Content-Encoding"
	HeaderContentLength       = "Content-Length"
	HeaderContentType         = "Content-Type"
	HeaderCookie              = "Cookie"
	HeaderSetCookie           = "Set-Cookie"
	HeaderIfModifiedSince     = "If-Modified-Since"
	HeaderLastModified        = "Last-Modified"
	HeaderLocation            = "Location"
	HeaderUpgrade             = "Upgrade"
	HeaderVary                = "Vary"
	HeaderWWWAuthenticate     = "WWW-Authenticate"
	HeaderXForwardedFor       = "X-Forwarded-For"
	HeaderXForwardedProto     = "X-Forwarded-Proto"
	HeaderXForwardedProtocol  = "X-Forwarded-Protocol"
	HeaderXForwardedSsl       = "X-Forwarded-Ssl"
	HeaderXUrlScheme          = "X-Url-Scheme"
	HeaderXHTTPMethodOverride = "X-HTTP-Method-Override"
	HeaderXRealIP             = "X-Real-Ip"
	HeaderXRequestID          = "X-Request-Id"
	HeaderXRequestedWith      = "X-Requested-With"
	HeaderServer              = "Server"
	HeaderOrigin              = "Origin"
	HeaderCacheControl        = "Cache-Control"
	HeaderConnection          = "Connection"
	
	// Access control
	HeaderAccessControlRequestMethod    = "Access-Control-Request-Method"
	HeaderAccessControlRequestHeaders   = "Access-Control-Request-Headers"
	HeaderAccessControlAllowOrigin      = "Access-Control-Allow-Origin"
	HeaderAccessControlAllowMethods     = "Access-Control-Allow-Methods"
	HeaderAccessControlAllowHeaders     = "Access-Control-Allow-Headers"
	HeaderAccessControlAllowCredentials = "Access-Control-Allow-Credentials"
	HeaderAccessControlExposeHeaders    = "Access-Control-Expose-Headers"
	HeaderAccessControlMaxAge           = "Access-Control-Max-Age"
	
	// Security
	HeaderStrictTransportSecurity         = "Strict-Transport-Security"
	HeaderXContentTypeOptions             = "X-Content-Type-Options"
	HeaderXXSSProtection                  = "X-XSS-Protection"
	HeaderXFrameOptions                   = "X-Frame-Options"
	HeaderContentSecurityPolicy           = "Content-Security-Policy"
	HeaderContentSecurityPolicyReportOnly = "Content-Security-Policy-Report-Only"
	HeaderXCSRFToken                      = "X-CSRF-Token"
	HeaderReferrerPolicy                  = "Referrer-Policy"
)

// Methods
const (
	CONNECT = http.MethodConnect
	DELETE  = http.MethodDelete
	GET     = http.MethodGet
	HEAD    = http.MethodHead
	OPTIONS = http.MethodOptions
	PATCH   = http.MethodPatch
	POST    = http.MethodPost
	PUT     = http.MethodPut
	TRACE   = http.MethodTrace
)

// StatusText returns the text for the HTTP status code
func StatusText(code int) string {
	return http.StatusText(code)
}