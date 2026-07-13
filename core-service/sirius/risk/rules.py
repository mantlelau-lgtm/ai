from __future__ import annotations

from datetime import datetime, timezone

from sirius.data.models import OrderIntent, RiskDecision
from sirius.guardrails import live_trading_allowed


def evaluate_order_intent(intent: OrderIntent | None, max_notional: float = 100_000.0) -> RiskDecision:
    if intent is None:
        return RiskDecision(status="allow", reason="no executable order intent", checks={}, timestamp=datetime.now(timezone.utc))
    notional = intent.quantity * intent.limit_price
    checks = {
        "notional": notional,
        "max_notional": max_notional,
        "market": intent.symbol.market,
        "live_trading_allowed": live_trading_allowed(intent.symbol.market),
    }
    if not live_trading_allowed(intent.symbol.market):
        return RiskDecision(status="review", reason="live trading disabled; simulation only", checks=checks, timestamp=datetime.now(timezone.utc))
    if notional > max_notional:
        return RiskDecision(status="block", reason="single order notional exceeds limit", checks=checks, timestamp=datetime.now(timezone.utc))
    return RiskDecision(status="allow", reason="risk checks passed", checks=checks, timestamp=datetime.now(timezone.utc))
