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

- `mock`：内置模拟 provider，便于本地联调与测试
- `openai`：兼容 OpenAI 风格上游，通过 `base_url + api_key` 访问

## 配置文件管理

当前支持通过一个 JSON catalog 文件统一管理：

- `keys`
- `providers`
- `models`

启动时如果配置了 `CATALOG_PATH`，服务会先从该文件导入配置；运行中可调用 `POST /admin/reload` 重新加载。

示例文件见 [catalog.example.json](file:///Users/zxz/AI/llm-gateway/catalog.example.json)。

## 环境变量

| 变量 | 说明 |
|------|------|
| `LISTEN_ADDR` | HTTP 监听地址，默认 `:8080` |
| `DATABASE_URL` | PostgreSQL 连接串 |
| `ADMIN_TOKEN` | 管理面 Bearer Token |
| `CATALOG_PATH` | 配置文件路径，按 JSON 管理 keys/providers/models |
| `REQUEST_TIMEOUT_SECONDS` | 请求超时秒数，默认 `60` |

## 本地运行

```bash
cd /Users/zxz/AI/llm-gateway
go mod tidy
go run ./cmd/llm-gateway
```

如果使用配置文件：

```bash
cd /Users/zxz/AI/llm-gateway
export CATALOG_PATH=/Users/zxz/AI/llm-gateway/catalog.example.json
export OPENAI_API_KEY=sk-xxx
go run ./cmd/llm-gateway
```

## 示例

创建一个默认 mock provider：

```bash
curl -X POST http://localhost:8080/admin/providers \
  -H 'Authorization: Bearer change-me' \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "default-mock",
    "type": "mock",
    "enabled": true,
    "is_default": true,
    "model_prefixes": ["mock-"]
  }'
```

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
