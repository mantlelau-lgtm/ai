import { useState } from 'react'

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
import type { LlmModel } from '@/types'

const emptyModel = (): LlmModel => ({
  name: '',
  model_id: '',
  provider: '',
  upstream_model: '',
  owned_by: '',
  prompt_cost_per_1k_tokens: 0,
  completion_cost_per_1k_tokens: 0,
  unit_price: 0,
  enabled: true,
})

export function LlmModelsPage() {
  const llm = useAdminStore((state) => state.bundle.llm)
  const updateModels = useAdminStore((state) => state.updateModels)
  const saveModelCard = useAdminStore((state) => state.saveModelCard)
  const deleteModelCard = useAdminStore((state) => state.deleteModelCard)
  const saving = useAdminStore((state) => state.saving)
  const [openIndex, setOpenIndex] = useState<number | null>(0)
  const [createOpen, setCreateOpen] = useState(false)
  const [draft, setDraft] = useState<LlmModel>(emptyModel())
  const [confirmIndex, setConfirmIndex] = useState<number | null>(null)

  const closeCreate = () => {
    setCreateOpen(false)
    setDraft(emptyModel())
  }

  const existingModelIds = new Set(llm.models.map((item) => item.model_id.trim().toLowerCase()))
  const draftHint = (() => {
    if (!draft.name.trim()) return '请填写模型名称'
    if (!draft.model_id.trim()) return '请填写模型ID'
    if (existingModelIds.has(draft.model_id.trim().toLowerCase())) return '模型ID 已存在'
    if (!draft.provider.trim()) return '请填写厂商名称'
    return ''
  })()
  const draftValid = !draftHint

  return (
    <div className="space-y-6">
      <Panel
        title="模型管理"
        eyebrow="Model Catalog"
        action={
          <MiniButton onClick={() => setCreateOpen(true)}>
            <Plus className="h-4 w-4" />
            新增模型
          </MiniButton>
        }
      >
        <div className="space-y-4">
          {llm.models.length === 0 ? (
            <div className="rounded-[24px] border border-dashed border-white/15 bg-white/5 px-5 py-8 text-sm text-slate-300">
              当前还没有模型，新增后可在这里维护模型ID、厂商、单价信息。
            </div>
          ) : null}
          {llm.models.map((model, index) => (
            <AccordionCard
              key={`${model.model_id}-${index}`}
              title={model.name || `未命名模型 #${index + 1}`}
              subtitle={buildModelSubtitle(model)}
              isOpen={openIndex === index}
              onToggle={() => setOpenIndex((current) => (current === index ? null : index))}
              footerActions={
                <>
                  <SaveIconButton
                    onClick={async () => {
                      await saveModelCard(model, model.model_id)
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
              <div className="grid gap-4 md:grid-cols-2">
                <Field
                  label="模型名称"
                  value={model.name}
                  placeholder="例如 DeepSeek Chat"
                  onChange={(value) => patchModel(updateModels, index, { name: value })}
                />
                <Field
                  label="模型ID（全局唯一）"
                  value={model.model_id}
                  placeholder="例如 deepseek-chat"
                  onChange={(value) => patchModel(updateModels, index, { model_id: value })}
                />
                <Field
                  label="厂商名称"
                  value={model.provider}
                  placeholder="例如 deepseek"
                  onChange={(value) => patchModel(updateModels, index, { provider: value })}
                />
                <Field
                  label="上游模型名"
                  value={model.upstream_model}
                  placeholder="例如 deepseek-chat"
                  onChange={(value) => patchModel(updateModels, index, { upstream_model: value })}
                />
                <Field
                  label="归属"
                  value={model.owned_by}
                  placeholder="例如 deepseek"
                  onChange={(value) => patchModel(updateModels, index, { owned_by: value })}
                />
                <Field
                  label="输入单价 / 1k tokens"
                  type="number"
                  value={String(model.prompt_cost_per_1k_tokens)}
                  onChange={(value) =>
                    patchModel(updateModels, index, {
                      prompt_cost_per_1k_tokens: Number(value) || 0,
                    })
                  }
                />
                <Field
                  label="输出单价 / 1k tokens"
                  type="number"
                  value={String(model.completion_cost_per_1k_tokens)}
                  onChange={(value) =>
                    patchModel(updateModels, index, {
                      completion_cost_per_1k_tokens: Number(value) || 0,
                    })
                  }
                />
                <Field
                  label="统一单价 (unit_price)"
                  type="number"
                  value={String(model.unit_price)}
                  onChange={(value) =>
                    patchModel(updateModels, index, {
                      unit_price: Number(value) || 0,
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
            </AccordionCard>
          ))}
        </div>
      </Panel>

      <Modal
        open={createOpen}
        title="新增模型"
        description="录入模型ID、厂商和单价信息，模型ID必须全局唯一。"
        onClose={closeCreate}
        footer={
          <>
            <SecondaryButton onClick={closeCreate} disabled={saving}>
              取消
            </SecondaryButton>
            <MiniButton
              disabled={saving || !draftValid}
              onClick={async () => {
                const insertIndex = llm.models.length
                updateModels((items) => [...items, draft])
                setOpenIndex(insertIndex)
                const ok = await saveModelCard(draft)
                if (ok) {
                  closeCreate()
                } else {
                  updateModels((items) => items.filter((_, currentIndex) => currentIndex !== insertIndex))
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
        <div className="grid gap-4 md:grid-cols-2">
          <Field
            label="模型名称"
            value={draft.name}
            placeholder="例如 DeepSeek Chat"
            onChange={(value) => setDraft((current) => ({ ...current, name: value }))}
          />
          <Field
            label="模型ID"
            value={draft.model_id}
            placeholder="例如 deepseek-chat"
            onChange={(value) => setDraft((current) => ({ ...current, model_id: value }))}
          />
          <Field
            label="厂商名称"
            value={draft.provider}
            placeholder="例如 deepseek"
            onChange={(value) => setDraft((current) => ({ ...current, provider: value }))}
          />
          <Field
            label="上游模型名"
            value={draft.upstream_model}
            placeholder="例如 deepseek-chat"
            onChange={(value) => setDraft((current) => ({ ...current, upstream_model: value }))}
          />
          <Field
            label="归属"
            value={draft.owned_by}
            placeholder="例如 deepseek"
            onChange={(value) => setDraft((current) => ({ ...current, owned_by: value }))}
          />
          <Field
            label="输入单价 / 1k tokens"
            type="number"
            value={String(draft.prompt_cost_per_1k_tokens)}
            onChange={(value) =>
              setDraft((current) => ({ ...current, prompt_cost_per_1k_tokens: Number(value) || 0 }))
            }
          />
          <Field
            label="输出单价 / 1k tokens"
            type="number"
            value={String(draft.completion_cost_per_1k_tokens)}
            onChange={(value) =>
              setDraft((current) => ({ ...current, completion_cost_per_1k_tokens: Number(value) || 0 }))
            }
          />
          <Field
            label="统一单价 (unit_price)"
            type="number"
            value={String(draft.unit_price)}
            onChange={(value) =>
              setDraft((current) => ({ ...current, unit_price: Number(value) || 0 }))
            }
          />
        </div>
        <div className="mt-4">
          <Toggle
            label="启用"
            checked={draft.enabled}
            onChange={(checked) => setDraft((current) => ({ ...current, enabled: checked }))}
          />
        </div>
      </Modal>

      <ConfirmDialog
        open={confirmIndex !== null}
        title="删除模型确认"
        description={
          confirmIndex !== null && llm.models[confirmIndex]
            ? `确认删除模型「${llm.models[confirmIndex].name || '未命名模型'}」吗？删除后将立即写入数据库。`
            : '确认删除该模型吗？'
        }
        confirmText="删除"
        tone="danger"
        busy={saving}
        onCancel={() => setConfirmIndex(null)}
        onConfirm={async () => {
          if (confirmIndex === null) return
          const target = confirmIndex
          const modelId = llm.models[target]?.model_id
          if (!modelId) return
          const ok = await deleteModelCard(modelId)
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

function buildModelSubtitle(model: LlmModel) {
  const parts = [
    model.model_id,
    model.provider,
    `${model.prompt_cost_per_1k_tokens}/${model.completion_cost_per_1k_tokens}`,
  ]
    .map((item) => item?.toString().trim())
    .filter(Boolean)
  return parts.join(' · ') || '未配置模型ID 与单价信息'
}

function patchModel(
  updateModels: (updater: (models: LlmModel[]) => LlmModel[]) => void,
  index: number,
  patch: Partial<LlmModel>,
) {
  updateModels((items) => items.map((item, currentIndex) => (currentIndex === index ? { ...item, ...patch } : item)))
}
