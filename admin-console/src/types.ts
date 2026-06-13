export type BotConfig = {
  bot_id: string
  app_id: string
  app_secret: string
  open_base_url: string
}

export type BotsFile = {
  bots: BotConfig[]
}

export type LlmKey = {
  name: string
  value_env: string
}

export type LlmProvider = {
  name: string
  type: string
  base_url: string
  api_key_from: string
  enabled: boolean
  is_default: boolean
}

export type LlmModel = {
  name: string
  provider: string
  upstream_model: string
  owned_by: string
  prompt_cost_per_1k_tokens: number
  completion_cost_per_1k_tokens: number
  enabled: boolean
}

export type LlmCatalog = {
  keys: LlmKey[]
  providers: LlmProvider[]
  models: LlmModel[]
}

export type RoutingEntry = {
  bot_id: string
  agent_name: string
}

export type RoutingConfig = {
  default_agent: string
  bots: RoutingEntry[]
}

export type BundlePayload = {
  bots: BotsFile
  llm: LlmCatalog
  routing: RoutingConfig
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
  llm: { keys: [], providers: [], models: [] },
  routing: { default_agent: 'general', bots: [] },
})
