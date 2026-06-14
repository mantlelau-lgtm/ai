import { useEffect, useMemo, useState } from 'react'

import { LayoutList, Save, Zap } from 'lucide-react'

import { AccordionCard } from '@/components/AccordionCard'
import { SaveIconButton } from '@/components/ConfigControls'
import { Panel } from '@/components/Panel'
import { useAdminStore } from '@/store/useAdminStore'

export function AgentsPage() {
  const agents = useAdminStore((state) => state.agents)
  const credentials = useAdminStore((state) => state.bundle.llm.credentials)
  const updateAgentKey = useAdminStore((state) => state.updateAgentKey)
  const saving = useAdminStore((state) => state.saving)
  const [drafts, setDrafts] = useState<Record<string, string>>({})
  const [pendingName, setPendingName] = useState<string | null>(null)
  const [openIndex, setOpenIndex] = useState<number | null>(0)

  const keyOptions = useMemo(
    () => credentials.map((item) => item.key_name).filter((name) => name.trim().length > 0),
    [credentials],
  )

  useEffect(() => {
    setDrafts((current) => {
      const next: Record<string, string> = {}
      for (const agent of agents) {
        next[agent.name] = current[agent.name] ?? agent.key_name ?? ''
      }
      return next
    })
  }, [agents])

  return (
    <div className="space-y-6">
      <Panel title="Agent 注册列表" eyebrow="Registered Agents">
        <div className="space-y-4">
          {agents.length === 0 ? (
            <div className="rounded-[24px] border border-dashed border-white/15 bg-white/5 px-5 py-8 text-sm text-slate-300">
              当前还没有注册 agent，请等待 core-service 启动后自动注册。
            </div>
          ) : null}
          {agents.map((agent, index) => {
            const draftValue = drafts[agent.name] ?? agent.key_name ?? ''
            const dirty = draftValue !== (agent.key_name ?? '')
            const Icon = agent.type === 'general' ? Zap : LayoutList
            return (
              <AccordionCard
                key={agent.name}
                title={agent.name}
                subtitle={buildAgentSubtitle(agent.type, agent.source, agent.key_name)}
                isOpen={openIndex === index}
                onToggle={() => setOpenIndex((current) => (current === index ? null : index))}
                footerActions={
                  <SaveIconButton
                    disabled={saving || !dirty}
                    onClick={async () => {
                      setPendingName(agent.name)
                      await updateAgentKey(agent.name, draftValue)
                      setPendingName(null)
                    }}
                  >
                    <Save className="h-4 w-4" />
                  </SaveIconButton>
                }
              >
                <div className="flex items-start gap-4">
                  <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-2xl bg-cyan-400/10 text-cyan-200">
                    <Icon className="h-5 w-5" />
                  </div>
                  <div className="min-w-0 flex-1">
                    <p className="text-xs leading-6 text-slate-300">{agent.description || '无描述'}</p>
                    <div className="mt-3 flex flex-wrap gap-3">
                      <span className="rounded-full border border-white/10 px-3 py-1 text-[10px] uppercase tracking-[0.2em] text-slate-400">
                        {agent.type}
                      </span>
                      <span className="rounded-full border border-cyan-300/20 bg-cyan-300/5 px-3 py-1 text-[10px] uppercase tracking-[0.2em] text-cyan-200/70">
                        {agent.source}
                      </span>
                      <span className="rounded-full border border-emerald-300/20 bg-emerald-300/5 px-3 py-1 text-[10px] uppercase tracking-[0.2em] text-emerald-100/80">
                        当前密钥 {agent.key_name || '未设置'}
                      </span>
                    </div>
                  </div>
                </div>
                <label className="mt-5 block">
                  <span className="mb-2 block text-xs uppercase tracking-[0.25em] text-slate-400">使用的密钥</span>
                  <select
                    value={draftValue}
                    onChange={(event) =>
                      setDrafts((current) => ({ ...current, [agent.name]: event.target.value }))
                    }
                    className="w-full rounded-2xl border border-white/10 bg-white/5 px-4 py-3 text-sm text-white outline-none transition focus:border-cyan-300/40"
                  >
                    <option value="">未设置（沿用 llm-gw 默认密钥）</option>
                    {keyOptions.map((option) => (
                      <option key={option} value={option}>
                        {option}
                      </option>
                    ))}
                  </select>
                  {keyOptions.length === 0 ? (
                    <p className="mt-2 text-xs text-amber-200/80">请先到「密钥管理」新增密钥。</p>
                  ) : null}
                </label>
                {saving && pendingName === agent.name ? (
                  <p className="mt-3 text-xs text-cyan-200/80">保存中...</p>
                ) : null}
              </AccordionCard>
            )
          })}
        </div>
      </Panel>
    </div>
  )
}

function buildAgentSubtitle(type: string, source: string, keyName: string) {
  const parts = [type, source, keyName ? `key: ${keyName}` : '未设置密钥'].filter(Boolean)
  return parts.join(' · ')
}
