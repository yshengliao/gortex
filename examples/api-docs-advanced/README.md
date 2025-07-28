# Advanced API Documentation Example

This example demonstrates comprehensive API documentation features using OpenAPI 3.0 specification with Gortex framework.

## Features Demonstrated

### 1. OpenAPI 3.0 Specification
- **Auto-generated spec**: Complete OpenAPI spec from code
- **Multiple servers**: Development, production, and environment-based
- **Rich schemas**: Detailed request/response models with validation
- **Security schemes**: Bearer token and API key authentication

### 2. Interactive Documentation
- **Swagger UI**: Try-it-out functionality with live API testing
- **ReDoc**: Alternative clean documentation interface
- **Custom themes**: Branded documentation experience

### 3. API Versioning
- **Multiple versions**: V1 (deprecated) and V2 APIs
- **Deprecation headers**: Sunset dates for deprecated endpoints
- **Breaking changes**: Handled gracefully between versions

### 4. Advanced Features
- **Request validation**: Comprehensive input validation with detailed errors
- **Response examples**: Multiple examples for different scenarios
- **Webhook documentation**: Event-driven API documentation
- **Rate limiting**: Documented rate limits per endpoint

## Quick Start

### Running the Example

```bash
go run main.go
```

### Accessing Documentation

1. **Home Page**: http://localhost:8085
   - Overview of all features and endpoints

2. **Swagger UI**: http://localhost:8085/docs
   - Interactive API documentation
   - Try-it-out functionality
   - Authentication testing

3. **ReDoc**: http://localhost:8085/docs?style=redoc
   - Clean, three-panel documentation
   - Better for reading/printing

4. **OpenAPI Spec**: http://localhost:8085/openapi.json
   - Raw OpenAPI 3.0 specification
   - Import into Postman/Insomnia

## API Endpoints

### Public Endpoints

#### Home - GET /
Overview page with links to all documentation.

#### Documentation - GET /docs
Interactive Swagger UI documentation.

#### OpenAPI Spec - GET /openapi.json
Machine-readable API specification.

### API v1 (Deprecated)

⚠️ **Deprecated**: These endpoints will be removed on 2024-12-31

#### List Users - GET /api/v1/users
Returns a simple array of users.

**Headers:**
- `Deprecation: true`
- `Sunset: 2024-12-31`

### API v2

#### List Users - GET /api/v2/users
Paginated user listing with filtering.

**Query Parameters:**
- `page` (integer): Page number (default: 1)
- `page_size` (integer): Items per page (default: 20, max: 100)
- `role` (string): Filter by role (admin, user, guest)

**Response:**
```json
{
  "users": [
    {
      "id": "user_123",
      "email": "user@example.com",
      "name": "John Doe",
      "role": "user",
      "active": true,
      "created_at": "2024-01-26T10:30:00Z",
      "updated_at": "2024-01-26T10:30:00Z"
    }
  ],
  "total_count": 150,
  "page": 1,
  "page_size": 20,
  "has_more": true
}
```

#### Create User - POST /api/v2/users
Create a new user account.

**Request Body:**
```json
{
  "email": "user@example.com",
  "name": "John Doe",
  "password": "securepass123",
  "role": "user"
}
```

**Validation Rules:**
- `email`: Required, valid email format
- `name`: Required, 2-100 characters
- `password`: Required, 8-72 characters
- `role`: Required, one of: admin, user, guest

**Response:** 201 Created
```json
{
  "id": "user_1706276400",
  "email": "user@example.com",
  "name": "John Doe",
  "role": "user",
  "active": true,
  "created_at": "2024-01-26T10:30:00Z",
  "updated_at": "2024-01-26T10:30:00Z"
}
```

**Response Headers:**
- `Location: /api/v2/users/user_1706276400`

#### Advanced Search - POST /api/v2/search
Search across multiple entity types with filters.

**Request Body:**
```json
{
  "query": "laptop",
  "type": "product",
  "filters": [
    {
      "field": "price",
      "operator": "lt",
      "value": "1000"
    },
    {
      "field": "category",
      "operator": "eq",
      "value": "electronics"
    }
  ],
  "page": 1,
  "page_size": 20
}
```

**Filter Operators:**
- `eq`: Equals
- `ne`: Not equals
- `lt`: Less than
- `gt`: Greater than
- `le`: Less than or equal
- `ge`: Greater than or equal
- `in`: In list
- `contains`: Contains substring

### Admin Endpoints

#### Dashboard - GET /admin/dashboard
Admin dashboard statistics.

**Response:**
```json
{
  "total_users": 1234,
  "total_orders": 5678,
  "total_revenue": 123456.78,
  "active_users": 890
}
```

#### Generate Report - GET /admin/reports/{type}
Generate various reports in JSON or PDF format.

**Path Parameters:**
- `type`: Report type (sales, users, inventory)

**Query Parameters:**
- `start_date` (required): Start date (YYYY-MM-DD)
- `end_date` (required): End date (YYYY-MM-DD)
- `format` (optional): Output format (json, pdf)

## Authentication

### Bearer Token
Add the Authorization header:
```
Authorization: Bearer YOUR_TOKEN
```

Test token: `test-token-123`

### API Key
Add the X-API-Key header:
```
X-API-Key: YOUR_API_KEY
```

Test API key: `test-api-key-123`

## Error Responses

All errors follow a consistent format:

```json
{
  "error": "Validation failed",
  "code": "VALIDATION_ERROR",
  "details": {
    "errors": {
      "email": "invalid email format",
      "password": "must be at least 8 characters"
    }
  }
}
```

**Error Codes:**
- `VALIDATION_ERROR`: Input validation failed
- `UNAUTHORIZED`: Missing or invalid authentication
- `FORBIDDEN`: Insufficient permissions
- `NOT_FOUND`: Resource not found
- `CONFLICT`: Resource conflict (e.g., duplicate email)
- `INTERNAL_ERROR`: Server error

## Testing with cURL

### List Users
```bash
curl -H "Authorization: Bearer test-token-123" \
  http://localhost:8085/api/v2/users?page=1&page_size=10
```

### Create User
```bash
curl -X POST http://localhost:8085/api/v2/users \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test-token-123" \
  -d '{
    "email": "newuser@example.com",
    "name": "New User",
    "password": "securepass123",
    "role": "user"
  }'
```

### Search Products
```bash
curl -X POST http://localhost:8085/api/v2/search \
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
  }'
```

## Client SDK Generation

Generate client SDKs from the OpenAPI spec:

### JavaScript/TypeScript
```bash
npm install -g @openapitools/openapi-generator-cli
openapi-generator-cli generate \
  -i http://localhost:8085/openapi.json \
  -g typescript-axios \
  -o ./sdk/typescript
```

### Python
```bash
openapi-generator-cli generate \
  -i http://localhost:8085/openapi.json \
  -g python \
  -o ./sdk/python
```

### Go
```bash
openapi-generator-cli generate \
  -i http://localhost:8085/openapi.json \
  -g go \
  -o ./sdk/go
```

## Mock Server

Use Prism to create a mock server from the spec:

```bash
npm install -g @stoplight/prism-cli
prism mock http://localhost:8085/openapi.json
```

## Contract Testing

Validate API responses against the OpenAPI spec:

```bash
npm install -g @openapitools/openapi-generator-cli
openapi-generator-cli validate -i http://localhost:8085/openapi.json
```

## Postman Integration

1. Open Postman
2. Click Import → Link
3. Enter: `http://localhost:8085/openapi.json`
4. Postman will create a collection with all endpoints

## Advanced Documentation Features

### 1. Multiple Examples
The search endpoint includes multiple examples for different use cases:
- User search
- Product search with filters
- Order search with date range

### 2. Enum Validation
Fields like `role` and `status` use enums for strict validation.

### 3. Format Validation
- Email fields use `format: email`
- Dates use `format: date` or `format: date-time`
- UUIDs use custom regex patterns

### 4. Nested Objects
Complex request/response bodies with nested objects and arrays.

### 5. Polymorphic Responses
Search endpoint returns different result types based on the search type.

## Webhook Documentation

### Order Status Webhook
```json
{
  "event": "order.status.changed",
  "timestamp": "2024-01-26T10:30:00Z",
  "data": {
    "order_id": "order_789",
    "old_status": "pending",
    "new_status": "shipped",
    "updated_by": "system"
  }
}
```

### User Registration Webhook
```json
{
  "event": "user.registered",
  "timestamp": "2024-01-26T10:30:00Z",
  "data": {
    "user_id": "user_123",
    "email": "user@example.com",
    "source": "api"
  }
}
```

## Rate Limiting

**Default Limits:**
- Anonymous: 100 requests/hour
- Authenticated: 1000 requests/hour
- Admin: 10000 requests/hour

**Headers:**
- `X-RateLimit-Limit`: Request limit
- `X-RateLimit-Remaining`: Remaining requests
- `X-RateLimit-Reset`: Reset timestamp

## Best Practices

### 1. API Design
- Use consistent naming conventions
- Version your APIs properly
- Provide meaningful error messages
- Include request/response examples

### 2. Documentation
- Keep OpenAPI spec in sync with code
- Document all parameters and responses
- Include authentication requirements
- Provide working examples

### 3. Validation
- Validate all inputs
- Use appropriate data types
- Set reasonable limits
- Return helpful validation errors

### 4. Security
- Always use HTTPS in production
- Implement proper authentication
- Validate and sanitize inputs
- Don't expose sensitive data

## Customization

### Custom Swagger UI Theme
Edit the Swagger UI initialization in `DocsHandler`:
```javascript
window.ui = SwaggerUIBundle({
  // ... other options
  theme: 'material',
  displayRequestDuration: true,
  filter: true,
  showExtensions: true,
  showCommonExtensions: true
});
```

### Custom ReDoc Theme
```javascript
Redoc.init(specUrl, {
  theme: {
    colors: {
      primary: {
        main: '#1976d2'
      }
    },
    typography: {
      fontSize: '16px',
      fontFamily: 'Roboto, sans-serif'
    }
  }
}, document.getElementById('redoc'));
```

## Monitoring

Track API documentation usage:
- Page views for /docs
- OpenAPI spec downloads
- SDK generation requests
- Support ticket correlation

## Troubleshooting

### Swagger UI not loading
1. Check browser console for errors
2. Verify OpenAPI spec is valid JSON
3. Check CORS headers are set

### Authentication not working
1. Verify token format
2. Check middleware registration
3. Test with curl first

### Missing endpoints in docs
1. Ensure handlers are registered
2. Check struct tags are correct
3. Verify OpenAPI generation logic