# ai

## 文档索引

- 仓库总入口：`README.md`
- `admin-console`：`admin-console/README.md`
- `llm-gateway`：`llm-gateway/README.md`
- `message-gateway`：`message-gateway/README.md`
- `core-service`：`core-service/README.md`
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

1. 启动基础设施（Postgres）

```bash
./deploy/local/storage.sh up
```

2. 为各服务创建本地环境文件

```bash
cp admin-console/.env.local.example admin-console/.env.local
cp llm-gateway/.env.local.example llm-gateway/.env.local
cp core-service/.env.local.example core-service/.env.local
cp message-gateway/.env.local.example message-gateway/.env.local
```

3. 通过 `admin-console` 统一管理 bot / llm / routing / message routes 配置

运行时配置已收敛到本地 PostgreSQL 中的 `admin_console` 数据库，不再依赖仓库内 JSON 配置文件。

4. 启动全部本地服务

```bash
./deploy/local/start.sh all
```

默认端口约定（5008x）：

- llm-gateway: `:50080`
- core-service: `:50081`
- message-gateway: `:50082`
- admin-console: `:50083`

也支持单独启动任意服务，例如：

```bash
./deploy/local/start.sh admin-console
./deploy/local/start.sh llm-gateway
./deploy/local/start.sh core-service message-gateway
```

停止服务：

```bash
./deploy/local/stop.sh all
```

## Docker 部署（本机 Docker）

```bash
cd /Users/zxz/AI/deploy/docker
cp .env.example .env
docker compose up -d --build
```

管理后台启动后可访问：`http://localhost:50083`

`admin-console` 当前把 bot / llm / routing 配置写入本地 PostgreSQL `admin_console` 数据库。

需要在 `deploy/docker/.env` 中填写：

- `DEEPSEEK_API_KEY`
- `LARK_APP_ID` / `LARK_APP_SECRET` / `LARK_VERIFICATION_TOKEN` / `LARK_ENCRYPT_KEY`
