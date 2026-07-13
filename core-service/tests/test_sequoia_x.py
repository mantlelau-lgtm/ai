import os
import unittest

os.environ.setdefault("DATABASE_URL", "postgres://test:test@localhost:5432/test")

import httpx

from core_service.agent_tools import ToolCall
from sequoia_x.client import SequoiaXClient
from sequoia_x.tools import sequoia_x_tool_registry


def make_registry(handler):
    transport = httpx.MockTransport(handler)

    class _Client(SequoiaXClient):
        def __init__(self) -> None:
            super().__init__(base_url="http://test")

        async def get_json(self, path, params=None):
            async with httpx.AsyncClient(transport=transport, base_url=self.base_url) as client:
                resp = await client.get(path, params=params)
                resp.raise_for_status()
                return resp.json()

        async def get_text(self, path, params=None):
            async with httpx.AsyncClient(transport=transport, base_url=self.base_url) as client:
                resp = await client.get(path, params=params)
                resp.raise_for_status()
                return resp.text

        async def post_json(self, path, params=None, json=None):
            async with httpx.AsyncClient(transport=transport, base_url=self.base_url) as client:
                resp = await client.post(path, params=params, json=json)
                resp.raise_for_status()
                return resp.json()

    import sequoia_x.tools as tools_module

    tools_module.SequoiaXClient = _Client  # type: ignore[assignment]
    return sequoia_x_tool_registry()


class SequoiaXToolsTest(unittest.IsolatedAsyncioTestCase):
    async def test_repositories_and_strategy_run_paths(self) -> None:
        seen: list[tuple[str, str, dict]] = []

        def handler(request: httpx.Request) -> httpx.Response:
            seen.append((request.method, request.url.path, dict(request.url.params)))
            if request.url.path == "/api/repositories":
                return httpx.Response(200, json=[{"id": "sequoia-x"}])
            if request.url.path.endswith("/strategies/turtle/run"):
                return httpx.Response(200, json={"strategy": "turtle", "count": 3, "symbols": ["000001"]})
            if request.url.path.endswith("/symbols"):
                return httpx.Response(200, json=["000001", "000002"])
            if request.url.path.endswith("/logs"):
                return httpx.Response(200, text="line1\nline2")
            return httpx.Response(404, text="not found")

        registry = make_registry(handler)

        repos = await registry.run(ToolCall(name="sx.repositories"), None)
        run = await registry.run(ToolCall(name="sx.strategy_run", arguments={"strategy": "turtle"}), None)
        symbols = await registry.run(ToolCall(name="sx.symbols", arguments={"query": "300", "limit": 10}), None)
        logs = await registry.run(ToolCall(name="sx.logs", arguments={"lines": 50}), None)

        self.assertTrue(repos.ok)
        self.assertEqual(repos.data["data"], [{"id": "sequoia-x"}])
        self.assertTrue(run.ok)
        self.assertEqual(run.data["data"]["strategy"], "turtle")
        self.assertTrue(symbols.ok)
        self.assertEqual(symbols.data["data"], ["000001", "000002"])
        self.assertTrue(logs.ok)
        self.assertIn("line1", logs.data["data"]["text"])

        paths = [(method, path) for method, path, _ in seen]
        self.assertIn(("GET", "/api/repositories"), paths)
        self.assertIn(("POST", "/api/repositories/sequoia-x/strategies/turtle/run"), paths)

    async def test_strategy_requires_strategy_argument(self) -> None:
        registry = make_registry(lambda request: httpx.Response(200, json={}))
        result = await registry.run(ToolCall(name="sx.strategy_run"), None)
        self.assertFalse(result.ok)
        self.assertIn("strategy is required", result.error)


if __name__ == "__main__":
    unittest.main()
