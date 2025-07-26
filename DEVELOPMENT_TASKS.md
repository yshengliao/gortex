# Gortex 開發任務清單

基於 DEVELOPER_EXPERIENCE_PLAN.md 的實施任務拆解，每個任務包含明確的 commit 範例和 AI 開發提示詞。

## 📋 任務總覽

### Phase 1: 核心功能（優先級：高）
1. ✅ [AUTO-INIT] Handler 自動初始化功能 (2025-07-26)
2. ✅ [ROUTES-LOG] 路由日誌系統 (2025-07-26)
3. ✅ [CTX-HELPER] Context 輔助方法 (2025-07-26)

### Phase 2: 開發體驗（優先級：中）
4. [DEV-MODE] 開發模式增強
5. [ERROR-PAGE] 友善錯誤頁面

### Phase 3: 進階特性（優先級：中）
6. [STRUCT-TAGS] 進階 Struct Tags 系統
7. [PERF-OPT] 基礎效能優化

---

## 🎯 Task 1: Handler 自動初始化功能 ✅

**Status**: Completed (2025-07-26)
**Commit**: 5a17544

### Commit 範例
```bash
git commit -m "feat(core): implement auto handler initialization

- Add autoInitHandlers function to recursively initialize nil handlers
- Integrate with WithHandlers option for automatic initialization
- Support nested handler groups
- Add comprehensive tests for edge cases

This eliminates the need for manual handler initialization, making the
framework more developer-friendly.

Closes #AUTO-INIT"
```

### AI 開發提示詞
```
請實作 Gortex 框架的 Handler 自動初始化功能。

技術需求：
1. 在 app/router.go 中實作 autoInitHandlers 函數
2. 使用反射遞迴處理所有 struct 欄位
3. 只初始化 nil 的指標欄位
4. 支援任意深度的嵌套結構
5. 在 WithHandlers 選項中調用此功能

實作要點：
- 使用 reflect.Value 和 reflect.Type
- 檢查 CanSet() 避免 panic
- 處理非導出欄位
- 保持現有 API 不變

測試案例：
- 簡單 handler 初始化
- 嵌套 handler groups
- 部分已初始化的情況
- 循環引用檢測

參考 Go 的 Effective Go 原則，保持代碼簡潔清晰。
```

---

## 🎯 Task 2: 路由日誌系統 ✅

**Status**: Completed (2025-07-26)
**Commit**: eec084d

### Commit 範例
```bash
git commit -m "feat(app): add automatic route logging system

- Add RouteInfo struct to store route metadata
- Implement route collection during registration
- Add WithRoutesLogger option for enabling logging
- Support table and simple output formats
- Include middleware information in route display

Provides better visibility into registered routes during development.

Closes #ROUTES-LOG"
```

### AI 開發提示詞
```
請實作 Gortex 框架的智慧路由日誌系統。

功能需求：
1. 在路由註冊時自動收集路由信息
2. 支援美化的表格輸出格式
3. 顯示 Method、Path、Handler、Middlewares
4. 提供 WithRoutesLogger() 選項啟用

資料結構：
```go
type RouteInfo struct {
    Method      string
    Path        string
    Handler     string
    Middlewares []string
}
```

輸出格式範例：
┌────────┬─────────────────────────┬─────────────────────┬──────────────┐
│ Method │ Path                    │ Handler             │ Middlewares  │
├────────┼─────────────────────────┼─────────────────────┼──────────────┤
│ GET    │ /users/:id              │ UserHandler         │ auth         │
└────────┴─────────────────────────┴─────────────────────┴──────────────┘

注意保持輸出的可讀性和對齊。
```

---

## 🎯 Task 3: Context 輔助方法 ✅

**Status**: Completed (2025-07-26)
**Commit**: (pending)

### Commit 範例
```bash
git commit -m "feat(context): add helper methods for better DX

- Add ParamInt, QueryInt with default values
- Add OK, Created, BadRequest convenience methods
- Implement smart parameter binding
- Follow Effective Go principles

Makes common operations more concise and developer-friendly.

Closes #CTX-HELPER"
```

### AI 開發提示詞
```
請為 Gortex Context 新增輔助方法，提升開發體驗。

新增方法：
1. 參數類型轉換
   - ParamInt(name string, defaultValue int) int
   - QueryInt(name string, defaultValue int) int
   - QueryBool(name string, defaultValue bool) bool

2. 便利回應方法
   - OK(data interface{}) error // 200
   - Created(data interface{}) error // 201
   - NoContent() error // 204
   - BadRequest(message string) error // 400

3. 改進 Bind 方法
   - 自動處理 JSON/Form 內容類型
   - 提供清晰的錯誤訊息

設計原則：
- 方法名稱簡潔明瞭
- 提供合理的預設值
- 錯誤訊息要友善
- 遵循 Go 慣例

保持與現有 API 的一致性。
```

---

## 🎯 Task 4: 開發模式增強

### Commit 範例
```bash
git commit -m "feat(app): enhance development mode experience

- Add development mode detection and hints
- Show available debug endpoints on startup
- Add hot reload suggestions
- Improve startup logging with emojis
- Add WithDevelopmentMode option

Makes development more pleasant and informative.

Closes #DEV-MODE"
```

### AI 開發提示詞
```
請實作 Gortex 的開發模式增強功能。

功能需求：
1. 啟動時顯示友善提示
2. 列出所有 debug 端點
3. 建議安裝 hot reload 工具
4. 使用 emoji 讓輸出更友善

實作內容：
```go
if cfg.IsDevelopment() {
    logger.Info("🔥 Development mode enabled!")
    logger.Info("📝 Available debug endpoints:")
    logger.Info("   • /_routes   - View all routes")
    logger.Info("   • /_config   - View configuration")
    logger.Info("   • /_monitor  - System metrics")
    logger.Info("💡 Tip: Install air for hot reload")
}
```

新增 WithDevelopmentMode() 選項，自動設定：
- Logger level = "debug"
- 啟用所有 debug 端點
- 顯示詳細錯誤信息

保持專業但友善的語調。
```

---

## 🎯 Task 5: 友善錯誤頁面

### Commit 範例
```bash
git commit -m "feat(middleware): improve error page for development

- Enhance error page with better styling
- Add stack trace visualization
- Include request details
- Suggest possible solutions
- Auto-detect JSON/HTML response format

Provides more helpful error information during development.

Closes #ERROR-PAGE"
```

### AI 開發提示詞
```
請改進 Gortex 的開發模式錯誤頁面。

功能需求：
1. 美化的 HTML 錯誤頁面（開發模式）
2. 清晰的堆疊追蹤顯示
3. 請求詳情（headers、params、body）
4. 可能的解決方案建議
5. 自動判斷回應格式（HTML/JSON）

錯誤頁面應包含：
- 錯誤標題和訊息
- 堆疊追蹤（高亮關鍵行）
- 請求信息（隱藏敏感資料）
- 相關文檔連結

樣式要求：
- 使用內嵌 CSS（無外部依賴）
- 深色主題友善
- 響應式設計
- 代碼高亮

參考 Laravel 或 Django 的錯誤頁面設計。
```

---

## 🎯 Task 6: 進階 Struct Tags 系統

### Commit 範例
```bash
git commit -m "feat(core): add advanced struct tags support

- Add inject tag for dependency injection
- Support middleware composition via tags
- Add ratelimit tag for rate limiting
- Prepare for future cache tag support
- Follow Spring philosophy with Go simplicity

Enables declarative programming while maintaining Go idioms.

Closes #STRUCT-TAGS"
```

### AI 開發提示詞
```
請實作 Gortex 的進階 struct tags 系統。

支援的 tags：
1. inject:"" - 依賴注入
   ```go
   type Handler struct {
       DB *sql.DB `inject:""`
   }
   ```

2. middleware:"auth,rbac" - 組合多個中間件
   ```go
   type AdminAPI struct{} `url:"/admin" middleware:"auth,rbac"`
   ```

3. ratelimit:"100/min" - 限流控制
   ```go
   type PublicAPI struct{} `url:"/public" ratelimit:"100/min"`
   ```

實作要點：
- 在 processHandlers 時解析 tags
- 保持標籤語法簡單
- 提供清晰的錯誤訊息
- 所有行為必須透明可預測

設計原則：
- 借鑒 Spring 但保持 Go 風格
- 顯式優於隱式
- 無隱藏魔法

測試各種 tag 組合情況。
```

---

## 🎯 Task 7: 基礎效能優化

### Commit 範例
```bash
git commit -m "perf(core): implement basic performance optimizations

- Add context pool to reduce allocations
- Implement smart parameter storage
- Use sync.Pool for reusable objects
- Maintain simple API while improving performance

Reduces memory allocations without compromising developer experience.

Closes #PERF-OPT"
```

### AI 開發提示詞
```
請實作 Gortex 的基礎效能優化，保持 API 簡單。

優化項目：
1. Context Pool
   ```go
   var ctxPool = sync.Pool{
       New: func() interface{} {
           return &gortexContext{
               params: make(map[string]string, 4),
           }
       },
   }
   ```

2. 智慧參數存儲
   ```go
   type smartParams struct {
       count  int
       keys   [4]string    // 小陣列優化
       values [4]string
       overflow map[string]string
   }
   ```

實作要求：
- 對開發者完全透明
- 不改變現有 API
- 適當的預分配大小
- 正確的 pool 清理

效能目標：
- 減少 50% 記憶體分配
- 保持程式碼可讀性
- 不增加使用複雜度

記住：可讀性優於微小的效能提升。
```

---

## 📊 任務執行建議

### 執行順序
1. 先完成 Task 1-3（核心功能）
2. 再進行 Task 4-5（開發體驗）
3. 最後實作 Task 6-7（進階特性）

### 每個任務的驗收標準
- ✅ 功能完整實現
- ✅ 單元測試覆蓋 > 80%
- ✅ 更新相關文檔
- ✅ 範例程式可正常運行
- ✅ 無破壞性變更

### Git 工作流程
```bash
# 為每個任務創建分支
git checkout -b feature/auto-handler-init
# 完成後合併到 feature/auto-handler-init 主分支
git checkout feature/auto-handler-init
git merge --no-ff feature/auto-handler-init
```

---

## 🤖 AI 協作最佳實踐

### 提供上下文
每次開始新任務時，提供：
1. 當前任務編號和名稱
2. 相關的提示詞
3. 現有代碼結構
4. 預期的行為

### 迭代開發
1. 先實現基本功能
2. 加入錯誤處理
3. 優化效能
4. 補充測試
5. 更新文檔

### 程式碼品質
- 遵循 Effective Go
- 保持簡潔性
- 適當的註釋
- 一致的命名

---

## 📝 文檔更新檢查清單

每個任務完成後需更新：
- [ ] API.md - API 參考文檔
- [ ] README.md - 使用範例
- [ ] CHANGELOG.md - 變更記錄
- [ ] Examples - 範例程式

---

此任務清單將隨著開發進展持續更新。