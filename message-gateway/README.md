# message-gateway

Go 版 `message-gateway` 第一版实现，优先支持 `Lark Bot` 渠道。

## 当前能力

- 接收 `POST /callbacks/feishu` 事件回调
- 处理 `url_verification` 验证请求
- 解析 `im.message.receive_v1` 文本消息
- 基于 `event_id` 写入 `inbound_event` 做去重
- 基于 `job` 表异步发送 Lark 文本回复
- 暴露 `GET /admin/healthz` 和 `GET /admin/metrics`
- 提供 DLQ 基础运维接口：`GET /admin/jobs/dead`、`POST /admin/jobs/{job_id}/replay`

## 当前默认路由

- `/help`：返回帮助文本
- 其他任意文本：返回默认回声回复

## 路由规则文件（JSON）

默认内置路由只覆盖最小闭环；如果配置了 `ADMIN_CONFIG_BASE_URL`，会优先从 admin runtime 接口拉取路由规则并按轮询间隔热加载；如果未配置 admin，则回退使用 `ROUTE_RULES_PATH` 本地文件。

示例文件见 [routes.example.json](file:///Users/zxz/AI/message-gateway/routes.example.json)。

## 下游转发（Core Service）

当配置 `CORE_BASE_URL` 后，网关会把入站事件写入 `forward_to_core` job，由 worker 调用 core service 获取回复，并使用“流式卡片”边生成边更新到 Lark（`im.message.patch`）。

如需回退到“聚合后一次性文本回写”，可设置 `LARK_STREAMING_CARD_ENABLED=false`。

## 端到端流式（Lark 流式卡片）

网关的端到端流式更新是通过“先发卡片、再持续更新同一条消息”的策略实现的：

1. worker 创建一条 `msg_type=interactive` 的卡片消息（流式模式，`config.streaming_mode=true`，并显式设置 `config.update_multi=true`）
2. worker 调用 core service 的流式接口，边读边累积输出
3. worker 按节流频率（`LARK_STREAMING_CARD_UPDATE_INTERVAL`）对同一条消息执行 `im.message.patch`，把最新累计内容写回卡片
4. core stream 结束后，worker 再 `patch` 一次，将卡片切为“最终态”（非流式模式）

### 约束与注意事项

- 单条消息更新频控为 5QPS，网关通过 `LARK_STREAMING_CARD_UPDATE_INTERVAL` 做节流（默认 400ms）
- 仅支持更新应用发送的“共享卡片”消息，卡片 `config.update_multi` 必须为 true
- 卡片/富文本消息请求体大小限制为 30KB，网关用 `LARK_STREAMING_CARD_MAX_BYTES` 做展示截断，避免无限增长导致 patch 失败
- 当前实现不会持久化 `message_id`：worker 崩溃/重启后无法继续更新同一条历史卡片，会在重试时重新发一条新卡片

bot -> agent 的映射与 llm model 的选择由 core-service 统一管理：message-gateway 只负责把 `X-Bot-Id` 等上下文 header 透传给 core-service，由 core-service 在内部完成 agent 路由与模型路由。

core service 需要提供一个“流式响应”的 HTTP 接口（路径由 `CORE_STREAM_PATH` 控制），网关发起请求时会带上这些 header：

| Header | 说明 |
|--------|------|
| `X-Bot-Id` | bot 标识（优先取事件 header 的 app_id；回退到 `LARK_APP_ID`） |
| `X-Session-Id` | 会话标识（优先 `chat_id`，否则 `sender_open_id`） |
| `X-User-Id` | 用户标识（优先取事件内的 user_id / employee_id） |
| `X-Open-Id` | 用户 open_id（如果有） |
| `X-Chat-Id` | chat_id（如果有） |
| `X-Message-Id` | message_id（如果有） |
| `X-Event-Id` | event_id（如果有） |

请求 body 为 JSON：`{"envelope": <GatewayEnvelope>}`。

响应支持两种形式：

- SSE：`Content-Type: text/event-stream`，每条 `data:` 行可以是纯文本增量或 JSON `{"text":"...","done":false}`，也支持 `data: [DONE]` 结束
- NDJSON/JSON：每行 JSON `{"text":"...","done":false}` 或单条 JSON/文本

## 环境变量

| 变量 | 说明 |
|------|------|
| `HTTP_ADDR` | HTTP 监听地址，默认 `:8080` |
| `DATABASE_URL` | PostgreSQL 连接串 |
| `LARK_APP_ID` | Lark 应用 App ID |
| `LARK_APP_SECRET` | Lark 应用 App Secret |
| `LARK_VERIFICATION_TOKEN` | URL 验证 token，可选 |
| `LARK_ENCRYPT_KEY` | 事件加密 key（控制台开启加密时必填） |
| `LARK_OPEN_BASE_URL` | OpenAPI 域名，默认 `https://open.larksuite.com` |
| `ADMIN_CONFIG_BASE_URL` | admin-console 基础地址，例如 `http://admin-console:50083` |
| `ADMIN_MESSAGE_BOTS_PATH` | admin runtime bot 配置路径，默认 `/api/runtime/message-gateway/bots` |
| `ADMIN_MESSAGE_ROUTES_PATH` | admin runtime 路由配置路径，默认 `/api/runtime/message-gateway/routes` |
| `LARK_BOTS_PATH` | 多 bot 配置文件路径（JSON）；未配置 admin 时作为回退来源 |
| `LARK_WS_ENABLED` | 是否启用“长连接模式”接收事件（true/false），默认 false |
| `LARK_STREAMING_CARD_ENABLED` | 是否启用“端到端流式卡片更新”，默认 true |
| `LARK_STREAMING_CARD_UPDATE_INTERVAL` | 卡片更新节流间隔（需满足单条消息 5QPS 限制），默认 `400ms` |
| `LARK_STREAMING_CARD_MAX_BYTES` | 卡片展示内容最大字节数（超出会截断展示末尾），默认 `20480` |
| `CORE_BASE_URL` | 下游 core service 基础地址（为空则不转发），例如 `http://core-service:8000` |
| `CORE_STREAM_PATH` | core service 流式接收接口路径，默认 `/v1/messages:stream` |
| `CORE_TIMEOUT` | core service 请求超时，默认 `60s` |
| `ROUTE_RULES_PATH` | 路由规则 JSON 文件路径（为空则禁用），未配置 admin 时作为回退来源 |
| `ROUTE_RULES_RELOAD_INTERVAL` | 路由规则热加载轮询间隔，默认 `2s` |
| `WORKER_POLL_INTERVAL` | job 轮询间隔，默认 `2s` |
| `WORKER_BATCH_SIZE` | 每次拉取 job 数量，默认 `10` |
| `WORKER_MAX_ATTEMPTS` | 最大重试次数，默认 `8` |
| `WORKER_RETRY_BASE_INTERVAL` | 重试基准退避，默认 `5s` |

## 本地运行

先确保共享基础设施已启动：

```bash
cd /Users/zxz/AI/infra/storage
docker compose up -d
```

再启动服务：

```bash
cd /Users/zxz/AI/message-gateway
go mod tidy
go run ./cmd/message-gateway
```

也可以在仓库根目录使用统一脚本，并通过 `message-gateway/.env.local` 管理本地环境变量：

```bash
cp /Users/zxz/AI/message-gateway/.env.local.example /Users/zxz/AI/message-gateway/.env.local
./deploy/local/start.sh message-gateway
```

如果本机没有安装 Go，可以使用 Docker：

```bash
docker run --rm \
  -v /Users/zxz/AI/message-gateway:/app \
  -w /app \
  --network host \
  golang:1.23 \
  /bin/sh -lc '/usr/local/go/bin/go mod tidy && /usr/local/go/bin/go run ./cmd/message-gateway'
```

## Lark 配置说明

- 事件订阅地址配置为：`POST /callbacks/feishu`
- 首阶段仅支持文本消息入站
- 机器人需要具备消息能力，并在目标会话中可发言

## 后续建议

- 将卡片模板扩展为“按钮建议 / 引用来源 / 复制”等富交互能力
- 增加 Redis：短期去重、限流与映射缓存
- 将路由规则 action 扩展为“选择本地回复 / 转发 core / 两者并行”
