import os


class Config:
    def __init__(self) -> None:
        self.listen_addr = os.getenv("LISTEN_ADDR", "0.0.0.0:8081")
        self.database_url = os.getenv("DATABASE_URL", "")
        self.admin_token = os.getenv("ADMIN_TOKEN", "")
        self.routing_config_path = os.getenv("ROUTING_CONFIG_PATH", "")
        self.routing_reload_interval_seconds = float(os.getenv("ROUTING_RELOAD_INTERVAL_SECONDS", "2"))
        self.admin_config_base_url = os.getenv("ADMIN_CONFIG_BASE_URL", "").rstrip("/")
        self.admin_core_routing_path = os.getenv("ADMIN_CORE_ROUTING_PATH", "/api/runtime/core-service/routing")
        self.llm_base_url = os.getenv("LLM_BASE_URL", "http://localhost:8090").rstrip("/")
        self.llm_chat_path = os.getenv("LLM_CHAT_PATH", "/v1/chat/completions")
        self.default_model = os.getenv("DEFAULT_MODEL", "deepseek-v4-flash")
        self.request_timeout_seconds = int(os.getenv("REQUEST_TIMEOUT_SECONDS", "90"))
        self.conversation_window_size = int(os.getenv("CONVERSATION_WINDOW_SIZE", "20"))
        self.system_prompt = os.getenv(
            "SYSTEM_PROMPT",
            "You are the core orchestrator of Tracer 2.0. Provide concise, helpful responses.",
        )

        if not self.database_url:
            raise ValueError("DATABASE_URL is required")


cfg = Config()
