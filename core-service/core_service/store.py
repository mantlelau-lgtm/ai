from __future__ import annotations

from dataclasses import asdict
from datetime import datetime, timezone
from typing import Any, Optional

import asyncpg

from core_service.models import Message, Task


class Store:
    def __init__(self, pool: asyncpg.Pool) -> None:
        self._pool = pool

    async def migrate(self) -> None:
        statements = [
            """
            CREATE TABLE IF NOT EXISTS core_conversations (
              conversation_id TEXT PRIMARY KEY,
              bot_id TEXT NOT NULL DEFAULT '',
              user_id TEXT NOT NULL DEFAULT '',
              open_id TEXT NOT NULL DEFAULT '',
              chat_id TEXT NOT NULL DEFAULT '',
              created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
              updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
              last_message_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
            )
            """,
            """
            CREATE TABLE IF NOT EXISTS core_messages (
              id BIGSERIAL PRIMARY KEY,
              conversation_id TEXT NOT NULL REFERENCES core_conversations(conversation_id) ON DELETE CASCADE,
              role TEXT NOT NULL,
              content TEXT NOT NULL,
              event_id TEXT NOT NULL DEFAULT '',
              message_id TEXT NOT NULL DEFAULT '',
              created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
            )
            """,
            """
            CREATE TABLE IF NOT EXISTS core_tasks (
              task_id TEXT PRIMARY KEY,
              request_id TEXT NOT NULL,
              conversation_id TEXT NOT NULL REFERENCES core_conversations(conversation_id) ON DELETE CASCADE,
              status TEXT NOT NULL,
              error_message TEXT NOT NULL DEFAULT '',
              created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
              updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
              completed_at TIMESTAMPTZ NULL
            )
            """,
            "CREATE INDEX IF NOT EXISTS idx_core_messages_conversation_id ON core_messages(conversation_id, id DESC)",
            "CREATE INDEX IF NOT EXISTS idx_core_tasks_conversation_id ON core_tasks(conversation_id, created_at DESC)",
        ]
        async with self._pool.acquire() as conn:
            for stmt in statements:
                await conn.execute(stmt)

    async def ping(self) -> None:
        async with self._pool.acquire() as conn:
            await conn.execute("SELECT 1")

    async def upsert_conversation(
        self,
        conversation_id: str,
        bot_id: str,
        user_id: str,
        open_id: str,
        chat_id: str,
    ) -> None:
        async with self._pool.acquire() as conn:
            await conn.execute(
                """
                INSERT INTO core_conversations (conversation_id, bot_id, user_id, open_id, chat_id, created_at, updated_at, last_message_at)
                VALUES ($1,$2,$3,$4,$5,NOW(),NOW(),NOW())
                ON CONFLICT (conversation_id) DO UPDATE SET
                  bot_id = EXCLUDED.bot_id,
                  user_id = EXCLUDED.user_id,
                  open_id = EXCLUDED.open_id,
                  chat_id = EXCLUDED.chat_id,
                  updated_at = NOW(),
                  last_message_at = NOW()
                """,
                conversation_id,
                bot_id,
                user_id,
                open_id,
                chat_id,
            )

    async def append_message(
        self,
        conversation_id: str,
        role: str,
        content: str,
        event_id: str = "",
        message_id: str = "",
    ) -> None:
        async with self._pool.acquire() as conn:
            await conn.execute(
                """
                INSERT INTO core_messages (conversation_id, role, content, event_id, message_id, created_at)
                VALUES ($1,$2,$3,$4,$5,NOW())
                """,
                conversation_id,
                role,
                content,
                event_id,
                message_id,
            )
            await conn.execute(
                "UPDATE core_conversations SET updated_at = NOW(), last_message_at = NOW() WHERE conversation_id = $1",
                conversation_id,
            )

    async def list_recent_messages(self, conversation_id: str, limit: int) -> list[Message]:
        if limit <= 0:
            limit = 20
        async with self._pool.acquire() as conn:
            rows = await conn.fetch(
                """
                SELECT id, conversation_id, role, content, event_id, message_id, created_at
                FROM core_messages
                WHERE conversation_id = $1
                ORDER BY id DESC
                LIMIT $2
                """,
                conversation_id,
                limit,
            )
        items = [
            Message(
                id=int(r["id"]),
                conversation_id=str(r["conversation_id"]),
                role=str(r["role"]),
                content=str(r["content"]),
                event_id=str(r["event_id"]),
                message_id=str(r["message_id"]),
                created_at=r["created_at"],
            )
            for r in rows
        ]
        items.reverse()
        return items

    async def create_task(self, task_id: str, request_id: str, conversation_id: str, status: str) -> None:
        async with self._pool.acquire() as conn:
            await conn.execute(
                """
                INSERT INTO core_tasks (task_id, request_id, conversation_id, status, error_message, created_at, updated_at)
                VALUES ($1,$2,$3,$4,'',NOW(),NOW())
                """,
                task_id,
                request_id,
                conversation_id,
                status,
            )

    async def complete_task(self, task_id: str) -> None:
        now = datetime.now(timezone.utc)
        async with self._pool.acquire() as conn:
            await conn.execute(
                "UPDATE core_tasks SET status = 'succeeded', updated_at = NOW(), completed_at = $2, error_message = '' WHERE task_id = $1",
                task_id,
                now,
            )

    async def fail_task(self, task_id: str, message: str) -> None:
        now = datetime.now(timezone.utc)
        async with self._pool.acquire() as conn:
            await conn.execute(
                "UPDATE core_tasks SET status = 'failed', updated_at = NOW(), completed_at = $2, error_message = $3 WHERE task_id = $1",
                task_id,
                now,
                message,
            )

    async def get_task(self, task_id: str) -> Optional[Task]:
        async with self._pool.acquire() as conn:
            row = await conn.fetchrow(
                """
                SELECT task_id, request_id, conversation_id, status, error_message, created_at, updated_at, completed_at
                FROM core_tasks
                WHERE task_id = $1
                """,
                task_id,
            )
        if row is None:
            return None
        return Task(
            task_id=str(row["task_id"]),
            request_id=str(row["request_id"]),
            conversation_id=str(row["conversation_id"]),
            status=str(row["status"]),
            error_message=str(row["error_message"]),
            created_at=row["created_at"],
            updated_at=row["updated_at"],
            completed_at=row["completed_at"],
        )

    async def list_conversation_messages(self, conversation_id: str, limit: int) -> list[dict[str, Any]]:
        items = await self.list_recent_messages(conversation_id, limit)
        return [asdict(i) for i in items]
