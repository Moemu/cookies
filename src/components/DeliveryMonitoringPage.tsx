import { useCallback, useEffect, useState } from 'react'
import { Check, CircleAlert, Clock3, Database, Play, ShieldAlert, X } from 'lucide-react'
import { DeliveryApiError, deliveryAlertApi, type DeliveryAlert, type DeliveryAlertEvaluation, type DeliveryAlertFixture } from '../api/delivery'
import { useProject } from '../context/ProjectContext'
import { StateBoundary } from './StateBoundary'

const fixtures: Array<{ value: DeliveryAlertFixture; label: string }> = [
  { value: 'normal_day', label: '正常日' },
  { value: 'anomaly_day', label: '异常日' },
  { value: 'stale_data', label: '数据滞后' },
  { value: 'insufficient_data', label: '数据不足' },
]

const typeLabel: Record<DeliveryAlert['type'], string> = {
  review_rejected: '审核拒绝',
  spend_spike: '消耗突增',
  zero_conversion: '零转化',
  cost_worsening: '成本恶化',
}

const statusLabel: Record<DeliveryAlert['status'], string> = { open: '需行动', acknowledged: '已确认', dismissed: '已忽略' }
const freshnessLabel: Record<DeliveryAlert['freshness']['status'], string> = { fresh: '数据新鲜', stale: '数据滞后', unknown: '新鲜度未知', insufficient_data: '数据不足' }
const severityLabel: Record<DeliveryAlert['severity'], string> = { critical: '严重', high: '高', medium: '中', low: '低' }
const scenarioLabel: Record<DeliveryAlertFixture, string> = { normal_day: '正常日', anomaly_day: '异常日', stale_data: '数据滞后', insufficient_data: '数据不足' }

const alertCopy: Record<DeliveryAlert['type'], { summary: string; metricLabel: string }> = {
  review_rejected: { summary: '本次投放审核未通过，需要先核对拒绝原因，再决定后续处理。', metricLabel: '审核结果' },
  spend_spike: { summary: '当前消耗明显高于预警线，需要核对预算设置与投放节奏。', metricLabel: '消耗情况' },
  zero_conversion: { summary: '当前窗口尚未记录转化，需要检查转化链路和数据回传。', metricLabel: '转化情况' },
  cost_worsening: { summary: '当前转化成本超过预警线，需要检查定向、素材和预算。', metricLabel: '转化成本' },
}

export function DeliveryMonitoringPage() {
  const { currentProject } = useProject()
  const [alerts, setAlerts] = useState<DeliveryAlert[] | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [forbidden, setForbidden] = useState(false)
  const [fixture, setFixture] = useState<DeliveryAlertFixture>('normal_day')
  const [busyId, setBusyId] = useState<string | null>(null)
  const [lastEvaluation, setLastEvaluation] = useState<DeliveryAlertEvaluation | null>(null)

  const load = useCallback(async () => {
    setError(null)
    setForbidden(false)
    try {
      setAlerts(await deliveryAlertApi.list(currentProject.id))
    } catch (reason) {
      const message = reason instanceof Error ? reason.message : '无法读取监控告警。'
      setForbidden(reason instanceof DeliveryApiError && (reason.status === 403 || reason.code === 'PROJECT_ACCESS_DENIED'))
      setError(message)
      setAlerts(null)
    }
  }, [currentProject.id])

  useEffect(() => { void load() }, [load])

  const evaluate = async () => {
    setBusyId('evaluate')
    setError(null)
    try {
      const result = await deliveryAlertApi.evaluate(currentProject.id, fixture)
      setAlerts(result.items)
      setLastEvaluation(result)
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : '无法生成监控结果。')
    } finally {
      setBusyId(null)
    }
  }

  const update = async (alert: DeliveryAlert, action: 'acknowledge' | 'dismiss') => {
    setBusyId(alert.id)
    setError(null)
    try {
      const updated = await deliveryAlertApi.action(currentProject.id, alert.id, action, alert.version)
      setAlerts(current => current?.map(item => item.id === updated.id ? updated : item) ?? null)
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : '无法更新告警状态。')
    } finally {
      setBusyId(null)
    }
  }

  const state = forbidden ? 'forbidden' : error && alerts === null ? 'error' : alerts === null ? 'loading' : alerts.length === 0 ? 'empty' : 'ready'
  return <section className="delivery-monitoring" aria-label="投放监控告警">
      <header className="delivery-monitoring__header">
        <div><span className="section-label">Delivery monitoring</span><h2>监控告警</h2><p>告警、状态和证据均来自服务端评估；演示 fixture 仅用于模拟数据，不能代表真实平台同步或投放效果。</p></div>
        <div className="delivery-monitoring__controls"><label>演示场景<select value={fixture} onChange={event => setFixture(event.target.value as DeliveryAlertFixture)}>{fixtures.map(option => <option key={option.value} value={option.value}>{option.label}</option>)}</select></label><button className="secondary-button" disabled={busyId !== null} onClick={() => void evaluate()}><Play size={14} fill="currentColor"/>{busyId === 'evaluate' ? '评估中…' : '运行评估'}</button></div>
      </header>
      {error && alerts !== null ? <div className="delivery-monitoring__error" role="alert"><CircleAlert size={15}/>{error}</div> : null}
      <div className="delivery-monitoring__notice"><Database size={15}/><span>数据来源由每条记录标识。只有显示“demo_fixture / 模拟数据”的记录才是演示数据。</span></div>
      {lastEvaluation?.scenario === 'stale_data' || lastEvaluation?.scenario === 'insufficient_data' ? <div className="delivery-monitoring__caution" role="status"><ShieldAlert size={15}/>{lastEvaluation.scenario === 'stale_data' ? '数据滞后：本次仅标记数据新鲜度，不生成确定性异常或优化动作。' : '数据不足：本次仅标记缺失或样本不足，不生成确定性异常或优化动作。'}</div> : null}
      <StateBoundary state={state} contextLabel="投放 / 监控告警" emptyTitle="暂无服务端监控告警" emptyDetail="尚未返回当前 Project 的告警记录。可选择演示 fixture 后运行一次评估。" onRetry={() => void load()}>
        <div className="delivery-monitoring__list">{alerts?.map(alert => <AlertCard key={alert.id} alert={alert} busy={busyId === alert.id} onAction={update}/>)}</div>
      </StateBoundary>
    </section>
}

function AlertCard({ alert, busy, onAction }: { alert: DeliveryAlert; busy: boolean; onAction: (alert: DeliveryAlert, action: 'acknowledge' | 'dismiss') => Promise<void> }) {
  const actionNeeded = alert.status === 'open'
  const copy = alertCopy[alert.type]
  return <article className={`delivery-alert-card severity-${alert.severity}`}>
    <header><div><span className="delivery-alert-card__severity">风险等级：{severityLabel[alert.severity]}</span><span className="delivery-alert-card__type">模拟告警</span><h3>{typeLabel[alert.type]}</h3></div><span className={`delivery-alert-card__status status-${alert.status}`}>{statusLabel[alert.status]}</span></header>
    <p className="delivery-alert-card__summary">{copy.summary}</p>
    <dl><div><dt>监控窗口</dt><dd>{formatTime(alert.window.start)} 至 {formatTime(alert.window.end)} · {alert.window.timezone}</dd></div><div><dt>数据覆盖到</dt><dd>{formatTime(alert.window.dataThrough)} · {freshnessLabel[alert.freshness.status]}（截至 {formatTime(alert.freshness.asOf)}，评估于 {formatTime(alert.freshness.evaluatedAt)}）</dd></div><div><dt>{copy.metricLabel}</dt><dd>{describeMetric(alert)}</dd></div><div><dt>负责人</dt><dd>{describeOwner(alert)}</dd></div><div><dt>证据来源</dt><dd>{describeEvidence(alert)}</dd></div><div><dt>数据标识</dt><dd>{alert.isSimulated ? '模拟数据' : '非模拟数据'} · {scenarioLabel[alert.scenario]}场景 · 预设投放数据集</dd></div></dl>
    <details className="delivery-alert-card__technical"><summary>查看技术标识</summary><span>规则：{alert.ruleId} v{alert.ruleVersion} · 数据集：{alert.datasetVersion} · 场景版本：{alert.fixtureVersion}</span></details>
    {alert.freshness.status === 'stale' || alert.freshness.status === 'insufficient_data' ? <div className="delivery-alert-card__caution"><ShieldAlert size={15}/>{alert.freshness.status === 'stale' ? `数据滞后：已超过最大允许新鲜度 ${alert.freshness.maxAgeSeconds} 秒，请先确认数据覆盖时间，再据此做业务判断。` : `数据不足：缺少 ${alert.freshness.missingMetrics?.join('、') || '服务端标记的必要指标'}，当前评估不构成投放效果结论。`}</div> : null}
    {actionNeeded ? <footer><button className="secondary-button" disabled={busy} onClick={() => void onAction(alert, 'acknowledge')}><Check size={14}/>确认跟进</button><button className="secondary-button" disabled={busy} onClick={() => void onAction(alert, 'dismiss')}><X size={14}/>忽略</button></footer> : <footer><Clock3 size={14}/>{alert.status === 'acknowledged' ? '已确认，告警仍保留以供跟进。' : '已忽略，服务端保留该处置记录。'}</footer>}
  </article>
}

function formatTime(value: string) { const date = new Date(value); return Number.isNaN(date.valueOf()) ? value : date.toLocaleString('zh-CN', { hour12: false }) }
function displayMetric(value: number | undefined, unit: string) { if (value === undefined) return '未提供'; if (unit === 'CNY_cents') return `¥${(value / 100).toLocaleString('zh-CN', { minimumFractionDigits: 0, maximumFractionDigits: 2 })}`; return value.toLocaleString('zh-CN') }
function formatNumber(value: number) { return value.toLocaleString('zh-CN') }
function describeMetric(alert: DeliveryAlert) {
  const metric = alert.metricDefinition
  switch (alert.type) {
    case 'review_rejected': return '审核结果为“拒绝”。'
    case 'spend_spike': return `当前消耗 ${displayMetric(metric.observedValue, metric.unit)}；预警线 ${displayMetric(metric.threshold, metric.unit)}；基准窗口 ${displayMetric(metric.baselineValue, metric.unit)}。`
    case 'zero_conversion': return `转化 ${formatNumber(metric.numerator ?? metric.observedValue ?? 0)} 次；已点击 ${formatNumber(metric.denominator ?? 0)} 次；预警线为少于 ${formatNumber(metric.threshold ?? 1)} 次转化。`
    case 'cost_worsening': return `当前每次转化成本 ${displayMetric(metric.observedValue, metric.unit)}；预警线 ${displayMetric(metric.threshold, metric.unit)}；基准窗口 ${displayMetric(metric.baselineValue, metric.unit)}。`
  }
}
function describeOwner(alert: DeliveryAlert) { return alert.owner.source === 'actor_context' ? `${alert.owner.displayName}（本地演示会话）` : alert.owner.displayName }
function describeEvidence(alert: DeliveryAlert) { return alert.evidenceRefs.length ? alert.evidenceRefs.map(reference => reference.startsWith('mock://metric/') ? '演示指标快照' : reference.startsWith('mock://fixture/') ? `演示场景：${scenarioLabel[alert.scenario]}` : '服务端证据记录').join(' · ') : '服务端未提供证据记录' }
