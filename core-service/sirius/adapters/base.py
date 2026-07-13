from __future__ import annotations

from datetime import datetime, timezone
from typing import Any

from sirius.data.models import AdapterResult


def adapter_ok(source: str, data: Any, warnings: list[str] | None = None) -> AdapterResult:
    return AdapterResult(ok=True, source=source, timestamp=datetime.now(timezone.utc), data=data, warnings=warnings or [])


def adapter_error(source: str, error: str, warnings: list[str] | None = None) -> AdapterResult:
    return AdapterResult(ok=False, source=source, timestamp=datetime.now(timezone.utc), errors=[error], warnings=warnings or [])
