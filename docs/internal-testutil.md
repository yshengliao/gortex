# Test Utilities

This directory contains shared test utilities used across the Gortex framework.

## Directory Structure

- `mock/` - Mock implementations of interfaces for testing
- `fixture/` - Test data fixtures and configurations  
- `assert/` - Custom assertion functions

## Usage

### Mock Objects

```go
import "github.com/yshengliao/gortex/internal/testutil/mock"

// Use mock logger
logger := mock.NewLogger()

// Use mock context
ctx := mock.NewContext()
```

### Test Fixtures

```go
import "github.com/yshengliao/gortex/internal/testutil/fixture"

// Load test configuration
cfg := fixture.TestConfig()

// Get sample request data
data := fixture.SampleRequest()
```

### Custom Assertions

```go
import "github.com/yshengliao/gortex/internal/testutil/assert"

// Assert JSON response
assert.JSONResponse(t, rec, expected)

// Assert error type
assert.ErrorType(t, err, expectedType)
```