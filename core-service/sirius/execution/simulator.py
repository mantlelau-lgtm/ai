from __future__ import annotations

from datetime import datetime, timezone
from uuid import uuid4

from sirius.data.models import OrderIntent, SimulatedOrder


def submit_simulated_order(intent: OrderIntent | None) -> SimulatedOrder | None:
    if intent is None:
        return None
    return SimulatedOrder(
        order_id=f"sim-{uuid4().hex[:12]}",
        intent=intent,
        status="filled",
        filled_price=intent.limit_price,
        filled_quantity=intent.quantity,
        timestamp=datetime.now(timezone.utc),
    )
