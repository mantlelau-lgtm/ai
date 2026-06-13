from __future__ import annotations

import json
from typing import Protocol
from urllib.parse import urlparse

import asyncpg

from app.models import (
    AgentSpec,
    BotConfig,
    BotsFile,
    BundlePayload,
    DatabaseMeta,
    LlmCatalog,
    LlmKey,
    LlmModel,
    LlmProvider,
    MessageRouteAction,
    MessageRouteConfig,
    MessageRouteMatch,
    MessageRouteRule,
    RoutingConfig,
    RoutingEntry,
    TableMeta,
)

SCHEMA_SQL = """
CREATE TABLE IF NOT EXISTS admin_bots (
    id BIGSERIAL PRIMARY KEY,
    bot_id TEXT NOT NULL UNIQUE,
    app_id TEXT NOT NULL UNIQUE,
    app_secret TEXT NOT NULL,
    open_base_url TEXT NOT NULL DEFAULT '',
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS admin_llm_keys (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    value TEXT NOT NULL DEFAULT '',
    value_env TEXT NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS admin_llm_providers (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    provider_type TEXT NOT NULL,
    base_url TEXT NOT NULL,
    api_key TEXT NOT NULL DEFAULT '',
    api_key_id BIGINT REFERENCES admin_llm_keys(id) ON DELETE RESTRICT,
    model_prefixes_json TEXT NOT NULL DEFAULT '[]',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    metadata_json TEXT NOT NULL DEFAULT '{}',
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS admin_llm_models (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    provider_id BIGINT NOT NULL REFERENCES admin_llm_providers(id) ON DELETE RESTRICT,
    upstream_model TEXT NOT NULL,
    owned_by TEXT NOT NULL DEFAULT '',
    prompt_cost_per_1k_tokens NUMERIC(18, 6) NOT NULL DEFAULT 0,
    completion_cost_per_1k_tokens NUMERIC(18, 6) NOT NULL DEFAULT 0,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS admin_routing_settings (
    id BIGINT PRIMARY KEY CHECK (id = 1),
    default_agent TEXT NOT NULL DEFAULT 'general',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS admin_routes (
    id BIGSERIAL PRIMARY KEY,
    bot_id TEXT NOT NULL UNIQUE,
    agent_name TEXT NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS admin_agent_specs (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    agent_type TEXT NOT NULL DEFAULT 'custom',
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS admin_message_route_rules (
    id BIGSERIAL PRIMARY KEY,
    rule_id TEXT NOT NULL UNIQUE,
    priority INTEGER NOT NULL DEFAULT 0,
    match_json TEXT NOT NULL DEFAULT '{}',
    action_json TEXT NOT NULL DEFAULT '{}',
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
"""


class ConfigRepository(Protocol):
    async def load_bundle(self) -> BundlePayload: ...

    async def save_bundle(self, bundle: BundlePayload) -> list[str]: ...

    async def table_meta(self) -> dict[str, TableMeta]: ...

    async def database_meta(self) -> DatabaseMeta: ...


class PostgresConfigRepository:
    def __init__(self, database_url: str):
        self.database_url = database_url
        self.pool: asyncpg.Pool | None = None

    async def open(self) -> None:
        self.pool = await asyncpg.create_pool(dsn=self.database_url, min_size=1, max_size=5)

    async def close(self) -> None:
        if self.pool is not None:
            await self.pool.close()

    async def migrate(self) -> None:
        pool = self._pool()
        async with pool.acquire() as conn:
            await conn.execute(SCHEMA_SQL)
            await conn.execute("ALTER TABLE admin_llm_keys ADD COLUMN IF NOT EXISTS value TEXT NOT NULL DEFAULT ''")
            await conn.execute("ALTER TABLE admin_llm_providers ADD COLUMN IF NOT EXISTS api_key TEXT NOT NULL DEFAULT ''")
            await conn.execute(
                "ALTER TABLE admin_llm_providers ADD COLUMN IF NOT EXISTS model_prefixes_json TEXT NOT NULL DEFAULT '[]'"
            )
            await conn.execute(
                "ALTER TABLE admin_llm_providers ADD COLUMN IF NOT EXISTS metadata_json TEXT NOT NULL DEFAULT '{}'"
            )
            await conn.execute("ALTER TABLE admin_llm_providers ALTER COLUMN api_key_id DROP NOT NULL")
            await conn.execute(
                """
                INSERT INTO admin_routing_settings(id, default_agent)
                VALUES (1, 'general')
                ON CONFLICT (id) DO NOTHING
                """
            )

    async def load_bundle(self) -> BundlePayload:
        pool = self._pool()
        async with pool.acquire() as conn:
            bots_rows = await conn.fetch(
                """
                SELECT bot_id, app_id, app_secret, open_base_url
                FROM admin_bots
                ORDER BY sort_order, id
                """
            )
            key_rows = await conn.fetch(
                """
                SELECT id, name, value, value_env
                FROM admin_llm_keys
                ORDER BY sort_order, id
                """
            )
            provider_rows = await conn.fetch(
                """
                SELECT p.id, p.name, p.provider_type, p.base_url, p.api_key, p.model_prefixes_json,
                       p.enabled, p.is_default, p.metadata_json, COALESCE(k.name, '') AS api_key_from
                FROM admin_llm_providers p
                LEFT JOIN admin_llm_keys k ON k.id = p.api_key_id
                ORDER BY p.sort_order, p.id
                """
            )
            model_rows = await conn.fetch(
                """
                SELECT m.name,
                       p.name AS provider_name,
                       m.upstream_model,
                       m.owned_by,
                       m.prompt_cost_per_1k_tokens,
                       m.completion_cost_per_1k_tokens,
                       m.enabled
                FROM admin_llm_models m
                JOIN admin_llm_providers p ON p.id = m.provider_id
                ORDER BY m.sort_order, m.id
                """
            )
            routing_settings = await conn.fetchrow(
                """
                SELECT default_agent
                FROM admin_routing_settings
                WHERE id = 1
                """
            )
            route_rows = await conn.fetch(
                """
                SELECT bot_id, agent_name
                FROM admin_routes
                ORDER BY sort_order, id
                """
            )
            agent_rows = await conn.fetch(
                """
                SELECT name, agent_type
                FROM admin_agent_specs
                ORDER BY sort_order, id
                """
            )
            message_route_rows = await conn.fetch(
                """
                SELECT rule_id, priority, match_json, action_json
                FROM admin_message_route_rules
                ORDER BY sort_order, id
                """
            )

        return BundlePayload(
            bots=BotsFile(
                bots=[
                    BotConfig(
                        bot_id=row["bot_id"],
                        app_id=row["app_id"],
                        app_secret=row["app_secret"],
                        open_base_url=row["open_base_url"],
                    )
                    for row in bots_rows
                ]
            ),
            llm=LlmCatalog(
                keys=[
                    LlmKey(name=row["name"], value=row["value"], value_env=row["value_env"])
                    for row in key_rows
                ],
                providers=[
                    LlmProvider(
                        name=row["name"],
                        type=row["provider_type"],
                        base_url=row["base_url"],
                        api_key=row["api_key"],
                        api_key_from=row["api_key_from"],
                        model_prefixes=_loads_json_list(row["model_prefixes_json"]),
                        enabled=row["enabled"],
                        is_default=row["is_default"],
                        metadata=_loads_json_dict(row["metadata_json"]),
                    )
                    for row in provider_rows
                ],
                models=[
                    LlmModel(
                        name=row["name"],
                        provider=row["provider_name"],
                        upstream_model=row["upstream_model"],
                        owned_by=row["owned_by"],
                        prompt_cost_per_1k_tokens=float(row["prompt_cost_per_1k_tokens"]),
                        completion_cost_per_1k_tokens=float(row["completion_cost_per_1k_tokens"]),
                        enabled=row["enabled"],
                    )
                    for row in model_rows
                ],
            ),
            routing=RoutingConfig(
                default_agent=(routing_settings["default_agent"] if routing_settings else "general"),
                bots=[
                    RoutingEntry(bot_id=row["bot_id"], agent_name=row["agent_name"])
                    for row in route_rows
                ],
                agents=[
                    AgentSpec(name=row["name"], type=row["agent_type"])
                    for row in agent_rows
                ],
            ),
            message_routes=MessageRouteConfig(
                rules=[
                    MessageRouteRule(
                        id=row["rule_id"],
                        priority=row["priority"],
                        match=MessageRouteMatch.model_validate(_loads_json_dict(row["match_json"])),
                        action=MessageRouteAction.model_validate(_loads_json_dict(row["action_json"])),
                    )
                    for row in message_route_rows
                ],
            ),
        )

    async def save_bundle(self, bundle: BundlePayload) -> list[str]:
        pool = self._pool()
        async with pool.acquire() as conn:
            async with conn.transaction():
                await conn.execute(
                    """
                    TRUNCATE TABLE
                        admin_llm_models,
                        admin_llm_providers,
                        admin_llm_keys,
                        admin_message_route_rules,
                        admin_agent_specs,
                        admin_routes,
                        admin_bots
                    RESTART IDENTITY CASCADE
                    """
                )
                await conn.execute(
                    """
                    UPDATE admin_routing_settings
                    SET default_agent = $1, updated_at = NOW()
                    WHERE id = 1
                    """,
                    bundle.routing.default_agent,
                )

                key_ids: dict[str, int] = {}
                for index, key in enumerate(bundle.llm.keys):
                    key_id = await conn.fetchval(
                        """
                        INSERT INTO admin_llm_keys(name, value, value_env, sort_order, updated_at)
                        VALUES ($1, $2, $3, $4, NOW())
                        RETURNING id
                        """,
                        key.name.strip(),
                        key.value.strip(),
                        key.value_env.strip(),
                        index,
                    )
                    key_ids[key.name.strip()] = int(key_id)

                provider_ids: dict[str, int] = {}
                for index, provider in enumerate(bundle.llm.providers):
                    api_key_from = provider.api_key_from.strip()
                    api_key_id = key_ids.get(api_key_from)
                    provider_id = await conn.fetchval(
                        """
                        INSERT INTO admin_llm_providers(
                            name, provider_type, base_url, api_key, api_key_id, model_prefixes_json,
                            enabled, is_default, metadata_json, sort_order, updated_at
                        )
                        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW())
                        RETURNING id
                        """,
                        provider.name.strip(),
                        provider.type.strip(),
                        provider.base_url.strip(),
                        provider.api_key.strip(),
                        api_key_id,
                        json.dumps(provider.model_prefixes, ensure_ascii=False),
                        provider.enabled,
                        provider.is_default,
                        json.dumps(provider.metadata, ensure_ascii=False),
                        index,
                    )
                    provider_ids[provider.name.strip()] = int(provider_id)

                for index, model in enumerate(bundle.llm.models):
                    await conn.execute(
                        """
                        INSERT INTO admin_llm_models(
                            name, provider_id, upstream_model, owned_by,
                            prompt_cost_per_1k_tokens, completion_cost_per_1k_tokens,
                            enabled, sort_order, updated_at
                        )
                        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
                        """,
                        model.name.strip(),
                        provider_ids[model.provider.strip()],
                        model.upstream_model.strip(),
                        model.owned_by.strip(),
                        model.prompt_cost_per_1k_tokens,
                        model.completion_cost_per_1k_tokens,
                        model.enabled,
                        index,
                    )

                for index, bot in enumerate(bundle.bots.bots):
                    await conn.execute(
                        """
                        INSERT INTO admin_bots(bot_id, app_id, app_secret, open_base_url, sort_order, updated_at)
                        VALUES ($1, $2, $3, $4, $5, NOW())
                        """,
                        bot.bot_id.strip(),
                        bot.app_id.strip(),
                        bot.app_secret.strip(),
                        bot.open_base_url.strip(),
                        index,
                    )

                for index, route in enumerate(bundle.routing.bots):
                    await conn.execute(
                        """
                        INSERT INTO admin_routes(bot_id, agent_name, sort_order, updated_at)
                        VALUES ($1, $2, $3, NOW())
                        """,
                        route.bot_id.strip(),
                        route.agent_name.strip(),
                        index,
                    )

                for index, agent in enumerate(bundle.routing.agents):
                    await conn.execute(
                        """
                        INSERT INTO admin_agent_specs(name, agent_type, sort_order, updated_at)
                        VALUES ($1, $2, $3, NOW())
                        """,
                        agent.name.strip().lower(),
                        agent.type.strip() or "custom",
                        index,
                    )

                for index, rule in enumerate(bundle.message_routes.rules):
                    await conn.execute(
                        """
                        INSERT INTO admin_message_route_rules(rule_id, priority, match_json, action_json, sort_order, updated_at)
                        VALUES ($1, $2, $3, $4, $5, NOW())
                        """,
                        rule.id.strip(),
                        rule.priority,
                        json.dumps(rule.match.model_dump(mode="json"), ensure_ascii=False),
                        json.dumps(rule.action.model_dump(mode="json"), ensure_ascii=False),
                        index,
                    )

        return [
            "admin_bots",
            "admin_llm_keys",
            "admin_llm_providers",
            "admin_llm_models",
            "admin_routing_settings",
            "admin_routes",
            "admin_agent_specs",
            "admin_message_route_rules",
        ]

    async def table_meta(self) -> dict[str, TableMeta]:
        pool = self._pool()
        async with pool.acquire() as conn:
            results: dict[str, TableMeta] = {}
            for table_name in (
                "admin_bots",
                "admin_llm_keys",
                "admin_llm_providers",
                "admin_llm_models",
                "admin_routing_settings",
                "admin_routes",
                "admin_agent_specs",
                "admin_message_route_rules",
            ):
                row = await conn.fetchrow(
                    f"SELECT COUNT(*)::BIGINT AS row_count, MAX(updated_at) AS updated_at FROM {table_name}"
                )
                updated_at = row["updated_at"].timestamp() if row["updated_at"] is not None else None
                results[table_name] = TableMeta(
                    name=table_name,
                    rows=int(row["row_count"]),
                    updated_at=updated_at,
                )
            return results

    async def database_meta(self) -> DatabaseMeta:
        pool = self._pool()
        async with pool.acquire() as conn:
            await conn.execute("SELECT 1")
        parsed = urlparse(self.database_url)
        return DatabaseMeta(
            engine="postgresql",
            database=(parsed.path or "/").lstrip("/") or "unknown",
            status="ok",
            detail="database connected",
        )

    def _pool(self) -> asyncpg.Pool:
        if self.pool is None:
            raise RuntimeError("database pool not initialized")
        return self.pool


def _loads_json_list(raw: str) -> list[str]:
    try:
        data = json.loads(raw or "[]")
    except Exception:
        return []
    if not isinstance(data, list):
        return []
    return [str(item) for item in data if str(item).strip()]


def _loads_json_dict(raw: str) -> dict[str, str]:
    try:
        data = json.loads(raw or "{}")
    except Exception:
        return {}
    if not isinstance(data, dict):
        return {}
    return {str(key): str(value) for key, value in data.items()}
