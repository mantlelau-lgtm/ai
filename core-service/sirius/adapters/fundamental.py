from __future__ import annotations

from datetime import datetime, timezone

from sirius.adapters.base import adapter_ok
from sirius.data.models import AdapterResult, Symbol


class MockFundamentalDataAdapter:
    source = "mock-fundamental-data"

    async def financials(self, code: str, market: str, period: str = "latest") -> AdapterResult:
        symbol = Symbol(code=(code or "000001").strip(), market=market if market in {"A股", "港股"} else "A股")  # type: ignore[arg-type]
        data = {
            "symbol": {"code": symbol.code, "market": symbol.market},
            "period": period,
            "revenue": 1_000_000_000.0,
            "net_profit": 120_000_000.0,
            "roe": 0.118,
            "debt_to_asset": 0.42,
            "source_url": "mock://fundamental/financials",
            "timestamp": datetime.now(timezone.utc).isoformat(),
        }
        return adapter_ok(self.source, data, ["mock financials for research pipeline only"])

    async def announcements(self, code: str, market: str, keyword: str = "", limit: int = 10) -> AdapterResult:
        symbol = Symbol(code=(code or "000001").strip(), market=market if market in {"A股", "港股"} else "A股")  # type: ignore[arg-type]
        items = [
            {
                "symbol": {"code": symbol.code, "market": symbol.market},
                "title": f"{symbol.code} 年度报告摘要",
                "category": "annual_report",
                "published_at": datetime.now(timezone.utc).isoformat(),
                "source_url": "mock://fundamental/announcement/annual-report",
            },
            {
                "symbol": {"code": symbol.code, "market": symbol.market},
                "title": f"{symbol.code} 风险提示公告",
                "category": "risk_disclosure",
                "published_at": datetime.now(timezone.utc).isoformat(),
                "source_url": "mock://fundamental/announcement/risk",
            },
        ]
        if keyword:
            items = [item for item in items if keyword in item["title"]]
        return adapter_ok(self.source, items[: max(1, min(limit, 50))], ["mock announcements for research pipeline only"])
