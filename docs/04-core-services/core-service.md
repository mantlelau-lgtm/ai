# Core Service 详细设计

> `Core Service` 是 Tracer 2.0 的核心编排服务，负责请求主流程、记忆管理、状态持久化和跨能力协调。

## 1. 服务概述

| 属性 | 值 |
|------|-----|
| 服务名称 | `core-service` |
| gRPC 端口 | `50052` |
| Proto 定义 | `core_service.proto` |
| 依赖存储 | PostgreSQL + Qdrant + Redis |

### 核心职责

- 任务编排：协调 `Prompt Engine`、`LLM Gateway`、`Tool SDK`
- 记忆管理：维护短期记忆、长期记忆和会话历史
- 状态持久化：保存任务执行状态、对话记录、追踪和 Token 使用信息

## 2. gRPC 接口

| 接口 | 用途 |
|------|------|
| `ProcessRequest` | 处理用户请求主入口 |
| `GetTaskStatus` | 查询任务状态 |
| `CancelTask` | 取消正在执行的任务 |
| `GetConversationHistory` | 获取对话历史 |
| `SearchMemories` | 向量检索相似记忆 |
| `SaveLongTermMemory` | 保存长期记忆 |

## 3. 内部模块

```text
core-service/
├── handler/        # gRPC Handler
├── orchestrator/   # 编排引擎与工作流执行
├── memory/         # 短期记忆、长期记忆、摘要
└── workflow/       # Agent 工作流与工具调用工作流
```

## 4. 主处理流程

1. 接收 `ProcessRequest`
2. 创建任务记录并加载会话上下文
3. 获取对话历史和相关记忆
4. 调用 `Prompt Engine` 渲染输入
5. 调用 `LLM Gateway` 获取模型输出
6. 若有工具调用意图，则通过 `Tool SDK` 执行并继续编排
7. 保存消息、任务状态和长期记忆
8. 返回最终响应

## 5. 数据存储设计

### PostgreSQL

| 表名 | 用途 |
|------|------|
| `agents` | Agent 元数据与配置 |
| `conversations` | 会话基本信息 |
| `messages` | 对话消息 |
| `tasks` | 任务执行日志与追踪 |
| `token_usage` | Token 使用记录 |

### Redis

- `SESS:{conversation_id}`：会话缓存
- `MEMORY:SHORT:{conversation_id}`：短期记忆
- `LOCK:task:{request_id}`：任务锁

### Qdrant

- Collection：`long_term_memories`
- 向量维度：`1536`
- 距离度量：`Cosine`
- 用途：长期记忆、语义记忆与程序记忆检索

## 6. 记忆模型

| 类型 | 存储 | 用途 |
|------|------|------|
| 工作记忆 | Redis | 当前会话上下文 |
| 情景记忆 | PostgreSQL | 历史对话记录 |
| 语义记忆 | Qdrant | 事实性知识与知识片段 |
| 程序记忆 | Qdrant | 工具与技能使用模式 |

## 7. 错误与重试

| 错误码 | 说明 |
|--------|------|
| `CORE_001` | 任务未找到 |
| `CORE_002` | 任务已取消 |
| `CORE_003` | 上下文过期 |
| `CORE_004` | 记忆检索失败 |
| `CORE_005` | 编排失败 |

- `LLM Gateway` 超时：指数退避，最多 3 次
- `Prompt Engine` 失败：直接返回错误
- 记忆系统失败：允许降级为仅依赖 PostgreSQL 历史记录

## 8. 监控指标

```text
core_tasks_total{agent_id, status}
core_tasks_duration_seconds{agent_id, quantile}
core_memory_search_duration_seconds{quantile}
core_memory_search_results_total{type}
core_llm_calls_total{model}
core_llm_tokens_total{model, type}
```

## 9. 当前注意事项

- 设计上依赖 Qdrant，但当前本地基础设施尚未落地，需要补齐后才能完整闭环长期记忆能力
- 与消息网关的职责边界应保持清晰：网关只做轻编排与可靠投递，复杂 AI 逻辑统一由 Core Service 处理
