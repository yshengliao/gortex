# Gortex 技術文件

Gortex 框架的完整技術文件。

## 指南

| 文件 | 說明 |
|------|------|
| [API 參考](API.md) | Context 介面、router、struct tag、middleware、WebSocket、安全預設 |
| [安全指南](security.md) | 檔案提供、重導向、CORS、JSON body 限制 |
| [架構哲學與設計決策](architecture-philosophy.md) | Kitchen-sink 設計理由、K8s Config 策略、Jaeger/OTel 追蹤背景 |
| [設計模式與學習指南](design-patterns.md) | 10 個核心工程模式與難度分級學習路徑 |
| [部署指南](deployment.md) | Dockerfile、Docker Compose、K8s 範例、DevOps 注意事項 |

## 最佳實踐

| 文件 | 說明 |
|------|------|
| [Context 處理](best-practices/context-handling.md) | 生命週期、取消、goroutine、逾時策略 |

## 效能分析

| 文件 | 說明 |
|------|------|
| [Metrics 分析](performance/metrics-analysis.md) | 指標收集器效能測試與選擇指南 |

## 快速連結

- [範例程式](../../examples/) — 可直接執行的 `main.go` 參考（basic、auth、websocket）
- [SECURITY.md](../../SECURITY.md) — 漏洞通報流程
- [English Documentation](../en/)
