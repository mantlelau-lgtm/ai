# admin-console

独立的 Web 管理后台，用于统一配置 bot、llm、routing，并把配置写入本地 PostgreSQL。

同时它也作为运行时配置中心，对外提供服务可直接消费的查询接口：

- `GET /api/runtime/llm-gateway/catalog`
- `GET /api/runtime/message-gateway/bots`
- `GET /api/runtime/message-gateway/routes`
- `GET /api/runtime/core-service/routing`

## 本地开发

1. 启动后端 API：

```bash
cd /Users/zxz/AI/admin-console
python3 -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
npm install
npm run dev:api
```

2. 另开一个终端启动前端：

```bash
cd /Users/zxz/AI/admin-console
npm run dev
```

默认地址：

- 前端：`http://localhost:3001`
- 后端：`http://localhost:50083`

## 环境变量

- `DATABASE_URL`：后台配置数据库连接串
- `ADMIN_CONSOLE_ENCRYPTION_SECRET`：用于统一加密/解密 LLM 密钥值
- `MESSAGE_GATEWAY_HEALTH_URL`：message-gateway 健康检查地址
- `CORE_SERVICE_HEALTH_URL`：core-service 健康检查地址
- `LLM_GATEWAY_HEALTH_URL`：llm-gateway 健康检查地址
- `ADMIN_CONSOLE_DISABLE_DB_STARTUP=1`：测试场景下跳过真实数据库初始化

## 测试

前端：

```bash
cd /Users/zxz/AI/admin-console
npm run test
npm run check
```

后端：

```bash
cd /Users/zxz/AI/admin-console
python3 -m unittest discover -s tests
```

## Docker

```bash
cd /Users/zxz/AI/deploy/docker
docker compose up -d --build admin-console
```

后台默认暴露在 `http://localhost:50083`。

如果你的本地 `infra-postgres` 使用的是旧 volume，需要先确保 `admin_console` 数据库和 `admin_console` 用户已经存在；否则请重建存储 volume 或手动建库建用户。

## PostgreSQL 表设计

管理后台会自动初始化以下表：

- `admin_bots`
- `admin_llm_credentials`
- `admin_llm_models`
- `admin_routing_settings`
- `admin_routes`
- `admin_agent_specs`
- `admin_message_route_rules`

其中 LLM 密钥管理已收敛为单表结构，统一维护：

- 厂商名称
- `base_url`
- 调用类型
- 密钥名称
- 密钥值
- 其他附属信息

密钥值写库前会加密，读取运行时配置时再解密输出给下游服务。

当前保存策略是单事务覆盖式写入，确保 bot / llm / routing 三组配置始终保持一致快照。
