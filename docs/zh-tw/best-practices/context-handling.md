# Context 處理最佳實踐

## 總覽

Context（上下文）傳播是建立穩健 Go 應用程式的關鍵環節。在 Gortex 中，正確的 Context 處理能確保優雅關機（Graceful shutdown）、請求取消以及超時（Timeout）管理。本指南涵蓋了在 Gortex 應用程式中有效使用 `context.Context` 的最佳實踐。

## 目錄

1. [Context 生命週期管理](#context-生命週期管理)
2. [取消訊號傳播](#取消訊號傳播)
3. [常見的 Context 洩漏模式](#常見的-context-洩漏模式)
4. [與 Goroutines 協作](#與-goroutines-協作)
5. [超時最佳實踐](#超時最佳實踐)
6. [實戰範例：HTTP 請求追蹤](#實戰範例-http-請求追蹤)
7. [效能考量](#效能考量)
8. [疑難排解](#疑難排解)

## Context 生命週期管理

### 理解 Context 階層關係

Gortex 中的每個 Context 都遵循父子關係。當父 Context 被取消時，其所有的子 Context 也會自動被取消。

```go
// 範例 1：建立子 Context
func (h *UserHandler) GetUserWithDetails(c httpctx.Context) error {
    // 來自 HTTP 請求的父 Context
    parentCtx := c.Request().Context()
    
    // 為資料庫查詢建立帶有超時的子 Context
    dbCtx, cancel := context.WithTimeout(parentCtx, 5*time.Second)
    defer cancel() // 永遠使用 defer 呼叫 cancel 以避免 Context 洩漏
    
    // 使用子 Context 進行資料庫操作
    user, err := h.db.GetUser(dbCtx, c.Param("id"))
    if err != nil {
        return c.JSON(500, map[string]string{"error": "Database error"})
    }
    
    // 為外部 API 呼叫建立另一個子 Context
    apiCtx, apiCancel := context.WithTimeout(parentCtx, 3*time.Second)
    defer apiCancel()
    
    details, err := h.api.GetUserDetails(apiCtx, user.ID)
    if err != nil {
        // 記錄錯誤但不中斷請求
        h.logger.Warn("Failed to fetch user details", zap.Error(err))
    }
    
    return c.JSON(200, map[string]interface{}{
        "user":    user,
        "details": details,
    })
}
```

### Context 建立的最佳實踐

1. **永遠使用請求 Context 作為父節點**：以請求 Context 為起點，確保正確的取消傳播。
2. **設定適當的超時時間**：不同的操作需要不同的超時設定。
3. **永遠呼叫 cancel**：使用 `defer` 確保 `cancel` 一定會被呼叫。

```go
// 範例 2：Context 建立模式
func (h *Handler) ProcessRequest(c httpctx.Context) error {
    ctx := c.Request().Context()
    
    // 模式 1：短時間操作
    shortCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
    defer cancel()
    
    // 模式 2：可取消的操作
    cancelCtx, cancel := context.WithCancel(ctx)
    defer cancel()
    
    // 模式 3：帶有期限 (Deadline) 的操作
    deadline := time.Now().Add(30 * time.Second)
    deadlineCtx, cancel := context.WithDeadline(ctx, deadline)
    defer cancel()
    
    return nil
}
```

## 取消訊號傳播

### 檢查 Context 是否被取消

在長時間執行的操作中，應隨時檢查 Context 是否已被取消：

```go
// 範例 3：正確的取消檢查
func (h *DataProcessor) ProcessLargeDataset(ctx context.Context, data []Item) error {
    for i, item := range data {
        // 在每次迭代開始時檢查取消訊號
        select {
        case <-ctx.Done():
            return fmt.Errorf("processing cancelled at item %d: %w", i, ctx.Err())
        default:
            // 繼續處理
        }
        
        // 處理項目
        if err := h.processItem(ctx, item); err != nil {
            return fmt.Errorf("failed to process item %d: %w", i, err)
        }
        
        // 對於極長的操作，可以增加檢查頻率
        if i%100 == 0 {
            select {
            case <-ctx.Done():
                return ctx.Err()
            default:
            }
        }
    }
    return nil
}
```

### 透過 Channel 傳播 Context

使用 Channel 時，永遠將 Context 納入 select 中以支援取消：

```go
// 範例 4：Context 與 Channels
func (h *WorkerPool) ProcessJobs(ctx context.Context, jobs <-chan Job) error {
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case job, ok := <-jobs:
            if !ok {
                return nil // Channel 已關閉
            }
            
            // 帶入 Context 處理任務
            if err := h.processJob(ctx, job); err != nil {
                h.logger.Error("Job processing failed", 
                    zap.String("job_id", job.ID),
                    zap.Error(err))
            }
        }
    }
}
```

## 常見的 Context 洩漏模式

### 模式 1：忘記呼叫 Cancel

```go
// ❌ 錯誤：Context 洩漏 - 從未呼叫 cancel
func BadExample(ctx context.Context) {
    newCtx, _ := context.WithTimeout(ctx, 5*time.Second)
    doSomething(newCtx)
    // 漏了 cancel() 呼叫 - Context 將會洩漏！
}

// ✅ 正確：妥善清理
func GoodExample(ctx context.Context) {
    newCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel() // 建立後立即 defer cancel
    doSomething(newCtx)
}
```

### 模式 2：將 Context 存入 Struct

```go
// ❌ 錯誤：將 Context 存入 struct 欄位
type BadService struct {
    ctx context.Context // 不要這樣做！
}

// ✅ 正確：將 Context 當作參數傳遞
type GoodService struct {
    logger *zap.Logger
}

func (s *GoodService) DoWork(ctx context.Context) error {
    // 使用傳入的 Context
    return nil
}
```

### 模式 3：使用錯誤的 Context

```go
// 範例 5：使用正確的 Context
func (h *Handler) ComplexOperation(c httpctx.Context) error {
    // ❌ 錯誤：使用 background context 會忽略取消訊號
    badResult, err := h.service.Process(context.Background(), data)
    
    // ✅ 正確：使用請求 Context 以傳遞取消訊號
    ctx := c.Request().Context()
    goodResult, err := h.service.Process(ctx, data)
    
    if err != nil {
        return c.JSON(500, map[string]string{"error": err.Error()})
    }
    
    return c.JSON(200, goodResult)
}
```

## 與 Goroutines 協作

### 傳遞 Context 給 Goroutines

永遠將 Context 傳遞給 Goroutines 並處理取消：

```go
// 範例 6：Context 與 goroutines
func (h *AsyncHandler) ProcessAsync(c httpctx.Context) error {
    ctx := c.Request().Context()
    results := make(chan Result, 10)
    errors := make(chan error, 10)
    
    // 啟動 workers
    var wg sync.WaitGroup
    for i := 0; i < 5; i++ {
        wg.Add(1)
        go func(workerID int) {
            defer wg.Done()
            
            // Worker 尊重 Context 取消訊號
            for {
                select {
                case <-ctx.Done():
                    h.logger.Info("Worker shutting down", 
                        zap.Int("worker_id", workerID))
                    return
                    
                case job := <-h.jobQueue:
                    result, err := h.processJob(ctx, job)
                    if err != nil {
                        errors <- err
                        continue
                    }
                    results <- result
                }
            }
        }(i)
    }
    
    // 等待完成或超時
    done := make(chan struct{})
    go func() {
        wg.Wait()
        close(done)
    }()
    
    select {
    case <-done:
        return c.JSON(200, map[string]string{"status": "completed"})
    case <-ctx.Done():
        return c.JSON(408, map[string]string{"error": "request timeout"})
    }
}
```

### 優雅關閉 Goroutine (Graceful Shutdown)

```go
// 範例 7：優雅關閉模式
type Worker struct {
    logger *zap.Logger
    jobs   chan Job
}

func (w *Worker) Start(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            w.logger.Info("Worker received shutdown signal")
            w.cleanup()
            return
            
        case job := <-w.jobs:
            // 建立特定任務的超時
            jobCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
            
            err := w.processJob(jobCtx, job)
            cancel() // 清理任務 Context
            
            if err != nil {
                if errors.Is(err, context.DeadlineExceeded) {
                    w.logger.Warn("Job timeout", zap.String("job_id", job.ID))
                } else {
                    w.logger.Error("Job failed", zap.Error(err))
                }
            }
        }
    }
}
```

## 超時最佳實踐

### 設定適當的超時時間

不同的操作需要不同的超時策略：

```go
// 範例 8：超時策略
func (h *TimeoutHandler) DemoTimeouts(c httpctx.Context) error {
    parentCtx := c.Request().Context()
    
    // 快速快取查詢 - 短超時
    cacheCtx, cancel := context.WithTimeout(parentCtx, 50*time.Millisecond)
    defer cancel()
    
    if cached, err := h.cache.Get(cacheCtx, "key"); err == nil {
        return c.JSON(200, cached)
    }
    
    // 資料庫查詢 - 中等超時
    dbCtx, cancel := context.WithTimeout(parentCtx, 2*time.Second)
    defer cancel()
    
    data, err := h.db.Query(dbCtx, "SELECT ...")
    if err != nil {
        return c.JSON(500, map[string]string{"error": "Database error"})
    }
    
    // 外部 API 呼叫 - 較長超時
    apiCtx, cancel := context.WithTimeout(parentCtx, 10*time.Second)
    defer cancel()
    
    enriched, err := h.api.EnrichData(apiCtx, data)
    if err != nil {
        // 若 enrichment 失敗則不中斷流程
        h.logger.Warn("Enrichment failed", zap.Error(err))
        enriched = data
    }
    
    // 非同步更新快取 - 射後不理並帶有超時
    go func() {
        cacheUpdateCtx, cancel := context.WithTimeout(
            context.Background(), // 非同步操作使用 background
            5*time.Second,
        )
        defer cancel()
        
        if err := h.cache.Set(cacheUpdateCtx, "key", enriched); err != nil {
            h.logger.Error("Cache update failed", zap.Error(err))
        }
    }()
    
    return c.JSON(200, enriched)
}
```

### 處理超時錯誤

```go
// 範例 9：正確處理超時錯誤
func (h *Handler) HandleTimeouts(ctx context.Context) error {
    result, err := h.service.SlowOperation(ctx)
    if err != nil {
        switch {
        case errors.Is(err, context.DeadlineExceeded):
            return fmt.Errorf("operation timed out after %v", h.timeout)
        case errors.Is(err, context.Canceled):
            return fmt.Errorf("operation was cancelled")
        default:
            return fmt.Errorf("operation failed: %w", err)
        }
    }
    return nil
}
```

## 實戰範例：HTTP 請求追蹤

這是一個完整的範例，展示 Context 在帶有追蹤（Tracing）的 HTTP 請求中的傳播：

```go
// 範例 10：完整的請求追蹤範例
package handlers

import (
    "context"
    "database/sql"
    "time"
    
    "github.com/yshengliao/gortex/transport/http"
    "github.com/yshengliao/gortex/observability/tracing"
    "go.uber.org/zap"
)

type OrderHandler struct {
    db       *sql.DB
    logger   *zap.Logger
    tracer   tracing.EnhancedTracer
    cache    Cache
    payment  PaymentService
    shipping ShippingService
}

func (h *OrderHandler) CreateOrder(c http.Context) error {
    // 取得請求 Context
    ctx := c.Request().Context()
    
    // 若開啟 tracing，則從 context 提取 span
    span := c.Span()
    if span != nil {
        span.AddTags(map[string]interface{}{
            "handler": "CreateOrder",
            "user_id": c.Header("X-User-ID"),
        })
    }
    
    // 解析請求
    var req CreateOrderRequest
    if err := c.Bind(&req); err != nil {
        if span != nil {
            span.SetError(err)
        }
        return c.JSON(400, map[string]string{"error": "Invalid request"})
    }
    
    // 驗證並加上超時
    validateCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
    defer cancel()
    
    if err := h.validateOrder(validateCtx, &req); err != nil {
        if span != nil {
            span.LogEvent(tracing.WARN, "Validation failed", map[string]any{
                "error": err.Error(),
            })
        }
        return c.JSON(400, map[string]string{"error": err.Error()})
    }
    
    // 開啟資料庫交易
    tx, err := h.db.BeginTx(ctx, nil)
    if err != nil {
        if span != nil {
            span.SetError(err)
        }
        return c.JSON(500, map[string]string{"error": "Database error"})
    }
    defer tx.Rollback()
    
    // 建立訂單並加上超時
    orderCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
    defer cancel()
    
    order, err := h.createOrderTx(orderCtx, tx, &req)
    if err != nil {
        if span != nil {
            span.SetError(err)
        }
        return c.JSON(500, map[string]string{"error": "Failed to create order"})
    }
    
    // 非同步處理付款，使用獨立 Context
    paymentCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()
    
    paymentResult := make(chan PaymentResult, 1)
    go func() {
        result, err := h.payment.Process(paymentCtx, order)
        paymentResult <- PaymentResult{Result: result, Error: err}
    }()
    
    // 帶超時等待付款完成
    select {
    case <-ctx.Done():
        // 請求被取消，嘗試取消付款
        go h.payment.Cancel(context.Background(), order.ID)
        return c.JSON(408, map[string]string{"error": "Request timeout"})
        
    case result := <-paymentResult:
        if result.Error != nil {
            if span != nil {
                span.LogEvent(tracing.ERROR, "Payment failed", map[string]any{
                    "order_id": order.ID,
                    "error":    result.Error.Error(),
                })
            }
            return c.JSON(402, map[string]string{"error": "Payment failed"})
        }
        
        // 更新訂單狀態
        updateCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
        defer cancel()
        
        if err := h.updateOrderStatus(updateCtx, tx, order.ID, "paid"); err != nil {
            // 記錄但不中斷 - 可後續重試
            h.logger.Error("Failed to update order status",
                zap.String("order_id", order.ID),
                zap.Error(err))
        }
    }
    
    // 提交交易
    if err := tx.Commit(); err != nil {
        if span != nil {
            span.SetError(err)
        }
        return c.JSON(500, map[string]string{"error": "Failed to finalize order"})
    }
    
    // 回應後的非同步操作
    go func() {
        // 為非同步操作建立新 Context
        asyncCtx, cancel := context.WithTimeout(
            context.Background(),
            5*time.Minute,
        )
        defer cancel()
        
        // 更新快取
        cacheCtx, cacheCancel := context.WithTimeout(asyncCtx, 1*time.Second)
        h.cache.Set(cacheCtx, order.CacheKey(), order)
        cacheCancel()
        
        // 排程物流
        shipCtx, shipCancel := context.WithTimeout(asyncCtx, 2*time.Minute)
        h.shipping.Schedule(shipCtx, order)
        shipCancel()
    }()
    
    if span != nil {
        span.LogEvent(tracing.INFO, "Order created successfully", map[string]any{
            "order_id": order.ID,
            "total":    order.Total,
        })
    }
    
    return c.JSON(201, order)
}
```

## 效能考量

### Context 變數儲存

避免在 Context 中儲存過大的變數：

```go
// ❌ 錯誤：在 Context 儲存大物件
ctx = context.WithValue(ctx, "user", largeUserObject)

// ✅ 正確：僅儲存識別碼 (Identifiers)
ctx = context.WithValue(ctx, "user_id", userID)
```

### Context Key 最佳實踐

```go
// 定義具有型別的 Key 避免衝突
type contextKey string

const (
    userIDKey     contextKey = "user_id"
    requestIDKey  contextKey = "request_id"
    spanKey       contextKey = "span"
)

// 具備型別安全的輔助函式
func WithUserID(ctx context.Context, userID string) context.Context {
    return context.WithValue(ctx, userIDKey, userID)
}

func GetUserID(ctx context.Context) (string, bool) {
    userID, ok := ctx.Value(userIDKey).(string)
    return userID, ok
}
```

## 疑難排解

### 常見問題與解法

1. **Context Deadline Exceeded 錯誤**
   - 檢查超時設定是否符合操作需求。
   - 確認網路延遲與依賴服務的回應時間。
   - 考慮實作帶有退避機制 (Backoff) 的重試邏輯。

2. **Goroutine 洩漏 (Leaks)**
   - 在迴圈中永遠要檢查 Context 取消訊號。
   - 建立 Context 後立即使用 `defer cancel()`。
   - 監控生產環境的 Goroutine 數量。

3. **Context 傳播失敗**
   - 確認傳遞了正確的 Context。
   - 請求 Handler 中**不要**使用 `context.Background()`。
   - 檢查中介軟體是否有正確傳播 Context。

### 除錯 Context 問題

```go
// 輔助函式：追蹤 Context 被取消的時機
func DebugContext(ctx context.Context, name string) {
    go func() {
        <-ctx.Done()
        fmt.Printf("Context %s cancelled: %v\n", name, ctx.Err())
    }()
}
```

### 測試 Context 處理

```go
func TestContextCancellation(t *testing.T) {
    // 建立可取消的 Context
    ctx, cancel := context.WithCancel(context.Background())
    
    // 開始操作
    done := make(chan error, 1)
    go func() {
        done <- longRunningOperation(ctx)
    }()
    
    // 延遲後取消
    time.Sleep(100 * time.Millisecond)
    cancel()
    
    // 驗證取消是否有被處理
    err := <-done
    assert.ErrorIs(t, err, context.Canceled)
}
```

## 總結

正確處理 Context 是建立穩健 Gortex 應用程式的基礎。核心重點：

1. 永遠從 HTTP 請求開始傳遞 Context。
2. 為不同的操作設定適當的超時時間。
3. 在長時間執行的操作中檢查取消訊號。
4. 使用 `defer cancel()` 清理資源。
5. 不要將 Context 存放在 Struct 欄位中。
6. 優雅地處理超時與取消錯誤。

遵循這些模式，你能建立出更可靠、具備高回應性，且能妥善處理失敗與尊重客戶端超時的應用程式。
