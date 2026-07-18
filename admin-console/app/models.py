from __future__ import annotations

from typing import Dict, List

from pydantic import BaseModel, Field


class BotConfig(BaseModel):
    bot_id: str = ""
    app_id: str = ""
    app_secret: str = ""
    open_base_url: str = ""
    agent_name: str = ""


class BotsFile(BaseModel):
    bots: List[BotConfig] = Field(default_factory=list)


class LlmCredential(BaseModel):
    vendor_name: str = ""
    base_url: str = ""
    type: str = "openai"
    call_type: str = "non_stream"
    key_name: str = ""
    key_value: str = ""
    model_id: str = ""
    model_name: str = ""
    metadata: Dict[str, str] = Field(default_factory=dict)
    enabled: bool = True
    is_default: bool = False


class LlmModel(BaseModel):
    name: str = ""
    model_id: str = ""
    provider: str = ""
    upstream_model: str = ""
    owned_by: str = ""
    prompt_cost_per_1k_tokens: float = 0
    completion_cost_per_1k_tokens: float = 0
    unit_price: float = 0
    enabled: bool = True


class AdminLlmConfig(BaseModel):
    credentials: List[LlmCredential] = Field(default_factory=list)
    models: List[LlmModel] = Field(default_factory=list)


class LlmKey(BaseModel):
    name: str = ""
    value: str = ""
    value_env: str = ""


class LlmProvider(BaseModel):
    name: str = ""
    type: str = "openai"
    base_url: str = ""
    api_key: str = ""
    enabled: bool = True
    is_default: bool = False
    metadata: Dict[str, str] = Field(default_factory=dict)


class LlmCatalog(BaseModel):
    keys: List[LlmKey] = Field(default_factory=list)
    providers: List[LlmProvider] = Field(default_factory=list)
    models: List[LlmModel] = Field(default_factory=list)


class RoutingEntry(BaseModel):
    bot_id: str = ""
    agent_name: str = ""


class AgentSpec(BaseModel):
    name: str = ""
    type: str = "custom"
    key_name: str = ""
    tools: List[str] = Field(default_factory=list)


class RegisteredAgent(BaseModel):
    name: str = ""
    type: str = "custom"
    source: str = "local"
    description: str = ""
    key_name: str = ""
    is_default: bool = False
    tools: List[str] = Field(default_factory=list)


class ToolDescriptor(BaseModel):
    model_config = {"populate_by_name": True}

    service: str = "agent-runtime"
    source: str = ""
    name: str = ""
    description: str = ""
    tool_schema: Dict[str, object] = Field(default_factory=dict, alias="schema")
    reported_at: str = ""


class ToolRegistryReport(BaseModel):
    service: str = "agent-runtime"
    instance_id: str = ""
    tools: List[ToolDescriptor] = Field(default_factory=list)
    reported_at: str = ""


class RoutingConfig(BaseModel):
    default_agent: str = ""
    bots: List[RoutingEntry] = Field(default_factory=list)
    agents: List[AgentSpec] = Field(default_factory=list)


class AdminRoutingConfig(BaseModel):
    default_agent: str = ""


class MessageRouteMatch(BaseModel):
    kind: str = ""
    event_type: str = ""
    text_equals: str = ""
    text_prefix: str = ""
    action_name: str = ""
    action_tag: str = ""


class MessageRouteAction(BaseModel):
    reply_text: str = ""
    toast_text: str = ""


class MessageRouteRule(BaseModel):
    id: str = ""
    priority: int = 0
    match: MessageRouteMatch = Field(default_factory=MessageRouteMatch)
    action: MessageRouteAction = Field(default_factory=MessageRouteAction)


class MessageRouteConfig(BaseModel):
    rules: List[MessageRouteRule] = Field(default_factory=list)


class BundlePayload(BaseModel):
    bots: BotsFile = Field(default_factory=BotsFile)
    llm: AdminLlmConfig = Field(default_factory=AdminLlmConfig)
    routing: AdminRoutingConfig = Field(default_factory=AdminRoutingConfig)
    message_routes: MessageRouteConfig = Field(default_factory=MessageRouteConfig)


class ValidationBucket(BaseModel):
    errors: List[str] = Field(default_factory=list)
    warnings: List[str] = Field(default_factory=list)


class ValidationResponse(BaseModel):
    errors: List[str] = Field(default_factory=list)
    warnings: List[str] = Field(default_factory=list)
    sections: Dict[str, ValidationBucket] = Field(default_factory=dict)


class TableMeta(BaseModel):
    name: str
    rows: int = 0
    updated_at: float | None = None


class DatabaseMeta(BaseModel):
    engine: str
    database: str
    status: str
    detail: str = ""


class ServiceStatus(BaseModel):
    name: str
    url: str
    status: str
    detail: str = ""


class OverviewResponse(BaseModel):
    services: List[ServiceStatus] = Field(default_factory=list)
    database: DatabaseMeta
    tables: Dict[str, TableMeta] = Field(default_factory=dict)
    summary: Dict[str, int] = Field(default_factory=dict)


class DiffResponse(BaseModel):
    validation: ValidationResponse
    bots_diff: str
    llm_diff: str
    routing_diff: str
    message_routes_diff: str


class SaveResponse(BaseModel):
    saved: bool
    validation: ValidationResponse
    updated_resources: List[str] = Field(default_factory=list)
    needs_restart: List[str] = Field(default_factory=list)
