from __future__ import annotations

import json
from typing import Any

import httpx

from core_service.agent_tools import AgentTool, ToolCall, ToolParameter, ToolRegistry, ToolResult, ToolSpec
from core_service.config import cfg
from sequoia_x.client import SequoiaXClient


def _ok(name: str, data: Any) -> ToolResult:
    payload = {"source": "sequoia-x", "data": data}
    return ToolResult(name=name, ok=True, content=json.dumps(payload, ensure_ascii=False), data=payload)


def _err(name: str, error: str) -> ToolResult:
    return ToolResult(name=name, ok=False, content="", error=error, data={"source": "sequoia-x", "error": error})


def _repo(call: ToolCall) -> str:
    raw = str(call.arguments.get("repo", "") or "").strip()
    return raw or cfg.sequoia_x_default_repo


async def _call_json(call: ToolCall, client: SequoiaXClient, method: str, path: str, *, params: dict[str, Any] | None = None) -> ToolResult:
    try:
        if method == "GET":
            data = await client.get_json(path, params=params)
        else:
            data = await client.post_json(path, params=params)
    except httpx.HTTPStatusError as exc:
        return _err(call.name, f"sequoia-x http {exc.response.status_code}: {exc.response.text[:300]}")
    except Exception as exc:
        return _err(call.name, str(exc))
    return _ok(call.name, data)


async def _health_tool(call: ToolCall, ctx: Any) -> ToolResult:
    return await _call_json(call, SequoiaXClient(), "GET", "/health")


async def _repositories_tool(call: ToolCall, ctx: Any) -> ToolResult:
    return await _call_json(call, SequoiaXClient(), "GET", "/api/repositories")


async def _repo_status_tool(call: ToolCall, ctx: Any) -> ToolResult:
    return await _call_json(call, SequoiaXClient(), "GET", f"/api/repositories/{_repo(call)}/status")


async def _data_summary_tool(call: ToolCall, ctx: Any) -> ToolResult:
    return await _call_json(call, SequoiaXClient(), "GET", f"/api/repositories/{_repo(call)}/data/summary")


async def _symbols_tool(call: ToolCall, ctx: Any) -> ToolResult:
    params: dict[str, Any] = {}
    query = str(call.arguments.get("query", "") or "").strip()
    if query:
        params["query"] = query
    limit = int(call.arguments.get("limit", 50) or 50)
    params["limit"] = max(1, min(limit, 200))
    return await _call_json(call, SequoiaXClient(), "GET", f"/api/repositories/{_repo(call)}/symbols", params=params)


async def _ohlcv_tool(call: ToolCall, ctx: Any) -> ToolResult:
    symbol = str(call.arguments.get("symbol", "") or "").strip()
    if not symbol:
        return _err(call.name, "symbol is required")
    limit = int(call.arguments.get("limit", 180) or 180)
    params = {"limit": max(20, min(limit, 1000))}
    return await _call_json(call, SequoiaXClient(), "GET", f"/api/repositories/{_repo(call)}/symbols/{symbol}/ohlcv", params=params)


async def _strategies_tool(call: ToolCall, ctx: Any) -> ToolResult:
    return await _call_json(call, SequoiaXClient(), "GET", f"/api/repositories/{_repo(call)}/strategies")


async def _strategy_run_tool(call: ToolCall, ctx: Any) -> ToolResult:
    strategy = str(call.arguments.get("strategy", "") or "").strip()
    if not strategy:
        return _err(call.name, "strategy is required")
    return await _call_json(call, SequoiaXClient(), "POST", f"/api/repositories/{_repo(call)}/strategies/{strategy}/run")


async def _strategy_run_all_tool(call: ToolCall, ctx: Any) -> ToolResult:
    return await _call_json(call, SequoiaXClient(), "POST", f"/api/repositories/{_repo(call)}/strategies/run-all")


async def _incremental_backfill_tool(call: ToolCall, ctx: Any) -> ToolResult:
    return await _call_json(call, SequoiaXClient(), "POST", f"/api/repositories/{_repo(call)}/tasks/incremental-backfill")


async def _logs_tool(call: ToolCall, ctx: Any) -> ToolResult:
    lines = int(call.arguments.get("lines", 120) or 120)
    params = {"lines": max(20, min(lines, 1000))}
    try:
        text = await SequoiaXClient().get_text(f"/api/repositories/{_repo(call)}/logs", params=params)
    except httpx.HTTPStatusError as exc:
        return _err(call.name, f"sequoia-x http {exc.response.status_code}: {exc.response.text[:300]}")
    except Exception as exc:
        return _err(call.name, str(exc))
    return _ok(call.name, {"text": text, "lines": params["lines"]})


def sequoia_x_tool_registry(base: ToolRegistry | None = None) -> ToolRegistry:
    registry = base or ToolRegistry()
    sx_tools = [
        AgentTool(ToolSpec(name="sx.health", description="检查 Sequoia-X 服务健康状态。"), _health_tool),
        AgentTool(ToolSpec(name="sx.repositories", description="列出 Sequoia-X 已接入的仓库列表。"), _repositories_tool),
        AgentTool(ToolSpec(name="sx.repository_status", description="查询指定仓库状态、数据库摘要和脚本可用性。", parameters=[ToolParameter("repo", "string", "仓库 ID，默认为 sequoia-x")]), _repo_status_tool),
        AgentTool(ToolSpec(name="sx.data_summary", description="查询仓库数据库的行情摘要。", parameters=[ToolParameter("repo", "string", "仓库 ID")]), _data_summary_tool),
        AgentTool(ToolSpec(name="sx.symbols", description="模糊搜索仓库中的股票代码。", parameters=[ToolParameter("query", "string", "代码片段"), ToolParameter("limit", "integer", "返回条数 1-200"), ToolParameter("repo", "string", "仓库 ID")]), _symbols_tool),
        AgentTool(ToolSpec(name="sx.ohlcv", description="获取股票 K 线，优先 baostock 实时数据。", parameters=[ToolParameter("symbol", "string", "6 位股票代码", True), ToolParameter("limit", "integer", "K 线根数 20-1000"), ToolParameter("repo", "string", "仓库 ID")]), _ohlcv_tool),
        AgentTool(ToolSpec(name="sx.strategies", description="列出仓库内置策略。", parameters=[ToolParameter("repo", "string", "仓库 ID")]), _strategies_tool),
        AgentTool(ToolSpec(name="sx.strategy_run", description="运行指定策略并返回选股结果。", parameters=[ToolParameter("strategy", "string", "策略 ID", True), ToolParameter("repo", "string", "仓库 ID")]), _strategy_run_tool),
        AgentTool(ToolSpec(name="sx.strategy_run_all", description="顺序运行全部策略，耗时较长。", parameters=[ToolParameter("repo", "string", "仓库 ID")]), _strategy_run_all_tool),
        AgentTool(ToolSpec(name="sx.incremental_backfill", description="后台触发每日增量回填任务。", parameters=[ToolParameter("repo", "string", "仓库 ID")]), _incremental_backfill_tool),
        AgentTool(ToolSpec(name="sx.logs", description="读取仓库最近日志（plain text）。", parameters=[ToolParameter("lines", "integer", "行数 20-1000"), ToolParameter("repo", "string", "仓库 ID")]), _logs_tool),
    ]
    for tool in sx_tools:
        tool.source = "sequoia-x"
        registry.register(tool)
    return registry
