import type { ReactNode } from 'react'

import { AlertTriangle, CheckCircle2, RefreshCw, Save } from 'lucide-react'

import { CodePanel } from '@/components/CodePanel'
import { Panel } from '@/components/Panel'
import { useAdminStore } from '@/store/useAdminStore'

export function PublishPage() {
  const validation = useAdminStore((state) => state.validation)
  const diff = useAdminStore((state) => state.diff)
  const loading = useAdminStore((state) => state.loading)
  const saving = useAdminStore((state) => state.saving)
  const previewDiff = useAdminStore((state) => state.previewDiff)
  const validateDraft = useAdminStore((state) => state.validateDraft)
  const applyDraft = useAdminStore((state) => state.applyDraft)

  return (
    <div className="space-y-6">
      <Panel
        title="发布中心"
        eyebrow="Validate / Diff / Apply"
        action={
          <div className="flex flex-wrap gap-3">
            <ActionButton onClick={() => void validateDraft()} disabled={loading}>
              <RefreshCw className="h-4 w-4" />
              先校验
            </ActionButton>
            <ActionButton onClick={() => void previewDiff()} disabled={loading}>
              <AlertTriangle className="h-4 w-4" />
              生成 Diff
            </ActionButton>
            <button
              type="button"
              onClick={() => void applyDraft()}
              disabled={saving}
              className="inline-flex items-center gap-2 rounded-2xl bg-cyan-300/90 px-4 py-3 text-sm font-medium text-slate-950 transition hover:bg-cyan-200 disabled:cursor-not-allowed disabled:opacity-60"
            >
              <Save className="h-4 w-4" />
              一键保存
            </button>
          </div>
        }
      >
        <div className="grid gap-6 lg:grid-cols-2">
          <ResultList title="错误" items={validation?.errors ?? []} tone="error" />
          <ResultList title="警告" items={validation?.warnings ?? []} tone="warning" />
        </div>
      </Panel>

      <div className="grid gap-6 xl:grid-cols-3">
        <Panel title="Bot Diff" eyebrow="admin_bots">
          <CodePanel code={diff?.bots_diff ?? '先点击“生成 Diff”'} heightClassName="h-[28rem]" />
        </Panel>
        <Panel title="LLM Diff" eyebrow="admin_llm_*">
          <CodePanel code={diff?.llm_diff ?? '先点击“生成 Diff”'} heightClassName="h-[28rem]" />
        </Panel>
        <Panel title="Routing Diff" eyebrow="admin_routes">
          <CodePanel code={diff?.routing_diff ?? '先点击“生成 Diff”'} heightClassName="h-[28rem]" />
        </Panel>
      </div>
    </div>
  )
}

function ActionButton({
  children,
  disabled,
  onClick,
}: {
  children: ReactNode
  disabled?: boolean
  onClick: () => void
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      className="inline-flex items-center gap-2 rounded-2xl border border-white/10 bg-white/5 px-4 py-3 text-sm text-slate-100 transition hover:border-cyan-300/30 hover:bg-cyan-300/10 disabled:cursor-not-allowed disabled:opacity-60"
    >
      {children}
    </button>
  )
}

function ResultList({
  title,
  items,
  tone,
}: {
  title: string
  items: string[]
  tone: 'error' | 'warning'
}) {
  const isError = tone === 'error'
  return (
    <div className="rounded-[24px] border border-white/10 bg-slate-950/45 p-5">
      <div className="mb-4 flex items-center gap-3 text-white">
        {isError ? <AlertTriangle className="h-4 w-4 text-rose-200" /> : <CheckCircle2 className="h-4 w-4 text-amber-200" />}
        <span>{title}</span>
      </div>
      <div className="space-y-3 text-sm text-slate-300">
        {items.length === 0 ? <p>当前没有{title === '错误' ? '阻塞性问题' : '预警信息'}。</p> : null}
        {items.map((item) => (
          <div
            key={item}
            className={`rounded-2xl border px-4 py-3 ${
              isError
                ? 'border-rose-300/20 bg-rose-500/10 text-rose-50'
                : 'border-amber-300/20 bg-amber-500/10 text-amber-50'
            }`}
          >
            {item}
          </div>
        ))}
      </div>
    </div>
  )
}
