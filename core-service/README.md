# core-service

Python 版 `core-service`，作为独立 HTTP 服务承接 `message-gateway` 的流式请求，并编排 `llm-gateway` 完成回复生成。

## 当前能力

- `POST /v1/messages:stream`：接收 message-gateway 转发的 `{"envelope": ...}`，返回 SSE 流式增量
- `GET /healthz`
- `GET /metrics`
- `GET /admin/tasks/{task_id}`
- `GET /admin/conversations/{conversation_id}/messages`
- PostgreSQL 持久化：会话、消息、任务状态
- 内置独立 agent 模块，当前包含 `general`、`echo`
- 根据 `admin-console` 运行时配置进行 bot -> agent 路由
- `general` agent 调用 `llm-gateway` 的 `/v1/chat/completions`（`stream=true`）
- `echo` agent 直接返回回声文本，便于联调

## 环境变量

参考 [.env.example](file:///Users/zxz/AI/core-service/.env.example)。

## Agent 路由

- `core-service` 优先通过 `ADMIN_CONFIG_BASE_URL + ADMIN_CORE_ROUTING_PATH` 从 admin-console 拉取 bot -> agent 配置
- `message-gateway` 只透传 `X-Bot-Id` 等上下文 header
- `core-service` 收到请求后，会优先按 bot 映射得到的 agent 选择对应 agent
- 未命中或为空时，默认回退到 `general`
- 当前内置 agent：`general`、`echo`

除请求 body 中的 `envelope` 外，建议同时透传这些 header：

- `X-Bot-Id`
- `X-Session-Id`
- `X-User-Id`
- `X-Open-Id`
- `X-Chat-Id`
- `X-Trace-Id`
- `X-Request-Id`

运行中修改 admin 数据后，core-service 会按 `ROUTING_RELOAD_INTERVAL_SECONDS` 定期轮询并热加载（默认 2 秒）。

## 本地运行

```bash
cd /Users/zxz/AI/core-service
python -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
export $(cat .env.example | xargs)
uvicorn core_service.server:app --host 0.0.0.0 --port 8081
```

也可以在仓库根目录使用统一脚本，并通过 `core-service/.env.local` 管理本地环境变量：

```bash
cp /Users/zxz/AI/core-service/.env.local.example /Users/zxz/AI/core-service/.env.local
./deploy/local/start.sh core-service
```

## SSE 输出格式

每条事件：

```text
data: {"type":"delta","text":"hello","done":false}
```

结束事件：

```text
data: {"type":"done","text":"full response","done":true,"usage":{...}}
data: [DONE]
```
