# 設計模式與學習指南

> 本文檔以 v0.5.1-alpha 為基礎，從「可以從這個框架中學到什麼」的角度，整理 Gortex 中值得研究的工程模式與尚未實作但值得探討的方向。

## 已實作的核心設計模式

### 1. Struct Tag 驅動路由（Declarative Routing）

利用 Go 的 `reflect` 遞迴掃描 struct，把 `url`, `middleware`, `hijack`, `ratelimit` 等 tag 轉為路由註冊。這種「宣告式路由」的設計在 Go 生態中相對少見（多數框架仍用 `r.GET()` 命令式）。

**學習重點：**
- **Reflection 的實戰用法**：如何安全地遍歷 struct、取 tag、檢查方法簽章
- **Convention over Configuration**：方法名 `GET`, `POST` 自動對應 HTTP Method，自訂方法名自動轉 kebab-case 路徑
- **Middleware 繼承**：父 group 的 middleware 透過遞迴自動傳遞給子路由

```go
type HandlersManager struct {
    Users  *UserHandler  `url:"/users/:id"`
    Admin  *AdminGroup   `url:"/admin" middleware:"auth"`
}
```

**參考檔案**：`core/app/route_registration.go`

---

### 2. Segment-trie Router

一棵純手工的 Trie 路由樹，支援靜態路徑、`:param` 動態參數與 `*` 萬用字元。相比 Radix-tree，段落式 Trie 更好理解。

**學習重點：**
- **路由匹配的優先順序**：Static > Param > Wildcard 的回溯策略
- **為什麼用 Trie 而不是正則**：每次請求只做字串切割 + map 查詢，避免正則回溯的不確定性

**參考檔案**：`transport/http/gortex_router.go`

---

### 3. Context Pool（`sync.Pool` 實戰）

每次 HTTP 請求從 `sync.Pool` 取出 Context，用完歸還。

**學習重點：**
- `Pool.New` 的初始化策略（預先分配 `store` map）
- `ReleaseContext` 歸還前清理欄位但保留 map 容量（避免下次重新分配）
- `store` 用 `for k := range` 逐一刪除而非重新 `make`（保留底層 bucket 空間）

**參考檔案**：`transport/http/pool.go`

---

### 4. smartParams — 小物件最佳化（Small Buffer Optimization）

用固定陣列 `[4]string` 處理常見情況（≤4 個路徑參數），溢出才退化到 map。

**學習重點：**
- 為什麼 4 是合理的 threshold（多數 REST API 路徑參數不超過 3 層）
- 在 Go 中如何避免小物件產生 heap allocation

**參考檔案**：`transport/http/smart_params.go`

---

### 5. Sharded Metrics Collector

16 個獨立分片，每片有自己的 `RWMutex` 和 LRU。用 FNV hash 做 key 分配。

**學習重點：**
- **Lock Sharding**：將一把全域鎖拆成 N 把，降低高併發下的鎖競爭
- **Per-shard LRU eviction**：避免全域 LRU 成為瓶頸
- **Atomic counters**：高頻的 HTTP 計數器用 `atomic.AddInt64` 而非鎖

**參考檔案**：`observability/metrics/sharded_collector.go`

---

### 6. Circuit Breaker（斷路器）

教科書級的 Closed → Open → Half-Open 三態機。

**學習重點：**
- **State Machine 在 Go 中的慣用表達**：`atomic.Value` 存 State、`sync.Mutex` 保護 Counts
- **Generation-based 併發控制**：Half-Open 狀態下用 generation number 辨識請求所屬世代
- **`atomic.Uint32` 做 Half-Open 限流**：不用鎖也能限制同時進入的請求數

**參考檔案**：`pkg/utils/circuitbreaker/circuitbreaker.go`

---

### 7. WebSocket Hub（Actor-like 單執行緒 Goroutine）

`Hub.Run()` 是一個永遠跑在獨立 Goroutine 的事件迴圈。所有對 `clients` map 的讀寫都透過 channel 進入這個唯一 Goroutine，避免鎖。

**學習重點：**
- **Go 版 Actor 模型**：用 channel 取代 mutex，單一 Goroutine 序列化所有狀態存取
- **Graceful shutdown 設計**：`shutdownOnce` + 先發 close message 再等 500ms 的兩階段關閉
- **Channel full 的處理策略**：broadcast 時 `select { case client.send <- msg: default: }` 直接丟棄慢消費者

**參考檔案**：`transport/websocket/hub.go`

---

### 8. Rate Limiter 的 TTL 與 Cleanup

`MemoryRateLimiter` 用 `golang.org/x/time/rate` 的 Token Bucket 為每個 IP 建立 Limiter。

**學習重點：**
- **背景清理 Goroutine**：`runCleanup` 定期掃描過期 entry，避免記憶體無限成長
- **符合 HTTP 標準的回應標頭**：`X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset`, `Retry-After` 的計算邏輯
- **`RateLimitStatuser` 介面分離**：不是所有 Store 都能提供 status，用 type assertion 選擇性輸出 header

**參考檔案**：`middleware/ratelimit.go`

---

### 9. Health Check 三態系統

可註冊多個 `HealthCheck` 函式，背景 Goroutine 定時跑，每個 check 帶有 Timeout。

**學習重點：**
- **三態健康狀態**：Healthy / Degraded / Unhealthy 比簡單的 up/down 更貼近真實世界的 K8s readiness/liveness probe 設計
- **Concurrent check with WaitGroup**：並行執行所有 check，用 `sync.WaitGroup` 等待全部完成

**參考檔案**：`observability/health/health.go`

---

### 10. Tracing 抽象層

定義了 `Tracer` → `EnhancedTracer` 介面層級，搭配 `SimpleTracer`（In-memory）與 OTel adapter。

**學習重點：**
- **介面驅動設計**：框架只依賴 `Tracer` 介面，可在不改程式碼的情況下從 in-memory 切換到 Jaeger/OTel
- **Context propagation**：Span 如何透過 `context.WithValue` 在請求鏈路中傳遞
- **Severity level 設計**：8 級嚴重性（DEBUG 到 EMERGENCY）如何與 OTel 的 3 級 Status 做映射

**參考檔案**：`observability/tracing/tracing.go`、`observability/otel/adapter.go`

---

## 尚未實作但值得探討的方向

以下列出幾個方向，作為「如果繼續深化這個框架，還可以研究什麼」的紀錄。

### 1. Graceful Shutdown 的完整鏈路

目前 `App.Shutdown()` 已經有基本流程，但完整的生產級 Graceful Shutdown 包含：

- **Drain connections**：先從 Load Balancer 摘除（K8s `preStop` hook + readiness probe 切 false），等 in-flight 請求處理完
- **Dependency ordering**：先關 HTTP listener → drain 請求 → 關 WebSocket Hub → 關 DB 連線池
- **Shutdown deadline propagation**：把 OS signal 的 deadline 傳到所有子系統

### 2. Middleware Chain 的錯誤語意

目前 middleware 回傳 `error` 後，由 `ServeHTTP` 統一處理。生產框架通常需要區分：

- **可重試的錯誤** vs **不可重試的錯誤**
- **應該記錄 log 的錯誤** vs **已由 middleware 自行處理的錯誤**
- **Error boundary**：某個 middleware 吞掉 error 後，上層 middleware 是否還能觀測到

### 3. 組態熱更新（Config Hot-reload）

`pkg/config` 目前是一次性載入。在 K8s 中，ConfigMap 會被 kubelet 自動更新到 Pod 的掛載目錄。如果框架支援 `fsnotify` 監聯設定檔變更，就能做到不重啟即調整 log level、動態調整 rate limit 閾值、Feature flag 熱切換等。

### 4. 請求級別的 Timeout Propagation

目前框架沒有「全域請求超時」的機制。生產框架通常會在最外層 middleware 設定一個 deadline context，確保即使 Handler 忘記設定 timeout，整個請求也不會無限期掛住。

### 5. 結構化錯誤（Structured Error）

目前 `pkg/errors` 是簡單的 code-to-message 註冊表。更進階的設計可能包含 Error Code 命名空間（`USER.NOT_FOUND` vs `ORDER.PAYMENT_FAILED`）、Error Chain（用 `errors.As` / `errors.Is` 做分層判斷）以及 i18n 錯誤訊息。

### 6. OpenAPI / Swagger 自動產生

`core/app/doc/` 已有 `swagger.Provider` 的骨架，但 spec 產生邏輯尚未完成。從 struct tag + method 簽章自動推導出 OpenAPI spec 是一個很有深度的題目。

---

## 學習路徑建議

| 難度 | 主題 | 起始檔案 |
|------|------|----------|
| ⭐ | `sync.Pool` 與 Context 重用 | `transport/http/pool.go` |
| ⭐ | Small Buffer Optimization | `transport/http/smart_params.go` |
| ⭐⭐ | Segment-trie 路由匹配 | `transport/http/gortex_router.go` |
| ⭐⭐ | Token Bucket Rate Limiter | `middleware/ratelimit.go` |
| ⭐⭐ | Health Check 三態設計 | `observability/health/health.go` |
| ⭐⭐⭐ | Circuit Breaker 三態機 | `pkg/utils/circuitbreaker/circuitbreaker.go` |
| ⭐⭐⭐ | Lock Sharding + LRU eviction | `observability/metrics/sharded_collector.go` |
| ⭐⭐⭐ | Actor-like WebSocket Hub | `transport/websocket/hub.go` |
| ⭐⭐⭐⭐ | Reflection 路由註冊 | `core/app/route_registration.go` |
| ⭐⭐⭐⭐ | OTel Adapter + Tracing | `observability/tracing/tracing.go` |
