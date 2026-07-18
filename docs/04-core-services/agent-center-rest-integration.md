# Agent Center REST 接入方案

## 目标

`agent-center` 作为统一的 agent 注册中心，对外只暴露 HTTP REST 接口。

其它 agent 服务接入时不需要依赖任何 SDK，也不需要额外 sidecar，只需要：

- 能发送 HTTP 请求
- 能保存自己的运行信息
- 能在启动、运行中、退出前分别调用注册、心跳、下线接口

适用语言包括但不限于：

- Go
- Python
- Node.js
- Java
- Shell / 运维脚本

## 接入原则

- 协议优先：所有语言统一按 REST 协议接入，不绑定某种 SDK 实现
- 生命周期清晰：启动注册、运行保活、退出下线
- 服务端判定在线状态：agent 只负责按时上报，离线 TTL 由 `agent-center` 统一计算
- 幂等优先：重复注册同名 agent 会走 upsert，允许重启覆盖
- 运行时隔离：注册信息保留，是否可参与转发由在线状态和 `runtime_url` 决定

## 基础信息

- 默认地址：`http://host:9999`
- 管理鉴权：`Authorization: Bearer <ADMIN_TOKEN>` 或 `X-Admin-Token: <ADMIN_TOKEN>`
- 内容类型：`application/json`

## 生命周期

标准接入时序如下：

1. agent 进程启动
2. 调用 `POST /api/agents/register` 注册基础信息
3. 调用 `POST /api/agents/{name}/heartbeat` 上报 `online`
4. 按固定间隔持续发送心跳
5. 进程退出、重启、摘流前调用 `POST /api/agents/{name}/offline`

如果 agent 异常退出，没有主动下线，`agent-center` 会在超过 `AGENT_OFFLINE_TIMEOUT` 后自动把其视为 `offline`。

## 数据模型

注册时可提交的字段如下：

| 字段 | 必填 | 说明 |
|------|------|------|
| `name` | 是 | agent 唯一名称，建议全局唯一、稳定不变 |
| `type` | 否 | agent 类型，例如 `research`、`assistant`、`workflow` |
| `source` | 否 | 来源，默认 `local` |
| `description` | 否 | agent 描述 |
| `key_name` | 否 | 关联的密钥标识 |
| `is_default` | 否 | 是否默认回退 agent；同一时刻仅允许一个默认 agent |
| `tools` | 否 | 支持的工具列表 |
| `runtime_url` | 否 | agent 对外运行地址，消息转发依赖此字段 |
| `workspace_path` | 否 | 本地工作目录 |
| `entrypoint` | 否 | 启动命令或入口说明 |
| `owner` | 否 | 负责人 |
| `tags` | 否 | 标签列表 |
| `metadata` | 否 | 附加元数据，键值对形式 |
| `enabled` | 是 | 是否启用；`false` 时会被视为 `disabled` |
| `status` | 否 | 初始状态，默认 `registered` |

## 状态语义

`agent-center` 当前约定的状态如下：

| 状态 | 含义 |
|------|------|
| `registered` | 已注册，但尚未进入在线可转发状态 |
| `online` | 在线，可被运行时列表返回 |
| `offline` | 主动下线或超时离线 |
| `disabled` | 被禁用，不参与运行时转发 |
| `unavailable` | 已知不可用，但保留注册信息 |

运行时可转发 agent 需要同时满足：

- `enabled = true`
- `status = online`
- `runtime_url` 非空

## 接口说明

### 1. 注册

- 方法：`POST`
- 路径：`/api/agents/register`
- 作用：注册或覆盖同名 agent 的基础信息

请求示例：

```bash
curl -X POST http://127.0.0.1:9999/api/agents/register \
  -H 'Authorization: Bearer change-me' \
  -H 'Content-Type: application/json' \
  -d '{
    "agent": {
      "name": "atlas",
      "type": "research",
      "source": "local",
      "description": "python research agent",
      "is_default": false,
      "tools": ["search.code", "market.quote"],
      "runtime_url": "http://127.0.0.1:7001",
      "workspace_path": "/workspace/atlas",
      "entrypoint": "python -m atlas.server",
      "owner": "rocky",
      "tags": ["research", "python"],
      "metadata": {
        "language": "python",
        "framework": "fastapi"
      },
      "enabled": true,
      "status": "registered"
    }
  }'
```

返回示例：

```json
{
  "registered": 1,
  "agents": [
    {
      "name": "atlas",
      "type": "research",
      "source": "local",
      "description": "python research agent",
      "tools": ["search.code", "market.quote"],
      "runtime_url": "http://127.0.0.1:7001",
      "workspace_path": "/workspace/atlas",
      "entrypoint": "python -m atlas.server",
      "owner": "rocky",
      "tags": ["research", "python"],
      "metadata": {
        "framework": "fastapi",
        "language": "python"
      },
      "enabled": true,
      "status": "registered"
    }
  ]
}
```

说明：

- 同名注册会覆盖旧配置，不需要先删除再创建
- 建议 `name`、`runtime_url`、`enabled` 在首次接入时固定下来

### 2. 心跳

- 方法：`POST`
- 路径：`/api/agents/{name}/heartbeat`
- 作用：更新 `last_seen_at`，并把 agent 标记为当前在线状态

请求示例：

```bash
curl -X POST http://127.0.0.1:9999/api/agents/atlas/heartbeat \
  -H 'Authorization: Bearer change-me' \
  -H 'Content-Type: application/json' \
  -d '{"status":"online"}'
```

也可以发送空对象：

```bash
curl -X POST http://127.0.0.1:9999/api/agents/atlas/heartbeat \
  -H 'Authorization: Bearer change-me' \
  -H 'Content-Type: application/json' \
  -d '{}'
```

说明：

- 如果未传 `status`，服务端会默认视为 `online`
- 建议心跳间隔小于 `AGENT_OFFLINE_TIMEOUT` 的一半
- 例如超时配置为 `90s`，心跳建议 `20s` 到 `30s`

### 3. 主动下线

- 方法：`POST`
- 路径：`/api/agents/{name}/offline`
- 作用：在退出、重启、摘流前显式把 agent 标记为 `offline`

请求示例：

```bash
curl -X POST http://127.0.0.1:9999/api/agents/atlas/offline \
  -H 'Authorization: Bearer change-me'
```

说明：

- 推荐在进程收到 `SIGTERM`、发布摘流、运行异常准备退出时调用
- 主动下线后，agent 会立刻从运行时可用列表中移除

### 4. 查询运行时可用列表

- 方法：`GET`
- 路径：`/api/runtime/agents`
- 作用：返回当前可参与消息转发的在线 agent

请求示例：

```bash
curl http://127.0.0.1:9999/api/runtime/agents
```

返回示例：

```json
{
  "agents": [
    {
      "name": "atlas",
      "type": "research",
      "source": "local",
      "runtime_url": "http://127.0.0.1:7001",
      "enabled": true,
      "status": "online"
    }
  ]
}
```

说明：

- 这个接口不要求管理鉴权
- `message-gateway` 等运行时消费者应优先使用这个接口，而不是使用全量管理接口

### 5. 查询已注册列表

- 方法：`GET`
- 路径：`/api/agents/registered`
- 作用：返回全部已注册 agent，包括未在线、离线、禁用项

请求示例：

```bash
curl http://127.0.0.1:9999/api/agents/registered \
  -H 'Authorization: Bearer change-me'
```

## 接入时序示例

### Python / 任意语言通用伪代码

```text
on_start:
  POST /api/agents/register
  POST /api/agents/{name}/heartbeat {"status":"online"}
  start heartbeat timer
  start agent server

heartbeat_loop:
  every 30s:
    POST /api/agents/{name}/heartbeat {}

on_shutdown:
  stop heartbeat timer
  POST /api/agents/{name}/offline
  stop agent server
```

### Python 示例

```python
import atexit
import os
import signal
import threading
import time

import requests

BASE_URL = os.getenv("AGENT_CENTER_BASE_URL", "http://127.0.0.1:9999")
TOKEN = os.getenv("ADMIN_TOKEN", "change-me")
AGENT_NAME = "atlas"

HEADERS = {
    "Authorization": f"Bearer {TOKEN}",
    "Content-Type": "application/json",
}


def register():
    payload = {
        "agent": {
            "name": AGENT_NAME,
            "type": "research",
            "source": "local",
            "runtime_url": "http://127.0.0.1:7001",
            "enabled": True,
            "status": "registered",
            "metadata": {"language": "python"},
        }
    }
    requests.post(f"{BASE_URL}/api/agents/register", json=payload, headers=HEADERS, timeout=10).raise_for_status()


def heartbeat():
    requests.post(
        f"{BASE_URL}/api/agents/{AGENT_NAME}/heartbeat",
        json={},
        headers=HEADERS,
        timeout=10,
    ).raise_for_status()


def offline():
    requests.post(
        f"{BASE_URL}/api/agents/{AGENT_NAME}/offline",
        headers={"Authorization": f"Bearer {TOKEN}"},
        timeout=10,
    ).raise_for_status()


def heartbeat_loop(stop_event):
    while not stop_event.wait(30):
        heartbeat()


stop_event = threading.Event()

register()
heartbeat()
threading.Thread(target=heartbeat_loop, args=(stop_event,), daemon=True).start()


def shutdown(*_):
    stop_event.set()
    try:
        offline()
    finally:
        raise SystemExit(0)


atexit.register(lambda: not stop_event.is_set() and offline())
signal.signal(signal.SIGINT, shutdown)
signal.signal(signal.SIGTERM, shutdown)

# 在这里启动你自己的 agent HTTP 服务
while True:
    time.sleep(1)
```

## 错误处理建议

- 注册失败：阻止 agent 进入可服务状态，避免未注册实例对外工作
- 心跳失败：记录日志并重试；连续失败时可触发告警
- 下线失败：不阻塞退出，但要打印错误日志
- `404 not found`：通常表示 agent 名称未注册或已被清理，应重新注册
- `401 unauthorized`：检查 `ADMIN_TOKEN`
- `5xx`：视为注册中心临时异常，按退避策略重试

## 运行建议

- `name` 使用稳定值，不要每次启动都生成随机名称
- `runtime_url` 必须是其它服务可达的地址，不能只在本进程自用
- 心跳间隔建议固定，不要过短，避免注册中心被高频打点
- 下线动作放在优雅退出钩子中，而不是业务线程里随意触发
- `metadata` 只放辅助信息，不要放敏感密钥

## 与 message-gateway 的关系

对于会被 `message-gateway` 路由的 agent，必须满足：

- 已成功注册
- 已发送心跳并进入 `online`
- `runtime_url` 可访问
- 未被禁用

否则即使 agent 已存在于注册中心，也不会出现在 `/api/runtime/agents` 中。
