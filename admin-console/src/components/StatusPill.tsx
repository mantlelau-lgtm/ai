import { cn } from '@/lib/utils'

type StatusPillProps = {
  status: 'ok' | 'error' | 'offline'
  label?: string
}

const styles: Record<StatusPillProps['status'], string> = {
  ok: 'border-emerald-400/30 bg-emerald-500/10 text-emerald-200',
  error: 'border-amber-400/30 bg-amber-500/10 text-amber-200',
  offline: 'border-rose-400/30 bg-rose-500/10 text-rose-200',
}

export function StatusPill({ status, label }: StatusPillProps) {
  return (
    <span
      className={cn(
        'inline-flex items-center gap-2 rounded-full border px-3 py-1 text-xs uppercase tracking-[0.24em]',
        styles[status],
      )}
    >
      <span className="h-1.5 w-1.5 rounded-full bg-current" />
      {label ?? status}
    </span>
  )
}
