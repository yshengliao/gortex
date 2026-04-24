# Gortex API 參考指南

> 標準匯入路徑：`core/app`, `core/types`, `core/context`, `transport/http`, `transport/websocket`, `middleware`, `pkg/auth`, `pkg/validation`。

## 核心介面

### Context 介面

定義於 `core/types`。在 Handler 中通常以 `types.Context` 參照。

```go
// core/types.Context
type Context interface {
    // 請求
    Request() *http.Request
    SetRequest(r *http.Request)
    
    // 回應
    Response() ResponseWriter
    
    // 路徑參數
    Param(name string) string
    ParamNames() []string
    SetParamNames(names ...string)
    ParamValues() []string
    SetParamValues(values ...string)
    
    // 查詢參數 (Query parameters)
    QueryParam(name string) string
    QueryParams() url.Values
    QueryString() string
    
    // 表單數值
    FormValue(name string) string
    FormParams() (url.Values, error)
    FormFile(name string) (*multipart.FileHeader, error)
    
    // 請求標頭
    Cookie(name string) (*http.Cookie, error)
    SetCookie(cookie *http.Cookie)
    Cookies() []*http.Cookie
    
    // Context 內部儲存
    Get(key string) interface{}
    Set(key string, val interface{})
    
    // 資料綁定與驗證
    Bind(i interface{}) error
    Validate(i interface{}) error
    
    // 回應方法
    JSON(code int, i interface{}) error
    JSONPretty(code int, i interface{}, indent string) error
    JSONBlob(code int, b []byte) error
    JSONByte(code int, b []byte) error
    JSONP(code int, callback string, i interface{}) error
    JSONPBlob(code int, callback string, b []byte) error
    XML(code int, i interface{}) error
    XMLPretty(code int, i interface{}, indent string) error
    XMLBlob(code int, b []byte) error
    HTML(code int, html string) error
    HTMLBlob(code int, b []byte) error
    String(code int, s string) error
    Blob(code int, contentType string, b []byte) error
    Stream(code int, contentType string, r io.Reader) error
    File(fsys fs.FS, name string) error      // 安全：基於 fsys，強制 fs.ValidPath
    FileFS(fsys fs.FS, name string) error     // File 的別名，明確標示 fs.FS
    Attachment(file, name string) error
    Inline(file, name string) error
    NoContent(code int) error
    Redirect(code int, url string) error
    Error(err error)
}
```

### Router 介面
```go
type GortexRouter interface {
    // HTTP 方法
    GET(path string, h HandlerFunc, m ...MiddlewareFunc)
    POST(path string, h HandlerFunc, m ...MiddlewareFunc)
    PUT(path string, h HandlerFunc, m ...MiddlewareFunc)
    DELETE(path string, h HandlerFunc, m ...MiddlewareFunc)
    PATCH(path string, h HandlerFunc, m ...MiddlewareFunc)
    HEAD(path string, h HandlerFunc, m ...MiddlewareFunc)
    OPTIONS(path string, h HandlerFunc, m ...MiddlewareFunc)
    
    // 路由
    Group(prefix string, m ...MiddlewareFunc) GortexRouter
    Use(m ...MiddlewareFunc)
    Routes() []Route
    ServeHTTP(w http.ResponseWriter, r *http.Request)
}
```

### Handler 與 Middleware
```go
// Handler 函式簽章
type HandlerFunc func(Context) error

// Middleware 函式簽章
type MiddlewareFunc func(HandlerFunc) HandlerFunc
```

## Struct Tag 路由

### 基本用法
```go
type HandlersManager struct {
    Home    *HomeHandler    `url:"/"`
    Users   *UserHandler    `url:"/users/:id"`
    Static  *StaticHandler  `url:"/static/*"`
    API     *APIGroup       `url:"/api"`
}
```

### 支援的標籤 (Tags)
- `url:"/path"` - 定義路由路徑
- `middleware:"auth,cors"` - 套用中介軟體（以逗號分隔）
- `hijack:"ws"` - 協議劫持（例如 WebSocket）

### 動態參數
- `:param` - 具名參數（例如 `/users/:id`）
- `*` - 萬用字元（例如 `/static/*`）

### HTTP 方法映射
- `GET()` → GET /path
- `POST()` → POST /path
- `PUT()` → PUT /path
- `DELETE()` → DELETE /path
- `PATCH()` → PATCH /path
- `HEAD()` → HEAD /path
- `OPTIONS()` → OPTIONS /path
- 自訂方法 → POST /path/method-name（例如 `Profile()` → POST /users/:id/profile）

## 應用程式配置

### 建立應用程式
```go
app, err := app.NewApp(
    app.WithConfig(cfg),
    app.WithLogger(logger),
    app.WithHandlers(handlers),
)
```

### 配置選項
```go
type Config struct {
    Server ServerConfig
    Logger LoggerConfig
    // ... 其他配置
}
```

### 執行應用程式
```go
if err := app.Run(); err != nil {
    log.Fatal(err)
}
```

## 中介軟體 (Middleware)

### 內建中介軟體
- RequestID - 自動產生唯一請求 ID
- RateLimit - 依 IP 進行流量限制
- DevErrorPage - 開發環境錯誤頁面
- Logger - 請求/回應日誌記錄
- Recover - Panic 捕捉與恢復

### 自訂中介軟體
```go
func MyMiddleware() middleware.MiddlewareFunc {
    return func(next middleware.HandlerFunc) middleware.HandlerFunc {
        return func(c types.Context) error {
            // Handler 執行前
            err := next(c)
            // Handler 執行後
            return err
        }
    }
}
```

## 錯誤處理

### 業務錯誤
```go
// 註冊錯誤碼
errors.Register(ErrUserNotFound, 404, "User not found")

// 於 Handler 中使用
func (h *UserHandler) GET(c types.Context) error {
    user, err := h.service.GetUser(c.Param("id"))
    if err != nil {
        return err // 框架會自動處理回應
    }
    return c.JSON(200, user)
}
```

### HTTP 錯誤
```go
import httpctx "github.com/yshengliao/gortex/transport/http"

return httpctx.NewHTTPError(404, "Not found")
```

## WebSocket 支援

### WebSocket Handler
```go
import (
    gortexws "github.com/yshengliao/gortex/transport/websocket"
    gorillaws "github.com/gorilla/websocket"
)

type WSHandler struct {
    hub *gortexws.Hub
}

func (h *WSHandler) HandleConnection(c types.Context) error {
    conn, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
    if err != nil {
        return err
    }
    client := gortexws.NewClient(h.hub, conn, clientID, logger)
    h.hub.RegisterClient(client)
    go client.WritePump()
    go client.ReadPump()
    return nil
}
```

Hub 支援讀取大小限制、類型白名單與可插拔的授權器：

```go
hub := gortexws.NewHubWithConfig(logger, gortexws.Config{
    MaxMessageBytes:     4 << 10,
    AllowedMessageTypes: []string{"chat", "ping"},
    Authorizer:          myAuthorizer,
})
```

### WebSocket 的 Struct Tag
```go
type HandlersManager struct {
    WS *WSHandler `url:"/ws" hijack:"ws"`
}
```

## 開發工具功能

當 `Logger.Level = "debug"` 時：
- `GET /_routes` - 列出所有已註冊的路由
- `GET /_monitor` - 系統監控與指標
- `GET /_config` - 檢視配置檔（敏感資訊會被遮蔽）
- 開啟 Request/Response body 日誌

## 回應輔助函式

```go
// 成功回應
response.Success(c, 200, data)
response.Created(c, data)

// 錯誤回應
response.BadRequest(c, "Invalid input")
response.Unauthorized(c, "Login required")
response.Forbidden(c, "Access denied")
response.NotFound(c, "Resource not found")
response.InternalServerError(c, "Server error")
```

## 安全預設值

| 範圍 | 預設行為 | 覆寫方式 |
|------|---------|----------|
| JSON body 大小 | 1 MiB | `BinderConfig.MaxJSONBodyBytes` |
| Multipart body 大小 | 32 MiB | `ContextConfig.MaxMultipartBytes` |
| `Context.File` | 綁定於 `fs.FS`，強制執行 `fs.ValidPath` | `FileDir(dir, name)` 封裝了 `os.DirFS` |
| `Context.Redirect` | 僅允許同源（Same-origin）路徑 | `RedirectOptions.AllowAbsolute` 白名單 |
| CORS | 拒絕 `*` 加上 `AllowCredentials=true` | `CORSWithConfig` 會回傳 `error` |
| 開發錯誤頁面 | 遮蔽 Auth/Secret 標頭與查詢參數 | — |
| 受信任的代理 (Trusted proxies) | 除非連線來自 `LoggerConfig.TrustedProxies`，否則忽略 `X-Forwarded-For` | — |
| JWT Secret | 在 `NewJWTService` 強制 ≥ 32 bytes | — |
| 日誌 Body 遮蔽 | `BodyRedactor` 遮蔽 JSON 敏感資訊 | 自訂 `func([]byte) []byte` |
| CSRF | 在 `middleware/csrf.go` 提供 Synchroniser-token 機制 | `CSRFConfig` |
| Rate limit | 輸出 `X-RateLimit-*` 與 `Retry-After` | `RateLimitConfig` |
| WebSocket | `SetReadLimit(MaxMessageBytes)`、類型白名單、授權勾子 | `websocket.Config` |

如需通報流程與完整加固說明，請參閱根目錄的 [SECURITY.md](../SECURITY.md) 及 [security.md](./security.md)。
