# Gortex - High-Performance Go Web Framework

[![Go Version](https://img.shields.io/badge/go-1.24+-blue.svg)](https://go.dev/)
![Status](https://img.shields.io/badge/status-v0.4.0--alpha-orange.svg)
[![License](https://img.shields.io/badge/license-MIT-brightgreen.svg)](LICENSE)

> **Zero boilerplate, pure Go web framework. Define routes with struct tags, not code.**

## Why Gortex?

```go
// Traditional: Manual route registration
r.GET("/", homeHandler)
r.GET("/users/:id", userHandler)
r.GET("/api/v1/users", apiV1UserHandler)
// ... dozens more routes

// Gortex: Automatic discovery from struct tags
type HandlersManager struct {
    Home  *HomeHandler  `url:"/"`
    Users *UserHandler  `url:"/users/:id"`
    API   *APIGroup     `url:"/api"`
}
```

## Quick Start

```bash
go get github.com/yshengliao/gortex
```

```go
package main

import (
    "github.com/yshengliao/gortex/core/app"
    "github.com/yshengliao/gortex/core/types"
)

// Define routes with struct tags
type HandlersManager struct {
    Home   *HomeHandler   `url:"/"`
    Users  *UserHandler   `url:"/users/:id"`
    Admin  *AdminGroup    `url:"/admin" middleware:"auth"`
    WS     *WSHandler     `url:"/ws" hijack:"ws"`
}

type HomeHandler struct{}
func (h *HomeHandler) GET(c types.Context) error {
    return c.JSON(200, map[string]string{"message": "Welcome to Gortex!"})
}

type UserHandler struct{}
func (h *UserHandler) GET(c types.Context) error {
    return c.JSON(200, map[string]string{"id": c.Param("id")})
}

type AdminGroup struct {
    Dashboard *DashboardHandler `url:"/dashboard"`
}

type DashboardHandler struct{}
func (h *DashboardHandler) GET(c types.Context) error {
    return c.JSON(200, map[string]string{"data": "admin only"})
}

type WSHandler struct{}
func (h *WSHandler) HandleConnection(c types.Context) error {
    // WebSocket upgrade logic
    return nil
}

func main() {
    app, _ := app.NewApp(
        app.WithHandlers(&HandlersManager{
            Home:  &HomeHandler{},
            Users: &UserHandler{},
            Admin: &AdminGroup{
                Dashboard: &DashboardHandler{},
            },
            WS: &WSHandler{},
        }),
    )
    app.Run() // :8080
}
```

## Core Concepts

### 1. Struct Tag Routing

```go
type HandlersManager struct {
    Users    *UserHandler    `url:"/users/:id"`        // Dynamic params
    Static   *FileHandler    `url:"/static/*"`         // Wildcards
    API      *APIGroup       `url:"/api"`              // Nested groups
    Auth     *AuthHandler    `url:"/auth"`             // Public route
    Profile  *ProfileHandler `url:"/profile" middleware:"jwt"` // Protected
    Chat     *ChatHandler    `url:"/chat" hijack:"ws"` // WebSocket
}
```

### 2. HTTP Method Mapping

```go
type UserHandler struct{}

func (h *UserHandler) GET(c context.Context) error    { /* GET /users/:id */ }
func (h *UserHandler) POST(c context.Context) error   { /* POST /users/:id */ }
func (h *UserHandler) DELETE(c context.Context) error { /* DELETE /users/:id */ }
func (h *UserHandler) Profile(c context.Context) error { /* POST /users/:id/profile */ }
```

### 3. Nested Route Groups

```go
type APIGroup struct {
    V1 *V1Handlers `url:"/v1"`
    V2 *V2Handlers `url:"/v2"`
}

// Results in:
// /api/v1/...
// /api/v2/...
```

## Key Features

### Performance
- **45% Faster Routing** - Optimized reflection caching
- **Zero Dependencies** - No Redis, Kafka, or external services
- **Memory Efficient** - Context pooling & smart parameter storage
- **38% Less Memory** - Reduced allocations with object pooling

### Developer Experience
- **No Route Registration** - Framework discovers routes automatically
- **Type-Safe** - Compile-time route validation
- **Hot Reload** - Instant feedback in development
- **Built-in Debugging** - `/_routes`, `/_monitor` in dev mode

### Production Ready
- **JWT Auth** - Built-in authentication middleware
- **WebSocket** - First-class real-time support  
- **Metrics** - Prometheus-compatible metrics
- **Graceful Shutdown** - Proper connection cleanup
- **API Documentation** - Automatic OpenAPI/Swagger generation from struct tags

## Middleware

```go
// Apply middleware via struct tags
type HandlersManager struct {
    Public  *PublicHandler  `url:"/public"`
    Private *PrivateHandler `url:"/private" middleware:"auth"`
    Admin   *AdminHandler   `url:"/admin" middleware:"auth,rbac"`
}

// Or globally
app.NewApp(
    app.WithHandlers(handlers),
    app.WithMiddleware(
        middleware.Logger(),
        middleware.Recover(),
        middleware.CORS(),
    ),
)
```

## Advanced Features

### WebSocket Support
```go
type WSHandler struct {
    hub *hub.Hub
}

func (h *WSHandler) HandleConnection(c context.Context) error {
    // Auto-upgrades to WebSocket with hijack:"ws" tag
    conn, _ := upgrader.Upgrade(c.Response(), c.Request(), nil)
    client := hub.NewClient(h.hub, conn, id, logger)
    h.hub.RegisterClient(client)
    return nil
}
```

### API Documentation
```go
// Automatic OpenAPI/Swagger generation
import "github.com/yshengliao/gortex/core/app/doc/swagger"

app.NewApp(
    app.WithHandlers(handlers),
    app.WithDocProvider(swagger.NewProvider()),
)

// Access at:
// /_docs      - API documentation JSON
// /_docs/ui   - Swagger UI interface
```

### Configuration
```go
cfg := config.NewConfigBuilder().
    LoadYamlFile("config.yaml").
    LoadDotEnv(".env").
    LoadEnvironmentVariables("GORTEX").
    MustBuild()

app.NewApp(
    app.WithConfig(cfg),
    app.WithHandlers(handlers),
)
```

### Error Handling
```go
// Register business errors
errors.Register(ErrUserNotFound, 404, "User not found")
errors.Register(ErrUnauthorized, 401, "Unauthorized")

// Automatic error responses
func (h *UserHandler) GET(c context.Context) error {
    user, err := h.service.GetUser(c.Param("id"))
    if err != nil {
        return err // Framework handles HTTP response
    }
    return c.JSON(200, user)
}
```

## Benchmarks

| Operation | Performance | vs Echo | Memory |
|-----------|------------|---------|--------|
| Route Lookup | 541 ns/op | 45% faster | 0 allocs |
| HTTP Request | 1.57 μs/op | 15% faster | 50% less |
| Context Pool | 139 ns/op | N/A | 4 allocs |
| Smart Params | 99.9 ns/op | N/A | 1 alloc |

## Project Structure

The framework is organized into clear, purpose-driven modules:

```
gortex/
├── app/                    # Core application framework
│   ├── interfaces/         # Service interfaces
│   └── testutil/           # App-specific test utilities
├── http/                   # HTTP-related packages
│   ├── router/             # HTTP routing engine
│   ├── middleware/         # HTTP middleware
│   ├── context/            # Request/response context
│   └── response/           # Response utilities
├── websocket/              # WebSocket functionality
│   └── hub/                # WebSocket connection hub
├── auth/                   # Authentication (JWT, etc.)
├── validation/             # Input validation
├── observability/          # Monitoring & metrics
│   ├── health/             # Health checks
│   ├── metrics/            # Metrics collection
│   └── tracing/            # Distributed tracing
├── config/                 # Configuration management
├── errors/                 # Error handling
├── utils/                  # Utility packages
│   ├── pool/               # Object pools
│   ├── circuitbreaker/     # Circuit breaker pattern
│   ├── httpclient/         # HTTP client utilities
│   └── requestid/          # Request ID generation
├── middleware/             # Framework middleware
├── internal/               # Internal packages
└── examples/               # Example applications
```

## Best Practices

### 1. Structure Your Handlers
```go
// Group related endpoints
type HandlersManager struct {
    Auth    *AuthHandlers    `url:"/auth"`
    Users   *UserHandlers    `url:"/users"`
    Admin   *AdminHandlers   `url:"/admin" middleware:"auth,admin"`
}
```

### 2. Use Service Layer (Optional)
```go
type UserHandler struct {
    service *UserService // Business logic here
}

func (h *UserHandler) GET(c context.Context) error {
    user, err := h.service.GetUser(c.Request().Context(), c.Param("id"))
    // Handle response...
}
```

### 3. Leverage Development Mode
```go
cfg.Logger.Level = "debug" // Enables /_routes, /_monitor, etc.
```

## Examples

Check out the [examples](./examples) directory:
- [Simple](./examples/simple) - Basic routing and groups
- [Auth](./examples/auth) - JWT authentication
- [WebSocket](./examples/websocket) - Real-time communication
- [Advanced Tracing](./examples/advanced-tracing) - Distributed tracing with 8 severity levels
- [Metrics Dashboard](./examples/metrics-dashboard) - Prometheus + Grafana integration
- [API Docs Advanced](./examples/api-docs-advanced) - OpenAPI 3.0 documentation

## Recent Improvements (v0.4.0-alpha)

### Enhanced Observability
- **Advanced Tracing**: 8-level severity system (DEBUG to EMERGENCY)
- **Performance Tracking**: Built-in benchmarking and bottleneck detection
- **Metrics Collection**: ShardedCollector for high-performance metrics

### Developer Experience
- **Context Propagation Checker**: Static analysis tool for proper context usage
- **Performance Reports**: Weekly automated performance analysis
- **Best Practices Documentation**: Comprehensive guides for production use

### CI/CD Integration
- **Static Analysis**: 30+ linters with automatic PR comments
- **Performance Regression Tests**: Automatic detection of performance degradation
- **Benchmark Tracking**: Historical performance data with trend analysis

## Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md).

## License

MIT License - see [LICENSE](LICENSE) file.

---

<p align="center">
Built with love by the Go community | Pure Go Framework
</p>