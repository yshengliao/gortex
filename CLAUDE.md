# CLAUDE.md - VibeCore Framework Memory

This file provides guidance to Claude Code (claude.ai/code) when working with the VibeCore game server framework.

## Project Overview

**VibeCore** is a Go 1.24 backend framework for game servers that combines HTTP and WebSocket functionality with declarative routing.

### Core Facts
- **Framework**: Echo v4 HTTP server
- **Routing**: Declarative routing via `HandlersManager` struct-tags (`url:"/path"`, `hijack:"ws"`)
- **Initialization**: Functional Options pattern (`WithConfig`, `WithLogger`, `WithHandlers`)
- **DI Container**: Lightweight `AppContext` for dependency injection
- **WebSocket**: First-class support with Gorilla WebSocket + Hub/Client pattern

### Key Principles
1. **宣告式優於命令式 (Declarative over Imperative)**: Routes defined via struct tags, not code
2. **約定優於配置 (Convention over Configuration)**: Developers only write handlers
3. **開發者友好 (Developer-friendly)**: Just add handler + tag, run `go run` (dev) or `go generate` (prod)

## Project Structure

```
/stmp (vibecore)
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

## Development Workflow

### Two Router Modes
1. **Development Mode**: Runtime reflection for instant feedback
   - Just run `go run cmd/server/main.go`
   - Routes automatically discovered from struct tags
   - Enhanced dependency injection (v2) with support for Logger, Config, Hub

2. **Production Mode**: Compile-time code generation for performance
   - Run `go generate ./...` to generate static routes
   - Build with `go build -tags production`

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

## Mandatory Middleware Stack

1. **JWT Authentication**: Protected routes under `/api/*`
2. **OTEL Tracing**: Distributed tracing with trace-id injection
3. **Prometheus Metrics**: Exposed at `/metrics`
4. **Graceful Shutdown**: Proper cleanup on SIGTERM/SIGINT
5. **Structured Logging**: Zap with trace-id correlation

## Development Phases

- **Alpha**: ✅ Core HTTP, Echo startup, reflection routing, 80%+ test coverage
- **Beta**: WebSocket, validation, graceful shutdown
- **RC**: Observability (OTEL + Prometheus), rate limiting
- **1.0**: CLI scaffolding, code generation, multi-tenancy

## Configuration

Uses Builder Pattern for flexible configuration loading:
```go
cfg := config.NewConfigBuilder().
    LoadYamlFile("config.yaml").
    LoadEnvironmentVariables("VIBECORE").
    Validate().
    MustBuild()
```

## Best Practices

1. **Error Handling**: Always use `response.Error()` for consistent error responses
2. **Validation**: DTOs with `validate` tags using go-playground/validator
3. **Logging**: Use request-scoped logger from context, not global
4. **Testing**: Each handler should have unit tests with `httptest`
5. **Security**: JWT validation, input sanitization, rate limiting

## Key Conventions

- Handler methods match HTTP verbs: `GET`, `POST`, `PUT`, `DELETE`
- Custom methods become sub-paths: `Ping()` → `/ping`
- All handlers receive dependencies via DI, not globals
- WebSocket connections managed centrally by Hub
- Configuration via YAML with environment override support

## Do's and Don'ts

**DO:**
- Use struct tags for routing declaration
- Keep handlers thin, business logic in services
- Use functional options for configuration
- Implement graceful shutdown
- Add comprehensive error handling

**DON'T:**
- Use global variables (except in main.go)
- Mix HTTP and WebSocket logic in same handler
- Skip validation on user input
- Ignore context cancellation
- Hardcode configuration values

---
**Last Updated**: 2025/07/21 | **Framework Version**: Alpha (Optimized) | **Go Version**: 1.24