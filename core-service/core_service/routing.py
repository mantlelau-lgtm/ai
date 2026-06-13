from __future__ import annotations

import json
import os
from dataclasses import dataclass
from typing import Any, Optional

import httpx


@dataclass(frozen=True)
class AgentSpec:
    name: str
    type: str


@dataclass(frozen=True)
class RoutingConfig:
    default_agent: str
    bot_to_agent: dict[str, str]
    agents: dict[str, AgentSpec]

    def lookup_agent_name(self, bot_id: str) -> str:
        key = (bot_id or "").strip()
        if not key:
            return ""
        return (self.bot_to_agent.get(key) or "").strip()

    def resolve_agent_name(self, bot_id: str) -> str:
        key = (bot_id or "").strip()
        if key:
            v = (self.bot_to_agent.get(key) or "").strip()
            if v:
                return v
        return self.default_agent


def load_routing_config(path: str) -> Optional[RoutingConfig]:
    if not (path or "").strip():
        return None

    try:
        raw = _load_json_file(path)
    except Exception:
        return None
    return parse_routing_config(raw)


async def load_routing_config_from_url(url: str) -> Optional[RoutingConfig]:
    if not (url or "").strip():
        return None
    try:
        async with httpx.AsyncClient(timeout=5.0) as client:
            response = await client.get(url)
            response.raise_for_status()
            raw = response.json()
    except Exception:
        return None
    if not isinstance(raw, dict):
        return None
    return parse_routing_config(raw)


def parse_routing_config(raw: dict[str, Any]) -> RoutingConfig:
    default_agent = str(raw.get("default_agent") or "general").strip().lower() or "general"
    bot_to_agent: dict[str, str] = {}
    for item in raw.get("bots") or []:
        if not isinstance(item, dict):
            continue
        bot_id = str(item.get("bot_id") or "").strip()
        agent_name = str(item.get("agent_name") or "").strip().lower()
        if bot_id and agent_name:
            bot_to_agent[bot_id] = agent_name

    agents: dict[str, AgentSpec] = {}
    for item in raw.get("agents") or []:
        if not isinstance(item, dict):
            continue
        name = str(item.get("name") or "").strip().lower()
        if not name:
            continue
        typ = str(item.get("type") or "").strip() or "custom"
        agents[name] = AgentSpec(name=name, type=typ)

    if default_agent not in agents:
        agents[default_agent] = AgentSpec(name=default_agent, type="general")

    return RoutingConfig(default_agent=default_agent, bot_to_agent=bot_to_agent, agents=agents)


@dataclass
class RoutingManager:
    path: str
    source_url: str
    current: Optional[RoutingConfig]
    mtime_ns: int = 0
    signature: str = ""

    def __post_init__(self) -> None:
        self.mtime_ns = _get_mtime_ns(self.path)
        self.signature = _routing_signature(self.current)

    def reload_if_changed(self) -> bool:
        mtime_ns = _get_mtime_ns(self.path)
        if mtime_ns == 0:
            return False
        if self.mtime_ns == mtime_ns:
            return False
        cfg = load_routing_config(self.path)
        if cfg is None:
            return False
        self.current = cfg
        self.mtime_ns = mtime_ns
        self.signature = _routing_signature(cfg)
        return True

    async def reload_from_remote_if_changed(self) -> bool:
        cfg = await load_routing_config_from_url(self.source_url)
        if cfg is None:
            return False
        signature = _routing_signature(cfg)
        if self.signature == signature:
            return False
        self.current = cfg
        self.signature = signature
        return True


def _get_mtime_ns(path: str) -> int:
    if not (path or "").strip():
        return 0
    try:
        st = os.stat(path)
    except Exception:
        return 0
    return int(getattr(st, "st_mtime_ns", int(st.st_mtime * 1_000_000_000)))


def _load_json_file(path: str) -> dict[str, Any]:
    with open(path, "rb") as f:
        raw = f.read()
    obj = json.loads(raw)
    if not isinstance(obj, dict):
        raise ValueError("routing config must be a json object")
    return obj


def _routing_signature(config: Optional[RoutingConfig]) -> str:
    if config is None:
        return ""
    payload = {
        "default_agent": config.default_agent,
        "bots": [{"bot_id": bot_id, "agent_name": agent_name} for bot_id, agent_name in sorted(config.bot_to_agent.items())],
        "agents": [{"name": agent.name, "type": agent.type} for agent in sorted(config.agents.values(), key=lambda item: item.name)],
    }
    return json.dumps(payload, ensure_ascii=False, sort_keys=True)
