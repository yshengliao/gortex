# Development Mode Example

This example demonstrates the development mode features of the Gortex framework.

## Features Demonstrated

1. **Development Routes**
   - `/_routes` - Lists all registered routes
   - `/_error` - Test error responses
   - `/_config` - View configuration

2. **Request/Response Logging**
   - All requests and responses are logged in detail
   - Request bodies are logged (up to 10KB)
   - Response bodies are logged
   - Sensitive headers are masked

3. **Development Error Pages**
   - HTML error pages with stack traces
   - Request details shown
   - Different rendering for browser vs API clients

## Running the Example

```bash
go run main.go
```

## Testing Development Features

### List All Routes
```bash
curl http://localhost:8080/_routes | jq
```

### Test Request/Response Logging
```bash
# Watch the console output to see detailed logging
curl -X POST http://localhost:8080/api/users \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer secret-token" \
  -d '{"test": "data"}'
```

### Test Error Pages

#### In Terminal (JSON Response)
```bash
# Internal server error
curl -X POST http://localhost:8080/example/error | jq

# Validation error
curl -X POST http://localhost:8080/example/validation | jq

# Panic recovery
curl -X POST http://localhost:8080/example/panic | jq
```

#### In Browser (HTML Error Page)
Open these URLs in your browser to see the development error pages:
- http://localhost:8080/example/error
- http://localhost:8080/example/validation
- http://localhost:8080/_error?type=panic

### Test Slow Endpoints
```bash
# Default 2s delay
curl -X POST http://localhost:8080/example/slow

# Custom delay
curl -X POST "http://localhost:8080/example/slow?duration=5s"
```

## Development Mode Configuration

Development mode is enabled when the logger level is set to "debug":

```go
cfg.Logger.Level = "debug"
```

This automatically enables:
- Development routes (/_routes, /_error, /_config)
- Request/response logging middleware
- Development error pages with stack traces
- Detailed error responses

## Security Note

Development mode features expose sensitive information and should **NEVER** be enabled in production. Always ensure `Logger.Level` is set to "info" or higher in production environments.