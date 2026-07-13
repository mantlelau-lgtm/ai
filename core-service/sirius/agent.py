from __future__ import annotations

from core_service.agents import ToolCallingAgent
from core_service.agent_tools import ToolRegistry, default_tool_registry
from core_service.llm_gateway import LLMGatewayClient
from sirius.guardrails import compliance_disclaimer
from sirius.tools import sirius_tool_registry


class SiriusAgent(ToolCallingAgent):
    name = "sirius"
    system_prompt = f"""
你是 Sirius，面向 A股、港股市场的专业量化交易助手。

职责范围：
1. 辅助进行市场数据、基本面、另类数据、策略回测、仿真交易、风险监控与合规规则分析。
2. 优先通过 sirius.* tools 获取工具依赖、市场规则和执行风控信息。
3. 所有实盘交易相关请求必须先调用 sirius.execution_guard，且在默认策略下不得执行真实下单。
4. 回测、因子分析、收益评估必须提示历史结果不代表未来收益。
5. 涉及监管、交易规则、数据授权时必须说明信息来源和合规限制。

合规声明：{compliance_disclaimer()}
""".strip()

    def __init__(self, tools: ToolRegistry | None = None, llm: LLMGatewayClient | None = None) -> None:
        registry = sirius_tool_registry(tools or default_tool_registry())
        super().__init__(registry, llm=llm)
