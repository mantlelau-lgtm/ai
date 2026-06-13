type CodePanelProps = {
  code: string
  heightClassName?: string
}

export function CodePanel({ code, heightClassName = 'h-80' }: CodePanelProps) {
  return (
    <pre
      className={`overflow-auto rounded-[24px] border border-white/10 bg-slate-950/80 p-4 font-mono text-xs leading-6 text-cyan-100 ${heightClassName}`}
    >
      <code>{code}</code>
    </pre>
  )
}
