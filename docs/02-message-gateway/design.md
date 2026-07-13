# Message Gateway 设计方案

> `Message Gateway` 是 Tracer 2.0 的接入层，负责多渠道消息接入、标准化、路由和可靠出站。

## 1. 范围与目标

| 项目 | 说明 |
|------|------|
| 使用场景 | 内部使用的消息通路 |
| 交互形态 | 文本命令、@机器人、交互卡片、线程回复 |
| 可靠性 | 不使用 MQ，使用 `DB Job + Worker` |
| 部署方式 | Docker 容器化，依赖共享存储栈 |

### 核心目标

- 用统一入口承接 Feishu/Lark 等渠道事件
- 用标准 Envelope 屏蔽渠道差异
- 将耗时逻辑异步化，确保回调快速 ACK
- 对出站发送、内部调用和卡片更新提供重试能力
- 提供基本运维能力，包括健康检查、指标、DLQ 和 replay

### 非目标

- 不在网关中承载复杂业务计算和长链路 AI 编排
- 不追求 Exactly-once，以 At-least-once + 幂等为基线

## 2. 总体架构

```text
渠道回调
  -> Ingress
  -> Normalizer
  -> Router
  -> Handlers
  -> Job Store
  -> Worker Pool
  -> Dispatcher
  -> 渠道 / 内部系统 / Core Service
```

## 3. 组件职责

| 组件 | 职责 | 关键点 |
|------|------|--------|
| Ingress | 接收回调与内部请求，完成鉴权、限流和超时控制 | 快速返回 |
| Normalizer | 将文本、卡片等事件统一为标准 Envelope | 屏蔽渠道差异 |
| Router | 根据规则匹配 handler | 支持优先级与兜底 |
| Handlers | 调用内部服务或 Core Service，产出出站动作 | 仅做轻编排 |
| Worker | 消费 job，执行重试和失败流转 | 指数退避、DLQ |
| Dispatcher | 统一适配出站接口 | 限速、熔断、幂等 |

## 4. 消息模型与路由

### 统一 Envelope

| 字段 | 说明 |
|------|------|
| `event_id` | 渠道事件唯一标识，用于入站去重 |
| `channel` | 渠道来源，如 `feishu`、`lark` |
| `event_type` | 事件类型，如 `message_received`、`card_action` |
| `chat_id` | 会话或群组标识 |
| `thread_id` | 回复线程标识 |
| `message_id` | 消息标识 |
| `sender_id` | 外部用户标识 |
| `employee_id` | 映射后的内部用户标识 |
| `intent` | 解析后的命令或动作信息 |
| `trace_id` | 全链路追踪标识 |

### Intent 解析

- 文本消息：支持命令前缀、`@机器人`、参数解析和原文保留
- 卡片 Action：支持 `action_id`、`card_instance_id`、`form_value`

### 路由规则

```yaml
priority: 100
match:
  event_type: message_received
  command_prefix: /ticket
action:
  type: call_internal_http
  endpoint: http://ticket-service/api/v1/create
  reply: card
```

### 默认兜底

- 未命中规则时返回帮助卡片或帮助文本
- 参数解析失败时返回结构化错误和 `request_id`
- 业务侧返回可重试/不可重试错误，供网关决定是否重试

## 5. Feishu/Lark 入站

### 回调入口

| 接口 | 用途 |
|------|------|
| `POST /callbacks/feishu` | Feishu/Lark 事件回调统一入口 |
| `GET /admin/healthz` | 健康检查 |
| `GET /admin/metrics` | 指标采集 |

### 安全校验

- 校验回调签名或 token
- 校验时间戳窗口，防止重放
- 校验 `app_id` 和机器人身份
- 严禁将 `token`、`secret` 打印到日志中

### 去重与处理时序

1. 回调进入网关
2. 完成鉴权、标准化和去重
3. 对 `inbound_event.event_id` 做唯一约束校验
4. 写入 `job` 或派发轻量动作
5. 立即返回 `200 OK`
6. Worker 异步消费 job，调用内部系统或执行出站

## 6. 出站设计

| 能力 | 说明 | 关键字段 |
|------|------|----------|
| `send_message` | 发送文本或线程回复 | `chat_id`、`thread_id`、`content` |
| `send_card` | 发送交互卡片 | `card_json`、`variables` |
| `update_card` | 更新已有卡片实例 | `card_instance_id` |
| `upload_file` | 上传附件或图片 | `file_path` / `url` |

### 出站策略

- 先落库为 job，再由 Worker 执行
- 对发送消息、更新卡片、上传附件分别限速
- 429、超时、5xx 进入重试分支
- 连续失败时短期熔断，保护主流程

### 幂等建议

- 业务动作生成稳定 `idempotency_key`
- `job.dedup_key` 可加唯一约束
- 对重复通知优先更新卡片，而不是重复发送消息

## 7. 可靠性设计

### 核心表

| 表 | 用途 | 关键点 |
|------|------|--------|
| `inbound_event` | 入站去重与审计 | `event_id UNIQUE` |
| `job` | 异步任务队列 | 状态机、`next_run_at`、`attempts` |
| `mapping` | 线程与业务对象映射 | 按需启用 |

### job 状态机

- `pending`：等待执行
- `running`：已被 worker 取走
- `succeeded`：执行成功
- `failed`：本次失败，稍后重试
- `dead`：超过重试次数或不可重试错误，进入 DLQ

### 重试策略

| 类型 | 例子 | 处理 |
|------|------|------|
| 可重试 | 超时、5xx、429 | 指数退避 + 抖动 |
| 不可重试 | 参数错误、权限错误 | 直接转 `dead` |

## 8. 部署与运维

| 层次 | 目录 | 职责 |
|------|------|------|
| 基础设施层 | `/Users/zxz/AI/infra/storage` | PostgreSQL、Redis、MinIO |
| 业务层 | `/Users/zxz/AI/message-gateway` | 回调、路由、Worker、Dispatcher |

### 启动顺序

1. 启动 `infra/storage`
2. 启动 `message-gateway`
3. 校验 `/admin/healthz`、`/admin/metrics` 和数据库连接
4. 验证回调签名配置与渠道可达性

### 运维重点

- 关注 `event_id`、`job_id`、`route`、`duration_ms`、`error_code`
- 观察 DLQ 增长速度与错误类型分布
- 回放 dead job 前先修复依赖、权限或配置问题
- 管理接口仅允许内网访问
