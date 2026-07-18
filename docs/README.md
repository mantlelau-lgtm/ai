# Tracer 2.0 文档中心

> 本目录是 `Tracer 2.0` 微服务方案的本地主文档集，按“总体架构 -> 接入层 -> 编排能力 -> 基础设施”组织，作为飞书知识库的本地可维护版本。

## 目标

- 统一维护 Tracer 2.0 的架构设计与模块边界
- 收敛旧版 `*-doc.md`、`*-content.md`、`*-feishu.md` 等重复文档
- 将知识库中的消息网关与基础设施方案同步到本地 `docs`
- 显式区分“目标架构”和“当前已落地状态”

## 目录结构

| 目录 | 说明 |
|------|------|
| `01-overview/` | 总体架构、主链路、存储职责、路线图 |
| `02-message-gateway/` | 渠道接入、消息模型、路由、出站、可靠性、运维 |
| `03-llm-gateway/` | LLM Gateway（独立服务）：OpenAI 兼容接口、流式、路由、usage、密钥/模型管理 |
| `04-core-services/` | Prompt Engine、Tool SDK、Agent Center 接入方案等编排与能力服务文档 |
| `05-infrastructure/` | 共享存储栈、部署方式、现状差异与待补项 |

## 阅读顺序

1. 先读 `01-overview/system-architecture.md`
2. 再读 `02-message-gateway/design.md`
3. 再读 `03-llm-gateway/`（模型出口能力）
4. 然后读 `04-core-services/` 下的编排能力设计文档
5. 如需接入注册中心，再读 `04-core-services/agent-center-rest-integration.md`
6. 最后读 `05-infrastructure/storage-and-deployment.md`

## 当前结论

- 方案层面已形成完整的微服务架构闭环
- 本地 `infra/storage` 已落地 `PostgreSQL + Redis + MinIO`
- `Qdrant` 仍是设计依赖，但当前尚未加入本地基础设施
- 知识库中的消息网关分支已同步收敛为本地一份主设计文档
