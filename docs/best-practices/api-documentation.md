# API Documentation Best Practices

## Overview

Well-documented APIs are essential for developer experience. Gortex provides built-in support for automatic API documentation generation using struct tags and OpenAPI/Swagger specifications. This guide covers best practices for documenting your APIs effectively.

## Table of Contents

1. [Struct Tag Design](#struct-tag-design)
2. [Documentation Versioning](#documentation-versioning)
3. [Custom Theme and Branding](#custom-theme-and-branding)
4. [API Grouping and Tagging](#api-grouping-and-tagging)
5. [CI/CD Integration](#cicd-integration)
6. [Documentation as Code](#documentation-as-code)
7. [Advanced Documentation Features](#advanced-documentation-features)
8. [Troubleshooting](#troubleshooting)

## Struct Tag Design

### Basic Documentation Tags

Gortex uses struct tags to generate API documentation automatically:

```go
// Example 1: Comprehensive struct tag usage
type HandlersManager struct {
    // Basic API information
    Users  *UserHandler  `url:"/users" api:"group=User Management,version=v1,desc=User CRUD operations"`
    Auth   *AuthHandler  `url:"/auth" api:"group=Authentication,version=v1,desc=Authentication endpoints"`
    Orders *OrderHandler `url:"/orders" api:"group=Orders,version=v1,desc=Order management,tags=orders|commerce"`
    
    // Versioned APIs
    V1 *V1Handlers `url:"/api/v1" api:"version=v1,desc=Version 1 API endpoints"`
    V2 *V2Handlers `url:"/api/v2" api:"version=v2,desc=Version 2 API endpoints"`
    
    // Internal APIs (hidden from public docs)
    Admin   *AdminHandler   `url:"/admin" api:"group=Admin,internal=true"`
    Metrics *MetricsHandler `url:"/metrics" api:"internal=true,desc=Prometheus metrics"`
}

// Handler method documentation
type UserHandler struct{}

// GetUser retrieves a user by ID
// @Summary Get user details
// @Description Retrieve detailed user information by user ID
// @Tags users
// @Accept json
// @Produce json
// @Param id path string true "User ID" example(user_12345)
// @Success 200 {object} User "User details"
// @Failure 404 {object} ErrorResponse "User not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /users/{id} [get]
func (h *UserHandler) GetUser(c httpctx.Context) error {
    // Implementation
}
```

### Request/Response Documentation

Document your request and response structures thoroughly:

```go
// Example 2: Detailed model documentation
// User represents a system user
// @Description User account information
type User struct {
    // Unique identifier
    ID string `json:"id" example:"user_12345" description:"User's unique identifier"`
    
    // Basic information
    Username string `json:"username" example:"johndoe" validate:"required,min=3,max=20" description:"Unique username"`
    Email    string `json:"email" example:"john@example.com" validate:"required,email" description:"User's email address"`
    
    // Profile details
    Profile UserProfile `json:"profile" description:"User's profile information"`
    
    // Account status
    Status string `json:"status" enum:"active,inactive,suspended" example:"active" description:"Account status"`
    
    // Timestamps
    CreatedAt time.Time `json:"created_at" example:"2024-01-15T10:30:00Z" description:"Account creation timestamp"`
    UpdatedAt time.Time `json:"updated_at" example:"2024-01-20T15:45:00Z" description:"Last update timestamp"`
    
    // Relationships
    Roles []string `json:"roles" example:"[\"user\",\"admin\"]" description:"User's assigned roles"`
}

// UserProfile contains user profile details
type UserProfile struct {
    FirstName   string    `json:"first_name" example:"John" description:"User's first name"`
    LastName    string    `json:"last_name" example:"Doe" description:"User's last name"`
    DateOfBirth *time.Time `json:"date_of_birth,omitempty" example:"1990-01-15" description:"User's date of birth"`
    Phone       string    `json:"phone,omitempty" example:"+1-555-123-4567" validate:"omitempty,e164" description:"Phone number in E.164 format"`
    Address     *Address  `json:"address,omitempty" description:"User's address"`
}

// CreateUserRequest represents user creation payload
// @Description Request payload for creating a new user
type CreateUserRequest struct {
    Username string `json:"username" validate:"required,min=3,max=20" example:"johndoe" description:"Desired username"`
    Email    string `json:"email" validate:"required,email" example:"john@example.com" description:"User's email address"`
    Password string `json:"password" validate:"required,min=8" example:"SecurePass123!" description:"Password (min 8 characters)"`
    Profile  UserProfile `json:"profile" description:"Optional profile information"`
}

// ErrorResponse represents an API error
// @Description Standard error response format
type ErrorResponse struct {
    Success bool        `json:"success" example:"false" description:"Always false for errors"`
    Error   ErrorDetail `json:"error" description:"Error details"`
    TraceID string      `json:"trace_id,omitempty" example:"abc123xyz" description:"Request trace ID for debugging"`
}
```

### Advanced Tag Features

```go
// Example 3: Advanced documentation features
type PaymentHandler struct{}

// ProcessPayment handles payment processing
// @Summary Process a payment
// @Description Process a payment transaction with various payment methods
// @Tags payments
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token" default(Bearer <token>)
// @Param X-Idempotency-Key header string true "Idempotency key for safe retries"
// @Param payment body ProcessPaymentRequest true "Payment details"
// @Success 200 {object} PaymentResponse "Payment processed successfully"
// @Success 202 {object} PaymentResponse "Payment accepted for processing"
// @Failure 400 {object} ErrorResponse "Invalid request"
// @Failure 402 {object} ErrorResponse "Payment required"
// @Failure 409 {object} ErrorResponse "Duplicate payment (idempotency conflict)"
// @Security BearerAuth
// @x-code-samples
// @x-code-samples-language shell
// @x-code-samples-label cURL
// @x-code-samples-source curl -X POST https://api.example.com/payments \
//   -H "Authorization: Bearer <token>" \
//   -H "Content-Type: application/json" \
//   -d '{"amount": 100.50, "currency": "USD", "method": "card"}'
func (h *PaymentHandler) ProcessPayment(c httpctx.Context) error {
    // Implementation
}

// ProcessPaymentRequest with extensive validation documentation
type ProcessPaymentRequest struct {
    // Payment amount in minor units (cents)
    Amount int64 `json:"amount" validate:"required,min=1" example:"10050" description:"Amount in cents (10050 = $100.50)"`
    
    // ISO 4217 currency code
    Currency string `json:"currency" validate:"required,iso4217" example:"USD" description:"3-letter ISO currency code"`
    
    // Payment method details
    Method PaymentMethod `json:"method" validate:"required" description:"Payment method details"`
    
    // Optional metadata
    Metadata map[string]string `json:"metadata,omitempty" example:"{\"order_id\":\"ord_123\"}" description:"Additional metadata"`
}

// PaymentMethod with discriminated union pattern
// @Description Payment method details (card, bank_transfer, or wallet)
// @discriminator type
type PaymentMethod struct {
    Type string `json:"type" validate:"required,oneof=card bank_transfer wallet" example:"card" description:"Payment method type"`
    
    // Card payment details (when type=card)
    Card *CardDetails `json:"card,omitempty" description:"Credit/debit card details"`
    
    // Bank transfer details (when type=bank_transfer)
    BankTransfer *BankTransferDetails `json:"bank_transfer,omitempty" description:"Bank transfer details"`
    
    // Digital wallet details (when type=wallet)
    Wallet *WalletDetails `json:"wallet,omitempty" description:"Digital wallet details"`
}
```

## Documentation Versioning

### Version Management Strategy

```go
// Example 4: API versioning patterns
type V1Handlers struct {
    Users  *v1.UserHandler  `url:"/users" api:"group=Users,desc=User management (v1)"`
    Orders *v1.OrderHandler `url:"/orders" api:"group=Orders,desc=Order management (v1)"`
}

type V2Handlers struct {
    Users  *v2.UserHandler  `url:"/users" api:"group=Users,desc=User management (v2)"`
    Orders *v2.OrderHandler `url:"/orders" api:"group=Orders,desc=Order management (v2)"`
    // New in v2
    Payments *v2.PaymentHandler `url:"/payments" api:"group=Payments,desc=Payment processing"`
}

// Version-specific documentation
func setupVersionedDocs(app *app.App) {
    // V1 documentation
    v1Provider := swagger.NewSwaggerProvider(
        swagger.WithTitle("API v1"),
        swagger.WithVersion("1.0.0"),
        swagger.WithBasePath("/api/v1"),
        swagger.WithDescription("Legacy API - will be deprecated on 2025-01-01"),
    )
    
    // V2 documentation
    v2Provider := swagger.NewSwaggerProvider(
        swagger.WithTitle("API v2"),
        swagger.WithVersion("2.0.0"),
        swagger.WithBasePath("/api/v2"),
        swagger.WithDescription("Current API version with enhanced features"),
    )
    
    // Register both versions
    app.Router().GET("/docs/v1", v1Provider.UIHandler())
    app.Router().GET("/docs/v2", v2Provider.UIHandler())
    
    // Redirect latest to v2
    app.Router().GET("/docs", func(c httpctx.Context) error {
        return c.Redirect(302, "/docs/v2")
    })
}
```

### Deprecation Documentation

```go
// Example 5: Documenting deprecated endpoints
type UserHandler struct{}

// GetUserLegacy retrieves user data (deprecated)
// @Summary Get user (deprecated)
// @Description This endpoint is deprecated. Use GET /api/v2/users/{id} instead.
// @Tags users,deprecated
// @Deprecated true
// @Param id path string true "User ID"
// @Success 200 {object} LegacyUser
// @Header 200 {string} X-Deprecated "true"
// @Header 200 {string} X-Sunset-Date "2025-01-01"
// @Header 200 {string} Link "</api/v2/users/{id}>; rel=\"successor-version\""
func (h *UserHandler) GetUserLegacy(c httpctx.Context) error {
    c.Response().Header().Set("X-Deprecated", "true")
    c.Response().Header().Set("X-Sunset-Date", "2025-01-01")
    c.Response().Header().Set("Link", "</api/v2/users/"+c.Param("id")+">; rel=\"successor-version\"")
    
    // Return with deprecation warning
    return c.JSON(200, map[string]interface{}{
        "_deprecation": map[string]string{
            "message": "This endpoint is deprecated and will be removed on 2025-01-01",
            "successor": "/api/v2/users/{id}",
        },
        "data": legacyUserData,
    })
}
```

## Custom Theme and Branding

### Creating Custom Swagger UI Theme

```go
// Example 6: Custom documentation UI
package docs

import (
    "embed"
    "html/template"
)

//go:embed custom-theme/*
var customTheme embed.FS

type CustomSwaggerUI struct {
    title       string
    logoURL     string
    faviconURL  string
    cssOverride string
}

func (ui *CustomSwaggerUI) Handler() http.HandlerFunc {
    tmpl := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>{{ .Title }}</title>
    <link rel="icon" type="image/png" href="{{ .FaviconURL }}">
    <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@4/swagger-ui.css">
    <style>
        /* Custom branding */
        .swagger-ui .topbar {
            background-color: #2c3e50;
        }
        .swagger-ui .topbar .wrapper {
            padding: 10px 0;
        }
        .swagger-ui .topbar-wrapper img {
            content: url('{{ .LogoURL }}');
            height: 40px;
        }
        .swagger-ui .info .title {
            color: #2c3e50;
        }
        /* Custom color scheme */
        .swagger-ui .btn.authorize {
            background-color: #3498db;
        }
        .swagger-ui .btn.execute {
            background-color: #27ae60;
        }
        {{ .CSSOverride }}
    </style>
</head>
<body>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@4/swagger-ui-bundle.js"></script>
    <script>
    window.onload = function() {
        SwaggerUIBundle({
            url: "/docs/swagger.json",
            dom_id: '#swagger-ui',
            deepLinking: true,
            presets: [
                SwaggerUIBundle.presets.apis,
                SwaggerUIBundle.SwaggerUIStandalonePreset
            ],
            plugins: [
                SwaggerUIBundle.plugins.DownloadUrl
            ],
            layout: "StandaloneLayout",
            defaultModelsExpandDepth: 1,
            defaultModelExpandDepth: 1,
            docExpansion: "list",
            filter: true,
            showExtensions: true,
            showCommonExtensions: true,
            customOptions: {
                // Add custom options
            }
        });
    };
    </script>
</body>
</html>`
    
    return func(w http.ResponseWriter, r *http.Request) {
        t := template.Must(template.New("swagger").Parse(tmpl))
        t.Execute(w, ui)
    }
}
```

### Custom Documentation Renderer

```go
// Example 7: Alternative documentation format (ReDoc)
func setupReDoc(app *app.App) {
    app.Router().GET("/docs/redoc", func(c httpctx.Context) error {
        html := `<!DOCTYPE html>
<html>
<head>
    <title>API Documentation</title>
    <meta charset="utf-8"/>
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        body {
            margin: 0;
            padding: 0;
        }
        #redoc-container {
            height: 100vh;
        }
    </style>
</head>
<body>
    <div id="redoc-container"></div>
    <script src="https://cdn.jsdelivr.net/npm/redoc/bundles/redoc.standalone.js"></script>
    <script>
        Redoc.init('/docs/swagger.json', {
            theme: {
                colors: {
                    primary: {
                        main: '#2c3e50'
                    }
                },
                typography: {
                    fontSize: '16px',
                    headings: {
                        fontFamily: 'Roboto, sans-serif'
                    }
                }
            },
            scrollYOffset: 50,
            hideDownloadButton: false,
            disableSearch: false,
            nativeScrollbars: false,
            expandResponses: "200,201",
            requiredPropsFirst: true,
            sortPropsAlphabetically: true
        }, document.getElementById('redoc-container'));
    </script>
</body>
</html>`
        
        c.Response().Header().Set("Content-Type", "text/html; charset=utf-8")
        return c.String(200, html)
    })
}
```

## API Grouping and Tagging

### Organizing APIs Logically

```go
// Example 8: Effective API organization
type APIStructure struct {
    // Public APIs
    Public PublicAPIs `url:"/api" api:"group=Public APIs,desc=Publicly accessible endpoints"`
    
    // Partner APIs (require API key)
    Partner PartnerAPIs `url:"/partner" api:"group=Partner APIs,desc=Partner integration endpoints"`
    
    // Internal APIs (require special auth)
    Internal InternalAPIs `url:"/internal" api:"group=Internal APIs,desc=Internal service endpoints,internal=true"`
    
    // Webhooks
    Webhooks WebhookHandlers `url:"/webhooks" api:"group=Webhooks,desc=Webhook endpoints for external services"`
}

type PublicAPIs struct {
    Auth     *AuthHandler     `url:"/auth" api:"tags=authentication"`
    Users    *UserHandler     `url:"/users" api:"tags=users"`
    Products *ProductHandler  `url:"/products" api:"tags=products|catalog"`
    Search   *SearchHandler   `url:"/search" api:"tags=search"`
}

// Tag descriptions for better organization
func configureDocTags(provider *swagger.SwaggerProvider) {
    provider.AddTag("authentication", "User authentication and authorization")
    provider.AddTag("users", "User account management")
    provider.AddTag("products", "Product catalog operations")
    provider.AddTag("catalog", "Product catalog browsing")
    provider.AddTag("search", "Search functionality across resources")
    provider.AddTag("orders", "Order processing and management")
    provider.AddTag("payments", "Payment processing")
    provider.AddTag("webhooks", "Webhook event notifications")
}
```

### Dynamic Tag Generation

```go
// Example 9: Dynamic documentation based on features
type FeatureAwareDocProvider struct {
    features map[string]bool
}

func (p *FeatureAwareDocProvider) GenerateDocs(routes []RouteInfo) (*OpenAPI, error) {
    spec := &OpenAPI{
        OpenAPI: "3.0.0",
        Info: Info{
            Title:   "Dynamic API",
            Version: "1.0.0",
        },
        Paths: make(map[string]*PathItem),
    }
    
    for _, route := range routes {
        // Skip disabled features
        feature := extractFeature(route.Handler)
        if !p.features[feature] {
            continue
        }
        
        // Add to documentation
        p.addRoute(spec, route)
    }
    
    return spec, nil
}
```

## CI/CD Integration

### Automated Documentation Generation

```yaml
# Example 10: GitHub Actions workflow for docs
name: API Documentation

on:
  push:
    branches: [main, develop]
    paths:
      - '**.go'
      - 'api/openapi.yaml'

jobs:
  generate-docs:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version-file: 'go.mod'
      
      - name: Generate OpenAPI spec
        run: |
          go run ./cmd/docgen -output openapi.json
      
      - name: Validate OpenAPI spec
        run: |
          npx @apidevtools/swagger-cli validate openapi.json
      
      - name: Generate SDK
        run: |
          npx @openapitools/openapi-generator-cli generate \
            -i openapi.json \
            -g go \
            -o ./sdk/go \
            --additional-properties=packageName=gortexsdk
      
      - name: Update documentation site
        run: |
          # Generate static docs
          npx @redocly/openapi-cli build-docs openapi.json \
            -o docs/index.html
          
          # Deploy to GitHub Pages or other platform
          # ...
```

### Documentation Testing

```go
// Example 11: Testing API documentation accuracy
func TestAPIDocumentation(t *testing.T) {
    app := createTestApp()
    
    // Get OpenAPI spec
    req := httptest.NewRequest("GET", "/docs/swagger.json", nil)
    rec := httptest.NewRecorder()
    app.ServeHTTP(rec, req)
    
    assert.Equal(t, 200, rec.Code)
    
    var spec OpenAPI
    err := json.Unmarshal(rec.Body.Bytes(), &spec)
    assert.NoError(t, err)
    
    // Validate all documented endpoints exist
    for path, pathItem := range spec.Paths {
        for method := range pathItem.Operations() {
            // Test that endpoint exists
            testReq := httptest.NewRequest(method, path, nil)
            testRec := httptest.NewRecorder()
            app.ServeHTTP(testRec, testReq)
            
            // Should not be 404
            assert.NotEqual(t, 404, testRec.Code, 
                "Documented endpoint %s %s does not exist", method, path)
        }
    }
}

// Contract testing
func TestAPIContract(t *testing.T) {
    // Load contract from OpenAPI spec
    contract, err := LoadContract("openapi.json")
    assert.NoError(t, err)
    
    // Test each endpoint against contract
    for _, endpoint := range contract.Endpoints {
        t.Run(endpoint.Name, func(t *testing.T) {
            // Generate request from contract
            req := endpoint.GenerateValidRequest()
            
            // Execute request
            resp, err := client.Do(req)
            assert.NoError(t, err)
            
            // Validate response against contract
            err = endpoint.ValidateResponse(resp)
            assert.NoError(t, err)
        })
    }
}
```

## Documentation as Code

### Inline Documentation

```go
// Example 12: Documentation co-located with code
package handlers

// Package handlers implements HTTP request handlers.
//
// # Authentication
//
// Most endpoints require authentication via Bearer token:
//   Authorization: Bearer <token>
//
// # Rate Limiting
//
// API requests are rate limited per IP:
//   - Anonymous: 100 requests per hour
//   - Authenticated: 1000 requests per hour
//
// # Errors
//
// All errors follow a standard format:
//   {
//     "success": false,
//     "error": {
//       "code": 400,
//       "message": "Validation failed",
//       "details": {}
//     }
//   }
package handlers

// UserHandler handles user-related operations.
//
// # Endpoints
//
//   - GET    /users      - List users
//   - POST   /users      - Create user
//   - GET    /users/{id} - Get user
//   - PUT    /users/{id} - Update user
//   - DELETE /users/{id} - Delete user
//
// # Permissions
//
//   - List/Get: Requires authentication
//   - Create: Requires admin role
//   - Update: Requires admin role or own user
//   - Delete: Requires admin role
type UserHandler struct {
    db     Database
    cache  Cache
    logger *zap.Logger
}
```

### Generated Documentation

```go
// Example 13: Auto-generating docs from code
//go:generate go run github.com/swaggo/swag/cmd/swag init -g main.go -o ./docs

package main

// @title Gortex API
// @version 1.0
// @description Production-ready API built with Gortex framework
// @termsOfService https://example.com/terms

// @contact.name API Support
// @contact.url https://example.com/support
// @contact.email api@example.com

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host api.example.com
// @BasePath /v1

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Bearer token authentication

// @x-extension-openapi {"example": "value"}
func main() {
    // Application setup
}
```

## Advanced Documentation Features

### Interactive Examples

```go
// Example 14: Providing interactive examples
type ExampleProvider struct {
    examples map[string][]Example
}

type Example struct {
    Name        string
    Description string
    Request     ExampleRequest
    Response    ExampleResponse
}

func (p *ExampleProvider) AddEndpointExamples(endpoint string) {
    p.examples[endpoint] = []Example{
        {
            Name:        "Success Case",
            Description: "Successful user creation",
            Request: ExampleRequest{
                Method: "POST",
                Path:   "/users",
                Headers: map[string]string{
                    "Authorization": "Bearer eyJ...",
                    "Content-Type":  "application/json",
                },
                Body: `{
                    "username": "johndoe",
                    "email": "john@example.com",
                    "password": "SecurePass123!"
                }`,
            },
            Response: ExampleResponse{
                Status: 201,
                Headers: map[string]string{
                    "Location": "/users/user_12345",
                },
                Body: `{
                    "id": "user_12345",
                    "username": "johndoe",
                    "email": "john@example.com",
                    "created_at": "2024-01-15T10:30:00Z"
                }`,
            },
        },
        {
            Name:        "Validation Error",
            Description: "Invalid email format",
            Request: ExampleRequest{
                Body: `{"username": "john", "email": "invalid-email"}`,
            },
            Response: ExampleResponse{
                Status: 400,
                Body: `{
                    "success": false,
                    "error": {
                        "code": 400,
                        "message": "Validation failed",
                        "details": {
                            "email": "must be a valid email address"
                        }
                    }
                }`,
            },
        },
    }
}
```

### API Playground

```go
// Example 15: Embedded API playground
func setupAPIPlayground(app *app.App) {
    app.Router().GET("/playground", func(c httpctx.Context) error {
        html := `<!DOCTYPE html>
<html>
<head>
    <title>API Playground</title>
    <style>
        /* Playground styles */
    </style>
</head>
<body>
    <div id="playground">
        <h1>API Playground</h1>
        <select id="endpoint-selector">
            <option value="">Select an endpoint...</option>
        </select>
        
        <div id="request-builder">
            <h2>Request</h2>
            <input type="text" id="method" placeholder="GET" />
            <input type="text" id="path" placeholder="/api/users" />
            <textarea id="headers" placeholder="Headers (JSON)"></textarea>
            <textarea id="body" placeholder="Request body (JSON)"></textarea>
            <button onclick="sendRequest()">Send Request</button>
        </div>
        
        <div id="response-viewer">
            <h2>Response</h2>
            <pre id="response-content"></pre>
        </div>
    </div>
    
    <script>
        // Playground implementation
        async function sendRequest() {
            const method = document.getElementById('method').value;
            const path = document.getElementById('path').value;
            const headers = JSON.parse(document.getElementById('headers').value || '{}');
            const body = document.getElementById('body').value;
            
            try {
                const response = await fetch(path, {
                    method: method,
                    headers: headers,
                    body: body || undefined
                });
                
                const responseText = await response.text();
                document.getElementById('response-content').textContent = 
                    'Status: ' + response.status + '\n\n' + responseText;
            } catch (error) {
                document.getElementById('response-content').textContent = 
                    'Error: ' + error.message;
            }
        }
    </script>
</body>
</html>`
        
        return c.HTML(200, html)
    })
}
```

## Troubleshooting

### Common Issues

1. **Missing Documentation**
   ```go
   // Ensure all handlers are properly tagged
   func validateDocumentation(app *app.App) error {
       routes := app.Routes()
       undocumented := []string{}
       
       for _, route := range routes {
           if !route.HasDocumentation() {
               undocumented = append(undocumented, route.Path)
           }
       }
       
       if len(undocumented) > 0 {
           return fmt.Errorf("undocumented routes: %v", undocumented)
       }
       return nil
   }
   ```

2. **Incorrect Examples**
   ```go
   // Validate examples match schemas
   func validateExamples(spec *OpenAPI) error {
       for path, pathItem := range spec.Paths {
           for method, operation := range pathItem.Operations() {
               if err := validateOperationExamples(operation); err != nil {
                   return fmt.Errorf("%s %s: %w", method, path, err)
               }
           }
       }
       return nil
   }
   ```

3. **Version Conflicts**
   ```go
   // Check for version compatibility
   func checkVersionCompatibility(v1Spec, v2Spec *OpenAPI) []BreakingChange {
       changes := []BreakingChange{}
       
       // Check removed endpoints
       for path, v1Path := range v1Spec.Paths {
           if _, exists := v2Spec.Paths[path]; !exists {
               changes = append(changes, BreakingChange{
                   Type: "endpoint_removed",
                   Path: path,
               })
           }
       }
       
       // Check changed schemas
       // ... implementation
       
       return changes
   }
   ```

### Documentation Quality Checklist

```go
// Example 16: Documentation linting
type DocLinter struct {
    rules []LintRule
}

func (l *DocLinter) LintEndpoint(endpoint EndpointDoc) []LintIssue {
    issues := []LintIssue{}
    
    // Check summary exists and is concise
    if len(endpoint.Summary) == 0 {
        issues = append(issues, LintIssue{
            Severity: "error",
            Message:  "Missing summary",
        })
    } else if len(endpoint.Summary) > 80 {
        issues = append(issues, LintIssue{
            Severity: "warning",
            Message:  "Summary too long (>80 chars)",
        })
    }
    
    // Check description
    if len(endpoint.Description) < 20 {
        issues = append(issues, LintIssue{
            Severity: "warning",
            Message:  "Description too brief",
        })
    }
    
    // Check examples
    if len(endpoint.Examples) == 0 {
        issues = append(issues, LintIssue{
            Severity: "warning",
            Message:  "No examples provided",
        })
    }
    
    // Check error responses
    if !endpoint.HasErrorResponse(400) {
        issues = append(issues, LintIssue{
            Severity: "info",
            Message:  "No 400 error response documented",
        })
    }
    
    return issues
}
```

## Summary

Effective API documentation requires:

1. **Consistent Structure**: Use struct tags and annotations consistently
2. **Comprehensive Examples**: Provide examples for both success and error cases
3. **Version Management**: Handle multiple API versions gracefully
4. **Custom Branding**: Match documentation to your brand
5. **CI/CD Integration**: Automate documentation generation and validation
6. **Interactive Features**: Provide playgrounds and testing tools
7. **Quality Control**: Lint and validate documentation regularly

Remember:
- Document as you code, not after
- Keep examples up to date
- Version your API thoughtfully
- Make documentation easily discoverable
- Test your documentation like code