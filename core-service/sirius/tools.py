from __future__ import annotations

import json
from typing import Any

from core_service.agent_tools import AgentTool, ToolCall, ToolParameter, ToolRegistry, ToolResult, ToolSpec
from sirius.backtest.engine import run_simple_backtest
from sirius.catalog import dependency_catalog
from sirius.compliance.audit import recent_events, record_event
from sirius.data.fundamental_service import FundamentalDataService
from sirius.data.market_service import MarketDataService
from sirius.data.models import AdapterResult, Bar, dataclass_to_dict
from sirius.execution.simulator import submit_simulated_order
from sirius.guardrails import compliance_disclaimer, live_trading_allowed, market_rules
from sirius.risk.rules import evaluate_order_intent
from sirius.strategy.signals import generate_signal, signal_to_order_intent


def _result(name: str, data: dict[str, Any]) -> ToolResult:
    return ToolResult(name=name, ok=True, content=json.dumps(data, ensure_ascii=False), data=data)


def _adapter_payload(result: AdapterResult) -> dict[str, Any]:
    return {
        "ok": result.ok,
        "source": result.source,
        "timestamp": result.timestamp.isoformat(),
        "data": dataclass_to_dict(result.data),
        "errors": result.errors,
        "warnings": result.warnings,
        "disclaimer": compliance_disclaimer(),
    }


async def _dependency_catalog_tool(call: ToolCall, ctx: Any) -> ToolResult:
    category = str(call.arguments.get("category", "") or "").strip()
    items = dependency_catalog()
    if category:
        items = [item for item in items if item["category"] == category]
    return _result(call.name, {"dependencies": items, "disclaimer": compliance_disclaimer()})


async def _market_rules_tool(call: ToolCall, ctx: Any) -> ToolResult:
    market = str(call.arguments.get("market", "") or "").strip()
    rules = market_rules()
    if market:
        rules = [item for item in rules if item["market"] == market]
    return _result(call.name, {"rules": rules, "disclaimer": compliance_disclaimer()})


async def _execution_guard_tool(call: ToolCall, ctx: Any) -> ToolResult:
    market = str(call.arguments.get("market", "") or "").strip()
    action = str(call.arguments.get("action", "") or "").strip()
    allowed = live_trading_allowed(market)
    data = {
        "market": market,
        "action": action,
        "allowed": allowed,
        "reason": "live trading is disabled by default" if not allowed else "live trading enabled by policy",
        "required_controls": ["用户显式授权", "券商/交易所合规接口", "交易前风控校验", "订单与风控审计留痕", "人工可追溯回滚机制"],
        "disclaimer": compliance_disclaimer(),
    }
    await record_event("execution_guard", data)
    return _result(call.name, data)


async def _quote_tool(call: ToolCall, ctx: Any) -> ToolResult:
    service = MarketDataService()
    result = await service.quote(str(call.arguments.get("code", "000001")), str(call.arguments.get("market", "A股")))
    data = _adapter_payload(result)
    await record_event("quote", data)
    return _result(call.name, data)


async def _bars_tool(call: ToolCall, ctx: Any) -> ToolResult:
    service = MarketDataService()
    result = await service.bars(
        str(call.arguments.get("code", "000001")),
        str(call.arguments.get("market", "A股")),
        int(call.arguments.get("periods", 30) or 30),
    )
    data = _adapter_payload(result)
    await record_event("bars", {"source": data["source"], "count": len(data.get("data") or [])})
    return _result(call.name, data)


async def _signal_tool(call: ToolCall, ctx: Any) -> ToolResult:
    bars_result = await MarketDataService().bars(
        str(call.arguments.get("code", "000001")),
        str(call.arguments.get("market", "A股")),
        int(call.arguments.get("periods", 30) or 30),
    )
    if not bars_result.ok:
        return ToolResult(name=call.name, ok=False, content="", error=";".join(bars_result.errors), data=_adapter_payload(bars_result))
    signal = generate_signal(bars_result.data)
    data = {"signal": dataclass_to_dict(signal), "disclaimer": compliance_disclaimer()}
    await record_event("signal_generate", data)
    return _result(call.name, data)


async def _backtest_tool(call: ToolCall, ctx: Any) -> ToolResult:
    bars_result = await MarketDataService().bars(
        str(call.arguments.get("code", "000001")),
        str(call.arguments.get("market", "A股")),
        int(call.arguments.get("periods", 60) or 60),
    )
    if not bars_result.ok:
        return ToolResult(name=call.name, ok=False, content="", error=";".join(bars_result.errors), data=_adapter_payload(bars_result))
    report = run_simple_backtest(bars_result.data)
    data = {"report": report, "source": bars_result.source, "disclaimer": compliance_disclaimer()}
    await record_event("backtest_run", data)
    return _result(call.name, data)


async def _risk_tool(call: ToolCall, ctx: Any) -> ToolResult:
    bars_result = await MarketDataService().bars(str(call.arguments.get("code", "000001")), str(call.arguments.get("market", "A股")), 30)
    if not bars_result.ok:
        return ToolResult(name=call.name, ok=False, content="", error=";".join(bars_result.errors), data=_adapter_payload(bars_result))
    bars: list[Bar] = bars_result.data
    signal = generate_signal(bars)
    intent = signal_to_order_intent(signal, bars[-1].close, int(call.arguments.get("quantity", 100) or 100))
    decision = evaluate_order_intent(intent)
    data = {"signal": dataclass_to_dict(signal), "order_intent": dataclass_to_dict(intent), "risk_decision": dataclass_to_dict(decision), "disclaimer": compliance_disclaimer()}
    await record_event("risk_evaluate", data)
    return _result(call.name, data)


async def _sim_order_tool(call: ToolCall, ctx: Any) -> ToolResult:
    bars_result = await MarketDataService().bars(str(call.arguments.get("code", "000001")), str(call.arguments.get("market", "A股")), 30)
    if not bars_result.ok:
        return ToolResult(name=call.name, ok=False, content="", error=";".join(bars_result.errors), data=_adapter_payload(bars_result))
    bars: list[Bar] = bars_result.data
    signal = generate_signal(bars)
    intent = signal_to_order_intent(signal, bars[-1].close, int(call.arguments.get("quantity", 100) or 100))
    decision = evaluate_order_intent(intent)
    order = submit_simulated_order(intent) if decision.status in {"allow", "review"} else None
    data = {"signal": dataclass_to_dict(signal), "risk_decision": dataclass_to_dict(decision), "simulated_order": dataclass_to_dict(order), "disclaimer": compliance_disclaimer()}
    await record_event("sim_order_submit", data)
    return _result(call.name, data)


async def _fundamental_tool(call: ToolCall, ctx: Any) -> ToolResult:
    result = await FundamentalDataService().financials(
        str(call.arguments.get("code", "000001")),
        str(call.arguments.get("market", "A股")),
        str(call.arguments.get("period", "latest")),
    )
    data = _adapter_payload(result)
    await record_event("fundamental", {"source": data["source"], "code": call.arguments.get("code", "")})
    return _result(call.name, data)


async def _announcements_tool(call: ToolCall, ctx: Any) -> ToolResult:
    result = await FundamentalDataService().announcements(
        str(call.arguments.get("code", "000001")),
        str(call.arguments.get("market", "A股")),
        str(call.arguments.get("keyword", "")),
        int(call.arguments.get("limit", 10) or 10),
    )
    data = _adapter_payload(result)
    await record_event("announcements", {"source": data["source"], "count": len(data.get("data") or [])})
    return _result(call.name, data)


async def _audit_events_tool(call: ToolCall, ctx: Any) -> ToolResult:
    limit = int(call.arguments.get("limit", 20) or 20)
    return _result(call.name, {"events": await recent_events(limit)})


def sirius_tool_registry(base: ToolRegistry | None = None) -> ToolRegistry:
    registry = base or ToolRegistry()
    sirius_tools = [
        AgentTool(ToolSpec(name="sirius.dependencies", description="查看 Sirius 量化交易助手依赖的第三方工具清单，可按 category 过滤。", parameters=[ToolParameter("category", "string", "工具分类，例如 market_data、backtest、execution")]), _dependency_catalog_tool),
        AgentTool(ToolSpec(name="sirius.market_rules", description="查看 A股/港股交易时段、结算制度和实盘交易开关。", parameters=[ToolParameter("market", "string", "市场名称：A股 或 港股")]), _market_rules_tool),
        AgentTool(ToolSpec(name="sirius.execution_guard", description="检查 Sirius 是否允许某市场的实盘交易动作；默认用于阻断未授权实盘交易。", parameters=[ToolParameter("market", "string", "市场名称：A股 或 港股", True), ToolParameter("action", "string", "交易动作，例如 order_submit、cancel_order", True)]), _execution_guard_tool),
        AgentTool(ToolSpec(name="sirius.quote", description="查询 A股/港股 mock 实时行情快照。", parameters=[ToolParameter("code", "string", "证券代码", True), ToolParameter("market", "string", "A股 或 港股", True)]), _quote_tool),
        AgentTool(ToolSpec(name="sirius.bars", description="查询 A股/港股 mock 历史 K 线。", parameters=[ToolParameter("code", "string", "证券代码", True), ToolParameter("market", "string", "A股 或 港股", True), ToolParameter("periods", "integer", "周期数量，2-250")]), _bars_tool),
        AgentTool(ToolSpec(name="sirius.signal_generate", description="基于 mock 行情生成基础动量/波动率策略信号。", parameters=[ToolParameter("code", "string", "证券代码", True), ToolParameter("market", "string", "A股 或 港股", True), ToolParameter("periods", "integer", "周期数量")]), _signal_tool),
        AgentTool(ToolSpec(name="sirius.backtest_run", description="运行本地 mock 回测并返回绩效指标。", parameters=[ToolParameter("code", "string", "证券代码", True), ToolParameter("market", "string", "A股 或 港股", True), ToolParameter("periods", "integer", "周期数量")]), _backtest_tool),
        AgentTool(ToolSpec(name="sirius.risk_evaluate", description="对策略信号生成的订单意图执行风险评估。", parameters=[ToolParameter("code", "string", "证券代码", True), ToolParameter("market", "string", "A股 或 港股", True), ToolParameter("quantity", "integer", "数量")]), _risk_tool),
        AgentTool(ToolSpec(name="sirius.sim_order_submit", description="在仿真环境中提交订单，实盘交易不启用。", parameters=[ToolParameter("code", "string", "证券代码", True), ToolParameter("market", "string", "A股 或 港股", True), ToolParameter("quantity", "integer", "数量")]), _sim_order_tool),
        AgentTool(ToolSpec(name="sirius.fundamental", description="查询 mock 基本面财务指标，后续可替换为巨潮/披露易/Wind 等适配器。", parameters=[ToolParameter("code", "string", "证券代码", True), ToolParameter("market", "string", "A股 或 港股", True), ToolParameter("period", "string", "报告期，默认 latest")]), _fundamental_tool),
        AgentTool(ToolSpec(name="sirius.announcements", description="查询 mock 公告列表，后续可替换为巨潮/披露易真实数据。", parameters=[ToolParameter("code", "string", "证券代码", True), ToolParameter("market", "string", "A股 或 港股", True), ToolParameter("keyword", "string", "关键词"), ToolParameter("limit", "integer", "最多条数")]), _announcements_tool),
        AgentTool(ToolSpec(name="sirius.audit_events", description="查看 Sirius 最近审计事件。", parameters=[ToolParameter("limit", "integer", "最多返回条数")]), _audit_events_tool),
    ]
    for tool in sirius_tools:
        tool.source = "sirius"
        registry.register(tool)
    return registry
