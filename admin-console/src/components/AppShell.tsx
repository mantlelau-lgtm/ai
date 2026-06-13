import type { ReactNode } from 'react'

import { Activity, Bot, GitBranch, Layers3, RefreshCw, ShieldCheck } from 'lucide-react'
import { NavLink } from 'react-router-dom'

import { cn } from '@/lib/utils'
import { useAdminStore } from '@/store/useAdminStore'

const navItems = [
  { to: '/', label: '总览', icon: Activity, end: true },
  { to: '/bots', label: 'Bot', icon: Bot },
  { to: '/llm', label: 'LLM', icon: Layers3 },
  { to: '/routing', label: '路由', icon: GitBranch },
  { to: '/publish', label: '发布', icon: ShieldCheck },
]

type AppShellProps = {
  children: ReactNode
}

export function AppShell({ children }: AppShellProps) {
  const hydrate = useAdminStore((state) => state.hydrate)
  const error = useAdminStore((state) => state.error)
  const clearError = useAdminStore((state) => state.clearError)

  return (
    <div className="relative min-h-screen overflow-hidden bg-[#07111e] text-slate-100">
      <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(circle_at_top_left,rgba(18,224,255,0.18),transparent_30%),radial-gradient(circle_at_bottom_right,rgba(30,84,216,0.24),transparent_28%)]" />
      <div className="relative mx-auto flex min-h-screen max-w-[1600px] gap-6 px-6 py-6">
        <aside className="hidden w-72 shrink-0 rounded-[32px] border border-white/10 bg-slate-950/55 p-6 backdrop-blur lg:block">
          <p className="text-[10px] uppercase tracking-[0.4em] text-cyan-200/60">Admin Console</p>
          <h1 className="mt-3 font-['Rajdhani'] text-4xl font-semibold tracking-[0.08em] text-white">
            Control Grid
          </h1>
          <p className="mt-3 text-sm leading-6 text-slate-300">
            统一维护 bot、LLM 与路由配置，把分散的配置收敛到 PostgreSQL 中的单一入口。
          </p>

          <nav className="mt-8 space-y-2">
            {navItems.map((item) => {
              const Icon = item.icon
              return (
                <NavLink
                  key={item.to}
                  to={item.to}
                  end={item.end}
                  className={({ isActive }) =>
                    cn(
                      'flex items-center gap-3 rounded-2xl px-4 py-3 text-sm transition',
                      isActive
                        ? 'bg-cyan-400/15 text-white shadow-[inset_0_0_0_1px_rgba(103,232,249,0.28)]'
                        : 'text-slate-300 hover:bg-white/5 hover:text-white',
                    )
                  }
                >
                  <Icon className="h-4 w-4" />
                  {item.label}
                </NavLink>
              )
            })}
          </nav>

          <div className="mt-8 rounded-[24px] border border-cyan-300/10 bg-cyan-300/5 p-4">
            <p className="text-xs uppercase tracking-[0.3em] text-cyan-100/70">Storage Mode</p>
            <p className="mt-3 text-sm leading-6 text-slate-300">
              当前后台不做鉴权，所有配置直接写入本地 PostgreSQL。
            </p>
            <button
              type="button"
              onClick={() => {
                clearError()
                void hydrate()
              }}
              className="mt-3 inline-flex items-center gap-2 rounded-2xl border border-white/10 px-4 py-3 text-sm text-cyan-100 transition hover:border-cyan-300/30 hover:bg-cyan-400/10"
            >
              <RefreshCw className="h-4 w-4" />
              重新拉取配置
            </button>
          </div>
        </aside>

        <main className="min-w-0 flex-1">
          <header className="mb-6 rounded-[32px] border border-white/10 bg-white/6 px-6 py-5 backdrop-blur-xl">
            <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
              <div>
                <p className="text-[10px] uppercase tracking-[0.38em] text-cyan-200/70">Independent Web Admin</p>
                <h2 className="mt-2 font-['Rajdhani'] text-3xl tracking-[0.1em] text-white">
                  配置中心
                </h2>
              </div>
              <button
                type="button"
                onClick={() => {
                  clearError()
                  void hydrate()
                }}
                className="inline-flex items-center justify-center gap-2 rounded-2xl bg-cyan-300/90 px-4 py-3 text-sm font-medium text-slate-950 transition hover:bg-cyan-200"
              >
                <RefreshCw className="h-4 w-4" />
                刷新
              </button>
            </div>
            {error ? (
              <div className="mt-4 rounded-2xl border border-rose-400/20 bg-rose-500/10 px-4 py-3 text-sm text-rose-100">
                {error}
              </div>
            ) : null}
          </header>
          <div className="space-y-6">{children}</div>
        </main>
      </div>
    </div>
  )
}
