# Gortex Error Response System

The Gortex error response system provides a standardized way to handle and return errors in your API, ensuring consistency across all endpoints.

## Features

- **Standardized error structure** with code, message, details, timestamp, and request ID
- **Error code categories**: Validation (1xxx), Auth (2xxx), System (3xxx), Business (4xxx)
- **Pre-defined error codes** for common scenarios
- **Helper functions** for quick error responses
- **Request ID tracking** for debugging
- **Backward compatibility** with existing response package
- **High performance**: ~59ns per error creation with minimal allocations

## Error Response Structure

```json
{
  "success": false,
  "error": {
    "code": 1001,
    "message": "Invalid input provided",
    "details": {
      "field": "email",
      "error": "Invalid email format"
    }
  },
  "timestamp": "2025-01-22T10:30:00Z",
  "request_id": "req-123-abc",
  "meta": {
    "version": "1.0"
  }
}
```

## Error Code Categories

### Validation Errors (1xxx)
- `1000` - Validation failed
- `1001` - Invalid input
- `1002` - Missing required field
- `1003` - Invalid format
- `1004` - Value out of range
- `1005` - Duplicate value
- `1006` - Invalid length
- `1007` - Invalid type
- `1008` - Invalid JSON
- `1009` - Invalid query parameter

### Authentication/Authorization Errors (2xxx)
- `2000` - Unauthorized
- `2001` - Invalid credentials
- `2002` - Token expired
- `2003` - Token invalid
- `2004` - Token missing
- `2005` - Forbidden
- `2006` - Insufficient permissions
- `2007` - Account locked
- `2008` - Account not found
- `2009` - Session expired

### System Errors (3xxx)
- `3000` - Internal server error
- `3001` - Database error
- `3002` - Service unavailable
- `3003` - Timeout
- `3004` - Rate limit exceeded
- `3005` - Resource exhausted
- `3006` - Not implemented
- `3007` - Bad gateway
- `3008` - Circuit breaker open
- `3009` - Configuration error

### Business Logic Errors (4xxx)
- `4000` - Business logic error
- `4001` - Resource not found
- `4002` - Resource already exists
- `4003` - Invalid operation
- `4004` - Precondition failed
- `4005` - Conflict
- `4006` - Insufficient balance
- `4007` - Quota exceeded
- `4008` - Invalid state
- `4009` - Dependency failed

## Usage Examples

### Basic Error Response

```go
import "github.com/yshengliao/gortex/pkg/errors"

// Simple validation error
func CreateUser(c echo.Context) error {
    if email == "" {
        return errors.ValidationFieldError(c, "email", "Email is required")
    }
    // ...
}

// Not found error
func GetUser(c echo.Context) error {
    user, err := findUser(id)
    if err != nil {
        return errors.NotFoundError(c, "User")
    }
    // ...
}
```

### Detailed Error Response

```go
// Error with multiple details
func TransferMoney(c echo.Context) error {
    if sender.Balance < amount {
        return errors.New(errors.CodeInsufficientBalance, "Insufficient balance").
            WithDetail("current_balance", sender.Balance).
            WithDetail("requested_amount", amount).
            WithDetail("shortage", amount - sender.Balance).
            Send(c, http.StatusPaymentRequired)
    }
    // ...
}

// Multiple validation errors
func ValidateInput(c echo.Context) error {
    validationErrors := make(map[string]interface{})
    
    if len(password) < 8 {
        validationErrors["password"] = "Password must be at least 8 characters"
    }
    if age < 18 {
        validationErrors["age"] = "Must be at least 18 years old"
    }
    
    if len(validationErrors) > 0 {
        return errors.ValidationError(c, "Validation failed", validationErrors)
    }
    // ...
}
```

### System Errors

```go
// Database error
func GetUserData(c echo.Context) error {
    data, err := db.Query("SELECT * FROM users WHERE id = ?", id)
    if err != nil {
        return errors.DatabaseError(c, "fetch user data")
    }
    // ...
}

// Timeout error
func CallExternalAPI(c echo.Context) error {
    ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
    defer cancel()
    
    resp, err := client.Do(req.WithContext(ctx))
    if err != nil {
        if ctx.Err() == context.DeadlineExceeded {
            return errors.TimeoutError(c, "external API call")
        }
        return errors.InternalServerError(c, err)
    }
    // ...
}

// Rate limiting
func RateLimitedEndpoint(c echo.Context) error {
    if isRateLimited(userID) {
        return errors.RateLimitError(c, 300) // Retry after 5 minutes
    }
    // ...
}
```

### Custom Error Codes

```go
// Using custom error messages
func CustomBusinessLogic(c echo.Context) error {
    err := errors.New(errors.CodeBusinessLogicError, "Custom business error").
        WithDetail("operation", "account_verification").
        WithDetail("reason", "Documents pending review").
        WithMeta(map[string]interface{}{
            "support_ticket": "TICK-123",
            "estimated_time": "24 hours",
        })
    
    return err.Send(c, http.StatusUnprocessableEntity)
}

// Using error code with default message
func QuickError(c echo.Context) error {
    return errors.SendErrorCode(c, errors.CodeResourceNotFound)
}
```

### Error Handling Middleware

```go
func ErrorHandlingMiddleware() echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            err := next(c)
            if err != nil {
                // Check if it's already an ErrorResponse
                if errResp, ok := err.(*errors.ErrorResponse); ok {
                    return errResp.Send(c, http.StatusInternalServerError)
                }
                
                // Check if it's an Echo HTTP error
                if he, ok := err.(*echo.HTTPError); ok {
                    code := errors.CodeInternalServerError
                    switch he.Code {
                    case http.StatusBadRequest:
                        code = errors.CodeInvalidInput
                    case http.StatusUnauthorized:
                        code = errors.CodeUnauthorized
                    case http.StatusNotFound:
                        code = errors.CodeResourceNotFound
                    }
                    return errors.SendError(c, code, he.Message.(string), nil)
                }
                
                // Generic error
                return errors.InternalServerError(c, err)
            }
            return nil
        }
    }
}
```

## Helper Functions

| Function | Description | HTTP Status |
|----------|-------------|-------------|
| `ValidationError()` | Validation errors with details | 400 |
| `ValidationFieldError()` | Single field validation error | 400 |
| `UnauthorizedError()` | Authentication required | 401 |
| `ForbiddenError()` | Access forbidden | 403 |
| `NotFoundError()` | Resource not found | 404 |
| `ConflictError()` | Resource conflict | 409 |
| `TimeoutError()` | Request timeout | 408 |
| `RateLimitError()` | Rate limit exceeded | 429 |
| `DatabaseError()` | Database operation failed | 500 |
| `InternalServerError()` | Generic server error | 500 |

## Migration from Old Response Package

The new error system is backward compatible with the existing response package:

```go
// Old way (still works)
response.BadRequest(c, "Invalid input")
response.NotFound(c, "User not found")

// New way (recommended)
errors.ValidationError(c, "Invalid input", nil)
errors.NotFoundError(c, "User")
```

## Performance

Benchmark results on Apple M1:

```
BenchmarkNewErrorResponse-8           19808899    58.95 ns/op    96 B/op    1 allocs/op
BenchmarkErrorResponseWithDetails-8    9171976   128.7 ns/op   432 B/op    3 allocs/op
BenchmarkValidationError-8              922950  1261 ns/op    1745 B/op   20 allocs/op
BenchmarkSendError-8                   1000000  1024 ns/op    1553 B/op   15 allocs/op
```

## Best Practices

1. **Use specific error codes** instead of generic ones when possible
2. **Include relevant details** to help with debugging
3. **Keep error messages user-friendly** while logging technical details
4. **Use request IDs** for tracing errors across services
5. **Don't expose sensitive information** in error details
6. **Be consistent** with error codes across your API
7. **Document your custom error codes** in API documentation

## Complete Example

See the [examples/errors](../../examples/errors) directory for a complete working example demonstrating all error types and patterns.