from __future__ import annotations

import importlib
import os
import unittest

from fastapi.testclient import TestClient

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


class FakeRepository:
    def __init__(self) -> None:
        self.bundle = BundlePayload(
            bots=BotsFile(bots=[]),
            llm=LlmCatalog(
                keys=[LlmKey(name="deepseek-main", value_env="DEEPSEEK_API_KEY")],
                providers=[
                    LlmProvider(
                        name="deepseek-default",
                        type="openai",
                        base_url="https://api.deepseek.com",
                        api_key_from="deepseek-main",
                        enabled=True,
                        is_default=True,
                    )
                ],
                models=[
                    LlmModel(
                        name="deepseek-v4-flash",
                        provider="deepseek-default",
                        upstream_model="deepseek-v4-flash",
                        owned_by="deepseek",
                        prompt_cost_per_1k_tokens=0.001,
                        completion_cost_per_1k_tokens=0.002,
                        enabled=True,
                    )
                ],
            ),
            routing=RoutingConfig(
                default_agent="general",
                bots=[],
                agents=[AgentSpec(name="general", type="general")],
            ),
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

    async def load_bundle(self) -> BundlePayload:
        return self.bundle

    async def save_bundle(self, bundle: BundlePayload) -> list[str]:
        self.bundle = bundle
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
        return {
            "admin_bots": TableMeta(name="admin_bots", rows=len(self.bundle.bots.bots), updated_at=None),
            "admin_routes": TableMeta(name="admin_routes", rows=len(self.bundle.routing.bots), updated_at=None),
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
                    }
                ]
            },
            "llm": {
                "keys": [{"name": "deepseek-main", "value_env": "DEEPSEEK_API_KEY"}],
                "providers": [
                    {
                        "name": "deepseek-default",
                        "type": "openai",
                        "base_url": "https://api.deepseek.com",
                        "api_key_from": "deepseek-main",
                        "enabled": True,
                        "is_default": True,
                    }
                ],
                "models": [
                    {
                        "name": "deepseek-v4-flash",
                        "provider": "deepseek-default",
                        "upstream_model": "deepseek-v4-flash",
                        "owned_by": "deepseek",
                        "prompt_cost_per_1k_tokens": 0.001,
                        "completion_cost_per_1k_tokens": 0.002,
                        "enabled": True,
                    }
                ],
            },
            "routing": {
                "default_agent": "general",
                "bots": [{"bot_id": "cli_bot_1", "agent_name": "general"}],
                "agents": [{"name": "general", "type": "general"}],
            },
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

    def test_overview_returns_database_metadata(self) -> None:
        response = self.client.get("/api/overview")
        self.assertEqual(response.status_code, 200)
        body = response.json()
        self.assertEqual(body["database"]["database"], "admin_console")
        self.assertIn("admin_bots", body["tables"])

    def test_runtime_endpoints_return_rendered_configs(self) -> None:
        response = self.client.get("/api/runtime/llm-gateway/catalog")
        self.assertEqual(response.status_code, 200)
        self.assertEqual(response.json()["providers"][0]["name"], "deepseek-default")

        response = self.client.get("/api/runtime/message-gateway/bots")
        self.assertEqual(response.status_code, 200)
        self.assertIn("bots", response.json())

        response = self.client.get("/api/runtime/message-gateway/routes")
        self.assertEqual(response.status_code, 200)
        self.assertEqual(response.json()["rules"][0]["id"], "help")

        response = self.client.get("/api/runtime/core-service/routing")
        self.assertEqual(response.status_code, 200)
        self.assertEqual(response.json()["agents"][0]["name"], "general")

    def test_validate_config_rejects_missing_provider_reference(self) -> None:
        payload = {
            "bots": {"bots": []},
            "llm": {
                "keys": [{"name": "deepseek-main", "value_env": "DEEPSEEK_API_KEY"}],
                "providers": [],
                "models": [
                    {
                        "name": "broken-model",
                        "provider": "missing-provider",
                        "upstream_model": "broken-model",
                        "enabled": True,
                    }
                ],
            },
            "routing": {"default_agent": "general", "bots": [], "agents": [{"name": "general", "type": "general"}]},
            "message_routes": {"rules": []},
        }

        response = self.client.post("/api/config/validate", json=payload)
        self.assertEqual(response.status_code, 200)
        body = response.json()
        self.assertIn("Model #1 引用了不存在的 provider: missing-provider。", body["errors"])


if __name__ == "__main__":
    unittest.main()
