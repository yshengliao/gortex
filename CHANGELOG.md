# Changelog

All notable changes to Gortex are documented here.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/); versions follow SemVer 0.x convention (breaking changes expressed via minor bump; `-alpha` suffix retained while pre-1.0).

---

## [v0.8.1-alpha] — 2026-06-19

Patch release — no breaking changes.

### Changed
- **Route logging**: registered-route logging and the `/_routes` debug view now show the actual middleware function names, resolved via `runtime.FuncForPC`, instead of a placeholder count (`"N middleware"`) or index-based names (`middleware_0`). Package paths and the method-value `-fm` suffix are trimmed for readability. Affects `RouteLogInfo.Middlewares` in `core/app` (#26).

---

## [v0.8.0-alpha] — 2026-06-10

Third-round security and correctness audit. This release is a **minor bump** because it contains multiple breaking changes to the JWT, middleware-tag, and rate-limit APIs.

### BREAKING CHANGES

1. **JWT `typ` claim — re-issue all tokens required**
   `pkg/auth.JWTService` now embeds a `typ` claim in every issued token (`"access"` or `"refresh"`). `ValidateToken` rejects any token whose `typ` is not `"access"`; `ValidateRefreshToken` rejects anything whose `typ` is not `"refresh"`. Tokens issued by earlier versions carry no `typ` claim and are therefore **rejected by both validators**. Re-issue all tokens after upgrading. Access and refresh tokens are no longer interchangeable. Signing and verification are pinned to HS256 exactly; tokens signed with any other algorithm are rejected.

2. **`middleware:"..."` and `ratelimit:"..."` struct tags now fail loudly**
   Unresolved names and malformed rate-limit tags (e.g. `"100permin"`) now return errors from `NewApp`/`RegisterRoutes`. Previously these were warn-and-drop, meaning routes were registered **without** the intended protection. Built-in names are `auth`, `requestid`, and `recover` (`auth` additionally requires a `middleware.MiddlewareFunc` registered in the app context); any other name — `rbac`, `jwt`, etc. — must be removed from the tag or registered as a custom middleware under that name in the app context.

3. **Rate-limit default key no longer honours forwarding headers without `TrustedProxies`**
   The default `KeyFunc` in `GortexRateLimitConfig` now keys on the direct TCP peer address. `X-Forwarded-For` / `X-Real-IP` are only respected when the source IP is within the configured `TrustedProxies` CIDRs. This closes the IP-spoofing vector that allowed any client to evade rate limiting by forging a forwarding header. `DefaultGortexRateLimitConfig()` no longer pre-creates a `Store`; the middleware lazily creates and writes back a stoppable `MemoryRateLimiter` on first use.

4. **`LoggerConfig.LogResponseBody` is now a documented no-op**
   The previous implementation relied on a `SetResponse` hook that no context type implemented, so it silently logged status 200 and an empty body for every request. The dead code has been removed. The field is retained for compatibility but has no effect. To log response bodies, wrap the response writer in your own middleware. Request-body logging (`LogRequestBody` + `BodyRedactor`) is unaffected.

5. **Binder default JSON body cap lowered 10 MiB → 1 MiB**
   `DefaultMaxJSONBodyBytes` is now 1 MiB, matching `transport/http.DefaultMaxBodyBytes` and the documented security default. Fields tagged with an explicit `bind` struct tag now surface conversion errors (auto-bound fields remain lenient).

6. **Removed: `OptimizedCollector`, `FixedHealthChecker`, `RouteCache`**
   - `observability/metrics.OptimizedCollector` (the `sync.Map` variant) is removed. It offered no improvement on high-write loads (-1%), was 20x slower on mixed read/write workloads, and its non-atomic first-write path double-counted cardinality. Use `ShardedCollector` or `ImprovedCollector`.
   - `observability/health.FixedHealthChecker` is removed (its `Stop()` never set the stopped flag; `SafeHealthChecker` and `HealthChecker` remain).
   - `core/app.RouteCache` is removed (only tests referenced it; `ClearCache` still clears the handler cache).
   - Doc/parser verb inference is now aligned to runtime: custom handler methods register as POST; the doc layer no longer advertises GET/PUT/DELETE routes that do not exist at runtime.

7. **`websocket.Hub.RegisterClient` now returns `error`**
   A registration that arrives while the hub is shutting down is refused with `ErrHubShuttingDown`, and the refused client's send channel is closed so a `WritePump` blocked on it exits instead of leaking until TCP teardown. Previously the client was silently dropped with its send channel left open. Existing callers still compile (Go allows discarding the return value), but should check the error before starting the client's pumps.

---

### Fixed

- **Router**: Named wildcard params are now populated — `c.Param("filepath")` returns the matched suffix for routes declared as `url:"/static/*filepath"`. Previously the param was always empty.
- **Router**: Error path no longer double-writes the response status line on custom writers.
- **Router**: Registration data race on shared route groups resolved by cloning the middleware slice per field instead of appending onto the parent's backing array.
- **Router**: Middleware chain aliasing fixed — sibling handler fields no longer clobber each other's inherited middleware when the parent chain is extended.
- **Middleware/logger**: Logs the real HTTP status code from the tracked response writer; 4xx responses log at Warn, 5xx at Error.
- **Middleware/error_handler**: Double-write guard — only takes the raw `WriteHeader` fast path when the tracked writer reports the response as uncommitted.
- **Middleware/compression**: Emits `Vary: Accept-Encoding` on every negotiated response (compressed, below-MinSize, and ineligible-type) so shared caches key correctly. Responses with a pre-set `Content-Encoding` pass through instead of being double-gzipped.
- **Middleware/CORS**: `Validate()` rejects `AllowHeaders: ["*"]` combined with `AllowCredentials: true`; the Fetch spec's literal-asterisk restriction was previously bypassable via header reflection.
- **Middleware/auth**: `SkipPaths` now match on segment boundaries — `/public` no longer skips `/publicadmin`. Bearer token parsing tolerates extra whitespace and rejects empty tokens.
- **Middleware/recovery**: Nil `Logger` falls back to `zap.L()` so recovered panics reach the structured pipeline.
- **Pkg/errors**: `HandleBusinessError` no longer echoes raw `err.Error()` to clients for unregistered errors, preventing internal strings (SQL fragments, file paths, etc.) from leaking into HTTP responses.
- **Pkg/config**: `.env` lines and `--flag` CLI arguments are parsed into in-memory overlay maps rather than written into `os.Environ`. Config loading is now idempotent and no longer leaks values to child processes or parallel tests.
- **Transport/websocket — hub shutdown**: Shutdown no longer freezes the event loop with bare sleeps. A grace window services register/metrics/broadcast channels so concurrent callers are answered; queued notices drain to clients before send channels close; an empty hub shuts down immediately. `ShutdownWithTimeout` with a sub-500ms deadline now succeeds on a healthy hub.
- **Transport/websocket — private-target resolution**: A private message's recipient is resolved from the client-supplied `Data["target"]` into `Message.Target` *before* the whitelist/Authorizer check, so a configured Authorizer sees — and can veto — the final recipient.

### Added

- **Transport/websocket**: Hub metrics expose `dropped_broadcasts` (Broadcast calls discarded because the hub's broadcast queue was full) and `forced_disconnects` (clients evicted because their send buffer was full during delivery).
- **Pkg/config**: `Validate()` enforces a 32-byte minimum JWT secret at config-load time, surfacing misconfigurations before any token is issued.
- **Pkg/auth**: `ErrInvalidTokenType` — returned when a token's `typ` claim does not match the expected type, or when the claim is absent (legacy tokens).

### Changed

- `tools/analyzer/go.mod` directive changed from `go 1.26.2` to `go 1.25.0` to match the project's documented toolchain.
- Benchmark runner (`BenchmarkGortexRouter`) writes results to `b.TempDir()` instead of the tracked `benchmark_db.json`, preventing stray zero-value entries on every `go test ./...` run.

---

## [v0.7.1-alpha] — 2026-06-05

- Fix: `TestBenchmarkSuite` now writes to `t.TempDir()` instead of the real `benchmark_db.json`, preventing zero-value entries from being appended on every `go test ./...` run.

## [v0.7.0-alpha] — 2026-06-04

- Milestone version bump consolidating the v0.6.x second-round audit cycle.
- Circuit-breaker half-open: the Open→HalfOpen transition request is now counted as the first probe, so half-open admits exactly `MaxRequests` (not `MaxRequests + 1`) and the probe's success counts toward closing the circuit.

## [v0.6.2-alpha] — 2026-06-04

- WebSocket `ReadPump` exits cleanly on hub shutdown (no goroutine leak).
- Circuit-breaker half-open closes on successes rather than admissions (correct for `MaxRequests > 1`).
- Logger honours `BodyLogLimit` for response bodies and guards a non-string `request_id`.
- Documentation accuracy: corrected `errors.Register` signature, response-helper API, `File`/`FileFS`, `Redirect`, and a `Context` header example across `docs/en`, `docs/zh-tw`, and `CLAUDE.md`.

## [v0.6.1-alpha] — 2026-04-25

- Documentation audit: fixed typos, verified Traditional Chinese terminology.
- Updated `CLAUDE.md` with current performance data and feature inventory.
- Updated `SECURITY.md` supported version line.
- Consolidated version references across all docs.

## [v0.5.4-alpha] — 2026-04-25

- Zero-allocation routing: eliminated `map[string]string` and `strings.Split` from hot path; embedded `responseWriter` in pooled context. 0 allocs/op for static, param, wildcard, and deep-param routes.

## [v0.5.3-alpha] — 2026-04-24

- Simplified root READMEs; moved detailed content to `docs/` index pages.
- Added deployment guide (Dockerfile, Docker Compose, K8s manifests, DevOps notes).

## [v0.5.2-alpha] — 2026-04-24

- Restructured `docs/` into `docs/en/` and `docs/zh-tw/` with full bilingual coverage.
- Added architecture philosophy and design patterns learning guide.

## [v0.5.1-alpha] — 2026-04-24

- Marked as research and architectural record project.

## [v0.4.1-alpha] — 2026-04-24

- Security hardening (body cap, idempotent shutdown, rate limiter TTL).
- Removed duplicate router; dependency count reduced from 50 → 41 modules.

## [v0.4.0-alpha]

- Path-traversal-safe file serving, CORS/CSRF/JWT hardening.
- 8-level severity tracing, ShardedCollector, CI/CD pipeline.
