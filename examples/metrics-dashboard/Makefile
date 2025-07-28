.PHONY: help up down logs build test load-test clean dashboards

# Default target
help:
	@echo "Metrics Dashboard Example - Available commands:"
	@echo "  make up         - Start all services with docker-compose"
	@echo "  make down       - Stop all services"
	@echo "  make logs       - View application logs"
	@echo "  make build      - Build the application"
	@echo "  make test       - Run integration tests"
	@echo "  make load-test  - Run load testing"
	@echo "  make dashboards - Open Grafana dashboards"
	@echo "  make clean      - Clean up containers and volumes"

# Start all services
up:
	docker-compose up -d
	@echo "Waiting for services to be ready..."
	@sleep 10
	@echo "Services started!"
	@echo "- Application: http://localhost:8084"
	@echo "- Prometheus: http://localhost:9090"
	@echo "- Grafana: http://localhost:3000 (admin/admin)"
	@echo ""
	@echo "Run 'make dashboards' to open Grafana"

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
	@echo "Checking application health..."
	@curl -s http://localhost:8084/ > /dev/null && echo "✓ Application is running" || echo "✗ Application is not running"
	@echo ""
	@echo "Checking Prometheus targets..."
	@curl -s http://localhost:9090/api/v1/targets | grep -q '"health":"up"' && echo "✓ Prometheus is scraping metrics" || echo "✗ Prometheus scraping failed"
	@echo ""
	@echo "Testing endpoints..."
	@curl -s http://localhost:8084/products > /dev/null && echo "✓ Products endpoint working" || echo "✗ Products endpoint failed"
	@curl -s -X POST http://localhost:8084/orders \
		-H "Content-Type: application/json" \
		-d '{"user_id":"test","items":[{"product_id":"prod_1","quantity":1}],"payment_method":"credit_card"}' > /dev/null && \
		echo "✓ Orders endpoint working" || echo "✗ Orders endpoint failed"
	@echo ""
	@echo "Checking metrics..."
	@curl -s http://localhost:8084/metrics | grep -q 'http_requests_total' && echo "✓ Metrics are being collected" || echo "✗ Metrics collection failed"

# Run load test
load-test:
	docker-compose --profile load-test up load-generator

# Open Grafana dashboards
dashboards:
	@echo "Opening Grafana dashboards..."
	@echo "Login: admin/admin"
	@open http://localhost:3000 || xdg-open http://localhost:3000 || echo "Please open http://localhost:3000 in your browser"

# Clean up everything
clean:
	docker-compose down -v
	docker system prune -f
	@echo "Cleanup complete!"