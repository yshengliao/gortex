# Echo 移除計畫

## 計畫目標

以階段性任務推動專案全面移除 Echo 框架，轉型至自家 Gortex context、router、middleware 架構，確保所有核心模組、路由、中介層與測試皆與新介面兼容，進而簡化依賴、提升彈性與可維護性。

## 任務分解與執行計畫

### Phase 1: 核心基礎建設 (Core Infrastructure) ✅

*目標：建立 Gortex 框架的最小可行核心，定義好未來的標準介面。*

1. **Context 與 Handler** ✅:
    * `gortex/context`: 建立純 Gortex `Context` interface，包含請求與回應處理能力。 ✅
    * `gortex/app`: 定義 `gortex.HandlerFunc` 作為標準 handler 簽名。 ✅
2. **基礎路由 (Basic Router)** ✅:
    * `gortex/router`: 實作一個基礎的 Gortex router，支援靜態路由註冊與 handler 綁定。 ✅
3. **Middleware 介面** ✅:
    * `gortex/middleware`: 定義 `gortex.MiddlewareFunc` interface，並建立 middleware 鏈的基礎機制。 ✅

### Phase 2: 模組與 Middleware 遷移 (Module & Middleware Migration) 🚧

*目標：將專案內部的獨立模組與 middleware 從 Echo 依賴遷移至 Gortex 標準介面。*

1. **核心 Middleware 重構** 🚧:
    * `middleware/request_id.go`: 重寫 RequestID middleware，移除 Echo 依賴 ✅
    * `middleware/ratelimit.go`: 重寫 RateLimit middleware，使其符合 `gortex.MiddlewareFunc` 介面 🔄
    * `middleware/dev_error_page.go`: 重寫開發錯誤頁面 middleware 🔄
2. **認證模組 (Auth) 重構**:
    * `auth/`: 重構 `jwt.go` 與 `middleware.go`，移除 `echo.Context` 依賴。
3. **可觀測性 (Observability) 解耦**:
    * `observability/`: 修改 `metrics.go`，使其從 Echo hook 改為與 Gortex `Context` 或 middleware 整合。
4. **通用套件 (pkg) 遷移**:
    * `pkg/`: 逐一重構 `abtest`, `errors`, `requestid` 等模組，移除 Echo 依賴。
    * `validation/`: 將 `validator.go` 與 Echo 的整合層移除。
5. **回應 (Response) 處理**:
    * `response/`: 重構 `response.go`，使其直接操作 Gortex `Context`。

### Phase 3: 路由系統與應用層整合 (Routing & App Integration)

*目標：完成 Gortex Router 的功能並替換掉所有 Echo Router 的使用場景。*

1. **增強 Gortex Router**:
    * `gortex/router`: 為 Gortex router 增加路由分組 (Route Groups)、動態參數、middleware 鏈的完整支援。
2. **替換應用層 (App) 框架**:
    * `app/app.go`: 將主應用程式的啟動流程從 `echo.New()` 替換為 Gortex app。
    * `app/binder.go`: 建立一個不依賴 Echo 的新 binder。
    * `app/router.go`: 將所有路由註冊邏輯遷移至 Gortex router。
3. **移除兼容層 (Compatibility Layer)**:
    * 刪除 `app/router_adapter.go`, `context/adapter.go`, `pkg/compat/`, `pkg/middleware/adapter.go` 等所有為兼容 Echo 而生的程式碼。

### Phase 4: 全面驗證與清理 (Validation & Cleanup)

*目標：確保所有功能在新架構下正常運作，並徹底移除 Echo 依賴。*

1. **單元與整合測試**:
    * `*_test.go`: 增修或重寫所有 handler、middleware、router 的單元測試與整合測試。
2. **範例程式更新**:
    * `examples/`: 全面改寫所有範例程式，使其完全使用 Gortex 框架。
3. **依賴清理**:
    * `go.mod`, `go.sum`: 執行 `go mod tidy`，確保 `github.com/labstack/echo/v4` 已被完全移除。

### Phase 5: 文件與工具 (Documentation & Tooling)

*目標：更新所有對外文件，並提供必要的遷移輔助。*

1. **文件更新**:
    * `README.md`, `CLAUDE.md`: 徹底重寫文件，移除所有 Echo 相關字眼。
2. **遷移指引**:
    * 撰寫一份遷移指引，協助外部使用者將他們的專案從舊架構遷移至 Gortex。
3. **兼容層封存**:
    * 建立 `legacy-echo` 分支，將移除前的兼容層程式碼封存備查。
4. **(可選) 自動化工具**:
    * 開發腳本工具，自動轉換 handler 簽名，加速大型專案的遷移過程。

## Commit 說明建議

| 任務階段 | Commit 範例 |
| :--- | :--- |
| Phase 1: 核心基礎建設 | `feat(core): 實作 Gortex Context 與基礎 Router` |
| Phase 2: 模組遷移 | `refactor(auth): 移除 auth 模組對 Echo 的依賴` <br> `refactor(middleware): 將 ratelimit middleware 遷移至 Gortex` |
| Phase 3: 路由與整合 | `feat(router): 為 Gortex Router 新增路由分組功能` <br> `refactor(app): 全面以 Gortex 取代 Echo 作為應用框架` |
| Phase 4: 驗證與清理 | `test: 為 Gortex router 增加整合測試` <br> `chore: 從 go.mod 移除 Echo 依賴` |
| Phase 5: 文件與工具 | `docs: 更新 README，全面使用 Gortex API` <br> `chore: 建立 legacy-echo 分支保存兼容層程式碼` |

## Echo 依賴盤點（摘要）

語系與架構適用性判斷需依據以下各檔案實際 import 狀況調整：

| 檔案 | 目的 / 用途 |
| :--- | :--- |
| `app/app.go` | 主要應用框架，採用 Echo router 與 middleware 集成 |
| `app/binder.go` | 請求 binding 工具，需 Echo context |
| `app/router.go` | 功能開發與註冊依賴 Echo router |
| `app/router_adapter.go` | routerAdapter 切換 Echo / Gortex |
| `auth/jwt.go` | JWT 認證，需 Echo context |
| `auth/middleware.go` | 認證 middleware，需 Echo context |
| `context/adapter.go` | context 轉換 Echo <-> Gortex |
| `examples/**/main.go` | 所有範例程式皆使用 Echo |
| `middleware/dev_error_page.go` | 開發環境錯誤頁面，需 Echo context |
| `middleware/ratelimit.go` | 流量限制 middleware，需 Echo context |
| `middleware/request_id.go` | 請求 ID middleware，需 Echo context |
| `observability/metrics.go` | 指標採集，與 Echo 整合 |
| `pkg/abtest/abtest.go` | A/B 測試，需 Echo context |
| `pkg/compat/echo_adapter.go` | Echo context 轉 Gortex router context |
| `pkg/compat/echo_context_wrapper.go` | Echo context 包裝器 |
| `pkg/errors/response.go` | 錯誤回應處理，需 Echo context |
| `pkg/middleware/adapter.go` | Echo middleware / generic chain 轉換 |
| `pkg/requestid/requestid.go` | 請求 ID 產生器，與 Echo 整合 |
| `response/response.go` | 統一 JSON 輸出，依賴 Echo context |
| `validation/validator.go` | 請求驗證器，與 Echo 整合 |

## 範例計畫提示詞（英文）

Refactor the codebase to remove all usage of Echo’s context and routing system.
Finalize and adopt a Gortex-native context.Context interface throughout the codebase.
Update all handlers, routers, and middleware to use this interface directly; remove all adapters and compatibility utilities related to Echo (including EchoContextAdapter and any bridge functions).
Replace all route registration and middleware chains with a new, fully featured Gortex router supporting route groups, dynamic parameters, and middleware chaining.
Drop all Echo-specific imports from middleware, and rewrite middleware to the new standard interface.
Update all user and developer documentation to reflect the new architecture, and supply migration guides.
Strictly verify with new and updated tests that all features—including handler binding, middleware, and context propagation—work as expected.
Provide a legacy branch maintaining previous Echo compatibility, and supply a script or tool to help automate handler interface migrations.
