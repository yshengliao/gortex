.PHONY: help run test docs validate generate-sdk clean

# Default target
help:
	@echo "API Documentation Example - Available commands:"
	@echo "  make run        - Run the API server"
	@echo "  make test       - Test API endpoints"
	@echo "  make docs       - Open API documentation"
	@echo "  make validate   - Validate OpenAPI spec"
	@echo "  make generate-sdk - Generate client SDKs"
	@echo "  make clean      - Clean generated files"

# Run the server
run:
	go run main.go

# Test endpoints
test:
	@echo "Testing API endpoints..."
	@echo ""
	@echo "1. Health check:"
	@curl -s http://localhost:8085/ > /dev/null && echo "✓ Server is running" || echo "✗ Server is not running"
	@echo ""
	@echo "2. OpenAPI spec:"
	@curl -s http://localhost:8085/openapi.json | jq -e '.openapi' > /dev/null && echo "✓ OpenAPI spec is valid" || echo "✗ OpenAPI spec is invalid"
	@echo ""
	@echo "3. List users (V2):"
	@curl -s -H "Authorization: Bearer test-token-123" \
		http://localhost:8085/api/v2/users | jq -e '.users' > /dev/null && \
		echo "✓ Users endpoint working" || echo "✗ Users endpoint failed"
	@echo ""
	@echo "4. Create user:"
	@curl -s -X POST http://localhost:8085/api/v2/users \
		-H "Content-Type: application/json" \
		-H "Authorization: Bearer test-token-123" \
		-d '{"email":"test@example.com","name":"Test User","password":"testpass123","role":"user"}' | \
		jq -e '.id' > /dev/null && echo "✓ User creation working" || echo "✗ User creation failed"
	@echo ""
	@echo "5. Search endpoint:"
	@curl -s -X POST http://localhost:8085/api/v2/search \
		-H "Content-Type: application/json" \
		-H "Authorization: Bearer test-token-123" \
		-d '{"query":"test","type":"user","page":1,"page_size":10}' | \
		jq -e '.results' > /dev/null && echo "✓ Search endpoint working" || echo "✗ Search endpoint failed"

# Open documentation
docs:
	@echo "Opening API documentation..."
	@open http://localhost:8085/docs || xdg-open http://localhost:8085/docs || echo "Please open http://localhost:8085/docs in your browser"

# Validate OpenAPI spec
validate:
	@echo "Validating OpenAPI spec..."
	@curl -s http://localhost:8085/openapi.json -o openapi.json
	@npx @apidevtools/swagger-cli validate openapi.json && echo "✓ OpenAPI spec is valid" || echo "✗ OpenAPI spec validation failed"

# Generate client SDKs
generate-sdk:
	@echo "Generating client SDKs..."
	@mkdir -p sdk
	@echo "Fetching OpenAPI spec..."
	@curl -s http://localhost:8085/openapi.json -o openapi.json
	@echo ""
	@echo "Generating TypeScript SDK..."
	@docker run --rm -v "${PWD}:/local" openapitools/openapi-generator-cli generate \
		-i /local/openapi.json \
		-g typescript-axios \
		-o /local/sdk/typescript
	@echo "✓ TypeScript SDK generated in sdk/typescript"
	@echo ""
	@echo "Generating Python SDK..."
	@docker run --rm -v "${PWD}:/local" openapitools/openapi-generator-cli generate \
		-i /local/openapi.json \
		-g python \
		-o /local/sdk/python
	@echo "✓ Python SDK generated in sdk/python"
	@echo ""
	@echo "Generating Go SDK..."
	@docker run --rm -v "${PWD}:/local" openapitools/openapi-generator-cli generate \
		-i /local/openapi.json \
		-g go \
		-o /local/sdk/go
	@echo "✓ Go SDK generated in sdk/go"

# Clean generated files
clean:
	rm -rf sdk/
	rm -f openapi.json
	@echo "Cleanup complete!"