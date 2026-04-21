# Gortex examples

Minimal reference implementations — each one stays in a single file and
focuses on a single piece of the framework.

| Example                   | Shows                                                        |
| ------------------------- | ------------------------------------------------------------ |
| [basic](basic/)           | Struct-tag routing, binder, the default middleware chain     |
| [websocket](websocket/)   | Hub config with message cap, type allow-list, authorizer     |
| [auth](auth/)             | `pkg/auth` JWT service: login, refresh, protected endpoint   |

Run any example with `go run ./examples/<name>`. Each directory has its
own `README.md` with the specific commands and a `curl` transcript of the
golden path. All three listen on `:8080` by default, so run them one at
a time.
