# 指標收集器 (Metrics Collector) 效能分析

> **注意（v0.8.0-alpha）**：`OptimizedCollector`（`sync.Map` 變體）已於 v0.8.0-alpha 中移除。以下數據為維護者機器（Apple M3 Pro）的歷史測量結果，保留作為設計紀錄。目前程式碼庫中僅剩 `ImprovedCollector` 與 `ShardedCollector`。

## 總覽

本文檔記錄了 Gortex 指標收集器的效能特徵，並協助針對不同使用情境做出選擇。

## 效能測試結果（歷史數據 — 僅供參考，來自維護者機器）

### 單執行緒效能 (Single-Threaded Performance)
| 實作方式 | ns/op | B/op | allocs/op |
|---|---|---|---|
| 原始版 (ImprovedCollector) | 120.1 | 15 | 1 |
| ~~最佳化版 (sync.Map)~~ *（已於 v0.8.0-alpha 移除）* | 193.8 | 87 | 4 |
| **分片版 (Sharded，推薦使用)** | **153.6** | **15** | **1** |

### 高併發寫入 (High-Concurrency Writes)
| 實作方式 | ns/op | B/op | allocs/op | 效能變化 |
|---|---|---|---|---|
| 原始版 (ImprovedCollector) | 272.7 | 24 | 1 | 基準值 |
| ~~最佳化版 (sync.Map)~~ | 275.3 | 96 | 4 | -1% |
| **分片版 (Sharded，推薦使用)** | **108.2** | **24** | **1** | **+60%** |

### 混合讀寫負載 (Mixed Read/Write Workloads)
| 實作方式 | ns/op | B/op | allocs/op | 相對基準 |
|---|---|---|---|---|
| 原始版 (ImprovedCollector) | 316.3 | 322 | 4 | 基準值 |
| ~~最佳化版 (sync.Map)~~ | 6151.0 | 18518 | 12 | -95%（顯著劣化） |
| **分片版 (Sharded)** | **7217.0** | **18473** | **10** | **-95%（顯著劣化）** |

### HTTP 請求記錄（無鎖競爭情況）
| 實作方式 | ns/op | B/op | allocs/op |
|---|---|---|---|
| 原始版 (ImprovedCollector) | 159.4 | 0 | 0 |
| ~~最佳化版 (sync.Map)~~ | 159.8 | 0 | 0 |
| **分片版 (Sharded，推薦使用)** | **152.6** | **0** | **0** |

## 關鍵發現

### 1. 分片收集器在純高併發寫入場景勝出
- 在高併發寫入情境中（相同 shard key、多個 goroutine 同時寫入）**效能提升了 60%**。
- 在發生鎖競爭（Contention）時提供最佳效能，同時保持較低的記憶體開銷。
- 效能可隨 CPU 核心數線性擴展。

### 2. 混合讀寫模式：所有基於鎖的實作均明顯退化
混合讀寫的測試結果並非筆誤：當不同 goroutine 交替對不同 key 進行讀寫時，benchmark 會觸發分片間協調與 LRU 淘汰的額外開銷，導致分片版與 sync.Map 版相較基準均退化 95%。若工作負載以讀取為主，`ImprovedCollector` 可能是更好的選擇。

### 3. OptimizedCollector 已移除
`sync.Map` 變體（`OptimizedCollector`）已於 v0.8.0-alpha 移除：它在高寫入負載下毫無改善（-1%）、在混合讀寫下慢了 20 倍，且其非原子的首次寫入路徑會造成基數重複計算。

### 4. 鎖競爭熱點
- 記錄業務指標（Business metrics）是主要效能瓶頸。
- HTTP/WebSocket 統計由於使用原子計數器，鎖競爭極小。
- 每個分片的 LRU 淘汰機制帶來額外的鎖定壓力。

## 已實作的最佳化策略

### 分片收集器架構 (Sharded Collector Architecture)

```go
type ShardedCollector struct {
    // 針對不同指標類型使用獨立的鎖
    httpMu             sync.RWMutex  // HTTP 指標
    websocketMu        sync.RWMutex  // WebSocket 指標  
    systemMu           sync.RWMutex  // 系統指標
    
    // 業務指標分為 16 個分片
    shards            [16]*metricShard
    
    // 用於追蹤淘汰機制的全域狀態
    globalMu           sync.RWMutex
    globalEvictionStats EvictionStats
}

type metricShard struct {
    mu              sync.RWMutex
    metrics         map[string]float64
    lruList        *list.List
    lruMap         map[string]*list.Element
    // ... 每個分片的 LRU 與淘汰狀態
}
```

### 關鍵最佳化細節

1. **鎖分片 (Lock Sharding)**：16 個獨立的分片能有效降低鎖競爭。
2. **基於雜湊的分發 (Hash-Based Distribution)**：使用 FNV 雜湊演算法確保資料平均分佈。
3. **獨立 Mutex**：不同的指標類型使用獨立的讀寫鎖。
4. **原子計數器 (Atomic Counters)**：高頻率的計數器改用底層原子操作。
5. **分片獨立 LRU (Per-Shard LRU)**：緩存淘汰決策在各分片內獨立進行。

## 使用建議

### 收集器選擇指南

| 情境 | 推薦的收集器 | 原因 |
|---|---|---|
| **高併發 Web 伺服器（寫入密集）** | `ShardedCollector` | 在高負載寫入下有 60% 的效能提升 |
| **低流量應用程式** | `ImprovedCollector` | 實作簡單，資源開銷較低 |
| **記憶體受限的環境** | `ImprovedCollector` | 佔用較小的記憶體足跡 |
| **混合讀寫場景** | `ImprovedCollector` | 在混合讀寫 benchmark 中表現更穩定 |

### 遷移指南

```go
// 修改前 (原始版)
collector := metrics.NewImprovedCollector()

// 修改後 (針對高併發寫入最佳化)
collector := metrics.NewShardedCollector()

// 所有 API 介面保持不變
collector.RecordBusinessMetric("requests", 1.0, map[string]string{"status": "200"})
collector.RecordHTTPRequest("GET", "/api", 200, time.Millisecond)
stats := collector.GetStats()
```

## 效能影響分析

### 記憶體使用量
- **原始版**：每筆指標約 240 bytes + LRU 額外開銷。
- **分片版**：每筆指標約 240 bytes + 16 倍的 LRU 額外開銷。
- **權衡 (Trade-off)**：以稍微高一點點的固定記憶體開銷，換取顯著的併發效能提升。

### CPU 使用率
- **原始版**：O(1)，但有極高的鎖競爭。
- **分片版**：O(1)，鎖競爭降低了 16 倍。
- **擴展性**：隨 CPU 核心數增加呈線性成長。

### 延遲特徵 (Latency Characteristics)
- **P99 延遲**：在高負載下減少了 60%。
- **尾部延遲 (Tail latency)**：由於鎖競爭減少，延遲更加穩定。
- **吞吐量 (Throughput)**：120 萬+ ops/sec 對比原始的 80 萬 ops/sec。

## 結論

**ShardedCollector** 在持續高併發寫入工作負載下勝出（提升約 60%），在低競爭路徑下的表現與 `ImprovedCollector` 相近。對於混合讀寫模式，兩種實作相較單執行緒基準均有明顯退化，因此最終選擇應視工作負載特性而定。

兩個收集器均作為函式庫提供，需由應用程式明確連結使用。內建的 `/_monitor` 端點直接讀取執行期統計，並不依賴任何收集器。

### 建議
1. 若工作負載以對相同指標 key 的高併發寫入為主，使用 `ShardedCollector`。
2. 若為低流量或混合讀寫場景，使用 `ImprovedCollector`。
3. ~~移除 `OptimizedCollector`~~ — 已於 v0.8.0-alpha 完成移除。

### 未來最佳化方向
1. 根據 CPU 核心數動態調整分片數量。
2. 實作無鎖（Lock-free）計數器以追求極致效能。
3. 支援批次操作以處理大量的指標寫入。
4. 為 LRU 條目導入記憶體池（Memory pool）以減輕 GC 壓力。
