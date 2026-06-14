from __future__ import annotations

from typing import Any, AsyncIterator, Optional

import asyncio
import asyncpg
from fastapi import FastAPI, Header, HTTPException, Request
from fastapi.responses import PlainTextResponse, StreamingResponse

from core_service.agents import AgentRegistry
from core_service.config import cfg
from core_service.logging import logger
from core_service.metrics import metrics
from core_service.models import Envelope
from core_service.orchestrator import Orchestrator
from core_service.routing import RoutingManager, load_routing_config, load_routing_config_from_url
from core_service.store import Store


app = FastAPI()


@app.on_event("startup")
async def _startup() -> None:
    logger.info("core-service startup started")
    pool = await asyncpg.create_pool(dsn=cfg.database_url, min_size=1, max_size=10)
    store = Store(pool)
    await store.migrate()
    registry = AgentRegistry()
    routing_url = ""
    routing_cfg = None
    if cfg.admin_config_base_url:
        routing_url = f"{cfg.admin_config_base_url}{cfg.admin_core_routing_path}"
        routing_cfg = await load_routing_config_from_url(routing_url)
    if routing_cfg is None:
        routing_cfg = load_routing_config(cfg.routing_config_path)
    routing = RoutingManager(path=cfg.routing_config_path, source_url=routing_url, current=routing_cfg)
    app.state.pool = pool
    app.state.store = store
    app.state.agent_registry = registry
    app.state.routing = routing
    app.state.orchestrator = Orchestrator(store, registry, routing)
    app.state.routing_task = _start_routing_reload_task(routing)
    await _register_agents_with_admin(registry)


@app.on_event("shutdown")
async def _shutdown() -> None:
    task = getattr(app.state, "routing_task", None)
    if task is not None:
        task.cancel()
    pool: asyncpg.Pool = app.state.pool
    await pool.close()


@app.get("/healthz")
async def healthz() -> dict[str, str]:
    store: Store = app.state.store
    await store.ping()
    return {"status": "ok"}


@app.get("/metrics")
async def metrics_endpoint() -> PlainTextResponse:
    return PlainTextResponse(metrics.render(), media_type="text/plain; version=0.0.4")


@app.post("/v1/messages:stream")
async def messages_stream(
    request: Request,
    x_bot_id: Optional[str] = Header(default=None, alias="X-Bot-Id"),
    x_agent_name: Optional[str] = Header(default=None, alias="X-Agent-Name"),
    x_session_id: Optional[str] = Header(default=None, alias="X-Session-Id"),
    x_user_id: Optional[str] = Header(default=None, alias="X-User-Id"),
    x_open_id: Optional[str] = Header(default=None, alias="X-Open-Id"),
    x_chat_id: Optional[str] = Header(default=None, alias="X-Chat-Id"),
    x_trace_id: Optional[str] = Header(default=None, alias="X-Trace-Id"),
    x_request_id: Optional[str] = Header(default=None, alias="X-Request-Id"),
) -> StreamingResponse:
    body: dict[str, Any] = await request.json()
    raw_env = body.get("envelope") or {}
    envelope = Envelope.from_dict(raw_env)
    logger.info(
        "message stream request received",
        extra={
            "request_id": x_request_id or envelope.event_id,
            "event_id": envelope.event_id,
            "bot_id": x_bot_id or envelope.bot_id,
            "chat_id": x_chat_id or envelope.chat_id,
            "agent_header": x_agent_name or "",
        },
    )

    orchestrator: Orchestrator = app.state.orchestrator

    async def gen() -> AsyncIterator[bytes]:
        async for chunk in orchestrator.process_stream(
            envelope=envelope,
            header_bot_id=x_bot_id or "",
            header_agent_name=x_agent_name or "",
            header_session_id=x_session_id or "",
            header_user_id=x_user_id or "",
            header_open_id=x_open_id or "",
            header_chat_id=x_chat_id or "",
            header_trace_id=x_trace_id or "",
            header_request_id=x_request_id or "",
        ):
            yield chunk

    return StreamingResponse(gen(), media_type="text/event-stream")


@app.get("/admin/tasks/{task_id}")
async def admin_task(
    task_id: str,
    authorization: Optional[str] = Header(default=None, alias="Authorization"),
    x_admin_token: Optional[str] = Header(default=None, alias="X-Admin-Token"),
) -> dict[str, Any]:
    _require_admin(authorization, x_admin_token)
    store: Store = app.state.store
    task = await store.get_task(task_id)
    if task is None:
        raise HTTPException(status_code=404, detail="task not found")
    return {
        "task_id": task.task_id,
        "request_id": task.request_id,
        "conversation_id": task.conversation_id,
        "status": task.status,
        "error_message": task.error_message,
        "created_at": task.created_at,
        "updated_at": task.updated_at,
        "completed_at": task.completed_at,
    }


@app.get("/admin/conversations/{conversation_id}/messages")
async def admin_conversation_messages(
    conversation_id: str,
    limit: int = 50,
    authorization: Optional[str] = Header(default=None, alias="Authorization"),
    x_admin_token: Optional[str] = Header(default=None, alias="X-Admin-Token"),
) -> dict[str, Any]:
    _require_admin(authorization, x_admin_token)
    store: Store = app.state.store
    items = await store.list_conversation_messages(conversation_id, limit)
    return {"data": items}


def _require_admin(authorization: Optional[str], x_admin_token: Optional[str]) -> None:
    if not cfg.admin_token:
        return
    if x_admin_token and x_admin_token == cfg.admin_token:
        return
    if authorization and authorization == f"Bearer {cfg.admin_token}":
        return
    metrics.inc_failed()
    raise HTTPException(status_code=401, detail="unauthorized")


def _start_routing_reload_task(manager: RoutingManager) -> Optional[asyncio.Task]:
    if not cfg.routing_config_path.strip() and not cfg.admin_config_base_url.strip():
        return None
    if cfg.routing_reload_interval_seconds <= 0:
        return None

    async def _loop() -> None:
        while True:
            if manager.source_url:
                await manager.reload_from_remote_if_changed()
            else:
                manager.reload_if_changed()
            await asyncio.sleep(cfg.routing_reload_interval_seconds)

    return asyncio.create_task(_loop())


async def _register_agents_with_admin(registry: AgentRegistry) -> None:
    if not cfg.admin_config_base_url:
        return
    import json
    import aiohttp

    url = f"{cfg.admin_config_base_url}{cfg.admin_agents_register_path}"
    agents = []
    for name in registry.names():
        agent = registry.get(name)
        agents.append({
            "name": name,
            "type": agent.__class__.__name__.replace("Agent", "").lower(),
            "source": "core-service",
            "description": f"Built-in agent: {name}",
        })
    try:
        async with aiohttp.ClientSession() as session:
            async with session.post(
                url,
                json={"agents": agents},
                timeout=aiohttp.ClientTimeout(total=5),
            ) as response:
                if response.status >= 400:
                    body = await response.text()
                    logger.error("agent registration failed", extra={"status": response.status, "body": body})
                else:
                    logger.info("agents registered with admin", extra={"count": len(agents)})
    except Exception as exc:
        logger.exception("agent registration error", exc_info=exc)
