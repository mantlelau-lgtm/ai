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
    MessageRouteConfig,
    OverviewResponse,
    RoutingConfig,
    SaveResponse,
    ServiceStatus,
    ValidationResponse,
)
from app.repository import ConfigRepository, PostgresConfigRepository
from app.services.config_store import build_bundle_diff, validate_bundle

@asynccontextmanager
async def lifespan(app: FastAPI):
    if settings.disable_db_startup:
        yield
        return
    repository = PostgresConfigRepository(settings.database_url)
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
    return OverviewResponse(
        services=await _service_statuses(),
        database=await repository.database_meta(),
        tables=await repository.table_meta(),
        summary={
            "bot_count": len(bundle.bots.bots),
            "provider_count": len(bundle.llm.providers),
            "model_count": len(bundle.llm.models),
            "route_count": len(bundle.routing.bots),
            "agent_count": len(bundle.routing.agents),
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


@app.get("/api/runtime/llm-gateway/catalog", response_model=LlmCatalog)
async def runtime_llm_catalog(request: Request) -> LlmCatalog:
    repository = repository_from_app(request)
    bundle = await repository.load_bundle()
    return bundle.llm


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
    bundle = await repository.load_bundle()
    return bundle.routing


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
