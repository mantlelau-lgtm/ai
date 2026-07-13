# Tool SDK 详细设计

> `Tool SDK` 是 Core Service 的工具执行层，负责工具注册、参数校验、执行隔离与结果回传。

## 1. SDK 概述

| 属性 | 值 |
|------|-----|
| 名称 | `tool-sdk` |
| 语言 | Go 1.21+ |
| 引入方式 | `import "github.com/tracer-ai/tool-sdk"` |
| 执行模式 | 本地执行为主，预留远程 RPC |

### 核心组件

- `Tool Registry`：管理工具元数据和注册关系
- `Tool Executor`：完成参数校验、超时控制和执行调度
- `Built-in Tools`：内置搜索、代码执行、文件处理、HTTP 调用、数据库查询
- `Sandbox`：控制代码执行类工具的资源限制和网络隔离

## 2. 核心接口

```go
type Tool interface {
  Name() string
  Description() string
  Schema() *jsonschema.Schema
  Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error)
}
```

### ToolResult

| 字段 | 说明 |
|------|------|
| `Content` | 主要结果内容 |
| `AdditionalMsgs` | 可选附加消息 |
| `Metadata` | 执行元数据 |
| `Error` | 工具层错误 |

## 3. 内置工具范围

| 工具 | 作用 |
|------|------|
| `SearchTool` | 互联网或内部搜索 |
| `CodeExecutor` | Python / SQL / Shell 代码执行 |
| `FileProcessor` | PDF / Word / Excel / TXT 处理 |
| `HTTPClient` | 外部 API 请求 |
| `DBQuery` | 数据库只读查询与受控写入 |

## 4. 执行流程

1. 根据工具名从 `Tool Registry` 获取工具实现
2. 使用 JSON Schema 校验参数
3. 设置超时上下文
4. 对代码执行类工具应用沙箱策略
5. 执行并返回 `ToolResult`

## 5. 沙箱设计

- 代码类工具运行在 Docker 容器或等价隔离环境中
- 默认资源限制：内存 `128MB`、CPU `0.5` 核、超时 `10s`
- 默认关闭网络，仅在显式放开时允许访问外部网络

## 6. 错误处理

| 错误码 | 说明 |
|--------|------|
| `TOOL_001` | 工具未找到 |
| `TOOL_002` | 参数无效 |
| `TOOL_003` | 执行超时 |
| `TOOL_004` | 沙箱错误 |
| `TOOL_005` | 网络错误 |

## 7. 监控指标

```text
tool_executions_total{tool_name, status}
tool_execution_duration_seconds{tool_name, quantile}
sandbox_running_containers
sandbox_memory_usage_bytes
tool_registry_size
```

## 8. 设计建议

- 保持 Tool SDK 与上层编排解耦，只暴露稳定的工具协议
- 对写操作类工具建立显式确认机制
- 对内置工具和业务工具统一使用同一套注册与校验链路
