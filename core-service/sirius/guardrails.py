from __future__ import annotations

from dataclasses import dataclass
from datetime import time
from zoneinfo import ZoneInfo


MARKET_TIMEZONE = ZoneInfo("Asia/Shanghai")


@dataclass(frozen=True)
class MarketRule:
    market: str
    trading_sessions: tuple[tuple[time, time], ...]
    settlement: str
    live_trading_enabled: bool = False


MARKET_RULES: dict[str, MarketRule] = {
    "A股": MarketRule(
        market="A股",
        trading_sessions=((time(9, 30), time(11, 30)), (time(13, 0), time(15, 0))),
        settlement="T+1",
    ),
    "港股": MarketRule(
        market="港股",
        trading_sessions=((time(9, 30), time(12, 0)), (time(13, 0), time(16, 0))),
        settlement="T+0",
    ),
}


def compliance_disclaimer() -> str:
    return (
        "Sirius 仅提供量化研究、数据分析、回测与风控辅助能力；"
        "默认不执行实盘交易，不构成投资建议或收益承诺。"
        "任何实盘交易必须经过用户授权、券商/交易所合规接口、前置风控与完整审计。"
    )


def live_trading_allowed(market: str) -> bool:
    rule = MARKET_RULES.get((market or "").strip())
    return bool(rule and rule.live_trading_enabled)


def market_rules() -> list[dict[str, object]]:
    return [
        {
            "market": rule.market,
            "trading_sessions": [
                {"start": start.strftime("%H:%M"), "end": end.strftime("%H:%M")}
                for start, end in rule.trading_sessions
            ],
            "settlement": rule.settlement,
            "live_trading_enabled": rule.live_trading_enabled,
        }
        for rule in MARKET_RULES.values()
    ]
