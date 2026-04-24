# Security Guide

This document describes the security-relevant defaults of Gortex and
the common patterns for using them safely in application code.

See `SECURITY.md` at the repository root for the vulnerability
reporting process.

## File serving

### `ctx.File(path)`

Intended for server-trusted paths only. The implementation cleans the
input and rejects any path containing `..` segments. Do **not** pass
user input (request parameters, form data, query strings) to this
method.

### `ctx.FileFS(fsys, name)`

Safe for user-supplied filenames. `name` is validated with
`fs.ValidPath`, which rejects absolute paths, `..` segments, leading
slashes, and empty elements. Construct `fsys` with `os.DirFS(root)` or
an embedded `embed.FS`.

```go
var uploadRoot = os.DirFS("/var/app/uploads")

func (h *FileHandler) GET(c httpctx.Context) error {
    return c.FileFS(uploadRoot, c.Param("name"))
}
```

## Redirects

`ctx.Redirect(code, target)` accepts only same-origin paths starting
with `/`. Protocol-relative (`//`), absolute-scheme (`http://`,
`https://`, `javascript:`, `data:`), and control-character-bearing
targets are rejected. If you legitimately need to redirect to an
external origin (for example, an OAuth flow), validate the URL against
an explicit whitelist and set the `Location` header directly:

```go
if !isAllowedExternal(next) {
    return httpctx.ErrUnsafeRedirectURL
}
c.Response().Header().Set("Location", next)
c.Response().WriteHeader(http.StatusFound)
return nil
```

## CORS

`middleware.CORSWithConfig(cfg)` returns an error for unsafe
configurations. In particular, `AllowOrigins = ["*"]` combined with
`AllowCredentials = true` is rejected — browsers ignore such
responses, and the combination often hides a logic bug.

When credentials are required, list concrete origins:

```go
mw, err := middleware.CORSWithConfig(&middleware.CORSConfig{
    AllowOrigins:     []string{"https://app.example"},
    AllowCredentials: true,
})
if err != nil {
    return err
}
```

## JSON request bodies

`context.ParameterBinder` enforces a `1 MiB` cap on JSON bodies by
default via `http.MaxBytesReader`. Oversized payloads surface as
decode errors (and HTTP 413 once the response headers are written).
Malformed JSON also surfaces as an error rather than being silently
ignored. Adjust the limit with:

```go
binder.SetMaxJSONBodyBytes(2 << 20) // 2 MiB
```

## Development error page

`middleware.GortexDevErrorPage` returns detailed diagnostics to help
during local development. The middleware redacts sensitive HTTP
headers (`Authorization`, `Cookie`, `Set-Cookie`, `X-Api-Key`,
`X-Auth-Token`, `X-Csrf-Token`, `Proxy-Authorization`) and any query
parameter whose name matches `(?i)(token|password|secret|key|apikey|auth)`.

Even with redaction, **do not enable this middleware in production**.
Gate it on `cfg.Logger.Level == "debug"` or equivalent.

## Further reading

- [SECURITY.md](../SECURITY.md) — vulnerability reporting process
- [API reference — Security Defaults](../API.md#security-defaults)
