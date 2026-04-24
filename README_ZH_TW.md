# Gortex — 高效能 Go Web 框架

[![Go Version](https://img.shields.io/badge/go-1.25+-blue.svg)](https://go.dev/)
![Status](https://img.shields.io/badge/status-v0.5.3--alpha-orange.svg)
[![License](https://img.shields.io/badge/license-MIT-brightgreen.svg)](LICENSE)
![AI Generated](https://img.shields.io/badge/AI_Generated-Antigravity-blueviolet.svg)

> **⚠️ [僅供研究] 此專案透過 AI 在本地復刻過去常用的「通用基礎設施框架」架構哲學。僅為留作紀錄與架構學習之用，請勿用於生產環境。**
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

type HandlersManager struct {
    Home   *HomeHandler   `url:"/"`
    Users  *UserHandler   `url:"/users/:id"`
    Admin  *AdminGroup    `url:"/admin" middleware:"auth"`
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

func main() {
    app, _ := app.NewApp(
        app.WithHandlers(&HandlersManager{
            Home:  &HomeHandler{},
            Users: &UserHandler{},
            Admin: &AdminGroup{
                Dashboard: &DashboardHandler{},
            },
        }),
    )
    app.Run() // :8080
}
```

## 核心概念

### Struct Tag 路由

```go
type HandlersManager struct {
    Users    *UserHandler    `url:"/users/:id"`                   // 動態參數
    Static   *FileHandler    `url:"/static/*"`                    // 萬用字元
    API      *APIGroup       `url:"/api"`                         // 巢狀群組
    Profile  *ProfileHandler `url:"/profile" middleware:"jwt"`    // 受保護路由
    Chat     *ChatHandler    `url:"/chat" hijack:"ws"`            // WebSocket
}
```

### HTTP Method 對應

```go
type UserHandler struct{}

func (h *UserHandler) GET(c types.Context) error    { /* GET /users/:id */ }
func (h *UserHandler) POST(c types.Context) error   { /* POST /users/:id */ }
func (h *UserHandler) DELETE(c types.Context) error { /* DELETE /users/:id */ }
func (h *UserHandler) Profile(c types.Context) error { /* POST /users/:id/profile */ }
```

### Middleware

```go
// 透過 struct tag 套用
type HandlersManager struct {
    Public  *PublicHandler  `url:"/public"`
    Private *PrivateHandler `url:"/private" middleware:"auth"`
    Admin   *AdminHandler   `url:"/admin" middleware:"auth,rbac"`
}

// 或全域套用
app.Router().Use(middleware.Logger(logger), middleware.RequestID())
```

### 組態載入（多來源）

```go
cfg := config.NewConfigBuilder().
    LoadYamlFile("config.yaml").       // 本地開發
    LoadDotEnv(".env").                // 覆蓋值
    LoadEnvironmentVariables("GORTEX"). // K8s 環境變數注入
    MustBuild()
```

## 框架特性總覽

| 分類 | 特性 |
|------|------|
| **路由** | Struct-tag 自動發現、segment-trie、巢狀群組、Context pooling |
| **安全** | 防穿越 `File`、同源鎖定 `Redirect`、CORS 防護、1 MiB body 上限、CSRF、敏感值遮蔽 |
| **可觀測性** | Jaeger/OTel 追蹤、分片指標收集、三態健康檢查、`/_routes` & `/_monitor` |
| **韌性** | Circuit breaker、Token-bucket rate limiter（TTL 自動清理）、優雅關機 |
| **即時通訊** | WebSocket Hub：訊息大小限制、類型白名單、授權勾子 |

## 範例

```bash
go run ./examples/basic      # Struct-tag 路由、binder、middleware chain
go run ./examples/auth       # JWT 登入 / 更新 / /me
go run ./examples/websocket  # Hub 訊息限制與授權
```

## 技術文件

完整技術文件同時提供英文與繁體中文兩個版本：

- 📖 **[繁體中文文件](docs/zh-tw/)** — API 參考、安全指南、架構哲學、設計模式、最佳實踐
- 📖 **[English Documentation](docs/en/)** — API reference, security guide, architecture philosophy, design patterns, best practices
- 🔒 [SECURITY.md](SECURITY.md) — 漏洞通報流程

## 變更紀錄

### v0.5.3-alpha (2026-04-24)

- 精簡根目錄 README，詳細內容移至 `docs/` 索引頁。
- 新增部署指南（Dockerfile、Docker Compose、K8s 範例、DevOps 注意事項）。

### v0.5.2-alpha (2026-04-24)

- 將 `docs/` 重組為 `docs/en/` 與 `docs/zh-tw/` 雙語結構，完成全文件翻譯。
- 新增架構哲學與設計模式學習指南。

### v0.5.1-alpha (2026-04-24)

- 明確標示為架構哲學研究與紀錄專案。

### v0.4.1-alpha (2026-04-24)

- 安全強化（body 上限、冪等 shutdown、rate limiter TTL）。
- 移除重複 router；依賴數從 50 降至 41 個模組。

### v0.4.0-alpha

- 防穿越檔案提供、CORS/CSRF/JWT 強化。
- 8 級嚴重性追蹤、ShardedCollector、CI/CD pipeline。

## 授權

MIT License — 見 [LICENSE](LICENSE)。
