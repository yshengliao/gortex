# Gortex Framework - Actionable Improvement Plan

> **Status**: Active | **Last Updated**: 2025/07/26 | **Version**: 3.0

This document contains actionable tasks for improving the Gortex framework. Each task includes the exact prompt to execute the improvement.

## ✅ Completed Tasks

### 1. Metrics Collector Improvements
- [x] **Remove SimpleCollector** - Removed deprecated SimpleCollector implementation
- [x] **Set ImprovedCollector as default** - `NewCollector()` now returns ImprovedCollector
- [x] **Clean up deprecated functions** - Removed deprecated response helper functions

### 2. Code Quality
- [x] **Run go vet** - No issues found
- [x] **Fix all tests** - All tests passing after removing deprecated code

## 📋 Pending Tasks

### Priority 1: Context Cancellation Support

#### Task 1.1: Add Context Checker Tool
**Prompt**: "Create a Go static analysis tool at `internal/analyzer/context_checker.go` that scans all handler and middleware functions to verify they properly handle context cancellation. The tool should check for: 1) Functions accepting context.Context that don't check ctx.Done(), 2) Long-running operations without cancellation checks, 3) Missing context propagation to child operations. Generate a report showing which files need fixes."

#### Task 1.2: Implement Context Cancellation in Handlers
**Prompt**: "Update all HTTP handlers and middleware in the `app/` directory to properly handle context cancellation. For each handler: 1) Add ctx.Done() checks before long operations, 2) Use context.WithTimeout for external calls, 3) Return immediately when context is cancelled. Focus on router.go, handler.go, and all middleware implementations."

#### Task 1.3: Add Context Cancellation Tests
**Prompt**: "Create comprehensive tests for context cancellation in `app/context_cancellation_test.go`. Tests should cover: 1) Request cancellation during handler execution, 2) Timeout scenarios, 3) Cascading cancellation through middleware chain, 4) WebSocket connection cancellation. Use test helpers to simulate slow operations."

### Priority 2: Observability Enhancements

#### Task 2.1: Integrate OpenTelemetry Tracing
**Prompt**: "Create OpenTelemetry integration in `observability/otel/` directory. Implement: 1) `provider.go` with OTLP exporter configuration, 2) `tracing.go` with span creation helpers, 3) `middleware.go` for automatic HTTP tracing. The integration should be optional and configured via `TracingConfig` in the app config."

#### Task 2.2: Add Metrics Limits
**Prompt**: "Update ImprovedCollector in `observability/improved_collector.go` to add configurable limits: 1) Max unique paths (default 1000), 2) Max business metrics (default 100), 3) LRU eviction when limits reached. Add configuration options and ensure thread-safe implementation."

#### Task 2.3: Create Observability Examples
**Prompt**: "Create a new example at `examples/observability/` demonstrating: 1) Metrics collection with custom business metrics, 2) Health checks for database/Redis/external APIs, 3) OpenTelemetry integration with Jaeger. Include docker-compose.yml for running Jaeger locally."

### Priority 3: Project Structure Simplification

#### Task 3.1: Reorganize Observability Package
**Prompt**: "Reorganize the observability package structure: 1) Create `observability/metrics/` and move all metrics-related files, 2) Create `observability/health/` for health check files, 3) Create `observability/tracing/` for tracing files. Update all imports and ensure tests still pass."

#### Task 3.2: Consolidate App Tests
**Prompt**: "Consolidate test files in the app package: 1) Merge all router tests into `router_test.go` and `router_integration_test.go`, 2) Combine binder tests into single `binder_test.go`, 3) Remove redundant test files. Group tests by functionality using subtests."

#### Task 3.3: Create Test Utilities
**Prompt**: "Create `internal/testutil/` package with: 1) Common mock implementations in `mock/`, 2) Test fixtures in `fixture/`, 3) Custom test assertions in `assert/`. Extract all repeated test helpers from existing tests into this package."

### Priority 4: Auto Documentation

#### Task 4.1: Design Documentation Interface
**Prompt**: "Create documentation interfaces in `pkg/doc/interfaces.go`: 1) `DocProvider` interface for different doc formats, 2) `RouteDoc` struct for route documentation, 3) `HandlerDoc` for handler metadata. The design should support OpenAPI/Swagger generation."

#### Task 4.2: Implement Swagger Provider
**Prompt**: "Implement Swagger/OpenAPI provider in `pkg/doc/swagger/`: 1) Parse struct tags for API documentation, 2) Generate OpenAPI 3.0 spec from routes, 3) Support request/response schema generation from struct tags. Include example tags like `api:\"group=User,version=v1\"`."

#### Task 4.3: Add Documentation Middleware
**Prompt**: "Create documentation middleware that: 1) Serves Swagger UI at `/_docs`, 2) Exposes OpenAPI spec at `/_docs/openapi.json`, 3) Auto-generates documentation from registered routes. Add `WithAutoDoc()` option to app initialization."

### Priority 5: Enhanced Tracing

#### Task 5.1: Extend Span Interface
**Prompt**: "Enhance the tracing span interface in `observability/tracing.go` to support severity levels: 1) Add Debug, Info, Warn, Error, Critical methods to Span, 2) Add structured logging with fields, 3) Maintain backward compatibility. Follow the severity level design from Bofry/trace but keep it lightweight."

#### Task 5.2: Add Trace Context Propagation
**Prompt**: "Implement W3C Trace Context propagation: 1) Extract trace headers in middleware, 2) Inject headers for outgoing HTTP requests, 3) Support both W3C and Jaeger formats. Add helpers for common HTTP clients."

## 🚀 Quick Start Commands

### Run Tests
```bash
go test ./... -race -cover
```

### Check Code Quality
```bash
go vet ./...
golangci-lint run
```

### Build Examples
```bash
cd examples/simple && go build
cd examples/auth && go build
cd examples/websocket && go build
```

## 📊 Success Metrics

- **Test Coverage**: Maintain >80% coverage
- **Benchmark Performance**: No regression in router benchmarks
- **Memory Usage**: No memory leaks in 24-hour stress tests
- **API Compatibility**: Zero breaking changes to public APIs

## 🔄 Development Workflow

1. Pick a task from the pending list
2. Copy the prompt and execute it
3. Run tests to ensure nothing breaks
4. Create focused commits (one task = one commit)
5. Update this document marking task as completed

## 📝 Commit Message Format

```
<type>(<scope>): <subject>

<body>

<footer>
```

Types: feat, fix, docs, style, refactor, perf, test, chore

Example:
```
feat(observability): add OpenTelemetry tracing support

- Implement OTLP exporter configuration
- Add tracing middleware for automatic spans
- Support configurable sampling rates

Closes #123
```

---

**Note**: This is a living document. Update task status as you complete them, and add new tasks as they're identified.