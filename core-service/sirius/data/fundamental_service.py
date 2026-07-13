from __future__ import annotations

from sirius.adapters.fundamental import MockFundamentalDataAdapter
from sirius.data.models import AdapterResult


class FundamentalDataService:
    def __init__(self, adapter: MockFundamentalDataAdapter | None = None) -> None:
        self._adapter = adapter or MockFundamentalDataAdapter()

    async def financials(self, code: str, market: str, period: str = "latest") -> AdapterResult:
        return await self._adapter.financials(code, market, period)

    async def announcements(self, code: str, market: str, keyword: str = "", limit: int = 10) -> AdapterResult:
        return await self._adapter.announcements(code, market, keyword, limit)
