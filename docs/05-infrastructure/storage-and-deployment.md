# 基础设施与部署设计

> 当前 Tracer 2.0 的本地基础设施以共享 Docker 栈为主，已落地 `PostgreSQL + Redis + MinIO`，并预留向量存储 `Qdrant` 的后续补齐空间。

## 1. 基础设施目标

- 为消息网关、核心服务等多个项目提供可复用的共享存储能力
- 将基础设施与业务代码解耦，避免每个服务重复维护一套 Docker 依赖
- 支撑任务状态、缓存、附件与后续长期记忆能力的本地开发与集成验证

## 2. 当前目录与文件

| 路径 | 用途 |
|------|------|
| `infra/storage/docker-compose.yml` | 共享存储栈编排 |
| `infra/storage/postgres-init/01-init.sql` | 初始化数据库与账号 |
| `infra/storage/.env.example` | 环境变量示例 |
| `infra/storage/README.md` | 启停与接入说明 |

## 3. 当前已落地组件

| 组件 | 状态 | 用途 |
|------|------|------|
| PostgreSQL | 已落地 | 持久化任务、会话、模板、网关事件 |
| Redis | 已落地 | 缓存、限速、短期去重、短期上下文 |
| MinIO | 已落地 | 附件与图片对象存储 |
| Qdrant | 未落地 | 长期记忆与向量检索 |

## 4. 当前 compose 结构

- `postgres`：默认启用，承载主持久化数据
- `redis`：通过 profile 启用，承载缓存和限速
- `minio`：通过 profile 启用，承载对象存储
- 网络统一使用 `infra_net`

## 5. 数据库初始化

当前初始化脚本会创建：

- `message_gateway` 数据库与 `mgw` 账号
- `content_system` 数据库与 `cms` 账号

这说明当前 infra 已优先服务消息网关与内容系统场景。

## 6. 与目标方案的差异

| 项目 | 目标方案 | 当前现状 |
|------|----------|----------|
| 长期记忆存储 | Qdrant | 尚未接入 |
| Core Service 依赖 | PG + Redis + Qdrant | 当前仅 PG + Redis 可支撑 |
| 对象存储 | MinIO | 已具备 |
| 共享网络 | `infra_net` | 已具备 |

## 7. 推荐补齐项

### P0

- 将 `Qdrant` 加入 `infra/storage/docker-compose.yml`
- 为 `Core Service` 增加指向 Qdrant 的环境配置与 README 示例
- 在初始化文档中补充向量存储用途和 collection 规划

### P1

- 为不同业务系统补齐数据库与账号隔离策略说明
- 为 Redis key 前缀和 MinIO bucket 命名建立统一约定
- 为基础设施补充备份、恢复和升级 Runbook

## 8. 推荐部署顺序

1. 启动 `infra/storage`
2. 校验 `postgres`、`redis`、`minio` 健康状态
3. 启动 `message-gateway` 和后续核心服务
4. 校验业务容器是否成功加入 `infra_net`
5. 逐步补齐 `Qdrant` 并联调长期记忆能力

## 9. 安全与运维建议

- 数据库、Redis 和 MinIO 凭据仅通过 `.env` 或 secret 注入
- 管理接口与监控面板优先走内网
- 升级前先备份数据卷，尤其是 PostgreSQL
- 基础设施故障排查优先从 `docker compose ps`、容器健康检查和连接串配置入手
