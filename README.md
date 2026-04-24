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

type HandlersManager struct {
    Home   *HomeHandler   `url:"/"`
    Users  *UserHandler   `url:"/users/:id"`
    Admin  *AdminGroup    `url:"/admin" middleware:"auth"`
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

func main() {
    app, _ := app.NewApp(
        app.WithHandlers(&HandlersManager{
            Home:  &HomeHandler{},
            Users: &UserHandler{},
            Admin: &AdminGroup{
                Dashboard: &DashboardHandler{},
            },
        }),
    )
    app.Run() // :8080
}
```

## Core Concepts

### Struct Tag Routing

```go
type HandlersManager struct {
    Users    *UserHandler    `url:"/users/:id"`                   // Dynamic params
    Static   *FileHandler    `url:"/static/*"`                    // Wildcards
    API      *APIGroup       `url:"/api"`                         // Nested groups
    Profile  *ProfileHandler `url:"/profile" middleware:"jwt"`    // Protected
    Chat     *ChatHandler    `url:"/chat" hijack:"ws"`            // WebSocket
}
```

### HTTP Method Mapping

```go
type UserHandler struct{}

func (h *UserHandler) GET(c types.Context) error    { /* GET /users/:id */ }
func (h *UserHandler) POST(c types.Context) error   { /* POST /users/:id */ }
func (h *UserHandler) DELETE(c types.Context) error { /* DELETE /users/:id */ }
func (h *UserHandler) Profile(c types.Context) error { /* POST /users/:id/profile */ }
```

### Middleware

```go
// Via struct tags
type HandlersManager struct {
    Public  *PublicHandler  `url:"/public"`
    Private *PrivateHandler `url:"/private" middleware:"auth"`
    Admin   *AdminHandler   `url:"/admin" middleware:"auth,rbac"`
}

// Or globally
app.Router().Use(middleware.Logger(logger), middleware.RequestID())
```

### Configuration (Multi-source)

```go
cfg := config.NewConfigBuilder().
    LoadYamlFile("config.yaml").       // Local dev
    LoadDotEnv(".env").                // Overrides
    LoadEnvironmentVariables("GORTEX"). // K8s env injection
    MustBuild()
```

## Framework Highlights

| Category | Features |
|----------|----------|
| **Routing** | Struct-tag auto-discovery, segment-trie, nested groups, context pooling |
| **Security** | Path-traversal-safe `File`, origin-locked `Redirect`, CORS guard, 1 MiB body cap, CSRF, secret redaction |
| **Observability** | Jaeger/OTel tracing, sharded metrics, health checks (healthy/degraded/unhealthy), `/_routes` & `/_monitor` |
| **Resilience** | Circuit breaker, token-bucket rate limiter with TTL cleanup, graceful shutdown |
| **Real-time** | WebSocket Hub with read-size limits, type whitelist, and authoriser hooks |

## Examples

```bash
go run ./examples/basic      # Struct-tag routing, binder, middleware chain
go run ./examples/auth       # JWT login / refresh / /me
go run ./examples/websocket  # Hub with message limits and authoriser
```

## Documentation

Full technical documentation is available in both English and Traditional Chinese:

- 📖 **[English Documentation](docs/en/)** — API reference, security guide, architecture philosophy, design patterns, best practices
- 📖 **[繁體中文文件](docs/zh-tw/)** — API 參考、安全指南、架構哲學、設計模式、最佳實踐
- 🔒 [SECURITY.md](SECURITY.md) — Vulnerability reporting process

## Changelog

### v0.5.2-alpha (2026-04-24)

- Restructured `docs/` into `docs/en/` and `docs/zh-tw/` with full bilingual coverage.
- Added architecture philosophy and design patterns learning guide.

### v0.5.1-alpha (2026-04-24)

- Marked as research and architectural record project.

### v0.4.1-alpha (2026-04-24)

- Security hardening (body cap, idempotent shutdown, rate limiter TTL).
- Removed duplicate router; dependency count reduced from 50 → 41 modules.

### v0.4.0-alpha

- Path-traversal-safe file serving, CORS/CSRF/JWT hardening.
- 8-level severity tracing, ShardedCollector, CI/CD pipeline.

## License

MIT License — see [LICENSE](LICENSE).