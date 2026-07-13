# core-service Agent 注册与发现机制

## 1. 总览

当前 agent 的注册与发现分为两层：

```text
core-service 内部发现
  ↓
AgentRegistry
  ↓
core-service 启动时主动注册到 admin-console
  ↓
admin-console 持久化到 admin_registered_agents
  ↓
admin-console 后台展示与配置 agent
  ↓
core-service 拉取 runtime routing
  ↓
消息进入时按 bot_id 选择 agent
```

核心结论：

```text
admin-console 负责保存、展示、配置 agent 元信息；
core-service 负责真正持有和执行 agent 代码。
```

## 2. core-service 内部如何发现 agent

core-service 内部通过 `AgentRegistry` 发现本地可用 agent。

文件：

```text
core_service/agents.py
```

当前注册表结构：

```python
class AgentRegistry:
    def __init__(self, tools: ToolRegistry | None = None) -> None:
        self._tools = tools or default_tool_registry()
        self._agents: dict[str, BaseAgent] = {
            GeneralAgent.name: GeneralAgent(self._tools),
            EchoAgent.name: EchoAgent(),
        }
```

当前内置 agent：

```text
general
echo
```

内部发现方式：

```python
registry.names()
```

获取 agent 实例：

```python
registry.get(agent_name)
```

如果传入空 agent name 或未知 agent，默认回退到：

```text
general
```

## 3. core-service 启动时如何注册到 admin-console

core-service 启动入口在：

```text
core_service/server.py
```

启动时会创建 registry：

```python
registry = AgentRegistry()
```

然后调用：

```python
await _register_agents_with_admin(registry)
```

注册方法：

```python
async def _register_agents_with_admin(registry: AgentRegistry) -> None:
    if not cfg.admin_config_base_url:
        return

    url = f"{cfg.admin_config_base_url}{cfg.admin_agents_register_path}"
    agents = []
    for name in registry.names():
        agent = registry.get(name)
        agents.append({
            "name": name,
            "type": agent.__class__.__name__.replace("Agent", "").lower(),
            "source": "core-service",
            "description": f"Built-in agent: {name}",
        })
```

然后 POST 到 admin-console：

```python
POST {ADMIN_CONFIG_BASE_URL}{ADMIN_AGENTS_REGISTER_PATH}
```

请求体：

```json
{
  "agents": [
    {
      "name": "general",
      "type": "general",
      "source": "core-service",
      "description": "Built-in agent: general"
    },
    {
      "name": "echo",
      "type": "echo",
      "source": "core-service",
      "description": "Built-in agent: echo"
    }
  ]
}
```

## 4. admin-console 如何保存 agent

admin-console 使用表：

```sql
admin_registered_agents
```

表结构核心字段：

```sql
name TEXT NOT NULL UNIQUE,
agent_type TEXT NOT NULL DEFAULT 'custom',
source TEXT NOT NULL DEFAULT '',
description TEXT NOT NULL DEFAULT '',
key_name TEXT NOT NULL DEFAULT '',
created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
```

字段含义：

| 字段 | 含义 |
|---|---|
| `name` | agent 名称，例如 `general`、`echo` |
| `agent_type` | agent 类型，例如 `general`、`echo`、`toolcalling` |
| `source` | 来源，当前为 `core-service` |
| `description` | 描述 |
| `key_name` | 绑定的 LLM key name |
| `updated_at` | 最近注册或更新的时间 |

注册是 upsert：

```sql
INSERT INTO admin_registered_agents(...)
ON CONFLICT (name) DO UPDATE
```

所以 core-service 多次重启不会重复创建 agent，只会更新已有 agent 元信息。

## 5. admin-console 如何展示 agent

admin-console 提供接口：

```http
GET /api/agents
```

返回来自：

```text
admin_registered_agents
```

示例返回：

```json
{
  "agents": [
    {
      "name": "general",
      "type": "general",
      "source": "core-service",
      "description": "Built-in agent: general",
      "key_name": "deepseek-main"
    }
  ]
}
```

后台页面通过这个接口发现和展示已注册 agent。

## 6. admin-console 如何提供 runtime routing

core-service 运行时不会直接从 admin-console 执行 agent，而是从 admin-console 拉取 routing 配置。

admin-console 提供：

```http
GET /api/runtime/core-service/routing
```

返回结构包括：

```text
default_agent
bots
agents
```

其中 `agents` 来自：

```text
admin_registered_agents
```

并携带每个 agent 绑定的：

```text
key_name
```

示例：

```json
{
  "default_agent": "general",
  "bots": [
    {
      "bot_id": "cli_xxx",
      "agent_name": "general"
    }
  ],
  "agents": [
    {
      "name": "general",
      "type": "general",
      "key_name": "deepseek-main"
    },
    {
      "name": "echo",
      "type": "echo",
      "key_name": ""
    }
  ]
}
```

## 7. core-service 如何拉取 routing

core-service 启动时会读取 admin-console runtime routing。

文件：

```text
core_service/server.py
```

逻辑：

```python
if cfg.admin_config_base_url:
    routing_url = f"{cfg.admin_config_base_url}{cfg.admin_core_routing_path}"
    routing_cfg = await load_routing_config_from_url(routing_url)
```

如果 admin-console 不可用，则回退本地 routing 配置：

```python
routing_cfg = load_routing_config(cfg.routing_config_path)
```

## 8. 消息进入时如何选择 agent

消息进入 core-service 后，`Orchestrator` 会根据 bot_id 查 routing。

流程：

```python
agent_name = routing.current.lookup_agent_name(bot_id)
agent = self._registry.get(agent_name)
```

如果 routing 配置中存在：

```text
bot A -> general
bot B -> echo
```

则消息会分别进入对应 agent。

同时，core-service 会获取 agent 绑定的 LLM key：

```python
llm_key_name = routing.current.get_agent_key_name(agent_name)
```

这个值后续会作为请求头传给 llm-gateway：

```text
X-LLM-Key: deepseek-main
```

## 9. 新增 agent 的注册流程

例如新增一个文档 agent：

```python
class DocsAgent(ToolCallingAgent):
    name = "docs"
    system_prompt = "你是文档检索和总结助手。"
```

需要加入 `AgentRegistry`：

```python
self._agents: dict[str, BaseAgent] = {
    GeneralAgent.name: GeneralAgent(self._tools),
    DocsAgent.name: DocsAgent(self._tools),
    EchoAgent.name: EchoAgent(),
}
```

然后重启 core-service。

重启后流程：

```text
core-service 启动
→ AgentRegistry.names() 包含 docs
→ POST /api/agents/register
→ admin_registered_agents upsert docs
→ admin-console 后台出现 docs
→ 可以为 docs 绑定 LLM key
→ 可以配置 bot -> docs 路由
→ core-service runtime routing 拉取生效
```

## 10. 当前机制的职责边界

### core-service 负责

```text
持有 agent 代码
执行 agent
注册 agent 元信息
拉取 runtime routing
根据 bot_id 选择 agent
根据 agent key_name 调用 llm-gateway
```

### admin-console 负责

```text
保存 agent 元信息
展示 agent 列表
配置 agent 绑定的 key_name
配置 bot -> agent 路由
提供 runtime routing
```

### llm-gateway 负责

```text
根据 X-LLM-Key 选择具体模型密钥
调用上游 LLM provider
返回 OpenAI-compatible 响应
```

## 11. 注册与发现完整时序

```text
core-service 启动
  ↓
AgentRegistry 初始化
  ↓
发现本地内置 agent: general / echo
  ↓
POST /api/agents/register
  ↓
admin-console upsert admin_registered_agents
  ↓
admin-console 后台展示 agent
  ↓
用户配置 bot -> agent 和 agent -> key_name
  ↓
core-service 拉取 /api/runtime/core-service/routing
  ↓
消息进入 core-service
  ↓
按 bot_id 查找 agent_name
  ↓
AgentRegistry.get(agent_name)
  ↓
执行对应 agent
```

## 12. 一句话总结

```text
AgentRegistry 是 core-service 内部发现机制；
/api/agents/register 是 core-service 向 admin-console 上报机制；
admin_registered_agents 是后台持久化与展示机制；
/api/runtime/core-service/routing 是运行时发现和路由机制。
```
