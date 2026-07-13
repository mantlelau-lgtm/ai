from __future__ import annotations

import json
import os
import platform
import shutil
from dataclasses import dataclass, field
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Awaitable, Callable


@dataclass(frozen=True)
class ToolParameter:
    name: str
    type: str
    description: str
    required: bool = False


@dataclass(frozen=True)
class ToolSpec:
    name: str
    description: str
    parameters: list[ToolParameter] = field(default_factory=list)

    def to_openai_schema(self) -> dict[str, Any]:
        properties: dict[str, Any] = {}
        required: list[str] = []
        for item in self.parameters:
            properties[item.name] = {"type": item.type, "description": item.description}
            if item.required:
                required.append(item.name)
        return {
            "type": "function",
            "function": {
                "name": self.name,
                "description": self.description,
                "parameters": {
                    "type": "object",
                    "properties": properties,
                    "required": required,
                },
            },
        }


@dataclass
class ToolCall:
    name: str
    arguments: dict[str, Any] = field(default_factory=dict)


@dataclass
class ToolResult:
    name: str
    ok: bool
    content: str
    data: dict[str, Any] = field(default_factory=dict)
    error: str = ""


ToolHandler = Callable[[ToolCall, Any], Awaitable[ToolResult]]


class AgentTool:
    def __init__(self, spec: ToolSpec, handler: ToolHandler, source: str = "core") -> None:
        self.spec = spec
        self.source = source
        self._handler = handler

    async def run(self, call: ToolCall, ctx: Any) -> ToolResult:
        return await self._handler(call, ctx)


class ToolRegistry:
    def __init__(self) -> None:
        self._tools: dict[str, AgentTool] = {}

    def register(self, tool: AgentTool) -> None:
        self._tools[tool.spec.name] = tool

    def get(self, name: str) -> AgentTool | None:
        return self._tools.get((name or "").strip())

    def specs(self) -> list[ToolSpec]:
        return [tool.spec for tool in self._tools.values()]

    def items(self) -> list[AgentTool]:
        return list(self._tools.values())

    def names(self) -> list[str]:
        return list(self._tools.keys())

    async def run(self, call: ToolCall, ctx: Any) -> ToolResult:
        tool = self.get(call.name)
        if tool is None:
            return ToolResult(name=call.name, ok=False, content="", error=f"tool not found: {call.name}")
        return await tool.run(call, ctx)


def _workspace_root() -> Path:
    return Path(os.getenv("AGENT_TOOLS_WORKSPACE_ROOT", "/Users/zxz/AI")).resolve()


def _safe_path(raw: Any) -> Path:
    root = _workspace_root()
    path_text = str(raw or ".").strip() or "."
    candidate = (root / path_text).resolve() if not Path(path_text).is_absolute() else Path(path_text).resolve()
    if root != candidate and root not in candidate.parents:
        raise ValueError(f"path outside workspace: {path_text}")
    return candidate


def _limit_text(value: str, max_chars: int) -> str:
    if max_chars <= 0 or len(value) <= max_chars:
        return value
    return value[:max_chars] + "\n...[truncated]"


def _json_result(name: str, data: dict[str, Any]) -> ToolResult:
    return ToolResult(name=name, ok=True, content=json.dumps(data, ensure_ascii=False), data=data)


async def _now_tool(call: ToolCall, ctx: Any) -> ToolResult:
    now = datetime.now(timezone.utc).isoformat()
    return ToolResult(name=call.name, ok=True, content=now, data={"now": now, "timezone": "UTC"})


async def _context_tool(call: ToolCall, ctx: Any) -> ToolResult:
    data = {
        "conversation_id": getattr(ctx, "conversation_id", ""),
        "agent_name": getattr(ctx, "agent_name", ""),
        "bot_id": getattr(ctx, "bot_id", ""),
        "user_id": getattr(ctx, "user_id", ""),
        "open_id": getattr(ctx, "open_id", ""),
        "chat_id": getattr(ctx, "chat_id", ""),
        "request_id": getattr(ctx, "request_id", ""),
        "trace_id": getattr(ctx, "trace_id", ""),
    }
    return _json_result(call.name, data)


async def _fs_list_tool(call: ToolCall, ctx: Any) -> ToolResult:
    try:
        base = _safe_path(call.arguments.get("path", "."))
        limit = int(call.arguments.get("limit", 100) or 100)
        if not base.exists():
            return ToolResult(name=call.name, ok=False, content="", error=f"path not found: {base}")
        if not base.is_dir():
            return ToolResult(name=call.name, ok=False, content="", error=f"path is not a directory: {base}")
        items = []
        for item in sorted(base.iterdir(), key=lambda p: (not p.is_dir(), p.name.lower()))[:limit]:
            stat = item.stat()
            items.append({"name": item.name, "path": str(item.relative_to(_workspace_root())), "type": "directory" if item.is_dir() else "file", "size": stat.st_size})
        return _json_result(call.name, {"root": str(_workspace_root()), "path": str(base), "items": items})
    except Exception as exc:
        return ToolResult(name=call.name, ok=False, content="", error=str(exc))


async def _fs_read_tool(call: ToolCall, ctx: Any) -> ToolResult:
    try:
        path = _safe_path(call.arguments.get("path", ""))
        max_chars = int(call.arguments.get("max_chars", 12000) or 12000)
        if not path.exists():
            return ToolResult(name=call.name, ok=False, content="", error=f"file not found: {path}")
        if not path.is_file():
            return ToolResult(name=call.name, ok=False, content="", error=f"path is not a file: {path}")
        text = path.read_text(encoding="utf-8", errors="replace")
        content = _limit_text(text, max_chars)
        return ToolResult(name=call.name, ok=True, content=content, data={"path": str(path), "chars": len(text), "truncated": len(content) < len(text)})
    except Exception as exc:
        return ToolResult(name=call.name, ok=False, content="", error=str(exc))


async def _fs_find_tool(call: ToolCall, ctx: Any) -> ToolResult:
    try:
        base = _safe_path(call.arguments.get("path", "."))
        pattern = str(call.arguments.get("pattern", "*") or "*")
        limit = int(call.arguments.get("limit", 50) or 50)
        results = []
        for item in base.rglob(pattern):
            if len(results) >= limit:
                break
            if item.is_file():
                results.append(str(item.relative_to(_workspace_root())))
        return _json_result(call.name, {"path": str(base), "pattern": pattern, "results": results})
    except Exception as exc:
        return ToolResult(name=call.name, ok=False, content="", error=str(exc))


async def _doc_read_tool(call: ToolCall, ctx: Any) -> ToolResult:
    args = dict(call.arguments)
    args.setdefault("max_chars", 20000)
    return await _fs_read_tool(ToolCall(name=call.name, arguments=args), ctx)


async def _doc_find_tool(call: ToolCall, ctx: Any) -> ToolResult:
    try:
        base = _safe_path(call.arguments.get("path", "."))
        keyword = str(call.arguments.get("keyword", "") or "").lower()
        limit = int(call.arguments.get("limit", 50) or 50)
        suffixes = {".md", ".txt", ".rst"}
        results = []
        for item in base.rglob("*"):
            if len(results) >= limit:
                break
            if not item.is_file() or item.suffix.lower() not in suffixes:
                continue
            rel = str(item.relative_to(_workspace_root()))
            if not keyword or keyword in item.name.lower() or keyword in item.read_text(encoding="utf-8", errors="replace").lower():
                results.append(rel)
        return _json_result(call.name, {"path": str(base), "keyword": keyword, "results": results})
    except Exception as exc:
        return ToolResult(name=call.name, ok=False, content="", error=str(exc))


async def _system_info_tool(call: ToolCall, ctx: Any) -> ToolResult:
    data = {
        "platform": platform.platform(),
        "system": platform.system(),
        "release": platform.release(),
        "machine": platform.machine(),
        "python": platform.python_version(),
        "workspace_root": str(_workspace_root()),
        "cwd": os.getcwd(),
    }
    return _json_result(call.name, data)


async def _system_resources_tool(call: ToolCall, ctx: Any) -> ToolResult:
    usage = shutil.disk_usage(_workspace_root())
    loadavg = os.getloadavg() if hasattr(os, "getloadavg") else (0.0, 0.0, 0.0)
    data = {
        "cpu_count": os.cpu_count(),
        "load_avg": list(loadavg),
        "disk": {"total": usage.total, "used": usage.used, "free": usage.free},
    }
    return _json_result(call.name, data)


def default_tool_registry() -> ToolRegistry:
    registry = ToolRegistry()
    core_tools = [
        AgentTool(ToolSpec(name="time.now", description="获取当前 UTC 时间。"), _now_tool),
        AgentTool(ToolSpec(name="context.info", description="获取当前消息的会话、用户、bot 和 trace 上下文。"), _context_tool),
        AgentTool(
            ToolSpec(
                name="fs.list",
                description="列举工作区内目录内容。只允许访问 AGENT_TOOLS_WORKSPACE_ROOT 内路径。",
                parameters=[
                    ToolParameter("path", "string", "相对工作区根目录的目录路径，默认 ."),
                    ToolParameter("limit", "integer", "最多返回条目数，默认 100"),
                ],
            ),
            _fs_list_tool,
        ),
        AgentTool(
            ToolSpec(
                name="fs.read",
                description="读取工作区内文本文件内容。只允许访问 AGENT_TOOLS_WORKSPACE_ROOT 内路径。",
                parameters=[
                    ToolParameter("path", "string", "相对工作区根目录的文件路径", True),
                    ToolParameter("max_chars", "integer", "最多返回字符数，默认 12000"),
                ],
            ),
            _fs_read_tool,
        ),
        AgentTool(
            ToolSpec(
                name="fs.find",
                description="按 glob pattern 查找工作区内文件。",
                parameters=[
                    ToolParameter("path", "string", "查找起点目录，默认 ."),
                    ToolParameter("pattern", "string", "glob 模式，例如 **/*.md 或 *.py", True),
                    ToolParameter("limit", "integer", "最多返回结果数，默认 50"),
                ],
            ),
            _fs_find_tool,
        ),
        AgentTool(
            ToolSpec(
                name="doc.read",
                description="读取工作区内文档文件内容，适合 Markdown/TXT/RST 等文本文档。",
                parameters=[
                    ToolParameter("path", "string", "相对工作区根目录的文档路径", True),
                    ToolParameter("max_chars", "integer", "最多返回字符数，默认 20000"),
                ],
            ),
            _doc_read_tool,
        ),
        AgentTool(
            ToolSpec(
                name="doc.find",
                description="在工作区内查找文档文件，可按文件名或内容关键词匹配 Markdown/TXT/RST。",
                parameters=[
                    ToolParameter("path", "string", "查找起点目录，默认 ."),
                    ToolParameter("keyword", "string", "文件名或内容关键词，默认返回文档文件"),
                    ToolParameter("limit", "integer", "最多返回结果数，默认 50"),
                ],
            ),
            _doc_find_tool,
        ),
        AgentTool(
            ToolSpec(name="system.info", description="查看基础系统信息、Python 版本和工作区根目录。"),
            _system_info_tool,
        ),
        AgentTool(
            ToolSpec(name="system.resources", description="查看 CPU 数量、load average 和工作区磁盘使用情况。"),
            _system_resources_tool,
        ),
    ]
    for tool in core_tools:
        tool.source = "core"
        registry.register(tool)
    return registry
