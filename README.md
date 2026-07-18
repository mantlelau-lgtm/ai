# ai

## 文档索引

- 仓库总入口：`README.md`
- `agent-center`：`agent-center/README.md`
- `admin-console`：`admin-console/README.md`
- `llm-gateway`：`llm-gateway/README.md`
- `message-gateway`：`message-gateway/README.md`
- `infra/storage`：`infra/storage/README.md`
- 本地部署：`deploy/local/README.md`
- Docker 部署：`deploy/docker/README.md`
- 方案文档：`.trae/documents/admin-console-prd.md`
- 技术架构：`.trae/documents/admin-console-tech-arch.md`

## 文档与目录约定

- `.tmp/`：本地临时产物，已加入 Git ignore
- `deploy/`：本地部署与运行目录，已加入 Git ignore，不做版本管理
- `docs/`：预留本地文档目录，已加入 Git ignore，不做版本管理
- 正式说明统一收敛到各模块 `README.md` 与 `.trae/documents/`

## 本地启动（推荐）

1. 确保本机已启动所需中间件

当前项目实际运行依赖的是本机 PostgreSQL；仓库里没有启用中的 MySQL / Redis 运行时依赖。

2. 为各服务创建本地环境文件

```bash
cp agent-center/.env.local.example agent-center/.env.local
cp admin-console/.env.local.example admin-console/.env.local
cp llm-gateway/.env.local.example llm-gateway/.env.local
cp message-gateway/.env.local.example message-gateway/.env.local
```

3. 通过 `admin-console` 统一管理 bot / llm / routing / message routes 配置

运行时配置已收敛到本地 PostgreSQL 中的 `admin_console` 数据库，不再依赖仓库内 JSON 配置文件。

4. 启动全部本地服务

```bash
./deploy/local/start.sh all
```

默认端口约定：

- agent-center: `:9999`
- llm-gateway: `:50080`
- message-gateway: `:50082`
- admin-console: `:50083`
- robot-d: `:7004`（Docker 映射默认 `:50084`）

也支持单独启动任意服务，例如：

```bash
./deploy/local/start.sh agent-center
./deploy/local/start.sh admin-console
./deploy/local/start.sh llm-gateway
./deploy/local/start.sh message-gateway
```

停止服务：

```bash
./deploy/local/stop.sh all
```

## Docker 部署（本机 Docker）

```bash
cd /Users/rocky/CodingSpace/ai-coding/ai/deploy/docker
cp .env.example .env
docker compose up -d --build
```

管理后台启动后可访问：`http://localhost:50083`

容器部署会启动这些业务服务：

- `agent-center`
- `llm-gateway`
- `message-gateway`
- `admin-console`
- `robot-d`
- `robot-d`

这些容器会直接连接宿主机已启动的 PostgreSQL。

需要在 `deploy/docker/.env` 中填写：

- `ADMIN_TOKEN`
- `ADMIN_CONSOLE_ENCRYPTION_SECRET`
- `AGENT_CENTER_DATABASE_URL`
- `LLM_GATEWAY_DATABASE_URL`
- `MESSAGE_GATEWAY_DATABASE_URL`
- `ADMIN_CONSOLE_DATABASE_URL`
- `LARK_APP_ID` / `LARK_APP_SECRET` / `LARK_VERIFICATION_TOKEN` / `LARK_ENCRYPT_KEY`
