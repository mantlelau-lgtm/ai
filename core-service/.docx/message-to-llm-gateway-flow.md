# core-service 消息到 LLM Gateway 流转路径

## 总览

```text
message-gateway
  ↓ HTTP POST /v1/messages:stream
core-service server.py
  ↓
Envelope 解析
  ↓
Orchestrator.process_stream()
  ↓
路由解析 agent
  ↓
创建/更新 conversation
  ↓
创建 task
  ↓
写入 user message
  ↓
读取历史消息 history
  ↓
构造 AgentContext
  ↓
调用 Agent.stream_reply()
  ↓
GeneralAgent
  ↓
LLM tool decision / tool calling
  ↓
LLMGatewayClient
  ↓ HTTP POST /v1/chat/completions
llm-gateway
  ↓ SSE / JSON
core-service
  ↓
聚合 assistant answer
  ↓
写入 assistant message
  ↓
task succeeded
  ↓ SSE 返回给 message-gateway
```

## 1. core-service 接收 message-gateway 请求

入口文件：`core_service/server.py`

接口：

```http
POST /v1/messages:stream
```

core-service 从请求体读取 envelope：

```python
body = await request.json()
raw_env = body.get("envelope") or {}
envelope = Envelope.from_dict(raw_env)
```

同时读取 headers：

```text
X-Bot-Id
X-Agent-Name
X-Session-Id
X-User-Id
X-Open-Id
X-Chat-Id
X-Trace-Id
X-Request-Id
```

然后交给：

```python
orchestrator.process_stream(...)
```

## 2. Envelope 标准化

定义文件：`core_service/models.py`

Envelope 会把 message-gateway 的原始消息统一成 core-service 内部结构：

```python
Envelope(
    event_id,
    event_type,
    kind,
    chat_id,
    chat_type,
    message_id,
    message_type,
    sender_open_id,
    sender_user_id,
    sender_union_id,
    tenant_key,
    text,
    action_name,
    action_tag,
    action_token,
    input_value,
    trace_id,
)
```

后续 core-service 不直接依赖飞书原始事件，而是统一依赖 Envelope。

## 3. Orchestrator 处理主流程

入口文件：`core_service/orchestrator.py`

核心方法：

```python
async def process_stream(...)
```

### 3.1 初始化 conversation / request / task

```python
conversation_id = self._pick_conversation_id(header_session_id, envelope)
request_id = header_request_id or envelope.event_id or req-uuid
task_id = task-uuid
```

conversation 选择优先级：

```text
header_session_id
→ envelope.chat_id
→ envelope.sender_user_id
→ envelope.sender_open_id
```

## 4. 提取用户输入

提取顺序：

```text
text
→ input_value
→ action_name
→ action_tag
```

如果没有有效输入，core-service 会返回 error SSE：

```text
empty envelope input
```

## 5. 解析 agent 路由

如果存在 routing 配置：

```python
agent_name = routing.current.lookup_agent_name(bot_id)
```

否则默认使用：

```text
general
```

然后获取 agent：

```python
agent = self._registry.get(agent_name)
```

同时获取 agent 绑定的 LLM key：

```python
llm_key_name = routing.current.get_agent_key_name(agent_name)
```

这个 `llm_key_name` 会作为 `X-LLM-Key` 传给 llm-gateway。

## 6. 写入 conversation / task / user message

### 6.1 upsert conversation

```python
await self._store.upsert_conversation(...)
```

记录：

```text
conversation_id
bot_id
user_id
open_id
chat_id
```

### 6.2 创建 task

```python
await self._store.create_task(...)
```

初始状态：

```text
running
```

### 6.3 写入用户消息

```python
await self._store.append_message(
    role="user",
    content=user_input,
    event_id=envelope.event_id,
    message_id=envelope.message_id,
)
```

## 7. 读取历史消息并构造 AgentContext

读取最近 N 条历史：

```python
history_rows = await self._store.list_recent_messages(
    conversation_id,
    cfg.conversation_window_size,
)
```

构造 AgentContext：

```python
AgentContext(
    conversation_id,
    agent_name,
    bot_id,
    user_id,
    open_id,
    chat_id,
    trace_id,
    request_id,
    user_input,
    history,
    envelope,
    llm_key_name,
)
```

AgentContext 是 agent 执行时的完整上下文。

## 8. 执行 Agent

orchestrator 先返回 start chunk：

```python
yield StreamChunk(type="start")
```

然后执行 agent：

```python
async for delta, usage in agent.stream_reply(agent_ctx):
    answer_parts.append(delta)
    yield StreamChunk(type="delta", text=delta)
```

agent 只负责产出 delta；orchestrator 负责：

- SSE 输出
- 聚合完整 assistant 回复
- usage 记录
- task 状态维护
- 消息持久化

## 9. GeneralAgent 处理路径

文件：`core_service/agents.py`

当前 `general` 是默认 LLM agent。

### 9.1 显式 tool 调用

如果用户输入：

```text
/tool context.info
```

或：

```text
tool:context.info
```

会直接执行 tool：

```python
tool_call = self._parse_explicit_tool_call(ctx.user_input)
result = await self._tools.run(tool_call, ctx)
```

然后返回工具结果，不进入 LLM。

### 9.2 自动 tool calling

普通消息会进入：

```python
_stream_with_auto_tools(...)
```

流程：

```text
构造 messages
  ↓
带 tools schema 调用 LLM chat_once
  ↓
LLM 判断是否需要 tool_calls
  ↓
如果有 tool_calls:
    core-service 执行本地 tool
    将结果追加为 role=tool 消息
    再次调用 LLM stream_chat_events 生成最终回复
  ↓
如果没有 tool_calls:
    直接返回 LLM content
    或 fallback 到 stream_chat_events
```

## 10. 构造 LLM messages

文件：`core_service/llm_gateway.py`

```python
messages = build_chat_messages(cfg.system_prompt, ctx.history)
```

结构：

```text
system prompt
+ conversation history
```

history 中包含刚刚写入的 user message。

## 11. 调用 llm-gateway

### 11.1 第一次非流式调用：tool decision

```python
chat_once(
    model=model,
    messages=messages,
    headers=headers,
    tools=tool_schemas,
    tool_choice="auto",
)
```

请求到：

```http
POST {LLM_BASE_URL}/v1/chat/completions
```

请求体类似：

```json
{
  "model": "deepseek-main",
  "messages": [],
  "stream": false,
  "tools": [],
  "tool_choice": "auto"
}
```

headers：

```text
X-User-Id
X-Session-Id
X-Trace-Id
X-Request-Id
X-LLM-Key
```

### 11.2 如果 LLM 返回 tool_calls

core-service 执行本地 tool：

```python
result = await self._tools.run(tool_call, ctx)
```

然后追加 role=tool 消息：

```python
{
    "role": "tool",
    "tool_call_id": "...",
    "name": "fs.read",
    "content": "..."
}
```

再调用流式 LLM：

```python
stream_chat_events(...)
```

### 11.3 如果 LLM 不调用 tool

如果第一次 `chat_once` 已经返回普通内容：

```python
content = message.get("content")
```

则直接返回。

如果没有内容，则 fallback 到：

```python
stream_chat_events(...)
```

## 12. llm-gateway 处理请求

llm-gateway 接收：

```http
POST /v1/chat/completions
```

处理流程：

```text
解析 model
↓
根据 X-LLM-Key / model 解析 provider/key/model route
↓
转发到上游模型
↓
返回 OpenAI-compatible response 或 SSE
```

llm-gateway 当前已支持透传：

```text
tools
tool_choice
tool_calls
tool_call_id
role=tool
```

## 13. 聚合 assistant 回复并持久化

agent 返回 delta 后，orchestrator 聚合：

```python
answer_parts.append(delta)
assistant = "".join(answer_parts).strip()
```

如果 assistant 非空：

```python
await self._store.append_message(
    role="assistant",
    content=assistant,
)
```

然后完成 task：

```python
await self._store.complete_task(task_id)
```

## 14. 返回给 message-gateway

最终返回 done chunk：

```python
StreamChunk(
    type="done",
    text=assistant,
    done=True,
    request_id=request_id,
    task_id=task_id,
    usage=last_usage,
)
```

最后输出：

```text
data: [DONE]
```

message-gateway 收到 SSE 后负责发送飞书回复。

## 15. 当前 core-service 在链路中的定位

当前 core-service 已经承担：

```text
消息标准化
+ agent 路由
+ conversation 管理
+ task 管理
+ message 持久化
+ tool calling
+ llm-gateway 编排
+ SSE 输出
```

它不直接持有具体 LLM provider 的 key，也不直接调用上游模型；这些由 llm-gateway 负责。

## 简化时序图

```text
message-gw
  |
  | POST /v1/messages:stream
  v
server.py
  |
  | Envelope.from_dict()
  v
Orchestrator.process_stream()
  |
  | resolve agent + llm_key_name
  | upsert conversation
  | create task
  | append user message
  | load history
  v
AgentContext
  |
  v
GeneralAgent.stream_reply()
  |
  | chat_once(tools=..., tool_choice=auto)
  v
llm-gw /v1/chat/completions
  |
  | tool_calls?
  v
core-service ToolRegistry.run()
  |
  | append role=tool message
  v
llm-gw /v1/chat/completions stream
  |
  | SSE delta
  v
Orchestrator
  |
  | append assistant message
  | complete task
  v
SSE done
  |
  v
message-gw
```
