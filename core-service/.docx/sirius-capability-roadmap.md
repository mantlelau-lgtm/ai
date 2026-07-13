# Sirius 能力扩展实施路线文档

## 1. 目标

本文定义 Sirius 量化交易助手后续能力扩展步骤。Sirius 是一个独立项目模块，代码位于：

```text
core-service/sirius
```

上层通过：

```text
SiriusAgent -> ToolCallingAgent -> core-service Agent Runtime
```

接入现有消息链路。

## 2. 当前基线

当前已完成：

```text
sirius/agent.py       SiriusAgent
sirius/catalog.py     第三方工具依赖清单
sirius/guardrails.py  A股/港股交易规则与实盘交易护栏
sirius/tools.py       sirius.* tools
```

已注册 agent：

```text
sirius
```

已注册工具：

```text
sirius.dependencies
sirius.market_rules
sirius.execution_guard
```

当前策略：

```text
实盘交易默认禁用
先做研究、数据、回测、风控、合规能力
不直接连接真实券商下单
```

## 3. 目标架构

```text
用户 / Bot
  ↓
message-gateway
  ↓
core-service
  ↓
SiriusAgent
  ↓
Sirius Tool Registry
  ↓
Sirius Domain Services
  ↓
Adapters / Storage / Scheduler / Risk / Backtest / Execution
```

目标目录：

```text
sirius/
  agent.py
  catalog.py
  guardrails.py
  tools.py
  adapters/
    base.py
    market_data.py
    fundamental.py
    backtest.py
    risk.py
    execution.py
  data/
    models.py
    market_service.py
    fundamental_service.py
  storage/
    repository.py
    schema.sql
    cache.py
  strategy/
    factors.py
    signals.py
    portfolio.py
  backtest/
    engine.py
    metrics.py
    providers.py
  risk/
    rules.py
    metrics.py
    monitor.py
  execution/
    simulator.py
    broker.py
    orders.py
  scheduler/
    calendar.py
    jobs.py
  compliance/
    audit.py
    policies.py
    permissions.py
```

## 4. 阶段 1：工具适配层与领域模型

### 4.1 目标

建立 Sirius 的领域模型和第三方工具统一适配接口。

### 4.2 新增模块

```text
sirius/adapters/base.py
sirius/data/models.py
```

### 4.3 核心模型

建议定义：

```python
Market
Symbol
Quote
Bar
FinancialReport
Announcement
FactorValue
StrategySignal
OrderIntent
RiskDecision
AdapterResult
```

### 4.4 Adapter 统一返回

```python
@dataclass
class AdapterResult:
    ok: bool
    source: str
    timestamp: datetime
    data: Any
    errors: list[str]
    warnings: list[str]
```

### 4.5 验收标准

```text
所有 adapter 接口统一
无真实 key 时可运行 mock adapter
所有返回结果携带 source 和 timestamp
单测覆盖 adapter success / failure / timeout
```

## 5. 阶段 2：市场数据能力

### 5.1 目标

支持 A股/港股行情数据查询、历史数据查询和多源交叉校验。

### 5.2 新增模块

```text
sirius/adapters/market_data.py
sirius/data/market_service.py
```

### 5.3 新增 tools

```text
sirius.quote
sirius.bars
sirius.market_calendar
sirius.data_sources
```

### 5.4 数据源优先级

```text
P0: Choice / Wind
P1: Tushare / 富途
P2: Mock / cached sample
```

### 5.5 数据校验

```text
symbol 格式校验
market 校验：A股 / 港股
交易时间校验
多源时间戳校验
价格字段异常值校验
```

### 5.6 验收标准

```text
支持查询 A股/港股基础 quote
支持查询日线/分钟 bars
无真实 API key 时自动使用 mock adapter
返回数据包含 source/timestamp
异常数据标记 warnings
```

## 6. 阶段 3：基本面与公告能力

### 6.1 目标

支持上市公司财报、公告、监管问询函、股权变动等信息查询。

### 6.2 新增模块

```text
sirius/adapters/fundamental.py
sirius/data/fundamental_service.py
```

### 6.3 新增 tools

```text
sirius.financials
sirius.announcements
sirius.disclosure_search
sirius.company_profile
```

### 6.4 数据源

```text
A股: 巨潮资讯
港股: 港交所披露易
补充: Wind / iFind / 聚源
```

### 6.5 验收标准

```text
公告结果包含原始链接
财报数据包含报告期和披露时间
文本抽取结果可追溯原文
工具返回明确数据来源
```

## 7. 阶段 4：策略与因子能力

### 7.1 目标

构建多因子、统计套利、日内回转等策略的信号生成骨架。

### 7.2 新增模块

```text
sirius/strategy/factors.py
sirius/strategy/signals.py
sirius/strategy/portfolio.py
```

### 7.3 新增 tools

```text
sirius.factor_compute
sirius.signal_generate
sirius.strategy_validate
sirius.portfolio_snapshot
```

### 7.4 内置基础因子

```text
动量因子
反转因子
波动率因子
换手率因子
估值因子
盈利质量因子
资金流因子
AH 溢价因子
```

### 7.5 验收标准

```text
至少 3 类因子可计算
信号输出包含规则解释
所有信号必须经过 market_rules 校验
不得直接生成实盘订单，只生成 OrderIntent
```

## 8. 阶段 5：回测能力

### 8.1 目标

支持本地轻量回测和外部回测平台适配。

### 8.2 新增模块

```text
sirius/backtest/engine.py
sirius/backtest/metrics.py
sirius/backtest/providers.py
```

### 8.3 新增 tools

```text
sirius.backtest_run
sirius.backtest_report
sirius.backtest_compare
```

### 8.4 回测指标

```text
累计收益
年化收益
夏普比率
最大回撤
胜率
换手率
交易成本
滑点影响
VaR / ES
```

### 8.5 验收标准

```text
支持至少一种本地 mock 回测
支持外部 provider 占位适配
回测报告明确展示假设条件
回测结论必须带风险声明
```

## 9. 阶段 6：风险监控能力

### 9.1 目标

对组合、策略和订单意图进行风险评估。

### 9.2 新增模块

```text
sirius/risk/rules.py
sirius/risk/metrics.py
sirius/risk/monitor.py
```

### 9.3 新增 tools

```text
sirius.risk_evaluate
sirius.risk_limits
sirius.risk_report
```

### 9.4 风控规则

```text
单票持仓比例限制
行业集中度限制
最大回撤阈值
VaR 阈值
预期损失阈值
黑名单标的
涨跌停/停牌限制
港股最小报价单位限制
```

### 9.5 验收标准

```text
OrderIntent 必须先经过 risk_evaluate
风险结果包含 allow/block/review
风险事件可审计
风控不通过时不能进入 execution
```

## 10. 阶段 7：仿真交易能力

### 10.1 目标

支持仿真订单撮合、订单状态追踪、持仓更新和绩效报告。

### 10.2 新增模块

```text
sirius/execution/simulator.py
sirius/execution/orders.py
```

### 10.3 新增 tools

```text
sirius.sim_order_submit
sirius.sim_order_cancel
sirius.sim_positions
sirius.sim_performance
```

### 10.4 验收标准

```text
仿真环境与实盘环境隔离
订单生命周期完整
成交后更新持仓
能输出仿真绩效报告
```

## 11. 阶段 8：实盘执行预留能力

### 11.1 目标

只建设接口边界，不默认启用实盘交易。

### 11.2 新增模块

```text
sirius/execution/broker.py
sirius/compliance/permissions.py
sirius/compliance/audit.py
```

### 11.3 新增 tools

```text
sirius.live_order_submit
sirius.live_order_cancel
sirius.live_positions
```

### 11.4 强制前置条件

```text
用户实盘权限
策略审批通过
sirius.execution_guard allowed=true
risk_evaluate allowed=true
交易接口健康
审计日志可写
人工确认或白名单授权
```

### 11.5 验收标准

```text
默认配置下所有 live_* tools 均拒绝执行
拒绝结果必须说明原因
所有尝试写入 audit log
只有完整权限链路后才能打开
```

## 12. 阶段 9：调度与自动化

### 12.1 目标

按 A股、港股交易时间自动执行盘前、盘中、盘后任务。

### 12.2 新增模块

```text
sirius/scheduler/calendar.py
sirius/scheduler/jobs.py
```

### 12.3 任务类型

```text
盘前：公告更新、隔夜数据、策略准备
盘中：行情采集、因子计算、信号生成、风险监控
盘后：数据清洗、回测更新、绩效归因、风险报告
非交易日：策略研究、财报更新、政策同步
```

### 12.4 验收标准

```text
支持 A股/港股交易日历
任务可手动触发和自动触发
任务执行有状态和日志
失败任务支持重试和告警
```

## 13. 阶段 10：审计与合规闭环

### 13.1 目标

所有关键工具调用、策略变更、订单意图、风控决策可追溯。

### 13.2 新增模块

```text
sirius/compliance/audit.py
sirius/compliance/policies.py
```

### 13.3 审计字段

```text
request_id
trace_id
user_id
agent_name
tool_name
action
input_hash
output_hash
source
risk_decision
created_at
```

### 13.4 验收标准

```text
关键工具调用可查询
实盘相关尝试必须审计
审计记录不可静默失败
审计保留策略满足监管要求
```

## 14. 推荐实施顺序

```text
1. adapters/base + data/models
2. market_data mock adapter + sirius.quote
3. fundamental mock adapter + sirius.announcements
4. strategy factor skeleton + sirius.factor_compute
5. local backtest skeleton + sirius.backtest_run
6. risk rules + sirius.risk_evaluate
7. simulator execution + sirius.sim_order_submit
8. audit log + compliance policies
9. scheduler jobs
10. live execution boundary only
```

## 15. 每阶段共同验收要求

每个阶段必须满足：

```text
单元测试通过
无真实密钥时可用 mock 模式运行
所有工具返回 source/timestamp
所有风险/交易相关工具带合规声明
实盘交易默认禁用
文档同步更新
```

## 16. 当前下一步建议

下一步优先实现：

```text
sirius/adapters/base.py
sirius/data/models.py
sirius/adapters/market_data.py
sirius/data/market_service.py
sirius.quote tool
```

原因：

```text
市场数据是策略、回测、风控的前置基础；
先用 mock adapter 跑通工具链路；
后续再替换为 Choice / Wind / Tushare / 富途真实适配器。
```
