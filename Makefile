.PHONY: all build test clean install lint fmt vet

# Variables
BINARY_NAME=gortex
CLI_PATH=./cmd/gortex
GO_FILES=$(shell find . -name '*.go' -not -path "./vendor/*")

# Default target
all: test build

# Build the CLI tool
build:
	@echo "Building CLI tool..."
	@go build -o $(BINARY_NAME) $(CLI_PATH)
	@echo "Build complete: $(BINARY_NAME)"

# Install the CLI tool globally
install: build
	@echo "Installing CLI tool..."
	@go install $(CLI_PATH)
	@echo "Installed gortex"

# Run tests
test:
	@echo "Running tests..."
	@go test -v -race -coverprofile=coverage.out ./...
	@echo "Tests complete"

# Run tests with coverage report
test-coverage: test
	@echo "Generating coverage report..."
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Format code
fmt:
	@echo "Formatting code..."
	@gofmt -s -w $(GO_FILES)
	@echo "Code formatted"

# Run go vet
vet:
	@echo "Running go vet..."
	@go vet ./...
	@echo "Vet complete"

# Run linter (requires golangci-lint)
lint:
	@echo "Running linter..."
	@golangci-lint run
	@echo "Linting complete"

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -f $(BINARY_NAME)
	@rm -f coverage.out coverage.html
	@rm -f .gortex-dev-server
	@echo "Clean complete"

# Quick check - format, vet, and test
check: fmt vet test

# Release preparation
release: check
	@echo "Preparing for release..."
	@echo "Current version: v0.1.10"
	@echo "Don't forget to:"
	@echo "  1. Update version in cmd/gortex/main.go"
	@echo "  2. Update README.md changelog section"
	@echo "  3. Create git tag: git tag v0.1.10"
	@echo "  4. Push tag: git push origin v0.1.10"