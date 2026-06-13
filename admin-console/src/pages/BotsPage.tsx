import { Plus, Trash2 } from 'lucide-react'

import { CodePanel } from '@/components/CodePanel'
import { Panel } from '@/components/Panel'
import type { BotConfig } from '@/types'
import { useAdminStore } from '@/store/useAdminStore'

const emptyBot = (): BotConfig => ({
  bot_id: '',
  app_id: '',
  app_secret: '',
  open_base_url: 'https://open.feishu.cn/',
})

export function BotsPage() {
  const bots = useAdminStore((state) => state.bundle.bots.bots)
  const routes = useAdminStore((state) => state.bundle.routing.bots)
  const updateBots = useAdminStore((state) => state.updateBots)

  return (
    <div className="grid gap-6 xl:grid-cols-[1.1fr_0.9fr]">
      <Panel
        title="Bot 配置"
        eyebrow="Message Gateway"
        action={
          <button
            type="button"
            onClick={() => updateBots((items) => [...items, emptyBot()])}
            className="inline-flex items-center gap-2 rounded-2xl bg-cyan-300/90 px-4 py-3 text-sm font-medium text-slate-950 transition hover:bg-cyan-200"
          >
            <Plus className="h-4 w-4" />
            新增 Bot
          </button>
        }
      >
        <div className="space-y-4">
          {bots.length === 0 ? (
            <div className="rounded-[24px] border border-dashed border-white/15 bg-white/5 px-5 py-8 text-sm text-slate-300">
              当前还没有 Bot，新增后即可在路由页把多个 bot 指向同一个 agent。
            </div>
          ) : null}
          {bots.map((bot, index) => {
            const linkedRoute = routes.find((item) => item.bot_id === bot.bot_id)
            return (
              <div key={`${bot.bot_id}-${index}`} className="rounded-[28px] border border-white/10 bg-slate-950/45 p-5">
                <div className="mb-4 flex items-center justify-between">
                  <div>
                    <p className="text-xs uppercase tracking-[0.3em] text-cyan-200/65">Bot #{index + 1}</p>
                    <p className="mt-2 text-sm text-slate-300">
                      路由目标：{linkedRoute?.agent_name || '未绑定 agent'}
                    </p>
                  </div>
                  <button
                    type="button"
                    onClick={() =>
                      updateBots((items) => items.filter((_, currentIndex) => currentIndex !== index))
                    }
                    className="rounded-2xl border border-rose-300/20 bg-rose-500/10 p-3 text-rose-100 transition hover:bg-rose-500/15"
                  >
                    <Trash2 className="h-4 w-4" />
                  </button>
                </div>
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
              </div>
            )
          })}
        </div>
      </Panel>

      <Panel title="数据库快照" eyebrow="admin_bots">
        <CodePanel code={JSON.stringify({ bots }, null, 2)} />
      </Panel>
    </div>
  )
}

type FieldProps = {
  label: string
  value: string
  onChange: (value: string) => void
}

function Field({ label, value, onChange }: FieldProps) {
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

function patchBot(
  updateBots: (updater: (bots: BotConfig[]) => BotConfig[]) => void,
  index: number,
  patch: Partial<BotConfig>,
) {
  updateBots((items) => items.map((item, currentIndex) => (currentIndex === index ? { ...item, ...patch } : item)))
}
