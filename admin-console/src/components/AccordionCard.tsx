import type { ReactNode } from 'react'

import { ChevronDown } from 'lucide-react'

import { cn } from '@/lib/utils'

type AccordionCardProps = {
  title: string
  subtitle?: string
  isOpen: boolean
  onToggle: () => void
  actions?: ReactNode
  footerActions?: ReactNode
  children: ReactNode
}

export function AccordionCard({
  title,
  subtitle,
  isOpen,
  onToggle,
  actions,
  footerActions,
  children,
}: AccordionCardProps) {
  return (
    <div
      className={cn(
        'rounded-[24px] border bg-slate-950/45 transition-colors',
        isOpen ? 'border-white' : 'border-white/10',
      )}
    >
      <div className="flex items-center gap-3 px-4 py-4">
        <button
          type="button"
          onClick={onToggle}
          className="flex min-w-0 flex-1 items-center gap-3 text-left"
        >
          <div className="min-w-0 flex-1">
            <div className="truncate text-sm font-medium text-white">{title}</div>
            {subtitle ? <div className="mt-1 truncate text-xs text-slate-400">{subtitle}</div> : null}
          </div>
          <ChevronDown
            className={cn('h-4 w-4 shrink-0 text-slate-400 transition', isOpen && 'rotate-180 text-cyan-200')}
          />
        </button>
        {actions ? <div className="shrink-0">{actions}</div> : null}
      </div>
      {isOpen ? (
        <div className="border-t border-white/10 px-4 py-4">
          {children}
          {footerActions ? <div className="mt-4 flex justify-end gap-3">{footerActions}</div> : null}
        </div>
      ) : null}
    </div>
  )
}
