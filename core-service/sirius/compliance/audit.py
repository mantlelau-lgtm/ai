from __future__ import annotations

from datetime import datetime, timezone
from typing import Any
import json

import asyncpg

_AUDIT_EVENTS: list[dict[str, Any]] = []
_POOL: asyncpg.Pool | None = None


def configure_audit_pool(pool: asyncpg.Pool | None) -> None:
    global _POOL
    _POOL = pool


async def record_event(event_type: str, payload: dict[str, Any]) -> dict[str, Any]:
    event = {"event_type": event_type, "timestamp": datetime.now(timezone.utc).isoformat(), "payload": payload}
    _AUDIT_EVENTS.append(event)
    if _POOL is not None:
        async with _POOL.acquire() as conn:
            row = await conn.fetchrow(
                """
                INSERT INTO sirius_audit_events(event_type, payload, created_at)
                VALUES ($1, $2::jsonb, NOW())
                RETURNING id, created_at
                """,
                event_type,
                json.dumps(payload, ensure_ascii=False),
            )
            if row is not None:
                event["id"] = int(row["id"])
                event["timestamp"] = row["created_at"].isoformat()
    return event


async def recent_events(limit: int = 20) -> list[dict[str, Any]]:
    if _POOL is not None:
        async with _POOL.acquire() as conn:
            rows = await conn.fetch(
                """
                SELECT id, event_type, payload, created_at
                FROM sirius_audit_events
                ORDER BY created_at DESC
                LIMIT $1
                """,
                max(1, min(limit, 200)),
            )
        return [
            {
                "id": int(row["id"]),
                "event_type": str(row["event_type"]),
                "timestamp": row["created_at"].isoformat(),
                "payload": dict(row["payload"]),
            }
            for row in rows
        ]
    return _AUDIT_EVENTS[-limit:]
