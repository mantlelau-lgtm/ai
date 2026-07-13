import os
import unittest

os.environ.setdefault("DATABASE_URL", "postgres://test:test@localhost:5432/test")

from core_service.agent_tools import ToolCall
from core_service.agents import AgentRegistry
from sirius.catalog import dependency_catalog
from sirius.tools import sirius_tool_registry


class SiriusTest(unittest.IsolatedAsyncioTestCase):
    def test_registry_discovers_sirius_agent(self) -> None:
        registry = AgentRegistry()

        self.assertIn("sirius", registry.names())
        self.assertEqual(registry.get("sirius").name, "sirius")

    def test_dependency_catalog_contains_execution_guarded_tools(self) -> None:
        names = {item["name"] for item in dependency_catalog()}

        self.assertIn("中信证券CTP接口", names)
        self.assertIn("辉立证券港股交易API", names)
        self.assertIn("恒生UMP风控系统API", names)

    async def test_sirius_execution_guard_blocks_live_trading_by_default(self) -> None:
        registry = sirius_tool_registry()
        result = await registry.run(
            ToolCall(name="sirius.execution_guard", arguments={"market": "A股", "action": "order_submit"}),
            None,
        )

        self.assertTrue(result.ok)
        self.assertFalse(result.data["allowed"])
        self.assertIn("默认", result.data["disclaimer"])

    async def test_sirius_market_rules_returns_a_and_hk_rules(self) -> None:
        registry = sirius_tool_registry()
        result = await registry.run(ToolCall(name="sirius.market_rules"), None)

        self.assertTrue(result.ok)
        markets = {item["market"] for item in result.data["rules"]}
        self.assertEqual(markets, {"A股", "港股"})

    async def test_sirius_quote_signal_backtest_risk_simulation_pipeline(self) -> None:
        registry = sirius_tool_registry()
        args = {"code": "000001", "market": "A股", "periods": 30, "quantity": 100}

        quote = await registry.run(ToolCall(name="sirius.quote", arguments=args), None)
        signal = await registry.run(ToolCall(name="sirius.signal_generate", arguments=args), None)
        backtest = await registry.run(ToolCall(name="sirius.backtest_run", arguments=args), None)
        risk = await registry.run(ToolCall(name="sirius.risk_evaluate", arguments=args), None)
        order = await registry.run(ToolCall(name="sirius.sim_order_submit", arguments=args), None)
        fundamental = await registry.run(ToolCall(name="sirius.fundamental", arguments=args), None)
        announcements = await registry.run(ToolCall(name="sirius.announcements", arguments=args), None)
        audit = await registry.run(ToolCall(name="sirius.audit_events", arguments={"limit": 10}), None)

        self.assertTrue(quote.ok)
        self.assertEqual(quote.data["source"], "mock-market-data")
        self.assertTrue(signal.ok)
        self.assertIn(signal.data["signal"]["action"], {"buy", "sell", "hold"})
        self.assertTrue(backtest.ok)
        self.assertIn("total_return", backtest.data["report"])
        self.assertTrue(risk.ok)
        self.assertIn(risk.data["risk_decision"]["status"], {"allow", "block", "review"})
        self.assertTrue(order.ok)
        self.assertIn("simulated_order", order.data)
        self.assertTrue(fundamental.ok)
        self.assertEqual(fundamental.data["source"], "mock-fundamental-data")
        self.assertTrue(announcements.ok)
        self.assertGreaterEqual(len(announcements.data["data"]), 1)
        self.assertTrue(audit.ok)
        self.assertGreaterEqual(len(audit.data["events"]), 4)


if __name__ == "__main__":
    unittest.main()
