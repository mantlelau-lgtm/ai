from __future__ import annotations

import importlib
import os
import unittest

from fastapi.testclient import TestClient

from app.models import (
    AdminLlmConfig,
    AdminRoutingConfig,
    AgentSpec,
    BotConfig,
    BotsFile,
    BundlePayload,
    DatabaseMeta,
    LlmCatalog,
    LlmCredential,
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


class FakeRepository:
    def __init__(self) -> None:
        self.bundle = BundlePayload(
            bots=BotsFile(bots=[]),
            llm=AdminLlmConfig(
                credentials=[
                    LlmCredential(
                        vendor_name="deepseek",
                        type="openai",
                        call_type="non_stream",
                        base_url="https://api.deepseek.com",
                        key_name="deepseek-main",
                        key_value="secret-value",
                        model_id="deepseek-v4-flash",
                        model_name="deepseek-v4-flash",
                        enabled=True,
                        is_default=True,
                    ),
                    LlmCredential(
                        vendor_name="deepseek",
                        type="openai",
                        call_type="stream",
                        base_url="https://api.deepseek.com",
                        key_name="deepseek-backup",
                        key_value="backup-value",
                        model_id="deepseek-v4-flash",
                        model_name="deepseek-v4-flash",
                        enabled=True,
                        is_default=False,
                    ),
                ],
                models=[
                    LlmModel(
                        name="deepseek-v4-flash",
                        model_id="deepseek-v4-flash",
                        provider="deepseek",
                        upstream_model="deepseek-v4-flash",
                        owned_by="deepseek",
                        prompt_cost_per_1k_tokens=0.001,
                        completion_cost_per_1k_tokens=0.002,
                        unit_price=0.0015,
                        enabled=True,
                    )
                ],
            ),
            routing=AdminRoutingConfig(default_agent="general"),
            message_routes=MessageRouteConfig(
                rules=[
                    MessageRouteRule(
                        id="help",
                        priority=10,
                        match=MessageRouteMatch(kind="message", text_prefix="/help"),
                        action=MessageRouteAction(reply_text="help text"),
                    )
                ]
            ),
        )
        self.agents = [
            RegisteredAgent(name="general", type="general", source="core-service", key_name="deepseek-main")
        ]

    async def load_bundle(self) -> BundlePayload:
        return self.bundle

    async def load_runtime_catalog(self) -> LlmCatalog:
        return LlmCatalog(
            keys=[],
            providers=[
                LlmProvider(
                    name=item.key_name,
                    type=item.type,
                    base_url=item.base_url,
                    api_key=item.key_value,
                    enabled=item.enabled,
                    is_default=item.is_default,
                )
                for item in self.bundle.llm.credentials
            ],
            models=[
                LlmModel(
                    name=cred.key_name,
                    model_id=model.model_id,
                    provider=cred.key_name,
                    upstream_model=model.upstream_model,
                    owned_by=model.owned_by,
                    prompt_cost_per_1k_tokens=model.prompt_cost_per_1k_tokens,
                    completion_cost_per_1k_tokens=model.completion_cost_per_1k_tokens,
                    unit_price=model.unit_price,
                    enabled=model.enabled,
                )
                for cred in self.bundle.llm.credentials
                for model in self.bundle.llm.models
                if model.model_id == cred.model_id
            ],
        )

    async def load_runtime_routing(self) -> RoutingConfig:
        return RoutingConfig(
            default_agent=self.bundle.routing.default_agent,
            bots=[],
            agents=[
                AgentSpec(name=item.name, type=item.type, key_name=item.key_name)
                for item in self.agents
            ],
        )

    async def load_registered_agents(self) -> list[RegisteredAgent]:
        return self.agents

    async def register_agents(self, agents: list[RegisteredAgent]) -> int:
        self.agents = agents
        return len(agents)

    async def update_agent_key(self, name: str, key_name: str) -> RegisteredAgent | None:
        normalized = (name or "").strip().lower()
        for index, agent in enumerate(self.agents):
            if agent.name == normalized:
                updated = agent.model_copy(update={"key_name": (key_name or "").strip()})
                self.agents[index] = updated
                return updated
        return None

    async def list_credential_key_names(self) -> list[str]:
        return [item.key_name for item in self.bundle.llm.credentials]

    async def set_default_credential(self, key_name: str) -> str | None:
        target = (key_name or "").strip()
        if not target:
            return None
        if not any(item.key_name == target for item in self.bundle.llm.credentials):
            return None
        self.bundle.llm.credentials = [
            item.model_copy(update={"is_default": item.key_name == target})
            for item in self.bundle.llm.credentials
        ]
        return target

    async def upsert_llm_model(self, model: LlmModel, original_model_id: str = "") -> LlmModel:
        target = original_model_id or model.model_id
        self.bundle.llm.models = [item for item in self.bundle.llm.models if item.model_id != target]
        self.bundle.llm.models.append(model)
        return model

    async def delete_llm_model(self, model_id: str) -> bool:
        before = len(self.bundle.llm.models)
        self.bundle.llm.models = [item for item in self.bundle.llm.models if item.model_id != model_id]
        return len(self.bundle.llm.models) != before

    async def upsert_llm_credential(self, credential: LlmCredential, original_key_name: str = "") -> LlmCredential:
        if not any(item.model_id == credential.model_id for item in self.bundle.llm.models):
            raise ValueError(f"模型ID不存在: {credential.model_id}")
        target = original_key_name or credential.key_name
        self.bundle.llm.credentials = [item for item in self.bundle.llm.credentials if item.key_name != target]
        self.bundle.llm.credentials.append(credential)
        return credential

    async def delete_llm_credential(self, key_name: str) -> bool:
        before = len(self.bundle.llm.credentials)
        self.bundle.llm.credentials = [item for item in self.bundle.llm.credentials if item.key_name != key_name]
        return len(self.bundle.llm.credentials) != before

    async def save_bundle(self, bundle: BundlePayload) -> list[str]:
        self.bundle = bundle
        return [
            "admin_bots",
            "admin_llm_credentials",
            "admin_llm_models",
            "admin_routing_settings",
            "admin_registered_agents",
            "admin_message_route_rules",
        ]

    async def table_meta(self) -> dict[str, TableMeta]:
        return {
            "admin_bots": TableMeta(name="admin_bots", rows=len(self.bundle.bots.bots), updated_at=None),
        }

    async def database_meta(self) -> DatabaseMeta:
        return DatabaseMeta(
            engine="postgresql",
            database="admin_console",
            status="ok",
            detail="database connected",
        )


class AdminApiTestCase(unittest.TestCase):
    def setUp(self) -> None:
        os.environ["ADMIN_CONSOLE_DISABLE_DB_STARTUP"] = "1"

        import app.config
        import app.main

        importlib.reload(app.config)
        main = importlib.reload(app.main)
        main.app.state.repository = FakeRepository()
        self.client = TestClient(main.app)

    def tearDown(self) -> None:
        for key in ("ADMIN_CONSOLE_DISABLE_DB_STARTUP",):
            os.environ.pop(key, None)

    def test_apply_config_persists_bundle(self) -> None:
        payload = {
            "bots": {
                "bots": [
                    {
                        "bot_id": "cli_bot_1",
                        "app_id": "cli_bot_1",
                        "app_secret": "secret-1",
                        "open_base_url": "https://open.feishu.cn/",
                        "agent_name": "general",
                    }
                ]
            },
            "llm": {
                "credentials": [
                    {
                        "vendor_name": "deepseek",
                        "type": "openai",
                        "call_type": "non_stream",
                        "base_url": "https://api.deepseek.com",
                        "key_name": "deepseek-main",
                        "key_value": "secret-value",
                        "model_id": "deepseek-v4-flash",
                        "model_name": "deepseek-v4-flash",
                        "metadata": {},
                        "enabled": True,
                        "is_default": True,
                    }
                ],
                "models": [
                    {
                        "name": "deepseek-v4-flash",
                        "model_id": "deepseek-v4-flash",
                        "provider": "deepseek",
                        "upstream_model": "deepseek-v4-flash",
                        "owned_by": "deepseek",
                        "prompt_cost_per_1k_tokens": 0.001,
                        "completion_cost_per_1k_tokens": 0.002,
                        "unit_price": 0.0015,
                        "enabled": True,
                    }
                ],
            },
            "routing": {"default_agent": "general"},
            "message_routes": {
                "rules": [
                    {
                        "id": "help",
                        "priority": 10,
                        "match": {"kind": "message", "text_prefix": "/help"},
                        "action": {"reply_text": "help text"},
                    }
                ]
            },
        }

        response = self.client.post("/api/config/apply", json=payload)
        self.assertEqual(response.status_code, 200)
        body = response.json()
        self.assertTrue(body["saved"])
        bundle = self.client.app.state.repository.bundle
        self.assertEqual(bundle.bots.bots[0].bot_id, "cli_bot_1")
        self.assertEqual(bundle.llm.models[0].unit_price, 0.0015)

    def test_overview_returns_database_metadata(self) -> None:
        response = self.client.get("/api/overview")
        self.assertEqual(response.status_code, 200)
        body = response.json()
        self.assertEqual(body["database"]["database"], "admin_console")
        self.assertIn("admin_bots", body["tables"])

    def test_runtime_endpoints_return_rendered_configs(self) -> None:
        response = self.client.get("/api/runtime/llm-gateway/catalog")
        self.assertEqual(response.status_code, 200)
        body = response.json()
        # 关键：provider.name == credential.key_name
        self.assertEqual(body["providers"][0]["name"], "deepseek-main")
        self.assertEqual(body["models"][0]["provider"], "deepseek-main")

        response = self.client.get("/api/runtime/message-gateway/bots")
        self.assertEqual(response.status_code, 200)
        self.assertIn("bots", response.json())

        response = self.client.get("/api/runtime/message-gateway/routes")
        self.assertEqual(response.status_code, 200)
        self.assertEqual(response.json()["rules"][0]["id"], "help")

        response = self.client.get("/api/runtime/core-service/routing")
        self.assertEqual(response.status_code, 200)
        body = response.json()
        self.assertEqual(body["agents"][0]["name"], "general")
        # 关键：routing 携带 agent.key_name，core-service 据此调用 llm-gw
        self.assertEqual(body["agents"][0]["key_name"], "deepseek-main")

    def test_agents_list_returns_registered(self) -> None:
        response = self.client.get("/api/agents")
        self.assertEqual(response.status_code, 200)
        data = response.json()
        self.assertEqual(data["agents"][0]["name"], "general")
        self.assertEqual(data["agents"][0]["source"], "core-service")
        self.assertEqual(data["agents"][0]["key_name"], "deepseek-main")

    def test_agents_register_upserts(self) -> None:
        payload = {
            "agents": [
                {"name": "general", "type": "general", "source": "core-service", "description": "desc"},
                {"name": "echo", "type": "echo", "source": "core-service", "description": "echo"},
            ]
        }
        response = self.client.post("/api/agents/register", json=payload)
        self.assertEqual(response.status_code, 200)
        self.assertEqual(response.json()["registered"], 2)

    def test_update_agent_key_persists(self) -> None:
        response = self.client.put("/api/agents/general/key", json={"key_name": "deepseek-main"})
        self.assertEqual(response.status_code, 200)
        self.assertEqual(response.json()["agent"]["key_name"], "deepseek-main")

    def test_update_agent_key_rejects_unknown_key(self) -> None:
        response = self.client.put("/api/agents/general/key", json={"key_name": "no-such-key"})
        self.assertEqual(response.status_code, 400)

    def test_update_agent_legacy_path_still_works(self) -> None:
        response = self.client.put("/api/agents/general/model", json={"model_name": "deepseek-main"})
        self.assertEqual(response.status_code, 200)
        self.assertEqual(response.json()["agent"]["key_name"], "deepseek-main")

    def test_update_agent_missing_returns_404(self) -> None:
        response = self.client.put("/api/agents/missing/key", json={"key_name": "deepseek-main"})
        self.assertEqual(response.status_code, 404)

    def test_validate_config_rejects_missing_provider_reference(self) -> None:
        payload = {
            "bots": {"bots": []},
            "llm": {
                "credentials": [
                    {
                        "vendor_name": "x",
                        "type": "openai",
                        "call_type": "non_stream",
                        "base_url": "https://api.test.com",
                        "key_name": "broken-key",
                        "key_value": "v",
                        "model_id": "missing-id",
                        "enabled": True,
                        "is_default": True,
                    }
                ],
                "models": [],
            },
            "routing": {"default_agent": "general"},
            "message_routes": {"rules": []},
        }

        response = self.client.post("/api/config/validate", json=payload)
        self.assertEqual(response.status_code, 200)
        body = response.json()
        self.assertTrue(any("missing-id" in err for err in body["errors"]))

    def test_validate_rejects_multiple_default_credentials(self) -> None:
        payload = {
            "bots": {"bots": []},
            "llm": {
                "credentials": [
                    {
                        "vendor_name": "deepseek",
                        "type": "openai",
                        "call_type": "non_stream",
                        "base_url": "https://api.deepseek.com",
                        "key_name": "k1",
                        "key_value": "v",
                        "model_id": "m1",
                        "enabled": True,
                        "is_default": True,
                    },
                    {
                        "vendor_name": "deepseek",
                        "type": "openai",
                        "call_type": "non_stream",
                        "base_url": "https://api.deepseek.com",
                        "key_name": "k2",
                        "key_value": "v",
                        "model_id": "m1",
                        "enabled": True,
                        "is_default": True,
                    },
                ],
                "models": [
                    {
                        "name": "m1",
                        "model_id": "m1",
                        "provider": "deepseek",
                        "upstream_model": "m1",
                        "enabled": True,
                    }
                ],
            },
            "routing": {"default_agent": "general"},
            "message_routes": {"rules": []},
        }
        response = self.client.post("/api/config/validate", json=payload)
        self.assertEqual(response.status_code, 200)
        body = response.json()
        self.assertTrue(any("默认密钥只能有一个" in err for err in body["errors"]))

    def test_check_key_name_uniqueness(self) -> None:
        response = self.client.get("/api/llm/keys/check-name", params={"name": "deepseek-main"})
        self.assertEqual(response.status_code, 200)
        self.assertFalse(response.json()["available"])

        response = self.client.get("/api/llm/keys/check-name", params={"name": "another-key"})
        self.assertEqual(response.status_code, 200)
        self.assertTrue(response.json()["available"])

    def test_model_card_save_does_not_validate_dirty_credentials(self) -> None:
        repo = self.client.app.state.repository
        repo.bundle.llm.credentials[0] = repo.bundle.llm.credentials[0].model_copy(update={"model_id": ""})
        payload = {
            "name": "card-model",
            "model_id": "card-model-id",
            "provider": "card-provider",
            "upstream_model": "card-upstream",
            "owned_by": "card-provider",
            "prompt_cost_per_1k_tokens": 0.01,
            "completion_cost_per_1k_tokens": 0.02,
            "unit_price": 0.015,
            "enabled": True,
        }
        response = self.client.post("/api/llm/models", json=payload)
        self.assertEqual(response.status_code, 200)
        self.assertEqual(response.json()["model"]["model_id"], "card-model-id")

    def test_credential_card_save_validates_own_model_id_only(self) -> None:
        repo = self.client.app.state.repository
        repo.bundle.llm.credentials[0] = repo.bundle.llm.credentials[0].model_copy(update={"model_id": ""})
        payload = {
            "vendor_name": "deepseek",
            "type": "openai",
            "call_type": "non_stream",
            "base_url": "https://api.deepseek.com",
            "key_name": "card-key",
            "key_value": "secret-value",
            "model_id": "deepseek-v4-flash",
            "model_name": "deepseek-v4-flash",
            "metadata": {},
            "enabled": True,
            "is_default": False,
        }
        response = self.client.post("/api/llm/keys", json=payload)
        self.assertEqual(response.status_code, 200)
        self.assertEqual(response.json()["credential"]["key_name"], "card-key")

    def test_set_default_key_swaps_default_flag(self) -> None:
        # 初始 deepseek-main 是默认；切换到 backup
        response = self.client.post(
            "/api/llm/keys/set-default", json={"key_name": "deepseek-backup"}
        )
        self.assertEqual(response.status_code, 200)
        self.assertEqual(response.json()["key_name"], "deepseek-backup")
        bundle = self.client.app.state.repository.bundle
        defaults = [item for item in bundle.llm.credentials if item.is_default]
        self.assertEqual(len(defaults), 1)
        self.assertEqual(defaults[0].key_name, "deepseek-backup")

    def test_set_default_key_rejects_unknown(self) -> None:
        response = self.client.post(
            "/api/llm/keys/set-default", json={"key_name": "ghost-key"}
        )
        self.assertEqual(response.status_code, 404)


if __name__ == "__main__":
    unittest.main()
