# llm-gateway

Go 版 `llm-gateway`，作为独立服务对外暴露 OpenAI 兼容 HTTP 接口，并提供模型路由、流式透传、usage 统计与最小管理面能力。

## 当前能力

- `GET /healthz`
- `GET /v1/models`
- `POST /v1/chat/completions`
- `POST /v1/embeddings`
- `GET /admin/providers`
- `POST /admin/providers`
- `GET /admin/usages`
- `POST /admin/reload`

## Provider 说明

- `openai`：兼容 OpenAI 风格上游，通过 `base_url + api_key` 访问（也用于 DeepSeek 等 OpenAI-compatible 上游）

## 配置文件管理

当前支持通过 `admin-console` 的运行时配置接口统一管理：

- `keys`
- `providers`
- `models`

启动时优先从 `ADMIN_CONFIG_BASE_URL + ADMIN_LLM_CATALOG_PATH` 拉取 catalog；运行中可调用 `POST /admin/reload` 重新加载，也可通过 `ADMIN_CONFIG_RELOAD_INTERVAL` 开启定时刷新。

如果没有配置 admin 接口，也可以回退使用 `CATALOG_PATH` 本地文件。

示例文件见 [catalog.example.json](file:///Users/zxz/AI/llm-gateway/catalog.example.json)。

## 环境变量

| 变量 | 说明 |
|------|------|
| `LISTEN_ADDR` | HTTP 监听地址，默认 `:8080` |
| `DATABASE_URL` | PostgreSQL 连接串 |
| `ADMIN_TOKEN` | 管理面 Bearer Token |
| `ADMIN_CONFIG_BASE_URL` | admin-console 基础地址，例如 `http://admin-console:50083` |
| `ADMIN_LLM_CATALOG_PATH` | admin runtime catalog 路径，默认 `/api/runtime/llm-gateway/catalog` |
| `ADMIN_CONFIG_RELOAD_INTERVAL` | 定时从 admin 拉取配置的间隔，默认关闭 |
| `CATALOG_PATH` | 配置文件路径，按 JSON 管理 keys/providers/models |
| `REQUEST_TIMEOUT_SECONDS` | 请求超时秒数，默认 `60` |

## 本地运行

```bash
cd /Users/zxz/AI/llm-gateway
go mod tidy
go run ./cmd/llm-gateway
```

也可以在仓库根目录使用统一脚本，并通过 `llm-gateway/.env.local` 管理本地环境变量：

```bash
cp /Users/zxz/AI/llm-gateway/.env.local.example /Users/zxz/AI/llm-gateway/.env.local
./deploy/local/start.sh llm-gateway
```

如果使用配置文件：

```bash
cd /Users/zxz/AI/llm-gateway
export CATALOG_PATH=/Users/zxz/AI/llm-gateway/catalog.example.json
export OPENAI_API_KEY=sk-xxx
go run ./cmd/llm-gateway
```

## 示例

重新加载 catalog：

```bash
curl -X POST http://localhost:8080/admin/reload \
  -H 'Authorization: Bearer change-me'
```

调用聊天补全：

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "mock-chat",
    "messages": [{"role":"user","content":"hello"}]
  }'
```
