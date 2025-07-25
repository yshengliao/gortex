# Gortex 框架演進計畫 - 從 Echo 遷移到純業務邏輯架構

## 計畫概述

本計畫整合了兩個關鍵目標：
1. **技術層面**：將 Gortex 從 Echo v4 依賴完全遷移到自建 HTTP 框架
2. **架構層面**：實現業務邏輯與協議層的完全分離，讓開發者專注於功能實現

預期成果：
- 減少 25-33% 外部依賴
- 提升 20-40% 路由效能  
- 減少 30-40% Binary 大小
- 開發效率提升 3x（業務邏輯開發時間 < 5 分鐘）

## 階段一：自建 HTTP 框架基礎（1-2 週）

### 1.1 核心路由系統
**目標**：建立高效能的自建路由器，支援現有的宣告式路由

```go
// 保持現有的宣告式風格
type HandlersManager struct {
    Auth    *AuthHandler    `url:"/auth"`
    Health  *HealthHandler  `url:"/health"`
    APIv1   *APIv1Group     `url:"/api/v1"`  // 支援巢狀路由群組
}

type APIv1Group struct {
    Game *GameHandler `url:"/:gameid"`  // 新增：動態路由參數支援
}
```

**技術要求**：
- 路由效能目標：600-800 ns/op
- 支援所有 HTTP 方法
- 反射式路由發現
- 動態路徑參數（`:gameid`, `*wildcard`）
- WebSocket 升級支援

### 1.2 Context 系統
**目標**：輕量級 Context 實作，提供 Echo 相容 API

```go
type Context interface {
    // 請求資料
    Param(name string) string           // 路徑參數
    QueryParam(name string) string      // 查詢參數
    Bind(interface{}) error            // 請求體綁定
    
    // 回應方法
    JSON(code int, i interface{}) error
    String(code int, s string) error
    
    // 進階功能
    Get(key string) interface{}        // Context 值存取
    Set(key string, val interface{})   // Context 值設定
}
```

**效能目標**：
- JSON binding: <500 ns/op
- 參數存取: <50 ns/op
- 零不必要的記憶體分配

### 1.3 中間件系統
**目標**：標準 HTTP 中間件模式，簡化開發

```go
// 標準中間件簽名
type Middleware func(HandlerFunc) HandlerFunc

// 內建中間件
- ErrorHandler    // 統一錯誤處理
- RequestID       // 請求追蹤
- Recovery        // Panic 恢復
- RateLimit       // 速率限制（157 ns/op）
- Compression     // Gzip/Brotli 壓縮
- JWT             // 認證授權
```

### 1.4 向後相容層設計
**目標**：確保現有應用程式零修改即可運行

```go
// Echo 相容層 - 提供完全相同的 API
package compat

import (
    "github.com/labstack/echo/v4"
    "github.com/yshengliao/gortex/router"
)

// EchoAdapter 提供 Echo Context 到 Gortex Context 的轉換
type EchoAdapter struct {
    router *router.Router
}

// WrapEchoHandler 將 Echo Handler 轉換為 Gortex Handler
func WrapEchoHandler(h echo.HandlerFunc) router.HandlerFunc {
    return func(c router.Context) error {
        // 創建 Echo Context 包裝器
        echoCtx := &echoContextWrapper{
            Context: c,
            values:  make(map[string]interface{}),
        }
        return h(echoCtx)
    }
}

// echoContextWrapper 實現 echo.Context 介面
type echoContextWrapper struct {
    router.Context
    values map[string]interface{}
}

// Echo Context 方法實現
func (e *echoContextWrapper) Get(key string) interface{} {
    if val, ok := e.values[key]; ok {
        return val
    }
    return e.Context.Get(key)
}

func (e *echoContextWrapper) Set(key string, val interface{}) {
    e.values[key] = val
    e.Context.Set(key, val)
}

// 雙向運行模式
type RuntimeMode int

const (
    ModeEcho RuntimeMode = iota  // 使用 Echo (預設)
    ModeGortex                   // 使用 Gortex
    ModeDual                     // 雙系統並行（用於測試）
)

// App 配置選項
func WithRuntimeMode(mode RuntimeMode) Option {
    return func(app *App) {
        app.runtimeMode = mode
    }
}
```

**相容性保證**：
1. 所有現有的 Echo Handler 無需修改
2. 中間件可以逐步遷移
3. 可以在運行時切換框架
4. 完整的 API 相容性測試套件

## 階段二：智能參數綁定系統（2-3 週）

### 2.1 自動參數來源推斷
**目標**：根據參數名稱和類型自動判斷資料來源

```go
// 業務方法定義
func (s *GameService) PlaceBet(
    gameID string,      // 自動從路徑參數 /:gameid 提取
    playerID string,    // 自動從 JWT claims 提取
    bet BetRequest,     // 自動從請求體綁定
) (*BetResult, error) {
    // 純業務邏輯，無 HTTP 依賴
}

// Gortex 自動生成的 Handler
func (h *GameHandler) PlaceBet(c Context) error {
    // 自動綁定所有參數
    gameID := c.Param("gameid")
    playerID := c.Get("player_id").(string)  // 從 JWT 中間件設定
    
    var bet BetRequest
    if err := c.Bind(&bet); err != nil {
        return err
    }
    
    // 呼叫業務方法
    result, err := h.service.PlaceBet(gameID, playerID, bet)
    if err != nil {
        return h.errorMapper.ToHTTP(err)
    }
    
    return c.JSON(200, result)
}
```

### 2.2 參數綁定規則引擎
**實作細節**：

```go
// 參數來源映射規則
var paramSourceRules = map[string]ParamSource{
    // 常見參數名稱映射
    "id", "ID":           PathParam,
    "gameID", "gameid":   PathParam,
    "playerID":           JWTClaim,
    "userID":             JWTClaim,
    "token":              Header,
    "limit", "offset":    QueryParam,
    "page", "pageSize":   QueryParam,
    
    // 類型判斷
    // struct{} -> RequestBody
    // []byte -> RequestBody
    // 其他基本類型 -> 根據名稱判斷
}
```

### 2.3 錯誤映射系統
**目標**：業務錯誤自動轉換為適當的 HTTP 回應

```go
// 業務層錯誤定義
var (
    ErrGameNotFound      = errors.New("game not found")
    ErrInsufficientFunds = errors.New("insufficient funds")
    ErrUnauthorized      = errors.New("unauthorized")
)

// Gortex 自動映射配置
type ErrorMapping struct {
    mappings map[error]ErrorResponse{
        ErrGameNotFound:      {Status: 404, Code: "GAME_NOT_FOUND"},
        ErrInsufficientFunds: {Status: 400, Code: "INSUFFICIENT_FUNDS"},
        ErrUnauthorized:      {Status: 401, Code: "UNAUTHORIZED"},
    }
}
```

## 階段三：業務邏輯層架構（1-2 週）

### 3.1 服務層設計原則
**目標**：純業務邏輯，零協議依賴

```go
// services/game_service.go
type GameService struct {
    registry GameRegistry
    storage  Storage
    logger   Logger
}

// 業務方法：清晰的輸入輸出，無 HTTP 概念
func (s *GameService) Authenticate(gameID, token string) (*PlayerSession, error)
func (s *GameService) GetBalance(gameID, playerID string) (*Balance, error)
func (s *GameService) PlaceBet(gameID, playerID string, bet BetRequest) (*BetResult, error)
func (s *GameService) GetHistory(gameID, playerID string, limit, offset int) (*History, error)
```

### 3.2 Handler 自動生成
**目標**：從服務方法自動生成 HTTP Handler

```go
// Gortex 提供的程式碼生成工具
//go:generate gortex-gen handlers

// 自動生成的 Handler
type GameHandler struct {
    service *GameService `inject:"gameService"`
}

// 所有 HTTP 處理細節自動生成
func (h *GameHandler) PlaceBet(c Context) error {
    // 參數綁定、錯誤處理、回應格式化都自動處理
}
```

### 3.3 依賴注入增強
**目標**：簡化服務組裝和生命週期管理

```go
// 使用 Gortex DI 容器
app := gortex.New()

// 註冊服務
app.Register("gameService", func() *GameService {
    return &GameService{
        registry: app.MustResolve("gameRegistry"),
        storage:  app.MustResolve("storage"),
        logger:   app.MustResolve("logger"),
    }
})

// 自動注入到 Handler
type GameHandler struct {
    service *GameService `inject:"gameService"`
}
```

## 階段四：實際專案遷移（1 週）

### 4.1 tshttproj 漸進式遷移
**策略**：保持現有功能，逐步分離業務邏輯

```go
// Step 1: 保持現有路由結構
type HandlersManager struct {
    Auth   *AuthHandler   `url:"/auth"`
    Demo   *DemoHandler   `url:"/auth/demo"`
    Health *HealthHandler `url:"/health"`
    APIv1  *APIv1Group    `url:"/api/v1"`
}

// Step 2: 抽離業務邏輯到服務層
type GameService struct {
    // 整合現有的 adapter 邏輯
}

// Step 3: Handler 變成薄層
type GameHandler struct {
    service *GameService
}

func (h *GameHandler) Bet(c gortex.Context) error {
    // 只負責參數綁定和呼叫服務
    gameID := c.Param("gameid")
    // ... 最少的 HTTP 處理邏輯
}
```

### 4.2 測試策略轉換
**目標**：從整合測試為主轉向單元測試為主

```go
// 以前：需要模擬 HTTP 請求
func TestBetEndpoint(t *testing.T) {
    req := httptest.NewRequest(POST, "/api/v1/sg006/bet", body)
    rec := httptest.NewRecorder()
    // ... 複雜的 HTTP 測試
}

// 現在：直接測試業務邏輯
func TestPlaceBet(t *testing.T) {
    service := &GameService{registry: mockRegistry}
    result, err := service.PlaceBet("sg006", "player123", BetRequest{Amount: 100})
    assert.NoError(t, err)
    assert.Equal(t, 100, result.BetAmount)
}
```

### 4.3 相容性測試套件
**目標**：確保遷移過程中功能完全一致

```go
// 相容性測試框架
package compat_test

// 測試相同的 API 在兩個框架下的行為
func TestAPICompatibility(t *testing.T) {
    tests := []struct {
        name     string
        setup    func(*testing.T) (echoApp, gortexApp)
        request  *http.Request
        validate func(*testing.T, echoResp, gortexResp)
    }{
        {
            name: "JSON binding",
            // 測試 JSON 綁定行為一致
        },
        {
            name: "Error handling",
            // 測試錯誤處理一致
        },
        {
            name: "Middleware execution order",
            // 測試中間件執行順序一致
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            echoApp, gortexApp := tt.setup(t)
            
            // 執行相同請求
            echoResp := executeEcho(echoApp, tt.request)
            gortexResp := executeGortex(gortexApp, tt.request)
            
            // 驗證結果一致
            tt.validate(t, echoResp, gortexResp)
        })
    }
}

// A/B 測試工具
type ABTestRunner struct {
    echoApp   *echo.Echo
    gortexApp *gortex.App
    mode      RuntimeMode
}

// 並行運行請求，比較結果
func (r *ABTestRunner) Execute(req *http.Request) error {
    if r.mode == ModeDual {
        // 同時運行兩個系統
        ch1 := make(chan response)
        ch2 := make(chan response)
        
        go func() { ch1 <- r.runEcho(req) }()
        go func() { ch2 <- r.runGortex(req) }()
        
        resp1 := <-ch1
        resp2 := <-ch2
        
        // 比較結果
        if !compareResponses(resp1, resp2) {
            return fmt.Errorf("response mismatch")
        }
    }
    return nil
}
```

## 階段五：進階功能和最佳化（2-3 週）

### 5.1 自動 API 文檔生成
**目標**：從業務方法自動生成 OpenAPI 規範

```go
// 使用註解定義 API
// @Summary 玩家下注
// @Description 處理玩家的下注請求
// @Accept json
// @Produce json
// @Param gameID path string true "遊戲ID"
// @Param bet body BetRequest true "下注資訊"
// @Success 200 {object} BetResult "下注結果"
// @Failure 400 {object} ErrorResponse "請求錯誤"
func (s *GameService) PlaceBet(gameID, playerID string, bet BetRequest) (*BetResult, error)
```

### 5.2 效能最佳化
**目標**：達到或超越原始效能指標

```go
// 物件池減少 GC 壓力
var contextPool = sync.Pool{
    New: func() interface{} {
        return &httpContext{}
    },
}

// 預編譯的路由樹
type compiledRouter struct {
    staticRoutes  map[string]HandlerFunc  // O(1) 查找
    dynamicRoutes []*routeNode            // 優化的樹結構
}

// 零分配的參數提取
func (c *httpContext) Param(name string) string {
    // 使用預分配的緩衝區，避免字串分配
}
```

### 5.3 開發者體驗增強
**工具和功能**：

```go
// 1. CLI 工具
gortex new project myapp
gortex generate handler GameService
gortex generate tests GameService

// 2. 熱重載支援
gortex dev --watch

// 3. 除錯模式增強
// 自動產生 /_debug 端點
// 顯示路由樹、中間件鏈、效能統計
```

## 實施時間表

### 第 1-2 週：基礎框架（保持向後相容）
- [ ] Echo 相容層設計（1-2 天）
- [ ] 自建 HTTP 路由器（3-4 天）
- [ ] Context 系統實作（2-3 天）
- [ ] 相容性測試框架（1-2 天）
- [ ] 中間件遷移（3-4 天）

### 第 3-4 週：智能綁定
- [ ] 參數綁定引擎（3-4 天）
- [ ] 錯誤映射系統（2-3 天）
- [ ] Handler 生成器原型（3-4 天）
- [ ] A/B 測試工具（1-2 天）

### 第 5 週：專案遷移
- [ ] tshttproj 業務邏輯分離（2-3 天）
- [ ] 測試策略調整（1-2 天）
- [ ] 效能驗證（1-2 天）
- [ ] 雙系統並行測試（1 天）

### 第 6-7 週：最佳化和完善
- [ ] API 文檔自動生成（2-3 天）
- [ ] 效能最佳化（3-4 天）
- [ ] 開發工具完善（2-3 天）
- [ ] 文檔和範例更新（1-2 天）
- [ ] 完整相容性驗證（1-2 天）

## 成功指標

### 技術指標
- **路由效能**：600-800 ns/op（提升 20-40%）
- **Binary 大小**：減少 30-40%
- **記憶體使用**：減少 20-35%
- **依賴數量**：12 → 8-9 個

### 開發效率指標
- **新增 API 時間**：< 5 分鐘
- **測試覆蓋率**：> 90%（業務邏輯）
- **程式碼行數**：減少 60-80%（HTTP 處理相關）

### 品質指標
- **關注點分離**：業務邏輯零 HTTP 依賴
- **可測試性**：純函數設計，易於單元測試
- **可維護性**：清晰的分層架構

## 風險管理

### 技術風險與緩解策略
1. **效能退化**
   - 緩解：持續基準測試，設定效能紅線
   - 監控：每個 commit 執行效能測試
   - 回滾：保留 Echo 模式作為緊急回滾選項

2. **相容性問題**
   - 緩解：完整的 Echo API 相容層
   - 測試：A/B 測試確保行為一致
   - 遷移：支援漸進式、可逆的遷移路徑

3. **功能遺漏**
   - 緩解：完整的測試覆蓋，包含邊界情況
   - 驗證：雙系統並行運行對比
   - 文檔：詳細記錄所有 API 差異

### 專案風險與管理
1. **時程延誤**
   - 管理：分階段交付，每階段可獨立運作
   - 里程碑：每週交付可用版本
   - 緩衝：保留 20% 時間緩衝

2. **團隊接受度**
   - 管理：提供培訓和詳細文檔
   - 溝通：定期分享進度和效益
   - 支援：建立遷移支援小組

3. **生產穩定性**
   - 管理：灰度發布策略
   - 監控：完整的監控和告警
   - 回滾：5 分鐘內回滾能力

### 向後相容性承諾
1. **零破壞性變更**：現有程式碼無需修改即可運行
2. **漸進式遷移**：可以逐個模組遷移
3. **雙向運行**：支援新舊系統並行
4. **完整測試覆蓋**：確保行為完全一致

## 長期願景

### Gortex 定位
- **輕量級**：最小依賴，快速啟動
- **高效能**：接近原生 HTTP 效能
- **開發友善**：專注業務邏輯，框架處理其餘
- **可擴展**：插件系統，自定義中間件

### 生態系統
- **程式碼生成器**：自動化重複工作
- **開發工具**：CLI、IDE 插件
- **範例專案**：各種使用場景的最佳實踐
- **社群支援**：文檔、教程、討論區

---

**最後更新**：2025/07/25  
**狀態**：規劃階段  
**下一步**：開始實作自建 HTTP 路由器