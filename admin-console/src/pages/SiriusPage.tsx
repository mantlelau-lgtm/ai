import { useEffect, useMemo, useState } from 'react'

import { ShieldCheck, Save, TrendingUp } from 'lucide-react'

import { Panel } from '@/components/Panel'
import { useAdminStore } from '@/store/useAdminStore'

export function SiriusPage() {
  const agents = useAdminStore((state) => state.agents)
  const credentials = useAdminStore((state) => state.bundle.llm.credentials)
  const tools = useAdminStore((state) => state.tools)
  const updateAgentKey = useAdminStore((state) => state.updateAgentKey)
  const updateAgentTools = useAdminStore((state) => state.updateAgentTools)
  const saving = useAdminStore((state) => state.saving)
  const sirius = agents.find((item) => item.name === 'sirius')
  const [keyName, setKeyName] = useState(sirius?.key_name ?? '')
  const [selectedTools, setSelectedTools] = useState<string[]>(sirius?.tools ?? [])
  const keyOptions = useMemo(
    () => credentials.map((item) => item.key_name).filter((name) => name.trim().length > 0),
    [credentials],
  )
  const sortedTools = useMemo(() => [...tools].sort((a, b) => a.name.localeCompare(b.name)), [tools])

  useEffect(() => {
    setKeyName(sirius?.key_name ?? '')
    setSelectedTools(sirius?.tools ?? [])
  }, [sirius?.key_name, sirius?.tools])

  const toggleTool = (name: string) => {
    setSelectedTools((current) =>
      current.includes(name) ? current.filter((item) => item !== name) : [...current, name].sort(),
    )
  }

  return (
    <div className="space-y-6">
      <Panel title="Sirius Agent 配置" eyebrow="Quant Trading Assistant">
        <div className="grid gap-4 lg:grid-cols-3">
          <div className="rounded-[24px] border border-cyan-300/15 bg-cyan-300/5 p-5">
            <TrendingUp className="h-6 w-6 text-cyan-200" />
            <p className="mt-4 text-sm font-medium text-white">Agent 状态</p>
            <p className="mt-2 text-sm leading-6 text-slate-300">
              {sirius ? '已由 core-service 注册到后台，可绑定 bot 路由使用。' : '尚未发现 sirius，请确认 core-service 已启动。'}
            </p>
          </div>
          <div className="rounded-[24px] border border-emerald-300/15 bg-emerald-300/5 p-5">
            <ShieldCheck className="h-6 w-6 text-emerald-200" />
            <p className="mt-4 text-sm font-medium text-white">交易护栏</p>
            <p className="mt-2 text-sm leading-6 text-slate-300">
              实盘交易默认禁用，所有交易动作必须经过 sirius.execution_guard 与风控审计。
            </p>
          </div>
          <div className="rounded-[24px] border border-white/10 bg-white/5 p-5">
            <p className="text-sm font-medium text-white">当前密钥</p>
            <p className="mt-2 text-2xl font-semibold text-cyan-100">{sirius?.key_name || '未设置'}</p>
            <p className="mt-2 text-xs text-slate-400">未设置时使用 llm-gateway 默认密钥。</p>
          </div>
        </div>

        <label className="mt-6 block">
          <span className="mb-2 block text-xs uppercase tracking-[0.25em] text-slate-400">Sirius 使用的 LLM 密钥</span>
          <select
            value={keyName}
            onChange={(event) => setKeyName(event.target.value)}
            className="w-full rounded-2xl border border-white/10 bg-white/5 px-4 py-3 text-sm text-white outline-none transition focus:border-cyan-300/40"
          >
            <option value="">未设置（沿用 llm-gw 默认密钥）</option>
            {keyOptions.map((option) => (
              <option key={option} value={option}>
                {option}
              </option>
            ))}
          </select>
        </label>
        <button
          type="button"
          disabled={!sirius || saving || keyName === (sirius?.key_name ?? '')}
          onClick={() => void updateAgentKey('sirius', keyName)}
          className="mt-4 inline-flex items-center gap-2 rounded-2xl bg-cyan-300/90 px-4 py-3 text-sm font-medium text-slate-950 transition hover:bg-cyan-200 disabled:cursor-not-allowed disabled:opacity-50"
        >
          <Save className="h-4 w-4" />
          保存 Sirius 密钥绑定
        </button>
      </Panel>

      <Panel title="Sirius 工具白名单" eyebrow="Tools">
        <div className="mb-4 flex flex-wrap gap-3">
          <button
            type="button"
            onClick={() => setSelectedTools(sortedTools.map((item) => item.name))}
            className="rounded-2xl border border-white/10 px-4 py-2 text-xs text-slate-200 transition hover:border-cyan-300/40"
          >
            全选
          </button>
          <button
            type="button"
            onClick={() => setSelectedTools([])}
            className="rounded-2xl border border-white/10 px-4 py-2 text-xs text-slate-200 transition hover:border-cyan-300/40"
          >
            清空（表示不限制）
          </button>
          <button
            type="button"
            disabled={!sirius || saving}
            onClick={() => void updateAgentTools('sirius', selectedTools)}
            className="rounded-2xl bg-cyan-300/90 px-4 py-2 text-xs font-medium text-slate-950 transition hover:bg-cyan-200 disabled:cursor-not-allowed disabled:opacity-50"
          >
            保存工具白名单
          </button>
        </div>
        <p className="mb-4 text-xs text-slate-400">
          已选择 {selectedTools.length} / {sortedTools.length} 个工具。清空白名单时，core-service 会按“不限制工具”处理。
        </p>
        <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
          {sortedTools.map((tool) => (
            <label key={tool.name} className="rounded-2xl border border-white/10 bg-white/5 px-4 py-3 text-sm text-slate-200">
              <span className="flex items-start gap-3">
                <input
                  type="checkbox"
                  checked={selectedTools.includes(tool.name)}
                  onChange={() => toggleTool(tool.name)}
                  className="mt-1"
                />
                <span>
                  <span className="block font-medium text-white">{tool.name}</span>
                  <span className="mt-1 block text-xs text-slate-400">{tool.source} · {tool.description}</span>
                </span>
              </span>
            </label>
          ))}
        </div>
      </Panel>

      <Panel title="数据源配置说明" eyebrow="Market Data Provider">
        <div className="space-y-3 text-sm leading-7 text-slate-300">
          <p>后端通过环境变量选择 Sirius 行情数据源：</p>
          <pre className="overflow-auto rounded-2xl border border-white/10 bg-slate-950/70 p-4 text-xs text-cyan-100">
{`SIRIUS_MARKET_DATA_PROVIDER=mock | tushare
SIRIUS_TUSHARE_TOKEN=your_tushare_token
SIRIUS_TUSHARE_BASE_URL=http://api.tushare.pro`}
          </pre>
          <p>未配置 Tushare token 时，系统保持 mock 数据模式，确保研究链路和工具链路可运行。</p>
        </div>
      </Panel>
    </div>
  )
}
