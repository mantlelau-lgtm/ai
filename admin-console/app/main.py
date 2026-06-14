from __future__ import annotations

from contextlib import asynccontextmanager
from pathlib import Path
from typing import Any

import httpx
from fastapi import FastAPI, HTTPException, Request
from fastapi.responses import FileResponse
from fastapi.staticfiles import StaticFiles

from app.config import settings
from app.models import (
    BundlePayload,
    DiffResponse,
    LlmCatalog,
    LlmCredential,
    LlmModel,
    MessageRouteConfig,
    OverviewResponse,
    RegisteredAgent,
    RoutingConfig,
    SaveResponse,
    ServiceStatus,
    ValidationResponse,
)
from app.repository import ConfigRepository, PostgresConfigRepository
from app.services.crypto import SecretCryptor
from app.services.config_store import build_bundle_diff, validate_bundle

@asynccontextmanager
async def lifespan(app: FastAPI):
    if settings.disable_db_startup:
        yield
        return
    repository = PostgresConfigRepository(settings.database_url, SecretCryptor(settings.encryption_secret))
    await repository.open()
    await repository.migrate()
    app.state.repository = repository
    try:
        yield
    finally:
        await repository.close()


app = FastAPI(title="Admin Console API", lifespan=lifespan)


def repository_from_app(request: Request) -> ConfigRepository:
    repository = getattr(request.app.state, "repository", None)
    if repository is None:
        raise HTTPException(status_code=500, detail="repository not initialized")
    return repository


@app.get("/api/overview", response_model=OverviewResponse)
async def get_overview(request: Request) -> OverviewResponse:
    repository = repository_from_app(request)
    bundle = await repository.load_bundle()
    agents = await repository.load_registered_agents()
    return OverviewResponse(
        services=await _service_statuses(),
        database=await repository.database_meta(),
        tables=await repository.table_meta(),
        summary={
            "bot_count": len(bundle.bots.bots),
            "provider_count": len(bundle.llm.credentials),
            "model_count": len(bundle.llm.models),
            "agent_count": len(agents),
            "message_rule_count": len(bundle.message_routes.rules),
        },
    )


@app.get("/api/config/bundle")
async def get_bundle(request: Request) -> dict[str, Any]:
    repository = repository_from_app(request)
    bundle = await repository.load_bundle()
    database = await repository.database_meta()
    return {
        "storage": {
            "engine": database.engine,
            "database": database.database,
        },
        "data": bundle.model_dump(mode="json"),
    }


@app.post("/api/config/validate", response_model=ValidationResponse)
async def validate_config(payload: BundlePayload) -> ValidationResponse:
    return validate_bundle(payload)


@app.post("/api/config/diff", response_model=DiffResponse)
async def diff_config(payload: BundlePayload, request: Request) -> DiffResponse:
    repository = repository_from_app(request)
    current = await repository.load_bundle()
    validation = validate_bundle(payload)
    diff = build_bundle_diff(current, payload)
    return DiffResponse(validation=validation, **diff)


@app.post("/api/config/apply", response_model=SaveResponse)
async def apply_config(payload: BundlePayload, request: Request) -> SaveResponse:
    repository = repository_from_app(request)
    validation = validate_bundle(payload)
    if validation.errors:
        raise HTTPException(status_code=400, detail=validation.model_dump(mode="json"))

    updated_resources = await repository.save_bundle(payload)
    return SaveResponse(
        saved=True,
        validation=validation,
        updated_resources=updated_resources,
        needs_restart=["message-gateway", "llm-gateway", "core-service"],
    )


@app.put("/api/llm/models/{model_id}")
async def upsert_llm_model(model_id: str, payload: dict[str, Any], request: Request) -> dict[str, Any]:
    repository = repository_from_app(request)
    model = LlmModel(**payload)
    _validate_model_card(model)
    try:
        saved = await repository.upsert_llm_model(model, original_model_id=model_id)
    except Exception as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    return {"model": saved.model_dump(mode="json")}


@app.post("/api/llm/models")
async def create_llm_model(payload: dict[str, Any], request: Request) -> dict[str, Any]:
    repository = repository_from_app(request)
    model = LlmModel(**payload)
    _validate_model_card(model)
    try:
        saved = await repository.upsert_llm_model(model)
    except Exception as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    return {"model": saved.model_dump(mode="json")}


@app.delete("/api/llm/models/{model_id}")
async def delete_llm_model(model_id: str, request: Request) -> dict[str, Any]:
    repository = repository_from_app(request)
    try:
        deleted = await repository.delete_llm_model(model_id)
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    if not deleted:
        raise HTTPException(status_code=404, detail=f"model_id {model_id} 不存在")
    return {"deleted": True, "model_id": model_id}


@app.put("/api/llm/keys/{key_name}")
async def upsert_llm_key(key_name: str, payload: dict[str, Any], request: Request) -> dict[str, Any]:
    repository = repository_from_app(request)
    credential = LlmCredential(**payload)
    _validate_credential_card(credential)
    try:
        saved = await repository.upsert_llm_credential(credential, original_key_name=key_name)
    except Exception as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    return {"credential": saved.model_dump(mode="json")}


@app.post("/api/llm/keys")
async def create_llm_key(payload: dict[str, Any], request: Request) -> dict[str, Any]:
    repository = repository_from_app(request)
    credential = LlmCredential(**payload)
    _validate_credential_card(credential)
    if not (await _is_key_name_available(repository, credential.key_name)):
        raise HTTPException(status_code=400, detail=f"密钥名称已存在: {credential.key_name}")
    try:
        saved = await repository.upsert_llm_credential(credential)
    except Exception as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    return {"credential": saved.model_dump(mode="json")}


@app.delete("/api/llm/keys/{key_name}")
async def delete_llm_key(key_name: str, request: Request) -> dict[str, Any]:
    repository = repository_from_app(request)
    deleted = await repository.delete_llm_credential(key_name)
    if not deleted:
        raise HTTPException(status_code=404, detail=f"key_name {key_name} 不存在")
    return {"deleted": True, "key_name": key_name}


@app.get("/api/runtime/llm-gateway/catalog", response_model=LlmCatalog)
async def runtime_llm_catalog(request: Request) -> LlmCatalog:
    repository = repository_from_app(request)
    return await repository.load_runtime_catalog()


@app.get("/api/runtime/message-gateway/bots")
async def runtime_message_gateway_bots(request: Request) -> dict[str, Any]:
    repository = repository_from_app(request)
    bundle = await repository.load_bundle()
    return bundle.bots.model_dump(mode="json")


@app.get("/api/runtime/message-gateway/routes", response_model=MessageRouteConfig)
async def runtime_message_gateway_routes(request: Request) -> MessageRouteConfig:
    repository = repository_from_app(request)
    bundle = await repository.load_bundle()
    return bundle.message_routes


@app.get("/api/runtime/core-service/routing", response_model=RoutingConfig)
async def runtime_core_service_routing(request: Request) -> RoutingConfig:
    repository = repository_from_app(request)
    return await repository.load_runtime_routing()


@app.get("/api/agents")
async def list_agents(request: Request) -> dict[str, Any]:
    repository = repository_from_app(request)
    agents = await repository.load_registered_agents()
    return {"agents": [item.model_dump(mode="json") for item in agents]}


@app.post("/api/agents/register")
async def register_agents(payload: dict[str, Any], request: Request) -> dict[str, Any]:
    repository = repository_from_app(request)
    items = payload.get("agents") or []
    agents = [RegisteredAgent(**item) for item in items if isinstance(item, dict)]
    count = await repository.register_agents(agents)
    return {"registered": count}


@app.put("/api/agents/{name}/key")
async def update_agent_key(name: str, payload: dict[str, Any], request: Request) -> dict[str, Any]:
    repository = repository_from_app(request)
    raw = payload.get("key_name")
    if raw is None:
        raw = payload.get("model_name")  # 兼容旧前端字段
    key_name = (raw or "").strip()
    if key_name:
        valid_keys = await repository.list_credential_key_names()
        if key_name not in valid_keys:
            raise HTTPException(status_code=400, detail=f"key_name {key_name} 不存在于密钥列表")
    updated = await repository.update_agent_key(name, key_name)
    if not updated:
        raise HTTPException(status_code=404, detail=f"agent {name} not found")
    return {"agent": updated.model_dump(mode="json")}


# 兼容旧路径，转发到 /api/agents/{name}/key
@app.put("/api/agents/{name}/model")
async def update_agent_model_legacy(name: str, payload: dict[str, Any], request: Request) -> dict[str, Any]:
    return await update_agent_key(name, payload, request)


@app.post("/api/llm/keys/set-default")
async def set_default_key(payload: dict[str, Any], request: Request) -> dict[str, Any]:
    repository = repository_from_app(request)
    key_name = (payload.get("key_name") or "").strip()
    if not key_name:
        raise HTTPException(status_code=400, detail="key_name 不能为空")
    result = await repository.set_default_credential(key_name)
    if not result:
        raise HTTPException(status_code=404, detail=f"key_name {key_name} 不存在")
    return {"key_name": result, "is_default": True}


@app.get("/api/llm/keys/check-name")
async def check_key_name(name: str, request: Request) -> dict[str, Any]:
    repository = repository_from_app(request)
    available = await _is_key_name_available(repository, name)
    return {"name": name, "available": available}


async def _is_key_name_available(repository: ConfigRepository, name: str) -> bool:
    target = name.strip().lower()
    if not target:
        return False
    bundle = await repository.load_bundle()
    return not any(item.key_name.strip().lower() == target for item in bundle.llm.credentials)


def _validate_model_card(model: LlmModel) -> None:
    if not model.name.strip():
        raise HTTPException(status_code=400, detail="模型名称不能为空")
    if not model.model_id.strip():
        raise HTTPException(status_code=400, detail="模型ID不能为空")
    if not model.provider.strip():
        raise HTTPException(status_code=400, detail="厂商名称不能为空")
    if model.prompt_cost_per_1k_tokens < 0 or model.completion_cost_per_1k_tokens < 0 or model.unit_price < 0:
        raise HTTPException(status_code=400, detail="单价字段不能为负数")


def _validate_credential_card(credential: LlmCredential) -> None:
    if not credential.model_id.strip():
        raise HTTPException(status_code=400, detail="密钥必须绑定模型ID")
    if not credential.key_name.strip():
        raise HTTPException(status_code=400, detail="密钥名称不能为空")
    if not credential.key_value.strip():
        raise HTTPException(status_code=400, detail="密钥值不能为空")
    if not credential.base_url.strip():
        raise HTTPException(status_code=400, detail="base_url不能为空")
    if not credential.base_url.startswith(("http://", "https://")):
        raise HTTPException(status_code=400, detail="base_url 需要以 http:// 或 https:// 开头")
    if credential.call_type.strip() not in {"stream", "non_stream"}:
        raise HTTPException(status_code=400, detail="调用类型必须是 stream 或 non_stream")


async def _service_statuses() -> list[ServiceStatus]:
    services = [
        ("message-gateway", settings.message_gateway_health_url),
        ("core-service", settings.core_service_health_url),
        ("llm-gateway", settings.llm_gateway_health_url),
    ]
    results: list[ServiceStatus] = []
    async with httpx.AsyncClient(timeout=2.0) as client:
        for name, url in services:
            try:
                response = await client.get(url)
                detail = response.text.strip()
                status = "ok" if response.status_code < 400 else "error"
                results.append(ServiceStatus(name=name, url=url, status=status, detail=detail))
            except Exception as exc:  # noqa: BLE001
                results.append(ServiceStatus(name=name, url=url, status="offline", detail=str(exc)))
    return results


static_dir = Path(settings.static_dir)
assets_dir = static_dir / "assets"

if assets_dir.exists():
    app.mount("/assets", StaticFiles(directory=assets_dir), name="assets")


@app.get("/{full_path:path}")
async def serve_spa(full_path: str) -> FileResponse:
    if full_path.startswith("api/"):
        raise HTTPException(status_code=404, detail="not found")
    index_path = static_dir / "index.html"
    if not index_path.exists():
        raise HTTPException(status_code=404, detail="frontend build not found")
    return FileResponse(index_path)
