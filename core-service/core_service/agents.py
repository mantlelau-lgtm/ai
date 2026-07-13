from __future__ import annotations

import json
from dataclasses import dataclass
from datetime import datetime, timezone
from typing import Any, AsyncIterator, Optional

from core_service.agent_tools import ToolCall, ToolRegistry, default_tool_registry
from core_service.config import cfg
from core_service.llm_gateway import LLMGatewayClient, build_chat_messages
from core_service.models import Envelope, Usage


@dataclass
class AgentContext:
    conversation_id: str
    agent_name: str
    bot_id: str
    user_id: str
    open_id: str
    chat_id: str
    trace_id: str
    request_id: str
    user_input: str
    history: list[dict[str, str]]
    envelope: Envelope
    llm_key_name: str = ""
    allowed_tools: list[str] | None = None


class BaseAgent:
    name: str = "base"
    uses_llm: bool = False

    async def stream_reply(self, ctx: AgentContext) -> AsyncIterator[tuple[str, Optional[Usage]]]:
        raise NotImplementedError


class ToolCallingAgent(BaseAgent):
    uses_llm = True
    system_prompt: str = cfg.system_prompt
    default_model: str = cfg.default_model

    def __init__(self, tools: ToolRegistry | None = None, llm: LLMGatewayClient | None = None) -> None:
        self._tools = tools or default_tool_registry()
        self._llm = llm or LLMGatewayClient(cfg.llm_base_url, cfg.llm_chat_path, cfg.request_timeout_seconds)

    async def stream_reply(self, ctx: AgentContext) -> AsyncIterator[tuple[str, Optional[Usage]]]:
        tool_call = self._parse_explicit_tool_call(ctx.user_input)
        if tool_call is not None:
            async for chunk in self._stream_explicit_tool_call(tool_call, ctx):
                yield chunk
            return

        messages = self.build_messages(ctx)
        headers = self.build_headers(ctx)
        model = self.resolve_model(ctx)
        async for delta, usage in self.stream_with_auto_tools(model, messages, headers, ctx):
            yield delta, usage

    def build_messages(self, ctx: AgentContext) -> list[dict[str, Any]]:
        return build_chat_messages(self.system_prompt, ctx.history)

    def build_headers(self, ctx: AgentContext) -> dict[str, str]:
        headers = {
            "X-User-Id": ctx.user_id,
            "X-Session-Id": ctx.conversation_id,
            "X-Trace-Id": ctx.trace_id,
            "X-Request-Id": ctx.request_id,
        }
        if ctx.llm_key_name:
            headers["X-LLM-Key"] = ctx.llm_key_name
        return headers

    def resolve_model(self, ctx: AgentContext) -> str:
        return ctx.llm_key_name or self.default_model

    async def stream_with_auto_tools(
        self,
        model: str,
        messages: list[dict[str, Any]],
        headers: dict[str, str],
        ctx: AgentContext,
    ) -> AsyncIterator[tuple[str, Optional[Usage]]]:
        tool_schemas = [tool.spec.to_openai_schema() for tool in self._tools_for_context(ctx)]
        first = await self._llm.chat_once(model=model, messages=messages, headers=headers, tools=tool_schemas)
        message = ((first.get("choices") or [{}])[0].get("message") or {})
        tool_calls = self.extract_tool_calls(message)
        if not tool_calls:
            content = str(message.get("content") or "")
            if content:
                yield content, self.usage_from_response(first)
                return
            async for delta, usage in self._llm.stream_chat_events(model=model, messages=messages, headers=headers):
                yield delta, usage
            return

        messages.append(
            {
                "role": "assistant",
                "content": str(message.get("content") or ""),
                "tool_calls": message.get("tool_calls") or [],
            }
        )
        for tool_call in tool_calls:
            if not self._is_tool_allowed(tool_call.name, ctx):
                result_content = f"tool not allowed for agent {ctx.agent_name}: {tool_call.name}"
            else:
                result = await self._tools.run(tool_call, ctx)
                result_content = result.content if result.ok else result.error
            messages.append(
                {
                    "role": "tool",
                    "tool_call_id": tool_call.arguments.get("tool_call_id", ""),
                    "name": tool_call.name,
                    "content": result_content,
                }
            )
        async for delta, usage in self._llm.stream_chat_events(model=model, messages=messages, headers=headers):
            yield delta, usage

    async def _stream_explicit_tool_call(
        self, tool_call: ToolCall, ctx: AgentContext
    ) -> AsyncIterator[tuple[str, Optional[Usage]]]:
        if not self._is_tool_allowed(tool_call.name, ctx):
            yield f"tool {tool_call.name} error: tool not allowed for agent {ctx.agent_name}", Usage()
            return
        result = await self._tools.run(tool_call, ctx)
        status = "ok" if result.ok else "error"
        payload = result.content if result.ok else result.error
        yield f"tool {tool_call.name} {status}: {payload}", Usage()

    def _tools_for_context(self, ctx: AgentContext):
        allowed = {item.strip() for item in (ctx.allowed_tools or []) if item and item.strip()}
        if not allowed:
            return self._tools.items()
        return [tool for tool in self._tools.items() if tool.spec.name in allowed]

    def _is_tool_allowed(self, name: str, ctx: AgentContext) -> bool:
        allowed = {item.strip() for item in (ctx.allowed_tools or []) if item and item.strip()}
        return not allowed or name in allowed

    @staticmethod
    def extract_tool_calls(message: dict[str, Any]) -> list[ToolCall]:
        calls: list[ToolCall] = []
        for raw in message.get("tool_calls") or []:
            function = raw.get("function") or {}
            name = str(function.get("name") or "").strip()
            if not name:
                continue
            args_raw = function.get("arguments") or "{}"
            try:
                parsed = json.loads(args_raw) if isinstance(args_raw, str) else args_raw
            except Exception:
                parsed = {"input": str(args_raw)}
            if not isinstance(parsed, dict):
                parsed = {"input": parsed}
            parsed["tool_call_id"] = str(raw.get("id") or "")
            calls.append(ToolCall(name=name, arguments=parsed))
        return calls

    @staticmethod
    def usage_from_response(payload: dict[str, Any]) -> Usage:
        raw = payload.get("usage") or {}
        return Usage(
            prompt_tokens=int(raw.get("prompt_tokens") or 0),
            completion_tokens=int(raw.get("completion_tokens") or 0),
            total_tokens=int(raw.get("total_tokens") or 0),
            cost=float(raw.get("cost") or 0.0),
            latency_ms=int(raw.get("latency_ms") or 0),
        )

    @staticmethod
    def _parse_explicit_tool_call(text: str) -> ToolCall | None:
        raw = (text or "").strip()
        if raw.startswith("/tool "):
            raw = raw.removeprefix("/tool ").strip()
        elif raw.startswith("tool:"):
            raw = raw.removeprefix("tool:").strip()
        else:
            return None
        if not raw:
            return None
        name, _, args_text = raw.partition(" ")
        arguments: dict[str, object] = {}
        if args_text.strip():
            try:
                parsed = json.loads(args_text)
                if isinstance(parsed, dict):
                    arguments = parsed
            except Exception:
                arguments = {"input": args_text.strip()}
        return ToolCall(name=name.strip(), arguments=arguments)


class GeneralAgent(ToolCallingAgent):
    name = "general"


class EchoAgent(BaseAgent):
    name = "echo"

    async def stream_reply(self, ctx: AgentContext) -> AsyncIterator[tuple[str, Optional[Usage]]]:
        now = datetime.now(timezone.utc)
        usage = Usage(
            prompt_tokens=max(1, len(ctx.user_input.split())),
            completion_tokens=max(1, len(ctx.user_input.split())),
            total_tokens=max(2, len(ctx.user_input.split()) * 2),
            started_at=now,
            finished_at=now,
            latency_ms=0,
        )
        yield f"echo agent handled: {ctx.user_input}", usage


class AgentRegistry:
    def __init__(self, tools: ToolRegistry | None = None) -> None:
        from sirius.agent import SiriusAgent
        from sequoia_x.tools import sequoia_x_tool_registry

        self._tools = tools or default_tool_registry()
        sequoia_x_tool_registry(self._tools)
        self._agents: dict[str, BaseAgent] = {
            GeneralAgent.name: GeneralAgent(self._tools),
            SiriusAgent.name: SiriusAgent(self._tools),
            EchoAgent.name: EchoAgent(),
        }
        self._agent_keys: dict[str, str] = {}

    def get(self, name: str) -> BaseAgent:
        key = ((name or "").strip() or "general").lower()
        return self._agents.get(key, self._agents["general"])

    def names(self) -> list[str]:
        return sorted(self._agents.keys())

    def tool_specs(self):
        return self._tools.specs()

    def tool_items(self):
        return self._tools.items()

    def set_agent_key(self, name: str, key_name: str) -> None:
        normalized = (name or "").strip().lower()
        if not normalized:
            return
        if key_name:
            self._agent_keys[normalized] = key_name.strip()
        else:
            self._agent_keys.pop(normalized, None)

    def get_agent_key(self, name: str) -> str:
        return self._agent_keys.get((name or "").strip().lower(), "")
