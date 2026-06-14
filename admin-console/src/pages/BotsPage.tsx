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
} from '@/components/ConfigControls'
import { Modal } from '@/components/Modal'
import { Panel } from '@/components/Panel'
import type { BotConfig } from '@/types'
import { useAdminStore } from '@/store/useAdminStore'

const emptyBot = (): BotConfig => ({
  bot_id: '',
  app_id: '',
  app_secret: '',
  open_base_url: 'https://open.feishu.cn/',
  agent_name: '',
})

export function BotsPage() {
  const bots = useAdminStore((state) => state.bundle.bots.bots)
  const agents = useAdminStore((state) => state.agents)
  const updateBots = useAdminStore((state) => state.updateBots)
  const saveDraft = useAdminStore((state) => state.saveDraft)
  const saving = useAdminStore((state) => state.saving)
  const [openIndex, setOpenIndex] = useState<number | null>(0)
  const [createOpen, setCreateOpen] = useState(false)
  const [draft, setDraft] = useState<BotConfig>(emptyBot())
  const [confirmIndex, setConfirmIndex] = useState<number | null>(null)

  const agentOptions = agents.map((item) => item.name).filter((name) => name.trim().length > 0)

  const closeCreate = () => {
    setCreateOpen(false)
    setDraft(emptyBot())
  }

  return (
    <div className="space-y-6">
      <Panel
        title="Bot 配置"
        eyebrow="Message Gateway"
        action={
          <MiniButton onClick={() => setCreateOpen(true)}>
            <Plus className="h-4 w-4" />
            新增 Bot
          </MiniButton>
        }
      >
        <div className="space-y-4">
          {bots.length === 0 ? (
            <div className="rounded-[24px] border border-dashed border-white/15 bg-white/5 px-5 py-8 text-sm text-slate-300">
              当前还没有 Bot，新增后即可选择已注册的 agent。
            </div>
          ) : null}
          {bots.map((bot, index) => (
            <AccordionCard
              key={`${bot.bot_id}-${index}`}
              title={bot.bot_id || `未命名 Bot #${index + 1}`}
              subtitle={buildBotSubtitle(bot)}
              isOpen={openIndex === index}
              onToggle={() => setOpenIndex((current) => (current === index ? null : index))}
              footerActions={
                <>
                  <SaveIconButton
                    onClick={async () => {
                      await saveDraft('Bot 配置已保存并生效')
                    }}
                    disabled={saving}
                  >
                    <Save className="h-4 w-4" />
                  </SaveIconButton>
                  <DeleteIconButton
                    disabled={saving}
                    onClick={() => setConfirmIndex(index)}
                  >
                    <Trash2 className="h-4 w-4" />
                  </DeleteIconButton>
                </>
              }
            >
              <div className="grid gap-4 md:grid-cols-2">
                <Field
                  label="bot_id"
                  value={bot.bot_id}
                  onChange={(value) => patchBot(updateBots, index, { bot_id: value })}
                />
                <Field
                  label="app_id"
                  value={bot.app_id}
                  onChange={(value) => patchBot(updateBots, index, { app_id: value })}
                />
                <Field
                  label="app_secret"
                  value={bot.app_secret}
                  onChange={(value) => patchBot(updateBots, index, { app_secret: value })}
                />
                <Field
                  label="open_base_url"
                  value={bot.open_base_url}
                  onChange={(value) => patchBot(updateBots, index, { open_base_url: value })}
                />
              </div>
              <div className="mt-4">
                <SelectField
                  label="agent_name"
                  value={bot.agent_name}
                  options={agentOptions}
                  placeholder="选择已注册的 agent"
                  onChange={(value) => patchBot(updateBots, index, { agent_name: value })}
                />
              </div>
            </AccordionCard>
          ))}
        </div>
      </Panel>

      <Modal
        open={createOpen}
        title="新增 Bot"
        description="录入 Bot 基础信息并选择 agent，确认后立即保存生效。"
        onClose={closeCreate}
        footer={
          <>
            <SecondaryButton onClick={closeCreate} disabled={saving}>
              取消
            </SecondaryButton>
            <MiniButton
              disabled={saving}
              onClick={async () => {
                const insertIndex = bots.length
                updateBots((items) => [...items, draft])
                setOpenIndex(insertIndex)
                const ok = await saveDraft('Bot 已新增并生效')
                if (ok) {
                  closeCreate()
                } else {
                  updateBots((items) => items.filter((_, currentIndex) => currentIndex !== insertIndex))
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
        <div className="grid gap-4 md:grid-cols-2">
          <Field label="bot_id" value={draft.bot_id} onChange={(value) => setDraft((current) => ({ ...current, bot_id: value }))} />
          <Field label="app_id" value={draft.app_id} onChange={(value) => setDraft((current) => ({ ...current, app_id: value }))} />
          <Field
            label="app_secret"
            type="password"
            value={draft.app_secret}
            onChange={(value) => setDraft((current) => ({ ...current, app_secret: value }))}
          />
          <Field
            label="open_base_url"
            value={draft.open_base_url}
            onChange={(value) => setDraft((current) => ({ ...current, open_base_url: value }))}
          />
        </div>
        <div className="mt-4">
          <SelectField
            label="agent_name"
            value={draft.agent_name}
            options={agentOptions}
            placeholder="选择已注册的 agent"
            onChange={(value) => setDraft((current) => ({ ...current, agent_name: value }))}
          />
        </div>
      </Modal>

      <ConfirmDialog
        open={confirmIndex !== null}
        title="删除 Bot 确认"
        description={
          confirmIndex !== null && bots[confirmIndex]
            ? `确认删除 Bot「${bots[confirmIndex].bot_id || '未命名 Bot'}」吗？删除后将立即写入数据库。`
            : '确认删除该 Bot 吗？'
        }
        confirmText="删除"
        tone="danger"
        busy={saving}
        onCancel={() => setConfirmIndex(null)}
        onConfirm={async () => {
          if (confirmIndex === null) return
          const target = confirmIndex
          updateBots((items) => items.filter((_, currentIndex) => currentIndex !== target))
          setOpenIndex((current) => {
            if (current === target) {
              return null
            }
            if (current !== null && current > target) {
              return current - 1
            }
            return current
          })
          const ok = await saveDraft('Bot 已删除并生效')
          if (ok) {
            setConfirmIndex(null)
          }
        }}
      />
    </div>
  )
}

function buildBotSubtitle(bot: BotConfig) {
  const parts = [bot.app_id, bot.agent_name ? `agent: ${bot.agent_name}` : '', bot.open_base_url].filter(Boolean)
  return parts.join(' · ') || '未配置 Bot 信息'
}

function patchBot(
  updateBots: (updater: (bots: BotConfig[]) => BotConfig[]) => void,
  index: number,
  patch: Partial<BotConfig>,
) {
  updateBots((items) => items.map((item, currentIndex) => (currentIndex === index ? { ...item, ...patch } : item)))
}

function SelectField({
  label,
  value,
  options,
  placeholder,
  onChange,
}: {
  label: string
  value: string
  options: string[]
  placeholder?: string
  onChange: (value: string) => void
}) {
  return (
    <label className="block">
      <span className="mb-2 block text-xs uppercase tracking-[0.25em] text-slate-400">{label}</span>
      <select
        value={value}
        onChange={(event) => onChange(event.target.value)}
        className="w-full rounded-2xl border border-white/10 bg-white/5 px-4 py-3 text-sm text-white outline-none transition focus:border-cyan-300/40"
      >
        <option value="">{placeholder || '请选择'}</option>
        {options.map((option) => (
          <option key={option} value={option}>
            {option}
          </option>
        ))}
      </select>
    </label>
  )
}
