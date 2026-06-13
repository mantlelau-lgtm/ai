import { Plus, Trash2 } from 'lucide-react'

import { CodePanel } from '@/components/CodePanel'
import { Panel } from '@/components/Panel'
import { useAdminStore } from '@/store/useAdminStore'
import type { RoutingEntry } from '@/types'

const emptyRoute = (): RoutingEntry => ({ bot_id: '', agent_name: 'general' })

export function RoutingPage() {
  const routing = useAdminStore((state) => state.bundle.routing)
  const knownBots = useAdminStore((state) => state.bundle.bots.bots)
  const updateRoutes = useAdminStore((state) => state.updateRoutes)
  const updateDefaultAgent = useAdminStore((state) => state.updateDefaultAgent)

  return (
    <div className="grid gap-6 xl:grid-cols-[1.05fr_0.95fr]">
      <Panel
        title="路由配置"
        eyebrow="Bot -> Agent"
        action={
          <button
            type="button"
            onClick={() => updateRoutes((items) => [...items, emptyRoute()])}
            className="inline-flex items-center gap-2 rounded-2xl bg-cyan-300/90 px-4 py-3 text-sm font-medium text-slate-950 transition hover:bg-cyan-200"
          >
            <Plus className="h-4 w-4" />
            新增路由
          </button>
        }
      >
        <label className="block rounded-[24px] border border-white/10 bg-slate-950/45 p-4">
          <span className="mb-2 block text-xs uppercase tracking-[0.25em] text-slate-400">default_agent</span>
          <input
            value={routing.default_agent}
            onChange={(event) => updateDefaultAgent(event.target.value)}
            className="w-full rounded-2xl border border-white/10 bg-white/5 px-4 py-3 text-sm text-white outline-none transition focus:border-cyan-300/40"
          />
        </label>

        <div className="mt-5 space-y-4">
          {routing.bots.map((route, index) => {
            const botExists = knownBots.some((bot) => bot.bot_id === route.bot_id)
            return (
              <div key={`${route.bot_id}-${index}`} className="rounded-[24px] border border-white/10 bg-slate-950/45 p-4">
                <div className="mb-4 flex items-center justify-between">
                  <div>
                    <p className="text-sm text-white">Route #{index + 1}</p>
                    <p className="mt-2 text-xs text-slate-400">
                      {botExists ? '已关联已知 Bot' : '未在 Bot 配置中找到该 bot_id'}
                    </p>
                  </div>
                  <button
                    type="button"
                    onClick={() =>
                      updateRoutes((items) => items.filter((_, currentIndex) => currentIndex !== index))
                    }
                    className="rounded-2xl border border-rose-300/20 bg-rose-500/10 p-3 text-rose-100 transition hover:bg-rose-500/15"
                  >
                    <Trash2 className="h-4 w-4" />
                  </button>
                </div>
                <div className="grid gap-4 md:grid-cols-2">
                  <Field
                    label="bot_id"
                    value={route.bot_id}
                    onChange={(value) => patchRoute(updateRoutes, index, { bot_id: value })}
                  />
                  <Field
                    label="agent_name"
                    value={route.agent_name}
                    onChange={(value) => patchRoute(updateRoutes, index, { agent_name: value })}
                  />
                </div>
              </div>
            )
          })}
        </div>
      </Panel>

      <div className="space-y-6">
        <Panel title="路由说明" eyebrow="Fallback Logic">
          <div className="space-y-4 text-sm leading-7 text-slate-300">
            <p>未配置 bot 的请求会由 `core-service` 返回固定文案：`当前 bot 没有配置对应的agent。`</p>
            <p>多个 bot 可以配置到同一个 agent，这一页的每一行都表示一次显式映射。</p>
            <p>如果先写路由、后补 bot，本页会提示警告，但仍允许提前规划上线配置。</p>
          </div>
        </Panel>

        <Panel title="数据库快照" eyebrow="admin_routes">
          <CodePanel code={JSON.stringify(routing, null, 2)} />
        </Panel>
      </div>
    </div>
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

function patchRoute(
  updateRoutes: (updater: (routes: RoutingEntry[]) => RoutingEntry[]) => void,
  index: number,
  patch: Partial<RoutingEntry>,
) {
  updateRoutes((items) => items.map((item, currentIndex) => (currentIndex === index ? { ...item, ...patch } : item)))
}
