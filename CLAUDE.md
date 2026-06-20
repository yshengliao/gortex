# Gortex Framework - Development Guide

> **Framework**: Gortex | **Language**: Go 1.25 | **Status**: v0.8.1-alpha | **Updated**: 2026-06-19

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
- **Fast Routing**: Segment-trie router with zero-allocation hot path (~65 ns/op, 0 allocs/op for trie traversal on Apple M3 Pro; not reproduced in CI — see Performance Optimizations)
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
    
    // With middleware — built-in names: auth, requestid, recover
    // "auth" requires a middleware.MiddlewareFunc registered in the app context
    // "rbac" is NOT implemented and will fail registration (error at NewApp time)
    Auth   *AuthHandler   `url:"/auth"`
    Admin  *AdminHandler  `url:"/admin" middleware:"auth"`
    
    // WebSocket
    Chat   *ChatHandler   `url:"/chat" hijack:"ws"`
    
    // Advanced features
    API    *APIHandler    `url:"/api" ratelimit:"100/min"`
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
- Request logging with body capture (request bodies only; response-body logging is not supported — `LogResponseBody` is a no-op)

### Performance Optimizations
- **Zero-Allocation Routing**: 0 allocs/op for trie traversal on static, param, wildcard, and deep-param routes (~65 ns/op on Apple M3 Pro; numbers from a maintainer machine, not reproduced in CI — full request context setup adds ~3 allocs/op from map and response-writer initialization)
- **Context Pool**: Embedded `responseWriter` value eliminates per-request allocation
- **Smart Params**: Inline [4]string array for ≤4 params; overflow to ordered slice (preserves insertion order)
- **Segment-Trie Router**: Predictable O(segments) matching without regex backtracking

## Testing

### Running Tests
```bash
go test ./... -race -count=1      # Full suite with race detector (matches CI)
go vet ./...
curl localhost:8080/_routes       # View debug routes (in debug mode)
```

## Security Defaults

Hardened as of v0.4.0-alpha. Do not regress:

- `Context.File(file string)` — server-trusted path; it is cleaned and `..` traversal is rejected (it does **not** block absolute paths or resolve symlinks). For user-supplied filenames use `Context.FileFS(fsys fs.FS, name string)`, which validates with `fs.ValidPath` (wrap a directory with `os.DirFS`).
- `Context.Redirect` — accepts same-origin paths only (must start with `/`, not `//`; rejects CR/LF/NUL). To redirect to an external host, set the `Location` header directly after validating it against your own allowlist.
- `middleware/cors.go` — `CORSWithConfig` returns `error` when `AllowOrigins` contains `*` and `AllowCredentials=true`; the `CORS()` convenience panics on the same misconfig.
- `core/context.Binder` — wraps each JSON body in `http.MaxBytesReader` (default 1 MiB via `DefaultMaxJSONBodyBytes`); original `Body` is restored after binding; fields with an explicit `bind` tag surface conversion errors. Multipart is capped at 32 MiB.
- `middleware/logger.go` — `TrustedProxies` CIDRs gate `X-Forwarded-For`/`X-Real-IP` for IP resolution; `BodyRedactor` masks sensitive JSON fields in **request** bodies. `LogResponseBody` is a documented no-op — response-body logging is not supported; 4xx/5xx log at Warn/Error using the real status from the tracked writer.
- `middleware/dev_error_page.go` — redacts headers: `Authorization`, `Cookie`, `Set-Cookie`, `X-Api-Key`, `X-Auth-Token`, `X-CSRF-Token`, `Proxy-Authorization`; masks query params matching `(?i)(token|password|secret|key|apikey|api_key|auth)`.
- `middleware/csrf.go` — double-submit cookie pattern (constant-time compare via `subtle.ConstantTimeCompare`; no server-side token store); `Secure`, `HttpOnly`, `SameSite=Lax` on the cookie; token echoed in `X-CSRF-Token` response header for SPA bootstrapping.
- `middleware/ratelimit.go` — emits `X-RateLimit-Limit/Remaining/Reset` on every response and `Retry-After` on 429. Default `KeyFunc` keys on the **direct peer address** (spoof-resistant); set `TrustedProxies` CIDRs to honour forwarding headers only from known proxies. `DefaultGortexRateLimitConfig()` leaves `Store` nil — the middleware creates a stoppable `MemoryRateLimiter` on first use.
- `pkg/auth.NewJWTService` — returns an error for secrets shorter than 32 bytes. Tokens carry a `typ` claim (`"access"` or `"refresh"`); tokens issued by earlier versions without `typ` are **rejected**. Signing and verification are pinned to HS256 exactly.
- `pkg/config.Validate` — enforces 32-byte minimum JWT secret at config-load time.
- `transport/websocket` — `Config.MaxMessageBytes` sets `conn.SetReadLimit`; unknown/unauthorised messages are dropped with a log line. Hub metrics expose `dropped_broadcasts` and `forced_disconnects`. Private message `Target` is resolved before the Authorizer runs, so the Authorizer sees the final recipient.

## Critical Don'ts

- **No Global State**: Keep state in handlers or services
- **No Mixed Concerns**: Separate HTTP from business logic
- **No Hardcoded Values**: Use configuration files
- **No Unvalidated Input**: Always validate user data
- **No Context Ignoring**: Handle cancellation properly

## Common Patterns

### Error Handling
```go
// Register business errors: Register(err, code ErrorCode, httpStatus int, message string)
errors.Register(ErrUserNotFound, errors.CodeResourceNotFound, http.StatusNotFound, "User not found")

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
    // Synchronous: nil means the hub recorded the client. ErrHubShuttingDown
    // means it was refused and the client's send channel is already closed.
    if err := h.hub.RegisterClient(client); err != nil {
        return err
    }
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
    // Authorizer receives the resolved Target (for private messages)
    // and can veto the final recipient. Private-targeting policy belongs here.
})
```

Hub metrics include `dropped_broadcasts` and `forced_disconnects`. `ShutdownWithTimeout` completes sub-500ms on an idle hub; registered clients drain queued messages before send channels close.

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

### Completed Features (v0.8.0-alpha)
**Core Features**
- Struct tag routing with segment-trie router (zero-allocation trie traversal hot path, ~65 ns/op on Apple M3 Pro; not reproduced in CI)
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

**Third Audit Round (v0.8.0-alpha)**
- JWT `typ` claim, HS256-only signing, fail-loud middleware tags, rate-limit TrustedProxies
- Named wildcard params (`c.Param("filepath")`), router error double-write fixed, shared group lock, middleware chain aliasing
- Logger reads real status; `LogResponseBody` documented no-op; ws hub responsive shutdown; private-target resolved before Authorizer
- Compression Vary header, CORS wildcard-headers+credentials guard, auth SkipPaths segment-boundary matching
- `OptimizedCollector` and `FixedHealthChecker` removed; `RouteCache` removed; config no longer mutates `os.Environ`

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
- **Routing**: ~65 ns/op trie traversal on Apple M3 Pro (not reproduced in CI; Linux/x86 runs ~275–315 ns/op with 3 allocs/op from context setup)
- **Memory**: Zero allocations in trie traversal hot path; full request context setup adds ~3 allocs
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
- ❌ OpenAPI/Swagger spec generation: skeleton exists (`core/app/doc/`), not yet functional

---

**Last Updated**: 2026-06-20 | **Framework**: Gortex v0.8.1-alpha | **Go**: 1.25+