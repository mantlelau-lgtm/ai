from __future__ import annotations

from dataclasses import dataclass


@dataclass(frozen=True)
class ToolDependency:
    name: str
    category: str
    markets: tuple[str, ...]
    capabilities: tuple[str, ...]
    compliance_note: str
    enabled_by_default: bool = False


TOOL_DEPENDENCIES: tuple[ToolDependency, ...] = (
    ToolDependency(
        name="东方财富Choice金融终端API",
        category="market_data",
        markets=("A股", "港股"),
        capabilities=("日线行情", "分钟行情", "逐笔行情", "复权因子", "股本变动", "市值换手率"),
        compliance_note="商业授权后使用，需遵守数据供应商与交易所数据披露要求。",
    ),
    ToolDependency(
        name="Wind金融终端API",
        category="market_data",
        markets=("A股", "港股"),
        capabilities=("公司基本面", "行业分类", "资金流向", "融资融券", "沪深港通资金"),
        compliance_note="商业授权后使用，适合作为基础数据与风险计量主数据源。",
    ),
    ToolDependency(
        name="Tushare Pro",
        category="market_data",
        markets=("A股", "港股"),
        capabilities=("历史行情", "分红送转", "龙虎榜", "备用数据源"),
        compliance_note="按 Tushare Pro 授权范围使用，不作为实盘交易唯一行情源。",
        enabled_by_default=True,
    ),
    ToolDependency(
        name="富途牛牛开放API",
        category="market_data",
        markets=("港股",),
        capabilities=("实时盘口", "暗盘行情", "AH股溢价"),
        compliance_note="需使用合规账户授权，注意港股实时行情授权与展示限制。",
    ),
    ToolDependency(
        name="巨潮资讯网官方API",
        category="fundamental_data",
        markets=("A股",),
        capabilities=("公告", "财报", "监管问询函"),
        compliance_note="使用官方披露信息，适合作为 A 股公告与财报主来源。",
        enabled_by_default=True,
    ),
    ToolDependency(
        name="香港交易所披露易API",
        category="fundamental_data",
        markets=("港股",),
        capabilities=("公告", "财报", "股权变动"),
        compliance_note="遵守联交所信息披露和使用规范。",
        enabled_by_default=True,
    ),
    ToolDependency(
        name="同花顺iFind另类数据接口",
        category="alternative_data",
        markets=("A股", "港股"),
        capabilities=("北向资金持仓", "龙虎榜机构席位", "调研记录", "舆情热度"),
        compliance_note="商业授权后使用，另类数据需标注来源与更新时间。",
    ),
    ToolDependency(
        name="聚源数据API",
        category="alternative_data",
        markets=("A股", "港股"),
        capabilities=("财报文本解析", "非结构化数据", "基本面情绪因子"),
        compliance_note="商业授权后使用，非结构化抽取结果需保留原文溯源。",
    ),
    ToolDependency(
        name="聚宽JoinQuant回测引擎API",
        category="backtest",
        markets=("A股", "港股"),
        capabilities=("多因子回测", "统计套利回测", "交易规则模拟", "绩效评估"),
        compliance_note="回测结果不构成收益承诺，需要展示假设条件与风险提示。",
    ),
    ToolDependency(
        name="米筐RiceQuant回测工具",
        category="backtest",
        markets=("A股", "港股"),
        capabilities=("日内回转回测", "风险指标", "绩效归因"),
        compliance_note="回测需声明历史表现不代表未来收益。",
    ),
    ToolDependency(
        name="迅投量化仿真交易系统",
        category="simulation",
        markets=("A股", "港股"),
        capabilities=("仿真撮合", "实盘前验证", "交易接口规范验证"),
        compliance_note="仅用于仿真验证，不直接代表实盘成交能力。",
    ),
    ToolDependency(
        name="中信证券CTP接口",
        category="execution",
        markets=("A股",),
        capabilities=("订单申报", "撤单", "持仓查询"),
        compliance_note="实盘交易必须启用强权限、审批、风控与审计；默认禁用。",
    ),
    ToolDependency(
        name="辉立证券港股交易API",
        category="execution",
        markets=("港股",),
        capabilities=("订单申报", "暗盘交易", "交易结算适配"),
        compliance_note="实盘交易必须启用强权限、审批、风控与审计；默认禁用。",
    ),
    ToolDependency(
        name="恒生UMP风控系统API",
        category="risk_control",
        markets=("A股", "港股"),
        capabilities=("仓位监控", "单票限制", "异常交易拦截"),
        compliance_note="应作为交易执行前置风控，不允许绕过。",
    ),
    ToolDependency(
        name="证监会监管政策数据库API",
        category="compliance",
        markets=("A股", "港股"),
        capabilities=("监管政策更新", "影响识别", "策略调整提醒"),
        compliance_note="政策解释需标注来源，不替代法律意见。",
    ),
    ToolDependency(
        name="交易所交易规则查询API",
        category="compliance",
        markets=("A股", "港股"),
        capabilities=("交易规则修订", "交易时间", "涨跌幅限制", "报价单位"),
        compliance_note="规则版本需要随策略执行留痕。",
        enabled_by_default=True,
    ),
    ToolDependency(
        name="万得风险计量系统API",
        category="risk_monitoring",
        markets=("A股", "港股"),
        capabilities=("组合VaR", "预期损失", "风险阈值监控"),
        compliance_note="风险结果需要与持仓快照、行情快照绑定留痕。",
    ),
)


def dependency_catalog() -> list[dict[str, object]]:
    return [
        {
            "name": item.name,
            "category": item.category,
            "markets": list(item.markets),
            "capabilities": list(item.capabilities),
            "compliance_note": item.compliance_note,
            "enabled_by_default": item.enabled_by_default,
        }
        for item in TOOL_DEPENDENCIES
    ]
