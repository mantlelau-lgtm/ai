import os
import tempfile
import unittest
from pathlib import Path

os.environ.setdefault("DATABASE_URL", "postgres://test:test@localhost:5432/test")

from core_service.agent_tools import ToolCall, default_tool_registry
from core_service.agents import AgentContext, GeneralAgent
from core_service.models import Envelope


def make_ctx(user_input: str) -> AgentContext:
    return AgentContext(
        conversation_id="conv-1",
        agent_name="general",
        bot_id="bot-1",
        user_id="user-1",
        open_id="open-1",
        chat_id="chat-1",
        trace_id="trace-1",
        request_id="req-1",
        user_input=user_input,
        history=[{"role": "user", "content": user_input}],
        envelope=Envelope(event_id="event-1", text=user_input),
        llm_key_name="key-1",
    )


class FakeLLM:
    def __init__(self) -> None:
        self.chat_once_calls = []
        self.stream_messages = []

    async def chat_once(self, **kwargs):
        self.chat_once_calls.append(kwargs)
        return {
            "choices": [
                {
                    "message": {
                        "role": "assistant",
                        "content": "",
                        "tool_calls": [
                            {
                                "id": "call-1",
                                "type": "function",
                                "function": {"name": "context.info", "arguments": "{}"},
                            }
                        ],
                    }
                }
            ],
            "usage": {"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2},
        }

    async def stream_chat_events(self, model, messages, headers):
        self.stream_messages = messages
        yield "final answer", None


class AgentToolsTest(unittest.IsolatedAsyncioTestCase):
    async def test_context_tool_returns_current_context(self) -> None:
        registry = default_tool_registry()
        result = await registry.run(ToolCall(name="context.info"), make_ctx("hello"))

        self.assertTrue(result.ok)
        self.assertEqual(result.data["conversation_id"], "conv-1")
        self.assertEqual(result.data["bot_id"], "bot-1")
        self.assertEqual(result.data["request_id"], "req-1")

    async def test_file_and_document_tools_are_workspace_scoped(self) -> None:
        previous = os.environ.get("AGENT_TOOLS_WORKSPACE_ROOT")
        try:
            with tempfile.TemporaryDirectory() as tmp:
                os.environ["AGENT_TOOLS_WORKSPACE_ROOT"] = tmp
                root = Path(tmp)
                (root / "docs").mkdir()
                (root / "docs" / "guide.md").write_text("hello docs", encoding="utf-8")
                registry = default_tool_registry()

                read_result = await registry.run(ToolCall(name="fs.read", arguments={"path": "docs/guide.md"}), make_ctx("read"))
                self.assertTrue(read_result.ok)
                self.assertEqual(read_result.content, "hello docs")

                find_result = await registry.run(ToolCall(name="doc.find", arguments={"keyword": "hello"}), make_ctx("find"))
                self.assertTrue(find_result.ok)
                self.assertIn("docs/guide.md", find_result.data["results"])

                blocked = await registry.run(ToolCall(name="fs.read", arguments={"path": "/etc/passwd"}), make_ctx("read"))
                self.assertFalse(blocked.ok)
                self.assertIn("outside workspace", blocked.error)
        finally:
            if previous is None:
                os.environ.pop("AGENT_TOOLS_WORKSPACE_ROOT", None)
            else:
                os.environ["AGENT_TOOLS_WORKSPACE_ROOT"] = previous

    async def test_system_tools_return_basic_information(self) -> None:
        registry = default_tool_registry()
        info = await registry.run(ToolCall(name="system.info"), make_ctx("system"))
        resources = await registry.run(ToolCall(name="system.resources"), make_ctx("system"))

        self.assertTrue(info.ok)
        self.assertIn("python", info.data)
        self.assertTrue(resources.ok)
        self.assertIn("cpu_count", resources.data)

    async def test_general_agent_runs_explicit_tool_call(self) -> None:
        agent = GeneralAgent(default_tool_registry())
        chunks = []
        async for text, usage in agent.stream_reply(make_ctx("/tool context.info")):
            chunks.append(text)

        output = "".join(chunks)
        self.assertIn("tool context.info ok", output)
        self.assertIn("conv-1", output)

    async def test_general_agent_reports_missing_tool(self) -> None:
        agent = GeneralAgent(default_tool_registry())
        chunks = []
        async for text, usage in agent.stream_reply(make_ctx("tool:missing.tool")):
            chunks.append(text)

        self.assertIn("tool missing.tool error", "".join(chunks))

    async def test_general_agent_runs_llm_tool_call_then_final_stream(self) -> None:
        fake_llm = FakeLLM()
        agent = GeneralAgent(default_tool_registry(), llm=fake_llm)

        chunks = []
        async for text, usage in agent.stream_reply(make_ctx("what is my context?")):
            chunks.append(text)

        self.assertEqual("".join(chunks), "final answer")
        self.assertTrue(fake_llm.chat_once_calls[0]["tools"])
        self.assertEqual(fake_llm.stream_messages[-1]["role"], "tool")
        self.assertEqual(fake_llm.stream_messages[-1]["tool_call_id"], "call-1")
        self.assertIn("conv-1", fake_llm.stream_messages[-1]["content"])


if __name__ == "__main__":
    unittest.main()
