from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime, timezone
from typing import AsyncIterator, Optional

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


class BaseAgent:
    name: str = "base"
    uses_llm: bool = False

    async def stream_reply(self, ctx: AgentContext) -> AsyncIterator[tuple[str, Optional[Usage]]]:
        raise NotImplementedError


class GeneralAgent(BaseAgent):
    name = "general"
    uses_llm = True

    def __init__(self) -> None:
        self._llm = LLMGatewayClient(cfg.llm_base_url, cfg.llm_chat_path, cfg.request_timeout_seconds)

    async def stream_reply(self, ctx: AgentContext) -> AsyncIterator[tuple[str, Optional[Usage]]]:
        messages = build_chat_messages(cfg.system_prompt, ctx.history)
        headers = {
            "X-User-Id": ctx.user_id,
            "X-Session-Id": ctx.conversation_id,
            "X-Trace-Id": ctx.trace_id,
            "X-Request-Id": ctx.request_id,
        }
        if ctx.llm_key_name:
            headers["X-LLM-Key"] = ctx.llm_key_name
        # model = key_name（llm-gw 的 catalog 中以 key_name 作为 model 名路由）
        model = ctx.llm_key_name or cfg.default_model
        async for delta, usage in self._llm.stream_chat_events(
            model=model,
            messages=messages,
            headers=headers,
        ):
            yield delta, usage


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
    def __init__(self) -> None:
        self._agents: dict[str, BaseAgent] = {
            GeneralAgent.name: GeneralAgent(),
            EchoAgent.name: EchoAgent(),
        }
        self._agent_keys: dict[str, str] = {}

    def get(self, name: str) -> BaseAgent:
        key = ((name or "").strip() or "general").lower()
        return self._agents.get(key, self._agents["general"])

    def names(self) -> list[str]:
        return sorted(self._agents.keys())

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
