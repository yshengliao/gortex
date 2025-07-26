# Gortex API Reference

## Core Interfaces

### Context Interface
```go
type Context interface {
    // Request
    Request() *http.Request
    SetRequest(r *http.Request)
    
    // Response
    Response() ResponseWriter
    
    // Path parameters
    Param(name string) string
    ParamNames() []string
    SetParamNames(names ...string)
    ParamValues() []string
    SetParamValues(values ...string)
    
    // Query parameters
    QueryParam(name string) string
    QueryParams() url.Values
    QueryString() string
    
    // Form values
    FormValue(name string) string
    FormParams() (url.Values, error)
    FormFile(name string) (*multipart.FileHeader, error)
    
    // Request headers
    Cookie(name string) (*http.Cookie, error)
    SetCookie(cookie *http.Cookie)
    Cookies() []*http.Cookie
    
    // Context storage
    Get(key string) interface{}
    Set(key string, val interface{})
    
    // Binding and validation
    Bind(i interface{}) error
    Validate(i interface{}) error
    
    // Response methods
    JSON(code int, i interface{}) error
    JSONPretty(code int, i interface{}, indent string) error
    JSONBlob(code int, b []byte) error
    JSONByte(code int, b []byte) error
    JSONP(code int, callback string, i interface{}) error
    JSONPBlob(code int, callback string, b []byte) error
    XML(code int, i interface{}) error
    XMLPretty(code int, i interface{}, indent string) error
    XMLBlob(code int, b []byte) error
    HTML(code int, html string) error
    HTMLBlob(code int, b []byte) error
    String(code int, s string) error
    Blob(code int, contentType string, b []byte) error
    Stream(code int, contentType string, r io.Reader) error
    File(file string) error
    Attachment(file, name string) error
    Inline(file, name string) error
    NoContent(code int) error
    Redirect(code int, url string) error
    Error(err error)
}
```

### Router Interface
```go
type GortexRouter interface {
    // HTTP Methods
    GET(path string, h HandlerFunc, m ...MiddlewareFunc)
    POST(path string, h HandlerFunc, m ...MiddlewareFunc)
    PUT(path string, h HandlerFunc, m ...MiddlewareFunc)
    DELETE(path string, h HandlerFunc, m ...MiddlewareFunc)
    PATCH(path string, h HandlerFunc, m ...MiddlewareFunc)
    HEAD(path string, h HandlerFunc, m ...MiddlewareFunc)
    OPTIONS(path string, h HandlerFunc, m ...MiddlewareFunc)
    
    // Routing
    Group(prefix string, m ...MiddlewareFunc) GortexRouter
    Use(m ...MiddlewareFunc)
    Routes() []Route
    ServeHTTP(w http.ResponseWriter, r *http.Request)
}
```

### Handler and Middleware
```go
// Handler function signature
type HandlerFunc func(Context) error

// Middleware function signature
type MiddlewareFunc func(HandlerFunc) HandlerFunc
```

## Struct Tag Routing

### Basic Usage
```go
type HandlersManager struct {
    Home    *HomeHandler    `url:"/"`
    Users   *UserHandler    `url:"/users/:id"`
    Static  *StaticHandler  `url:"/static/*"`
    API     *APIGroup       `url:"/api"`
}
```

### Supported Tags
- `url:"/path"` - Define the route path
- `middleware:"auth,cors"` - Apply middleware (comma-separated)
- `hijack:"ws"` - Protocol hijacking (e.g., WebSocket)

### Dynamic Parameters
- `:param` - Named parameter (e.g., `/users/:id`)
- `*` - Wildcard (e.g., `/static/*`)

### HTTP Method Mapping
- `GET()` → GET /path
- `POST()` → POST /path
- `PUT()` → PUT /path
- `DELETE()` → DELETE /path
- `PATCH()` → PATCH /path
- `HEAD()` → HEAD /path
- `OPTIONS()` → OPTIONS /path
- Custom methods → POST /path/method-name (e.g., `Profile()` → POST /users/:id/profile)

## Application Configuration

### Creating an Application
```go
app, err := app.NewApp(
    app.WithConfig(cfg),
    app.WithLogger(logger),
    app.WithHandlers(handlers),
)
```

### Configuration Options
```go
type Config struct {
    Server ServerConfig
    Logger LoggerConfig
    // ... other configurations
}
```

### Running the Application
```go
if err := app.Run(); err != nil {
    log.Fatal(err)
}
```

## Middleware

### Built-in Middleware
- RequestID - Adds unique request IDs
- RateLimit - Rate limiting per IP
- DevErrorPage - Development error pages
- Logger - Request/response logging
- Recover - Panic recovery

### Custom Middleware
```go
func MyMiddleware() middleware.MiddlewareFunc {
    return func(next middleware.HandlerFunc) middleware.HandlerFunc {
        return func(c context.Context) error {
            // Before handler
            err := next(c)
            // After handler
            return err
        }
    }
}
```

## Error Handling

### Business Errors
```go
// Register error codes
errors.Register(ErrUserNotFound, 404, "User not found")

// Use in handlers
func (h *UserHandler) GET(c context.Context) error {
    user, err := h.service.GetUser(c.Param("id"))
    if err != nil {
        return err // Framework handles response
    }
    return c.JSON(200, user)
}
```

### HTTP Errors
```go
return context.NewHTTPError(404, "Not found")
```

## WebSocket Support

### WebSocket Handler
```go
type WSHandler struct {
    hub *hub.Hub
}

func (h *WSHandler) HandleConnection(c context.Context) error {
    conn, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
    if err != nil {
        return err
    }
    // Handle WebSocket connection
    return nil
}
```

### Struct Tag for WebSocket
```go
type HandlersManager struct {
    WS *WSHandler `url:"/ws" hijack:"ws"`
}
```

## Development Features

When `Logger.Level = "debug"`:
- `GET /_routes` - List all registered routes
- `GET /_monitor` - System monitoring metrics
- `GET /_config` - Configuration (sensitive data masked)
- Request/response body logging

## Response Helpers

```go
// Success responses
response.Success(c, 200, data)
response.Created(c, data)

// Error responses
response.BadRequest(c, "Invalid input")
response.Unauthorized(c, "Login required")
response.Forbidden(c, "Access denied")
response.NotFound(c, "Resource not found")
response.InternalServerError(c, "Server error")
```