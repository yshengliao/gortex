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

func (h *UserHandler) GET(c types.Context) error    { /* GET /users/:id */ }
func (h *UserHandler) POST(c types.Context) error   { /* POST /users/:id */ }
func (h *UserHandler) DELETE(c types.Context) error { /* DELETE /users/:id */ }
func (h *UserHandler) Profile(c types.Context) error { /* POST /users/:id/profile */ }
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
- **JWT Auth** - Built-in authentication middleware with ≥32-byte secret enforcement
- **WebSocket** - First-class real-time support with read-size limits, type whitelisting and authoriser hooks
- **Metrics** - Prometheus-compatible metrics
- **Graceful Shutdown** - Proper connection cleanup
- **API Documentation** - Automatic OpenAPI/Swagger generation from struct tags

### Security-first defaults
- `Context.File` only serves from an `fs.FS` (path-traversal-safe)
- `Context.Redirect` rejects off-origin targets unless explicitly allow-listed
- CORS refuses `*` + `AllowCredentials=true` misconfigurations
- JSON body capped at 10 MiB (configurable); multipart capped at 32 MiB
- Logger redacts common secret headers and JSON keys; `X-Forwarded-For` only trusted for configured proxies
- Synchroniser-token CSRF middleware + `X-RateLimit-*` / `Retry-After` headers

Reporting process: see [SECURITY.md](SECURITY.md). Full defaults: see [docs/security.md](docs/security.md).

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
import gortexws "github.com/yshengliao/gortex/transport/websocket"

type WSHandler struct {
    hub *gortexws.Hub
}

func (h *WSHandler) HandleConnection(c types.Context) error {
    // Tag `hijack:"ws"` marks the route for upgrade.
    conn, _ := upgrader.Upgrade(c.Response(), c.Request(), nil)
    client := gortexws.NewClient(h.hub, conn, id, logger)
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
func (h *UserHandler) GET(c types.Context) error {
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

```
gortex/
├── core/                   # Framework core
│   ├── app/                # Application, lifecycle, route wiring
│   ├── context/            # Binder, request/response context
│   ├── handler/            # Handler cache & reflection helpers
│   └── types/              # Public interfaces (types.Context, …)
├── transport/              # I/O surfaces
│   ├── http/               # HTTP context, router, response helpers
│   └── websocket/          # Hub, client, message authorisation
├── middleware/             # CORS, CSRF, rate limit, logger, auth, recover, …
├── pkg/                    # Reusable building blocks
│   ├── auth/               # JWT (≥32-byte secret enforced)
│   ├── config/             # YAML / .env / env-var config
│   ├── errors/             # Error registry
│   ├── utils/              # Pool, circuit breaker, httpclient, requestid
│   └── validation/         # Input validation
├── observability/          # health, metrics, tracing, otel
├── performance/            # Benchmark DB, weekly reports, perfcheck CLI
├── examples/               # basic, websocket, auth
└── internal/               # Analyser tools, shared test utilities
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

func (h *UserHandler) GET(c types.Context) error {
    user, err := h.service.GetUser(c.Request().Context(), c.Param("id"))
    // Handle response...
}
```

### 3. Leverage Development Mode
```go
cfg.Logger.Level = "debug" // Enables /_routes, /_monitor, etc.
```

## Examples

Runnable references live under [`examples/`](examples/):

- [`examples/basic`](examples/basic) — struct-tag routing + binder + validator.
- [`examples/websocket`](examples/websocket) — chat demo exercising message-size limits and the authoriser hook.
- [`examples/auth`](examples/auth) — JWT login / refresh / `/me` flow using the entropy-checked `NewJWTService`.

Each example has its own README with a `curl`/`websocat` transcript covering the golden path and rejection cases.

## Recent Improvements (v0.4.0-alpha)

### Security hardening
- Path-traversal-safe `Context.File` and `Context.Redirect`
- CORS, dev error page, logger and binder hardened against common misuse
- JWT secret entropy check, trusted-proxy client-IP, WebSocket read limits + authoriser
- CSRF middleware and rate-limit response headers

### Enhanced Observability
- 8-level severity tracing (DEBUG to EMERGENCY)
- Built-in benchmarking and bottleneck detection
- ShardedCollector for high-throughput metrics

### CI/CD
- `go test ./... -race -count=1` on every PR
- `go vet` + static analysis; benchmark history tracked in `performance/`

## Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md).

## License

MIT License - see [LICENSE](LICENSE) file.

---

<p align="center">
Built with love by the Go community | Pure Go Framework
</p>