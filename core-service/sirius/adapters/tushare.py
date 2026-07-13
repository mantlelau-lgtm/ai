from __future__ import annotations

from datetime import datetime, timezone

import httpx

from core_service.config import cfg
from sirius.adapters.base import adapter_error, adapter_ok
from sirius.data.models import AdapterResult, Bar, Quote, Symbol


class TushareMarketDataAdapter:
    source = "tushare-pro"

    def __init__(self, token: str | None = None, base_url: str | None = None) -> None:
        self._token = token if token is not None else cfg.sirius_tushare_token
        self._base_url = (base_url or cfg.sirius_tushare_base_url).rstrip("/")

    async def quote(self, code: str, market: str) -> AdapterResult:
        bars = await self.bars(code, market, periods=2)
        if not bars.ok:
            return bars
        items = bars.data
        latest = items[-1]
        prev = items[-2] if len(items) > 1 else latest
        quote = Quote(
            symbol=latest.symbol,
            price=latest.close,
            previous_close=prev.close,
            volume=latest.volume,
            turnover=latest.close * latest.volume,
            timestamp=latest.timestamp,
            source=self.source,
        )
        return adapter_ok(self.source, quote)

    async def bars(self, code: str, market: str, periods: int = 30) -> AdapterResult:
        if not self._token:
            return adapter_error(self.source, "SIRIUS_TUSHARE_TOKEN is not configured")
        ts_code = _to_ts_code(code, market)
        req = {
            "api_name": "daily",
            "token": self._token,
            "params": {"ts_code": ts_code},
            "fields": "ts_code,trade_date,open,high,low,close,vol",
        }
        try:
            async with httpx.AsyncClient(timeout=15) as client:
                resp = await client.post(self._base_url, json=req)
                resp.raise_for_status()
                body = resp.json()
        except Exception as exc:
            return adapter_error(self.source, str(exc))
        if body.get("code") not in (0, None):
            return adapter_error(self.source, str(body.get("msg") or body))
        data = body.get("data") or {}
        fields = data.get("fields") or []
        rows = data.get("items") or []
        symbol = Symbol(code=code, market=market if market in {"A股", "港股"} else "A股")  # type: ignore[arg-type]
        bars: list[Bar] = []
        for row in rows[: max(1, min(periods, 250))]:
            item = dict(zip(fields, row))
            trade_date = str(item.get("trade_date") or "")
            timestamp = datetime.strptime(trade_date, "%Y%m%d").replace(tzinfo=timezone.utc) if trade_date else datetime.now(timezone.utc)
            bars.append(
                Bar(
                    symbol=symbol,
                    timestamp=timestamp,
                    open=float(item.get("open") or 0),
                    high=float(item.get("high") or 0),
                    low=float(item.get("low") or 0),
                    close=float(item.get("close") or 0),
                    volume=int(float(item.get("vol") or 0) * 100),
                    source=self.source,
                )
            )
        bars.reverse()
        return adapter_ok(self.source, bars)


def _to_ts_code(code: str, market: str) -> str:
    normalized = (code or "").strip().upper()
    if "." in normalized:
        return normalized
    if market == "港股":
        return f"{normalized}.HK"
    if normalized.startswith(("6", "9")):
        return f"{normalized}.SH"
    return f"{normalized}.SZ"
