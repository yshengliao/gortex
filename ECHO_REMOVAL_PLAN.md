# Gortex Echo 框架移除計畫 - Commit 任務清單

## 專案概述
本計畫將 Gortex 框架從 Echo v4 依賴完全遷移到自建 HTTP 框架，預期可減少 25-33% 依賴、提升 20-40% 路由效能、減少 30-40% Binary 大小。

## 階段一：核心路由系統替換（3 commits）

### Commit 1: 建立自建 HTTP 路由器基礎
**分支**: `feature/custom-router-base`
**預估時間**: 3-4 小時

**任務內容**:
```bash
# Claude Code 提示詞
Create a custom HTTP router to replace Echo's routing system while maintaining the existing declarative struct tag routing functionality. The router should:

1. **Core Router Implementation**:
   - Create `pkg/router/router.go` with `Router` struct
   - Support standard HTTP methods (GET, POST, PUT, DELETE, PATCH, HEAD, OPTIONS)
   - Maintain existing reflection-based route discovery from struct tags
   - Handle URL parameters and wildcards
   - Preserve current `url:` and `hijack:` tag semantics

2. **Handler Interface**:
   - Define `HandlerFunc` type: `func(ctx *Context) error`
   - Create `Context` struct wrapping `*http.Request` and `http.ResponseWriter`
   - Add JSON binding/rendering methods to Context
   - Implement parameter extraction methods

3. **Middleware Chain**:
   - Define `Middleware` type: `func(HandlerFunc) HandlerFunc`
   - Implement middleware chain composition
   - Support middleware ordering and conditional execution

**Requirements**:
- Zero breaking changes to existing handler signatures initially
- Maintain performance characteristics (target: 600-800 ns/op routing)
- Full test coverage with benchmarks
- Backward compatibility bridge for gradual migration

**Files to create**:
- `pkg/router/router.go`
- `pkg/router/context.go` 
- `pkg/router/middleware.go`
- `pkg/router/router_test.go`
```

**驗證標準**:
- [ ] 路由基準測試達到 600-800 ns/op
- [ ] 支援所有 HTTP 方法
- [ ] 單元測試覆蓋率 > 90%
- [ ] 反射式路由發現功能正常

---

### Commit 2: Context 系統實作
**分支**: `feature/context-implementation`
**預估時間**: 2-3 小時

**任務內容**:
```bash
# Claude Code 提示詞
Implement a lightweight HTTP context system to replace Echo's context while maintaining API compatibility. The context should:

1. **Context Structure**:
   - Wrap `*http.Request` and `http.ResponseWriter`
   - Provide JSON binding: `c.Bind(&struct{})` 
   - Provide JSON response: `c.JSON(status, data)`
   - URL parameter access: `c.Param("id")`
   - Query parameter access: `c.QueryParam("filter")`
   - Request header access: `c.Request().Header.Get()`

2. **Response Helpers**:
   - Status code setting
   - Header manipulation
   - Stream/file serving capabilities
   - Error response formatting

3. **Request Processing**:
   - Body parsing (JSON, form data)
   - File upload handling
   - Real IP detection
   - User agent extraction

**Performance Requirements**:
- JSON binding: <500 ns/op
- Parameter access: <50 ns/op
- Zero unnecessary allocations in hot paths

**Files to modify/create**:
- `pkg/router/context.go` (expand)
- `pkg/router/response.go`
- `pkg/router/request.go`
- `pkg/router/context_test.go`
```

**驗證標準**:
- [ ] JSON binding 效能 < 500 ns/op
- [ ] 參數存取效能 < 50 ns/op
- [ ] API 與 Echo Context 相容
- [ ] 零記憶體洩漏

---

### Commit 3: 應用程式整合層
**分支**: `feature/app-integration`
**預估時間**: 3-4 小時

**任務內容**:
```bash
# Claude Code 提示詞
Create an application integration layer that bridges the new router with the existing app structure while maintaining all current functionality:

1. **App Integration**:
   - Modify `internal/app/app.go` to use custom router instead of Echo
   - Maintain all existing `WithXXX` option functions
   - Preserve graceful shutdown functionality
   - Keep development mode features intact

2. **Route Registration**:
   - Update `internal/app/router.go` to use new router
   - Maintain reflection-based handler discovery
   - Preserve struct tag routing (`url:`, `hijack:`)
   - Support WebSocket upgrade handling

3. **Backward Compatibility**:
   - Create Echo context adapter for gradual migration
   - Provide conversion helpers between old/new contexts
   - Maintain existing handler method signatures temporarily

**Migration Strategy**:
- Parallel systems initially (can toggle between Echo/custom)
- Feature flag for router selection
- Comprehensive integration tests

**Files to modify**:
- `internal/app/app.go`
- `internal/app/router.go`
- Add `pkg/compat/echo_adapter.go`
```

**驗證標準**:
- [ ] 所有現有功能保持正常
- [ ] 可以在 Echo 和自建路由間切換
- [ ] 整合測試全部通過
- [ ] WebSocket 功能正常

---

## 階段二：中間件系統遷移（5 commits）

### Commit 4: 核心中間件轉換
**分支**: `feature/core-middleware-migration`
**預估時間**: 2-3 小時

**任務內容**:
```bash
# Claude Code 提示詞
Convert core middleware components from Echo middleware to standard HTTP middleware:

1. **Error Handler Middleware**:
   - Update error handling to use custom middleware format
   - Change from `echo.MiddlewareFunc` to `func(HandlerFunc) HandlerFunc`
   - Maintain existing error response format
   - Preserve request ID integration

2. **Request ID Middleware**:
   - Create `internal/middleware/request_id.go` if not exists
   - Remove Echo context dependencies
   - Use standard HTTP headers and context values

3. **Recovery Middleware**:
   - Implement panic recovery for standard HTTP handlers
   - Maintain current panic handling behavior
   - Integrate with error response system

**Requirements**:
- Identical functionality to current Echo middleware
- Performance parity or better
- Zero breaking changes to configuration APIs

**Files to create/modify**:
- Create `internal/middleware/error_handler.go`
- Create `internal/middleware/request_id.go`
- Create `internal/middleware/recovery.go`
- Update relevant test files
```

**驗證標準**:
- [ ] 錯誤處理行為不變
- [ ] Request ID 正確產生和傳遞
- [ ] Panic recovery 功能正常
- [ ] 效能不降低

---

### Commit 5: 認證中間件轉換
**分支**: `feature/auth-middleware-migration`
**預估時間**: 3-4 小時

**任務內容**:
```bash
# Claude Code 提示詞
Convert authentication middleware from Echo to standard HTTP middleware:

1. **JWT Middleware**:
   - Update `internal/auth/middleware.go`
   - Replace `echo.Context` with custom `Context`
   - Maintain JWT claims extraction and validation
   - Preserve role-based access control

2. **Context Integration**:
   - Update claims storage/retrieval methods
   - Maintain `GetClaims()`, `GetUserID()`, `GetUsername()` functions
   - Ensure thread-safe context operations

3. **Authorization Helpers**:
   - Update `RequireRole()` and `RequireGameID()` middleware if they exist
   - Maintain existing authorization logic
   - Preserve error response formats

**Files to modify**:
- `internal/auth/middleware.go`
- Update any auth helper functions
- Create comprehensive tests
```

**驗證標準**:
- [ ] JWT 驗證功能正常
- [ ] Claims 正確存取
- [ ] 角色驗證功能正常
- [ ] 向後相容性保持

---

### Commit 6: 可觀測性中間件轉換
**分支**: `feature/observability-middleware-migration`
**預估時間**: 2-3 小時

**任務內容**:
```bash
# Claude Code 提示詞
Convert observability middleware (metrics and tracing) from Echo to standard HTTP:

1. **Metrics Middleware**:
   - Update metrics collection to use custom middleware
   - Convert `MetricsMiddleware` from Echo to standard HTTP
   - Maintain all existing metrics collection
   - Preserve performance characteristics (163 ns/op)

2. **Health Check Integration**:
   - Update health check endpoints if needed
   - Ensure metrics are properly exposed
   - Maintain monitoring dashboard functionality

3. **Performance Monitoring**:
   - Ensure no performance degradation
   - Maintain zero allocation paths
   - Update benchmarks and tests

**Files to modify**:
- Update or create metrics middleware
- Related test files
- Ensure integration with monitoring dashboard
```

**驗證標準**:
- [ ] Metrics 收集正常
- [ ] 效能維持 163 ns/op
- [ ] 監控儀表板功能正常
- [ ] 零記憶體分配路徑

---

### Commit 7: 壓縮和靜態文件中間件
**分支**: `feature/compression-static-middleware`
**預估時間**: 3-4 小時

**任務內容**:
```bash
# Claude Code 提示詞
Convert compression and static file middleware from Echo to standard HTTP:

1. **Compression Middleware**:
   - Update `internal/middleware/compression/compression.go`
   - Convert from `echo.MiddlewareFunc` to standard middleware
   - Maintain Brotli and Gzip support
   - Preserve configuration options and performance

2. **Static File Middleware**:
   - Update `internal/middleware/static/static.go`
   - Convert to standard HTTP middleware
   - Maintain file serving capabilities
   - Preserve security features (path traversal protection)

3. **Rate Limiting**:
   - Update `internal/middleware/ratelimit.go`
   - Convert to standard HTTP middleware
   - Maintain TTL-based cleanup (157 ns/op performance)

**Files to modify**:
- `internal/middleware/compression/compression.go`
- `internal/middleware/static/static.go`
- `internal/middleware/ratelimit.go`
```

**驗證標準**:
- [ ] Brotli 和 Gzip 壓縮正常
- [ ] 靜態文件服務正常
- [ ] Rate limiting 效能維持 157 ns/op
- [ ] 安全性功能保持

---

### Commit 8: 開發模式中間件轉換
**分支**: `feature/dev-mode-middleware`
**預估時間**: 2-3 小時

**任務內容**:
```bash
# Claude Code 提示詞
Convert development-specific middleware from Echo to standard HTTP:

1. **Development Logger**:
   - Update development logging middleware
   - Convert from Echo middleware to standard HTTP
   - Maintain request/response body logging
   - Preserve sensitive header masking

2. **Development Error Pages**:
   - Update HTML error page rendering
   - Convert to use custom context
   - Maintain stack trace and request detail display
   - Preserve browser vs API detection

3. **Debug Endpoints**:
   - Update /_routes, /_error, /_config, /_monitor endpoints
   - Convert to use custom router
   - Maintain all development features

**Files to modify**:
- Update development mode middleware
- Update debug endpoint handlers
- Ensure all dev features work
```

**驗證標準**:
- [ ] 開發模式日誌正常
- [ ] 錯誤頁面顯示正常
- [ ] Debug endpoints 功能正常
- [ ] 敏感資訊遮罩正常

---

## 階段三：範例和測試更新（2 commits）

### Commit 9: 範例應用程式更新
**分支**: `feature/update-examples`
**預估時間**: 2-3 小時

**任務內容**:
```bash
# Claude Code 提示詞
Update all example applications to use the new custom HTTP framework instead of Echo:

1. **Simple Example**:
   - Update `examples/simple/main.go`
   - Replace Echo context usage with custom context
   - Maintain existing API endpoints and functionality
   - Update handler method signatures

2. **WebSocket Example**:
   - Update `examples/websocket/main.go`
   - Use custom router for WebSocket upgrade
   - Maintain WebSocket hub/client pattern
   - Preserve all WebSocket functionality

3. **Auth Example**:
   - Update `examples/auth/main.go`
   - Use new auth middleware
   - Maintain login and protected endpoint functionality
   - Update JWT handling

**Requirements**:
- All examples must work identically to current versions
- Update any Echo-specific code comments
- Ensure clean, idiomatic usage of new framework

**Files to modify**:
- `examples/simple/main.go`
- `examples/websocket/main.go`
- `examples/auth/main.go`
```

**驗證標準**:
- [ ] Simple example 運行正常（Port 8080）
- [ ] WebSocket example 運行正常（Port 8082）
- [ ] Auth example 運行正常（Port 8081）
- [ ] 所有端點功能相同

---

### Commit 10: 測試套件全面更新
**分支**: `feature/update-tests`
**預估時間**: 3-4 小時

**任務內容**:
```bash
# Claude Code 提示詞
Update all tests to use the custom HTTP framework and remove Echo dependencies:

1. **Unit Test Updates**:
   - Update all handler tests to use custom context
   - Replace Echo test utilities with custom ones
   - Update middleware tests to use standard HTTP middleware pattern

2. **Integration Tests**:
   - Update integration tests to use new router
   - Maintain test coverage levels
   - Ensure all functionality is properly tested

3. **Benchmark Updates**:
   - Update all benchmarks to reflect new performance characteristics
   - Add benchmarks for new router and context operations
   - Ensure performance targets are met

**Performance Targets**:
- Router: 600-800 ns/op
- Context operations: <100 ns/op  
- Middleware chain: Maintain current performance

**Files to update**:
- All `*_test.go` files
- Benchmark files
- Test utilities
```

**驗證標準**:
- [ ] 所有測試通過
- [ ] 測試覆蓋率 > 80%
- [ ] 效能基準測試達標
- [ ] 無 Echo 相關測試程式碼

---

## 階段四：清理和最佳化（2 commits）

### Commit 11: Echo 依賴清理
**分支**: `feature/remove-echo-dependencies`
**預估時間**: 1-2 小時

**任務內容**:
```bash
# Claude Code 提示詞
Remove Echo framework dependencies and clean up remaining Echo-specific code:

1. **Dependency Removal**:
   - Remove `github.com/labstack/echo/v4` from `go.mod`
   - Remove `github.com/labstack/gommon` (Echo's common utilities)
   - Remove any other Echo-related dependencies
   - Run `go mod tidy` to clean up

2. **Import Cleanup**:
   - Remove all Echo imports from codebase
   - Clean up unused import statements
   - Update any Echo-specific error types

3. **Documentation Updates**:
   - Update README.md to reflect custom HTTP framework
   - Update CLAUDE.md architecture description
   - Remove Echo references from code comments

**Verification**:
- Ensure all tests pass
- Verify all examples work
- Confirm no Echo imports remain
- Validate binary size reduction

**Files to modify**:
- `go.mod`
- All source files (import cleanup)
- Documentation files
```

**驗證標準**:
- [ ] go.mod 無 Echo 依賴
- [ ] 無 Echo import 存在
- [ ] Binary 大小減少 30-40%
- [ ] 所有功能正常運作

---

### Commit 12: 效能最佳化和文檔完整化
**分支**: `feature/performance-optimization`
**預估時間**: 3-4 小時

**任務內容**:
```bash
# Claude Code 提示詞
Optimize the custom HTTP framework and complete documentation:

1. **Performance Optimization**:
   - Profile and optimize hot paths in router and context
   - Implement object pooling where beneficial
   - Optimize memory allocations in middleware chain
   - Add connection pooling optimizations

2. **API Stabilization**:
   - Finalize public API interfaces
   - Add comprehensive API documentation
   - Ensure consistent error handling patterns

3. **Framework Documentation**:
   - Create migration guide from Echo
   - Document performance improvements
   - Update framework positioning and benefits
   - Add benchmark comparison results

**Performance Goals**:
- Router performance: 600-800 ns/op (20-40% improvement)
- Memory usage: 20-35% reduction
- Binary size: 30-40% smaller

**Files to create/modify**:
- `MIGRATION.md` (Echo to custom framework)
- `PERFORMANCE.md` (benchmarks and improvements)
- Update `README.md` and `CLAUDE.md`
```

**驗證標準**:
- [ ] 路由效能提升 20-40%
- [ ] 記憶體使用減少 20-35%
- [ ] 完整的遷移文檔
- [ ] 效能基準測試報告

---

## 執行指南

### 執行順序
1. 按照 commit 編號順序執行
2. 每個 commit 完成後進行測試驗證
3. 確保 CI/CD 通過後再進行下一個 commit

### 分支策略
```bash
# 建立功能分支
git checkout -b feature/custom-router-base

# 完成後合併到開發分支
git checkout develop
git merge --no-ff feature/custom-router-base

# 所有階段完成後合併到 main
git checkout main
git merge --no-ff develop
```

### 測試驗證
每個 commit 後執行：
```bash
# 執行所有測試
go test ./...

# 執行基準測試
go test -bench=. -benchmem ./...

# 檢查依賴
go mod graph | grep echo

# 測試範例
cd examples/simple && go run main.go
cd examples/websocket && go run main.go
cd examples/auth && go run main.go
```

### 回滾策略
如果遇到問題：
1. 使用 Echo 適配器恢復功能
2. 回滾到上一個穩定的 commit
3. 分析問題並調整計畫

## 預期成果

### 技術指標
- **依賴數量**: 12 → 8-9 個（減少 25-33%）
- **Binary 大小**: 減少 30-40%
- **路由效能**: 提升 20-40%
- **記憶體使用**: 減少 20-35%
- **啟動時間**: 顯著提升

### 架構改進
- 完全自主的 HTTP 框架
- 更好的效能特性
- 更簡潔的程式碼結構
- 符合 "Self-contained & Lightweight" 定位

### 風險管理
- 保持向後相容性直到最後階段
- 完整的測試覆蓋
- 漸進式遷移策略
- 詳細的效能監控

---

**最後更新**: 2025/07/24
**預估總時間**: 35-47 小時（約 5-7 個工作天）
**建議執行週期**: 2 週（包含測試和穩定期）