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
        if not bot.agent_name.strip():
            add("bots", "warnings", f"{label} 未绑定 agent_name。")

    model_names: set[str] = set()
    model_ids: set[str] = set()
    enabled_models = 0
    for index, model in enumerate(bundle.llm.models, start=1):
        label = f"Model #{index}"
        name = model.name.strip()
        model_id = model.model_id.strip()
        provider_name = model.provider.strip()
        upstream = model.upstream_model.strip() or model_id
        if not name:
            add("llm", "errors", f"{label} 缺少 模型名称。")
        if not model_id:
            add("llm", "errors", f"{label} 缺少 模型ID。")
        elif model_id in model_ids:
            add("llm", "errors", f"模型ID 重复: {model_id}。")
        else:
            model_ids.add(model_id)
        if not provider_name:
            add("llm", "errors", f"{label} 缺少 厂商名称。")
        if not upstream:
            add("llm", "errors", f"{label} 缺少 upstream_model。")
        if name:
            if name in model_names:
                add("llm", "errors", f"模型名称重复: {name}。")
            model_names.add(name)
        if model.enabled:
            enabled_models += 1
        if model.prompt_cost_per_1k_tokens < 0 or model.completion_cost_per_1k_tokens < 0:
            add("llm", "errors", f"{label} 的成本字段不能为负数。")
        if model.unit_price < 0:
            add("llm", "errors", f"{label} 的单价不能为负数。")

    if enabled_models == 0 and bundle.llm.models:
        add("llm", "warnings", "当前没有启用中的模型。")

    key_names: set[str] = set()
    default_credential_count = 0
    for index, credential in enumerate(bundle.llm.credentials, start=1):
        label = f"Key #{index}"
        key_name = credential.key_name.strip()
        if not key_name:
            add("llm", "errors", f"{label} 缺少 密钥名称。")
        elif key_name in key_names:
            add("llm", "errors", f"密钥名称重复: {key_name}。")
        else:
            key_names.add(key_name)
        if not credential.base_url.strip():
            add("llm", "errors", f"{label} 缺少 base_url。")
        elif not credential.base_url.startswith(("http://", "https://")):
            add("llm", "errors", f"{label} 的 base_url 需要以 http:// 或 https:// 开头。")
        if not credential.key_value.strip():
            add("llm", "errors", f"{label} 缺少 密钥值。")
        if credential.call_type.strip() not in {"stream", "non_stream"}:
            add("llm", "errors", f"{label} 的 调用类型 必须是 stream 或 non_stream。")
        model_id = credential.model_id.strip()
        if not model_id:
            add("llm", "errors", f"{label} 必须绑定 模型ID。")
        elif model_id not in model_ids:
            add("llm", "errors", f"{label} 引用了不存在的 模型ID: {model_id}。")
        if credential.is_default:
            default_credential_count += 1

    if default_credential_count == 0 and bundle.llm.credentials:
        add("llm", "warnings", "当前没有默认密钥，未显式指定 key 的 agent 请求将无法路由。")
    if default_credential_count > 1:
        add("llm", "errors", "默认密钥只能有一个。")

    if not bundle.routing.default_agent.strip():
        add("routing", "errors", "default_agent 不能为空。")

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
