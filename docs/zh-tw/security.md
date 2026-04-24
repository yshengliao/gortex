# 安全指南

本文檔說明了 Gortex 在安全性方面的預設行為，以及在應用程式碼中安全使用它們的常見模式。

漏洞通報流程請參閱專案根目錄的 `SECURITY.md`。

## 檔案提供 (File serving)

### `ctx.File(path)`

僅供伺服器信任的路徑使用。其實作會清理輸入並拒絕任何包含 `..` 片段的路徑。**絕對不要**將使用者輸入（如請求參數、表單資料、查詢字串）直接傳遞給這個方法。

### `ctx.FileFS(fsys, name)`

對於使用者提供的檔名而言是安全的。`name` 會透過 `fs.ValidPath` 進行驗證，這會拒絕絕對路徑、`..` 片段、開頭的斜線以及空元素。請使用 `os.DirFS(root)` 或內嵌的 `embed.FS` 來建構 `fsys`。

```go
var uploadRoot = os.DirFS("/var/app/uploads")

func (h *FileHandler) GET(c httpctx.Context) error {
    return c.FileFS(uploadRoot, c.Param("name"))
}
```

## 重導向 (Redirects)

`ctx.Redirect(code, target)` 僅接受以 `/` 開頭的同源路徑。會拒絕包含協定相對路徑（`//`）、絕對 scheme（`http://`, `https://`, `javascript:`, `data:`）以及帶有控制字元的目標。如果你有正當理由需要重導向到外部網域（例如 OAuth 流程），請自行用白名單驗證 URL，並直接設定 `Location` 標頭：

```go
if !isAllowedExternal(next) {
    return httpctx.ErrUnsafeRedirectURL
}
c.Response().Header().Set("Location", next)
c.Response().WriteHeader(http.StatusFound)
return nil
```

## CORS

對於不安全的配置，`middleware.CORSWithConfig(cfg)` 會回傳錯誤。特別是當 `AllowOrigins = ["*"]` 與 `AllowCredentials = true` 組合使用時會被拒絕——瀏覽器會忽略這類回應，且這種組合通常隱藏著邏輯漏洞。

當需要憑證（Credentials）時，請列出具體的 Origins：

```go
mw, err := middleware.CORSWithConfig(&middleware.CORSConfig{
    AllowOrigins:     []string{"https://app.example"},
    AllowCredentials: true,
})
if err != nil {
    return err
}
```

## JSON 請求主體 (Request bodies)

`context.ParameterBinder` 預設透過 `http.MaxBytesReader` 強制設定 JSON body 的上限為 `1 MiB`。過大的 Payload 會導致解碼錯誤（一旦寫入回應標頭，則回傳 HTTP 413）。格式錯誤的 JSON 也會直接拋出錯誤，而不是被默默忽略。可透過以下方式調整限制：

```go
binder.SetMaxJSONBodyBytes(2 << 20) // 2 MiB
```

## 開發環境錯誤頁面 (Development error page)

`middleware.GortexDevErrorPage` 會回傳詳細的診斷資訊以協助本地開發。此中介軟體會遮蔽敏感的 HTTP 標頭（如 `Authorization`, `Cookie`, `Set-Cookie`, `X-Api-Key`, `X-Auth-Token`, `X-Csrf-Token`, `Proxy-Authorization`），以及名稱符合正規表示式 `(?i)(token|password|secret|key|apikey|auth)` 的任何查詢參數。

儘管有遮蔽機制，**請絕對不要在生產環境啟用這個中介軟體**。請使用 `cfg.Logger.Level == "debug"` 或等效條件來控制它。

## 延伸閱讀

- [SECURITY.md](../../SECURITY.md) — 漏洞通報流程
- [API 參考 — 安全預設值](./API.md#安全預設值)
