# Gortex 最佳實踐

使用 Gortex 構建生產環境應用程式的實用指南。

## 指南列表

### [Context 處理 (Context Handling)](./context-handling.md)

關於 `context.Context` 生命周期、取消訊號傳播以及常見陷阱。

- Context 生命周期與父子階層關係
- Goroutines 與 channels 中的取消機制
- 資料庫、快取與外部 API 的超時策略
- 常見的洩漏模式與如何避免

## 其他資源

- [API 參考指南](../API.md)
- [安全指南](../security.md)
- [範例應用程式](../../examples/)

---

最後更新：2026-04-24
