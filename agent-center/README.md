# agent-center

`agent-center` 是一个独立的 agent 注册中心服务，用来统一登记和管理本机开发出来的 agent。

## 当前能力

- `GET /healthz`
- `POST /api/agents/register`
- `GET /api/agents/registered`
- `GET /api/agents/{name}`
- `PUT /api/agents/{name}`
- `DELETE /api/agents/{name}`
- `POST /api/agents/{name}/heartbeat`
- `POST /api/agents/{name}/offline`
- `GET /api/agents`
- `GET /api/runtime/agents`

## 设计目标

- 统一登记本机 agent 的基础信息，例如名称、类型、工具、工作目录、运行地址
- 提供一个稳定的查询入口，方便后续接入 `admin-console`、本地 runtime 或调试脚本
- 用心跳更新时间和状态，区分“已注册”和“当前在线”的 agent

## 数据模型

每个 agent 当前会保存这些核心字段：

- `name`
- `type`
- `source`
- `description`
- `key_name`
- `is_default`
- `tools`
- `runtime_url`
- `workspace_path`
- `entrypoint`
- `owner`
- `tags`
- `metadata`
- `enabled`
- `status`
- `last_seen_at`

## 环境变量

| 变量 | 说明 |
|------|------|
| `LISTEN_ADDR` | HTTP 监听地址，默认 `:9999` |
| `DATABASE_URL` | PostgreSQL 连接串 |
| `ADMIN_TOKEN` | 管理接口 Bearer Token |
| `AGENT_OFFLINE_TIMEOUT` | 心跳离线超时时间，默认 `90s` |

## 本地运行

```bash
cp agent-center/.env.local.example agent-center/.env.local
go mod tidy
go run ./cmd/agent-center
```

## REST 接入方案

推荐所有 agent 服务统一通过 HTTP REST 接入，不依赖 SDK。

- 方案文档：`docs/04-core-services/agent-center-rest-integration.md`
- 标准流程：启动时注册 -> 立即心跳上线 -> 定时心跳保活 -> 退出前主动下线
- 适用语言：Go、Python、Node.js、Java 及任意可发 HTTP 请求的运行时

## 核心接口

- 注册 agent：`POST /api/agents/register`
- 心跳保持：`POST /api/agents/{name}/heartbeat`
- 主动下线：`POST /api/agents/{name}/offline`
- 已注册列表：`GET /api/agents/registered`
- 运行时可用列表：`GET /api/runtime/agents`，仅返回当前在线且可转发的 agent
- `is_default=true` 表示该 agent 是默认回退 agent，注册中心同时只允许存在一个默认 agent；后注册为默认的 agent 会覆盖之前的默认标记

## 注册示例

```bash
curl -X POST http://localhost:9999/api/agents/register \
  -H 'Authorization: Bearer change-me' \
  -H 'Content-Type: application/json' \
  -d '{
    "agent": {
      "name": "atlas",
      "type": "research",
      "source": "local",
      "description": "local research assistant",
      "is_default": false,
      "tools": ["market.quote", "strategy.backtest"],
      "runtime_url": "http://127.0.0.1:7001",
      "entrypoint": "python -m atlas.server",
      "tags": ["finance", "research"],
      "metadata": {
        "language": "python"
      },
      "enabled": true,
      "status": "registered"
    }
  }'
```

## 心跳示例

```bash
curl -X POST http://localhost:9999/api/agents/atlas/heartbeat \
  -H 'Authorization: Bearer change-me' \
  -H 'Content-Type: application/json' \
  -d '{"status":"online"}'
```

如果心跳请求体为空或未传 `status`，服务端会默认把这次心跳视为 `online`。

## 主动下线示例

```bash
curl -X POST http://localhost:9999/api/agents/atlas/offline \
  -H 'Authorization: Bearer change-me'
```

建议 agent 在进程退出、重启前或健康检查失败准备摘流时先调用这个接口，避免等待 TTL 超时后才被移出运行时列表。

## 推荐接入时序

1. 调用 `POST /api/agents/register` 注册基础信息
2. 调用 `POST /api/agents/{name}/heartbeat` 将状态推进到 `online`
3. 按固定间隔持续发送心跳
4. 退出、重启、摘流前调用 `POST /api/agents/{name}/offline`

详细字段、状态语义、Python 示例和错误处理建议见 `docs/04-core-services/agent-center-rest-integration.md`。

## 已注册列表示例

```bash
curl http://localhost:9999/api/agents/registered \
  -H 'Authorization: Bearer change-me'
```
