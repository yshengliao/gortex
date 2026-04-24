# Design Patterns & Learning Guide

> Based on v0.5.1-alpha, this document catalogues the engineering patterns implemented in Gortex that are worth studying, along with areas not yet implemented but worth exploring.

## Implemented Core Design Patterns

### 1. Struct Tag Driven Routing (Declarative Routing)

Uses Go's `reflect` package to recursively scan structs, converting `url`, `middleware`, `hijack`, and `ratelimit` tags into route registrations. This declarative approach is relatively uncommon in the Go ecosystem (most frameworks still use imperative `r.GET()`).

**Key learnings:**
- **Practical reflection usage**: How to safely traverse structs, read tags, and validate method signatures
- **Convention over Configuration**: Method names `GET`, `POST` automatically map to HTTP methods; custom method names auto-convert to kebab-case paths
- **Middleware inheritance**: Parent group middleware automatically propagates to child routes through recursion

```go
type HandlersManager struct {
    Users  *UserHandler  `url:"/users/:id"`
    Admin  *AdminGroup   `url:"/admin" middleware:"auth"`
}
```

**Reference**: `core/app/route_registration.go`

---

### 2. Segment-trie Router

A hand-crafted Trie route tree supporting static paths, `:param` dynamic parameters, and `*` wildcards. Compared to radix trees, segment-based tries are easier to understand.

**Key learnings:**
- **Route matching priority**: Static > Param > Wildcard backtracking strategy
- **Why Trie over regex**: Each request only requires string splitting + map lookup, avoiding the non-deterministic backtracking of regex

**Reference**: `transport/http/gortex_router.go`

---

### 3. Context Pool (`sync.Pool` in Practice)

Each HTTP request acquires a Context from `sync.Pool` and returns it when done.

**Key learnings:**
- `Pool.New` initialisation strategy (pre-allocating the `store` map)
- `ReleaseContext` clears fields but retains map capacity before returning (avoiding reallocation)
- `store` uses `for k := range` to delete entries individually rather than `make` (preserving underlying bucket space)

**Reference**: `transport/http/pool.go`

---

### 4. smartParams — Small Buffer Optimisation (SBO)

Uses a fixed array `[4]string` for common cases (≤4 path parameters), falling back to a map only when overflow occurs.

**Key learnings:**
- Why 4 is a reasonable threshold (most REST API path parameters don't exceed 3 levels)
- How to avoid small object heap allocations in Go

**Reference**: `transport/http/smart_params.go`

---

### 5. Sharded Metrics Collector

16 independent shards, each with its own `RWMutex` and LRU. Uses FNV hash for key distribution.

**Key learnings:**
- **Lock Sharding**: Splitting a single global lock into N locks to reduce contention under high concurrency
- **Per-shard LRU eviction**: Preventing global LRU from becoming a bottleneck
- **Atomic counters**: High-frequency HTTP counters use `atomic.AddInt64` instead of locks

**Reference**: `observability/metrics/sharded_collector.go`

---

### 6. Circuit Breaker

A textbook implementation of the Closed → Open → Half-Open state machine.

**Key learnings:**
- **State Machine idiom in Go**: `atomic.Value` for State storage, `sync.Mutex` for Counts protection
- **Generation-based concurrency control**: Uses generation numbers to identify which generation a request belongs to during Half-Open state
- **`atomic.Uint32` for Half-Open throttling**: Limits concurrent requests without locks

**Reference**: `pkg/utils/circuitbreaker/circuitbreaker.go`

---

### 7. WebSocket Hub (Actor-like Single-goroutine Event Loop)

`Hub.Run()` is an event loop running perpetually in a dedicated goroutine. All reads and writes to the `clients` map flow through channels into this single goroutine, eliminating the need for locks.

**Key learnings:**
- **Go-style Actor Model**: Channels replace mutexes; a single goroutine serialises all state access
- **Graceful shutdown design**: `shutdownOnce` + two-phase close (send close message, wait 500ms, then force disconnect)
- **Channel-full strategy**: During broadcast, `select { case client.send <- msg: default: }` drops messages for slow consumers

**Reference**: `transport/websocket/hub.go`

---

### 8. Rate Limiter with TTL and Cleanup

`MemoryRateLimiter` uses `golang.org/x/time/rate` Token Bucket to create per-IP limiters.

**Key learnings:**
- **Background cleanup goroutine**: `runCleanup` periodically scans for expired entries to prevent unbounded memory growth
- **RFC-compliant response headers**: Calculation logic for `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset`, and `Retry-After`
- **`RateLimitStatuser` interface separation**: Not all stores can provide status; uses type assertion to conditionally emit headers

**Reference**: `middleware/ratelimit.go`

---

### 9. Three-state Health Check System

Supports registering multiple `HealthCheck` functions, run periodically by a background goroutine with per-check timeouts.

**Key learnings:**
- **Three-state health**: Healthy / Degraded / Unhealthy is closer to real-world K8s readiness/liveness probe design than simple up/down
- **Concurrent checks with WaitGroup**: All checks run in parallel, with `sync.WaitGroup` waiting for completion

**Reference**: `observability/health/health.go`

---

### 10. Tracing Abstraction Layer

Defines a `Tracer` → `EnhancedTracer` interface hierarchy, paired with `SimpleTracer` (in-memory) and an OTel adapter.

**Key learnings:**
- **Interface-driven design**: The framework depends only on the `Tracer` interface, allowing seamless switching from in-memory to Jaeger/OTel without code changes
- **Context propagation**: How Spans propagate through the request chain via `context.WithValue`
- **Severity level design**: How 8 severity levels (DEBUG to EMERGENCY) map to OTel's 3-level Status

**Reference**: `observability/tracing/tracing.go`, `observability/otel/adapter.go`

---

## Areas Not Yet Implemented but Worth Exploring

The following directions serve as a record of "what could be studied further if this framework were to be deepened."

### 1. Complete Graceful Shutdown Chain

The current `App.Shutdown()` has a basic flow, but production-grade graceful shutdown includes:

- **Drain connections**: Remove from Load Balancer first (K8s `preStop` hook + set readiness probe to false), wait for in-flight requests to complete
- **Dependency ordering**: Close HTTP listener → drain requests → close WebSocket Hub → close DB connection pool
- **Shutdown deadline propagation**: Pass the OS signal deadline to all subsystems

### 2. Middleware Chain Error Semantics

Currently, middleware returns `error` which is handled uniformly by `ServeHTTP`. Production frameworks typically need to distinguish:

- **Retriable errors** vs **non-retriable errors**
- **Errors that should be logged** vs **errors already handled by the middleware**
- **Error boundaries**: Whether upstream middleware can observe an error swallowed by downstream middleware

### 3. Configuration Hot-reload

`pkg/config` currently loads once at startup. In Kubernetes, ConfigMaps are automatically updated by kubelet to the Pod's mount directory. Supporting `fsnotify` to watch for config file changes would enable: adjusting log levels without restart, dynamically tuning rate limit thresholds, and hot-swapping feature flags.

### 4. Request-level Timeout Propagation

The framework currently lacks a "global request timeout" mechanism. Production frameworks typically set a deadline context in the outermost middleware, ensuring that even if a Handler forgets to set a timeout, the entire request won't hang indefinitely.

### 5. Structured Errors

The current `pkg/errors` is a simple code-to-message registry. More advanced designs might include: Error Code namespaces (`USER.NOT_FOUND` vs `ORDER.PAYMENT_FAILED`), Error Chains (using `errors.As` / `errors.Is` for layered matching), and i18n error messages.

### 6. OpenAPI / Swagger Auto-generation

`core/app/doc/` already has a `swagger.Provider` skeleton, but the spec generation logic is not yet complete. Automatically deriving an OpenAPI spec from struct tags and method signatures is a topic with significant depth.

---

## Suggested Learning Path

| Difficulty | Topic | Starting File |
|------------|-------|---------------|
| ⭐ | `sync.Pool` and Context reuse | `transport/http/pool.go` |
| ⭐ | Small Buffer Optimisation | `transport/http/smart_params.go` |
| ⭐⭐ | Segment-trie route matching | `transport/http/gortex_router.go` |
| ⭐⭐ | Token Bucket Rate Limiter | `middleware/ratelimit.go` |
| ⭐⭐ | Three-state Health Check | `observability/health/health.go` |
| ⭐⭐⭐ | Circuit Breaker state machine | `pkg/utils/circuitbreaker/circuitbreaker.go` |
| ⭐⭐⭐ | Lock Sharding + LRU eviction | `observability/metrics/sharded_collector.go` |
| ⭐⭐⭐ | Actor-like WebSocket Hub | `transport/websocket/hub.go` |
| ⭐⭐⭐⭐ | Reflection-based route registration | `core/app/route_registration.go` |
| ⭐⭐⭐⭐ | OTel Adapter + Tracing | `observability/tracing/tracing.go` |
