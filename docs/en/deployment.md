# Deployment Guide

Container deployment examples for Gortex, demonstrating the "same image, different env injection" strategy.

---

## Quick Start

### Minimal Dockerfile

```dockerfile
FROM golang:1.25-alpine
WORKDIR /app
COPY . .
RUN go build -o server ./cmd/server
EXPOSE 8080
CMD ["./server"]
```

```bash
docker build -t myapp .
docker run -p 8080:8080 myapp
```

### Docker Compose (Local Dev)

```yaml
services:
  app:
    build: .
    ports:
      - "8080:8080"
    environment:
      GORTEX_LOGGER_LEVEL: debug
      GORTEX_JWT_SECRET_KEY: "local-dev-secret-key-at-least-32-bytes!"
      GORTEX_DATABASE_USER: gortex
      GORTEX_DATABASE_PASSWORD: gortex
      GORTEX_DATABASE_HOST: db
    depends_on:
      - db
  db:
    image: postgres:17-alpine
    environment:
      POSTGRES_USER: gortex
      POSTGRES_PASSWORD: gortex
      POSTGRES_DB: gortex
    ports:
      - "5432:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data
volumes:
  pgdata:
```

---

## Production Dockerfile (Multi-stage)

```dockerfile
FROM golang:1.25-alpine AS builder
RUN apk add --no-cache git ca-certificates
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /app ./cmd/server

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
RUN adduser -D -u 1000 appuser
COPY --from=builder /app /app
USER appuser
EXPOSE 8080
ENTRYPOINT ["/app"]
```

## Config Precedence

**Env vars > .env > YAML > Code defaults**

```bash
# Docker: override via env
docker run -p 8080:8080 \
  -e GORTEX_LOGGER_LEVEL=info \
  -e GORTEX_JWT_SECRET_KEY="secret-at-least-32-bytes" \
  -e GORTEX_DATABASE_USER=user \
  -e GORTEX_DATABASE_PASSWORD=pass \
  myapp
```

For Kubernetes, use ConfigMap for non-sensitive values and Secret for credentials. See the [Chinese version](../zh-tw/deployment.md) for full K8s manifests.

## DevOps Notes

- **Graceful shutdown**: `ShutdownTimeout` defaults to 10s. Coordinate with K8s `terminationGracePeriodSeconds` (default 30s). Add a `preStop` hook with 5s delay for rolling updates.
- **Debug endpoints**: `/_routes` and `/_monitor` are only active when `Logger.Level = "debug"`. **Do not enable in production.**
- **Health probes**: Use `/health` for K8s liveness/readiness (supports healthy/degraded/unhealthy).
- **Image tags**: Avoid `latest`. Use Git SHA or semver (e.g. `myapp:v0.5.2-a395c44`).
- **Log format**: Use `Logger.Encoding = "json"` in production for structured log aggregation.
