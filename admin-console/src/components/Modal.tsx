import type { ReactNode } from 'react'

import { X } from 'lucide-react'

import { cn } from '@/lib/utils'

type ModalProps = {
  open: boolean
  title: string
  description?: string
  eyebrow?: string
  onClose: () => void
  footer?: ReactNode
  children: ReactNode
}

export function Modal({ open, title, description, eyebrow = 'Create', onClose, footer, children }: ModalProps) {
  if (!open) {
    return null
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/70 px-4 py-8 backdrop-blur-sm">
      <div
        className="absolute inset-0"
        onClick={onClose}
        aria-hidden="true"
      />
      <div
        className={cn(
          'relative z-10 w-full max-w-2xl rounded-[28px] border border-white/10 bg-[#08111d] shadow-[0_30px_120px_rgba(3,8,18,0.7)]',
        )}
      >
        <div className="flex items-start justify-between gap-4 border-b border-white/10 px-6 py-5">
          <div>
            <p className="text-[10px] uppercase tracking-[0.35em] text-cyan-200/70">{eyebrow}</p>
            <h3 className="mt-2 text-xl font-semibold text-white">{title}</h3>
            {description ? <p className="mt-2 text-sm leading-6 text-slate-300">{description}</p> : null}
          </div>
          <button
            type="button"
            onClick={onClose}
            className="rounded-2xl border border-white/10 bg-white/5 p-3 text-slate-200 transition hover:bg-white/10"
          >
            <X className="h-4 w-4" />
          </button>
        </div>
        <div className="max-h-[70vh] overflow-y-auto px-6 py-5">{children}</div>
        {footer ? <div className="flex justify-end gap-3 border-t border-white/10 px-6 py-5">{footer}</div> : null}
      </div>
    </div>
  )
}
