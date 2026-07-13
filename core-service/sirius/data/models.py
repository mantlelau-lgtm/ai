from __future__ import annotations

from dataclasses import dataclass, field
from datetime import datetime
from typing import Any, Literal

Market = Literal["A股", "港股"]
SignalAction = Literal["buy", "sell", "hold"]
RiskDecisionStatus = Literal["allow", "block", "review"]
OrderStatus = Literal["submitted", "filled", "rejected", "cancelled"]


@dataclass(frozen=True)
class AdapterResult:
    ok: bool
    source: str
    timestamp: datetime
    data: Any = None
    errors: list[str] = field(default_factory=list)
    warnings: list[str] = field(default_factory=list)


@dataclass(frozen=True)
class Symbol:
    code: str
    market: Market
    name: str = ""


@dataclass(frozen=True)
class Quote:
    symbol: Symbol
    price: float
    previous_close: float
    volume: int
    turnover: float
    timestamp: datetime
    source: str

    @property
    def change_pct(self) -> float:
        if self.previous_close <= 0:
            return 0.0
        return (self.price - self.previous_close) / self.previous_close


@dataclass(frozen=True)
class Bar:
    symbol: Symbol
    timestamp: datetime
    open: float
    high: float
    low: float
    close: float
    volume: int
    source: str


@dataclass(frozen=True)
class FactorValue:
    symbol: Symbol
    name: str
    value: float
    timestamp: datetime
    source: str


@dataclass(frozen=True)
class StrategySignal:
    symbol: Symbol
    action: SignalAction
    confidence: float
    reason: str
    timestamp: datetime
    factors: list[FactorValue] = field(default_factory=list)


@dataclass(frozen=True)
class OrderIntent:
    symbol: Symbol
    action: SignalAction
    quantity: int
    limit_price: float
    reason: str
    timestamp: datetime


@dataclass(frozen=True)
class RiskDecision:
    status: RiskDecisionStatus
    reason: str
    checks: dict[str, Any]
    timestamp: datetime


@dataclass(frozen=True)
class SimulatedOrder:
    order_id: str
    intent: OrderIntent
    status: OrderStatus
    filled_price: float
    filled_quantity: int
    timestamp: datetime


def dataclass_to_dict(value: Any) -> Any:
    if isinstance(value, datetime):
        return value.isoformat()
    if hasattr(value, "__dataclass_fields__"):
        return {key: dataclass_to_dict(getattr(value, key)) for key in value.__dataclass_fields__}
    if isinstance(value, list):
        return [dataclass_to_dict(item) for item in value]
    if isinstance(value, dict):
        return {key: dataclass_to_dict(item) for key, item in value.items()}
    return value
