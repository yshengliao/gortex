.PHONY: help up down logs build test clean

# Default target
help:
	@echo "Advanced Tracing Example - Available commands:"
	@echo "  make up        - Start all services with docker-compose"
	@echo "  make down      - Stop all services"
	@echo "  make logs      - View application logs"
	@echo "  make build     - Build the application"
	@echo "  make test      - Run integration tests"
	@echo "  make load-test - Run load testing"
	@echo "  make clean     - Clean up containers and volumes"

# Start all services
up:
	docker-compose up -d
	@echo "Waiting for services to be ready..."
	@sleep 5
	@echo "Services started!"
	@echo "- Application: http://localhost:8083"
	@echo "- Jaeger UI: http://localhost:16686"

# Stop all services
down:
	docker-compose down

# View logs
logs:
	docker-compose logs -f app

# Build the application
build:
	docker-compose build

# Run tests
test:
	@echo "Running health check..."
	@curl -s http://localhost:8083/health | jq .
	@echo "\nCreating test order..."
	@curl -s -X POST http://localhost:8083/orders \
		-H "Content-Type: application/json" \
		-d '{"user_id":"test_user","items":[{"item_id":"item_001","quantity":1}]}' | jq .
	@echo "\nChecking inventory..."
	@curl -s http://localhost:8083/inventory/item_001 | jq .
	@echo "\nGetting analytics..."
	@curl -s http://localhost:8083/analytics/sales | jq .

# Run load test
load-test:
	docker-compose --profile load-test up load-generator

# Clean up everything
clean:
	docker-compose down -v
	docker system prune -f