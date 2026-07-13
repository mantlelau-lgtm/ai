import type {
  BundlePayload,
  BundleResponse,
  DiffResponse,
  LlmCredential,
  LlmModel,
  OverviewResponse,
  SaveResponse,
  ToolDescriptor,
  ValidationResponse,
} from '@/types'

async function requestJson<T>(path: string, options: RequestInit = {}): Promise<T> {
  const headers = new Headers(options.headers)
  headers.set('Content-Type', 'application/json')

  const response = await fetch(path, {
    ...options,
    headers,
  })

  if (!response.ok) {
    let detail = '请求失败'
    try {
      const payload = await response.json()
      detail =
        typeof payload.detail === 'string'
          ? payload.detail
          : JSON.stringify(payload.detail ?? payload, null, 2)
    } catch {
      detail = await response.text()
    }
    throw new Error(detail || `请求失败: ${response.status}`)
  }

  return response.json() as Promise<T>
}

export const adminApi = {
  getOverview: () => requestJson<OverviewResponse>('/api/overview'),
  getBundle: () => requestJson<BundleResponse>('/api/config/bundle'),
  listAgents: () =>
    requestJson<{
      agents: { name: string; type: string; source: string; description: string; key_name: string; tools: string[] }[]
    }>('/api/agents'),
  listTools: () => requestJson<{ tools: ToolDescriptor[] }>('/api/tools'),
  updateAgentTools: (name: string, tools: string[]) =>
    requestJson<{ agent: string; tools: string[] }>(`/api/agents/${encodeURIComponent(name)}/tools`, {
      method: 'PUT',
      body: JSON.stringify({ tools }),
    }),
  updateAgentKey: (name: string, keyName: string) =>
    requestJson<{
      agent: { name: string; type: string; source: string; description: string; key_name: string; tools: string[] }
    }>(`/api/agents/${encodeURIComponent(name)}/key`, {
      method: 'PUT',
      body: JSON.stringify({ key_name: keyName }),
    }),
  validate: (payload: BundlePayload) =>
    requestJson<ValidationResponse>('/api/config/validate', {
      method: 'POST',
      body: JSON.stringify(payload),
    }),
  diff: (payload: BundlePayload) =>
    requestJson<DiffResponse>('/api/config/diff', {
      method: 'POST',
      body: JSON.stringify(payload),
    }),
  apply: (payload: BundlePayload) =>
    requestJson<SaveResponse>('/api/config/apply', {
      method: 'POST',
      body: JSON.stringify(payload),
    }),
  createModel: (model: LlmModel) =>
    requestJson<{ model: LlmModel }>('/api/llm/models', {
      method: 'POST',
      body: JSON.stringify(model),
    }),
  updateModel: (originalModelId: string, model: LlmModel) =>
    requestJson<{ model: LlmModel }>(`/api/llm/models/${encodeURIComponent(originalModelId)}`, {
      method: 'PUT',
      body: JSON.stringify(model),
    }),
  deleteModel: (modelId: string) =>
    requestJson<{ deleted: boolean; model_id: string }>(`/api/llm/models/${encodeURIComponent(modelId)}`, {
      method: 'DELETE',
    }),
  createCredential: (credential: LlmCredential) =>
    requestJson<{ credential: LlmCredential }>('/api/llm/keys', {
      method: 'POST',
      body: JSON.stringify(credential),
    }),
  updateCredential: (originalKeyName: string, credential: LlmCredential) =>
    requestJson<{ credential: LlmCredential }>(`/api/llm/keys/${encodeURIComponent(originalKeyName)}`, {
      method: 'PUT',
      body: JSON.stringify(credential),
    }),
  deleteCredential: (keyName: string) =>
    requestJson<{ deleted: boolean; key_name: string }>(`/api/llm/keys/${encodeURIComponent(keyName)}`, {
      method: 'DELETE',
    }),
  checkKeyName: (name: string) =>
    requestJson<{ name: string; available: boolean }>(
      `/api/llm/keys/check-name?name=${encodeURIComponent(name)}`,
    ),
  setDefaultKey: (keyName: string) =>
    requestJson<{ key_name: string; is_default: boolean }>('/api/llm/keys/set-default', {
      method: 'POST',
      body: JSON.stringify({ key_name: keyName }),
    }),
}
