from __future__ import annotations

from datetime import datetime, timezone

from sirius.data.models import Bar, OrderIntent, StrategySignal
from sirius.strategy.factors import momentum_factor, volatility_factor


def generate_signal(bars: list[Bar]) -> StrategySignal:
    symbol = bars[-1].symbol
    momentum = momentum_factor(symbol, bars)
    volatility = volatility_factor(symbol, bars)
    if momentum.value > 0.01 and volatility.value < 0.03:
        action = "buy"
        confidence = min(0.95, 0.55 + momentum.value * 5)
        reason = "momentum positive with controlled volatility"
    elif momentum.value < -0.01:
        action = "sell"
        confidence = min(0.95, 0.55 + abs(momentum.value) * 5)
        reason = "negative momentum"
    else:
        action = "hold"
        confidence = 0.5
        reason = "signal below threshold"
    return StrategySignal(symbol=symbol, action=action, confidence=confidence, reason=reason, timestamp=datetime.now(timezone.utc), factors=[momentum, volatility])


def signal_to_order_intent(signal: StrategySignal, latest_price: float, quantity: int = 100) -> OrderIntent | None:
    if signal.action == "hold":
        return None
    return OrderIntent(symbol=signal.symbol, action=signal.action, quantity=quantity, limit_price=latest_price, reason=signal.reason, timestamp=datetime.now(timezone.utc))
