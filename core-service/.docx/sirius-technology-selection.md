# Sirius 量化交易助手技术选型文档

## 1. 文档目标

本文定义 Sirius 量化交易助手的技术选型，用于指导后续在 `core-service/sirius` 目录下扩展行情数据、基本面数据、策略回测、仿真交易、风险监控与合规控制等能力。

Sirius 的设计原则：

```text
研究分析优先
合规安全优先
实盘交易默认禁用
工具适配层可插拔
数据源多路校验
回测与实盘严格隔离
所有操作可审计
```

## 2. 总体技术栈

| 层级 | 推荐技术 | 说明 |
|---|---|---|
| Agent Runtime | core-service ToolCallingAgent | 复用现有 LLM + tools 调用链路 |
| Sirius Agent | `sirius.agent.SiriusAgent` | 独立 agent，专属 prompt 与工具集 |
| Tool Registry | `core_service.agent_tools.ToolRegistry` | 注册 sirius.* tools |
| 数据模型 | Python dataclass / Pydantic | 内部领域模型、接口 DTO |
| HTTP 客户端 | httpx / aiohttp | 调用第三方数据源 API |
| 定时调度 | APScheduler | 盘前、盘中、盘后任务调度 |
| 结构化存储 | PostgreSQL / TimescaleDB | 财报、行情、持仓、策略、风控事件 |
| 高频缓存 | Redis | 高频行情、短周期指标、任务锁 |
| 非结构化存储 | PostgreSQL JSONB / 对象存储 | 公告、财报文本、研报片段 |
| 回测计算 | pandas / numpy / vectorbt / 自研适配层 | 本地轻量回测与外部回测平台桥接 |
| 指标计算 | pandas / numpy / scipy / statsmodels | 因子、收益、风险指标 |
| 风控计算 | numpy / scipy / pyfolio 风格指标 | VaR、ES、最大回撤、仓位限制 |
| 审计日志 | PostgreSQL + 现有 hourly logs | 交易、风控、工具调用、策略变更留痕 |

## 3. 模块目录选型

建议在 `core-service/sirius` 下形成如下结构：

```text
sirius/
  agent.py              # SiriusAgent
  catalog.py            # 第三方工具依赖清单
  guardrails.py         # 合规与交易护栏
  tools.py              # sirius.* tools 注册入口
  adapters/             # 第三方 API 适配器
  data/                 # 行情/基本面/公告数据模型与服务
  storage/              # 数据库与缓存访问
  strategy/             # 因子、策略、信号生成
  backtest/             # 回测抽象与外部回测平台适配
  risk/                 # 风控规则与风险指标
  execution/            # 仿真/实盘执行接口，实盘默认禁用
  scheduler/            # 交易日历和任务调度
  compliance/           # 合规规则、审计和权限
```

## 4. 第三方工具选型

### 4.1 市场行情与基础数据

| 工具 | 定位 | 使用优先级 | 说明 |
|---|---|---:|---|
| 东方财富 Choice API | A股/港股行情主源 | P0 | 商业授权后作为主行情源之一 |
| Wind API | 基础数据和资金流补充 | P0 | 行业、基本面、融资融券、北向资金 |
| Tushare Pro | 历史数据与备用源 | P1 | 适合开发、回测、补充数据 |
| 富途开放 API | 港股盘口和暗盘 | P1 | 港股实时场景增强 |

技术策略：

```text
统一封装 MarketDataAdapter
所有数据带 source、timestamp、market、symbol
关键行情字段做多源交叉校验
实时行情不只依赖单一免费数据源
```

### 4.2 基本面与另类数据

| 工具 | 定位 | 使用优先级 |
|---|---|---:|
| 巨潮资讯官方 API | A股公告/财报主源 | P0 |
| 港交所披露易 API | 港股公告/财报主源 | P0 |
| 同花顺 iFind | 另类数据 | P1 |
| 聚源数据 | 财报文本解析 | P1 |

技术策略：

```text
公告和财报必须保留原始文件链接
结构化指标与原文片段建立映射
非结构化抽取结果必须标注来源与更新时间
```

### 4.3 回测与仿真

| 工具 | 定位 | 使用优先级 |
|---|---|---:|
| JoinQuant | A股/港股回测主平台 | P0 |
| RiceQuant | 回测补充与绩效指标 | P1 |
| 迅投仿真交易 | 实盘前仿真 | P1 |

技术策略：

```text
本地只保留统一 BacktestAdapter 接口
外部平台作为执行后端
回测结果必须记录假设条件、费用、滑点、交易规则版本
```

### 4.4 交易执行与风控

| 工具 | 定位 | 默认状态 |
|---|---|---|
| 中信证券 CTP | A股实盘交易 | 禁用 |
| 辉立证券港股 API | 港股实盘交易 | 禁用 |
| 恒生 UMP | 前置风控 | 启用前置设计 |

技术策略：

```text
实盘交易默认禁用
任何 order_submit 必须经过 sirius.execution_guard
实盘交易需要强权限、人工确认、风控通过和审计记录
仿真交易与实盘交易接口必须物理隔离
```

### 4.5 合规与风险监控

| 工具 | 定位 |
|---|---|
| 证监会监管政策数据库 | 政策变动监控 |
| 交易所交易规则查询 API | 交易规则版本管理 |
| 万得风险计量系统 | VaR/ES/组合风险 |

技术策略：

```text
规则版本随策略执行留痕
风险指标与行情快照、持仓快照绑定
触发风控阈值时生成 risk_event
```

## 5. 数据存储选型

### 5.1 PostgreSQL / TimescaleDB

适合存储：

```text
日线行情
分钟行情
财务指标
策略定义
回测任务
回测结果
组合持仓
订单记录
风险事件
审计记录
```

如果后续高频分钟/逐笔数据量增长，建议启用 TimescaleDB hypertable。

### 5.2 Redis

适合存储：

```text
最新行情快照
任务锁
盘中指标缓存
风控状态缓存
接口限流计数
```

### 5.3 JSONB / 对象存储

适合存储：

```text
公告原文
财报原文
研报片段
舆情文本
第三方接口原始响应快照
```

## 6. 核心接口抽象选型

建议定义以下协议接口：

```python
class MarketDataAdapter:
    async def quote(symbols, market): ...
    async def bars(symbol, market, timeframe, start, end): ...

class FundamentalDataAdapter:
    async def financials(symbol, market, period): ...
    async def announcements(symbol, market, start, end): ...

class BacktestAdapter:
    async def run(strategy, universe, start, end, params): ...

class RiskAdapter:
    async def evaluate(portfolio, market_snapshot): ...

class ExecutionAdapter:
    async def submit_order(order): ...
    async def cancel_order(order_id): ...
```

所有 adapter 必须返回统一结果：

```text
ok
source
timestamp
data
errors
warnings
```

## 7. 合规安全选型

### 7.1 默认禁止实盘交易

当前 `sirius.execution_guard` 默认：

```json
{"allowed": false}
```

### 7.2 权限要求

未来实盘交易至少需要：

```text
用户身份认证
实盘交易权限
策略审批状态
风控通过状态
人工确认或授权策略白名单
完整审计记录
```

### 7.3 审计要求

所有关键动作需要记录：

```text
request_id
trace_id
user_id
agent_name
tool_name
input
output
source
risk_decision
timestamp
```

## 8. 当前阶段结论

当前 Sirius 应优先完成：

```text
依赖目录
市场规则
合规护栏
工具抽象
只读数据工具
回测任务骨架
风险指标骨架
```

暂不实现真实实盘交易。实盘交易必须等权限系统、风控前置、审计闭环、券商联调完成后再打开。
