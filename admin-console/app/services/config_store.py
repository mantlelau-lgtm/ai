from __future__ import annotations

import difflib
import json
from typing import Iterable

from pydantic import BaseModel

from app.models import (
    BundlePayload,
    ValidationBucket,
    ValidationResponse,
)


def dump_json(model: BaseModel) -> str:
    return json.dumps(model.model_dump(mode="json"), ensure_ascii=False, indent=2) + "\n"


def build_diff(current_text: str, next_text: str, label: str) -> str:
    if current_text == next_text:
        return "无变更\n"

    lines = difflib.unified_diff(
        current_text.splitlines(),
        next_text.splitlines(),
        fromfile=f"{label}:current",
        tofile=f"{label}:next",
        lineterm="",
    )
    return "\n".join(lines) + "\n"


def validate_bundle(bundle: BundlePayload) -> ValidationResponse:
    sections = {
        "bots": ValidationBucket(),
        "llm": ValidationBucket(),
        "routing": ValidationBucket(),
        "message_routes": ValidationBucket(),
        "cross": ValidationBucket(),
    }

    def add(section: str, level: str, message: str) -> None:
        bucket = sections[section]
        getattr(bucket, level).append(message)

    bot_ids: set[str] = set()
    app_ids: set[str] = set()
    for index, bot in enumerate(bundle.bots.bots, start=1):
        label = f"Bot #{index}"
        bot_id = bot.bot_id.strip()
        app_id = bot.app_id.strip()
        app_secret = bot.app_secret.strip()
        if not bot_id:
            add("bots", "errors", f"{label} 缺少 bot_id。")
        if not app_id:
            add("bots", "errors", f"{label} 缺少 app_id。")
        if not app_secret:
            add("bots", "errors", f"{label} 缺少 app_secret。")
        if bot_id:
            if bot_id in bot_ids:
                add("bots", "errors", f"Bot 配置中存在重复的 bot_id: {bot_id}。")
            bot_ids.add(bot_id)
        if app_id:
            if app_id in app_ids:
                add("bots", "errors", f"Bot 配置中存在重复的 app_id: {app_id}。")
            app_ids.add(app_id)
        if bot.open_base_url and not bot.open_base_url.startswith(("http://", "https://")):
            add("bots", "errors", f"{label} 的 open_base_url 需要以 http:// 或 https:// 开头。")
        if not bot.open_base_url:
            add("bots", "warnings", f"{label} 未设置 open_base_url，将依赖服务默认值。")

    key_names: set[str] = set()
    for index, key in enumerate(bundle.llm.keys, start=1):
        label = f"Key #{index}"
        name = key.name.strip()
        value_env = key.value_env.strip()
        if not name:
            add("llm", "errors", f"{label} 缺少 name。")
        if not value_env:
            add("llm", "warnings", f"{label} 未设置 value_env。")
        if name:
            if name in key_names:
                add("llm", "errors", f"LLM keys 中存在重复名称: {name}。")
            key_names.add(name)
        if not key.value.strip() and not value_env:
            add("llm", "errors", f"{label} 需要至少配置 value 或 value_env。")

    provider_names: set[str] = set()
    default_provider_count = 0
    for index, provider in enumerate(bundle.llm.providers, start=1):
        label = f"Provider #{index}"
        name = provider.name.strip()
        if not name:
            add("llm", "errors", f"{label} 缺少 name。")
        if not provider.base_url.strip():
            add("llm", "errors", f"{label} 缺少 base_url。")
        if not provider.api_key_from.strip():
            if not provider.api_key.strip():
                add("llm", "errors", f"{label} 缺少 api_key 或 api_key_from。")
        elif provider.api_key_from.strip() not in key_names:
            add("llm", "errors", f"{label} 引用了不存在的 key: {provider.api_key_from.strip()}。")
        if name:
            if name in provider_names:
                add("llm", "errors", f"LLM providers 中存在重复名称: {name}。")
            provider_names.add(name)
        if provider.is_default:
            default_provider_count += 1
        if provider.base_url and not provider.base_url.startswith(("http://", "https://")):
            add("llm", "errors", f"{label} 的 base_url 需要以 http:// 或 https:// 开头。")

    if default_provider_count == 0 and bundle.llm.providers:
        add("llm", "warnings", "当前没有默认 provider，服务端会依赖其他回退逻辑。")
    if default_provider_count > 1:
        add("llm", "errors", "默认 provider 只能有一个。")

    model_names: set[str] = set()
    enabled_models = 0
    for index, model in enumerate(bundle.llm.models, start=1):
        label = f"Model #{index}"
        name = model.name.strip()
        provider_name = model.provider.strip()
        upstream = model.upstream_model.strip()
        if not name:
            add("llm", "errors", f"{label} 缺少 name。")
        if not provider_name:
            add("llm", "errors", f"{label} 缺少 provider。")
        elif provider_name not in provider_names:
            add("llm", "errors", f"{label} 引用了不存在的 provider: {provider_name}。")
        if not upstream:
            add("llm", "errors", f"{label} 缺少 upstream_model。")
        if name:
            if name in model_names:
                add("llm", "errors", f"LLM models 中存在重复名称: {name}。")
            model_names.add(name)
        if model.enabled:
            enabled_models += 1
        if model.prompt_cost_per_1k_tokens < 0 or model.completion_cost_per_1k_tokens < 0:
            add("llm", "errors", f"{label} 的成本字段不能为负数。")

    if enabled_models == 0 and bundle.llm.models:
        add("llm", "warnings", "当前没有启用中的模型。")

    route_bot_ids: set[str] = set()
    if not bundle.routing.default_agent.strip():
        add("routing", "errors", "default_agent 不能为空。")
    agent_names: set[str] = set()
    for index, agent in enumerate(bundle.routing.agents, start=1):
        label = f"Agent #{index}"
        name = agent.name.strip().lower()
        if not name:
            add("routing", "errors", f"{label} 缺少 name。")
            continue
        if name in agent_names:
            add("routing", "errors", f"routing.agents 中存在重复名称: {name}。")
        agent_names.add(name)
    for index, route in enumerate(bundle.routing.bots, start=1):
        label = f"Route #{index}"
        bot_id = route.bot_id.strip()
        agent_name = route.agent_name.strip().lower()
        if not bot_id:
            add("routing", "errors", f"{label} 缺少 bot_id。")
        if not agent_name:
            add("routing", "errors", f"{label} 缺少 agent_name。")
        elif agent_names and agent_name not in agent_names:
            add("routing", "warnings", f"{label} 引用了未声明的 agent: {agent_name}。")
        if bot_id:
            if bot_id in route_bot_ids:
                add("routing", "errors", f"路由配置中存在重复的 bot_id: {bot_id}。")
            route_bot_ids.add(bot_id)

    if bundle.routing.default_agent.strip().lower() and agent_names and bundle.routing.default_agent.strip().lower() not in agent_names:
        add("routing", "warnings", f"default_agent {bundle.routing.default_agent.strip()} 未在 agents 列表中声明。")

    rule_ids: set[str] = set()
    for index, rule in enumerate(bundle.message_routes.rules, start=1):
        label = f"Message route #{index}"
        rule_id = rule.id.strip()
        if not rule_id:
            add("message_routes", "errors", f"{label} 缺少 id。")
            continue
        if rule_id in rule_ids:
            add("message_routes", "errors", f"message_routes 中存在重复的 id: {rule_id}。")
        rule_ids.add(rule_id)

    for route_bot_id in sorted(route_bot_ids):
        if route_bot_id not in bot_ids:
            add("cross", "warnings", f"路由中的 bot_id {route_bot_id} 未在 Bot 配置中声明。")

    for bot_id in sorted(bot_ids):
        if bot_id not in route_bot_ids:
            add("cross", "warnings", f"Bot {bot_id} 尚未配置 agent 路由。")

    errors = _flatten(bucket.errors for bucket in sections.values())
    warnings = _flatten(bucket.warnings for bucket in sections.values())
    return ValidationResponse(errors=errors, warnings=warnings, sections=sections)


def _flatten(chunks: Iterable[Iterable[str]]) -> list[str]:
    merged: list[str] = []
    for chunk in chunks:
        merged.extend(chunk)
    return merged


def build_bundle_diff(
    current: BundlePayload,
    draft: BundlePayload,
) -> dict[str, str]:
    return {
        "bots_diff": build_diff(
            dump_json(current.bots),
            dump_json(draft.bots),
            "bots",
        ),
        "llm_diff": build_diff(
            dump_json(current.llm),
            dump_json(draft.llm),
            "llm",
        ),
        "routing_diff": build_diff(
            dump_json(current.routing),
            dump_json(draft.routing),
            "routing",
        ),
        "message_routes_diff": build_diff(
            dump_json(current.message_routes),
            dump_json(draft.message_routes),
            "message_routes",
        ),
    }
