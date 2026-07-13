from __future__ import annotations

from datetime import datetime, timezone

from sirius.data.models import Bar, FactorValue, Symbol


def momentum_factor(symbol: Symbol, bars: list[Bar]) -> FactorValue:
    if len(bars) < 2 or bars[0].close <= 0:
        value = 0.0
    else:
        value = (bars[-1].close - bars[0].close) / bars[0].close
    return FactorValue(symbol=symbol, name="momentum", value=value, timestamp=datetime.now(timezone.utc), source="sirius.local")


def volatility_factor(symbol: Symbol, bars: list[Bar]) -> FactorValue:
    if len(bars) < 2:
        value = 0.0
    else:
        returns = [(bars[i].close - bars[i - 1].close) / bars[i - 1].close for i in range(1, len(bars)) if bars[i - 1].close]
        mean = sum(returns) / len(returns) if returns else 0.0
        variance = sum((item - mean) ** 2 for item in returns) / len(returns) if returns else 0.0
        value = variance ** 0.5
    return FactorValue(symbol=symbol, name="volatility", value=value, timestamp=datetime.now(timezone.utc), source="sirius.local")
