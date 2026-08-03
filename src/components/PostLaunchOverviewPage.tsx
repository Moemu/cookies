import { useCallback, useEffect, useMemo, useState } from 'react'
import { CircleAlert, CircleCheck, Database, Layers3, Lightbulb, RefreshCw, TrendingUp } from 'lucide-react'
import { useProject } from '../context/ProjectContext'
import {
  api,
  type ApiAssetMetricPerformance,
  type ApiConfidenceLevel,
  type ApiMetricOverview,
  type ApiMetricRates,
  type ApiQualityStatus,
} from '../data/api'
import type { DataState, SystemKey } from '../types'
import { FreezeReportAction, isoDay } from './FreezeReportAction'
import { StateBoundary } from './StateBoundary'

/**
 * 投后分析 · 指标总览（10-ad-data-connectors.md §6/§11，导航见 19 §5.2）。
 *
 * 这一页只回答一件事：这段时间投出去的钱换回了什么，以及这个数字能信到什么程度。
 * 22 §6.2 规定 MVP 阶段投后分析只开指标总览，疲劳、异常、归因这些 L2 先不放出来
 * ——没有真实的多周期数据，那几张图只会是好看的假话。
 *
 * 20 §4.1 对本页有四条硬要求：数据窗口、样本量、口径、置信范围都必须显示在数字
 * 旁边，而不是藏在说明文档里；错误与延迟必须置顶。
 *
 * 派生指标（CTR/CVR/CPA/CPM/ROAS）分母为零时后端不返回该字段（doc10 §6）。这里
 * 一律显示「不可用」，绝不退化成 0——0 会被当成「表现极差」而不是「算不出来」。
 */
const rangeOptions = [
  { label: '近 7 天', days: 7 },
  { label: '近 30 天', days: 30 },
  { label: '近 90 天', days: 90 },
]

const confidenceLabels: Record<ApiConfidenceLevel, string> = {
  sufficient: '充分',
  directional: '方向性',
  low_sample: '样本不足',
  confounded: '存在混杂',
}

// 03 §9 的置信提示词表，每一级都要说清「能拿它做什么、不能做什么」。
const confidenceMeaning: Record<ApiConfidenceLevel, string> = {
  sufficient: '样本量和数据质量都够，可以据此下结论并驱动下一步动作。',
  directional: '只能看方向，不足以支撑「A 比 B 好」这类判断，更不能据此自动调整投放。',
  low_sample: '样本太少，任何比较都可能只是噪声。先让它多跑一段时间。',
  confounded: '数据里有未匹配、延迟或质量问题，结论只能是方向性的，不应据此自动优化。',
}

const qualityLabels: Record<ApiQualityStatus, string> = {
  healthy: '正常',
  delayed: '延迟',
  partial: '不完整',
  mapping_incomplete: '映射未完成',
  tracking_broken: '追踪异常',
  reconciling: '对账中',
  blocked: '已阻断',
}

export function PostLaunchOverviewPage({ state, onOpenProject }: {
  state: DataState
  onOpenProject: (id: string, system?: SystemKey, navId?: string, objectId?: string, view?: string) => void
}) {
  const { currentProject } = useProject()
  const [rangeLabel, setRangeLabel] = useState('近 30 天')
  const [overview, setOverview] = useState<ApiMetricOverview | null>(null)
  const [selectedId, setSelectedId] = useState('')
  const [notice, setNotice] = useState('')
  const [listState, setListState] = useState<'loading' | 'ready' | 'error'>('loading')
  const [freezing, setFreezing] = useState(false)

  const load = useCallback(async () => {
    if (!currentProject.id) return
    setListState('loading')
    try {
      // 窗口显式写进请求，而不是让后端默认——页面上显示的窗口必须就是取数用的窗口（20 §4.1）。
      const days = rangeOptions.find(option => option.label === rangeLabel)?.days ?? 30
      const end = new Date()
      const start = new Date(end.valueOf() - days * 24 * 60 * 60 * 1000)
      // 后端只收 2006-01-02，带时分秒会被判成参数无效。
      const next = await api.getMetricOverview(currentProject.id, isoDate(start), isoDate(end))
      setOverview(next)
      setListState('ready')
    } catch (cause) {
      setOverview(null)
      setListState('error')
      setNotice(cause instanceof Error ? cause.message : '投后指标读取失败。')
    }
  }, [currentProject.id, rangeLabel])

  useEffect(() => { void load() }, [load])

  // 定格这一屏。窗口取的是后端返回的窗口，不重新算一遍——报告上的日期区间必须和
  // 人刚才在这一页读到的那个区间一模一样，否则「我当时看到的」和「报告里写的」
  // 会是两回事，而两边都不会报错。
  const freeze = useCallback(async (executionId: string) => {
    if (!overview) return
    setFreezing(true)
    setNotice('')
    try {
      const report = await api.createReport(currentProject.id, {
        execution_id: executionId,
        window: { start: isoDay(overview.window.start), end: isoDay(overview.window.end) },
      })
      onOpenProject(currentProject.id, 'insight', 'reports', report.id, '待确认')
    } catch (cause) {
      setNotice(cause instanceof Error ? cause.message : '定格失败，请重试。')
    } finally {
      setFreezing(false)
    }
  }, [overview, currentProject.id, onOpenProject])

  // 花得多的排前面：先看清钱去哪了，再谈哪一版更好。
  const assets = useMemo(() => [...(overview?.assets ?? [])]
    .sort((left, right) => right.counts.spend_cents - left.counts.spend_cents), [overview])

  useEffect(() => {
    const ids = assets.map(asset => asset.asset_id ?? asset.asset_title)
    setSelectedId(current => ids.includes(current) ? current : ids[0] ?? '')
  }, [assets])

  const selected = assets.find(asset => (asset.asset_id ?? asset.asset_title) === selectedId)
  const series = overview?.series ?? []
  const peakSpend = Math.max(...series.map(point => point.counts.spend_cents), 1)
  const hasData = Boolean(overview && overview.totals.impressions + overview.totals.spend_cents > 0)

  // 有问题的先说：延迟、质量异常、口径冲突、不可比，都要在图表之前出现（20 §4.1）。
  const troubles = [
    ...(overview?.warnings ?? []),
    ...(overview?.caliber_conflicts ?? []),
    ...(overview && !overview.comparable
      ? [overview.comparable_reason || '这些数字口径不一致，不能直接放在一起比。']
      : []),
    ...(overview && overview.unmatched_objects > 0
      ? [`有 ${overview.unmatched_objects} 个平台对象还没认领，${formatMoney(overview.unmatched_spend_cents)} 花费计入了总盘但算不到任何素材头上。去「分析素材库 · 待匹配」认领。`]
      : []),
  ]

  return <StateBoundary state={state} onRetry={() => { void load() }}>
    <div className="prelaunch-workspace">
      <section className="prelaunch-main">
        <div className="core-flow-toolbar">
          <div>
            <span className="section-label">POST-LAUNCH OVERVIEW</span>
            <h2>这段时间的钱换回了什么，以及这个数字能信到什么程度</h2>
            <p>当前 Project：{currentProject.name}。只统计已接入数据源导进来的真实指标，没有的就是没有。</p>
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
          <span>先看这些（会影响下面每一个数字怎么读）</span>
          {troubles.map((item, index) => <b key={index}>{item}</b>)}
        </div> : null}

        {listState === 'loading' ? <div className="panel-empty">正在取数…</div>
          : !hasData ? <div className="panel-empty">
            这个窗口里没有任何指标数据。先在「数据接入」接一个数据源、配好字段映射并导入报表，这一页才有东西可看。
          </div> : overview ? <>
            <div className="insight-metric-grid">
              <MetricCard label="花费" value={formatMoney(overview.totals.spend_cents)} note={`收入 ${formatMoney(overview.totals.revenue_cents)}`}/>
              <MetricCard label="曝光" value={formatCount(overview.totals.impressions)} note={`点击 ${formatCount(overview.totals.clicks)}`}/>
              <MetricCard label="点击率" value={formatRate(overview.rates.ctr)}
                note={overview.ctr_interval
                  ? `95% 区间 ${formatRate(overview.ctr_interval.low)} ~ ${formatRate(overview.ctr_interval.high)}`
                  : '样本不足以给出区间'}/>
              <MetricCard label="转化成本" value={formatMoney(overview.rates.cpa_cents)}
                note={`转化 ${formatCount(overview.totals.conversions)} · ROAS ${formatRatio(overview.rates.roas)}`}/>
            </div>

            <div className="prelaunch-fact"><Layers3 size={17}/><span><small>数据窗口</small><b>
              {formatDate(overview.window.start)} ~ {formatDate(overview.window.end)} · 共 {series.length} 天有数据
            </b></span></div>
            <div className="prelaunch-fact"><Database size={17}/><span><small>口径（两个数字并排之前必须一致的东西）</small><b>
              {overview.caliber.currency} · 归因 {overview.caliber.attribution_window} · 指标口径 {overview.caliber.metric_schema_version} · 时区 {overview.caliber.time_zone}
            </b></span></div>
            <div className="prelaunch-fact">
              {overview.confidence === 'sufficient' ? <CircleCheck size={17}/> : <CircleAlert size={17}/>}
              <span><small>置信 · {confidenceLabels[overview.confidence]}</small><b>
                {overview.confidence_note || confidenceMeaning[overview.confidence]}
              </b></span>
            </div>

            <div className="insight-series">
              <span className="section-label">每天的花费与点击率 · {series.length} 个数据点</span>
              {series.map(point => <div className="insight-series-row" key={point.date}>
                <span>{formatDate(point.date)}</span>
                <span className="insight-series-track">
                  {/* 条形只表示花费的相对大小，用来一眼看出钱压在哪几天，不承担精确读数。 */}
                  <i style={{ width: `${Math.max(2, Math.round((point.counts.spend_cents / peakSpend) * 100))}%` }}/>
                </span>
                <span>{formatMoney(point.counts.spend_cents)}</span>
                <span>点击率 {formatRate(point.rates.ctr)}</span>
                <span>曝光 {formatCount(point.counts.impressions)}</span>
              </div>)}
            </div>

            <div className="prelaunch-table" role="list" aria-label="素材表现矩阵">
              <div className="prelaunch-row insight-asset-row header">
                <span>素材版本</span><span>花费</span><span>点击率</span><span>转化成本</span><span>置信</span>
              </div>
              {assets.map(asset => {
                const id = asset.asset_id ?? asset.asset_title
                return <button role="listitem" key={id} className={`prelaunch-row insight-asset-row${id === selectedId ? ' active' : ''}`} onClick={() => setSelectedId(id)}>
                  <span><b>{asset.asset_title}</b><small>{asset.objects} 个平台对象{asset.attributable ? '' : ' · 归属不确定'}</small></span>
                  <span>{formatMoney(asset.counts.spend_cents)}</span>
                  <span>{formatRate(asset.rates.ctr)}</span>
                  <span>{formatMoney(asset.rates.cpa_cents)}</span>
                  <span>{confidenceLabels[asset.confidence]}</span>
                </button>
              })}
              {assets.length ? null : <div className="panel-empty">
                这个窗口里的花费还没有认领到任何素材版本上，所以排不出素材矩阵。
              </div>}
            </div>
          </> : null}
      </section>

      <aside className="prelaunch-detail">
        {selected ? <>
          <span className="section-label">素材表现</span><h3>{selected.asset_title}</h3>
          <p>{selected.objects} 个平台对象 · 置信{confidenceLabels[selected.confidence]}</p>

          <div className="prelaunch-fact"><TrendingUp size={17}/><span><small>这一版的账</small><b>
            花 {formatMoney(selected.counts.spend_cents)}，换回 {formatCount(selected.counts.impressions)} 次曝光、
            {formatCount(selected.counts.clicks)} 次点击、{formatCount(selected.counts.conversions)} 个转化。
          </b></span></div>
          <div className="prelaunch-fact"><Layers3 size={17}/><span><small>派生指标</small><b>
            点击率 {formatRate(selected.rates.ctr)} · 转化率 {formatRate(selected.rates.cvr)} ·
            转化成本 {formatMoney(selected.rates.cpa_cents)} · 千次曝光成本 {formatMoney(selected.rates.cpm_cents)} ·
            ROAS {formatRatio(selected.rates.roas)}
          </b></span></div>
          {missingRates(selected.rates).length ? <div className="prelaunch-boundary"><CircleAlert size={16}/><span>
            <small>有指标算不出来</small>{missingRates(selected.rates).join('、')}的分母是 0，所以没有值。这不是「表现差」，是「这个问题问不出答案」，排序和比较都不能把它当 0。
          </span></div> : null}

          <div className="prelaunch-fact"><Lightbulb size={17}/><span><small>这个数字能用来做什么</small><b>
            {confidenceMeaning[selected.confidence]}
          </b></span></div>
          {selected.attributable ? null : <div className="prelaunch-boundary"><CircleAlert size={16}/><span>
            <small>归属不确定</small>这一版的部分花费来自还没认领的平台对象，数字只能当参考，不能拿去和别的版本比。
          </span></div>}
        </> : <div className="panel-empty">左侧选一个素材版本，查看它的账和这个数字能用来做什么。</div>}

        {overview?.sources.length ? <div className="feature-stack">
          <span>数据从哪来（{overview.sources.length} 个数据源）</span>
          {overview.sources.map(source => <b key={source.data_source_id}>
            {source.label} · {qualityLabels[source.quality_status]}
            {source.data_through ? ` · 数据到 ${formatDate(source.data_through)}` : ' · 还没有数据'}
            {source.freshness_days > 1 ? ` · 已滞后 ${source.freshness_days} 天` : ''}
            {source.quality_note ? ` · ${source.quality_note}` : ''}
          </b>)}
        </div> : null}

        {overview && overview.platforms.length > 1 ? <div className="feature-stack">
          <span>分平台</span>
          {overview.platforms.map(item => <b key={item.platform}>
            {item.label} · {formatMoney(item.counts.spend_cents)} · 点击率 {formatRate(item.rates.ctr)}
          </b>)}
        </div> : null}

        <div className="prelaunch-boundary"><CircleAlert size={16}/><span><small>这一页只回答「花了多少、换回什么」</small>
          谁比谁好、有没有跑不动、哪一天不对劲、和什么特征有关，在同一板块的另外五个视图里
          （素材对比 / 趋势 / 疲劳 / 异常 / 驱动因素）。它们的数据窗口各自独立选择。
        </span></div>

        {/* 总览也要能定格。人读完这一屏就想留档是最常见的路径——如果只有另外五个列表
            视图上有这个按钮，人得先切到一个自己并不想看的视图才能存下刚才看到的东西。 */}
        {overview && hasData ? <FreezeReportAction window={overview.window} busy={freezing} onFreeze={freeze}/> : null}

        {notice ? <div className="inline-notice" role="status">{notice}</div> : null}
      </aside>
    </div>
  </StateBoundary>
}

function MetricCard({ label, value, note }: { label: string; value: string; note: string }) {
  return <div className="insight-metric-card">
    <small>{label}</small>
    <b>{value}</b>
    <span>{note}</span>
  </div>
}

/** 缺失的派生指标：给出人话名字，用来解释「为什么这里是不可用」。 */
function missingRates(rates: ApiMetricRates): string[] {
  const names: Array<[keyof ApiMetricRates, string]> = [
    ['ctr', '点击率'],
    ['cvr', '转化率'],
    ['completion_rate', '完播率'],
    ['cpa_cents', '转化成本'],
    ['cpm_cents', '千次曝光成本'],
    ['roas', 'ROAS'],
  ]
  return names.filter(([key]) => rates[key] === undefined).map(([, label]) => label)
}

// 下面三个格式化函数共用一条规则：undefined 是「算不出来」，不是 0。
function formatRate(value?: number): string {
  return value === undefined ? '不可用' : `${(value * 100).toFixed(2)}%`
}

function formatMoney(cents?: number): string {
  // 金额固定两位小数：同一列里 ¥28 和 ¥201.67 并排会让人以为量级差了一位。
  return cents === undefined ? '不可用' : `¥${(cents / 100).toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`
}

function formatRatio(value?: number): string {
  return value === undefined ? '不可用' : `${value.toFixed(2)}×`
}

function isoDate(value: Date): string {
  return `${value.getFullYear()}-${String(value.getMonth() + 1).padStart(2, '0')}-${String(value.getDate()).padStart(2, '0')}`
}

function formatCount(value: number): string {
  return value.toLocaleString('zh-CN')
}

function formatDate(value: string): string {
  const time = new Date(value)
  return Number.isNaN(time.valueOf()) ? value : time.toLocaleDateString('zh-CN')
}
