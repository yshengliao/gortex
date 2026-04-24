# Gortex — 高效能 Go Web 框架

[![Go Version](https://img.shields.io/badge/go-1.25+-blue.svg)](https://go.dev/)
![Status](https://img.shields.io/badge/status-v0.4.1--alpha-orange.svg)
[![License](https://img.shields.io/badge/license-MIT-brightgreen.svg)](LICENSE)

> **零樣板、純 Go Web 框架。用 struct tag 定義路由，不寫註冊程式碼。**
>
> [English](README.md)

## 為什麼選 Gortex？

```go
// 傳統做法：手動註冊路由
r.GET("/", homeHandler)
r.GET("/users/:id", userHandler)
r.GET("/api/v1/users", apiV1UserHandler)
// ... 再來幾十條

// Gortex：透過 struct tag 自動發現路由
type HandlersManager struct {
    Home  *HomeHandler  `url:"/"`
    Users *UserHandler  `url:"/users/:id"`
    API   *APIGroup     `url:"/api"`
}
```

## 快速開始

```bash
go get github.com/yshengliao/gortex
```

```go
package main

import (
    "github.com/yshengliao/gortex/core/app"
    "github.com/yshengliao/gortex/core/types"
)

// 用 struct tag 定義路由
type HandlersManager struct {
    Home   *HomeHandler   `url:"/"`
    Users  *UserHandler   `url:"/users/:id"`
    Admin  *AdminGroup    `url:"/admin" middleware:"auth"`
    WS     *WSHandler     `url:"/ws" hijack:"ws"`
}

type HomeHandler struct{}
func (h *HomeHandler) GET(c types.Context) error {
    return c.JSON(200, map[string]string{"message": "Welcome to Gortex!"})
}

type UserHandler struct{}
func (h *UserHandler) GET(c types.Context) error {
    return c.JSON(200, map[string]string{"id": c.Param("id")})
}

type AdminGroup struct {
    Dashboard *DashboardHandler `url:"/dashboard"`
}

type DashboardHandler struct{}
func (h *DashboardHandler) GET(c types.Context) error {
    return c.JSON(200, map[string]string{"data": "admin only"})
}

type WSHandler struct{}
func (h *WSHandler) HandleConnection(c types.Context) error {
    // WebSocket 升級邏輯
    return nil
}

func main() {
    app, _ := app.NewApp(
        app.WithHandlers(&HandlersManager{
            Home:  &HomeHandler{},
            Users: &UserHandler{},
            Admin: &AdminGroup{
                Dashboard: &DashboardHandler{},
            },
            WS: &WSHandler{},
        }),
    )
    app.Run() // :8080
}
```

## 核心概念

### 1. Struct Tag 路由

```go
type HandlersManager struct {
    Users    *UserHandler    `url:"/users/:id"`        // 動態參數
    Static   *FileHandler    `url:"/static/*"`         // 萬用字元
    API      *APIGroup       `url:"/api"`              // 巢狀群組
    Auth     *AuthHandler    `url:"/auth"`             // 公開路由
    Profile  *ProfileHandler `url:"/profile" middleware:"jwt"` // 受保護
    Chat     *ChatHandler    `url:"/chat" hijack:"ws"` // WebSocket
}
```

### 2. HTTP Method 對應

```go
type UserHandler struct{}

func (h *UserHandler) GET(c types.Context) error    { /* GET /users/:id */ }
func (h *UserHandler) POST(c types.Context) error   { /* POST /users/:id */ }
func (h *UserHandler) DELETE(c types.Context) error { /* DELETE /users/:id */ }
func (h *UserHandler) Profile(c types.Context) error { /* POST /users/:id/profile */ }
```

### 3. 巢狀路由群組

```go
type APIGroup struct {
    V1 *V1Handlers `url:"/v1"`
    V2 *V2Handlers `url:"/v2"`
}

// 產生：
// /api/v1/...
// /api/v2/...
```

## 主要特性

### 效能

- **路由速度提升 45%** — 最佳化的反射快取
- **零外部依賴** — 不需要 Redis、Kafka 或其他外部服務
- **記憶體效率高** — Context pooling 與智慧參數儲存
- **記憶體減少 38%** — 透過 object pooling 降低配置次數

### 開發體驗

- **免手動路由註冊** — 框架自動發現路由
- **型別安全** — 編譯時期路由驗證
- **即時回饋** — 開發模式下的 hot reload
- **內建除錯工具** — dev mode 下啟用 `/_routes`、`/_monitor`

### 正式環境就緒

- **JWT 認證** — 內建 middleware，強制 ≥32 bytes 的 secret
- **WebSocket** — 第一級即時通訊支援，含訊息大小限制、類型白名單與授權勾子
- **Metrics** — 相容 Prometheus 的指標收集
- **優雅關機** — 正確的連線清理流程
- **API 文件** — 從 struct tag 自動產生 OpenAPI/Swagger

### 安全優先預設

- `Context.File` 僅從 `fs.FS` 提供檔案（防路徑穿越）
- `Context.Redirect` 拒絕非同源目標，除非明確加入白名單
- CORS 拒絕 `*` + `AllowCredentials=true` 的錯誤配置
- JSON body 上限 1 MiB（可透過 `DefaultMaxBodyBytes` 調整）；multipart 上限 32 MiB
- Logger 自動遮蔽常見的敏感 header 與 JSON key；`X-Forwarded-For` 僅信任已設定的 proxy
- Synchroniser-token CSRF middleware + `X-RateLimit-*` / `Retry-After` 標頭

通報流程：見 [SECURITY.md](SECURITY.md)。完整預設值：見 [docs/security.md](docs/security.md)。

## Middleware

```go
// 透過 struct tag 套用 middleware
type HandlersManager struct {
    Public  *PublicHandler  `url:"/public"`
    Private *PrivateHandler `url:"/private" middleware:"auth"`
    Admin   *AdminHandler   `url:"/admin" middleware:"auth,rbac"`
}
```

全域 middleware 透過 `router.Use()` 套用：

```go
app, _ := app.NewApp(app.WithHandlers(handlers))
app.Router().Use(
    middleware.Logger(logger),
    middleware.RequestID(),
)
```

## 進階功能

### WebSocket 支援

```go
import gortexws "github.com/yshengliao/gortex/transport/websocket"

type WSHandler struct {
    hub *gortexws.Hub
}

func (h *WSHandler) HandleConnection(c types.Context) error {
    // 標記 `hijack:"ws"` 的路由會自動升級。
    conn, _ := upgrader.Upgrade(c.Response(), c.Request(), nil)
    client := gortexws.NewClient(h.hub, conn, id, logger)
    h.hub.RegisterClient(client)
    return nil
}
```

### API 文件

```go
// 自動產生 OpenAPI/Swagger
import "github.com/yshengliao/gortex/core/app/doc/swagger"

app.NewApp(
    app.WithHandlers(handlers),
    app.WithDocProvider(swagger.NewProvider()),
)

// 存取路徑：
// /_docs      - API 文件 JSON
// /_docs/ui   - Swagger UI 介面
```

### 設定檔

```go
cfg := config.NewConfigBuilder().
    LoadYamlFile("config.yaml").
    LoadDotEnv(".env").
    LoadEnvironmentVariables("GORTEX").
    MustBuild()

app.NewApp(
    app.WithConfig(cfg),
    app.WithHandlers(handlers),
)
```

### 錯誤處理

```go
// 註冊業務錯誤
errors.Register(ErrUserNotFound, 404, "User not found")
errors.Register(ErrUnauthorized, 401, "Unauthorized")

// 自動錯誤回應
func (h *UserHandler) GET(c types.Context) error {
    user, err := h.service.GetUser(c.Param("id"))
    if err != nil {
        return err // 框架自動處理 HTTP 回應
    }
    return c.JSON(200, user)
}
```

## 效能測試

測試環境：Apple M3 Pro、Go 1.24、`transport/http` 套件、`-benchmem -benchtime=2s`。

| 操作 | gortexRouter（正式環境） | 記憶體 | 備註 |
|------|--------------------------|--------|------|
| 靜態路由 | 163 ns/op | 288 B, 6 allocs | `GET /user/home` |
| 參數路由 | 267 ns/op | 576 B, 7 allocs | `GET /user/:name` |
| 多路由（23 條） | 325 ns/op | 624 B, 7 allocs | 深層參數查詢 |
| Context Pool | 122 ns/op | 208 B, 4 allocs | pooled vs 230 ns unpooled |
| Smart Params | 89 ns/op | 48 B, 1 alloc | struct-tag 參數繫結 |

> 使用正式環境的 segment-trie router（`gortex_router.go`）測量。

## 專案結構

```
gortex/
├── core/                   # 框架核心
│   ├── app/                # 應用程式、生命週期、路由佈線
│   ├── context/            # Binder、request/response context
│   ├── handler/            # Handler 快取與反射輔助
│   └── types/              # 公開介面（types.Context, …）
├── transport/              # I/O 表面
│   ├── http/               # HTTP context、router、response helpers
│   └── websocket/          # Hub、client、訊息授權
├── middleware/             # CORS、CSRF、rate limit、logger、auth、recover…
├── pkg/                    # 可重用元件
│   ├── auth/               # JWT（強制 ≥32 bytes secret）
│   ├── config/             # YAML / .env / 環境變數設定
│   ├── errors/             # 錯誤註冊表
│   ├── utils/              # Pool、circuit breaker、httpclient、requestid
│   └── validation/         # 輸入驗證
├── observability/          # health、metrics、tracing、otel
├── tools/                  # 獨立開發工具（獨立 go.mod）
│   └── analyzer/           # Context 傳播靜態分析器
├── examples/               # basic、websocket、auth
└── internal/               # 共用測試工具
```

## 最佳實踐

### 1. 組織 Handler 結構

```go
// 將相關端點分組
type HandlersManager struct {
    Auth    *AuthHandlers    `url:"/auth"`
    Users   *UserHandlers    `url:"/users"`
    Admin   *AdminHandlers   `url:"/admin" middleware:"auth,admin"`
}
```

### 2. 使用 Service 層（選用）

```go
type UserHandler struct {
    service *UserService // 業務邏輯放這裡
}

func (h *UserHandler) GET(c types.Context) error {
    user, err := h.service.GetUser(c.Request().Context(), c.Param("id"))
    // 處理回應...
}
```

### 3. 善用開發模式

```go
cfg.Logger.Level = "debug" // 啟用 /_routes、/_monitor 等端點
```

## 範例

可直接執行的參考實作 — 每個範例都是單一 `main.go`，附帶 README 與 `curl`/`websocat` 操作紀錄。

| 範例 | 展示內容 |
|------|----------|
| [basic](examples/basic/) | Struct-tag 路由、binder、預設 middleware chain |
| [auth](examples/auth/) | JWT 登入 / 更新 / `/me`，含 `NewJWTService` 熵值檢查 |
| [websocket](examples/websocket/) | Hub 設定：訊息大小限制、類型白名單、授權勾子 |

```bash
go run ./examples/basic      # 全部監聽 :8080
```

## 文件

| 文件 | 說明 |
|------|------|
| [API 參考](docs/API.md) | Context 介面、router、struct tag、middleware、WebSocket、安全預設 |
| [安全指南](docs/security.md) | 檔案提供、重導向、CORS、JSON body 限制的安全使用方式 |
| [Context 處理](docs/best-practices/context-handling.md) | 生命週期、取消、goroutine、逾時策略 |
| [Metrics 分析](docs/performance/metrics-analysis.md) | Collector 效能測試與選擇指南 |
| [SECURITY.md](SECURITY.md) | 漏洞通報流程與支援版本 |

## 變更紀錄

### v0.4.1-alpha (2026-04-24)

**安全性與可靠性**
- `Context.Bind()` 透過 `http.MaxBytesReader` 強制 1 MiB body 上限；`Context.Validate()` 在未註冊驗證器時回傳 `ErrValidatorNotRegistered`。
- `Hub.Shutdown()` / `ShutdownWithTimeout()` 透過 `sync.Once` 實現冪等呼叫。
- `MemoryRateLimiter` 重構：TTL 追蹤條目與背景清理。

**架構**
- 移除重複的 radix-tree router，保留正式環境的 segment-trie `gortex_router.go`。
- `/_routes` 與 `/_monitor` 回傳即時資料；`/_config` 遮蔽敏感值。
- `injectDependencies` 在無 `inject` tag 時跳過反射掃描（快速路徑）。

**依賴** — 主模組從 50 → 41 個模組（direct 13→11、indirect 23→16）。
- 移除 `Bofry/config`；`pkg/config` 使用零依賴 `simpleLoader`（YAML + .env + 環境變數 + CLI）。
- 將 `internal/analyzer` 遷移至獨立的 `tools/analyzer/`（獨立 `go.mod`），移除 `golang.org/x/tools`。
- `otel/sdk` 標記為測試專用；更新 `golang.org/x/crypto`。

**整理** — 移除 Echo 殘留、dead code、虛構 API 與 LLM 生成的佔位文件。合併設定測試（876 → 532 行）。更新 `.golangci.yml`、`.gitignore` 與全部文件以符合現狀。

### v0.4.0-alpha

**安全強化**
- 防路徑穿越的 `Context.File` 與 `Context.Redirect`
- CORS、dev error page、logger、binder 強化以防常見誤用
- JWT secret 熵值檢查、trusted-proxy client-IP、WebSocket 讀取限制 + 授權勾子
- CSRF middleware 與 rate-limit 回應標頭

**強化可觀測性**
- 8 級嚴重性追蹤（DEBUG 到 EMERGENCY）
- 內建效能測試與瓶頸偵測
- ShardedCollector 支援高吞吐量指標

**CI/CD**
- 每次 PR 執行 `go test ./... -race -count=1`
- `go vet` + 靜態分析；效能測試歷史紀錄於 `performance/`

## 貢獻

歡迎貢獻！請參閱 [CONTRIBUTING.md](CONTRIBUTING.md)。

## 授權

MIT License — 見 [LICENSE](LICENSE) 檔案。

---

<p align="center">
由 Go 社群用心打造 | 純 Go 框架
</p>
