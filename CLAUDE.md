# Gortex Framework - Development Guide

> **Framework**: Gortex | **Language**: Go 1.25 | **Status**: v0.6.1-alpha | **Updated**: 2026-04-25

Development guide for Gortex — a high-performance Go web framework with declarative struct-tag routing.

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
- **Auto-Initialization**: Handlers automatically initialized
- **Memory Efficient**: Context pooling & smart parameter storage

## Project Structure

```
gortex/
├── core/                   # Framework core
│   ├── app/                # Application lifecycle, route wiring
│   ├── context/            # Binder, request/response context
│   ├── handler/            # Handler cache, reflection helpers
│   └── types/              # Public interfaces (types.Context, …)
├── transport/
│   ├── http/               # HTTP context, router, response helpers
│   └── websocket/          # Hub, client, message authorisation
├── middleware/             # CORS, CSRF, rate limit, logger, auth, recover, compression, dev error page
├── pkg/
│   ├── auth/               # JWT (≥32-byte secret enforced)
│   ├── config/             # YAML / .env / env-var
│   ├── errors/             # Error registry
│   ├── utils/              # pool, circuitbreaker, httpclient, requestid
│   └── validation/
├── observability/          # health, metrics, tracing, otel
├── performance/            # Benchmark DB, perfcheck CLI
├── tools/                  # Standalone dev tools (separate go.mod)
│   └── analyzer/           # Context propagation static analyser
├── examples/               # basic, websocket, auth
└── internal/               # Shared test utilities
```

## Quick Start

### 1. Basic Handler with Struct Tags
```go
import "github.com/yshengliao/gortex/core/types"

type HandlersManager struct {
    Home  *HomeHandler  `url:"/"`
    Users *UserHandler  `url:"/users/:id"`
    Admin *AdminGroup   `url:"/admin" middleware:"auth"`
    WS    *WSHandler    `url:"/ws" hijack:"ws"`
}

type HomeHandler struct{}
func (h *HomeHandler) GET(c types.Context) error {
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
    
    // Advanced features
    API    *APIHandler    `url:"/api" middleware:"cors" ratelimit:"100/min"`
}
```

### 2. HTTP Method Mapping
```go
type UserHandler struct{}

func (h *UserHandler) GET(c types.Context) error    { /* GET /users/:id */ }
func (h *UserHandler) POST(c types.Context) error   { /* POST /users/:id */ }
func (h *UserHandler) Profile(c types.Context) error { /* POST /users/:id/profile */ }
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

### Performance Optimizations
- **Zero-Allocation Routing**: 0 allocs/op for static, param, wildcard, and deep-param routes (~65 ns/op)
- **Context Pool**: Embedded `responseWriter` value eliminates per-request allocation
- **Smart Params**: Inline [4]string array for ≤4 params; overflow to map
- **Reflection Caching**: 45% faster than standard routers

## Testing

### Running Tests
```bash
go test ./... -race -count=1      # Full suite with race detector (matches CI)
go vet ./...
curl localhost:8080/_routes       # View debug routes (in debug mode)
```

## Security Defaults

Hardened as of v0.4.0-alpha. Do not regress:

- `Context.File(fsys fs.FS, name string)` — rejects `../`, absolute paths, symlinks out of root; use `FileDir(dir, name)` for filesystem-rooted serving.
- `Context.Redirect` — only accepts same-origin paths by default; `RedirectOptions.AllowAbsolute` opts in specific hosts.
- `middleware/cors.go` — `CORSWithConfig` returns `error` when `AllowOrigins` contains `*` and `AllowCredentials=true`; the `CORS()` convenience panics on the same misconfig.
- `core/context.Binder` — wraps bodies in `http.MaxBytesReader` (default 1 MiB); surfaces decode errors rather than swallowing them.
- `middleware/logger.go` — `TrustedProxies` gates `X-Forwarded-For`/`X-Real-IP`; `BodyRedactor` masks JSON secret keys.
- `middleware/dev_error_page.go` — redacts `Authorization`, `Cookie`, `Set-Cookie`, `X-Api-Key`, `X-Auth-Token`, `Proxy-Authorization`, plus `(?i)(token|password|secret|key|apikey|auth)` query params.
- `middleware/csrf.go` — synchroniser-token pattern; `Secure`, `HttpOnly`, `SameSite=Lax`.
- `middleware/ratelimit.go` — emits `X-RateLimit-Limit/Remaining/Reset` on every response and `Retry-After` on 429.
- `pkg/auth.NewJWTService` — returns an error for secrets shorter than 32 bytes.
- `transport/websocket` — `Config.MaxMessageBytes` sets `conn.SetReadLimit`; unknown/unauthorised messages are dropped with a log line.

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

func (h *UserHandler) GET(c types.Context) error {
    user, err := h.service.GetUser(c.Param("id"))
    if err != nil {
        return err // Framework handles HTTP response
    }
    return c.JSON(200, user)
}
```

### WebSocket Setup
```go
import gortexws "github.com/yshengliao/gortex/transport/websocket"

type WSHandler struct {
    hub *gortexws.Hub
}

func (h *WSHandler) HandleConnection(c types.Context) error {
    conn, _ := upgrader.Upgrade(c.Response(), c.Request(), nil)
    client := gortexws.NewClient(h.hub, conn, clientID, logger)
    h.hub.RegisterClient(client) // synchronous; returns only after hub records client
    go client.WritePump()
    go client.ReadPump()
    return nil
}
```

Hardening knobs on the hub:

```go
hub := gortexws.NewHubWithConfig(logger, gortexws.Config{
    MaxMessageBytes:     4 << 10,
    AllowedMessageTypes: []string{"chat", "ping"},
    Authorizer:          myAuthorizer, // func(*Client, *Message) error
})
```

### Dependency Injection (Not Yet Implemented)
```go
// NOTE: The `inject` tag is parsed but actual DI is NOT implemented.
// Fields with `inject` tag must be set manually before RegisterRoutes,
// or the framework will return an error for nil pointers.
type UserService struct {
    DB *sql.DB `inject:""`  // Must be set manually; auto-inject is planned
}

// Set manually before registration:
handlers.UserService.DB = dbConnection
```

## Framework Development

### Completed Features (v0.6.1-alpha)
**Core Features**
- Struct tag routing with segment-trie router (45% faster than radix tree)
- **Zero-allocation routing hot path** (0 allocs/op, ~65 ns/op on M3 Pro)
- WebSocket support with hub pattern, size limits, type whitelist, authoriser hook
- JWT authentication with ≥32-byte secret enforcement
- Zero-dependency config loader (YAML, .env, env vars, CLI args)
- Development tools (`/_routes`, `/_monitor`, `/_config` with secret masking)
- Bilingual documentation (English + Traditional Chinese)

**Security Hardening**
- Path-traversal-safe `File()` and `Redirect()`
- CORS, CSRF, rate-limit middleware with proper response headers
- JSON body capped at 1 MiB; multipart at 32 MiB
- Logger redacts secrets in headers and JSON body

**Developer Experience**
- Auto handler initialisation — no nil pointer panics
- Route logging with live data (not placeholder)
- Context helper methods — `OK()`, `Created()`
- Dev error pages with header/query redaction

**Dependency Hygiene**
- Removed `Bofry/config` + 5 indirect deps
- Isolated `x/tools` to standalone `tools/analyzer/` module
- `otel/sdk` annotated as test-only
- Total modules: 50 → 41 (direct 13→11, indirect 23→16)

### Development Guidelines
- **Tests Required**: Unit tests + benchmarks for all changes
- **Examples Updated**: Verify affected examples still work
- **Documentation Current**: Keep README.md performance metrics updated
- **Zero Regressions**: `go test ./...` must pass before commits

### Performance Targets
- **Routing**: <100 ns/op (currently ~65 ns/op, 0 allocs)
- **Memory**: Zero allocations on routing hot path
- **Throughput**: >10k RPS on standard hardware

## Framework Context

### Positioning
**Gortex** is a **lightweight, self-contained** Go web framework with **no external infrastructure dependencies** (no Redis, Kafka, etc.). Ideal for:
- **Real-time applications**: WebSocket-heavy apps
- **Microservices**: Fast-starting, minimal footprint services  
- **Rapid prototyping**: No infrastructure setup required
- **Edge computing**: Minimal resource usage

### Design Philosophy
1. **Simplicity First**: If a feature needs explanation, redesign it
2. **Convention Over Configuration**: Sensible defaults everywhere
3. **Errors Should Help**: Every error tells you how to fix it
4. **Progressive Complexity**: Simple things simple, complex things possible

### Not Goals
- ❌ Not chasing extreme performance at the cost of usability
- ❌ Not implementing complex optimizations that confuse developers
- ❌ Not sacrificing developer experience for minor gains
- ❌ Not increasing learning curve unnecessarily

---

**Last Updated**: 2026-04-25 | **Framework**: Gortex v0.6.1-alpha | **Go**: 1.25+