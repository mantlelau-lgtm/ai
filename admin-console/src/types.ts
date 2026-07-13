export type BotConfig = {
  bot_id: string
  app_id: string
  app_secret: string
  open_base_url: string
  agent_name: string
}

export type BotsFile = {
  bots: BotConfig[]
}

export type LlmCredential = {
  vendor_name: string
  key_name: string
  key_value: string
  type: string
  call_type: string
  base_url: string
  model_id: string
  model_name: string
  metadata: Record<string, string>
  enabled: boolean
  is_default: boolean
}

export type LlmModel = {
  name: string
  model_id: string
  provider: string
  upstream_model: string
  owned_by: string
  prompt_cost_per_1k_tokens: number
  completion_cost_per_1k_tokens: number
  unit_price: number
  enabled: boolean
}

export type AdminLlmConfig = {
  credentials: LlmCredential[]
  models: LlmModel[]
}

export type RegisteredAgent = {
  name: string
  type: string
  source: string
  description: string
  key_name: string
  tools: string[]
}

export type ToolDescriptor = {
  service: string
  source: string
  name: string
  description: string
  schema: Record<string, unknown>
  reported_at: string
}

export type AdminRoutingConfig = {
  default_agent: string
}

export type MessageRouteMatch = {
  kind: string
  event_type: string
  text_equals: string
  text_prefix: string
  action_name: string
  action_tag: string
}

export type MessageRouteAction = {
  reply_text: string
  toast_text: string
}

export type MessageRouteRule = {
  id: string
  priority: number
  match: MessageRouteMatch
  action: MessageRouteAction
}

export type MessageRouteConfig = {
  rules: MessageRouteRule[]
}

export type BundlePayload = {
  bots: BotsFile
  llm: AdminLlmConfig
  routing: AdminRoutingConfig
  message_routes: MessageRouteConfig
}

export type ValidationBucket = {
  errors: string[]
  warnings: string[]
}

export type ValidationResponse = {
  errors: string[]
  warnings: string[]
  sections: Record<string, ValidationBucket>
}

export type DiffResponse = {
  validation: ValidationResponse
  bots_diff: string
  llm_diff: string
  routing_diff: string
  message_routes_diff: string
}

export type SaveResponse = {
  saved: boolean
  validation: ValidationResponse
  updated_resources: string[]
  needs_restart: string[]
}

export type ServiceStatus = {
  name: string
  url: string
  status: 'ok' | 'error' | 'offline'
  detail: string
}

export type TableMeta = {
  name: string
  rows: number
  updated_at: number | null
}

export type DatabaseMeta = {
  engine: string
  database: string
  status: 'ok' | 'error' | 'offline'
  detail: string
}

export type OverviewResponse = {
  services: ServiceStatus[]
  database: DatabaseMeta
  tables: Record<string, TableMeta>
  summary: Record<string, number>
}

export type BundleResponse = {
  storage: {
    engine: string
    database: string
  }
  data: BundlePayload
}

export const createEmptyBundle = (): BundlePayload => ({
  bots: { bots: [] },
  llm: { credentials: [], models: [] },
  routing: { default_agent: 'general' },
  message_routes: { rules: [] },
})
