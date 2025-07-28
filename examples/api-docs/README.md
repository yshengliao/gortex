# API Documentation Example

This example demonstrates Gortex's automatic API documentation generation using struct tags and the pluggable documentation provider system.

## Features Demonstrated

- Automatic route discovery and documentation
- Swagger/OpenAPI 3.0 specification generation
- Interactive Swagger UI
- Struct tag-based API metadata
- Zero configuration documentation

## Running the Example

```bash
go run main.go
```

Then visit:
- **API Documentation UI**: http://localhost:8083/docs
- **Swagger JSON**: http://localhost:8083/docs/swagger.json
- **Routes List**: http://localhost:8083/_routes

## API Endpoints

### User Management
- `GET /users` - List all users
- `POST /users` - Create a new user
- `POST /users/profile` - Get user profile (custom method)

### Product Catalog
- `GET /products` - List all products
- `POST /products/search` - Search products (custom method)

## Struct Tags

The example uses struct tags to provide API metadata:

```go
type Handlers struct {
    Users *UserHandler `url:"/users" api:"group=User Management,version=v1,desc=User operations,tags=users|admin"`
}
```

### Available Tags

- `url`: Route path prefix
- `api`: API metadata
  - `group`: API group name
  - `version`: API version
  - `desc`: Description
  - `tags`: Pipe-separated tags for categorization
  - `basePath`: Base path for all routes in this handler

## Customization

You can customize the documentation by:

1. Providing a custom `DocProvider` implementation
2. Adding more metadata through struct tags
3. Implementing custom parameter extraction
4. Adding request/response schemas

## Architecture

The documentation system consists of:

1. **DocProvider Interface**: Pluggable documentation generation
2. **SwaggerProvider**: Default OpenAPI 3.0 implementation
3. **Route Collection**: Automatic during route registration
4. **UI Handler**: Serves Swagger UI for interactive documentation

## Future Enhancements

- Request/response body schema generation from struct tags
- Authentication/security scheme definitions
- Example values in documentation
- Multiple output formats (ReDoc, Postman, etc.)
- Static documentation generation for CI/CD