import { useCallback, useEffect, useRef, useState } from 'react'
import { Check, CircleAlert, Clock3, Database, Play, ShieldAlert, X } from 'lucide-react'
import { DeliveryApiError, deliveryAlertApi, deliveryExecutionApi, type DeliveryAlert, type DeliveryAlertEvaluation, type DeliveryAlertFixture, type DeliveryOutcomeScenario, type DeliveryOutcomeSimulation } from '../api/delivery'
import { useProject } from '../context/ProjectContext'
import { StateBoundary } from './StateBoundary'

const fixtures: Array<{ value: DeliveryAlertFixture; label: string }> = [
  { value: 'normal_day', label: '正常日' },
  { value: 'anomaly_day', label: '异常日' },
  { value: 'stale_data', label: '数据滞后' },
  { value: 'insufficient_data', label: '数据不足' },
]

const outcomeScenarios: Array<{ value: DeliveryOutcomeScenario; label: string; detail: string }> = [
  { value: 'steady', label: '稳定投放', detail: '预算与效率在正常波动范围内。' },
  { value: 'cost_pressure', label: '成本承压', detail: '竞价成本上升、转化效率下降。' },
  { value: 'under_delivery', label: '跑量不足', detail: '可用预算未能转化为足量曝光。' },
  { value: 'creative_fatigue', label: '素材疲劳', detail: '点击率与转化率随窗口衰减。' },
  { value: 'tracking_anomaly', label: '追踪异常', detail: '点击存在，但转化回传中断。' },
  { value: 'review_rejected', label: '审核拒绝', detail: '平台审核事件阻止后续投放。' },
]

const typeLabel: Record<DeliveryAlert['type'], string> = {
  review_rejected: '审核拒绝',
  spend_spike: '消耗突增',
  zero_conversion: '零转化',
  cost_worsening: '成本恶化',
  under_delivery: '跑量不足',
  creative_fatigue: '素材疲劳',
  tracking_anomaly: '追踪异常',
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
  under_delivery: { summary: '当前消耗和曝光低于可用预算应支持的规模，需要检查竞价与定向边界。', metricLabel: '消耗规模' },
  creative_fatigue: { summary: '素材点击率与转化率在连续窗口中下降，需要评估素材轮换。', metricLabel: '点击效率' },
  tracking_anomaly: { summary: '存在点击但没有追踪到转化，需要检查回传链路。', metricLabel: '追踪转化' },
}

export function DeliveryMonitoringPage({ tourCase }: { tourCase?: string }) {
  const { currentProject } = useProject()
  const targetPlanId = new URLSearchParams(window.location.search).get('plan_id') ?? ''
  const [alerts, setAlerts] = useState<DeliveryAlert[] | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [forbidden, setForbidden] = useState(false)
  const [fixture, setFixture] = useState<DeliveryAlertFixture>('normal_day')
  const [outcomeScenario, setOutcomeScenario] = useState<DeliveryOutcomeScenario>(tourCase === 'review_rejected_alert' ? 'review_rejected' : 'cost_pressure')
  const [stableSeed, setStableSeed] = useState(() => new URLSearchParams(window.location.search).get('tour_run_id') ?? 'a07-default-seed')
  const [executionId, setExecutionId] = useState<string>()
  const [simulation, setSimulation] = useState<DeliveryOutcomeSimulation>()
  const [busyId, setBusyId] = useState<string | null>(null)
  const [lastEvaluation, setLastEvaluation] = useState<DeliveryAlertEvaluation | null>(null)
  const loadGeneration = useRef(0)

  const load = useCallback(async () => {
    const generation = ++loadGeneration.current
    setError(null)
    setForbidden(false)
    try {
      const [records, executions] = await Promise.all([
        deliveryAlertApi.list(currentProject.id, targetPlanId ? { planId: targetPlanId } : {}),
        targetPlanId ? deliveryExecutionApi.list(currentProject.id) : Promise.resolve([]),
      ])
      const succeededExecution = executions.find(value => value.changeSet.planId === targetPlanId && value.execution.status === 'succeeded')
      let latestSimulation: DeliveryOutcomeSimulation | undefined
      if (succeededExecution) {
        try {
          latestSimulation = await deliveryExecutionApi.getLatestOutcomeSimulation(currentProject.id, succeededExecution.execution.id)
        } catch (reason) {
          if (!(reason instanceof DeliveryApiError && reason.status === 404)) throw reason
        }
      }
      if (generation !== loadGeneration.current) return
      setExecutionId(succeededExecution?.execution.id)
      setSimulation(latestSimulation)
      setAlerts(targetPlanId
        ? records.filter(alert => alert.planId === targetPlanId && (!latestSimulation || alert.simulationRunId === latestSimulation.run.id))
        : records)
    } catch (reason) {
      if (generation !== loadGeneration.current) return
      const message = reason instanceof Error ? reason.message : '无法读取监控告警。'
      setForbidden(reason instanceof DeliveryApiError && (reason.status === 403 || reason.code === 'PROJECT_ACCESS_DENIED'))
      setError(message)
      setAlerts(null)
    }
  }, [currentProject.id, targetPlanId])

  useEffect(() => { void load() }, [load])

  const runSimulation = async () => {
    if (!executionId) {
      setError('当前计划还没有成功的平台操作演练；请先完成首次批准与演练。')
      return
    }
    ++loadGeneration.current
    setBusyId('simulation')
    setError(null)
    try {
      const result = await deliveryExecutionApi.runOutcomeSimulation(currentProject.id, executionId, outcomeScenario, stableSeed.trim())
      setSimulation(result)
      setAlerts([])
      setLastEvaluation(null)
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : '无法运行投放效果情景模拟。')
    } finally {
      setBusyId(null)
    }
  }

  const evaluate = async () => {
    ++loadGeneration.current
    setBusyId('evaluate')
    setError(null)
    try {
      if (targetPlanId && !executionId) throw new Error('当前计划还没有成功的平台操作演练；请先完成首次批准与演练。')
      if (targetPlanId && !simulation) throw new Error('请先运行投放效果情景模拟，再根据同一批指标运行告警规则。')
      const evaluationFixture = tourCase === 'review_rejected_alert' ? 'anomaly_day' : fixture
      const result = await deliveryAlertApi.evaluate(currentProject.id, evaluationFixture, executionId)
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
        <div><span className="section-label">上线后证据</span><h2>投放效果情景模拟与告警</h2><p>平台操作演练只验证操作结果；这里的规则模型根据计划配置生成可重复的投后指标，再由同一批指标触发告警与优化建议。</p></div>
      </header>
      {error && alerts !== null ? <div className="delivery-monitoring__error" role="alert"><CircleAlert size={15}/>{error}</div> : null}
      <section className="delivery-simulation-workspace" aria-label="投放效果情景模拟">
        <header><div><span className="section-label">投后演练</span><h3>运行确定性情景</h3><p>这不是效果预测。相同 PlanVersion、配置、场景与 seed 得到相同结果；改变预算、出价、定向或素材特征会产生可解释变化。</p></div><span className={`delivery-simulation-status ${simulation ? 'is-complete' : ''}`}>{simulation ? '已有模拟结果' : executionId ? '等待运行' : '等待平台操作演练'}</span></header>
        <div className="delivery-simulation-controls">
          <label>情景假设<select value={outcomeScenario} onChange={event => setOutcomeScenario(event.target.value as DeliveryOutcomeScenario)}>{outcomeScenarios.map(option => <option key={option.value} value={option.value}>{option.label}</option>)}</select><small>{outcomeScenarios.find(option => option.value === outcomeScenario)?.detail}</small></label>
          <label>稳定 seed<input value={stableSeed} maxLength={128} onChange={event => setStableSeed(event.target.value)} /><small>用于重放有界扰动，不代替业务参数。</small></label>
          <button className="primary-button" disabled={busyId !== null || !executionId} onClick={() => void runSimulation()}><Play size={14} fill="currentColor"/>{busyId === 'simulation' ? '模拟中…' : '运行投放效果情景模拟'}</button>
        </div>
        {simulation ? <SimulationResult value={simulation}/> : <div className="delivery-simulation-empty"><Database size={17}/><span>尚未产生投后指标。运行后将显示输入摘要、模型因子、三段指标窗口和模拟事件。</span></div>}
      </section>
      <div className="delivery-monitoring__notice"><Database size={15}/><span>证据链：PlanVersion → SimulationRun → MetricSnapshots → Alerts → Recommendations。当前 SimulationRun：{simulation?.run.id ?? '尚未生成'}。</span></div>
      <div className="delivery-monitoring__controls">{!targetPlanId ? <label>异常测试<select value={fixture} onChange={event => setFixture(event.target.value as DeliveryAlertFixture)}>{fixtures.map(option => <option key={option.value} value={option.value}>{option.label}</option>)}</select></label> : null}<button className="secondary-button" disabled={busyId !== null || (Boolean(targetPlanId) && !simulation)} onClick={() => void evaluate()}><Play size={14} fill="currentColor"/>{busyId === 'evaluate' ? '评估中…' : '根据本次指标运行告警规则'}</button></div>
      {lastEvaluation?.scenario === 'stale_data' || lastEvaluation?.scenario === 'insufficient_data' ? <div className="delivery-monitoring__caution" role="status"><ShieldAlert size={15}/>{lastEvaluation.scenario === 'stale_data' ? '数据滞后：本次仅标记数据新鲜度，不生成确定性异常或优化动作。' : '数据不足：本次仅标记缺失或样本不足，不生成确定性异常或优化动作。'}</div> : null}
      <StateBoundary state={state} contextLabel="投放 / 监控告警" emptyTitle="当前还没有告警" emptyDetail={simulation ? '情景模拟已产生指标；运行告警规则后，这里只显示由本次 SimulationRun 触发的结果。' : '这是正常初始状态。先运行投放效果情景模拟，再根据同一批指标运行告警规则。'} onRetry={() => void load()}>
        <div className="delivery-monitoring__list">{alerts?.map(alert => <AlertCard key={alert.id} alert={alert} busy={busyId === alert.id} onAction={update}/>)}</div>
      </StateBoundary>
    </section>
}

function SimulationResult({ value }: { value: DeliveryOutcomeSimulation }) {
  const input = value.run.input
  return <div className="delivery-simulation-result">
    <div className="delivery-simulation-summary"><dl><div><dt>计划版本</dt><dd>V{value.run.planVersion}</dd></div><div><dt>预算</dt><dd>{money(input.budgetMinor)}</dd></div><div><dt>出价</dt><dd>{money(input.bidMinor)}</dd></div><div><dt>优化目标</dt><dd>{input.optimizationGoal || '未指定'}</dd></div><div><dt>素材引用</dt><dd>{input.creativeCount} 个版本</dd></div><div><dt>模型版本</dt><dd>{value.run.modelVersion}</dd></div></dl></div>
    <div className="delivery-simulation-metrics">{value.metricSnapshots.map(metric => <article key={metric.id}><header><span>窗口 {metric.windowSequence}</span><time>{formatTime(metric.windowEnd)}</time></header><dl><div><dt>曝光</dt><dd>{formatNumber(metric.impressions)}</dd></div><div><dt>点击</dt><dd>{formatNumber(metric.clicks)}</dd></div><div><dt>转化</dt><dd>{formatNumber(metric.conversions)}</dd></div><div><dt>消耗</dt><dd>{money(metric.spendCents)}</dd></div><div><dt>模拟收入</dt><dd>{money(metric.revenueCents)}</dd></div></dl><details><summary>查看计算依据</summary><p>{metric.calculationBasis.formula}</p><span>消耗 ×{bp(metric.calculationBasis.spendMultiplierBP)} · 触达 ×{bp(metric.calculationBasis.reachMultiplierBP)} · CTR ×{bp(metric.calculationBasis.ctrMultiplierBP)} · CVR ×{bp(metric.calculationBasis.cvrMultiplierBP)} · 追踪 {bp(metric.calculationBasis.trackingRateBP)}</span></details></article>)}</div>
    <div className="delivery-simulation-explanation"><div><h4>输入如何影响结果</h4>{value.run.parameters.factors.map(factor => <p key={factor.key}><strong>{factor.key} · {bp(factor.valueBP)}</strong><span>{factor.explanation}</span></p>)}</div><div><h4>本次模拟事件</h4>{value.run.events.length ? value.run.events.map(event => <p key={`${event.type}-${event.windowSequence}`}><strong>{event.type} · 窗口 {event.windowSequence}</strong><span>{event.explanation}</span></p>) : <p><span>当前情景没有产生异常事件。</span></p>}</div></div>
  </div>
}

function AlertCard({ alert, busy, onAction }: { alert: DeliveryAlert; busy: boolean; onAction: (alert: DeliveryAlert, action: 'acknowledge' | 'dismiss') => Promise<void> }) {
  const actionNeeded = alert.status === 'open'
  const copy = alertCopy[alert.type]
  return <article className={`delivery-alert-card severity-${alert.severity}`}>
    <header><div><span className="delivery-alert-card__severity">风险等级：{severityLabel[alert.severity]}</span><h3>{typeLabel[alert.type]}</h3></div><span className={`delivery-alert-card__status status-${alert.status}`}>{statusLabel[alert.status]}</span></header>
    <p className="delivery-alert-card__summary">{copy.summary}</p>
    <dl><div><dt>监控窗口</dt><dd>{formatTime(alert.window.start)} 至 {formatTime(alert.window.end)} · {alert.window.timezone}</dd></div><div><dt>数据覆盖到</dt><dd>{formatTime(alert.window.dataThrough)} · {freshnessLabel[alert.freshness.status]}（截至 {formatTime(alert.freshness.asOf)}，评估于 {formatTime(alert.freshness.evaluatedAt)}）</dd></div><div><dt>{copy.metricLabel}</dt><dd>{describeMetric(alert)}</dd></div><div><dt>证据来源</dt><dd>{describeEvidence(alert)}</dd></div></dl>
    <details className="delivery-alert-card__technical"><summary>查看技术标识</summary><span>规则：{alert.ruleId} v{alert.ruleVersion} · 数据集：{alert.datasetVersion} · 场景版本：{alert.fixtureVersion}</span></details>
    {alert.freshness.status === 'stale' || alert.freshness.status === 'insufficient_data' ? <div className="delivery-alert-card__caution"><ShieldAlert size={15}/>{alert.freshness.status === 'stale' ? `数据滞后：已超过最大允许新鲜度 ${alert.freshness.maxAgeSeconds} 秒，请先确认数据覆盖时间，再据此做业务判断。` : `数据不足：缺少 ${alert.freshness.missingMetrics?.join('、') || '服务端标记的必要指标'}，当前评估不构成投放效果结论。`}</div> : null}
    {actionNeeded ? <footer><button className="secondary-button" disabled={busy} onClick={() => void onAction(alert, 'acknowledge')}><Check size={14}/>确认跟进</button><button className="secondary-button" disabled={busy} onClick={() => void onAction(alert, 'dismiss')}><X size={14}/>忽略</button></footer> : <footer><Clock3 size={14}/>{alert.status === 'acknowledged' ? '已确认，告警仍保留以供跟进。' : '已忽略，服务端保留该处置记录。'}</footer>}
  </article>
}

function formatTime(value: string) { const date = new Date(value); return Number.isNaN(date.valueOf()) ? value : date.toLocaleString('zh-CN', { hour12: false }) }
function money(value: number) { return `¥${(value / 100).toLocaleString('zh-CN', { minimumFractionDigits: 0, maximumFractionDigits: 2 })}` }
function bp(value: number) { return (value / 10_000).toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 }) }
function displayMetric(value: number | undefined, unit: string) { if (value === undefined) return '未提供'; if (unit === 'CNY_cents') return `¥${(value / 100).toLocaleString('zh-CN', { minimumFractionDigits: 0, maximumFractionDigits: 2 })}`; return value.toLocaleString('zh-CN') }
function formatNumber(value: number) { return value.toLocaleString('zh-CN') }
function describeMetric(alert: DeliveryAlert) {
  const metric = alert.metricDefinition
  switch (alert.type) {
    case 'review_rejected': return '审核结果为“拒绝”。'
    case 'spend_spike': return `当前消耗 ${displayMetric(metric.observedValue, metric.unit)}；预警线 ${displayMetric(metric.threshold, metric.unit)}；基准窗口 ${displayMetric(metric.baselineValue, metric.unit)}。`
    case 'zero_conversion': return `转化 ${formatNumber(metric.numerator ?? metric.observedValue ?? 0)} 次；已点击 ${formatNumber(metric.denominator ?? 0)} 次；预警线为少于 ${formatNumber(metric.threshold ?? 1)} 次转化。`
    case 'cost_worsening': return `当前每次转化成本 ${displayMetric(metric.observedValue, metric.unit)}；预警线 ${displayMetric(metric.threshold, metric.unit)}；基准窗口 ${displayMetric(metric.baselineValue, metric.unit)}。`
    case 'under_delivery': return `当前消耗 ${displayMetric(metric.observedValue, metric.unit)}；跑量下限 ${displayMetric(metric.threshold, metric.unit)}。`
    case 'creative_fatigue': return `当前点击 ${formatNumber(metric.numerator ?? 0)} 次，曝光 ${formatNumber(metric.denominator ?? 0)} 次。`
    case 'tracking_anomaly': return `追踪转化 ${formatNumber(metric.numerator ?? 0)} 次；已点击 ${formatNumber(metric.denominator ?? 0)} 次。`
  }
}
function describeEvidence(alert: DeliveryAlert) { return alert.evidenceRefs.length ? alert.evidenceRefs.map(reference => reference.startsWith('simulation://metric/') ? '持久指标窗口' : reference.startsWith('simulation://run/') ? '投放效果情景模拟' : reference.startsWith('simulation://execution/') ? '平台操作演练' : reference.startsWith('simulation://platform-event/') ? '平台事件场景' : '服务端证据记录').join(' · ') : '服务端未提供证据记录' }
