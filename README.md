# proxy-service-component

一个轻量的 HTTP 反向代理服务（reverse proxy），用于把请求转发到上游服务。

## 功能

- **反向代理**：把所有路径（除健康检查）转发到 `UPSTREAM_URL`
- **健康检查**：`GET /healthz`（可配置）
- **超时**：可配置请求超时（可选）
- **日志**：输出每个请求的 method/path/status/耗时/request_id
- **请求链路**：
  - 自动补齐 `X-Request-Id`（若下游未传）
  - 自动追加 `X-Forwarded-For`，补齐 `X-Forwarded-Host` / `X-Forwarded-Proto`

## 运行（本地）

需要 Go 1.22+。

```bash
export UPSTREAM_URL="http://127.0.0.1:9000"
export LISTEN_ADDR=":8080"
go run ./cmd/proxy
```

验证：

```bash
curl -i http://127.0.0.1:8080/healthz
curl -i http://127.0.0.1:8080/
```

## 运行（Docker Compose，一键带上游 echo 服务）

```bash
docker compose up --build
curl -i http://127.0.0.1:8080/healthz
curl -i http://127.0.0.1:8080/anything
```

## 配置（环境变量）

- **UPSTREAM_URL**（必填）：上游地址，例如 `http://echo:80` / `http://127.0.0.1:9000`
- **LISTEN_ADDR**（可选，默认 `:8080`）：监听地址
- **HEALTH_PATH**（可选，默认 `/healthz`）：健康检查路径
- **REQUEST_TIMEOUT_SECONDS**（可选，默认 `0` 表示不启用）：单请求超时秒数
- **SHUTDOWN_TIMEOUT_SECONDS**（可选，默认 `10`）：优雅退出等待秒数
