from __future__ import annotations

import json
import os
from typing import Protocol
from urllib.parse import urlparse

import asyncpg

from app.models import (
    AdminLlmConfig,
    AgentSpec,
    AdminRoutingConfig,
    BotConfig,
    BotsFile,
    BundlePayload,
    DatabaseMeta,
    LlmCatalog,
    LlmCredential,
    LlmKey,
    LlmModel,
    LlmProvider,
    MessageRouteAction,
    MessageRouteConfig,
    MessageRouteMatch,
    MessageRouteRule,
    RegisteredAgent,
    RoutingConfig,
    RoutingEntry,
    TableMeta,
)
from app.services.crypto import SecretCryptor

SCHEMA_SQL = """
CREATE TABLE IF NOT EXISTS admin_bots (
    id BIGSERIAL PRIMARY KEY,
    bot_id TEXT NOT NULL UNIQUE,
    app_id TEXT NOT NULL UNIQUE,
    app_secret TEXT NOT NULL,
    open_base_url TEXT NOT NULL DEFAULT '',
    agent_name TEXT NOT NULL DEFAULT '',
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS admin_llm_credentials (
    id BIGSERIAL PRIMARY KEY,
    vendor_name TEXT NOT NULL,
    provider_type TEXT NOT NULL,
    call_type TEXT NOT NULL DEFAULT 'non_stream',
    base_url TEXT NOT NULL,
    key_name TEXT NOT NULL UNIQUE,
    encrypted_key_value TEXT NOT NULL DEFAULT '',
    model_id TEXT NOT NULL DEFAULT '',
    metadata_json TEXT NOT NULL DEFAULT '{}',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE admin_llm_credentials
    ADD COLUMN IF NOT EXISTS call_type TEXT NOT NULL DEFAULT 'non_stream';
ALTER TABLE admin_llm_credentials
    ADD COLUMN IF NOT EXISTS model_id TEXT NOT NULL DEFAULT '';
ALTER TABLE admin_llm_credentials
    DROP CONSTRAINT IF EXISTS admin_llm_credentials_vendor_name_key;

CREATE UNIQUE INDEX IF NOT EXISTS admin_llm_credentials_key_name_key
    ON admin_llm_credentials(key_name);
CREATE UNIQUE INDEX IF NOT EXISTS admin_llm_credentials_default_unique
    ON admin_llm_credentials((1)) WHERE is_default;

CREATE TABLE IF NOT EXISTS admin_llm_models (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    model_id TEXT NOT NULL UNIQUE,
    vendor_name TEXT NOT NULL,
    upstream_model TEXT NOT NULL,
    owned_by TEXT NOT NULL DEFAULT '',
    prompt_cost_per_1k_tokens NUMERIC(18, 6) NOT NULL DEFAULT 0,
    completion_cost_per_1k_tokens NUMERIC(18, 6) NOT NULL DEFAULT 0,
    unit_price NUMERIC(18, 6) NOT NULL DEFAULT 0,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE admin_llm_models
    ADD COLUMN IF NOT EXISTS model_id TEXT;
ALTER TABLE admin_llm_models
    ADD COLUMN IF NOT EXISTS unit_price NUMERIC(18, 6) NOT NULL DEFAULT 0;

CREATE UNIQUE INDEX IF NOT EXISTS admin_llm_models_model_id_key
    ON admin_llm_models(model_id);

CREATE TABLE IF NOT EXISTS admin_routing_settings (
    id BIGINT PRIMARY KEY CHECK (id = 1),
    default_agent TEXT NOT NULL DEFAULT 'general',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS admin_registered_agents (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    agent_type TEXT NOT NULL DEFAULT 'custom',
    source TEXT NOT NULL DEFAULT 'core-service',
    description TEXT NOT NULL DEFAULT '',
    key_name TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE admin_registered_agents
    ADD COLUMN IF NOT EXISTS key_name TEXT NOT NULL DEFAULT '';

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

    async def load_runtime_catalog(self) -> LlmCatalog: ...

    async def load_runtime_routing(self) -> RoutingConfig: ...

    async def load_registered_agents(self) -> list[RegisteredAgent]: ...

    async def register_agents(self, agents: list[RegisteredAgent]) -> int: ...

    async def update_agent_key(self, name: str, key_name: str) -> RegisteredAgent | None: ...

    async def list_credential_key_names(self) -> list[str]: ...

    async def set_default_credential(self, key_name: str) -> str | None: ...

    async def upsert_llm_model(self, model: LlmModel, original_model_id: str = "") -> LlmModel: ...

    async def delete_llm_model(self, model_id: str) -> bool: ...

    async def upsert_llm_credential(self, credential: LlmCredential, original_key_name: str = "") -> LlmCredential: ...

    async def delete_llm_credential(self, key_name: str) -> bool: ...

    async def save_bundle(self, bundle: BundlePayload) -> list[str]: ...

    async def table_meta(self) -> dict[str, TableMeta]: ...

    async def database_meta(self) -> DatabaseMeta: ...


class PostgresConfigRepository:
    def __init__(self, database_url: str, cryptor: SecretCryptor):
        self.database_url = database_url
        self.cryptor = cryptor
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
            await conn.execute("ALTER TABLE admin_bots ADD COLUMN IF NOT EXISTS agent_name TEXT NOT NULL DEFAULT ''")
            await conn.execute("ALTER TABLE admin_llm_models ADD COLUMN IF NOT EXISTS vendor_name TEXT")
            await conn.execute("ALTER TABLE admin_llm_models ALTER COLUMN vendor_name DROP NOT NULL")
            if await self._table_exists(conn, "admin_llm_models") and await self._column_exists(
                conn, "admin_llm_models", "provider_id"
            ):
                await conn.execute("ALTER TABLE admin_llm_models ALTER COLUMN provider_id DROP NOT NULL")
            await conn.execute(
                "UPDATE admin_llm_models SET model_id = name WHERE model_id IS NULL OR model_id = ''"
            )
            await conn.execute(
                "ALTER TABLE admin_llm_models ALTER COLUMN model_id SET NOT NULL"
            )
            await conn.execute(
                """
                INSERT INTO admin_routing_settings(id, default_agent)
                VALUES (1, 'general')
                ON CONFLICT (id) DO NOTHING
                """
            )
            # 兼容旧表：admin_registered_agents.model_name -> key_name
            if await self._column_exists(conn, "admin_registered_agents", "model_name"):
                await conn.execute(
                    """
                    UPDATE admin_registered_agents
                    SET key_name = model_name
                    WHERE COALESCE(key_name, '') = '' AND COALESCE(model_name, '') <> ''
                    """
                )
                await conn.execute(
                    "ALTER TABLE admin_registered_agents DROP COLUMN IF EXISTS model_name"
                )
            await self._migrate_legacy_llm(conn)
            await self._migrate_legacy_routes(conn)
            await self._migrate_legacy_agents(conn)

    async def load_bundle(self) -> BundlePayload:
        pool = self._pool()
        async with pool.acquire() as conn:
            bots_rows = await conn.fetch(
                """
                SELECT bot_id, app_id, app_secret, open_base_url, agent_name
                FROM admin_bots
                ORDER BY sort_order, id
                """
            )
            credential_rows = await conn.fetch(
                """
                SELECT vendor_name, provider_type, call_type, base_url, key_name, encrypted_key_value,
                       model_id, metadata_json, enabled, is_default
                FROM admin_llm_credentials
                ORDER BY sort_order, id
                """
            )
            model_rows = await conn.fetch(
                """
                SELECT name, model_id, vendor_name, upstream_model, owned_by,
                       prompt_cost_per_1k_tokens, completion_cost_per_1k_tokens,
                       unit_price, enabled
                FROM admin_llm_models
                ORDER BY sort_order, id
                """
            )
            routing_settings = await conn.fetchrow(
                """
                SELECT default_agent
                FROM admin_routing_settings
                WHERE id = 1
                """
            )
            message_route_rows = await conn.fetch(
                """
                SELECT rule_id, priority, match_json, action_json
                FROM admin_message_route_rules
                ORDER BY sort_order, id
                """
            )

        models = [
            LlmModel(
                name=row["name"],
                model_id=row["model_id"],
                provider=row["vendor_name"],
                upstream_model=row["upstream_model"],
                owned_by=row["owned_by"],
                prompt_cost_per_1k_tokens=float(row["prompt_cost_per_1k_tokens"]),
                completion_cost_per_1k_tokens=float(row["completion_cost_per_1k_tokens"]),
                unit_price=float(row["unit_price"]),
                enabled=row["enabled"],
            )
            for row in model_rows
        ]
        model_index = {item.model_id: item for item in models if item.model_id}
        credentials = []
        for row in credential_rows:
            model_id = row["model_id"] or ""
            referenced = model_index.get(model_id)
            credentials.append(
                LlmCredential(
                    vendor_name=(referenced.provider if referenced else row["vendor_name"]),
                    type=row["provider_type"],
                    call_type=row["call_type"],
                    base_url=row["base_url"],
                    key_name=row["key_name"],
                    key_value=self.cryptor.decrypt(row["encrypted_key_value"]),
                    model_id=model_id,
                    model_name=(referenced.name if referenced else ""),
                    metadata=_loads_json_dict(row["metadata_json"]),
                    enabled=row["enabled"],
                    is_default=row["is_default"],
                )
            )
        return BundlePayload(
            bots=BotsFile(
                bots=[
                    BotConfig(
                        bot_id=row["bot_id"],
                        app_id=row["app_id"],
                        app_secret=row["app_secret"],
                        open_base_url=row["open_base_url"],
                        agent_name=row["agent_name"],
                    )
                    for row in bots_rows
                ]
            ),
            llm=AdminLlmConfig(
                credentials=credentials,
                models=models,
            ),
            routing=AdminRoutingConfig(
                default_agent=(routing_settings["default_agent"] if routing_settings else "general"),
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

    async def load_runtime_catalog(self) -> LlmCatalog:
        bundle = await self.load_bundle()
        models_by_id = {item.model_id: item for item in bundle.llm.models if item.model_id}
        providers: list[LlmProvider] = []
        runtime_models: list[LlmModel] = []
        for credential in bundle.llm.credentials:
            referenced = models_by_id.get(credential.model_id)
            providers.append(
                LlmProvider(
                    name=credential.key_name,
                    type=credential.type,
                    base_url=credential.base_url,
                    api_key=credential.key_value,
                    enabled=credential.enabled,
                    is_default=credential.is_default,
                    metadata={**credential.metadata, "call_type": credential.call_type, "vendor_name": credential.vendor_name},
                )
            )
            if referenced is not None and referenced.enabled:
                runtime_models.append(
                    LlmModel(
                        name=credential.key_name,
                        model_id=referenced.model_id,
                        provider=credential.key_name,
                        upstream_model=referenced.upstream_model or referenced.model_id,
                        owned_by=referenced.owned_by or referenced.provider,
                        prompt_cost_per_1k_tokens=referenced.prompt_cost_per_1k_tokens,
                        completion_cost_per_1k_tokens=referenced.completion_cost_per_1k_tokens,
                        unit_price=referenced.unit_price,
                        enabled=referenced.enabled,
                    )
                )
        return LlmCatalog(
            keys=[],
            providers=providers,
            models=runtime_models,
        )

    async def load_runtime_routing(self) -> RoutingConfig:
        pool = self._pool()
        async with pool.acquire() as conn:
            routing_settings = await conn.fetchrow(
                """
                SELECT default_agent
                FROM admin_routing_settings
                WHERE id = 1
                """
            )
            bot_rows = await conn.fetch(
                """
                SELECT bot_id, agent_name
                FROM admin_bots
                WHERE COALESCE(agent_name, '') <> ''
                ORDER BY sort_order, id
                """
            )
            agent_rows = await conn.fetch(
                """
                SELECT name, agent_type, COALESCE(key_name, '') AS key_name
                FROM admin_registered_agents
                ORDER BY name
                """
            )

        agents = [
            AgentSpec(name=row["name"], type=row["agent_type"], key_name=row["key_name"])
            for row in agent_rows
        ]
        default_agent = (routing_settings["default_agent"] if routing_settings else "general") or "general"
        if default_agent and not any(item.name == default_agent for item in agents):
            agents.append(AgentSpec(name=default_agent, type="general"))
        return RoutingConfig(
            default_agent=default_agent,
            bots=[RoutingEntry(bot_id=row["bot_id"], agent_name=row["agent_name"]) for row in bot_rows],
            agents=agents,
        )

    async def load_registered_agents(self) -> list[RegisteredAgent]:
        pool = self._pool()
        async with pool.acquire() as conn:
            rows = await conn.fetch(
                """
                SELECT name, agent_type, source, description, COALESCE(key_name, '') AS key_name
                FROM admin_registered_agents
                ORDER BY name
                """
            )
        return [
            RegisteredAgent(
                name=row["name"],
                type=row["agent_type"],
                source=row["source"],
                description=row["description"],
                key_name=row["key_name"],
            )
            for row in rows
        ]

    async def register_agents(self, agents: list[RegisteredAgent]) -> int:
        pool = self._pool()
        async with pool.acquire() as conn:
            async with conn.transaction():
                count = 0
                for item in agents:
                    name = item.name.strip().lower()
                    if not name:
                        continue
                    await conn.execute(
                        """
                        INSERT INTO admin_registered_agents(name, agent_type, source, description, key_name, updated_at)
                        VALUES ($1, $2, $3, $4, $5, NOW())
                        ON CONFLICT (name) DO UPDATE
                        SET agent_type = EXCLUDED.agent_type,
                            source = EXCLUDED.source,
                            description = EXCLUDED.description,
                            key_name = CASE
                                WHEN EXCLUDED.key_name <> '' THEN EXCLUDED.key_name
                                ELSE admin_registered_agents.key_name
                            END,
                            updated_at = NOW()
                        """,
                        name,
                        item.type.strip() or "custom",
                        item.source.strip() or "core-service",
                        item.description.strip(),
                        item.key_name.strip(),
                    )
                    count += 1
        return count

    async def update_agent_key(self, name: str, key_name: str) -> RegisteredAgent | None:
        normalized = (name or "").strip().lower()
        if not normalized:
            return None
        target_key = (key_name or "").strip()
        pool = self._pool()
        async with pool.acquire() as conn:
            if target_key:
                exists = await conn.fetchval(
                    "SELECT 1 FROM admin_llm_credentials WHERE key_name = $1",
                    target_key,
                )
                if not exists:
                    return None
            row = await conn.fetchrow(
                """
                UPDATE admin_registered_agents
                SET key_name = $2, updated_at = NOW()
                WHERE name = $1
                RETURNING name, agent_type, source, description, COALESCE(key_name, '') AS key_name
                """,
                normalized,
                target_key,
            )
        if not row:
            return None
        return RegisteredAgent(
            name=row["name"],
            type=row["agent_type"],
            source=row["source"],
            description=row["description"],
            key_name=row["key_name"],
        )

    async def list_credential_key_names(self) -> list[str]:
        pool = self._pool()
        async with pool.acquire() as conn:
            rows = await conn.fetch(
                "SELECT key_name FROM admin_llm_credentials ORDER BY sort_order, id"
            )
        return [row["key_name"] for row in rows]

    async def set_default_credential(self, key_name: str) -> str | None:
        normalized = (key_name or "").strip()
        if not normalized:
            return None
        pool = self._pool()
        async with pool.acquire() as conn:
            async with conn.transaction():
                exists = await conn.fetchval(
                    "SELECT 1 FROM admin_llm_credentials WHERE key_name = $1",
                    normalized,
                )
                if not exists:
                    return None
                # 先把所有密钥的 is_default 置 false，再把目标置 true
                await conn.execute(
                    "UPDATE admin_llm_credentials SET is_default = FALSE, updated_at = NOW() WHERE is_default = TRUE"
                )
                await conn.execute(
                    "UPDATE admin_llm_credentials SET is_default = TRUE, updated_at = NOW() WHERE key_name = $1",
                    normalized,
                )
        return normalized

    async def upsert_llm_model(self, model: LlmModel, original_model_id: str = "") -> LlmModel:
        model_id = model.model_id.strip() or model.name.strip()
        normalized_original = (original_model_id or model_id).strip()
        pool = self._pool()
        async with pool.acquire() as conn:
            row = await conn.fetchrow(
                """
                INSERT INTO admin_llm_models(
                    name, model_id, vendor_name, upstream_model, owned_by,
                    prompt_cost_per_1k_tokens, completion_cost_per_1k_tokens,
                    unit_price, enabled, updated_at
                )
                VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
                ON CONFLICT (model_id) DO UPDATE
                SET name = EXCLUDED.name,
                    vendor_name = EXCLUDED.vendor_name,
                    upstream_model = EXCLUDED.upstream_model,
                    owned_by = EXCLUDED.owned_by,
                    prompt_cost_per_1k_tokens = EXCLUDED.prompt_cost_per_1k_tokens,
                    completion_cost_per_1k_tokens = EXCLUDED.completion_cost_per_1k_tokens,
                    unit_price = EXCLUDED.unit_price,
                    enabled = EXCLUDED.enabled,
                    updated_at = NOW()
                RETURNING name, model_id, vendor_name, upstream_model, owned_by,
                          prompt_cost_per_1k_tokens, completion_cost_per_1k_tokens,
                          unit_price, enabled
                """,
                model.name.strip(),
                model_id,
                model.provider.strip(),
                model.upstream_model.strip() or model_id,
                model.owned_by.strip(),
                model.prompt_cost_per_1k_tokens,
                model.completion_cost_per_1k_tokens,
                model.unit_price,
                model.enabled,
            )
            if normalized_original and normalized_original != model_id:
                await conn.execute(
                    "UPDATE admin_llm_credentials SET model_id = $1, updated_at = NOW() WHERE model_id = $2",
                    model_id,
                    normalized_original,
                )
        return LlmModel(
            name=row["name"],
            model_id=row["model_id"],
            provider=row["vendor_name"],
            upstream_model=row["upstream_model"],
            owned_by=row["owned_by"],
            prompt_cost_per_1k_tokens=float(row["prompt_cost_per_1k_tokens"]),
            completion_cost_per_1k_tokens=float(row["completion_cost_per_1k_tokens"]),
            unit_price=float(row["unit_price"]),
            enabled=row["enabled"],
        )

    async def delete_llm_model(self, model_id: str) -> bool:
        normalized = (model_id or "").strip()
        if not normalized:
            return False
        pool = self._pool()
        async with pool.acquire() as conn:
            in_use = await conn.fetchval(
                "SELECT 1 FROM admin_llm_credentials WHERE model_id = $1 LIMIT 1",
                normalized,
            )
            if in_use:
                raise ValueError("模型已被密钥绑定，不能删除")
            result = await conn.execute("DELETE FROM admin_llm_models WHERE model_id = $1", normalized)
        return result.endswith(" 1")

    async def upsert_llm_credential(self, credential: LlmCredential, original_key_name: str = "") -> LlmCredential:
        target_key = credential.key_name.strip()
        original = (original_key_name or target_key).strip()
        pool = self._pool()
        async with pool.acquire() as conn:
            referenced = await conn.fetchrow(
                """
                SELECT name, model_id, vendor_name
                FROM admin_llm_models
                WHERE model_id = $1
                """,
                credential.model_id.strip(),
            )
            if referenced is None:
                raise ValueError(f"模型ID不存在: {credential.model_id}")
            async with conn.transaction():
                if credential.is_default:
                    await conn.execute(
                        "UPDATE admin_llm_credentials SET is_default = FALSE, updated_at = NOW() WHERE is_default = TRUE AND key_name <> $1",
                        target_key,
                    )
                row = await conn.fetchrow(
                    """
                    INSERT INTO admin_llm_credentials(
                        vendor_name, provider_type, call_type, base_url, key_name, encrypted_key_value,
                        model_id, metadata_json, enabled, is_default, updated_at
                    )
                    VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW())
                    ON CONFLICT (key_name) DO UPDATE
                    SET vendor_name = EXCLUDED.vendor_name,
                        provider_type = EXCLUDED.provider_type,
                        call_type = EXCLUDED.call_type,
                        base_url = EXCLUDED.base_url,
                        encrypted_key_value = EXCLUDED.encrypted_key_value,
                        model_id = EXCLUDED.model_id,
                        metadata_json = EXCLUDED.metadata_json,
                        enabled = EXCLUDED.enabled,
                        is_default = EXCLUDED.is_default,
                        updated_at = NOW()
                    RETURNING vendor_name, provider_type, call_type, base_url, key_name,
                              encrypted_key_value, model_id, metadata_json, enabled, is_default
                    """,
                    referenced["vendor_name"],
                    credential.type.strip() or "openai",
                    credential.call_type.strip() or "non_stream",
                    credential.base_url.strip(),
                    target_key,
                    self.cryptor.encrypt(credential.key_value),
                    credential.model_id.strip(),
                    json.dumps(credential.metadata, ensure_ascii=False),
                    credential.enabled,
                    credential.is_default,
                )
                if original and original != target_key:
                    await conn.execute("DELETE FROM admin_llm_credentials WHERE key_name = $1", original)
                    await conn.execute(
                        "UPDATE admin_registered_agents SET key_name = $1, updated_at = NOW() WHERE key_name = $2",
                        target_key,
                        original,
                    )
        return LlmCredential(
            vendor_name=row["vendor_name"],
            type=row["provider_type"],
            call_type=row["call_type"],
            base_url=row["base_url"],
            key_name=row["key_name"],
            key_value=self.cryptor.decrypt(row["encrypted_key_value"]),
            model_id=row["model_id"],
            model_name=referenced["name"],
            metadata=_loads_json_dict(row["metadata_json"]),
            enabled=row["enabled"],
            is_default=row["is_default"],
        )

    async def delete_llm_credential(self, key_name: str) -> bool:
        normalized = (key_name or "").strip()
        if not normalized:
            return False
        pool = self._pool()
        async with pool.acquire() as conn:
            async with conn.transaction():
                await conn.execute(
                    "UPDATE admin_registered_agents SET key_name = '', updated_at = NOW() WHERE key_name = $1",
                    normalized,
                )
                result = await conn.execute("DELETE FROM admin_llm_credentials WHERE key_name = $1", normalized)
        return result.endswith(" 1")

    async def save_bundle(self, bundle: BundlePayload) -> list[str]:
        pool = self._pool()
        async with pool.acquire() as conn:
            async with conn.transaction():
                await conn.execute(
                    """
                    TRUNCATE TABLE
                        admin_llm_models,
                        admin_llm_credentials,
                        admin_message_route_rules,
                        admin_bots
                    RESTART IDENTITY CASCADE
                    """
                )
                if await self._table_exists(conn, "admin_llm_keys") and await self._table_exists(conn, "admin_llm_providers"):
                    await conn.execute("TRUNCATE TABLE admin_llm_keys, admin_llm_providers RESTART IDENTITY CASCADE")
                await conn.execute(
                    """
                    UPDATE admin_routing_settings
                    SET default_agent = $1, updated_at = NOW()
                    WHERE id = 1
                    """,
                    bundle.routing.default_agent,
                )

                # 保证 is_default 唯一：仅保留第一条 is_default=True 的凭证
                seen_default = False
                normalized_credentials: list[LlmCredential] = []
                for credential in bundle.llm.credentials:
                    is_default = bool(credential.is_default and not seen_default)
                    if is_default:
                        seen_default = True
                    normalized_credentials.append(credential.model_copy(update={"is_default": is_default}))

                model_lookup = {item.model_id: item for item in bundle.llm.models if item.model_id}
                for index, credential in enumerate(normalized_credentials):
                    referenced = model_lookup.get(credential.model_id.strip())
                    vendor_name = (
                        referenced.provider.strip() if referenced else credential.vendor_name.strip()
                    )
                    await conn.execute(
                        """
                        INSERT INTO admin_llm_credentials(
                            vendor_name, provider_type, call_type, base_url, key_name, encrypted_key_value,
                            model_id, metadata_json, enabled, is_default, sort_order, updated_at
                        )
                        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NOW())
                        """,
                        vendor_name,
                        credential.type.strip(),
                        credential.call_type.strip() or "non_stream",
                        credential.base_url.strip(),
                        credential.key_name.strip(),
                        self.cryptor.encrypt(credential.key_value),
                        credential.model_id.strip(),
                        json.dumps(credential.metadata, ensure_ascii=False),
                        credential.enabled,
                        credential.is_default,
                        index,
                    )

                for index, model in enumerate(bundle.llm.models):
                    model_id = model.model_id.strip() or model.name.strip()
                    await conn.execute(
                        """
                        INSERT INTO admin_llm_models(
                            name, model_id, vendor_name, upstream_model, owned_by,
                            prompt_cost_per_1k_tokens, completion_cost_per_1k_tokens,
                            unit_price, enabled, sort_order, updated_at
                        )
                        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW())
                        """,
                        model.name.strip(),
                        model_id,
                        model.provider.strip(),
                        model.upstream_model.strip() or model_id,
                        model.owned_by.strip(),
                        model.prompt_cost_per_1k_tokens,
                        model.completion_cost_per_1k_tokens,
                        model.unit_price,
                        model.enabled,
                        index,
                    )

                for index, bot in enumerate(bundle.bots.bots):
                    await conn.execute(
                        """
                        INSERT INTO admin_bots(bot_id, app_id, app_secret, open_base_url, agent_name, sort_order, updated_at)
                        VALUES ($1, $2, $3, $4, $5, $6, NOW())
                        """,
                        bot.bot_id.strip(),
                        bot.app_id.strip(),
                        bot.app_secret.strip(),
                        bot.open_base_url.strip(),
                        bot.agent_name.strip().lower(),
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
            "admin_llm_credentials",
            "admin_llm_models",
            "admin_routing_settings",
            "admin_registered_agents",
            "admin_message_route_rules",
        ]

    async def table_meta(self) -> dict[str, TableMeta]:
        pool = self._pool()
        async with pool.acquire() as conn:
            results: dict[str, TableMeta] = {}
            for table_name in (
                "admin_bots",
                "admin_llm_credentials",
                "admin_llm_models",
                "admin_routing_settings",
                "admin_registered_agents",
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

    async def _migrate_legacy_llm(self, conn: asyncpg.Connection) -> None:
        has_new_rows = await conn.fetchval("SELECT EXISTS(SELECT 1 FROM admin_llm_credentials)")
        if has_new_rows:
            return

        if not await self._table_exists(conn, "admin_llm_providers"):
            return

        if await self._column_exists(conn, "admin_llm_models", "provider_id"):
            await conn.execute(
                """
                UPDATE admin_llm_models m
                SET vendor_name = p.name
                FROM admin_llm_providers p
                WHERE m.provider_id = p.id
                  AND COALESCE(m.vendor_name, '') = ''
                """
            )

        legacy_rows = await conn.fetch(
            """
            SELECT p.name, p.provider_type, p.base_url, p.api_key, p.enabled, p.is_default, p.metadata_json,
                   COALESCE(k.name, '') AS key_name, COALESCE(k.value, '') AS key_value, COALESCE(k.value_env, '') AS key_value_env
            FROM admin_llm_providers p
            LEFT JOIN admin_llm_keys k ON k.id = p.api_key_id
            ORDER BY p.sort_order, p.id
            """
        )
        for index, row in enumerate(legacy_rows):
            key_value = row["api_key"] or row["key_value"] or os.getenv(row["key_value_env"], "")
            await conn.execute(
                """
                INSERT INTO admin_llm_credentials(
                    vendor_name, provider_type, base_url, key_name, encrypted_key_value,
                    metadata_json, enabled, is_default, sort_order, updated_at
                )
                VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
                ON CONFLICT (vendor_name) DO NOTHING
                """,
                row["name"],
                row["provider_type"],
                row["base_url"],
                row["key_name"] or row["name"],
                self.cryptor.encrypt(key_value),
                row["metadata_json"] or "{}",
                row["enabled"],
                row["is_default"],
                index,
            )

    async def _migrate_legacy_routes(self, conn: asyncpg.Connection) -> None:
        if not await self._table_exists(conn, "admin_routes"):
            return
        await conn.execute(
            """
            UPDATE admin_bots b
            SET agent_name = r.agent_name
            FROM admin_routes r
            WHERE b.bot_id = r.bot_id
              AND COALESCE(b.agent_name, '') = ''
            """
        )

    async def _migrate_legacy_agents(self, conn: asyncpg.Connection) -> None:
        if not await self._table_exists(conn, "admin_agent_specs"):
            return
        await conn.execute(
            """
            INSERT INTO admin_registered_agents(name, agent_type, source, description, updated_at)
            SELECT name, agent_type, 'legacy-admin', '', NOW()
            FROM admin_agent_specs
            ON CONFLICT (name) DO NOTHING
            """
        )

    async def _table_exists(self, conn: asyncpg.Connection, table_name: str) -> bool:
        return bool(await conn.fetchval("SELECT to_regclass($1) IS NOT NULL", table_name))

    async def _column_exists(self, conn: asyncpg.Connection, table_name: str, column_name: str) -> bool:
        return bool(
            await conn.fetchval(
                """
                SELECT EXISTS(
                    SELECT 1
                    FROM information_schema.columns
                    WHERE table_name = $1 AND column_name = $2
                )
                """,
                table_name,
                column_name,
            )
        )


def _loads_json_dict(raw: str) -> dict[str, str]:
    try:
        data = json.loads(raw or "{}")
    except Exception:
        return {}
    if not isinstance(data, dict):
        return {}
    return {str(key): str(value) for key, value in data.items()}
