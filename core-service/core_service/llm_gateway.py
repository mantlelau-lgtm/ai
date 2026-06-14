from __future__ import annotations

import json
import time
from typing import Any, AsyncIterator, Optional, Tuple

import httpx

from core_service.logging import logger
from core_service.models import Usage


class LLMGatewayClient:
    def __init__(self, base_url: str, chat_path: str, timeout_seconds: int) -> None:
        self._url = base_url.rstrip("/") + chat_path
        self._timeout = timeout_seconds

    async def stream_chat_events(
        self,
        model: str,
        messages: list[dict[str, str]],
        headers: dict[str, str],
    ) -> AsyncIterator[Tuple[str, Optional[Usage]]]:
        req: dict[str, Any] = {
            "model": model,
            "messages": messages,
            "stream": True,
            "stream_options": {"include_usage": True},
        }

        started = time.monotonic()
        logger.info(
            "llm gateway stream request started",
            extra={
                "request_id": headers.get("X-Request-Id", ""),
                "trace_id": headers.get("X-Trace-Id", ""),
                "llm_key_name": headers.get("X-LLM-Key", ""),
                "model": model,
                "messages": len(messages),
                "url": self._url,
            },
        )
        async with httpx.AsyncClient(timeout=self._timeout) as client:
            async with client.stream(
                "POST",
                self._url,
                headers={**headers, "Content-Type": "application/json", "Accept": "text/event-stream"},
                json=req,
            ) as resp:
                logger.info(
                    "llm gateway stream response received",
                    extra={"request_id": headers.get("X-Request-Id", ""), "status_code": resp.status_code},
                )
                resp.raise_for_status()
                async for line in resp.aiter_lines():
                    line = line.strip()
                    if not line or line.startswith(":") or not line.startswith("data:"):
                        continue
                    data = line.removeprefix("data:").strip()
                    if not data:
                        continue
                    if data == "[DONE]":
                        logger.info(
                            "llm gateway stream completed",
                            extra={"request_id": headers.get("X-Request-Id", ""), "latency_ms": int((time.monotonic() - started) * 1000)},
                        )
                        return
                    try:
                        payload = json.loads(data)
                    except Exception:
                        yield data, None
                        continue

                    usage_obj = None
                    u = payload.get("usage")
                    if u:
                        usage_obj = Usage(
                            prompt_tokens=int(u.get("prompt_tokens") or 0),
                            completion_tokens=int(u.get("completion_tokens") or 0),
                            total_tokens=int(u.get("total_tokens") or 0),
                            cost=float(u.get("cost") or 0.0),
                            latency_ms=int(u.get("latency_ms") or 0),
                        )

                    delta_text = ""
                    for choice in payload.get("choices") or []:
                        delta_text += str((choice.get("delta") or {}).get("content") or "")
                    if delta_text:
                        yield delta_text, usage_obj
                    elif usage_obj is not None:
                        yield "", usage_obj


def build_chat_messages(system_prompt: str, history: list[dict[str, str]]) -> list[dict[str, str]]:
    items: list[dict[str, str]] = []
    if system_prompt.strip():
        items.append({"role": "system", "content": system_prompt})
    items.extend(history)
    return items
