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

The framework ships with these hardening defaults (as of the 2025-11-20
security audit follow-up). Each can be tuned per application.

| Area | Default | Override |
| --- | --- | --- |
| File serving | `File(path)` rejects any path with `..` segments. | Use `FileFS(fsys, name)` for user-supplied filenames. |
| Redirects | `Redirect(code, url)` accepts only same-origin paths starting with `/`. | Write the `Location` header directly when an external redirect is required. |
| CORS | Default config allows `*` origins but not credentials. Combining `*` with `AllowCredentials=true` is rejected. | `CORSWithConfig` returns an error on unsafe configs. |
| JSON body size | `10 MiB` cap, enforced via `http.MaxBytesReader`. | `ParameterBinder.SetMaxJSONBodyBytes(n)`. |
| Dev error page | Redacts `Authorization`, `Cookie`, `Set-Cookie`, `X-Api-Key`, `X-Auth-Token`, `Proxy-Authorization`, and any query parameter whose name matches `(?i)(token\|password\|secret\|key\|apikey\|auth)`. | Do not run the dev error page middleware in production. |

## Historical Reviews

- [2025-11-20 — Comprehensive code review](docs/reviews/2025-11-20-code-review.md)
- [2025-11-20 — Security audit](docs/reviews/2025-11-20-security-audit.md)

## Hall of Fame

Reporters are credited in the release notes of the version that fixes
their finding, with their consent.
