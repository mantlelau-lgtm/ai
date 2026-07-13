from __future__ import annotations

from core_service.config import cfg
from sirius.adapters.market_data import MockMarketDataAdapter
from sirius.adapters.tushare import TushareMarketDataAdapter
from sirius.data.models import AdapterResult


class MarketDataService:
    def __init__(self, adapter=None) -> None:
        self._adapter = adapter or self._default_adapter()

    def _default_adapter(self):
        if cfg.sirius_market_data_provider == "tushare":
            return TushareMarketDataAdapter()
        return MockMarketDataAdapter()

    async def quote(self, code: str, market: str) -> AdapterResult:
        return await self._adapter.quote(code, market)

    async def bars(self, code: str, market: str, periods: int = 30) -> AdapterResult:
        return await self._adapter.bars(code, market, periods)

    def provider_name(self) -> str:
        return getattr(self._adapter, "source", "unknown")
