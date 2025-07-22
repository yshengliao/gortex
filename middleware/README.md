# Gortex Middleware

This package provides common middleware for the Gortex framework.

## Error Handler Middleware

The error handler middleware ensures all errors are returned in a consistent format throughout your application.

### Features

- Converts all errors to a standardized JSON format
- Automatically adds request IDs to error responses
- Handles Echo HTTPError and converts to framework error codes
- Configurable error detail hiding for production environments
- Comprehensive error logging with structured fields

### Usage

```go
import (
    "github.com/yshengliao/gortex/middleware"
    "go.uber.org/zap"
)

// Basic usage with defaults
e.Use(middleware.ErrorHandler())

// Custom configuration
errorConfig := &middleware.ErrorHandlerConfig{
    Logger: logger,
    HideInternalServerErrorDetails: true,  // Hide sensitive errors in production
    DefaultMessage: "An internal error occurred",
}
e.Use(middleware.ErrorHandlerWithConfig(errorConfig))
```

### Error Response Format

All errors are returned in this consistent format:

```json
{
    "success": false,
    "error": {
        "code": 1001,
        "message": "Invalid input provided",
        "details": {
            "field": "email",
            "error": "invalid format"
        }
    },
    "timestamp": "2025-07-22T08:00:00Z",
    "request_id": "abc123"
}
```

### Integration with pkg/errors

The middleware works seamlessly with the `pkg/errors` package:

```go
// Return a validation error
return errors.ValidationError(c, "Invalid input", map[string]interface{}{
    "field": "email",
    "error": "must be a valid email address",
})

// Return a not found error
return errors.NotFoundError(c, "User")

// Return a custom error with details
return errors.New(errors.CodeInsufficientBalance, "Insufficient funds").
    WithDetail("current_balance", 100).
    WithDetail("required_amount", 150).
    Send(c, http.StatusPaymentRequired)
```

### Error Code Mapping

Echo HTTP errors are automatically mapped to framework error codes:

- 400 Bad Request → CodeInvalidInput
- 401 Unauthorized → CodeUnauthorized
- 403 Forbidden → CodeForbidden
- 404 Not Found → CodeResourceNotFound
- 429 Too Many Requests → CodeRateLimitExceeded
- 500 Internal Server Error → CodeInternalServerError
- And many more...

### Configuration Options

- `Logger`: Zap logger for structured error logging
- `HideInternalServerErrorDetails`: When true, hides actual error messages for standard Go errors (recommended for production)
- `DefaultMessage`: Message shown when hiding internal error details

## Rate Limit Middleware

See the existing rate limit middleware documentation for details on request rate limiting.