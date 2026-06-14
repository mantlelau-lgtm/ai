import type { ButtonHTMLAttributes } from 'react'

export function MiniButton(props: ButtonHTMLAttributes<HTMLButtonElement>) {
  return (
    <button
      type="button"
      {...props}
      className="inline-flex items-center gap-2 rounded-2xl bg-cyan-300/90 px-4 py-3 text-sm font-medium text-slate-950 transition hover:bg-cyan-200 disabled:cursor-not-allowed disabled:opacity-60"
    />
  )
}

export function DangerButton(props: ButtonHTMLAttributes<HTMLButtonElement>) {
  return (
    <button
      type="button"
      {...props}
      className="rounded-2xl border border-rose-300/20 bg-rose-500/10 p-3 text-rose-100 transition hover:bg-rose-500/15 disabled:cursor-not-allowed disabled:opacity-60"
    />
  )
}

export function SaveIconButton(props: ButtonHTMLAttributes<HTMLButtonElement>) {
  return (
    <button
      type="button"
      aria-label="保存"
      title="保存"
      {...props}
      className="inline-flex h-11 w-11 items-center justify-center rounded-2xl border border-cyan-300/40 bg-cyan-300/90 text-slate-950 shadow-[0_0_0_2px_rgba(34,211,238,0.18)] transition hover:bg-cyan-200 disabled:cursor-not-allowed disabled:opacity-60"
    />
  )
}

export function DeleteIconButton(props: ButtonHTMLAttributes<HTMLButtonElement>) {
  return (
    <button
      type="button"
      aria-label="删除"
      title="删除"
      {...props}
      className="inline-flex h-11 w-11 items-center justify-center rounded-2xl border border-rose-300/40 bg-rose-500/90 text-white shadow-[0_0_0_2px_rgba(244,63,94,0.18)] transition hover:bg-rose-400 disabled:cursor-not-allowed disabled:opacity-60"
    />
  )
}

export function SecondaryButton(props: ButtonHTMLAttributes<HTMLButtonElement>) {
  return (
    <button
      type="button"
      {...props}
      className="inline-flex items-center gap-2 rounded-2xl border border-white/10 bg-white/5 px-4 py-3 text-sm text-slate-100 transition hover:border-cyan-300/30 hover:bg-cyan-300/10 disabled:cursor-not-allowed disabled:opacity-60"
    />
  )
}

export function Toggle({
  label,
  checked,
  onChange,
}: {
  label: string
  checked: boolean
  onChange: (checked: boolean) => void
}) {
  return (
    <label className="inline-flex items-center gap-3 rounded-full border border-white/10 bg-white/5 px-4 py-2 text-sm text-slate-200">
      <input type="checkbox" checked={checked} onChange={(event) => onChange(event.target.checked)} />
      {label}
    </label>
  )
}

export function Field({
  label,
  value,
  onChange,
  type = 'text',
  placeholder,
}: {
  label: string
  value: string
  onChange: (value: string) => void
  type?: 'text' | 'number' | 'password'
  placeholder?: string
}) {
  return (
    <label className="block">
      <span className="mb-2 block text-xs uppercase tracking-[0.25em] text-slate-400">{label}</span>
      <input
        type={type}
        value={value}
        placeholder={placeholder}
        onChange={(event) => onChange(event.target.value)}
        className="w-full rounded-2xl border border-white/10 bg-white/5 px-4 py-3 text-sm text-white outline-none transition focus:border-cyan-300/40"
      />
    </label>
  )
}
