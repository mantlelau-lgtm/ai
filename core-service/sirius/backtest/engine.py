from __future__ import annotations

from sirius.data.models import Bar
from sirius.strategy.factors import momentum_factor, volatility_factor


def run_simple_backtest(bars: list[Bar]) -> dict[str, float | int | str]:
    if len(bars) < 2:
        return {"trades": 0, "total_return": 0.0, "max_drawdown": 0.0, "sharpe": 0.0, "note": "not enough bars"}
    returns = [(bars[i].close - bars[i - 1].close) / bars[i - 1].close for i in range(1, len(bars)) if bars[i - 1].close]
    equity = 1.0
    peak = 1.0
    max_drawdown = 0.0
    for item in returns:
        equity *= 1 + item
        peak = max(peak, equity)
        max_drawdown = min(max_drawdown, equity / peak - 1)
    avg = sum(returns) / len(returns) if returns else 0.0
    variance = sum((item - avg) ** 2 for item in returns) / len(returns) if returns else 0.0
    vol = variance ** 0.5
    sharpe = (avg / vol * (252 ** 0.5)) if vol else 0.0
    return {
        "trades": max(1, len(returns) // 5),
        "total_return": equity - 1,
        "max_drawdown": max_drawdown,
        "sharpe": sharpe,
        "momentum": momentum_factor(bars[-1].symbol, bars).value,
        "volatility": volatility_factor(bars[-1].symbol, bars).value,
        "note": "mock local backtest; historical performance is not future return",
    }
