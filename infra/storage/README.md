# 共享存储中间件（基础设施栈）

当前仓库默认不再依赖这里的 Docker 中间件栈。业务服务现在默认直接连接宿主机已启动的 PostgreSQL。

这个目录保留为可选的历史基础设施方案，供需要临时自带存储容器时使用。

该目录提供一个可复用的 Docker 存储栈，供消息网关、内容系统等多个内部服务共用。

## 组成

- Postgres（默认启用）：可靠持久化（inbound_event/job 等）
- Redis（可选，profile=redis）：缓存/限速/短期去重窗口
- MinIO（可选，profile=minio）：对象存储（附件/图片）

## 启动

```bash
cd infra/storage

# 仅 Postgres
docker compose up -d

# Postgres + Redis + MinIO
docker compose --profile redis --profile minio up -d
```

网络与服务名：

- Docker Network：`infra_net`
- Postgres Host：`postgres:5432`
- Redis Host：`redis:6379`
- MinIO Host：`minio:9000`（Console `minio:9001`）

## 环境变量

支持 `.env`（可参考 `.env.example`），未提供时使用默认值。

常用：

- `INFRA_PG_SUPERUSER` / `INFRA_PG_SUPERPASS` / `INFRA_PG_PORT`
- `MGW_PG_DB` / `MGW_PG_USER` / `MGW_PG_PASSWORD`
- `CMS_PG_DB` / `CMS_PG_USER` / `CMS_PG_PASSWORD`

## 初始化

首次启动（空数据卷）时会自动执行 [postgres-init/01-init.sql](./postgres-init/01-init.sql)：

- 创建 `message_gateway`、`llm_gateway`、`agent_center`、`admin_console`、`content_system` 数据库
- 为每个系统创建独立账号并授权（默认账号/密码见 SQL 文件）

## 业务服务接入示例

业务服务的 compose 需要加入同一网络：

```yaml
services:
  message-gateway:
    image: your/message-gateway:latest
    environment:
      DATABASE_URL: postgres://mgw:mgw_pwd@postgres:5432/message_gateway?sslmode=disable
    networks:
      - infra_net

networks:
  infra_net:
    external: true
    name: infra_net
```

## 停止与清理

```bash
docker compose down

# 清理数据（会删除 Postgres/Redis/MinIO 数据卷）
docker compose down -v
```
