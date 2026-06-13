import type {
  BundlePayload,
  BundleResponse,
  DiffResponse,
  OverviewResponse,
  SaveResponse,
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
}
