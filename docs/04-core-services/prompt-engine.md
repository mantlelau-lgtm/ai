# Prompt Engine 详细设计

> `Prompt Engine` 负责 Prompt 模板管理、上下文组装和渲染，是编排层与模型层之间的模板化桥梁。

## 1. 服务概述

| 属性 | 值 |
|------|-----|
| 服务名称 | `prompt-engine` |
| gRPC 端口 | `50053` |
| Proto 定义 | `prompt_engine.proto` |
| 依赖存储 | PostgreSQL |

### 核心职责

- Prompt 渲染：基于模板与上下文生成完整模型输入
- 模板管理：支持模板 CRUD 与版本演进
- 上下文组装：将系统指令、历史消息、用户输入组合成标准消息结构

## 2. gRPC 接口

| 接口 | 用途 |
|------|------|
| `RenderPrompt` | 渲染模板并生成 `messages` / `rendered_prompt` |
| `GetTemplate` | 获取模板详情 |
| `UpdateTemplate` | 更新模板 |

## 3. 模板引擎设计

### 模板语法

```gotemplate
{{define "system"}}
你是一个专业的 AI 助手。
当前时间：{{.CurrentTime}}
用户信息：{{.UserInfo.Name}} ({{.UserInfo.Role}})
{{end}}

{{define "user"}}
{{range .History}}
{{.Role}}: {{.Content}}
{{end}}
用户：{{.UserMessage}}
{{end}}
```

### 内置变量

| 变量 | 说明 |
|------|------|
| `.CurrentTime` | 当前时间 |
| `.UserInfo` | 用户信息 |
| `.History` | 对话历史 |
| `.UserMessage` | 用户当前输入 |
| `.AgentConfig` | Agent 配置 |

## 4. 数据结构

| 表 | 用途 |
|------|------|
| `prompt_templates` | 主模板表 |
| `prompt_template_versions` | 模板版本记录 |

## 5. 内部模块

```text
prompt-engine/
├── handler/
├── template/   # 加载、缓存、版本管理
├── renderer/   # Go template 渲染与变量注入
└── validator/  # 语法和安全校验
```

## 6. 缓存策略

- 模板缓存：进程内缓存，TTL 5 分钟，LRU 淘汰
- Prompt 缓存：Redis 中按 `PROMPT:{template_id}:{context_hash}` 保存，TTL 24 小时

## 7. 安全设计

- 禁止模板注入危险指令或危险函数名
- 对敏感词和敏感字段做过滤或脱敏
- 将模板管理权限与普通调用权限分开

## 8. 错误码

| 错误码 | 说明 |
|--------|------|
| `PROMPT_001` | 模板未找到 |
| `PROMPT_002` | 渲染失败 |
| `PROMPT_003` | 变量缺失 |
| `PROMPT_004` | 模板注入风险 |
| `PROMPT_005` | 危险函数或内容 |

## 9. 监控指标

```text
prompt_render_total{template_id, status}
prompt_render_duration_seconds{template_id, quantile}
prompt_cache_hit_ratio{template_id}
prompt_cache_size
prompt_template_count{agent_id}
prompt_template_version{template_id}
```
