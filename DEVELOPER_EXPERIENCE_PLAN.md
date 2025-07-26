# Gortex 開發者體驗優化計畫

## 願景

讓 Gortex 成為 Go 生態系中最簡單易用的 Web 框架，優先考慮開發者體驗，適當犧牲部分效能來換取更好的易用性。

## 核心理念

> **開發者體驗 > 效能優化**
> 
> 我們相信一個易用的框架比一個極致效能的框架更有價值。當開發者能快速上手並享受開發過程時，效能優化可以在後期逐步進行。

## 主要功能規劃

### 1. 🎯 Handler 自動初始化

#### 問題描述

目前開發者需要手動初始化每個 handler，這導致大量重複代碼：

```go
// 😩 現在的寫法 - 繁瑣且容易出錯
handlers := &HandlersManager{
    Home:   &HomeHandler{},     // 忘記初始化會 panic
    Health: &HealthHandler{},
    User:   &UserHandler{},
    Static: &StaticHandler{},
    API: &APIGroup{
        V1: &APIv1Group{
            Users:    &UserAPIHandler{},
            Products: &ProductHandler{},
        },
        V2: &APIv2Group{
            Users: &UserAPIHandlerV2{},
        },
    },
}
```

#### 解決方案

```go
// 😊 理想的寫法 - 簡潔優雅
app.NewApp(
    app.WithHandlers(&HandlersManager{}), // 自動初始化所有 handlers！
)
```

#### 實作細節

```go
func autoInitHandlers(v reflect.Value) {
    if v.Kind() == reflect.Ptr && v.IsNil() {
        v.Set(reflect.New(v.Type().Elem()))
    }
    
    // 遞迴處理所有欄位
    elem := v.Elem()
    for i := 0; i < elem.NumField(); i++ {
        field := elem.Field(i)
        if field.Kind() == reflect.Ptr && field.CanSet() && field.IsNil() {
            field.Set(reflect.New(field.Type().Elem()))
            autoInitHandlers(field) // 遞迴處理嵌套結構
        }
    }
}
```

### 2. 📊 智慧路由日誌

#### 問題描述

開發者需要手動追蹤所有註冊的路由，容易遺漏或錯誤：

```go
// 😩 現在需要手動維護路由列表
logger.Info("Routes:",
    zap.String("home", "GET /"),
    zap.String("health", "GET /health"),
    // ... 很容易遺漏或不同步
)
```

#### 解決方案

```go
// 😊 自動生成漂亮的路由表
app.NewApp(
    app.WithHandlers(&HandlersManager{}),
    app.WithRoutesLogger(), // 自動打印所有路由！
)
```

#### 輸出範例

```
┌────────┬─────────────────────────┬─────────────────────┬──────────────┐
│ Method │ Path                    │ Handler             │ Middlewares  │
├────────┼─────────────────────────┼─────────────────────┼──────────────┤
│ GET    │ /                       │ HomeHandler         │ none         │
│ GET    │ /health                 │ HealthHandler       │ none         │
│ GET    │ /users/:id              │ UserHandler         │ auth         │
│ POST   │ /users/:id              │ UserHandler         │ auth         │
│ GET    │ /api/v1/users/:id       │ UserAPIHandler      │ jwt          │
│ GET    │ /api/v1/products/:id    │ ProductHandler      │ jwt          │
│ GET    │ /api/v2/users/:id       │ UserAPIHandlerV2    │ jwt, rbac    │
└────────┴─────────────────────────┴─────────────────────┴──────────────┘
```

### 3. 🚀 開發模式增強

#### 自動重載提示

```go
if cfg.IsDevelopment() {
    app.logger.Info("🔥 Development mode enabled!")
    app.logger.Info("📝 Available debug endpoints:")
    app.logger.Info("   • /_routes   - View all routes")
    app.logger.Info("   • /_config   - View configuration")
    app.logger.Info("   • /_monitor  - System metrics")
    app.logger.Info("💡 Tip: Install air for hot reload: go install github.com/cosmtrek/air@latest")
}
```

#### 錯誤頁面美化

開發模式下提供更友善的錯誤頁面，包含：
- 堆疊追蹤
- 請求詳情
- 可能的解決方案建議

### 4. 🎨 程式碼生成器

#### 快速生成 Handler

```bash
# 生成新的 handler
gortex gen handler User

# 自動生成：
# - handlers/user_handler.go
# - handlers/user_handler_test.go
# - 自動註冊到 HandlersManager
```

生成的代碼：

```go
type UserHandler struct{}

// GET /users/:id
func (h *UserHandler) GET(c context.Context) error {
    id := c.Param("id")
    // TODO: Implement your logic here
    return c.JSON(200, map[string]interface{}{
        "id": id,
        "message": "Get user",
    })
}

// POST /users/:id
func (h *UserHandler) POST(c context.Context) error {
    // TODO: Implement your logic here
    return c.JSON(200, map[string]interface{}{
        "message": "Create user",
    })
}
```

### 5. 🔧 Context 輔助方法

#### 更友善的 API

```go
// 簡化的參數獲取
func (h *UserHandler) GET(c context.Context) error {
    // 自動類型轉換
    userID := c.ParamInt("id", 0)      // 預設值 0
    page := c.QueryInt("page", 1)       // 預設值 1
    
    // 簡化的綁定
    var req UserRequest
    if err := c.Bind(&req); err != nil {
        return c.BadRequest("Invalid request: " + err.Error())
    }
    
    // 便利的回應方法
    return c.OK(user) // 自動設定 200 狀態碼
}
```

### 6. 🛠️ 效能優化（保持簡單）

雖然我們優先考慮易用性，但仍可實施一些不影響 API 的優化：

#### Context Pool（透明實施）

```go
// 內部實現，對開發者透明
var ctxPool = sync.Pool{
    New: func() interface{} {
        return &gortexContext{
            params: make(map[string]string, 4), // 預分配小容量
        }
    },
}

// 開發者無需關心 pool 的存在
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
    ctx := acquireContext(req, w)
    defer releaseContext(ctx)
    // ...
}
```

#### 智慧參數存儲

```go
// 少量參數用 slice（快速）
// 大量參數自動切換到 map（方便）
type smartParams struct {
    count  int
    keys   [4]string    // 小陣列，避免分配
    values [4]string
    overflow map[string]string // 超過 4 個參數時使用
}
```

## 實施計畫

### 第一階段：核心功能（1 週）

1. **Handler 自動初始化** ⭐⭐⭐⭐⭐
   - 實作 autoInitHandlers 函數
   - 整合到 WithHandlers
   - 處理各種邊界情況

2. **路由日誌系統** ⭐⭐⭐⭐⭐
   - 收集路由信息
   - 實作美化輸出
   - 支援多種格式

### 第二階段：開發體驗（1 週）

3. **Context 增強** ⭐⭐⭐⭐
   - 新增便利方法
   - 類型轉換輔助
   - 錯誤處理簡化

4. **開發模式優化** ⭐⭐⭐⭐
   - 友善的錯誤頁面
   - 自動重載提示
   - Debug 端點增強

### 第三階段：工具鏈（1 週）

5. **程式碼生成器** ⭐⭐⭐
   - CLI 工具開發
   - 模板系統
   - 自動註冊機制

6. **效能優化** ⭐⭐⭐
   - Context Pool
   - 智慧參數存儲
   - 基準測試

## 成功指標

### 開發者體驗指標

- **上手時間**：新手 < 5 分鐘能跑起 Hello World
- **程式碼行數**：相比其他框架減少 50%
- **錯誤提示**：100% 的錯誤都有明確的解決建議

### 效能指標（次要）

- **可接受的效能損失**：相比極致優化版本慢 20-30%
- **記憶體使用**：保持在合理範圍內
- **啟動時間**：< 100ms

## 範例：完整應用

展示所有功能的整合效果：

```go
package main

import (
    "github.com/yshengliao/gortex/app"
    "github.com/yshengliao/gortex/context"
)

// 簡潔的 Handler 定義
type Handlers struct {
    *HomeHandler    `url:"/"`
    *UserHandler    `url:"/users/:id"`
    *AdminGroup     `url:"/admin" middleware:"auth"`
}

type HomeHandler struct{}

func (h *HomeHandler) GET(c context.Context) error {
    return c.OK("Welcome to Gortex! 🚀")
}

type UserHandler struct{}

func (h *UserHandler) GET(c context.Context) error {
    userID := c.ParamInt("id", 0)
    return c.OK(map[string]interface{}{
        "id": userID,
        "name": "User " + c.Param("id"),
    })
}

type AdminGroup struct {
    *DashboardHandler `url:"/dashboard"`
}

type DashboardHandler struct{}

func (h *DashboardHandler) GET(c context.Context) error {
    return c.OK("Admin Dashboard")
}

func main() {
    // 超級簡單的啟動方式
    app, _ := app.NewApp(
        app.WithHandlers(&Handlers{}),    // 自動初始化！
        app.WithRoutesLogger(),            // 自動打印路由！
        app.WithDevelopmentMode(),         // 開發模式！
    )
    
    app.Run() // 就這樣！
}
```

## 設計原則

1. **簡單優於複雜**：如果一個功能需要解釋，那就需要重新設計
2. **慣例優於配置**：提供合理的預設值
3. **錯誤要友善**：每個錯誤都應該告訴開發者如何修復
4. **漸進式複雜度**：簡單的事情簡單做，複雜的事情也能做

## 不做什麼

- ❌ 不追求極致效能
- ❌ 不實施複雜的優化
- ❌ 不犧牲易用性
- ❌ 不增加學習曲線

## 結論

Gortex 的目標是成為 Go 開發者最喜愛的 Web 框架。通過優先考慮開發者體驗，我們相信可以創造一個既強大又易用的框架。

> "Make it work, make it right, then make it fast" - Kent Beck

我們現在專注於前兩步，讓框架先「能用」且「好用」，效能優化可以在未來逐步進行。