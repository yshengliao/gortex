# Gortex Framework - Development Guide

> **Framework**: Gortex | **Language**: Go 1.24 | **Status**: v0.3.0-alpha | **Updated**: 2025/07/25

Development guide for Gortex web framework - a high-performance Go framework with declarative struct tag routing.

## Core Concepts

**Gortex** eliminates route registration boilerplate through struct tag routing:

```go
// Traditional
r.GET("/", homeHandler)
r.GET("/users/:id", userHandler)
r.POST("/api/login", loginHandler)

// Gortex - automatic discovery
type HandlersManager struct {
    Home  *HomeHandler  `url:"/"`
    Users *UserHandler  `url:"/users/:id"`
    API   *APIGroup     `url:"/api"`
}
```

### Key Features
- **Zero Dependencies**: No Redis, Kafka, external services
- **45% Faster Routing**: Optimized reflection caching  
- **WebSocket Native**: First-class real-time support
- **Type-Safe**: Compile-time route validation

## Quick Start

### 1. Basic Handler with Struct Tags
```go
type HandlersManager struct {
    Home  *HomeHandler  `url:"/"`
    Users *UserHandler  `url:"/users/:id"`
    Admin *AdminGroup   `url:"/admin" middleware:"auth"`
    WS    *WSHandler    `url:"/ws" hijack:"ws"`
}

type HomeHandler struct{}
func (h *HomeHandler) GET(c context.Context) error {
    return c.JSON(200, map[string]string{"message": "Hello Gortex!"})
}
```

### 2. Nested Groups
```go
type AdminGroup struct {
    Dashboard *DashboardHandler `url:"/dashboard"`
    Users     *UsersHandler     `url:"/users/:id"`
}
// Results in: /admin/dashboard, /admin/users/:id
```

## Best Practices

### 1. Struct Tag Reference
```go
type HandlersManager struct {
    // Basic routing
    Home   *HomeHandler   `url:"/"`
    Users  *UserHandler   `url:"/users/:id"`      // Dynamic params
    Static *FileHandler   `url:"/static/*"`        // Wildcards
    
    // With middleware
    Auth   *AuthHandler   `url:"/auth"`
    Admin  *AdminHandler  `url:"/admin" middleware:"jwt,rbac"`
    
    // WebSocket
    Chat   *ChatHandler   `url:"/chat" hijack:"ws"`
}
```

### 2. HTTP Method Mapping
```go
type UserHandler struct{}

func (h *UserHandler) GET(c context.Context) error    { /* GET /users/:id */ }
func (h *UserHandler) POST(c context.Context) error   { /* POST /users/:id */ }
func (h *UserHandler) Profile(c context.Context) error { /* POST /users/:id/profile */ }
```

### 3. Configuration Setup
```go
cfg := config.NewConfigBuilder().
    LoadYamlFile("config.yaml").
    LoadDotEnv(".env").
    LoadEnvironmentVariables("GORTEX").
    MustBuild()

app, _ := app.NewApp(
    app.WithConfig(cfg),
    app.WithHandlers(handlers),
)
```

## Development Tools

### Debug Mode Features
With `cfg.Logger.Level = "debug"`:
- `/_routes` - List all registered routes
- `/_monitor` - System metrics dashboard
- `/_config` - Masked configuration view
- Request/response logging with body capture

### Performance Targets
- **Routing**: 541 ns/op (45% faster than Echo)
- **Memory**: Zero allocations for cached routes
- **Throughput**: >10k RPS on standard hardware

## Testing

### Example Projects
- **[Simple](./examples/simple)** - Basic routing and groups (port 8080)
- **[Auth](./examples/auth)** - JWT authentication (port 8081)  
- **[WebSocket](./examples/websocket)** - Real-time communication (port 8082)

### Running Tests
```bash
go test ./...           # Run all tests
go run examples/simple  # Test basic routing
curl localhost:8080/_routes  # View debug routes
```

## Critical Don'ts

- **No Global State**: Keep state in handlers or services
- **No Mixed Concerns**: Separate HTTP from business logic
- **No Hardcoded Values**: Use configuration files
- **No Unvalidated Input**: Always validate user data
- **No Context Ignoring**: Handle cancellation properly

## Common Patterns

### Error Handling
```go
// Register business errors
errors.Register(ErrUserNotFound, 404, "User not found")

func (h *UserHandler) GET(c context.Context) error {
    user, err := h.service.GetUser(c.Param("id"))
    if err != nil {
        return err // Framework handles HTTP response
    }
    return c.JSON(200, user)
}
```

### WebSocket Setup
```go
type WSHandler struct {
    hub *hub.Hub
}

func (h *WSHandler) HandleConnection(c context.Context) error {
    conn, _ := upgrader.Upgrade(c.Response(), c.Request(), nil)
    client := hub.NewClient(h.hub, conn, clientID, logger)
    h.hub.RegisterClient(client)
    return nil
}
```

## Framework Context

### Positioning
**Gortex** is a **lightweight, self-contained** Go web framework with **zero external dependencies**. Ideal for:
- **Real-time applications**: WebSocket-heavy apps
- **Microservices**: Fast-starting, minimal footprint services  
- **Rapid prototyping**: No infrastructure setup required
- **Edge computing**: Minimal resource usage

### Current Status (v0.3.0-alpha)
✅ **Core Features Complete**
- Struct tag routing with 45% performance improvement
- WebSocket support with hub pattern and metrics
- JWT authentication with middleware integration
- Multi-source configuration (YAML, .env, env vars)
- Development tools (debug endpoints, monitoring)

✅ **Production Optimizations**
- Memory leak fixes in metrics and rate limiting
- Race condition resolution in health checks
- Graceful shutdown with WebSocket client notification
- Error handling unification with categorized codes

### Development Guidelines
- **Tests Required**: Unit tests + benchmarks for all changes
- **Examples Updated**: Verify affected examples still work
- **Documentation Current**: Keep README.md performance metrics updated
- **Zero Regressions**: `go test ./...` must pass before commits

---

**Last Updated**: 2025/07/26 | **Framework**: Gortex v0.3.0-alpha | **Go**: 1.24+

