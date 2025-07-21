# Gortex Framework - Claude AI Assistant Guide

> **Framework**: Gortex | **Language**: Go 1.24 | **Updated**: 2025/07/21

This file provides guidance to Claude Code when working with the Gortex game server framework.

## 🎯 Framework Overview

**Gortex** (Go + Vortex) is a high-performance Go backend framework designed for game servers, featuring declarative routing, first-class WebSocket support, and developer-friendly conventions.

### Architecture Highlights
- **HTTP Server**: Echo v4 with middleware stack
- **Routing System**: Declarative via struct tags (`url:"/path"`, `hijack:"ws"`)  
- **Dependency Injection**: Lightweight `AppContext` container
- **WebSocket**: Gorilla WebSocket + Hub/Client pattern
- **Configuration**: Builder pattern with multi-source support
- **Observability**: ✅ High-performance ImprovedCollector (163ns/op, zero memory leaks)
- **External Dependencies**: ✅ Zero external services (Redis, Jaeger, Prometheus not required)

### Core Design Principles
1. **宣告式優於命令式** → Routes via struct tags, not manual registration
2. **約定優於配置** → Minimal configuration, maximum convention
3. **開發者體驗優先** → Hot reload in dev, optimized builds for production

## Project Structure

```
/gortex
├── cmd/server/main.go         # Application entry point
├── internal/
│   ├── app/                   # Core application (App struct, DI, router, lifecycle)
│   │   ├── app.go            # Main application structure
│   │   ├── di.go             # Dependency injection container
│   │   ├── router_reflection.go   # Dev mode reflection-based routing
│   │   └── router_generated.go    # Prod mode generated routing (via go:generate)
│   ├── config/               # Configuration loading and models
│   ├── handlers/             # All HTTP and WebSocket handlers
│   │   ├── manager.go        # HandlersManager - central routing declaration
│   │   └── *.go             # Individual handler implementations
│   ├── hub/                  # WebSocket hub (connection management)
│   └── services/             # Business logic layer
├── pkg/                      # Reusable packages
│   ├── response/             # Standardized API responses
│   └── validator/            # Custom validator implementation
└── config.yaml               # Configuration file
```

## 🚀 Development Workflow

### Dual-Mode Router System
**Development Mode** (反射驅動)
```bash
go run cmd/server/main.go  # 即時路由發現，快速迭代
```
- Runtime reflection-based routing
- Instant code changes without restart
- Enhanced DI with auto-injection

**Production Mode** (程式碼生成)
```bash
gortex generate routes    # 生成靜態路由 (即將實現)
go build -tags production # 高效能建置
```
- Compile-time route generation
- Zero reflection overhead
- Maximum performance

### Adding a New API Endpoint

1. Create handler file in `internal/handlers/`:
```go
type GameHandler struct {
    Logger  *zap.Logger
    GameSvc *services.GameService
}

func (h *GameHandler) GET(c echo.Context) error {
    // Handler logic
    return response.Success(c, http.StatusOK, data)
}
```

2. Register in `HandlersManager`:
```go
type HandlersManager struct {
    Game *GameHandler `url:"/game"`
}
```

3. Initialize in `main.go`:
```go
handlersManager := &handlers.HandlersManager{
    Game: &handlers.GameHandler{},
}
```

### WebSocket Integration

WebSocket handlers use the same declarative pattern:
```go
type WSHandler struct {
    Hub *hub.Hub
}

// In HandlersManager:
WebSocket *WSHandler `url:"/ws" hijack:"ws"`
```

## 🔧 Production Requirements

### Production-Ready Middleware Stack
- **Authentication**: JWT validation for `/api/*` routes
- **Observability**: ✅ ImprovedCollector with JSON metrics endpoint
- **Resilience**: Rate limiting (⚠️ memory leak fix needed), graceful shutdown
- **Logging**: Structured logging with Zap
- **Health Checks**: ⚠️ Race condition fixes needed

### Current Status & Recent Optimizations
```
✅ COMPLETED (2025/07/21)
- High-performance metrics: ImprovedCollector (25%+ faster)
- Memory leak fixes: Eliminated unbounded growth in SimpleCollector  
- External dependency removal: Zero Redis/Jaeger/Prometheus requirements
- Documentation cleanup: Streamlined to 3 core MD files

🚧 NEXT PRIORITIES
- Production router code generation (eliminate reflection)
- Health checker race condition fixes
- Rate limiter memory leak resolution
- WebSocket hub concurrency simplification
```

### Performance Targets
- **Latency**: <10ms p95 for simple endpoints
- **Throughput**: >10k RPS on standard hardware  
- **Memory**: Stable usage under load
- **CPU**: <50% utilization at target RPS

## Configuration

Uses Builder Pattern for flexible configuration loading:
```go
cfg := config.NewConfigBuilder().
    LoadYamlFile("config.yaml").
    LoadEnvironmentVariables("VIBECORE").
    Validate().
    MustBuild()
```

## 📝 Development Standards

### Code Conventions
```go
// ✅ Good: Declarative routing
type UserHandler struct {
    Logger *zap.Logger
    UserSvc *services.UserService
} `url:"/users"`

// ✅ Good: Standard HTTP methods
func (h *UserHandler) GET(c echo.Context) error {
    return response.Success(c, http.StatusOK, users)
}

// ✅ Good: Custom sub-paths  
func (h *UserHandler) Profile(c echo.Context) error { } // → /users/profile
```

### Best Practices Checklist
- [ ] **Error Handling**: Use `response.Error()` for consistency
- [ ] **Validation**: DTOs with `validate` tags
- [ ] **Logging**: Request-scoped logger from context
- [ ] **Testing**: Unit tests with `httptest` for all handlers
- [ ] **Security**: Input sanitization, JWT validation
- [ ] **Performance**: Avoid reflection in hot paths

### Critical Don'ts ❌
- **No Global State**: Except in `main.go`
- **No Mixed Concerns**: Keep HTTP/WebSocket handlers separate  
- **No Hardcoded Values**: Use configuration
- **No Context Ignoring**: Always handle cancellation
- **No Unvalidated Input**: Validate all user data

## 📚 Project Memory & Context

### 🎯 Framework Positioning
**Gortex** is positioned as a **self-contained, lightweight** game server framework with **zero external service dependencies**. This differentiates it from heavy enterprise solutions requiring Redis, Jaeger, Prometheus infrastructure.

### 🔍 Recent Major Discoveries (2025/07/21)
1. **External Dependency Analysis**: Comprehensive code scan revealed framework is completely self-contained
   - ❌ No Redis, Jaeger, Prometheus, MongoDB, Elasticsearch usage
   - ✅ Only 12 core Go libraries (Echo, Zap, JWT, WebSocket, etc.)
   - ✅ PostgreSQL config exists but unused (potential future integration)

2. **Performance Critical Issues Fixed**:
   - ✅ SimpleCollector disaster: Global write locks blocking ALL HTTP requests  
   - ✅ Unbounded memory growth: Infinite slice appending fixed
   - ✅ ImprovedCollector: 163ns/op vs 217ns/op (25%+ faster, zero allocations)

3. **Documentation Streamlining**:
   - ✅ Consolidated to 3 core files: README.md, CLAUDE.md, OPTIMIZATION_ROADMAP.md
   - ✅ Removed CHANGELOG.md, MIGRATION.md 
   - ✅ Cleaned binary artifacts (observability-example, simple-example)

### 🚨 Known Critical Issues
1. **Health Checker Race Conditions**: `go test -race` detects concurrency issues
2. **Rate Limiter Memory Leak**: Cleanup routine exists but not implemented  
3. **Router Reflection Overhead**: 10-50x performance penalty in production
4. **WebSocket Hub Complexity**: Unnecessary RWMutex alongside channels

### 💡 Development Philosophy
- **Convention over Configuration**: Minimal setup, struct-tag routing
- **Self-Containment over Dependencies**: Built-in implementations preferred
- **Performance over Features**: Optimize hot paths, eliminate bottlenecks  
- **Developer Experience**: Fast iteration in dev, maximum performance in prod

### 🎮 Target Use Cases
- **Real-time game servers**: WebSocket-heavy applications
- **Microservices**: Lightweight, fast-starting services
- **Development prototypes**: Rapid iteration without infrastructure
- **Edge computing**: Minimal resource footprint

### 🔄 Commit Strategy
Each optimization commit should include:
1. **Tests**: Comprehensive unit tests + benchmarks
2. **Examples**: Update affected examples to ensure they work
3. **Documentation**: Update README.md status + performance metrics
4. **Verification**: `go test ./...` and example execution

## 🔗 Related Documentation

- **[Optimization Roadmap](./OPTIMIZATION_ROADMAP.md)**: Prioritized development plan with verified issues
- **[README](./README.md)**: Project overview reflecting latest optimizations  
- **Examples**: `/examples` directory (all verified working as of 2025/07/21)

---

**Last Updated**: 2025/07/21 | **Framework Status**: Alpha (Optimized, Self-Contained) | **Go**: 1.24
