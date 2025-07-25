# Auth Example - JWT Authentication with Middleware

This example demonstrates authentication using struct tag routing with middleware support.

## Key Features

### 1. Middleware via Struct Tags
```go
type HandlersManager struct {
    // Public endpoints
    Home  *HomeHandler  `url:"/"`
    Auth  *AuthHandler  `url:"/auth"`
    
    // Protected endpoints with middleware
    User  *UserHandler  `url:"/user" middleware:"auth"`
    Admin *AdminGroup   `url:"/admin" middleware:"auth,admin"`
}
```

The `middleware` tag automatically applies middleware to routes and their sub-routes.

### 2. Nested Protected Routes
```go
type AdminGroup struct {
    Dashboard *DashboardHandler `url:"/dashboard"`
    Users     *UsersHandler     `url:"/users/:id"`
}
```

All routes under `/admin` inherit the `auth,admin` middleware requirements.

### 3. JWT Token Generation
```go
token, err := jwtService.GenerateAccessToken(
    userID,
    username,
    email,
    role,
)
```

## Running the Example

```bash
go run main.go
```

## Testing Authentication Flow

1. **Get welcome message**:
```bash
curl http://localhost:8081/
```

2. **Login to get JWT token**:
```bash
curl -X POST http://localhost:8081/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "secret"}'

# Response:
{
  "token": "eyJhbGc...",
  "user": {
    "id": "user-123",
    "username": "admin",
    "role": "admin"
  }
}
```

3. **Access protected endpoint** (requires token):
```bash
# This would normally require Authorization header
curl http://localhost:8081/user/profile \
  -H "Authorization: Bearer <token>"
```

4. **Access admin endpoints** (requires admin role):
```bash
curl http://localhost:8081/admin/dashboard \
  -H "Authorization: Bearer <token>"

curl http://localhost:8081/admin/users/456 \
  -H "Authorization: Bearer <token>"
```

## Middleware Execution

The framework processes middleware in this order:

1. Global middleware (if any)
2. Route-specific middleware from tags
3. Handler execution

## Production Considerations

1. **JWT Secret**: Use environment variable, not hardcoded
2. **Token Storage**: Store tokens securely on client
3. **Refresh Tokens**: Implement token refresh mechanism
4. **Role Management**: Store roles in database
5. **Rate Limiting**: Add rate limiting to login endpoint

This example shows how Gortex makes authentication simple with declarative middleware!