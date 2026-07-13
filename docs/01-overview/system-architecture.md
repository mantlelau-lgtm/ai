# Tracer 2.0 总体架构

> `Tracer 2.0` 采用“消息接入 + AI 编排 + 模型网关 + 工具执行 + 共享基础设施”的微服务方案，用于支撑多渠道 AI Agent 场景。

## 1. 系统目标

- 支撑 Feishu/Lark 等聊天渠道接入与统一消息处理
- 通过 `Core Service` 完成多步骤任务编排与记忆管理
- 通过 `Prompt Engine` 实现 Prompt 模板化和上下文组装
- 通过 `LLM Gateway` 统一接入多模型服务与 Token 计费
- 通过 `Tool SDK` 提供可扩展的工具执行能力

## 2. 系统分层

| 层次 | 模块 | 核心职责 |
|------|------|----------|
| 接入层 | Message Gateway | 渠道回调接入、消息标准化、路由、出站发送 |
| 编排层 | Core Service | 请求主流程、任务状态、记忆检索、服务协调 |
| 能力层 | Prompt Engine / LLM Gateway / Tool SDK | Prompt 渲染、多模型调用、工具执行 |
| 存储层 | PostgreSQL / Redis / Qdrant | 结构化数据、缓存状态、长期记忆向量检索 |
| 运维层 | Health / Metrics / DLQ / Replay | 监控、诊断、恢复、运维闭环 |

## 3. 核心服务清单

| 模块 | 端口 | 说明 |
|------|------|------|
| Message Gateway | 50051 | 多渠道消息接入与出站网关 |
| Core Service | 50052 | AI 编排核心服务 |
| Prompt Engine | 50053 | Prompt 模板渲染与上下文组装 |
| LLM Gateway | 50054 | 多模型适配、路由、缓存和计费 |
| Tool SDK | - | 被 Core Service 直接引用的工具执行 SDK |

## 4. 主调用链路

```text
用户
  -> Message Gateway
  -> Core Service
  -> Prompt Engine
  -> LLM Gateway
  -> Tool SDK（按需调用）
  -> Core Service
  -> Message Gateway
  -> 用户
```

## 5. 数据与存储职责

| 存储 | 用途 |
|------|------|
| PostgreSQL | 对话、任务、模板、Token 使用、网关入站事件和异步任务 |
| Redis | 会话缓存、Prompt 缓存、LLM 响应缓存、短期记忆、限速与锁 |
| Qdrant | 长期记忆、语义记忆、程序记忆向量检索 |
| MinIO | 附件、图片等对象暂存 |

## 6. 关键架构决策

- 服务间采用 gRPC 和 Protocol Buffers 定义接口
- 当前阶段使用 `DB Job + Worker` 而不是 MQ 作为可靠异步机制
- 网关只做轻编排与可靠投递，复杂 AI 逻辑统一由 Core Service 处理
- 以 `PostgreSQL + Redis + Qdrant` 作为长期目标存储组合

## 7. 当前现状与差异

| 项目 | 目标方案 | 当前本地现状 |
|------|----------|--------------|
| 核心模块设计 | 已完整定义 | 已形成结构化文档 |
| 消息网关专题 | 知识库已完整拆解 | 本地已同步收敛为主文档 |
| 共享存储栈 | PostgreSQL + Redis + MinIO + Qdrant | 当前仅落地 PostgreSQL + Redis + MinIO |
| 长期记忆闭环 | Core Service + LLM Gateway + Qdrant | 仍缺 Qdrant 实际部署 |

## 8. 实施路线图

| 阶段 | 核心任务 | 交付物 |
|------|----------|--------|
| Phase 1 | gRPC 接口与 Proto 定义 | 服务 Proto 与公共消息定义 |
| Phase 2 | Tool SDK 与工具沙箱 | 工具注册、执行、基础工具集 |
| Phase 3 | Core Service 与 Message Gateway | 编排主流程与接入链路 |
| Phase 4 | Prompt Engine 与 LLM Gateway | 模板渲染、多模型调用与计费 |
| Phase 5 | 存储集成 | PG/Redis/Qdrant 接入 |
| Phase 6 | 生产部署与运维 | 部署、指标、健康检查、回放能力 |
