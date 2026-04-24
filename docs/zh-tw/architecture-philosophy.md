# 架構哲學與設計決策 (Architecture Philosophy & Design Decisions)

> **⚠️ 專案定位聲明**
> 本專案為研究與紀錄用途，主要透過 AI 協助，在本地復刻**過去常用的「通用基礎設施框架（Platform Engineering / Infrastructure Framework）」架構哲學**。
> 其設計並非為了解決單一業務需求，而是為了探討與保留大型基礎建設中常見的設計模式。

## 為什麼採用「重度封裝（Kitchen-sink）」的設計？

在現代 Go 語言的生態系中，社群主流傾向使用小巧、標準的庫進行組合（例如 Go 1.22+ 原生 `ServeMux` 或 `chi`）。在一般的微服務開發中，這符合 YAGNI (You Aren't Gonna Need It) 原則。

然而，當情境轉換到**大型基礎設施團隊（Platform Engineering）**時，目標會從「輕量」轉變為「強制一致性」與「高可觀測性」。Gortex 復刻的正是這種思維：

1. **強制一致性（Standardization）**：
   透過自訂的 `Context`、強制綁定的 `Config` 與統一的 `Logger`，確保跨部門、跨微服務的程式碼在處理請求、記錄日誌與讀取配置時，擁有一致的標準。這能大幅降低維運人員跨專案除錯的認知成本。
   此外，Config 的多來源載入設計（支援 YAML、環境變數與 `.env` 混搭覆蓋）正是為了適應 Kubernetes (K8s) 的各個隔離環境。在本地端測試時可讀取設定檔，而上線時維持同一個 Docker Image，改由 K8s 注入環境變數，兩者共用同一套解析邏輯。

2. **開箱即用的可觀測性（Observability Out-of-the-box）**：
   過去為了追蹤內部錯綜複雜的微服務鏈結，框架深度整合了分散式追蹤（Distributed Tracing，例如配合 Jaeger 與 OpenTelemetry），並內建了 `httpclient` 連線池指標收集，以及自帶的 `/_routes` 與 `/_monitor` 端點。業務開發者不需要每次建立新專案都重新配置 Prometheus 或 Tracing 的中介軟體，這些複雜度皆由框架底層吸收。

3. **依賴收斂（Dependency Consolidation）**：
   將路由、參數綁定、中介軟體等核心元件收斂在框架內部，當遇到資安漏洞或需要全域調整行為（例如修改 CORS 預設策略、調整全域 Timeout）時，只需更新框架版本，而不必去每個專案逐一追查它們使用了哪一套第三方函式庫。

## 權衡與技術債 (Trade-offs & Technical Debt)

這種「把複雜度留在框架層」的設計雖然對業務端友善，但也意味著框架本身需要承擔極高的維護責任：

* **與標準庫的解耦**：自訂 `Context` 導致無法直接無縫套用社群上針對 `http.Handler` 原生開發的 Middleware，必須實作轉接層。
* **維護成本**：自行維護 Segment-trie 路由器與 HTTP Client 連線池，需要處理極端的邊界條件與效能瓶頸（例如防範 Goroutine Leak 或惡意連線）。

## 總結

Gortex 並不是要與 Gin、Echo 或標準庫競爭的開源框架。它是作為一個實體範本（Reference Implementation），用來紀錄並展示：**當系統開發的重點從「單一微服務的輕量化」轉向「跨服務的基礎設施統一與治理」時，框架層需要做出的設計決策與權衡。**
