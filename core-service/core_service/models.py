from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime
from typing import Any, Optional


@dataclass
class Envelope:
    event_id: str = ""
    event_type: str = ""
    kind: str = ""
    chat_id: str = ""
    chat_type: str = ""
    message_id: str = ""
    message_type: str = ""
    sender_open_id: str = ""
    sender_user_id: str = ""
    sender_union_id: str = ""
    tenant_key: str = ""
    text: str = ""
    action_name: str = ""
    action_tag: str = ""
    action_token: str = ""
    input_value: str = ""
    trace_id: str = ""

    @staticmethod
    def from_dict(raw: dict[str, Any]) -> "Envelope":
        return Envelope(
            event_id=str(raw.get("event_id", "") or ""),
            event_type=str(raw.get("event_type", "") or ""),
            kind=str(raw.get("kind", "") or ""),
            chat_id=str(raw.get("chat_id", "") or ""),
            chat_type=str(raw.get("chat_type", "") or ""),
            message_id=str(raw.get("message_id", "") or ""),
            message_type=str(raw.get("message_type", "") or ""),
            sender_open_id=str(raw.get("sender_open_id", "") or ""),
            sender_user_id=str(raw.get("sender_user_id", "") or ""),
            sender_union_id=str(raw.get("sender_union_id", "") or ""),
            tenant_key=str(raw.get("tenant_key", "") or ""),
            text=str(raw.get("text", "") or ""),
            action_name=str(raw.get("action_name", "") or ""),
            action_tag=str(raw.get("action_tag", "") or ""),
            action_token=str(raw.get("action_token", "") or ""),
            input_value=str(raw.get("input_value", "") or ""),
            trace_id=str(raw.get("trace_id", "") or ""),
        )


@dataclass
class Usage:
    prompt_tokens: int = 0
    completion_tokens: int = 0
    total_tokens: int = 0
    cost: float = 0.0
    latency_ms: int = 0
    started_at: Optional[datetime] = None
    finished_at: Optional[datetime] = None


@dataclass
class StreamChunk:
    type: str
    text: str = ""
    done: bool = False
    request_id: str = ""
    task_id: str = ""
    error: str = ""
    usage: Optional[Usage] = None


@dataclass
class Conversation:
    conversation_id: str
    bot_id: str = ""
    user_id: str = ""
    open_id: str = ""
    chat_id: str = ""
    created_at: Optional[datetime] = None
    updated_at: Optional[datetime] = None
    last_message_at: Optional[datetime] = None


@dataclass
class Message:
    id: int
    conversation_id: str
    role: str
    content: str
    event_id: str = ""
    message_id: str = ""
    created_at: Optional[datetime] = None


@dataclass
class Task:
    task_id: str
    request_id: str
    conversation_id: str
    status: str
    error_message: str = ""
    created_at: Optional[datetime] = None
    updated_at: Optional[datetime] = None
    completed_at: Optional[datetime] = None


TASK_STATUS_RUNNING = "running"
TASK_STATUS_SUCCEEDED = "succeeded"
TASK_STATUS_FAILED = "failed"
