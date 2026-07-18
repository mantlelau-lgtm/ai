# robot-d

`robot-d` 是一个默认回退 agent。

## 设计目标

- 通过 `agent-center` 自动注册为默认 agent（`is_default=true`）
- 只处理当前这一次输入，不记录上下文，不保存历史会话
- 内部直接调用 `llm-gateway` 做单轮问答
- 对外提供兼容 `message-gateway` 的 `POST /v1/messages:stream` 接口

## 接口

- `GET /healthz`
- `POST /v1/messages:stream`

请求体格式：

```json
{
  "envelope": {
    "event_id": "evt_xxx",
    "message_id": "om_xxx",
    "bot_id": "cli_xxx",
    "chat_id": "oc_xxx",
    "message_type": "text",
    "text": "你好"
  }
}
```

响应格式：

```json
{
  "text": "你好，我是 robot-D。",
  "done": true
}
```

## 环境变量

- `HTTP_ADDR`: HTTP 监听地址，默认 `:7004`
- `ROBOT_D_AGENT_NAME`: 注册到 `agent-center` 的 agent 名称，默认 `robot-D`
- `ROBOT_D_AGENT_DESCRIPTION`: agent 描述
- `ROBOT_D_RUNTIME_URL`: 注册到 `agent-center` 的 runtime 地址
- `ADMIN_TOKEN`: `agent-center` 管理 token
- `AGENT_CENTER_BASE_URL`: `agent-center` 地址
- `ROBOT_D_HEARTBEAT_INTERVAL`: 心跳间隔，默认 `20s`
- `LLM_GATEWAY_BASE_URL`: `llm-gateway` 地址
- `ROBOT_D_LLM_MODEL`: 可选，固定使用的模型；为空时自动取 `llm-gateway /v1/models` 第一项
- `ROBOT_D_LLM_KEY_NAME`: 可选，透传为 `X-LLM-Key`
- `ROBOT_D_SYSTEM_PROMPT`: 默认系统提示词

## 本地运行

```bash
cp robot-d/.env.local.example robot-d/.env.local
go run ./cmd/robot-d
```
