import type { ButtonHTMLAttributes } from 'react'

import { Plus, Trash2 } from 'lucide-react'

import { CodePanel } from '@/components/CodePanel'
import { Panel } from '@/components/Panel'
import { useAdminStore } from '@/store/useAdminStore'
import type { LlmKey, LlmModel, LlmProvider } from '@/types'

const emptyKey = (): LlmKey => ({ name: '', value_env: '' })
const emptyProvider = (): LlmProvider => ({
  name: '',
  type: 'openai',
  base_url: '',
  api_key_from: '',
  enabled: true,
  is_default: false,
})
const emptyModel = (): LlmModel => ({
  name: '',
  provider: '',
  upstream_model: '',
  owned_by: '',
  prompt_cost_per_1k_tokens: 0,
  completion_cost_per_1k_tokens: 0,
  enabled: true,
})

export function LlmPage() {
  const llm = useAdminStore((state) => state.bundle.llm)
  const updateKeys = useAdminStore((state) => state.updateKeys)
  const updateProviders = useAdminStore((state) => state.updateProviders)
  const updateModels = useAdminStore((state) => state.updateModels)

  return (
    <div className="space-y-6">
      <div className="grid gap-6 xl:grid-cols-3">
        <Panel
          title="Key"
          eyebrow="Credential Mapping"
          action={
            <MiniButton onClick={() => updateKeys((items) => [...items, emptyKey()])}>
              <Plus className="h-4 w-4" />
              新增 Key
            </MiniButton>
          }
        >
          <div className="space-y-4">
            {llm.keys.map((key, index) => (
              <div key={`${key.name}-${index}`} className="rounded-[24px] border border-white/10 bg-slate-950/45 p-4">
                <div className="mb-4 flex items-center justify-between">
                  <div className="text-sm text-white">Key #{index + 1}</div>
                  <DangerButton
                    onClick={() => updateKeys((items) => items.filter((_, currentIndex) => currentIndex !== index))}
                  >
                    <Trash2 className="h-4 w-4" />
                  </DangerButton>
                </div>
                <Field
                  label="name"
                  value={key.name}
                  onChange={(value) => patchKey(updateKeys, index, { name: value })}
                />
                <div className="mt-4">
                  <Field
                    label="value_env"
                    value={key.value_env}
                    onChange={(value) => patchKey(updateKeys, index, { value_env: value })}
                  />
                </div>
              </div>
            ))}
          </div>
        </Panel>

        <Panel
          title="Provider"
          eyebrow="LLM Upstream"
          action={
            <MiniButton onClick={() => updateProviders((items) => [...items, emptyProvider()])}>
              <Plus className="h-4 w-4" />
              新增 Provider
            </MiniButton>
          }
        >
          <div className="space-y-4">
            {llm.providers.map((provider, index) => (
              <div key={`${provider.name}-${index}`} className="rounded-[24px] border border-white/10 bg-slate-950/45 p-4">
                <div className="mb-4 flex items-center justify-between">
                  <div className="text-sm text-white">Provider #{index + 1}</div>
                  <DangerButton
                    onClick={() =>
                      updateProviders((items) => items.filter((_, currentIndex) => currentIndex !== index))
                    }
                  >
                    <Trash2 className="h-4 w-4" />
                  </DangerButton>
                </div>
                <div className="grid gap-4">
                  <Field
                    label="name"
                    value={provider.name}
                    onChange={(value) => patchProvider(updateProviders, index, { name: value })}
                  />
                  <Field
                    label="type"
                    value={provider.type}
                    onChange={(value) => patchProvider(updateProviders, index, { type: value })}
                  />
                  <Field
                    label="base_url"
                    value={provider.base_url}
                    onChange={(value) => patchProvider(updateProviders, index, { base_url: value })}
                  />
                  <Field
                    label="api_key_from"
                    value={provider.api_key_from}
                    onChange={(value) => patchProvider(updateProviders, index, { api_key_from: value })}
                  />
                </div>
                <div className="mt-4 flex flex-wrap gap-3">
                  <Toggle
                    label="启用"
                    checked={provider.enabled}
                    onChange={(checked) => patchProvider(updateProviders, index, { enabled: checked })}
                  />
                  <Toggle
                    label="默认"
                    checked={provider.is_default}
                    onChange={(checked) =>
                      updateProviders((items) =>
                        items.map((item, currentIndex) => ({
                          ...item,
                          is_default: currentIndex === index ? checked : checked ? false : item.is_default,
                        })),
                      )
                    }
                  />
                </div>
              </div>
            ))}
          </div>
        </Panel>

        <Panel
          title="Model"
          eyebrow="Serving Catalog"
          action={
            <MiniButton onClick={() => updateModels((items) => [...items, emptyModel()])}>
              <Plus className="h-4 w-4" />
              新增 Model
            </MiniButton>
          }
        >
          <div className="space-y-4">
            {llm.models.map((model, index) => (
              <div key={`${model.name}-${index}`} className="rounded-[24px] border border-white/10 bg-slate-950/45 p-4">
                <div className="mb-4 flex items-center justify-between">
                  <div className="text-sm text-white">Model #{index + 1}</div>
                  <DangerButton
                    onClick={() => updateModels((items) => items.filter((_, currentIndex) => currentIndex !== index))}
                  >
                    <Trash2 className="h-4 w-4" />
                  </DangerButton>
                </div>
                <div className="grid gap-4">
                  <Field
                    label="name"
                    value={model.name}
                    onChange={(value) => patchModel(updateModels, index, { name: value })}
                  />
                  <Field
                    label="provider"
                    value={model.provider}
                    onChange={(value) => patchModel(updateModels, index, { provider: value })}
                  />
                  <Field
                    label="upstream_model"
                    value={model.upstream_model}
                    onChange={(value) => patchModel(updateModels, index, { upstream_model: value })}
                  />
                  <Field
                    label="owned_by"
                    value={model.owned_by}
                    onChange={(value) => patchModel(updateModels, index, { owned_by: value })}
                  />
                  <Field
                    label="prompt_cost_per_1k_tokens"
                    value={String(model.prompt_cost_per_1k_tokens)}
                    onChange={(value) =>
                      patchModel(updateModels, index, {
                        prompt_cost_per_1k_tokens: Number(value) || 0,
                      })
                    }
                  />
                  <Field
                    label="completion_cost_per_1k_tokens"
                    value={String(model.completion_cost_per_1k_tokens)}
                    onChange={(value) =>
                      patchModel(updateModels, index, {
                        completion_cost_per_1k_tokens: Number(value) || 0,
                      })
                    }
                  />
                </div>
                <div className="mt-4">
                  <Toggle
                    label="启用"
                    checked={model.enabled}
                    onChange={(checked) => patchModel(updateModels, index, { enabled: checked })}
                  />
                </div>
              </div>
            ))}
          </div>
        </Panel>
      </div>

      <Panel title="数据库快照" eyebrow="admin_llm_*">
        <CodePanel code={JSON.stringify(llm, null, 2)} />
      </Panel>
    </div>
  )
}

function MiniButton(props: ButtonHTMLAttributes<HTMLButtonElement>) {
  return (
    <button
      type="button"
      {...props}
      className="inline-flex items-center gap-2 rounded-2xl bg-cyan-300/90 px-4 py-3 text-sm font-medium text-slate-950 transition hover:bg-cyan-200"
    />
  )
}

function DangerButton(props: ButtonHTMLAttributes<HTMLButtonElement>) {
  return (
    <button
      type="button"
      {...props}
      className="rounded-2xl border border-rose-300/20 bg-rose-500/10 p-3 text-rose-100 transition hover:bg-rose-500/15"
    />
  )
}

function Toggle({
  label,
  checked,
  onChange,
}: {
  label: string
  checked: boolean
  onChange: (checked: boolean) => void
}) {
  return (
    <label className="inline-flex items-center gap-3 rounded-full border border-white/10 bg-white/5 px-4 py-2 text-sm text-slate-200">
      <input type="checkbox" checked={checked} onChange={(event) => onChange(event.target.checked)} />
      {label}
    </label>
  )
}

function Field({
  label,
  value,
  onChange,
}: {
  label: string
  value: string
  onChange: (value: string) => void
}) {
  return (
    <label className="block">
      <span className="mb-2 block text-xs uppercase tracking-[0.25em] text-slate-400">{label}</span>
      <input
        value={value}
        onChange={(event) => onChange(event.target.value)}
        className="w-full rounded-2xl border border-white/10 bg-white/5 px-4 py-3 text-sm text-white outline-none transition focus:border-cyan-300/40"
      />
    </label>
  )
}

function patchKey(
  updateKeys: (updater: (keys: LlmKey[]) => LlmKey[]) => void,
  index: number,
  patch: Partial<LlmKey>,
) {
  updateKeys((items) => items.map((item, currentIndex) => (currentIndex === index ? { ...item, ...patch } : item)))
}

function patchProvider(
  updateProviders: (updater: (providers: LlmProvider[]) => LlmProvider[]) => void,
  index: number,
  patch: Partial<LlmProvider>,
) {
  updateProviders((items) =>
    items.map((item, currentIndex) => (currentIndex === index ? { ...item, ...patch } : item)),
  )
}

function patchModel(
  updateModels: (updater: (models: LlmModel[]) => LlmModel[]) => void,
  index: number,
  patch: Partial<LlmModel>,
) {
  updateModels((items) => items.map((item, currentIndex) => (currentIndex === index ? { ...item, ...patch } : item)))
}
