from __future__ import annotations

from datetime import datetime, timedelta, timezone

from sirius.adapters.base import adapter_error, adapter_ok
from sirius.data.models import AdapterResult, Bar, Quote, Symbol


class MockMarketDataAdapter:
    source = "mock-market-data"

    async def quote(self, code: str, market: str) -> AdapterResult:
        symbol = _symbol(code, market)
        base = _base_price(code, market)
        quote = Quote(
            symbol=symbol,
            price=round(base * 1.012, 3),
            previous_close=base,
            volume=1_000_000 + len(code) * 10_000,
            turnover=round(base * 1_000_000, 2),
            timestamp=datetime.now(timezone.utc),
            source=self.source,
        )
        return adapter_ok(self.source, quote, ["mock data for research pipeline only"])

    async def bars(self, code: str, market: str, periods: int = 30) -> AdapterResult:
        if periods <= 1 or periods > 250:
            return adapter_error(self.source, "periods must be between 2 and 250")
        symbol = _symbol(code, market)
        base = _base_price(code, market)
        now = datetime.now(timezone.utc)
        bars: list[Bar] = []
        for index in range(periods):
            drift = (index - periods / 2) * 0.015
            close = round(base + drift, 3)
            bars.append(
                Bar(
                    symbol=symbol,
                    timestamp=now - timedelta(days=periods - index),
                    open=round(close * 0.995, 3),
                    high=round(close * 1.015, 3),
                    low=round(close * 0.985, 3),
                    close=close,
                    volume=900_000 + index * 2_500,
                    source=self.source,
                )
            )
        return adapter_ok(self.source, bars, ["mock bars for research pipeline only"])


def _symbol(code: str, market: str) -> Symbol:
    normalized_market = market if market in {"A股", "港股"} else "A股"
    return Symbol(code=(code or "000001").strip(), market=normalized_market)  # type: ignore[arg-type]


def _base_price(code: str, market: str) -> float:
    seed = sum(ord(char) for char in (code or "000001"))
    base = 10 + seed % 80
    return float(base if market == "A股" else base * 1.8)
