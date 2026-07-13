# LLM Gateway 实现计划（Go + net/http）

本文件用于把 [LLM Gateway 设计方案](file:///Users/zxz/AI/docs/03-llm-gateway/design.md) 落到可执行的实现拆解，便于后续按里程碑推进。

## 1. 里程碑

### M0：脚手架与最小可运行

- 初始化 `llm-gateway` 独立服务目录与模块
- 提供健康检查与基础 metrics
- 通过环境变量配置一个 OpenAI-compatible 后端（base_url + api_key）
- 跑通 `POST /v1/chat/completions`（非流式）

### M1：流式透传 + 基础路由

- `stream=true` 时透传 SSE（`data: ...`）
- alias model → targets 路由（weighted random + fallback）
- 记录基础调用日志与统计（requests_total / latency）

### M2：usage 归集 + 成本计算

- 非流式读取 `usage`
- 流式尽量启用 `stream_options.include_usage=true`（可配置开关）
- `llm_usage_event` 落库（PostgreSQL），并提供最小查询接口

### M3：密钥/模型/Provider 管理面

- 管理面鉴权（`X-Admin-Token`）
- CRUD：provider / key / alias model / target
- 密钥加密存储（AES-GCM，master key 从 env 注入）
- 配置热加载（watch + 周期 reload / 手动 reload）

### M4：治理能力（可选增强）

- RPM/TPM 限流（按 tenant/user/provider/model）
- 熔断与健康探测（失败统计 + cooldown）
- 缓存（仅非流式、低温度）

## 2. 目录结构（建议）

```text
llm-gateway/
├── cmd/llm-gateway/
│   └── main.go
├── internal/
│   ├── api/
│   │   ├── openai/         # /v1/* handlers
│   │   └── admin/          # /admin/* handlers
│   ├── config/             # env + file config
│   ├── router/             # alias->targets, picker, health/fallback
│   ├── provider/           # OpenAI-compatible HTTP client + stream proxy
│   ├── usage/              # usage parse + cost calc + persistence adapter
│   ├── store/              # postgres + migrations
│   ├── security/           # admin auth + key encryption/decryption
│   └── metrics/            # prometheus style metrics
└── README.md
```

## 3. 关键接口定义（方向性）

### 3.1 路由

- `ResolveAlias(model string) (AliasModel, bool)`
- `PickTarget(alias AliasModel, req RoutingRequest) (Target, error)`
- `MarkFailure(targetID string, err error)`
- `MarkSuccess(targetID string, usage Usage)`

### 3.2 Provider Client（OpenAI-compatible）

- `ChatCompletions(ctx, target, req) (resp, raw, err)`
- `StreamChatCompletions(ctx, target, req, onEvent func([]byte) error) error`
- `Embeddings(ctx, target, req) (resp, raw, err)`

### 3.3 Usage

- `ParseUsageFromResponse(respJSON []byte) (Usage, bool)`
- `PersistUsage(ctx, UsageEvent) error`

## 4. 数据库表（初版建议）

最小闭环建议先落：

- `llm_provider(provider_id, base_url, enabled, timeout_ms, created_at, updated_at)`
- `llm_api_key(key_id, provider_id, cipher_text, created_at, updated_at)`
- `llm_model_alias(alias, request_type, enabled, created_at, updated_at)`
- `llm_model_target(target_id, alias, provider_id, backend_model, weight, priority, enabled, created_at, updated_at)`
- `llm_usage_event(event_id, request_id, trace_id, tenant_id, user_id, session_id, alias, provider_id, backend_model, prompt_tokens, completion_tokens, total_tokens, cost, latency_ms, status, error_code, created_at)`

## 5. 测试策略

- 单元测试：
  - 路由选择（weighted/fallback）
  - SSE 透传（chunk 边界、flush、断连）
  - usage 解析（非流式与 include_usage 流式）
- 集成测试：
  - 启动一个 mock OpenAI server（支持 stream）
  - llm-gateway 调用后验证：
    - upstream 收到完整 SSE
    - usage_event 落库正确

