from __future__ import annotations

import os
from dataclasses import dataclass
from pathlib import Path


def _default_path(*parts: str) -> str:
    return str((Path(__file__).resolve().parent.parent / Path(*parts)).resolve())


@dataclass(frozen=True)
class Settings:
    database_url: str
    encryption_secret: str
    disable_db_startup: bool
    message_gateway_health_url: str
    core_service_health_url: str
    llm_gateway_health_url: str
    static_dir: str


def load_settings() -> Settings:
    return Settings(
        database_url=os.getenv(
            "DATABASE_URL",
            "postgres://admin_console:admin_console_pwd@127.0.0.1:5432/admin_console?sslmode=disable",
        ).strip(),
        encryption_secret=os.getenv(
            "ADMIN_CONSOLE_ENCRYPTION_SECRET",
            "admin-console-local-secret-change-me",
        ).strip(),
        disable_db_startup=os.getenv("ADMIN_CONSOLE_DISABLE_DB_STARTUP", "").strip() == "1",
        message_gateway_health_url=os.getenv(
            "MESSAGE_GATEWAY_HEALTH_URL",
            "http://127.0.0.1:50082/admin/healthz",
        ).strip(),
        core_service_health_url=os.getenv(
            "CORE_SERVICE_HEALTH_URL",
            "http://127.0.0.1:50081/healthz",
        ).strip(),
        llm_gateway_health_url=os.getenv(
            "LLM_GATEWAY_HEALTH_URL",
            "http://127.0.0.1:50080/healthz",
        ).strip(),
        static_dir=os.getenv(
            "STATIC_DIR",
            _default_path("dist"),
        ).strip(),
    )


settings = load_settings()
