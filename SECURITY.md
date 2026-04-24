# Security Policy

## Reporting a Vulnerability

Please report security vulnerabilities privately by emailing the maintainer
listed on the repository's `CODEOWNERS` file or by opening a draft GitHub
security advisory under the repository's **Security** tab. Do **not** file
a public issue for suspected security problems.

Include, where possible:

- A description of the issue and its impact.
- Steps to reproduce (proof-of-concept code preferred).
- The affected version or commit SHA.
- Any suggested mitigation.

You should receive an acknowledgement within 72 hours. Please allow up to
30 days for a patched release before any public disclosure.

## Supported Versions

Gortex is pre-1.0. Only the latest minor release line (currently
`v0.4.x-alpha`) receives security fixes. Older lines are unsupported.

## Security Defaults

The framework ships with these hardening defaults (as of v0.4.1-alpha).
Each can be tuned per application.

| Area | Default | Override |
| --- | --- | --- |
| File serving | `File(path)` rejects any path with `..` segments. | Use `FileFS(fsys, name)` for user-supplied filenames. |
| Redirects | `Redirect(code, url)` accepts only same-origin paths starting with `/`. | Write the `Location` header directly when an external redirect is required. |
| CORS | Default config allows `*` origins but not credentials. Combining `*` with `AllowCredentials=true` is rejected. | `CORSWithConfig` returns an error on unsafe configs. |
| JSON body size | `1 MiB` cap, enforced via `http.MaxBytesReader`. | `ParameterBinder.SetMaxJSONBodyBytes(n)`. |
| Dev error page | Redacts `Authorization`, `Cookie`, `Set-Cookie`, `X-Api-Key`, `X-Auth-Token`, `Proxy-Authorization`, and any query parameter whose name matches `(?i)(token\|password\|secret\|key\|apikey\|auth)`. | Do not run the dev error page middleware in production. |

## Trusted Proxies and Client IP

`DefaultContext.RealIP()` resolves the client IP by inspecting, in order:

1. The first value in the `X-Forwarded-For` header.
2. The `X-Real-IP` header.
3. `http.Request.RemoteAddr` (the TCP connection peer).

**Risk**: Steps 1 and 2 are header values set by the *caller*. If your server
is directly reachable from the internet (no reverse proxy), any client can
supply an arbitrary `X-Forwarded-For` value and impersonate any IP address.
This directly affects rate limiting (when using the default `KeyFunc`) and any
application logic that relies on `RealIP()` for access control.

**Recommended deployment**:

- Always place Gortex behind a trusted reverse proxy (nginx, Caddy, a cloud
  load balancer, etc.).
- Configure the proxy to **strip** any client-supplied `X-Forwarded-For`
  header before appending the real client IP.
- Do not expose the Gortex HTTP port directly to untrusted networks.

**Until a built-in trusted-proxy allowlist is added**, supply a custom
`KeyFunc` in `GortexRateLimitConfig` that uses authenticated identity (e.g.,
a verified JWT subject) rather than `c.RealIP()` when rate-limiting
authenticated endpoints.

## Hall of Fame

Reporters are credited in the release notes of the version that fixes
their finding, with their consent.
