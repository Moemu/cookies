import { useCallback, useEffect, useMemo, useState } from 'react'
import { CircleAlert, Layers3, RefreshCw, Ruler } from 'lucide-react'
import { useProject } from '../context/ProjectContext'
import {
  api,
  type ApiAssetTrend,
  type ApiConfidenceLevel,
  type ApiDeliveryExecutionResult,
  type ApiFatigueSignal,
  type ApiFeatureDriver,
  type ApiMetricAnomaly,
  type ApiMetricRates,
  type ApiPerformanceAnalysis,
  type ApiVariantComparison,
  type ApiVariantVerdict,
} from '../data/api'
import type { DataState, SystemKey } from '../types'
import { FreezeReportAction, isoDay } from './FreezeReportAction'
import { PostLaunchOverviewPage } from './PostLaunchOverviewPage'
import { StateBoundary } from './StateBoundary'

/**
 * 投后分析的后五个二级视图：素材对比 · 趋势 · 疲劳 · 异常 · 驱动因素
 * （视图清单出自 03 §一级导航，冲突见基线文档 §5 冲突 7；第一个视图「指标总览」
 * 仍由 PostLaunchOverviewPage 承担，这里只做分发）。
 *
 * 五个视图共用后端一次 `performance-analysis` 返回。拆成五次请求会让「趋势里看到的」
 * 和「疲劳里算的」来自两份数据，对不上时没人解释得清。
 *
 * 全页只有一条纪律：**能不能归因，和差异有多大，是两件事**。所以每一条结论旁边都
 * 有一句 note 说明它为什么是这个档位，疲劳还会把「没能排除的其他解释」原样列出来
 * ——列的是排除不了的，不是已排除的（03 §7.4 要求区分数据延迟 / 受众变化 /
 * 预算变化 / 真实衰退，当前只有前者和后者能从已有数据里分开）。
 */
const rangeOptions = [
  { label: '近 7 天', days: 7 },
  { label: '近 30 天', days: 30 },
  { label: '近 90 天', days: 90 },
]

type ViewTarget = 'comparisons' | 'trends' | 'fatigue' | 'anomalies' | 'drivers'

const viewTargets: Record<string, ViewTarget> = {
  素材对比: 'comparisons',
  趋势: 'trends',
  疲劳: 'fatigue',
  异常: 'anomalies',
  驱动因素: 'drivers',
}

const headings: Record<ViewTarget, { label: string; title: string; lead: string }> = {
  comparisons: {
    label: 'VARIANT COMPARISON',
    title: '两个版本之间到底改了什么，差异能不能算到它头上',
    lead: '同类型素材两两配对。先识别变量，再谈差异——同时改了两个地方的对比，数字差得再多也归不了因（AM-009）。',
  },
  trends: {
    label: 'TREND',
    title: '每个素材在这段时间里是往上走还是往下走',
    lead: '按真正有数据的天数把窗口对半切开比较，不是按日历天数。缺数据的日子不算平，算不出来。',
  },
  fatigue: {
    label: 'FATIGUE',
    title: '哪些素材开始跑不动了，以及有哪些解释还没被排除',
    lead: '曝光还在放大但点击率下滑、或点击率下滑同时成本上升，才判为「疑似疲劳」；单项恶化只到「需要观察」。',
  },
  anomalies: {
    label: 'ANOMALY',
    title: '哪一天的数字明显偏离了常态',
    lead: '用中位数和 MAD 判断偏离，不用平均值和标准差——一次大促尖峰会把标准差自己抬高，之后什么都不再异常。',
  },
  drivers: {
    label: 'DRIVER',
    title: '哪些内容特征和表现一起变化',
    lead: '一起变化不等于起了作用。组内还有别的特征同向变化时，这里会直接把它标成「分不开」，不给强结论。',
  },
}

const emptyHints: Record<ViewTarget, string> = {
  comparisons: '还没有可配对的素材。需要同一类型下至少两个素材、且都在这个窗口里有投放数据。',
  trends: '这个窗口里没有素材产生过投放数据。',
  fatigue: '这个窗口里没有素材跑够两段可比较的时间。',
  anomalies: '这个窗口里没有发现偏离常态的日子。窗口内至少要有 5 天数据才做这项判断。',
  drivers: '还没有素材记录内容特征，或同一取值下的素材不足 2 个。去「分析素材库 · 特征库」补齐特征后再来看。',
}

const confidenceLabels: Record<ApiConfidenceLevel, string> = {
  sufficient: '充分',
  directional: '方向性',
  low_sample: '样本不足',
  confounded: '存在混杂',
}

// 判定文案刻意写成「能不能」而不是「好不好」：读者要先知道这条结论能不能用。
const verdictLabels: Record<ApiVariantVerdict, string> = {
  attributable: '可归因到该变量',
  directional: '只能看方向',
  confounded: '归不了因',
  low_sample: '样本不足',
  no_features: '缺内容特征',
}

const verdictTone: Record<ApiVariantVerdict, string> = {
  attributable: 'ok',
  directional: 'warning',
  confounded: 'danger',
  low_sample: 'muted',
  no_features: 'muted',
}

const directionLabels: Record<ApiAssetTrend['direction'], string> = {
  rising: '上行',
  flat: '持平',
  declining: '下行',
  unknown: '看不出',
}

const severityLabels: Record<ApiFatigueSignal['severity'], string> = {
  none: '没有迹象',
  watch: '需要观察',
  likely: '疑似疲劳',
}

const severityTone: Record<ApiFatigueSignal['severity'], string> = {
  none: 'muted',
  watch: 'warning',
  likely: 'danger',
}

const anomalyLabels: Record<ApiMetricAnomaly['kind'], string> = {
  spike: '异常偏高',
  drop: '异常偏低',
  gap: '当天没有数据',
}

// 后端传回来的是指标键名。整页都是产品语言，这一列漏出 impressions 会让人
// 以为它是个需要认识的技术字段。认不出来的键就原样显示，不编一个中文名。
const metricLabels: Record<string, string> = {
  impressions: '曝光',
  clicks: '点击',
  spend_cents: '花费',
  conversions: '转化',
  revenue_cents: '收入',
}

type OpenProject = (id: string, system?: SystemKey, navId?: string, objectId?: string, view?: string) => void

export function PostLaunchAnalysisPage({ state, activeView, onOpenProject }: {
  state: DataState
  activeView: string
  onOpenProject: OpenProject
}) {
  const target = viewTargets[activeView]
  // 指标总览走原来那一页：它是「一个主图表 + 素材矩阵 + 解释区」的完整布局（20 §4.1），
  // 和这五个列表视图不是同一种页面，硬塞进来只会两边都不像。
  if (!target) return <PostLaunchOverviewPage state={state} onOpenProject={onOpenProject}/>
  return <AnalysisViews state={state} target={target} onOpenProject={onOpenProject}/>
}

function AnalysisViews({ state, target, onOpenProject }: {
  state: DataState
  target: ViewTarget
  onOpenProject: OpenProject
}) {
  const { currentProject } = useProject()
  const [rangeLabel, setRangeLabel] = useState('近 30 天')
  const [analysis, setAnalysis] = useState<ApiPerformanceAnalysis | null>(null)
  const [notice, setNotice] = useState('')
  const [listState, setListState] = useState<'loading' | 'ready' | 'error'>('loading')
  const [freezing, setFreezing] = useState(false)

  const load = useCallback(async () => {
    if (!currentProject.id) return
    setListState('loading')
    try {
      const days = rangeOptions.find(option => option.label === rangeLabel)?.days ?? 30
      const end = new Date()
      const start = new Date(end.valueOf() - days * 24 * 60 * 60 * 1000)
      const next = await api.getPerformanceAnalysis(currentProject.id, isoDate(start), isoDate(end))
      setAnalysis(next)
      setListState('ready')
    } catch (cause) {
      setAnalysis(null)
      setListState('error')
      setNotice(cause instanceof Error ? cause.message : '投后分析读取失败。')
    }
  }, [currentProject.id, rangeLabel])

  useEffect(() => { void load() }, [load])

  const heading = headings[target]

  // 会影响下面每一条结论怎么读的东西，一律放在结论前面（20 §4.1）。
  const troubles = useMemo(() => [
    ...(analysis?.notes ?? []),
    ...(analysis && !analysis.comparable
      ? [analysis.comparable_reason || '窗口内口径不一致，可归因的判定已全部降级为方向性观察。']
      : []),
  ], [analysis])

  const rows = analysis ? countFor(analysis, target) : 0

  // 定格这一屏。窗口用的是当前这一屏的窗口，不重新算——报告上写的日期区间
  // 必须和人刚才看的那一屏是同一个，否则「我当时看到的」和「报告里写的」
  // 会是两回事，而两边都写着同一句话。
  const freeze = useCallback(async (executionId: string) => {
    if (!analysis) return
    setFreezing(true)
    setNotice('')
    try {
      const report = await api.createReport(currentProject.id, {
        execution_id: executionId,
        window: { start: isoDay(analysis.window.start), end: isoDay(analysis.window.end) },
      })
      onOpenProject(currentProject.id, 'insight', 'reports', report.id, '待确认')
    } catch (cause) {
      setNotice(cause instanceof Error ? cause.message : '定格失败，请重试。')
    } finally {
      setFreezing(false)
    }
  }, [analysis, currentProject.id, onOpenProject])

  return <StateBoundary state={state} onRetry={() => { void load() }}>
    <div className="prelaunch-workspace">
      <section className="prelaunch-main">
        <div className="core-flow-toolbar">
          <div>
            <span className="section-label">{heading.label}</span>
            <h2>{heading.title}</h2>
            <p>{heading.lead}</p>
          </div>
          <div className="core-flow-actions">
            <label>窗口<select aria-label="数据窗口" value={rangeLabel} onChange={event => setRangeLabel(event.target.value)}>
              {rangeOptions.map(option => <option key={option.label}>{option.label}</option>)}
            </select></label>
            <button className="secondary-button" disabled={listState === 'loading'} onClick={() => { void load() }}>
              <RefreshCw size={15}/>刷新
            </button>
          </div>
        </div>

        {troubles.length ? <div className="feature-stack">
          <span>先看这些（会影响下面每一条结论怎么读）</span>
          {troubles.map((item, index) => <b key={index}>{item}</b>)}
        </div> : null}

        {listState === 'loading' ? <div className="panel-empty">正在取数…</div>
          : listState === 'error' ? <div className="panel-empty">读取失败，请重试。</div>
          : !analysis || rows === 0 ? <div className="panel-empty">{emptyHints[target]}</div>
          : target === 'comparisons' ? <ComparisonList items={analysis.comparisons}/>
          : target === 'trends' ? <TrendList items={analysis.trends}/>
          : target === 'fatigue' ? <FatigueList items={analysis.fatigue}/>
          : target === 'anomalies' ? <AnomalyList items={analysis.anomalies}/>
          : <DriverList items={analysis.drivers}/>}
      </section>

      <aside className="prelaunch-detail">
        <span className="section-label">这一页怎么读</span>
        <h3>{heading.title}</h3>
        <p>{heading.lead}</p>

        {analysis ? <>
          <div className="prelaunch-fact"><Layers3 size={17}/><span><small>数据窗口</small><b>
            {formatDate(analysis.window.start)} ~ {formatDate(analysis.window.end)}
          </b></span></div>
          <div className="prelaunch-fact"><Ruler size={17}/><span><small>口径</small><b>
            {analysis.caliber.currency} · 归因 {analysis.caliber.attribution_window} ·
            指标口径 {analysis.caliber.metric_schema_version} · 时区 {analysis.caliber.time_zone}
          </b></span></div>
          <div className="prelaunch-fact"><Layers3 size={17}/><span><small>覆盖情况</small><b>
            窗口内有 {analysis.assets_in_window} 个素材产生了数据，其中 {analysis.assets_with_features} 个记录了内容特征。
          </b></span></div>
          {analysis.assets_with_features < analysis.assets_in_window ? <div className="prelaunch-boundary">
            <CircleAlert size={16}/><span><small>特征没覆盖全</small>
              还有 {analysis.assets_in_window - analysis.assets_with_features} 个素材没有内容特征。它们不会出现在素材对比和驱动因素里
              ——不是它们表现不好，是没法判断它们和别人差在哪。去「分析素材库 · 待提取」补。
            </span></div> : null}
        </> : null}

        <div className="prelaunch-boundary"><CircleAlert size={16}/><span><small>这一页给不了什么</small>
          这里全部是观察到的相关，没有一条是实验证明的因果。要确认某个变量真的有效，得在同一批投放里只改它一个
          ——那是「实验中心」的事，可以从那里建一个实验来验证。
        </span></div>

        {analysis ? <FreezeReportAction window={analysis.window} busy={freezing} onFreeze={freeze}/> : null}

        {notice ? <div className="inline-notice" role="status">{notice}</div> : null}
      </aside>
    </div>
  </StateBoundary>
}

// 后端已经保证这些数组不会是 null，这里再兜一层：一个字段没返回不该让整页打不开。
function countFor(analysis: ApiPerformanceAnalysis, target: ViewTarget): number {
  if (target === 'comparisons') return (analysis.comparisons ?? []).length
  if (target === 'trends') return (analysis.trends ?? []).length
  if (target === 'fatigue') return (analysis.fatigue ?? []).length
  if (target === 'anomalies') return (analysis.anomalies ?? []).length
  return (analysis.drivers ?? []).length
}

function ComparisonList({ items }: { items: ApiVariantComparison[] }) {
  return <div className="insight-analysis-list" role="list" aria-label="素材对比">
    {items.map(item => <article className="insight-analysis-card" role="listitem"
      key={`${item.baseline_asset_id}-${item.variant_asset_id}`}>
      <header>
        <b>{item.baseline_title} ↔ {item.variant_title}</b>
        <em className={verdictTone[item.variant_verdict]}>{verdictLabels[item.variant_verdict]}</em>
      </header>
      <p>{item.note}</p>

      <div className="insight-analysis-diff">
        <span>改动的变量（{(item.changed_features ?? []).length} 个）· 两侧一致的特征 {item.controlled_count} 个</span>
        {(item.changed_features ?? []).length
          ? (item.changed_features ?? []).map(diff => <b key={diff.key}>
              {diff.label}：{diff.baseline} → {diff.variant}{diff.human_only ? '（只能人工判定）' : ''}
            </b>)
          : <b>已记录的特征上两边完全一致。</b>}
      </div>

      <div className="insight-analysis-metrics">
        <Side title={item.baseline_title} rates={item.baseline_rates}
          impressions={item.baseline_counts.impressions} interval={item.baseline_ctr_interval}/>
        <Side title={item.variant_title} rates={item.variant_rates}
          impressions={item.variant_counts.impressions} interval={item.variant_ctr_interval}/>
      </div>

      <footer>
        点击率相对变化 {formatSigned(item.ctr_lift)} ·
        {item.intervals_overlap ? ' 两侧置信区间重叠，这个差异可能只是噪声' : ' 两侧置信区间不重叠'} ·
        置信{confidenceLabels[item.confidence]}
      </footer>
    </article>)}
  </div>
}

function Side({ title, rates, impressions, interval }:
  { title: string; rates: ApiMetricRates; impressions: number; interval?: { low: number; high: number } }) {
  return <div>
    <small>{title}</small>
    <b>{formatRate(rates.ctr)}</b>
    <span>
      曝光 {formatCount(impressions)} · 转化成本 {formatMoney(rates.cpa_cents)}
      <br/>{interval ? `95% 区间 ${formatRate(interval.low)} ~ ${formatRate(interval.high)}` : '样本不足以给出区间'}
    </span>
  </div>
}

function TrendList({ items }: { items: ApiAssetTrend[] }) {
  return <div className="insight-analysis-list" role="list" aria-label="趋势">
    {items.map(item => {
      const points = item.points ?? []
      const peak = Math.max(...points.map(point => point.counts.impressions), 1)
      return <article className="insight-analysis-card" role="listitem" key={item.asset_id}>
        <header>
          <b>{item.asset_title}</b>
          <em className={item.direction === 'declining' ? 'danger' : item.direction === 'rising' ? 'ok' : 'muted'}>
            {directionLabels[item.direction]}
          </em>
        </header>
        <p>{item.note}</p>
        <div className="insight-series">
          <span className="section-label">
            {item.active_days} 天有数据 · 点击率变化 {formatSigned(item.ctr_change)} · 置信{confidenceLabels[item.confidence]}
          </span>
          {points.map(point => <div className="insight-series-row" key={point.date}>
            <span>{formatDate(point.date)}</span>
            <span className="insight-series-track">
              <i style={{ width: `${Math.max(2, Math.round((point.counts.impressions / peak) * 100))}%` }}/>
            </span>
            <span>曝光 {formatCount(point.counts.impressions)}</span>
            <span>点击率 {formatRate(point.rates.ctr)}</span>
            <span>花费 {formatMoney(point.counts.spend_cents)}</span>
          </div>)}
        </div>
      </article>
    })}
  </div>
}

function FatigueList({ items }: { items: ApiFatigueSignal[] }) {
  return <div className="insight-analysis-list" role="list" aria-label="疲劳">
    {items.map(item => <article className="insight-analysis-card" role="listitem" key={item.asset_id}>
      <header>
        <b>{item.asset_title}</b>
        {/* 「没有迹象」和「判断不了」对读者是两句相反的话：前者是查过了没问题，
            后者是压根没法查。后端在天数不足时会把置信压到 low_sample，这里据此分开。 */}
        <em className={item.severity === 'none' && item.confidence === 'low_sample' ? 'muted' : severityTone[item.severity]}>
          {item.severity === 'none' && item.confidence === 'low_sample' ? '判断不了' : severityLabels[item.severity]}
        </em>
      </header>
      <p>{item.note}</p>

      <div className="insight-analysis-metrics">
        <div>
          <small>前半段</small>
          <b>{formatRate(item.first_rates.ctr)}</b>
          <span>曝光 {formatCount(item.first_half.impressions)} · 转化成本 {formatMoney(item.first_rates.cpa_cents)}</span>
        </div>
        <div>
          <small>后半段</small>
          <b>{formatRate(item.last_rates.ctr)}</b>
          <span>曝光 {formatCount(item.second_half.impressions)} · 转化成本 {formatMoney(item.last_rates.cpa_cents)}</span>
        </div>
      </div>

      <footer>
        点击率 {formatSigned(item.ctr_change)} · 转化成本 {formatSigned(item.cpa_change)} ·
        曝光 {formatSigned(item.impression_change)} · 置信{confidenceLabels[item.confidence]}
      </footer>

      {/* 03 §7.4 要求区分四种解释。这里只列「排除不了的」，不写「已排除」——
          受众构成数据一条都没接入，声称排除了受众变化就是编。 */}
      {item.alternative_explanations?.length ? <div className="insight-analysis-diff">
        <span>下面这些解释还没被排除</span>
        {item.alternative_explanations.map((reason, index) => <b key={index}>{reason}</b>)}
      </div> : null}
    </article>)}
  </div>
}

function AnomalyList({ items }: { items: ApiMetricAnomaly[] }) {
  return <div className="prelaunch-table" role="list" aria-label="异常">
    <div className="prelaunch-row insight-anomaly-row header">
      <span>日期与范围</span><span>指标</span><span>类型</span><span>实测 / 常态</span><span>偏离</span>
    </div>
    {items.map((item, index) => <div role="listitem" className="prelaunch-row insight-anomaly-row"
      key={`${item.date}-${item.scope}-${item.asset_id ?? ''}-${item.metric}-${index}`}>
      <span><b>{formatDate(item.date)}</b><small>{item.scope === 'project' ? '整个项目' : item.asset_title || item.asset_id}</small></span>
      <span>{metricLabels[item.metric] ?? item.metric}</span>
      <span>{anomalyLabels[item.kind]}</span>
      {/* 断档行没有「实测」这回事：那天根本没有数据。写 0 会被读成「测到了 0 次曝光」，
          那是另一件事——停投和没回流在数字上都不是 0。 */}
      <span>{item.kind === 'gap' ? '—' : `${formatNumber(item.observed)} / ${formatNumber(item.median)}`}</span>
      <span>{item.kind === 'gap' ? '—' : `${item.deviation.toFixed(1)} 倍 MAD`}</span>
      <small className="insight-anomaly-note">{item.note}</small>
    </div>)}
  </div>
}

function DriverList({ items }: { items: ApiFeatureDriver[] }) {
  return <div className="insight-analysis-list" role="list" aria-label="驱动因素">
    {items.map(item => <article className="insight-analysis-card" role="listitem"
      key={`${item.asset_type ?? ''}-${item.key}-${item.value}`}>
      <header>
        <b>{item.label}：{item.value}</b>
        <em className={item.confidence === 'sufficient' ? 'ok' : item.confidence === 'confounded' ? 'danger' : 'warning'}>
          置信{confidenceLabels[item.confidence]}
        </em>
      </header>
      <p>{item.note}</p>

      <div className="insight-analysis-metrics">
        <div>
          <small>这一组（{item.assets} 个素材）</small>
          <b>{formatRate(item.rates.ctr)}</b>
          <span>
            曝光 {formatCount(item.counts.impressions)} · 转化成本 {formatMoney(item.rates.cpa_cents)}
            <br/>{item.ctr_interval ? `95% 区间 ${formatRate(item.ctr_interval.low)} ~ ${formatRate(item.ctr_interval.high)}` : '样本不足以给出区间'}
          </span>
        </div>
        <div>
          <small>其余素材（{item.rest_assets} 个）</small>
          <b>{formatRate(item.rest_rates.ctr)}</b>
          <span>
            曝光 {formatCount(item.rest_counts.impressions)} · 转化成本 {formatMoney(item.rest_rates.cpa_cents)}
            <br/>{item.rest_ctr_interval ? `95% 区间 ${formatRate(item.rest_ctr_interval.low)} ~ ${formatRate(item.rest_ctr_interval.high)}` : '样本不足以给出区间'}
          </span>
        </div>
      </div>

      <footer>
        点击率相对差异 {formatSigned(item.ctr_lift)} ·
        {item.intervals_overlap ? ' 两组置信区间重叠，差异可能只是噪声' : ' 两组置信区间不重叠'}
      </footer>

      {item.covarying_features?.length ? <div className="insight-analysis-diff">
        <span>这些特征跟着一起变了，分不开谁在起作用</span>
        {item.covarying_features.map((feature, index) => <b key={index}>{feature}</b>)}
      </div> : null}
    </article>)}
  </div>
}

// 下面的格式化函数共用一条规则：undefined 是「算不出来」，不是 0。
function formatRate(value?: number): string {
  return value === undefined ? '不可用' : `${(value * 100).toFixed(2)}%`
}

function formatSigned(value?: number): string {
  if (value === undefined) return '不可用'
  return `${value > 0 ? '+' : ''}${(value * 100).toFixed(1)}%`
}

function formatMoney(cents?: number): string {
  return cents === undefined ? '不可用' : `¥${(cents / 100).toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`
}

function formatCount(value: number): string {
  return value.toLocaleString('zh-CN')
}

function formatNumber(value: number): string {
  return Number.isInteger(value) ? value.toLocaleString('zh-CN') : value.toFixed(4)
}

function isoDate(value: Date): string {
  return `${value.getFullYear()}-${String(value.getMonth() + 1).padStart(2, '0')}-${String(value.getDate()).padStart(2, '0')}`
}

function formatDate(value: string): string {
  const time = new Date(value)
  return Number.isNaN(time.valueOf()) ? value : time.toLocaleDateString('zh-CN')
}
