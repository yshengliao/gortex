# Gortex - High-Performance Go Web Framework

[![Go Version](https://img.shields.io/badge/go-1.25+-blue.svg)](https://go.dev/)
![Status](https://img.shields.io/badge/status-v0.5.2--alpha-orange.svg)
[![License](https://img.shields.io/badge/license-MIT-brightgreen.svg)](LICENSE)

> **⚠️ [RESEARCH ONLY] This project is a local replica of past commonly used infrastructure architectural philosophies. It is maintained for record-keeping and architectural learning, and is NOT intended for production use.**
>
> [繁體中文](README_ZH_TW.md)

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
- JSON body capped at 1 MiB (configurable via `DefaultMaxBodyBytes`); multipart capped at 32 MiB
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
```

Global middleware is applied via `router.Use()`:

```go
app, _ := app.NewApp(app.WithHandlers(handlers))
app.Router().Use(
    middleware.Logger(logger),
    middleware.RequestID(),
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

Measured on Apple M3 Pro, Go 1.24, `transport/http` package, `-benchmem -benchtime=2s`.

| Operation | gortexRouter (production) | Memory | Notes |
|-----------|--------------------------|--------|-------|
| Static route | 163 ns/op | 288 B, 6 allocs | `GET /user/home` |
| Param route | 267 ns/op | 576 B, 7 allocs | `GET /user/:name` |
| Many routes (23 routes) | 325 ns/op | 624 B, 7 allocs | deep param lookup |
| Context Pool | 122 ns/op | 208 B, 4 allocs | pooled vs 230 ns unpooled |
| Smart Params | 89 ns/op | 48 B, 1 alloc | struct-tag param binding |

> Measured with the production segment-trie router (`gortex_router.go`).

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
├── tools/                  # Standalone dev tools (separate go.mod)
│   └── analyzer/           # Context propagation static analyser
├── examples/               # basic, websocket, auth
└── internal/               # Shared test utilities
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

Runnable references — each is a single `main.go` with a companion README and `curl`/`websocat` transcript.

| Example | What it shows |
|---------|---------------|
| [basic](examples/basic/) | Struct-tag routing, binder, default middleware chain |
| [auth](examples/auth/) | JWT login / refresh / `/me` with `NewJWTService` entropy check |
| [websocket](examples/websocket/) | Hub config with message-size limit, type whitelist, authoriser hook |

```bash
go run ./examples/basic      # all listen on :8080
```

## Documentation

| Document | Description |
|----------|-------------|
| [API Reference](docs/en/API.md) | Context interface, router, struct tags, middleware, WebSocket, security defaults |
| [Security Guide](docs/en/security.md) | Safe usage patterns for file serving, redirects, CORS, JSON body limits |
| [Context Handling](docs/en/best-practices/context-handling.md) | Lifecycle, cancellation, goroutines, timeout strategies |
| [Metrics Analysis](docs/en/performance/metrics-analysis.md) | Collector benchmarks and selection guide |
| [Architecture Philosophy](docs/en/architecture-philosophy.md) | Framework design decisions and rationale |
| [Design Patterns](docs/en/design-patterns.md) | 10 core patterns implemented and learning path |
| [SECURITY.md](SECURITY.md) | Vulnerability reporting process and supported versions |

## Changelog

### v0.5.2-alpha (2026-04-24)

**Documentation**
- Restructured `docs/` into `docs/en/` and `docs/zh-tw/` for bilingual documentation.
- Added `architecture-philosophy.md`: explains the Kitchen-sink design rationale, Jaeger/OTel tracing context, and K8s multi-environment config strategy.
- Added `design-patterns.md`: catalogues 10 core engineering patterns (Segment-trie, sync.Pool, SBO, Lock Sharding, Circuit Breaker, Actor-model Hub, etc.) with a difficulty-ranked learning path.
- Translated all existing docs (API, Security, Context Handling, Metrics Analysis) to Traditional Chinese (Taiwan style).
- Updated both READMEs with bilingual doc links.

### v0.5.1-alpha (2026-04-24)

**Project Status**
- Explicitly marked as a research and architectural record project. Not intended for production use.
- Added comprehensive client test examples utilizing `httptest` and the built-in `httpclient`.

### v0.4.1-alpha (2026-04-24)

**Security & Reliability**
- `Context.Bind()` enforces 1 MiB body cap via `http.MaxBytesReader`; `Context.Validate()` returns `ErrValidatorNotRegistered` instead of silent nil.
- `Hub.Shutdown()` / `ShutdownWithTimeout()` now idempotent via `sync.Once`.
- `MemoryRateLimiter` refactored with TTL-tracked entries and background cleanup.

**Architecture**
- Removed duplicate radix-tree router; kept production segment-trie `gortex_router.go`.
- `/_routes` and `/_monitor` now return live data; `/_config` masks sensitive values.
- `injectDependencies` skips reflect scan when no `inject` tag present (fast path).

**Dependencies** — main module reduced from 50 → 41 modules (direct 13→11, indirect 23→16).
- Removed `Bofry/config`; `pkg/config` uses zero-dependency `simpleLoader` (YAML + .env + env vars + CLI).
- Moved `internal/analyzer` to standalone `tools/analyzer/` with own `go.mod`, removing `golang.org/x/tools`.
- `otel/sdk` annotated test-only; updated `golang.org/x/crypto`.

**Hygiene** — removed Echo remnants, dead code stubs, fictitious APIs, and LLM-generated placeholder docs. Consolidated config tests (876 → 532 lines). Updated `.golangci.yml`, `.gitignore`, and all documentation to match current state.

### v0.4.0-alpha

**Security hardening**
- Path-traversal-safe `Context.File` and `Context.Redirect`
- CORS, dev error page, logger and binder hardened against common misuse
- JWT secret entropy check, trusted-proxy client-IP, WebSocket read limits + authoriser
- CSRF middleware and rate-limit response headers

**Enhanced Observability**
- 8-level severity tracing (DEBUG to EMERGENCY)
- Built-in benchmarking and bottleneck detection
- ShardedCollector for high-throughput metrics

**CI/CD**
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