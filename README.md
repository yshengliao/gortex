# Gortex - High-Performance Go Web Framework

[![Go Version](https://img.shields.io/badge/go-1.24+-blue.svg)](https://go.dev/)
[![Version](https://img.shields.io/badge/version-v0.1.10-green.svg)](https://github.com/yshengliao/gortex/releases)
[![License](https://img.shields.io/badge/license-MIT-brightgreen.svg)](LICENSE)

Gortex (Go + Vortex) is a high-performance web framework that creates a powerful vortex of connectivity between HTTP and WebSocket protocols. Like a vortex that seamlessly pulls and integrates different streams, Gortex unifies request handling through an elegant tag-based routing system, enabling developers to build real-time applications with the speed and efficiency of Go.

## Why Gortex?

The name combines "Go" with "Vortex", representing the framework's ability to create a swirling convergence of different communication protocols into a unified, powerful flow. Just as a vortex in nature creates a dynamic, self-organizing system, Gortex provides a self-organizing architecture through declarative routing and automatic handler registration.

## Version History

- **v0.1.10** (2025-07-21) - Production Release
  - CLI tool (`stmp`) for project scaffolding and code generation
  - Optimized codebase with improved thread safety
  - Complete observability implementation (metrics, tracing, health checks)
  - Enhanced documentation and examples
  
- **v0.1.0-rc.1** (2025-07-20) - Release Candidate
  - Added observability features (metrics, distributed tracing)
  - Implemented flexible rate limiting strategies
  - Health check system with periodic monitoring
  - Performance optimizations

- **v0.1.0-beta.2** (2025-07-19) - Beta Release
  - Service layer with dependency injection
  - JWT authentication with role-based access control
  - Request validation using go-playground/validator
  - WebSocket hub improvements

- **v0.1.0-beta.1** (2025-07-18) - Beta Release
  - WebSocket support with hub/client pattern
  - Configuration system (Bofry/config compatible)
  - Enhanced routing with automatic registration
  - Comprehensive test coverage

- **v0.1.0-alpha.2** (2025-07-17) - Alpha Release
  - Declarative routing via struct tags
  - Echo v4 integration
  - Basic middleware support
  - Initial documentation

- **v0.1.0-alpha.1** (2025-07-16) - Initial Alpha
  - Project initialization
  - Basic framework structure
  - Core routing concepts

## Features

- 🚀 **Declarative Routing** - Define routes using struct tags
- 🔌 **WebSocket Support** - Built-in hub for connection management
- 🔐 **JWT Authentication** - Complete auth middleware with role-based access
- ✅ **Validation** - Request validation using go-playground/validator
- 📊 **Observability** - Metrics, tracing, and health checks
- 🚦 **Rate Limiting** - Flexible rate limiting strategies
- ⚙️ **Configuration** - Bofry/config compatible configuration system
- 💉 **Dependency Injection** - Simple DI container with generics

## Installation

### Framework

```bash
go get github.com/yshengliao/gortex@v0.1.10
```

### CLI Tool

```bash
go install github.com/yshengliao/gortex/cmd/gortex@v0.1.10
```

Or build from source:

```bash
git clone https://github.com/yshengliao/gortex.git
cd gortex
go build -o gortex ./cmd/gortex
```

## Quick Start

```go
package main

import (
    "github.com/labstack/echo/v4"
    "github.com/yshengliao/gortex/app"
)

// Define handlers with struct tags
type Handlers struct {
    Hello  *HelloHandler  `url:"/hello"`
    Health *HealthHandler `url:"/health"`
}

type HelloHandler struct{}

func (h *HelloHandler) GET(c echo.Context) error {
    return c.JSON(200, map[string]string{"message": "Hello, World!"})
}

type HealthHandler struct{}

func (h *HealthHandler) GET(c echo.Context) error {
    return c.JSON(200, map[string]string{"status": "healthy"})
}

func main() {
    // Create and run application
    application, _ := app.NewApp(
        app.WithHandlers(&Handlers{
            Hello:  &HelloHandler{},
            Health: &HealthHandler{},
        }),
    )
    
    application.Run() // Starts on :8080 by default
}
```

## Core Concepts

### 1. Declarative Routing

Define routes using struct tags instead of manual registration:

```go
type HandlersManager struct {
    Users     *UserHandler      `url:"/users"`
    Auth      *AuthHandler      `url:"/auth"`
    WebSocket *WebSocketHandler `url:"/ws" hijack:"ws"`
}

// HTTP methods become handler methods
func (h *UserHandler) GET(c echo.Context) error     { /* list users */ }
func (h *UserHandler) POST(c echo.Context) error    { /* create user */ }
func (h *UserHandler) DELETE(c echo.Context) error  { /* delete user */ }

// Custom methods become sub-routes (POST by default)
func (h *AuthHandler) Login(c echo.Context) error   { /* POST /auth/login */ }
func (h *AuthHandler) Logout(c echo.Context) error  { /* POST /auth/logout */ }
```

### 2. WebSocket Support

```go
type WebSocketHandler struct {
    Hub *hub.Hub
}

func (h *WebSocketHandler) HandleConnection(c echo.Context) error {
    conn, _ := app.DefaultUpgrader.Upgrade(c.Response(), c.Request(), nil)
    client := hub.NewClient(h.Hub, conn, userID, logger)
    h.Hub.Register <- client
    
    go client.WritePump()
    go client.ReadPump()
    
    return nil
}

// Usage
wsHub := hub.NewHub(logger)
go wsHub.Run()

// Broadcast to all clients
wsHub.Broadcast(&hub.Message{
    Type: "notification",
    Data: map[string]interface{}{"message": "Hello everyone!"},
})
```

### 3. JWT Authentication

```go
// Setup
jwtService := auth.NewJWTService(secretKey, accessTTL, refreshTTL, issuer)

// Generate tokens
token, _ := jwtService.GenerateAccessToken(userID, username, email, role)

// Protect routes
protected := e.Group("/api")
protected.Use(auth.Middleware(jwtService))

// Role-based access
admin := e.Group("/admin")
admin.Use(auth.Middleware(jwtService))
admin.Use(auth.RequireRole("admin"))

// Get user info in handlers
username := auth.GetUsername(c)
userID := auth.GetUserID(c)
```

### 4. Validation

```go
type CreateUserRequest struct {
    Username string `json:"username" validate:"required,min=3,max=30,username"`
    Email    string `json:"email" validate:"required,email"`
    Password string `json:"password" validate:"required,min=8"`
}

func (h *UserHandler) Create(c echo.Context) error {
    var req CreateUserRequest
    if err := validation.BindAndValidate(c, &req); err != nil {
        return err // Returns 400 with validation errors
    }
    // Process valid request...
}
```

### 5. Configuration

```go
// Load configuration
loader := config.NewSimpleLoader().
    WithYAMLFile("config.yaml").
    WithEnvPrefix("STMP_")

cfg := &config.Config{}
loader.Load(cfg)

// Use with app
app.NewApp(
    app.WithConfig(cfg),
    app.WithLogger(logger),
)

// config.yaml example
server:
  address: ":8080"
  gzip: true
jwt:
  secret_key: ${GORTEX_JWT_SECRET_KEY}
  issuer: "my-app"
```

### 6. Observability

```go
// Metrics
collector := observability.NewSimpleCollector()
e.Use(observability.MetricsMiddleware(collector))

// Tracing
tracer := observability.NewSimpleTracer()
e.Use(observability.TracingMiddleware(tracer))

// Health checks
checker := observability.NewHealthChecker(30*time.Second, 5*time.Second)
checker.Register("database", func(ctx context.Context) observability.HealthCheckResult {
    if err := db.Ping(); err != nil {
        return observability.HealthCheckResult{
            Status: observability.HealthStatusUnhealthy,
            Message: err.Error(),
        }
    }
    return observability.HealthCheckResult{
        Status: observability.HealthStatusHealthy,
    }
})

// Rate limiting
e.Use(middleware.RateLimitByIP(100, 200)) // 100 req/sec, burst 200
```

## Complete Example

```go
package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "time"

    "github.com/labstack/echo/v4"
    "go.uber.org/zap"
    "github.com/yshengliao/gortex/app"
    "github.com/yshengliao/gortex/auth"
    "github.com/yshengliao/gortex/config"
    "github.com/yshengliao/gortex/hub"
    "github.com/yshengliao/gortex/middleware"
    "github.com/yshengliao/gortex/observability"
)

type Handlers struct {
    API       *APIHandler       `url:"/api"`
    WebSocket *WebSocketHandler `url:"/ws" hijack:"ws"`
    Health    *HealthHandler    `url:"/health"`
}

func main() {
    // Setup
    logger, _ := zap.NewDevelopment()
    cfg := config.DefaultConfig()
    wsHub := hub.NewHub(logger)
    jwtService := auth.NewJWTService("secret", time.Hour, 7*24*time.Hour, "app")
    
    // Observability
    collector := observability.NewSimpleCollector()
    tracer := observability.NewSimpleTracer()
    
    // Create app
    application, _ := app.NewApp(
        app.WithConfig(cfg),
        app.WithLogger(logger),
        app.WithHandlers(&Handlers{
            API:       &APIHandler{Logger: logger},
            WebSocket: &WebSocketHandler{Hub: wsHub},
            Health:    &HealthHandler{},
        }),
    )
    
    e := application.Echo()
    
    // Middleware
    e.Use(observability.MetricsMiddleware(collector))
    e.Use(observability.TracingMiddleware(tracer))
    e.Use(middleware.RateLimitByIP(100, 200))
    
    // Protected routes
    api := e.Group("/api")
    api.Use(auth.Middleware(jwtService))
    
    // Start services
    go wsHub.Run()
    
    // Graceful shutdown
    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
    defer stop()
    
    go func() {
        if err := application.Run(); err != nil {
            log.Fatal(err)
        }
    }()
    
    <-ctx.Done()
    application.Shutdown(context.Background())
}
```

## CLI Usage

### Create a New Project

```bash
# Initialize a new project
gortex init myapp

# With example handlers and services
gortex init myapp --with-examples
```

### Development Server

```bash
# Run with hot reload
gortex server

# Custom port
gortex server --port 3000
```

### Code Generation

```bash
# Generate a new HTTP handler
gortex generate handler user --methods GET,POST,PUT,DELETE

# Generate a WebSocket handler
gortex generate handler chat --type websocket

# Generate a service with interface and implementation
gortex generate service user

# Generate a model
gortex generate model user --fields username:string,email:string,active:bool
```

## Examples

See the `examples/` directory for complete examples:

- [simple](examples/simple) - Basic HTTP server with WebSocket
- [auth](examples/auth) - JWT authentication
- [config](examples/config) - Configuration management
- [observability](examples/observability) - Metrics, tracing, and rate limiting

## Testing

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run specific package tests
go test ./app
go test ./auth
go test ./hub
```

## Production Considerations

### Configuration

For production, use [Bofry/config](https://github.com/Bofry/config):

```go
import "github.com/Bofry/config"

cfg := &config.Config{}
config.NewConfigurationService(cfg).
    LoadYamlFile("config.yaml").
    LoadEnvironmentVariables("GORTEX").
    LoadDotEnv(".env")
```

### Observability

Integrate with monitoring systems:

```go
// Prometheus metrics
type PrometheusCollector struct {
    httpDuration *prometheus.HistogramVec
}

func (p *PrometheusCollector) RecordHTTPRequest(method, path string, status int, duration time.Duration) {
    p.httpDuration.WithLabelValues(method, path, fmt.Sprint(status)).Observe(duration.Seconds())
}

// Use custom collector
e.Use(observability.MetricsMiddleware(&PrometheusCollector{...}))
```

### Security

- Always use HTTPS in production
- Store secrets in environment variables
- Enable CORS only for trusted origins
- Use strong JWT secrets
- Implement proper rate limiting
- Validate all user inputs

## API Reference

### Quick Reference

```go
// Application Setup
import "github.com/yshengliao/gortex/app"

app, _ := app.NewApp(
    app.WithConfig(cfg),
    app.WithLogger(logger),
    app.WithHandlers(handlers),
)
app.Run()
```

### Package Structure

- `app/` - Core application framework
- `auth/` - JWT authentication
- `hub/` - WebSocket hub and client management
- `validation/` - Request validation
- `config/` - Configuration management
- `observability/` - Metrics, tracing, and health checks
- `middleware/` - HTTP middleware (rate limiting, etc.)
- `services/` - Service interfaces
- `response/` - HTTP response helpers

### Custom Validators

```go
// Register custom validators
validation.RegisterCustomValidators(v)

// Available custom validators:
- gameid    // 3-20 lowercase alphanumeric
- currency  // USD, EUR, GBP, JPY, CNY, TWD
- username  // 3-30 chars, alphanumeric with _ or -
```

### Error Handling

```go
return response.BadRequest(c, "Invalid input")
return response.Unauthorized(c, "Login required")
return response.InternalServerError(c, "Something went wrong")
```

## Changelog

### [v0.1.10] - 2025-07-21

#### Added

- CLI tool (`gortex`) for project scaffolding and code generation
- Code generation templates for common patterns
- Public `RegisterClient()` method for hub client registration
- Thread-safe metrics collection with mutex protection
- HTTP and memory health check implementations
- Configuration validation for required fields

#### Changed

- Hub's `Register` channel is now private, use `RegisterClient()` method instead
- Simplified middleware setup logic in app initialization
- Improved test coverage with proper mocking

#### Fixed

- Compilation errors in examples
- Unused variable warnings
- Missing imports in code generation templates

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## Acknowledgments

### Development

This framework was designed and developed entirely using [Claude Code](https://claude.ai/code) - Anthropic's AI-powered development assistant. The entire codebase, from initial concept to production-ready implementation, was created through collaborative AI-assisted development.

### Design Philosophy

Gortex follows the development practices and architectural patterns established by the [Bofry team](https://github.com/Bofry), specifically:

- **Declarative Configuration**: Using struct tags for routing definition, similar to Bofry's attribute-based patterns
- **Configuration Management**: Built to be compatible with [Bofry/config](https://github.com/Bofry/config) for production deployments
- **Service-Oriented Architecture**: Clear separation of concerns with dedicated service layers
- **Dependency Injection**: Lightweight DI container inspired by Bofry's component management
- **Observability First**: Built-in metrics, tracing, and health checks following Bofry's production standards

The framework aims to provide a Go-native experience while maintaining the rapid development philosophy that Bofry team members are accustomed to from their .NET ecosystem.

### Special Thanks

- The Bofry team for their innovative development patterns and practices
- The Echo framework community for the excellent HTTP router
- The Go community for the amazing ecosystem of packages

## License

MIT License - see [LICENSE](LICENSE) file for details

---

<p align="center">
  Built with ❤️ using <a href="https://claude.ai/code">Claude Code</a><br>
  Inspired by <a href="https://github.com/Bofry">Bofry</a> development practices
</p>
