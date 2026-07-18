# LLM Gateway 设计方案（独立服务 / HTTP / OpenAI 兼容）

> `LLM Gateway` 是 Tracer 2.0 的模型能力出口，作为**独立服务**对外暴露 HTTP 端口，封装市面上的 LLM（或 OpenAI-compatible endpoints），提供模型路由、usage 统计、密钥/模型管理等能力，并提供统一的 OpenAI 风格调用接口（含流式）。

## 1. 服务定位与边界

### 1.1 服务定位

- 面向上游业务（agent runtime、工具服务、离线任务等）：提供统一的 OpenAI 风格 API
- 面向下游 Provider：通过 `base_url + api_key` 方式调用（优先假设为 OpenAI-compatible API；非兼容 Provider 通过适配层接入）
- 面向运维/平台：提供模型/密钥/路由规则的管理接口与统计查询接口

### 1.2 与上游编排服务的边界

- 上游编排服务负责对话编排、工具调用、记忆与任务状态
- llm-gateway 只负责“模型调用相关”的能力：路由、重试、限流、计费/usage、密钥与模型治理、可观测

## 2. 核心能力

- **OpenAI 兼容接口**：`/v1/chat/completions`、`/v1/embeddings`、`/v1/models`
- **流式输出**：透传 OpenAI SSE 流（`data: ...`），支持 `stream_options.include_usage=true`（若下游支持）
- **模型路由**：别名模型 → 多后端（base_url/key）选择，支持权重/优先级/健康状态/fallback
- **usage 统计**：记录 prompt/completion/total tokens、延迟、错误、成本（按价格表可选计算）
- **密钥/模型管理**：统一保存与管理 API Key、Provider 端点、模型映射与路由规则（支持动态生效）
- **可靠性**：超时、重试（指数退避）、熔断/隔离、失败降级与 fallback
- **可观测**：metrics、结构化日志、trace_id 透传

## 3. 对外 HTTP API（OpenAI 风格）

### 3.1 Chat Completions

- `POST /v1/chat/completions`
- 请求/响应：与 OpenAI 协议对齐（兼容 `stream=true`）
- 关键行为：
  - `model` 支持使用“别名模型”（例如 `gpt-4o-mini`、`kimi-k2`、`glm-4`），网关内部映射到真实后端
  - 将上游传入的 `X-Request-Id`、`X-Trace-Id`（或内部生成）贯穿整个调用链路

### 3.2 Embeddings

- `POST /v1/embeddings`
- 同 OpenAI embeddings 协议

### 3.3 Models

- `GET /v1/models`
- 返回当前对外暴露的“可用模型列表”（别名模型 + 元信息）

### 3.4 约定的扩展 Header

用于治理、审计与统计归因（均为可选）：

| Header | 用途 |
|--------|------|
| `X-Tenant-Id` | 租户/业务归因 |
| `X-User-Id` | 用户归因（与 message-gateway 透传链路对齐） |
| `X-Session-Id` | 会话归因 |
| `X-Trace-Id` | 链路追踪 |
| `X-Request-Id` | 幂等/定位问题 |

## 4. Provider 接入模型（base_url + api_key）

### 4.1 基本假设

优先支持“OpenAI-compatible”下游（即存在 OpenAI 风格的 `/v1/chat/completions` 等接口）。对不兼容的 Provider：

- 方案 A：引入适配器（llm-gateway 内部实现 request/response 转换）
- 方案 B：要求 Provider 侧提供 OpenAI-compatible 代理层（llm-gateway 直接按 OpenAI 协议调用）

### 4.2 Provider 配置项（示例）

- `provider_id`
- `display_name`
- `base_url`
- `auth_type`：`bearer`（默认）
- `api_key_ref`：引用密钥表
- `default_headers`：额外 header（可选）
- `timeout_ms`
- `max_retries`
- `enabled`

## 5. 模型路由设计

### 5.1 模型抽象

- **公开模型（alias model）**：对上游暴露的 `model` 名称
- **后端模型（backend model）**：在某个 provider 上实际调用的 `model` 名称
- **路由目标（backend target）**：`provider_id + backend_model + 权重/优先级/配额`

### 5.2 路由输入

- `model`（alias）
- `request_type`：chat / embedding
- `stream` 与 `stream_options`
- 归因维度：tenant/user/session
- 规则标签：例如 `routing_hint=cheap|fast|stable`（可作为 header 或 body 扩展字段，后续再定）

### 5.3 路由策略（最小可用 → 可演进）

最小可用（MVP）：
- alias → targets 列表
- `weighted random` 选择一个 enabled 且健康的 target
- 失败则按优先级 fallback 到下一个 target

可演进：
- 按租户/用户灰度（canary）
- 按成本/延迟 SLA 选择（基于历史统计）
- 按配额（RPM/TPM）动态避让
- 熔断：连续失败将 target 标记为不健康一段时间

## 6. Usage 统计与成本计算

### 6.1 统计口径

对每次请求记录：
- 请求元信息：request_id、trace_id、tenant_id、user_id、session_id
- 路由结果：alias_model、provider_id、backend_model
- 时延：排队/请求/响应耗时（至少记录总耗时）
- usage：prompt_tokens、completion_tokens、total_tokens
- 结果：success/failed、错误码、HTTP status

### 6.2 usage 来源

- 非流式：优先读取 OpenAI 响应的 `usage` 字段
- 流式：优先启用 `stream_options.include_usage=true`（若 provider 支持）；否则记录为 “unknown/0”，后续可增量补齐 token 估算能力

### 6.3 成本计算（可选）

- `model_pricing` 维护输入/输出单价
- 计算 `cost = prompt_tokens*input_price + completion_tokens*output_price`

## 7. 密钥 / 模型 / 路由管理

### 7.1 管理面 API（建议走独立前缀）

- `GET /admin/providers`
- `POST /admin/providers`
- `GET /admin/models`
- `POST /admin/models`
- `GET /admin/keys`
- `POST /admin/keys`
- `POST /admin/reload`（触发热加载，可选）

### 7.2 管理面鉴权

MVP：
- `X-Admin-Token`（环境变量注入）或 `Authorization: Bearer <token>`

演进：
- OAuth / 内网 mTLS / IAM

### 7.3 密钥存储策略

MVP：
- 密钥加密后存 PostgreSQL（AES-GCM），master key 从环境变量注入
- 仅在内存解密使用，不写入日志

## 8. 存储与数据模型（建议 PostgreSQL）

> 与现有 infra（PostgreSQL）对齐，便于统一查询与运维。Redis 作为可选缓存/限流状态存储。

建议表（字段为方向性，落地时再细化）：

- `llm_provider`
- `llm_api_key`
- `llm_model_alias`
- `llm_model_target`
- `llm_route_rule`（可选）
- `llm_usage_event`（核心：每次调用的 usage 与归因）

## 9. 缓存与限流（可选能力）

- 缓存：仅对非流式、低温度、强确定性请求；键包含 messages hash / model / params
- 限流：按 tenant/user/provider/model 维度（RPM/TPM）

## 10. 错误与重试

### 10.1 错误码（网关侧）

| 错误码 | 说明 |
|--------|------|
| `LLMGW_001` | alias model 未找到 |
| `LLMGW_002` | 无可用路由目标 |
| `LLMGW_003` | 下游超时 |
| `LLMGW_004` | 下游限流（429） |
| `LLMGW_005` | 下游 5xx |
| `LLMGW_006` | 管理面鉴权失败 |

### 10.2 重试策略

- 可重试：连接错误、超时、429、5xx
- 指数退避 + jitter
- 流式请求谨慎重试：若已开始向上游输出，通常不再重试，改为直接终止并返回错误事件

## 11. 可观测性

### 11.1 Metrics（Prometheus 风格）

```text
llmgw_requests_total{provider, model, endpoint, status}
llmgw_request_duration_seconds{provider, model, endpoint}
llmgw_tokens_total{model, type}
llmgw_cost_total{model}
llmgw_route_fallback_total{model}
llmgw_provider_up{provider}
```

### 11.2 日志

- 结构化日志，默认不打印请求/响应全文
- 必须脱敏：api_key、Authorization 等

## 12. 代码实现建议（Go + net/http）

推荐模块拆分（示意）：

```text
llm-gateway/
├── cmd/llm-gateway/
├── internal/
│   ├── api/            # HTTP handlers(OpenAI/admin)
│   ├── router/         # 路由与健康状态
│   ├── provider/       # OpenAI-compatible client
│   ├── usage/          # usage 归集与成本计算
│   ├── store/          # PostgreSQL + migrations
│   ├── security/       # key 加解密 + 管理面鉴权
│   └── metrics/
└── README.md
```
