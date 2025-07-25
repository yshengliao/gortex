# Gortex Examples

These examples demonstrate the core features of Gortex framework, focusing on **struct tag routing** and **group functionality**.

## 📂 Examples Overview

### 1. [Simple](./simple) - Struct Tag Routing
The foundation of Gortex - declarative routing with struct tags.

**Key Features:**
- ✅ Basic routing with `url` tags
- ✅ Dynamic parameters (`:id`)
- ✅ Wildcard routes (`*`)
- ✅ Nested route groups
- ✅ Method-based routing

```go
type HandlersManager struct {
    Home  *HomeHandler  `url:"/"`
    User  *UserHandler  `url:"/users/:id"`
    API   *APIGroup     `url:"/api"`
}
```

### 2. [Auth](./auth) - JWT Authentication
Demonstrates middleware integration via struct tags.

**Key Features:**
- ✅ Middleware via `middleware` tag
- ✅ JWT token generation
- ✅ Protected route groups
- ✅ Role-based access control

```go
type HandlersManager struct {
    Auth  *AuthHandler  `url:"/auth"`
    User  *UserHandler  `url:"/user" middleware:"auth"`
    Admin *AdminGroup   `url:"/admin" middleware:"auth,admin"`
}
```

### 3. [WebSocket](./websocket) - Real-time Communication
Shows WebSocket support with the `hijack` tag.

**Key Features:**
- ✅ WebSocket routes with `hijack:"ws"`
- ✅ Built-in hub pattern
- ✅ Message broadcasting
- ✅ Graceful shutdown

```go
type HandlersManager struct {
    Home *HomeHandler `url:"/"`
    WS   *WSHandler   `url:"/ws" hijack:"ws"`
}
```

## 🚀 Running Examples

Each example runs on a different port:

```bash
# Simple example (port 8080)
cd simple && go run main.go

# Auth example (port 8081)
cd auth && go run main.go

# WebSocket example (port 8082)
cd websocket && go run main.go
```

## 🎯 Why Struct Tags?

Traditional routing:
```go
// Manual route registration 😫
e.GET("/", homeHandler)
e.GET("/users/:id", userHandler)
e.GET("/api/v1/users", apiV1UserHandler)
e.GET("/api/v2/users", apiV2UserHandler)
```

Gortex routing:
```go
// Automatic discovery from struct tags! 🎉
type HandlersManager struct {
    Home *HomeHandler `url:"/"`
    User *UserHandler `url:"/users/:id"`
    API  *APIGroup    `url:"/api"`
}
```

## 📚 Key Concepts

### 1. Handler Groups
Create nested route structures naturally:
```go
type APIGroup struct {
    V1 *V1Group `url:"/v1"`
    V2 *V2Group `url:"/v2"`
}
```

### 2. HTTP Methods
Method names map to HTTP verbs:
- `GET()` → GET request
- `POST()` → POST request  
- `Profile()` → POST /users/:id/profile

### 3. Special Tags
- `url:"/path"` - Route path
- `middleware:"auth,cors"` - Apply middleware
- `hijack:"ws"` - Protocol hijacking (WebSocket)

## 🔧 Development Features

With `Logger.Level = "debug"`, all examples provide:
- `GET /_routes` - List all routes
- `GET /_monitor` - System metrics
- Request/response logging

## 📖 Learn More

1. Start with the [Simple](./simple) example
2. Add authentication with [Auth](./auth)
3. Build real-time features with [WebSocket](./websocket)
4. Check the main [README](../README.md) for framework details

Happy coding with Gortex! 🚀