package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/yshengliao/gortex/core/app"
	"github.com/yshengliao/gortex/middleware"
	"github.com/yshengliao/gortex/pkg/config"
	httpctx "github.com/yshengliao/gortex/transport/http"
	"github.com/yshengliao/gortex/validation"
	"go.uber.org/zap"
)

// HandlersManager demonstrates advanced API documentation
type HandlersManager struct {
	Home    *HomeHandler    `url:"/"`
	API     *APIGroup       `url:"/api" middleware:"cors"`
	Docs    *DocsHandler    `url:"/docs"`
	OpenAPI *OpenAPIHandler `url:"/openapi.json"`
	Admin   *AdminGroup     `url:"/admin" middleware:"auth"`
}

// APIGroup contains versioned API endpoints
type APIGroup struct {
	V1 *APIv1Group `url:"/v1"`
	V2 *APIv2Group `url:"/v2"`
}

// APIv1Group contains v1 endpoints
type APIv1Group struct {
	Users    *UsersHandlerV1    `url:"/users/:id?"`
	Products *ProductsHandlerV1 `url:"/products/:id?"`
	Orders   *OrdersHandlerV1   `url:"/orders/:id?"`
}

// APIv2Group contains v2 endpoints with breaking changes
type APIv2Group struct {
	Users    *UsersHandlerV2    `url:"/users/:id?"`
	Products *ProductsHandlerV2 `url:"/products/:id?"`
	Orders   *OrdersHandlerV2   `url:"/orders/:id?"`
	Search   *SearchHandlerV2   `url:"/search"`
}

// AdminGroup contains admin endpoints
type AdminGroup struct {
	Dashboard *DashboardHandler `url:"/dashboard"`
	Settings  *SettingsHandler  `url:"/settings"`
	Reports   *ReportsHandler   `url:"/reports/:type"`
}

// OpenAPI structures for documentation
type OpenAPISpec struct {
	OpenAPI    string                 `json:"openapi"`
	Info       OpenAPIInfo            `json:"info"`
	Servers    []OpenAPIServer        `json:"servers"`
	Paths      map[string]PathItem    `json:"paths"`
	Components OpenAPIComponents      `json:"components"`
	Tags       []OpenAPITag           `json:"tags"`
}

type OpenAPIInfo struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Version     string   `json:"version"`
	Contact     Contact  `json:"contact"`
	License     License  `json:"license"`
}

type Contact struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	URL   string `json:"url"`
}

type License struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type OpenAPIServer struct {
	URL         string                    `json:"url"`
	Description string                    `json:"description"`
	Variables   map[string]ServerVariable `json:"variables,omitempty"`
}

type ServerVariable struct {
	Default     string   `json:"default"`
	Description string   `json:"description"`
	Enum        []string `json:"enum,omitempty"`
}

type PathItem struct {
	Get    *Operation `json:"get,omitempty"`
	Post   *Operation `json:"post,omitempty"`
	Put    *Operation `json:"put,omitempty"`
	Delete *Operation `json:"delete,omitempty"`
	Patch  *Operation `json:"patch,omitempty"`
}

type Operation struct {
	Tags        []string              `json:"tags"`
	Summary     string                `json:"summary"`
	Description string                `json:"description"`
	OperationID string                `json:"operationId"`
	Parameters  []Parameter           `json:"parameters,omitempty"`
	RequestBody *RequestBody          `json:"requestBody,omitempty"`
	Responses   map[string]Response   `json:"responses"`
	Security    []map[string][]string `json:"security,omitempty"`
	Deprecated  bool                  `json:"deprecated,omitempty"`
}

type Parameter struct {
	Name        string      `json:"name"`
	In          string      `json:"in"`
	Description string      `json:"description"`
	Required    bool        `json:"required"`
	Schema      SchemaRef   `json:"schema"`
	Example     interface{} `json:"example,omitempty"`
}

type RequestBody struct {
	Description string               `json:"description"`
	Required    bool                 `json:"required"`
	Content     map[string]MediaType `json:"content"`
}

type Response struct {
	Description string               `json:"description"`
	Content     map[string]MediaType `json:"content,omitempty"`
	Headers     map[string]Header    `json:"headers,omitempty"`
}

type MediaType struct {
	Schema   SchemaRef              `json:"schema"`
	Example  interface{}            `json:"example,omitempty"`
	Examples map[string]ExampleRef  `json:"examples,omitempty"`
}

type SchemaRef struct {
	Ref    string  `json:"$ref,omitempty"`
	Type   string  `json:"type,omitempty"`
	Format string  `json:"format,omitempty"`
	Schema *Schema `json:"-"`
}

type Schema struct {
	Type        string                 `json:"type,omitempty"`
	Properties  map[string]SchemaRef   `json:"properties,omitempty"`
	Required    []string               `json:"required,omitempty"`
	Description string                 `json:"description,omitempty"`
	Example     interface{}            `json:"example,omitempty"`
	Items       *SchemaRef             `json:"items,omitempty"`
	Enum        []interface{}          `json:"enum,omitempty"`
	Format      string                 `json:"format,omitempty"`
	Minimum     *float64               `json:"minimum,omitempty"`
	Maximum     *float64               `json:"maximum,omitempty"`
	MinLength   *int                   `json:"minLength,omitempty"`
	MaxLength   *int                   `json:"maxLength,omitempty"`
}

type Header struct {
	Description string    `json:"description"`
	Schema      SchemaRef `json:"schema"`
}

type ExampleRef struct {
	Summary     string      `json:"summary"`
	Description string      `json:"description"`
	Value       interface{} `json:"value"`
}

type OpenAPIComponents struct {
	Schemas         map[string]Schema         `json:"schemas"`
	SecuritySchemes map[string]SecurityScheme `json:"securitySchemes"`
}

type SecurityScheme struct {
	Type         string `json:"type"`
	Description  string `json:"description"`
	Name         string `json:"name,omitempty"`
	In           string `json:"in,omitempty"`
	Scheme       string `json:"scheme,omitempty"`
	BearerFormat string `json:"bearerFormat,omitempty"`
}

type OpenAPITag struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Domain models with validation tags
type User struct {
	ID        string    `json:"id" example:"user_123"`
	Email     string    `json:"email" validate:"required,email" example:"user@example.com"`
	Name      string    `json:"name" validate:"required,min=2,max=100" example:"John Doe"`
	Role      string    `json:"role" validate:"required,oneof=admin user guest" example:"user"`
	Active    bool      `json:"active" example:"true"`
	CreatedAt time.Time `json:"created_at" example:"2024-01-26T10:30:00Z"`
	UpdatedAt time.Time `json:"updated_at" example:"2024-01-26T10:30:00Z"`
}

type Product struct {
	ID          string   `json:"id" example:"prod_456"`
	Name        string   `json:"name" validate:"required,min=2,max=200" example:"Laptop"`
	Description string   `json:"description" validate:"max=1000" example:"High-performance laptop"`
	Price       float64  `json:"price" validate:"required,min=0" example:"999.99"`
	Category    string   `json:"category" validate:"required" example:"electronics"`
	Tags        []string `json:"tags" example:"[\"computer\",\"portable\"]"`
	Stock       int      `json:"stock" validate:"min=0" example:"50"`
	Active      bool     `json:"active" example:"true"`
}

type Order struct {
	ID         string      `json:"id" example:"order_789"`
	UserID     string      `json:"user_id" validate:"required" example:"user_123"`
	Items      []OrderItem `json:"items" validate:"required,min=1,dive"`
	Total      float64     `json:"total" example:"2099.98"`
	Status     string      `json:"status" validate:"required,oneof=pending processing shipped delivered cancelled" example:"pending"`
	PaymentID  string      `json:"payment_id,omitempty" example:"pay_abc123"`
	ShippingID string      `json:"shipping_id,omitempty" example:"ship_xyz789"`
	CreatedAt  time.Time   `json:"created_at" example:"2024-01-26T10:30:00Z"`
}

type OrderItem struct {
	ProductID string  `json:"product_id" validate:"required" example:"prod_456"`
	Quantity  int     `json:"quantity" validate:"required,min=1" example:"2"`
	Price     float64 `json:"price" validate:"required,min=0" example:"999.99"`
}

// Request/Response models
type CreateUserRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Name     string `json:"name" validate:"required,min=2,max=100"`
	Password string `json:"password" validate:"required,min=8,max=72"`
	Role     string `json:"role" validate:"required,oneof=admin user guest"`
}

type UpdateUserRequest struct {
	Email  *string `json:"email,omitempty" validate:"omitempty,email"`
	Name   *string `json:"name,omitempty" validate:"omitempty,min=2,max=100"`
	Role   *string `json:"role,omitempty" validate:"omitempty,oneof=admin user guest"`
	Active *bool   `json:"active,omitempty"`
}

type SearchRequest struct {
	Query    string   `json:"query" validate:"required,min=2,max=100"`
	Type     string   `json:"type" validate:"required,oneof=user product order"`
	Filters  []Filter `json:"filters,omitempty" validate:"dive"`
	Page     int      `json:"page" validate:"min=1" example:"1"`
	PageSize int      `json:"page_size" validate:"min=1,max=100" example:"20"`
}

type Filter struct {
	Field    string `json:"field" validate:"required"`
	Operator string `json:"operator" validate:"required,oneof=eq ne lt gt le ge in contains"`
	Value    string `json:"value" validate:"required"`
}

type SearchResponse struct {
	Results    interface{} `json:"results"`
	TotalCount int         `json:"total_count" example:"150"`
	Page       int         `json:"page" example:"1"`
	PageSize   int         `json:"page_size" example:"20"`
	HasMore    bool        `json:"has_more" example:"true"`
}

type ErrorResponse struct {
	Error   string                 `json:"error" example:"Invalid request"`
	Code    string                 `json:"code" example:"VALIDATION_ERROR"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// Handlers
type HomeHandler struct{}

func (h *HomeHandler) GET(c httpctx.Context) error {
	html := `<!DOCTYPE html>
<html>
<head>
    <title>Advanced API Documentation Example</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        .container { max-width: 1200px; margin: 0 auto; }
        .feature { background: #f5f5f5; padding: 15px; margin: 10px 0; border-radius: 5px; }
        .endpoint { background: #e8f4f8; padding: 10px; margin: 5px 0; }
        .method { font-weight: bold; padding: 2px 8px; border-radius: 3px; color: white; }
        .get { background: #61affe; }
        .post { background: #49cc90; }
        .put { background: #fca130; }
        .delete { background: #f93e3e; }
        code { background: #e0e0e0; padding: 2px 5px; border-radius: 3px; }
    </style>
</head>
<body>
    <div class="container">
        <h1>🚀 Gortex Advanced API Documentation Example</h1>
        
        <div class="feature">
            <h2>📚 Documentation Endpoints</h2>
            <div class="endpoint">
                <strong>Swagger UI:</strong> <a href="/docs">/docs</a> - Interactive API documentation
            </div>
            <div class="endpoint">
                <strong>OpenAPI Spec:</strong> <a href="/openapi.json">/openapi.json</a> - Machine-readable specification
            </div>
            <div class="endpoint">
                <strong>ReDoc:</strong> <a href="/docs?style=redoc">/docs?style=redoc</a> - Alternative documentation UI
            </div>
        </div>
        
        <div class="feature">
            <h2>🔄 API Versions</h2>
            <div class="endpoint">
                <span class="method get">GET</span> <code>/api/v1/users</code> - V1 Users API (deprecated)
            </div>
            <div class="endpoint">
                <span class="method get">GET</span> <code>/api/v2/users</code> - V2 Users API with pagination
            </div>
            <div class="endpoint">
                <span class="method post">POST</span> <code>/api/v2/search</code> - V2 Advanced search (new)
            </div>
        </div>
        
        <div class="feature">
            <h2>🔐 Authentication</h2>
            <div class="endpoint">
                <strong>Bearer Token:</strong> Add <code>Authorization: Bearer YOUR_TOKEN</code> header
            </div>
            <div class="endpoint">
                <strong>API Key:</strong> Add <code>X-API-Key: YOUR_KEY</code> header
            </div>
            <div class="endpoint">
                <strong>Test Token:</strong> Use <code>test-token-123</code> for testing
            </div>
        </div>
        
        <div class="feature">
            <h2>✨ Features Demonstrated</h2>
            <ul>
                <li>OpenAPI 3.0 specification generation</li>
                <li>Multiple API versions with deprecation</li>
                <li>Request/response validation</li>
                <li>Multiple authentication methods</li>
                <li>Rich examples and schemas</li>
                <li>Interactive documentation (Swagger UI & ReDoc)</li>
                <li>Custom themes and branding</li>
                <li>Webhook documentation</li>
                <li>Rate limiting documentation</li>
                <li>Error response standards</li>
            </ul>
        </div>
        
        <div class="feature">
            <h2>📋 Example Requests</h2>
            <div class="endpoint">
                <strong>Create User:</strong>
                <pre><code>curl -X POST http://localhost:8085/api/v2/users \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test-token-123" \
  -d '{
    "email": "user@example.com",
    "name": "John Doe",
    "password": "securepass123",
    "role": "user"
  }'</code></pre>
            </div>
            <div class="endpoint">
                <strong>Advanced Search:</strong>
                <pre><code>curl -X POST http://localhost:8085/api/v2/search \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test-token-123" \
  -d '{
    "query": "laptop",
    "type": "product",
    "filters": [
      {"field": "price", "operator": "lt", "value": "1000"}
    ],
    "page": 1,
    "page_size": 20
  }'</code></pre>
            </div>
        </div>
        
        <div class="feature">
            <h2>🛠️ Tools & Integration</h2>
            <ul>
                <li><strong>Postman Collection:</strong> Import from OpenAPI spec</li>
                <li><strong>Client SDK Generation:</strong> Use openapi-generator</li>
                <li><strong>Mock Server:</strong> Use Prism with the OpenAPI spec</li>
                <li><strong>Contract Testing:</strong> Validate against OpenAPI spec</li>
            </ul>
        </div>
    </div>
</body>
</html>`
	
	return c.HTML(200, html)
}

// DocsHandler serves interactive documentation
type DocsHandler struct{}

func (h *DocsHandler) GET(c httpctx.Context) error {
	style := c.Query("style")
	
	if style == "redoc" {
		// ReDoc UI
		html := `<!DOCTYPE html>
<html>
<head>
    <title>API Documentation - ReDoc</title>
    <meta charset="utf-8"/>
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <link href="https://fonts.googleapis.com/css?family=Montserrat:300,400,700|Roboto:300,400,700" rel="stylesheet">
</head>
<body>
    <redoc spec-url='/openapi.json'></redoc>
    <script src="https://cdn.jsdelivr.net/npm/redoc@next/bundles/redoc.standalone.js"></script>
</body>
</html>`
		return c.HTML(200, html)
	}
	
	// Default Swagger UI
	html := `<!DOCTYPE html>
<html>
<head>
    <title>API Documentation - Swagger UI</title>
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui.css">
    <style>
        body { margin: 0; }
        .swagger-ui .topbar { display: none; }
    </style>
</head>
<body>
    <div id="swagger-ui"></div>
    <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
    <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui-standalone-preset.js"></script>
    <script>
        window.onload = function() {
            window.ui = SwaggerUIBundle({
                url: "/openapi.json",
                dom_id: '#swagger-ui',
                deepLinking: true,
                presets: [
                    SwaggerUIBundle.presets.apis,
                    SwaggerUIStandalonePreset
                ],
                plugins: [
                    SwaggerUIBundle.plugins.DownloadUrl
                ],
                layout: "StandaloneLayout",
                validatorUrl: null,
                tryItOutEnabled: true,
                supportedSubmitMethods: ['get', 'post', 'put', 'delete', 'patch'],
                onComplete: function() {
                    console.log("Swagger UI loaded");
                }
            });
        };
    </script>
</body>
</html>`
	
	return c.HTML(200, html)
}

// OpenAPIHandler generates OpenAPI specification
type OpenAPIHandler struct{}

func (h *OpenAPIHandler) GET(c httpctx.Context) error {
	spec := generateOpenAPISpec()
	c.Response().Header().Set("Content-Type", "application/json")
	return c.JSON(200, spec)
}

// Generate complete OpenAPI specification
func generateOpenAPISpec() OpenAPISpec {
	spec := OpenAPISpec{
		OpenAPI: "3.0.3",
		Info: OpenAPIInfo{
			Title:       "Gortex Advanced API",
			Description: "Advanced API documentation example demonstrating all features of Gortex framework",
			Version:     "2.0.0",
			Contact: Contact{
				Name:  "Gortex Team",
				Email: "support@gortex.io",
				URL:   "https://github.com/yshengliao/gortex",
			},
			License: License{
				Name: "MIT",
				URL:  "https://opensource.org/licenses/MIT",
			},
		},
		Servers: []OpenAPIServer{
			{
				URL:         "http://localhost:8085",
				Description: "Local development server",
			},
			{
				URL:         "https://api.example.com",
				Description: "Production server",
			},
			{
				URL:         "https://{environment}.api.example.com",
				Description: "Environment-based server",
				Variables: map[string]ServerVariable{
					"environment": {
						Default:     "dev",
						Description: "Server environment",
						Enum:        []string{"dev", "staging", "prod"},
					},
				},
			},
		},
		Tags: []OpenAPITag{
			{Name: "Users", Description: "User management endpoints"},
			{Name: "Products", Description: "Product catalog endpoints"},
			{Name: "Orders", Description: "Order processing endpoints"},
			{Name: "Search", Description: "Advanced search functionality"},
			{Name: "Admin", Description: "Administrative endpoints"},
		},
		Paths: generatePaths(),
		Components: OpenAPIComponents{
			Schemas:         generateSchemas(),
			SecuritySchemes: generateSecuritySchemes(),
		},
	}
	
	return spec
}

func generatePaths() map[string]PathItem {
	paths := make(map[string]PathItem)
	
	// V2 Users endpoints
	paths["/api/v2/users"] = PathItem{
		Get: &Operation{
			Tags:        []string{"Users"},
			Summary:     "List users",
			Description: "Get a paginated list of users with optional filtering",
			OperationID: "listUsersV2",
			Parameters: []Parameter{
				{
					Name:        "page",
					In:          "query",
					Description: "Page number",
					Required:    false,
					Schema:      SchemaRef{Type: "integer", Format: "int32"},
					Example:     1,
				},
				{
					Name:        "page_size",
					In:          "query",
					Description: "Items per page",
					Required:    false,
					Schema:      SchemaRef{Type: "integer", Format: "int32"},
					Example:     20,
				},
				{
					Name:        "role",
					In:          "query",
					Description: "Filter by role",
					Required:    false,
					Schema:      SchemaRef{Type: "string"},
					Example:     "user",
				},
			},
			Responses: map[string]Response{
				"200": {
					Description: "Successful response",
					Content: map[string]MediaType{
						"application/json": {
							Schema: SchemaRef{Type: "object"},
							Example: map[string]interface{}{
								"users": []map[string]interface{}{
									{
										"id":         "user_123",
										"email":      "user@example.com",
										"name":       "John Doe",
										"role":       "user",
										"active":     true,
										"created_at": "2024-01-26T10:30:00Z",
									},
								},
								"total_count": 150,
								"page":        1,
								"page_size":   20,
								"has_more":    true,
							},
						},
					},
				},
				"401": {
					Description: "Unauthorized",
					Content: map[string]MediaType{
						"application/json": {
							Schema: SchemaRef{Ref: "#/components/schemas/ErrorResponse"},
						},
					},
				},
			},
			Security: []map[string][]string{
				{"bearerAuth": {}},
				{"apiKey": {}},
			},
		},
		Post: &Operation{
			Tags:        []string{"Users"},
			Summary:     "Create user",
			Description: "Create a new user account",
			OperationID: "createUserV2",
			RequestBody: &RequestBody{
				Description: "User creation request",
				Required:    true,
				Content: map[string]MediaType{
					"application/json": {
						Schema: SchemaRef{Ref: "#/components/schemas/CreateUserRequest"},
					},
				},
			},
			Responses: map[string]Response{
				"201": {
					Description: "User created successfully",
					Headers: map[string]Header{
						"Location": {
							Description: "URL of the created user",
							Schema:      SchemaRef{Type: "string"},
						},
					},
					Content: map[string]MediaType{
						"application/json": {
							Schema: SchemaRef{Ref: "#/components/schemas/User"},
						},
					},
				},
				"400": {
					Description: "Validation error",
					Content: map[string]MediaType{
						"application/json": {
							Schema: SchemaRef{Ref: "#/components/schemas/ErrorResponse"},
						},
					},
				},
			},
			Security: []map[string][]string{
				{"bearerAuth": {}},
			},
		},
	}
	
	// V2 Search endpoint
	paths["/api/v2/search"] = PathItem{
		Post: &Operation{
			Tags:        []string{"Search"},
			Summary:     "Advanced search",
			Description: "Perform advanced search across users, products, and orders",
			OperationID: "advancedSearch",
			RequestBody: &RequestBody{
				Description: "Search request",
				Required:    true,
				Content: map[string]MediaType{
					"application/json": {
						Schema: SchemaRef{Ref: "#/components/schemas/SearchRequest"},
						Examples: map[string]ExampleRef{
							"userSearch": {
								Summary: "Search for users",
								Value: map[string]interface{}{
									"query": "john",
									"type":  "user",
									"page":  1,
									"page_size": 20,
								},
							},
							"productSearch": {
								Summary: "Search for products with filters",
								Value: map[string]interface{}{
									"query": "laptop",
									"type":  "product",
									"filters": []map[string]string{
										{"field": "price", "operator": "lt", "value": "1000"},
										{"field": "category", "operator": "eq", "value": "electronics"},
									},
									"page": 1,
									"page_size": 20,
								},
							},
						},
					},
				},
			},
			Responses: map[string]Response{
				"200": {
					Description: "Search results",
					Content: map[string]MediaType{
						"application/json": {
							Schema: SchemaRef{Ref: "#/components/schemas/SearchResponse"},
						},
					},
				},
			},
			Security: []map[string][]string{
				{"bearerAuth": {}},
			},
		},
	}
	
	// Admin endpoints
	paths["/admin/reports/{type}"] = PathItem{
		Get: &Operation{
			Tags:        []string{"Admin"},
			Summary:     "Generate report",
			Description: "Generate various types of reports",
			OperationID: "generateReport",
			Parameters: []Parameter{
				{
					Name:        "type",
					In:          "path",
					Description: "Report type",
					Required:    true,
					Schema:      SchemaRef{Type: "string"},
					Example:     "sales",
				},
				{
					Name:        "start_date",
					In:          "query",
					Description: "Start date for the report",
					Required:    true,
					Schema:      SchemaRef{Type: "string", Format: "date"},
					Example:     "2024-01-01",
				},
				{
					Name:        "end_date",
					In:          "query",
					Description: "End date for the report",
					Required:    true,
					Schema:      SchemaRef{Type: "string", Format: "date"},
					Example:     "2024-01-31",
				},
			},
			Responses: map[string]Response{
				"200": {
					Description: "Report generated successfully",
					Content: map[string]MediaType{
						"application/json": {
							Schema: SchemaRef{Type: "object"},
						},
						"application/pdf": {
							Schema: SchemaRef{Type: "string", Format: "binary"},
						},
					},
				},
			},
			Security: []map[string][]string{
				{"bearerAuth": {"admin"}},
			},
		},
	}
	
	return paths
}

func generateSchemas() map[string]Schema {
	schemas := make(map[string]Schema)
	
	// User schema
	schemas["User"] = Schema{
		Type: "object",
		Properties: map[string]SchemaRef{
			"id":         {Type: "string", Schema: &Schema{Description: "User ID", Example: "user_123"}},
			"email":      {Type: "string", Schema: &Schema{Format: "email", Description: "User email", Example: "user@example.com"}},
			"name":       {Type: "string", Schema: &Schema{Description: "User name", Example: "John Doe"}},
			"role":       {Type: "string", Schema: &Schema{Description: "User role", Enum: []interface{}{"admin", "user", "guest"}, Example: "user"}},
			"active":     {Type: "boolean", Schema: &Schema{Description: "Whether user is active", Example: true}},
			"created_at": {Type: "string", Schema: &Schema{Format: "date-time", Description: "Creation timestamp", Example: "2024-01-26T10:30:00Z"}},
			"updated_at": {Type: "string", Schema: &Schema{Format: "date-time", Description: "Last update timestamp", Example: "2024-01-26T10:30:00Z"}},
		},
		Required: []string{"id", "email", "name", "role", "active"},
	}
	
	// CreateUserRequest schema
	schemas["CreateUserRequest"] = Schema{
		Type: "object",
		Properties: map[string]SchemaRef{
			"email":    {Type: "string", Schema: &Schema{Format: "email", Description: "User email"}},
			"name":     {Type: "string", Schema: &Schema{Description: "User name", MinLength: intPtr(2), MaxLength: intPtr(100)}},
			"password": {Type: "string", Schema: &Schema{Description: "User password", MinLength: intPtr(8), MaxLength: intPtr(72)}},
			"role":     {Type: "string", Schema: &Schema{Description: "User role", Enum: []interface{}{"admin", "user", "guest"}}},
		},
		Required: []string{"email", "name", "password", "role"},
	}
	
	// SearchRequest schema
	schemas["SearchRequest"] = Schema{
		Type: "object",
		Properties: map[string]SchemaRef{
			"query": {Type: "string", Schema: &Schema{Description: "Search query", MinLength: intPtr(2), MaxLength: intPtr(100)}},
			"type":  {Type: "string", Schema: &Schema{Description: "Search type", Enum: []interface{}{"user", "product", "order"}}},
			"filters": {
				Type: "array",
				Schema: &Schema{
					Description: "Search filters",
					Items: &SchemaRef{
						Type: "object",
						Schema: &Schema{
							Properties: map[string]SchemaRef{
								"field":    {Type: "string"},
								"operator": {Type: "string", Schema: &Schema{Enum: []interface{}{"eq", "ne", "lt", "gt", "le", "ge", "in", "contains"}}},
								"value":    {Type: "string"},
							},
							Required: []string{"field", "operator", "value"},
						},
					},
				},
			},
			"page":      {Type: "integer", Schema: &Schema{Description: "Page number", Minimum: float64Ptr(1)}},
			"page_size": {Type: "integer", Schema: &Schema{Description: "Items per page", Minimum: float64Ptr(1), Maximum: float64Ptr(100)}},
		},
		Required: []string{"query", "type"},
	}
	
	// SearchResponse schema
	schemas["SearchResponse"] = Schema{
		Type: "object",
		Properties: map[string]SchemaRef{
			"results":     {Type: "object", Schema: &Schema{Description: "Search results"}},
			"total_count": {Type: "integer", Schema: &Schema{Description: "Total number of results"}},
			"page":        {Type: "integer", Schema: &Schema{Description: "Current page"}},
			"page_size":   {Type: "integer", Schema: &Schema{Description: "Items per page"}},
			"has_more":    {Type: "boolean", Schema: &Schema{Description: "Whether more results are available"}},
		},
		Required: []string{"results", "total_count", "page", "page_size", "has_more"},
	}
	
	// ErrorResponse schema
	schemas["ErrorResponse"] = Schema{
		Type: "object",
		Properties: map[string]SchemaRef{
			"error":   {Type: "string", Schema: &Schema{Description: "Error message"}},
			"code":    {Type: "string", Schema: &Schema{Description: "Error code"}},
			"details": {Type: "object", Schema: &Schema{Description: "Additional error details"}},
		},
		Required: []string{"error", "code"},
	}
	
	return schemas
}

func generateSecuritySchemes() map[string]SecurityScheme {
	return map[string]SecurityScheme{
		"bearerAuth": {
			Type:         "http",
			Scheme:       "bearer",
			BearerFormat: "JWT",
			Description:  "JWT authentication token",
		},
		"apiKey": {
			Type:        "apiKey",
			In:          "header",
			Name:        "X-API-Key",
			Description: "API key authentication",
		},
	}
}

// V1 Handlers (deprecated)
type UsersHandlerV1 struct{}

func (h *UsersHandlerV1) GET(c httpctx.Context) error {
	c.Response().Header().Set("Deprecation", "true")
	c.Response().Header().Set("Sunset", "2024-12-31")
	
	users := []User{
		{
			ID:        "user_123",
			Email:     "user@example.com",
			Name:      "John Doe",
			Role:      "user",
			Active:    true,
			CreatedAt: time.Now().Add(-24 * time.Hour),
			UpdatedAt: time.Now(),
		},
	}
	
	return c.JSON(200, users)
}

// V2 Handlers
type UsersHandlerV2 struct{}

func (h *UsersHandlerV2) GET(c httpctx.Context) error {
	page := 1
	pageSize := 20
	
	// Mock data
	response := map[string]interface{}{
		"users": []User{
			{
				ID:        "user_123",
				Email:     "user@example.com",
				Name:      "John Doe",
				Role:      "user",
				Active:    true,
				CreatedAt: time.Now().Add(-24 * time.Hour),
				UpdatedAt: time.Now(),
			},
		},
		"total_count": 150,
		"page":        page,
		"page_size":   pageSize,
		"has_more":    true,
	}
	
	return c.JSON(200, response)
}

func (h *UsersHandlerV2) POST(c httpctx.Context) error {
	var req CreateUserRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(400, ErrorResponse{
			Error: "Invalid request body",
			Code:  "INVALID_REQUEST",
		})
	}
	
	// Validate request
	validator := validation.New()
	if err := validator.ValidateStruct(&req); err != nil {
		return c.JSON(400, ErrorResponse{
			Error:   "Validation failed",
			Code:    "VALIDATION_ERROR",
			Details: map[string]interface{}{"errors": err.Error()},
		})
	}
	
	// Create user
	user := User{
		ID:        fmt.Sprintf("user_%d", time.Now().Unix()),
		Email:     req.Email,
		Name:      req.Name,
		Role:      req.Role,
		Active:    true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	c.Response().Header().Set("Location", fmt.Sprintf("/api/v2/users/%s", user.ID))
	return c.JSON(201, user)
}

// ProductsHandlerV1
type ProductsHandlerV1 struct{}

func (h *ProductsHandlerV1) GET(c httpctx.Context) error {
	c.Response().Header().Set("Deprecation", "true")
	return c.JSON(200, []Product{})
}

// ProductsHandlerV2
type ProductsHandlerV2 struct{}

func (h *ProductsHandlerV2) GET(c httpctx.Context) error {
	products := []Product{
		{
			ID:          "prod_456",
			Name:        "Laptop",
			Description: "High-performance laptop",
			Price:       999.99,
			Category:    "electronics",
			Tags:        []string{"computer", "portable"},
			Stock:       50,
			Active:      true,
		},
	}
	
	return c.JSON(200, map[string]interface{}{
		"products":    products,
		"total_count": 100,
	})
}

// OrdersHandlerV1
type OrdersHandlerV1 struct{}

func (h *OrdersHandlerV1) GET(c httpctx.Context) error {
	c.Response().Header().Set("Deprecation", "true")
	return c.JSON(200, []Order{})
}

// OrdersHandlerV2
type OrdersHandlerV2 struct{}

func (h *OrdersHandlerV2) GET(c httpctx.Context) error {
	orders := []Order{
		{
			ID:     "order_789",
			UserID: "user_123",
			Items: []OrderItem{
				{ProductID: "prod_456", Quantity: 2, Price: 999.99},
			},
			Total:     1999.98,
			Status:    "pending",
			CreatedAt: time.Now(),
		},
	}
	
	return c.JSON(200, map[string]interface{}{
		"orders":      orders,
		"total_count": 50,
	})
}

// SearchHandlerV2
type SearchHandlerV2 struct{}

func (h *SearchHandlerV2) POST(c httpctx.Context) error {
	var req SearchRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(400, ErrorResponse{
			Error: "Invalid request body",
			Code:  "INVALID_REQUEST",
		})
	}
	
	// Mock search results
	var results interface{}
	switch req.Type {
	case "user":
		results = []User{{
			ID:     "user_123",
			Email:  "user@example.com",
			Name:   "John Doe",
			Role:   "user",
			Active: true,
		}}
	case "product":
		results = []Product{{
			ID:       "prod_456",
			Name:     "Laptop",
			Price:    999.99,
			Category: "electronics",
			Stock:    50,
			Active:   true,
		}}
	case "order":
		results = []Order{{
			ID:     "order_789",
			UserID: "user_123",
			Total:  1999.98,
			Status: "pending",
		}}
	}
	
	response := SearchResponse{
		Results:    results,
		TotalCount: 150,
		Page:       req.Page,
		PageSize:   req.PageSize,
		HasMore:    true,
	}
	
	return c.JSON(200, response)
}

// Admin handlers
type DashboardHandler struct{}

func (h *DashboardHandler) GET(c httpctx.Context) error {
	return c.JSON(200, map[string]interface{}{
		"total_users":   1234,
		"total_orders":  5678,
		"total_revenue": 123456.78,
		"active_users":  890,
	})
}

type SettingsHandler struct{}

func (h *SettingsHandler) GET(c httpctx.Context) error {
	return c.JSON(200, map[string]interface{}{
		"api_version":     "2.0.0",
		"rate_limit":      "1000 req/hour",
		"webhook_enabled": true,
		"features": map[string]bool{
			"search_v2":        true,
			"batch_operations": false,
			"webhooks":         true,
		},
	})
}

type ReportsHandler struct{}

func (h *ReportsHandler) GET(c httpctx.Context) error {
	reportType := c.Param("type")
	format := c.Query("format")
	
	if format == "pdf" {
		c.Response().Header().Set("Content-Type", "application/pdf")
		c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s-report.pdf", reportType))
		// In real implementation, generate PDF
		return c.String(200, "PDF content here")
	}
	
	return c.JSON(200, map[string]interface{}{
		"report_type": reportType,
		"period": map[string]string{
			"start": c.Query("start_date"),
			"end":   c.Query("end_date"),
		},
		"data": map[string]interface{}{
			"summary": "Report data here",
		},
	})
}

// Auth middleware
func authMiddleware() middleware.MiddlewareFunc {
	return func(next middleware.HandlerFunc) middleware.HandlerFunc {
		return func(c httpctx.Context) error {
			// Check Bearer token
			auth := c.Request().Header.Get("Authorization")
			if auth != "" && auth == "Bearer test-token-123" {
				return next(c)
			}
			
			// Check API key
			apiKey := c.Request().Header.Get("X-API-Key")
			if apiKey != "" && apiKey == "test-api-key-123" {
				return next(c)
			}
			
			return c.JSON(401, ErrorResponse{
				Error: "Unauthorized",
				Code:  "UNAUTHORIZED",
			})
		}
	}
}

// CORS middleware
func corsMiddleware() middleware.MiddlewareFunc {
	return func(next middleware.HandlerFunc) middleware.HandlerFunc {
		return func(c httpctx.Context) error {
			c.Response().Header().Set("Access-Control-Allow-Origin", "*")
			c.Response().Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Response().Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")
			
			if c.Request().Method == "OPTIONS" {
				return c.NoContent(204)
			}
			
			return next(c)
		}
	}
}

// Helper functions
func intPtr(i int) *int {
	return &i
}

func float64Ptr(f float64) *float64 {
	return &f
}

func main() {
	// Initialize logger
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()
	
	// Load configuration
	cfg := config.DefaultConfig()
	cfg.Server.Address = ":8085"
	cfg.Logger.Level = "info"
	
	// Create handlers
	handlers := &HandlersManager{
		Home:    &HomeHandler{},
		Docs:    &DocsHandler{},
		OpenAPI: &OpenAPIHandler{},
		API: &APIGroup{
			V1: &APIv1Group{
				Users:    &UsersHandlerV1{},
				Products: &ProductsHandlerV1{},
				Orders:   &OrdersHandlerV1{},
			},
			V2: &APIv2Group{
				Users:    &UsersHandlerV2{},
				Products: &ProductsHandlerV2{},
				Orders:   &OrdersHandlerV2{},
				Search:   &SearchHandlerV2{},
			},
		},
		Admin: &AdminGroup{
			Dashboard: &DashboardHandler{},
			Settings:  &SettingsHandler{},
			Reports:   &ReportsHandler{},
		},
	}
	
	// Create app
	application, err := app.NewApp(
		app.WithConfig(cfg),
		app.WithHandlers(handlers),
		app.WithLogger(logger),
	)
	if err != nil {
		log.Fatal("Failed to create app:", err)
	}
	
	// Add global middlewares
	application.Use(corsMiddleware())
	
	// Register auth middleware
	application.RegisterMiddleware("auth", authMiddleware())
	application.RegisterMiddleware("cors", corsMiddleware())
	
	logger.Info("Starting Advanced API Documentation Example",
		zap.String("address", cfg.Server.Address))
	logger.Info("Available endpoints:",
		zap.String("home", "http://localhost:8085/"),
		zap.String("swagger_ui", "http://localhost:8085/docs"),
		zap.String("redoc", "http://localhost:8085/docs?style=redoc"),
		zap.String("openapi_spec", "http://localhost:8085/openapi.json"),
	)
	
	if err := application.Run(); err != nil && err != http.ErrServerClosed {
		logger.Fatal("Server error", zap.Error(err))
	}
}