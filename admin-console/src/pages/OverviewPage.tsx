import { Server, ShieldCheck, Sparkles, Waypoints } from 'lucide-react'

import { Panel } from '@/components/Panel'
import { StatusPill } from '@/components/StatusPill'
import { useAdminStore } from '@/store/useAdminStore'

const icons = [ShieldCheck, Sparkles, Waypoints, Server]

export function OverviewPage() {
  const loading = useAdminStore((state) => state.loading)
  const overview = useAdminStore((state) => state.overview)
  const storage = useAdminStore((state) => state.storage)

  const stats = [
    { label: 'Bot 数量', value: overview?.summary.bot_count ?? 0 },
    { label: 'Provider 数量', value: overview?.summary.provider_count ?? 0 },
    { label: '模型数量', value: overview?.summary.model_count ?? 0 },
    { label: '路由映射', value: overview?.summary.route_count ?? 0 },
  ]

  return (
    <div className="space-y-6">
      <div className="grid gap-4 lg:grid-cols-4">
        {stats.map((item, index) => {
          const Icon = icons[index]
          return (
            <Panel key={item.label} title={item.label} className="overflow-hidden">
              <div className="flex items-center justify-between">
                <div>
                  <div className="text-4xl font-semibold text-white">{item.value}</div>
                  <p className="mt-2 text-sm text-slate-300">当前已接入配置对象</p>
                </div>
                <div className="rounded-2xl border border-cyan-300/20 bg-cyan-300/10 p-3 text-cyan-100">
                  <Icon className="h-5 w-5" />
                </div>
              </div>
            </Panel>
          )
        })}
      </div>

      <div className="grid gap-6 xl:grid-cols-[1.2fr_0.8fr]">
        <Panel title="服务状态" eyebrow="Runtime">
          <div className="grid gap-4">
            {loading && !overview ? (
              <div className="rounded-3xl border border-white/10 bg-white/5 p-5 text-sm text-slate-300">
                正在拉取服务状态...
              </div>
            ) : null}
            {overview?.services.map((service) => (
              <div
                key={service.name}
                className="rounded-[24px] border border-white/10 bg-slate-950/45 p-5"
              >
                <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
                  <div>
                    <div className="text-base font-medium text-white">{service.name}</div>
                    <div className="mt-2 break-all font-mono text-xs text-slate-400">{service.url}</div>
                  </div>
                  <StatusPill status={service.status} />
                </div>
                <div className="mt-4 rounded-2xl border border-white/10 bg-white/5 px-4 py-3 font-mono text-xs text-cyan-50/90">
                  {service.detail || '无返回详情'}
                </div>
              </div>
            ))}
          </div>
        </Panel>

        <Panel title="数据库摘要" eyebrow="PostgreSQL">
          <div className="space-y-4">
            {[
              { label: '存储引擎', value: overview?.database.engine || storage.engine || 'postgresql' },
              { label: '数据库名', value: overview?.database.database || storage.database || '未加载' },
              { label: '连接状态', value: overview?.database.detail || '未加载' },
            ].map((item) => (
              <div key={item.label} className="rounded-[24px] border border-white/10 bg-slate-950/45 p-4">
                <p className="text-xs uppercase tracking-[0.3em] text-cyan-200/65">{item.label}</p>
                <p className="mt-3 break-all font-mono text-xs leading-6 text-slate-300">{item.value}</p>
              </div>
            ))}
            {overview?.tables ? (
              <div className="rounded-[24px] border border-white/10 bg-slate-950/45 p-4">
                <p className="text-xs uppercase tracking-[0.3em] text-cyan-200/65">核心表</p>
                <div className="mt-3 space-y-2 font-mono text-xs leading-6 text-slate-300">
                  {Object.values(overview.tables).map((table) => (
                    <div key={table.name} className="flex items-center justify-between gap-4">
                      <span>{table.name}</span>
                      <span>{table.rows} rows</span>
                    </div>
                  ))}
                </div>
              </div>
            ) : null}
          </div>
        </Panel>
      </div>
    </div>
  )
}
