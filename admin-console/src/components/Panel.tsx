import type { ReactNode } from 'react'

import { cn } from '@/lib/utils'

type PanelProps = {
  title: string
  eyebrow?: string
  action?: ReactNode
  className?: string
  children: ReactNode
}

export function Panel({ title, eyebrow, action, className, children }: PanelProps) {
  return (
    <section
      className={cn(
        'rounded-[28px] border border-white/10 bg-white/6 p-6 shadow-[0_24px_80px_rgba(4,12,24,0.35)] backdrop-blur-xl',
        className,
      )}
    >
      <div className="mb-5 flex items-start justify-between gap-4">
        <div>
          {eyebrow ? (
            <p className="mb-2 text-[10px] uppercase tracking-[0.35em] text-cyan-200/70">{eyebrow}</p>
          ) : null}
          <h2 className="text-lg font-semibold text-white">{title}</h2>
        </div>
        {action}
      </div>
      {children}
    </section>
  )
}
