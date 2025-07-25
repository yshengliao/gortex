# Simple Example - Struct Tag Routing

This example demonstrates Gortex's core feature: **declarative routing with struct tags**.

## Key Features Demonstrated

### 1. Struct Tag Routing
```go
type HandlersManager struct {
    Home    *HomeHandler    `url:"/"`
    Health  *HealthHandler  `url:"/health"`
    User    *UserHandler    `url:"/users/:id"`
}
```

No manual route registration needed! The framework automatically discovers routes from struct tags.

### 2. Dynamic Parameters
```go
// Route: /users/:id
func (h *UserHandler) GET(c echo.Context) error {
    id := c.Param("id")
    return c.JSON(200, map[string]interface{}{
        "id": id,
        "name": "User " + id,
    })
}
```

### 3. Nested Groups
```go
type HandlersManager struct {
    API *APIGroup `url:"/api"`
}

type APIGroup struct {
    V1 *APIv1Group `url:"/v1"`
    V2 *APIv2Group `url:"/v2"`
}

// Creates routes like:
// - /api/v1/users/:id
// - /api/v2/users/:id
```

### 4. Method-Based Routes
- Standard HTTP methods (GET, POST, PUT, DELETE) become route handlers
- Custom method names create sub-routes (e.g., `Profile()` → `/users/:id/profile`)

## Running the Example

```bash
go run main.go
```

## Testing Routes

```bash
# Home route
curl http://localhost:8080/

# Health check
curl http://localhost:8080/health

# User endpoints
curl http://localhost:8080/users/123
curl -X POST http://localhost:8080/users/123
curl -X POST http://localhost:8080/users/123/profile

# Static files (wildcard)
curl http://localhost:8080/static/css/style.css

# API v1
curl http://localhost:8080/api/v1/users/456
curl http://localhost:8080/api/v1/products/789

# API v2
curl http://localhost:8080/api/v2/users/456
```

## Development Mode Features

With `Logger.Level = "debug"`, you get:
- `GET /_routes` - List all registered routes
- `GET /_monitor` - System monitoring dashboard
- Request/response logging

## Why Struct Tags?

1. **Zero Boilerplate** - No manual route registration
2. **Type Safety** - Compile-time verification
3. **Self-Documenting** - Routes visible in struct definition
4. **Refactoring Friendly** - Rename handlers without breaking routes
5. **IDE Support** - Full autocomplete and navigation

This is the simplest way to build web applications in Go!