# Gortex - Go Web Framework with Struct Tag Routing

[![Go Version](https://img.shields.io/badge/go-1.24+-blue.svg)](https://go.dev/)
![Status](https://img.shields.io/badge/status-v0.3.0--alpha-orange.svg)
[![License](https://img.shields.io/badge/license-MIT-brightgreen.svg)](LICENSE)

> **Zero boilerplate web framework for Go. Define routes with struct tags, not code.**

## ✨ Why Gortex?

```go
// ❌ Traditional: Manual route registration
e.GET("/", homeHandler)
e.GET("/users/:id", userHandler)
e.GET("/api/v1/users", apiV1UserHandler)
// ... dozens more routes

// ✅ Gortex: Automatic discovery from struct tags
type HandlersManager struct {
    Home  *HomeHandler  `url:"/"`
    Users *UserHandler  `url:"/users/:id"`
    API   *APIGroup     `url:"/api"`
}
```

## 🚀 Quick Start

```bash
go get github.com/yshengliao/gortex
```

```go
package main

import (
    "github.com/labstack/echo/v4"
    "github.com/yshengliao/gortex/app"
)

// Define routes with struct tags
type HandlersManager struct {
    Home   *HomeHandler   `url:"/"`
    Users  *UserHandler   `url:"/users/:id"`
    Admin  *AdminGroup    `url:"/admin" middleware:"auth"`
    WS     *WSHandler     `url:"/ws" hijack:"ws"`
}

type HomeHandler struct{}
func (h *HomeHandler) GET(c echo.Context) error {
    return c.JSON(200, map[string]string{"message": "Welcome!"})
}

type UserHandler struct{}
func (h *UserHandler) GET(c echo.Context) error {
    return c.JSON(200, map[string]string{"id": c.Param("id")})
}

type AdminGroup struct {
    Dashboard *DashboardHandler `url:"/dashboard"`
}

type DashboardHandler struct{}
func (h *DashboardHandler) GET(c echo.Context) error {
    return c.JSON(200, map[string]string{"data": "admin only"})
}

type WSHandler struct{}
func (h *WSHandler) GET(c echo.Context) error {
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

## 📚 Core Concepts

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

func (h *UserHandler) GET(c echo.Context) error    { /* GET /users/:id */ }
func (h *UserHandler) POST(c echo.Context) error   { /* POST /users/:id */ }
func (h *UserHandler) DELETE(c echo.Context) error { /* DELETE /users/:id */ }
func (h *UserHandler) Profile(c echo.Context) error { /* POST /users/:id/profile */ }
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

## 🔥 Key Features

### Performance
- **45% Faster Routing** - Optimized reflection caching
- **Zero Dependencies** - No Redis, Kafka, or external services
- **Memory Efficient** - Built-in object pooling

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

## 📦 Middleware

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

## 🛠️ Advanced Features

### WebSocket Support
```go
type WSHandler struct {
    hub *hub.Hub
}

func (h *WSHandler) GET(c echo.Context) error {
    // Auto-upgrades to WebSocket with hijack:"ws" tag
    conn, _ := upgrader.Upgrade(c.Response(), c.Request(), nil)
    client := hub.NewClient(h.hub, conn, id, logger)
    h.hub.RegisterClient(client)
    return nil
}
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
func (h *UserHandler) GET(c echo.Context) error {
    user, err := h.service.GetUser(c.Param("id"))
    if err != nil {
        return err // Framework handles HTTP response
    }
    return c.JSON(200, user)
}
```

## 📊 Benchmarks

| Operation | Performance | vs Echo |
|-----------|------------|---------|
| Route Lookup | 541 ns/op | 45% faster |
| HTTP Request | 1.57 μs/op | 15% faster |
| Memory Usage | 0 allocs | Same |

## 🎯 Best Practices

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

func (h *UserHandler) GET(c echo.Context) error {
    user, err := h.service.GetUser(c.Request().Context(), c.Param("id"))
    // Handle response...
}
```

### 3. Leverage Development Mode
```go
cfg.Logger.Level = "debug" // Enables /_routes, /_monitor, etc.
```

## 📖 Examples

Check out the [examples](./examples) directory:
- [Simple](./examples/simple) - Basic routing and groups
- [Auth](./examples/auth) - JWT authentication
- [WebSocket](./examples/websocket) - Real-time communication

## 🤝 Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md).

## 📝 License

MIT License - see [LICENSE](LICENSE) file.

---

<p align="center">
Built with ❤️ by the Go community | Powered by <a href="https://echo.labstack.com">Echo</a>
</p>