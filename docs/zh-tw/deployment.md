# 部署指南

本文檔提供 Gortex 應用程式的容器化部署範例，展示如何搭配 Config 系統實現「同一個 Image，不同環境注入變數」的部署策略。

---

## 快速開始

### 最簡 Dockerfile

適合本地開發和快速驗證：

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

### Docker Compose（本地全套環境）

```yaml
# docker-compose.yml
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

```bash
docker compose up        # 啟動
docker compose down -v   # 停止並清除資料
```

### Makefile 常用指令

```makefile
.PHONY: build run docker docker-up

build:
	go build -o bin/server ./cmd/server

run:
	go run ./cmd/server

docker:
	docker build -t myapp .

docker-up:
	docker compose up -d

docker-down:
	docker compose down -v
```

---

## 正式環境 Dockerfile（Multi-stage Build）

```dockerfile
# ── Stage 1: Build ────────────────────────────────────────
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /app ./cmd/server

# ── Stage 2: Runtime ──────────────────────────────────────
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata
RUN adduser -D -u 1000 appuser

COPY --from=builder /app /app

USER appuser
EXPOSE 8080

ENTRYPOINT ["/app"]
```

**要點：**
- **Multi-stage build**：最終 Image 不含 Go toolchain，通常 < 20 MB。
- **`CGO_ENABLED=0`**：純靜態連結，不依賴 glibc。
- **Non-root user**：以 `appuser` (UID 1000) 執行，符合 K8s Pod Security Standards。
- **不 COPY 設定檔**：設定透過環境變數或 ConfigMap 掛載注入。

## 搭配 Config 系統

Gortex 的 Config 載入優先順序：**環境變數 > .env > YAML > 程式碼預設值**。

### 本地開發

```bash
# 使用 config.yaml + .env
go run ./cmd/server
```

### Docker 單機

```bash
docker build -t myapp .

# 透過環境變數覆蓋設定
docker run -p 8080:8080 \
  -e GORTEX_SERVER_PORT=8080 \
  -e GORTEX_LOGGER_LEVEL=info \
  -e GORTEX_JWT_SECRET_KEY="your-secret-key-at-least-32-bytes" \
  -e GORTEX_DATABASE_USER=prod_user \
  -e GORTEX_DATABASE_PASSWORD=prod_pass \
  -e GORTEX_DATABASE_HOST=db.internal \
  myapp
```

### Kubernetes Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: myapp
spec:
  replicas: 3
  selector:
    matchLabels:
      app: myapp
  template:
    metadata:
      labels:
        app: myapp
    spec:
      containers:
        - name: myapp
          image: myapp:latest
          ports:
            - containerPort: 8080
          env:
            # 從 ConfigMap 注入非敏感設定
            - name: GORTEX_SERVER_PORT
              valueFrom:
                configMapKeyRef:
                  name: myapp-config
                  key: server-port
            - name: GORTEX_LOGGER_LEVEL
              valueFrom:
                configMapKeyRef:
                  name: myapp-config
                  key: logger-level
            # 從 Secret 注入敏感資訊
            - name: GORTEX_JWT_SECRET_KEY
              valueFrom:
                secretKeyRef:
                  name: myapp-secrets
                  key: jwt-secret
            - name: GORTEX_DATABASE_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: myapp-secrets
                  key: db-password
          # 健康檢查
          livenessProbe:
            httpGet:
              path: /health
              port: 8080
            initialDelaySeconds: 5
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: /health
              port: 8080
            initialDelaySeconds: 3
            periodSeconds: 5
          resources:
            requests:
              cpu: 100m
              memory: 64Mi
            limits:
              cpu: 500m
              memory: 128Mi
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: myapp-config
data:
  server-port: "8080"
  logger-level: "info"
---
apiVersion: v1
kind: Secret
metadata:
  name: myapp-secrets
type: Opaque
stringData:
  jwt-secret: "your-production-secret-at-least-32-bytes"
  db-password: "production-db-password"
```

### 或掛載 YAML ConfigMap

如果偏好用 YAML 設定檔而非逐一注入環境變數：

```yaml
# ConfigMap 內容
apiVersion: v1
kind: ConfigMap
metadata:
  name: myapp-yaml-config
data:
  config.yaml: |
    server:
      port: "8080"
      read_timeout: 30s
      write_timeout: 30s
    logger:
      level: "info"
      encoding: "json"
    database:
      host: "db.internal"
      port: 5432
      database: "myapp"
```

```yaml
# Deployment 中掛載
volumes:
  - name: config
    configMap:
      name: myapp-yaml-config
containers:
  - name: myapp
    volumeMounts:
      - name: config
        mountPath: /etc/myapp
        readOnly: true
    env:
      # 敏感值仍用 Secret 注入，會覆蓋 YAML
      - name: GORTEX_JWT_SECRET_KEY
        valueFrom:
          secretKeyRef:
            name: myapp-secrets
            key: jwt-secret
```

對應的 Go 程式碼：

```go
cfg := config.NewConfigBuilder().
    LoadYamlFile("/etc/myapp/config.yaml").  // ConfigMap 掛載
    LoadEnvironmentVariables("GORTEX_").      // Secret 透過 env 覆蓋
    MustBuild()
```

## .dockerignore

```
.git
.github
docs
examples
tools
performance
internal
*.md
.golangci.yml
.gitignore
```

## 設計原則

| 原則 | 說明 |
|------|------|
| **Image 不含設定** | 所有環境用同一個 Image，設定透過 env / ConfigMap 注入 |
| **Secret 不進 Image** | JWT key、DB password 等透過 K8s Secret 或 Vault 注入 |
| **Non-root 執行** | 容器內以非 root 使用者執行 |
| **Health check** | 搭配 `livenessProbe` / `readinessProbe` 做滾動更新 |
| **最小化 Image** | Multi-stage build + Alpine，減少攻擊面 |

## DevOps 注意事項

**Signal 處理與優雅關機**
- Gortex 的 `App.Shutdown()` 會等待 in-flight 請求完成。K8s 預設給 30 秒 `terminationGracePeriodSeconds`，框架預設 `ShutdownTimeout` 為 10 秒，兩者需協調。
- 建議加 `preStop` hook 延遲 5 秒，讓 kube-proxy 先摘除 endpoint，避免滾動更新時收到新請求又立即關閉。

**日誌格式**
- 正式環境建議 `Logger.Encoding = "json"`，方便 Fluentd / Loki 等日誌系統解析。
- `Logger.Level = "debug"` 會啟用 `/_routes` 和 `/_monitor` 除錯端點，**正式環境不要設為 debug**。

**Observability 端點**
- `/_routes`、`/_monitor`：僅在 debug 模式啟用，不會暴露到正式環境。
- `/health`：搭配 K8s probe 使用，支援 healthy / degraded / unhealthy 三態。

**Image Tag 策略**
- 避免使用 `latest` tag。建議用 Git SHA 或語意化版本（如 `myapp:v0.5.2-a395c44`）。
