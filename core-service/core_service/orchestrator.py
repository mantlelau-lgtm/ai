from __future__ import annotations

import json
import uuid
from datetime import datetime, timezone
from typing import AsyncIterator, Optional

from core_service.agents import AgentContext, AgentRegistry
from core_service.config import cfg
from core_service.logging import logger
from core_service.metrics import metrics
from core_service.models import (
    Envelope,
    StreamChunk,
    TASK_STATUS_FAILED,
    TASK_STATUS_RUNNING,
    TASK_STATUS_SUCCEEDED,
    Usage,
)
from core_service.routing import RoutingManager
from core_service.store import Store


class Orchestrator:
    def __init__(
        self,
        store: Store,
        registry: Optional[AgentRegistry] = None,
        routing: Optional[RoutingManager] = None,
    ) -> None:
        self._store = store
        self._registry = registry or AgentRegistry()
        self._routing = routing

    async def process_stream(
        self,
        envelope: Envelope,
        header_bot_id: str,
        header_agent_name: str,
        header_session_id: str,
        header_user_id: str,
        header_open_id: str,
        header_chat_id: str,
        header_trace_id: str,
        header_request_id: str,
    ) -> AsyncIterator[bytes]:
        metrics.inc_requests()
        metrics.inc_streams()

        started_at = datetime.now(timezone.utc)
        conversation_id = self._pick_conversation_id(header_session_id, envelope)
        request_id = header_request_id.strip() or envelope.event_id.strip() or f"req-{uuid.uuid4().hex}"
        task_id = f"task-{uuid.uuid4().hex}"

        user_input = self._extract_user_input(envelope)
        if not user_input.strip():
            logger.warning("message stream rejected: empty input", extra={"request_id": request_id, "event_id": envelope.event_id})
            metrics.inc_failed()
            yield self._sse(StreamChunk(type="error", done=True, error="empty envelope input", request_id=request_id, task_id=task_id))
            yield b"data: [DONE]\n\n"
            return

        bot_id = header_bot_id.strip()
        agent_name = header_agent_name.strip().lower()
        if not agent_name and self._routing is not None and self._routing.current is not None:
            agent_name = self._routing.current.lookup_agent_name(bot_id).strip().lower()
            if not agent_name:
                yield self._sse(StreamChunk(type="start", request_id=request_id, task_id=task_id))
                msg = "当前 bot 没有配置对应的agent。"
                yield self._sse(StreamChunk(type="done", text=msg, done=True, request_id=request_id, task_id=task_id, usage=Usage()))
                yield b"data: [DONE]\n\n"
                return
        if not agent_name:
            agent_name = "general"
        agent = self._registry.get(agent_name)
        llm_key_name = ""
        allowed_tools: list[str] = []
        if self._routing is not None and self._routing.current is not None:
            llm_key_name = self._routing.current.get_agent_key_name(agent_name)
            allowed_tools = self._routing.current.get_agent_tools(agent_name)
        logger.info(
            "agent resolved for message stream",
            extra={
                "request_id": request_id,
                "task_id": task_id,
                "event_id": envelope.event_id,
                "bot_id": bot_id,
                "agent_name": agent_name,
                "llm_key_name": llm_key_name,
                "allowed_tools_count": len(allowed_tools),
            },
        )
        user_id = header_user_id.strip() or envelope.sender_user_id
        open_id = header_open_id.strip() or envelope.sender_open_id
        chat_id = header_chat_id.strip() or envelope.chat_id
        trace_id = header_trace_id.strip() or envelope.trace_id

        await self._store.upsert_conversation(
            conversation_id=conversation_id,
            bot_id=bot_id,
            user_id=user_id,
            open_id=open_id,
            chat_id=chat_id,
        )
        await self._store.create_task(task_id=task_id, request_id=request_id, conversation_id=conversation_id, status=TASK_STATUS_RUNNING)
        await self._store.append_message(
            conversation_id=conversation_id,
            role="user",
            content=user_input,
            event_id=envelope.event_id,
            message_id=envelope.message_id,
        )

        history_rows = await self._store.list_recent_messages(conversation_id, cfg.conversation_window_size)
        history = [{"role": m.role, "content": m.content} for m in history_rows]
        agent_ctx = AgentContext(
            conversation_id=conversation_id,
            agent_name=agent.name,
            bot_id=bot_id,
            user_id=user_id,
            open_id=open_id,
            chat_id=chat_id,
            trace_id=trace_id,
            request_id=request_id,
            user_input=user_input,
            history=history,
            envelope=envelope,
            llm_key_name=llm_key_name,
            allowed_tools=allowed_tools,
        )

        yield self._sse(StreamChunk(type="start", request_id=request_id, task_id=task_id))

        if agent.uses_llm:
            metrics.inc_llm_calls()
        answer_parts: list[str] = []
        last_usage: Optional[Usage] = None
        try:
            async for delta, usage in agent.stream_reply(agent_ctx):
                if usage is not None:
                    last_usage = usage
                if delta:
                    answer_parts.append(delta)
                    yield self._sse(StreamChunk(type="delta", text=delta, request_id=request_id, task_id=task_id))
        except Exception as e:
            logger.exception("agent stream failed", extra={"request_id": request_id, "task_id": task_id, "agent_name": agent_name, "llm_key_name": llm_key_name})
            metrics.inc_failed()
            await self._store.fail_task(task_id, str(e))
            yield self._sse(StreamChunk(type="error", done=True, error=str(e), request_id=request_id, task_id=task_id))
            yield b"data: [DONE]\n\n"
            return

        assistant = "".join(answer_parts).strip()
        if assistant:
            await self._store.append_message(conversation_id=conversation_id, role="assistant", content=assistant)
        await self._store.complete_task(task_id)

        finished_at = datetime.now(timezone.utc)
        logger.info(
            "message stream completed",
            extra={
                "request_id": request_id,
                "task_id": task_id,
                "agent_name": agent_name,
                "llm_key_name": llm_key_name,
                "answer_chars": len(assistant),
                "latency_ms": int((finished_at - started_at).total_seconds() * 1000),
            },
        )
        if last_usage is None:
            last_usage = Usage()
        last_usage.started_at = started_at
        last_usage.finished_at = finished_at
        last_usage.latency_ms = int((finished_at - started_at).total_seconds() * 1000)

        yield self._sse(
            StreamChunk(
                type="done",
                text=assistant,
                done=True,
                request_id=request_id,
                task_id=task_id,
                usage=last_usage,
            )
        )
        yield b"data: [DONE]\n\n"

    @staticmethod
    def _extract_user_input(env: Envelope) -> str:
        for item in (env.text, env.input_value, env.action_name, env.action_tag):
            if item and item.strip():
                return item
        return ""

    @staticmethod
    def _pick_conversation_id(session_id: str, env: Envelope) -> str:
        if session_id and session_id.strip():
            return session_id.strip()
        if env.chat_id and env.chat_id.strip():
            return env.chat_id.strip()
        if env.sender_user_id and env.sender_user_id.strip():
            return env.sender_user_id.strip()
        if env.sender_open_id and env.sender_open_id.strip():
            return env.sender_open_id.strip()
        return f"conv-{uuid.uuid4().hex}"

    @staticmethod
    def _sse(chunk: StreamChunk) -> bytes:
        payload = {
            "type": chunk.type,
            "text": chunk.text,
            "done": chunk.done,
            "request_id": chunk.request_id,
            "task_id": chunk.task_id,
        }
        if chunk.error:
            payload["error"] = chunk.error
        if chunk.usage is not None:
            payload["usage"] = {
                "prompt_tokens": chunk.usage.prompt_tokens,
                "completion_tokens": chunk.usage.completion_tokens,
                "total_tokens": chunk.usage.total_tokens,
                "cost": chunk.usage.cost,
                "latency_ms": chunk.usage.latency_ms,
                "started_at": chunk.usage.started_at.isoformat() if chunk.usage.started_at else None,
                "finished_at": chunk.usage.finished_at.isoformat() if chunk.usage.finished_at else None,
            }
        data = json.dumps(payload, ensure_ascii=False)
        return f"data: {data}\n\n".encode("utf-8")
