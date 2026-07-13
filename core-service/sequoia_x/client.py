from __future__ import annotations

from typing import Any

import httpx

from core_service.config import cfg


class SequoiaXClient:
    def __init__(self, base_url: str | None = None, timeout: float | None = None) -> None:
        self._base_url = (base_url or cfg.sequoia_x_base_url).rstrip("/")
        self._timeout = float(timeout if timeout is not None else cfg.sequoia_x_request_timeout)

    @property
    def base_url(self) -> str:
        return self._base_url

    async def get_json(self, path: str, params: dict[str, Any] | None = None) -> Any:
        async with httpx.AsyncClient(timeout=self._timeout) as client:
            resp = await client.get(self._base_url + path, params=params)
            resp.raise_for_status()
            return resp.json()

    async def get_text(self, path: str, params: dict[str, Any] | None = None) -> str:
        async with httpx.AsyncClient(timeout=self._timeout) as client:
            resp = await client.get(self._base_url + path, params=params)
            resp.raise_for_status()
            return resp.text

    async def post_json(self, path: str, params: dict[str, Any] | None = None, json: Any | None = None) -> Any:
        async with httpx.AsyncClient(timeout=self._timeout) as client:
            resp = await client.post(self._base_url + path, params=params, json=json)
            resp.raise_for_status()
            return resp.json()
