import { useEffect, useMemo, useState } from 'react'

import { Plus, Save, Trash2 } from 'lucide-react'

import { AccordionCard } from '@/components/AccordionCard'
import { ConfirmDialog } from '@/components/ConfirmDialog'
import {
  DeleteIconButton,
  Field,
  MiniButton,
  SaveIconButton,
  SecondaryButton,
  Toggle,
} from '@/components/ConfigControls'
import { Modal } from '@/components/Modal'
import { Panel } from '@/components/Panel'
import { useAdminStore } from '@/store/useAdminStore'
import { adminApi } from '@/utils/api'
import type { LlmCredential, LlmModel } from '@/types'

const emptyCredential = (): LlmCredential => ({
  vendor_name: '',
  base_url: '',
  type: 'openai',
  call_type: 'non_stream',
  key_name: '',
  key_value: '',
  model_id: '',
  model_name: '',
  metadata: {},
  enabled: true,
  is_default: false,
})

export function LlmKeysPage() {
  const llm = useAdminStore((state) => state.bundle.llm)
  const updateCredentials = useAdminStore((state) => state.updateCredentials)
  const saveCredentialCard = useAdminStore((state) => state.saveCredentialCard)
  const deleteCredentialCard = useAdminStore((state) => state.deleteCredentialCard)
  const saving = useAdminStore((state) => state.saving)
  const [openIndex, setOpenIndex] = useState<number | null>(0)
  const [createOpen, setCreateOpen] = useState(false)
  const [draft, setDraft] = useState<LlmCredential>(emptyCredential())
  const [metadataDraft, setMetadataDraft] = useState('')
  const [confirmIndex, setConfirmIndex] = useState<number | null>(null)
  const [keyNameAvailable, setKeyNameAvailable] = useState<boolean | null>(null)
  const [keyNameChecking, setKeyNameChecking] = useState(false)

  const closeCreate = () => {
    setCreateOpen(false)
    setDraft(emptyCredential())
    setMetadataDraft('')
    setKeyNameAvailable(null)
  }

  // 同步 model_id 选择 -> 只读字段
  useEffect(() => {
    if (!draft.model_id) {
      if (draft.model_name || draft.vendor_name) {
        setDraft((current) => ({ ...current, model_name: '', vendor_name: '' }))
      }
      return
    }
    const referenced = llm.models.find((item) => item.model_id === draft.model_id)
    if (referenced) {
      const nextModelName = referenced.name
      const nextVendor = referenced.provider
      if (draft.model_name !== nextModelName || draft.vendor_name !== nextVendor) {
        setDraft((current) => ({ ...current, model_name: nextModelName, vendor_name: nextVendor }))
      }
    }
  }, [draft.model_id, llm.models, draft.model_name, draft.vendor_name])

  // 异步校验密钥名称唯一性
  useEffect(() => {
    const target = draft.key_name.trim()
    if (!target) {
      setKeyNameAvailable(null)
      return
    }
    let cancelled = false
    setKeyNameChecking(true)
    const handle = window.setTimeout(async () => {
      try {
        const result = await adminApi.checkKeyName(target)
        if (!cancelled) {
          setKeyNameAvailable(result.available)
        }
      } catch {
        if (!cancelled) {
          setKeyNameAvailable(null)
        }
      } finally {
        if (!cancelled) {
          setKeyNameChecking(false)
        }
      }
    }, 300)
    return () => {
      cancelled = true
      window.clearTimeout(handle)
    }
  }, [draft.key_name])

  const modelOptions = useMemo(
    () => llm.models.filter((item) => item.model_id.trim().length > 0),
    [llm.models],
  )

  const draftHint = (() => {
    if (!draft.model_id.trim()) return '请选择模型ID'
    if (!draft.key_name.trim()) return '请填写密钥名称'
    if (keyNameAvailable === false) return '密钥名称已存在，请更换'
    if (!draft.base_url.trim()) return '请填写 base_url'
    if (!/^https?:\/\//.test(draft.base_url.trim())) return 'base_url 需以 http:// 或 https:// 开头'
    if (!draft.key_value.trim()) return '请填写密钥值'
    return ''
  })()
  const draftValid = !draftHint && !keyNameChecking

  const setDefaultIndex = (index: number) => {
    updateCredentials((items) =>
      items.map((item, currentIndex) => ({
        ...item,
        is_default: currentIndex === index,
      })),
    )
  }

  return (
    <div className="space-y-6">
      <Panel
        title="密钥管理"
        eyebrow="Credential Vault"
        action={
          <MiniButton onClick={() => setCreateOpen(true)} disabled={modelOptions.length === 0}>
            <Plus className="h-4 w-4" />
            新增密钥
          </MiniButton>
        }
      >
        <div className="space-y-4">
          {modelOptions.length === 0 ? (
            <div className="rounded-[24px] border border-dashed border-amber-300/40 bg-amber-500/10 px-5 py-8 text-sm text-amber-100">
              请先到「模型管理」新增模型，再创建关联密钥。
            </div>
          ) : null}
          {llm.credentials.length === 0 ? (
            <div className="rounded-[24px] border border-dashed border-white/15 bg-white/5 px-5 py-8 text-sm text-slate-300">
              当前还没有密钥，新增后可在这里维护连接信息。
            </div>
          ) : null}
          {llm.credentials.map((credential, index) => (
            <AccordionCard
              key={`${credential.key_name}-${index}`}
              title={credential.key_name || `未命名密钥 #${index + 1}`}
              subtitle={buildSubtitle(credential)}
              isOpen={openIndex === index}
              onToggle={() => setOpenIndex((current) => (current === index ? null : index))}
              footerActions={
                <>
                  <SaveIconButton
                    onClick={async () => {
                      await saveCredentialCard(credential, credential.key_name)
                    }}
                    disabled={saving}
                  >
                    <Save className="h-4 w-4" />
                  </SaveIconButton>
                  <DeleteIconButton disabled={saving} onClick={() => setConfirmIndex(index)}>
                    <Trash2 className="h-4 w-4" />
                  </DeleteIconButton>
                </>
              }
            >
              <CredentialFields
                value={credential}
                models={modelOptions}
                isDefaultLocked={!credential.is_default}
                onChange={(patch) => patchCredential(updateCredentials, index, patch, llm.models)}
                onSetDefault={() => setDefaultIndex(index)}
              />
            </AccordionCard>
          ))}
        </div>
      </Panel>

      <Modal
        open={createOpen}
        title="新增密钥"
        description="选择模型ID 自动带入厂商与模型名，密钥名称需全局唯一并加密存储。"
        onClose={closeCreate}
        footer={
          <>
            <SecondaryButton onClick={closeCreate} disabled={saving}>
              取消
            </SecondaryButton>
            <MiniButton
              disabled={saving || !draftValid}
              onClick={async () => {
                const insertIndex = llm.credentials.length
                const willBeDefault = draft.is_default || llm.credentials.length === 0
                const finalDraft = { ...draft, is_default: willBeDefault, metadata: parseMetadata(metadataDraft) }
                updateCredentials((items) => {
                  const next = [...items, finalDraft]
                  if (willBeDefault) {
                    return next.map((item, idx) => ({ ...item, is_default: idx === insertIndex }))
                  }
                  return next
                })
                setOpenIndex(insertIndex)
                const ok = await saveCredentialCard(finalDraft)
                if (ok) {
                  closeCreate()
                } else {
                  updateCredentials((items) => items.filter((_, currentIndex) => currentIndex !== insertIndex))
                  setOpenIndex(null)
                }
              }}
            >
              <Save className="h-4 w-4" />
              {saving ? '保存中...' : '确认新增'}
            </MiniButton>
          </>
        }
      >
        {draftHint ? (
          <div className="mb-3 rounded-2xl border border-amber-300/30 bg-amber-500/10 px-4 py-2 text-xs text-amber-100">
            {draftHint}
          </div>
        ) : null}
        {keyNameChecking ? (
          <div className="mb-3 rounded-2xl border border-cyan-300/30 bg-cyan-500/10 px-4 py-2 text-xs text-cyan-100">
            正在校验密钥名称唯一性…
          </div>
        ) : null}
        <CredentialFields
          value={draft}
          models={modelOptions}
          metadataText={metadataDraft}
          onMetadataChange={setMetadataDraft}
          isDefaultLocked={!draft.is_default && llm.credentials.length === 0 ? false : !draft.is_default && llm.credentials.length === 0}
          isCreate
          onChange={(patch) => setDraft((current) => mergeDraftPatch(current, patch, llm.models))}
          onSetDefault={() => setDraft((current) => ({ ...current, is_default: true }))}
        />
      </Modal>

      <ConfirmDialog
        open={confirmIndex !== null}
        title="删除密钥确认"
        description={
          confirmIndex !== null && llm.credentials[confirmIndex]
            ? `确认删除密钥「${llm.credentials[confirmIndex].key_name || '未命名'}」吗？删除后将立即写入数据库。`
            : '确认删除该密钥吗？'
        }
        confirmText="删除"
        tone="danger"
        busy={saving}
        onCancel={() => setConfirmIndex(null)}
        onConfirm={async () => {
          if (confirmIndex === null) return
          const target = confirmIndex
          const keyName = llm.credentials[target]?.key_name
          if (!keyName) return
          const ok = await deleteCredentialCard(keyName)
          if (ok) {
            setConfirmIndex(null)
            setOpenIndex((current) => {
              if (current === target) {
                return null
              }
              if (current !== null && current > target) {
                return current - 1
              }
              return current
            })
          }
        }}
      />
    </div>
  )
}

type CredentialFieldsProps = {
  value: LlmCredential
  models: LlmModel[]
  isDefaultLocked: boolean
  isCreate?: boolean
  metadataText?: string
  onMetadataChange?: (value: string) => void
  onChange: (patch: Partial<LlmCredential>) => void
  onSetDefault: () => void
}

function CredentialFields({
  value,
  models,
  isDefaultLocked,
  isCreate,
  onChange,
  metadataText,
  onMetadataChange,
  onSetDefault,
}: CredentialFieldsProps) {
  const metadataValue = metadataText ?? formatMetadata(value.metadata)
  return (
    <>
      <div className="grid gap-4 md:grid-cols-2">
        <ModelSelect
          value={value.model_id}
          options={models}
          onChange={(modelId) => onChange({ model_id: modelId })}
        />
        <ReadonlyField label="模型名称" value={value.model_name} placeholder="选择模型ID 后自动带入" />
        <ReadonlyField label="厂商名称" value={value.vendor_name} placeholder="选择模型ID 后自动带入" />
        <Field
          label="密钥名称"
          value={value.key_name}
          placeholder="例如 deepseek-main，全局唯一"
          onChange={(next) => onChange({ key_name: next })}
        />
        <Field
          label="密钥值"
          type="password"
          value={value.key_value}
          placeholder="实际保存时会加密"
          onChange={(next) => onChange({ key_value: next })}
        />
        <Field
          label="Base URL"
          value={value.base_url}
          placeholder="https://api.deepseek.com/v1"
          onChange={(next) => onChange({ base_url: next })}
        />
        <CallTypeSelect
          value={value.call_type}
          onChange={(next) => onChange({ call_type: next })}
        />
        <Field
          label="协议类型"
          value={value.type}
          placeholder="openai"
          onChange={(next) => onChange({ type: next })}
        />
      </div>
      <label className="mt-4 block">
        <span className="mb-2 block text-xs uppercase tracking-[0.25em] text-slate-400">其他附属信息</span>
        <textarea
          value={metadataValue}
          onChange={(event) => {
            if (onMetadataChange) {
              onMetadataChange(event.target.value)
            } else {
              onChange({ metadata: parseMetadata(event.target.value) })
            }
          }}
          className="min-h-28 w-full rounded-2xl border border-white/10 bg-white/5 px-4 py-3 text-sm text-white outline-none transition focus:border-cyan-300/40"
          placeholder={'每行一项，格式 key=value'}
        />
      </label>
      <div className="mt-4 flex flex-wrap gap-3">
        <Toggle label="启用" checked={value.enabled} onChange={(checked) => onChange({ enabled: checked })} />
        <DefaultToggle
          checked={value.is_default}
          locked={isDefaultLocked && !isCreate}
          onSetDefault={onSetDefault}
        />
      </div>
    </>
  )
}

function DefaultToggle({
  checked,
  locked,
  onSetDefault,
}: {
  checked: boolean
  locked: boolean
  onSetDefault: () => void
}) {
  return (
    <button
      type="button"
      disabled={locked && !checked}
      onClick={() => {
        if (!checked) {
          onSetDefault()
        }
      }}
      className={
        checked
          ? 'inline-flex items-center gap-3 rounded-full border border-emerald-300/40 bg-emerald-500/15 px-4 py-2 text-sm text-emerald-100'
          : 'inline-flex items-center gap-3 rounded-full border border-white/10 bg-white/5 px-4 py-2 text-sm text-slate-200 transition hover:border-cyan-300/30 hover:bg-cyan-300/10 disabled:cursor-not-allowed disabled:opacity-60'
      }
    >
      <span
        className={
          checked
            ? 'inline-block h-2 w-2 rounded-full bg-emerald-300'
            : 'inline-block h-2 w-2 rounded-full bg-slate-500'
        }
      />
      {checked ? '当前默认密钥' : '设为默认'}
    </button>
  )
}

function ModelSelect({
  value,
  options,
  onChange,
}: {
  value: string
  options: LlmModel[]
  onChange: (modelId: string) => void
}) {
  return (
    <label className="block">
      <span className="mb-2 block text-xs uppercase tracking-[0.25em] text-slate-400">模型ID（绑定）</span>
      <select
        value={value}
        onChange={(event) => onChange(event.target.value)}
        className="w-full rounded-2xl border border-white/10 bg-white/5 px-4 py-3 text-sm text-white outline-none transition focus:border-cyan-300/40"
      >
        <option value="">请选择已存在的模型ID</option>
        {options.map((option) => (
          <option key={option.model_id} value={option.model_id}>
            {option.model_id} · {option.name}
          </option>
        ))}
      </select>
    </label>
  )
}

function CallTypeSelect({
  value,
  onChange,
}: {
  value: string
  onChange: (value: string) => void
}) {
  return (
    <label className="block">
      <span className="mb-2 block text-xs uppercase tracking-[0.25em] text-slate-400">调用类型</span>
      <select
        value={value}
        onChange={(event) => onChange(event.target.value)}
        className="w-full rounded-2xl border border-white/10 bg-white/5 px-4 py-3 text-sm text-white outline-none transition focus:border-cyan-300/40"
      >
        <option value="non_stream">非流式 (non_stream)</option>
        <option value="stream">流式 (stream)</option>
      </select>
    </label>
  )
}

function ReadonlyField({
  label,
  value,
  placeholder,
}: {
  label: string
  value: string
  placeholder?: string
}) {
  return (
    <label className="block">
      <span className="mb-2 block text-xs uppercase tracking-[0.25em] text-slate-400">{label}</span>
      <input
        readOnly
        value={value}
        placeholder={placeholder}
        className="w-full cursor-not-allowed rounded-2xl border border-white/10 bg-white/[0.02] px-4 py-3 text-sm text-slate-300 outline-none"
      />
    </label>
  )
}

function parseMetadata(raw: string): Record<string, string> {
  return raw
    .split('\n')
    .map((line) => line.trim())
    .filter(Boolean)
    .reduce<Record<string, string>>((acc, line) => {
      const [key, ...rest] = line.split('=')
      if (!key?.trim()) {
        return acc
      }
      acc[key.trim()] = rest.join('=').trim()
      return acc
    }, {})
}

function formatMetadata(metadata: Record<string, string>) {
  return Object.entries(metadata)
    .map(([key, value]) => `${key}=${value}`)
    .join('\n')
}

function buildSubtitle(item: LlmCredential) {
  const parts = [item.model_id, item.vendor_name, item.base_url, item.call_type, maskSecret(item.key_value)].filter(Boolean)
  if (item.is_default) parts.unshift('默认')
  return parts.join(' · ') || '未配置连接与密钥信息'
}

function maskSecret(value: string) {
  if (!value) {
    return '未配置密钥值'
  }
  if (value.length <= 8) {
    return '已录入密钥'
  }
  return `${value.slice(0, 4)}****${value.slice(-4)}`
}

function patchCredential(
  updateCredentials: (updater: (credentials: LlmCredential[]) => LlmCredential[]) => void,
  index: number,
  patch: Partial<LlmCredential>,
  models: LlmModel[],
) {
  updateCredentials((items) =>
    items.map((item, currentIndex) =>
      currentIndex === index ? mergeDraftPatch(item, patch, models) : item,
    ),
  )
}

function mergeDraftPatch(current: LlmCredential, patch: Partial<LlmCredential>, models: LlmModel[]): LlmCredential {
  const next = { ...current, ...patch }
  if (patch.model_id !== undefined) {
    const referenced = models.find((item) => item.model_id === patch.model_id)
    next.model_name = referenced?.name ?? ''
    next.vendor_name = referenced?.provider ?? ''
  }
  return next
}
