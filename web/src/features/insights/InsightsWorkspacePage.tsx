import { useCallback, useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import type { InsightEntry, InsightExperienceView } from '../../app/routes'
import { insightViewPath } from '../../app/routes'
import { ApiProblem } from '../../shared/api/client'
import type { Project } from '../platform/types'
import {
  confirmInsightReport,
  createExperience,
  createInsightReport,
  getPerformanceOverview,
  getPreLaunchInsights,
  listExperiences,
  listInsightReports,
} from './api'
import { ExperienceLibrary, ExperienceReferencesView } from './ExperienceLibrary'
import { experienceStatusLabels, groupExperiencesByStatus, toLines } from './experience-format'
import { InsightPageFrame, InsightToolbar } from './InsightPageFrame'
import type { DeliveryExecutionSnapshot, Experience, InsightReport, PerformanceOverview, PreLaunchInsight } from './types'
import '../outcomes/outcome-workspace.css'

/** entry/view 由路由层校验过：只有 built 的二级视图才会走到这里。 */
export function InsightsWorkspacePage({ entry, project, view }: { entry: InsightEntry, project?: Project, view: string }) {
  const { projectId = '' } = useParams()
  const section = entry.key
  const [reports, setReports] = useState<InsightReport[]>([])
  const [experiences, setExperiences] = useState<Experience[]>([])
  const [preLaunch, setPreLaunch] = useState<PreLaunchInsight | null>(null)
  const [performance, setPerformance] = useState<PerformanceOverview | null>(null)
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  const load = useCallback(async (signal?: AbortSignal) => {
    setError('')
    try {
      const [reportValues, experienceValues, preLaunchValue, performanceValue] = await Promise.all([
        listInsightReports(projectId, signal),
        listExperiences(projectId, undefined, signal),
        getPreLaunchInsights(projectId, signal),
        getPerformanceOverview(projectId, signal),
      ])
      setReports(reportValues.items)
      setExperiences(experienceValues.items)
      setPreLaunch(preLaunchValue)
      setPerformance(performanceValue)
    } catch (caught) {
      if (caught instanceof DOMException && caught.name === 'AbortError') return
      setError(caught instanceof Error ? caught.message : '无法读取洞察工作区。')
    } finally {
      if (!signal?.aborted) setLoading(false)
    }
  }, [projectId])

  useEffect(() => {
    const controller = new AbortController()
    const timer = window.setTimeout(() => void load(controller.signal), 0)
    return () => { window.clearTimeout(timer); controller.abort() }
  }, [load])

  const act = async (operation: () => Promise<unknown>) => {
    if (busy) return
    setBusy(true)
    setError('')
    try {
      await operation()
      await load()
    } catch (caught) {
      if (caught instanceof ApiProblem && caught.problem.error.code === 'VERSION_CONFLICT') {
        setError('该记录已被其他操作更新，页面已刷新为最新版本，请确认后重试。')
        await load()
      } else {
        setError(caught instanceof Error ? caught.message : '洞察操作未完成。')
      }
    } finally {
      setBusy(false)
    }
  }

  const grouped = groupExperiencesByStatus(experiences)
  // 只有经验库的页签需要条数；引用记录没有"多少条经验"的概念，不给数字。
  const experienceCounts: Record<string, number> = {
    pending: grouped.pending.length,
    confirmed: grouped.confirmed.length,
    'needs-review': grouped.needs_review.length,
    retired: grouped.retired.length,
  }
  // 经验只带证据主键，人话版本在报告里；投前卡片直接展示报告写好的证据说明。
  const evidenceSummaries = new Map(reports.map((item) => [item.id, item.evidence_summary]))

  return <InsightPageFrame
    counts={section === 'experiences' ? experienceCounts : undefined}
    entry={entry}
    project={project}
    view={view}
  >
    {error ? <div className="workspace-alert" role="alert"><span>{error}</span><button className="text-action" onClick={() => void load()} type="button">重试</button></div> : null}
    {loading ? <div className="outcome-loading" role="status">正在读取项目洞察与证据…</div> : null}

    {!loading && section === 'prelaunch' && view === 'strategy-evidence' ? <PreLaunchView
      disclosure={preLaunch?.disclosure || '正在读取已确认经验。'}
      evidence={evidenceSummaries}
      experiences={preLaunch?.experience_references || []}
      project={project}
      projectId={projectId}
    /> : null}

    {!loading && section === 'performance' && view === 'metrics' ? <div className="insight-panel">
      <InsightToolbar
        description="每一次投放执行都会留下一份指标快照，这里按时间列出来，作为复盘的证据来源。"
        label="投放之后"
        project={project}
        title="这次投出去，实际拿到了什么数据"
      />
      <div className="insight-panel__body">
        <Disclosure>{performance?.disclosure || '正在读取执行证据。'}</Disclosure>
        <section className="outcome-cards">
          {performance?.executions.map((item) => <ExecutionCard item={item} key={item.id} />)}
          {performance?.executions.length === 0 ? <EmptyState title="还没有投后数据" description="现在只接收本地模拟投放产生的证据；接入真实平台数据要等「数据接入」那一组做完。" /> : null}
        </section>
      </div>
    </div> : null}

    {!loading && section === 'reports' ? <div className="insight-panel">
      <InsightToolbar
        description="先把一次投放执行整理成复盘报告，确认之后再决定要不要沉淀成可以复用的经验。"
        label="复盘与沉淀"
        project={project}
        title="把这次投放的结果写成结论"
      />
      <div className="insight-panel__body">
        <ReportsView busy={busy} executions={performance?.executions || []} experiences={experiences} onAction={act} projectId={projectId} reports={reports} />
      </div>
    </div> : null}

    {!loading && (view === 'references' || section === 'experiences') ? <div className="insight-panel">
      <InsightToolbar
        description={view === 'references'
          ? '这里记录每条结论被哪个环节用过、用完的结果是什么，用来判断它到底有没有价值。'
          : '一条经验从待确认到已确认、再到失效，全过程都留在这里可以回溯。'}
        label={entry.label}
        project={project}
        title={view === 'references' ? '这些结论被谁用过' : '可以复用的结论都在这里'}
      />
      <div className="insight-panel__body">
        {view === 'references'
          ? <ExperienceReferencesView experiences={experiences} projectId={projectId} />
          : experiences.length > 0
            ? <ExperienceLibrary busy={busy} evidence={evidenceSummaries} experiences={experiences} onAction={act} projectId={projectId} view={view as Exclude<InsightExperienceView, 'references'>} />
            : <EmptyState title="还没有沉淀经验" description="确认复盘报告后，填写适用条件和反例，才能形成可复用的结论。" />}
      </div>
    </div> : null}
  </InsightPageFrame>
}

function ReportsView({ busy, executions, experiences, onAction, projectId, reports }: {
  busy: boolean
  executions: DeliveryExecutionSnapshot[]
  experiences: Experience[]
  onAction: (operation: () => Promise<unknown>) => void
  projectId: string
  reports: InsightReport[]
}) {
  const [composing, setComposing] = useState('')
  const reported = new Set(reports.map((item) => item.execution_id))
  // 已废除的经验不算"已沉淀"：结论失效后，同一份报告应当可以重新沉淀。
  const liveExperiences = new Map<string, Experience>()
  for (const item of experiences) if (item.status !== 'retired') liveExperiences.set(item.report_id, item)
  return <div className="outcome-grid">
    <aside className="outcome-list">
      <div className="outcome-list__heading"><strong>投放执行</strong><span>{executions.length}</span></div>
      {/* 列表按完成时间从新到旧，所以序号要倒着数，「第 1 次投放」才是最早那次。 */}
      {executions.map((item, index) => <div className="outcome-list__item" key={item.id}>
        <strong>第 {executions.length - index} 次投放</strong>
        <span>{reported.has(item.id) ? '已建立报告' : '等待复盘'}</span>
      </div>)}
      {executions.length === 0 ? <p className="outcome-list__empty">还没有投放执行。</p> : null}
    </aside>
    <main className="outcome-detail">
      {executions.filter((item) => !reported.has(item.id)).map((item) => <AutomatedReportComposer busy={busy} execution={item} key={item.id} onCreate={() => onAction(() => createInsightReport(projectId, { execution_id: item.id }))} />)}
      {reports.map((report) => <article className="report-card" key={report.id}>
        <div className="outcome-card__top"><span className="status-chip status-chip--active">{report.status === 'confirmed' ? '已确认' : '待确认'}</span><span className="status-chip">第 {report.version} 版</span></div>
        <h2>{report.summary}</h2>
        <ul>{report.findings.map((item) => <li key={item}>{item}</li>)}</ul>
        <Disclosure>{report.evidence_summary}</Disclosure>
        <div className="report-card__actions">
          {report.status === 'draft' ? <button className="button button--primary button--compact" disabled={busy} onClick={() => onAction(() => confirmInsightReport(projectId, report.id, report.version))} type="button">确认报告</button> : null}
          {report.status === 'confirmed' && !liveExperiences.has(report.id) && composing !== report.id
            ? <button className="button button--secondary button--compact" disabled={busy} onClick={() => setComposing(report.id)} type="button">沉淀为经验</button>
            : null}
          {liveExperiences.has(report.id)
            ? <span className="status-chip status-chip--active">经验{experienceStatusLabels[liveExperiences.get(report.id)!.status]}</span>
            : null}
        </div>
        {composing === report.id ? <ExperienceComposer busy={busy} report={report} onCancel={() => setComposing('')} onSubmit={(input) => {
          setComposing('')
          onAction(() => createExperience(projectId, report.id, { expected_report_version: report.version, ...input }))
        }} /> : null}
      </article>)}
      {executions.length === 0 && reports.length === 0 ? <EmptyState title="等待投放执行" description="先到「智能投放」里跑一次模拟投放，这里才有证据可以复盘。" /> : null}
    </main>
  </div>
}

function AutomatedReportComposer({ busy, execution, onCreate }: { busy: boolean, execution: DeliveryExecutionSnapshot, onCreate: () => void }) {
  const metrics = execution.metric_snapshot?.raw_metrics
  return <section className="report-composer">
    <span className="page-eyebrow">有证据来源的复盘</span>
    <h2>为这次投放执行生成模拟复盘</h2>
    <p>{execution.evidence_summary}</p>
    {metrics ? <dl className="outcome-facts">
      <div><dt>曝光</dt><dd>{metrics.impressions.toLocaleString()}</dd></div>
      <div><dt>点击</dt><dd>{metrics.clicks.toLocaleString()}</dd></div>
      <div><dt>转化</dt><dd>{metrics.conversions.toLocaleString()}</dd></div>
    </dl> : <p>请先在智能投放中生成模拟指标快照。</p>}
    <Disclosure>报告摘要和点击率、转化率由服务端根据演示数据集自动计算；模拟数据不代表真实广告效果。</Disclosure>
    <button className="button button--primary button--compact" disabled={busy || !metrics} onClick={onCreate} type="button">生成复盘草稿</button>
  </section>
}

function ExperienceComposer({ busy, onCancel, onSubmit, report }: {
  busy: boolean
  onCancel: () => void
  onSubmit: (input: { conclusion: string, conditions: string[], counterexamples: string[] }) => void
  report: InsightReport
}) {
  const [conclusion, setConclusion] = useState(report.summary)
  const [conditions, setConditions] = useState('')
  const [counterexamples, setCounterexamples] = useState('')
  return <form className="report-composer" onSubmit={(event) => {
    event.preventDefault()
    onSubmit({ conclusion: conclusion.trim(), conditions: toLines(conditions), counterexamples: toLines(counterexamples) })
  }}>
    <span className="page-eyebrow">沉淀新经验</span>
    <h2>把这份报告沉淀为可复用经验</h2>
    <p>适用条件决定这条结论在什么情况下可以被引用，反例决定它什么时候不该被引用。沉淀后先进入待确认，确认前不会被下游环节引用。</p>
    <label>结论<textarea maxLength={2000} onChange={(event) => setConclusion(event.target.value)} required value={conclusion} /></label>
    <label>适用条件（每行一条）<textarea onChange={(event) => setConditions(event.target.value)} placeholder={'小红书图文\n预算 5000 元以内'} value={conditions} /></label>
    <label>反例 / 限制（每行一条）<textarea onChange={(event) => setCounterexamples(event.target.value)} placeholder="当前仅有模拟指标，真实投放前需复核" value={counterexamples} /></label>
    <div className="report-card__actions">
      <button className="button button--primary button--compact" disabled={busy || !conclusion.trim()} type="submit">沉淀为待确认经验</button>
      <button className="button button--secondary button--compact" onClick={onCancel} type="button">取消</button>
    </div>
  </form>
}

/**
 * 投前洞察：左边是可引用结论的清单，右边是选中那条的完整说明。
 * 版式对齐参考产品——列表用表格行而不是卡片，选中项右侧常驻，
 * 这样一屏之内既能比较多条结论，又能看清当前这条的适用边界。
 */
function PreLaunchView({ disclosure, evidence, experiences, project, projectId }: {
  disclosure: string
  evidence: Map<string, string>
  experiences: Experience[]
  project?: Project
  projectId: string
}) {
  const [keyword, setKeyword] = useState('')
  const [selectedId, setSelectedId] = useState('')
  const query = keyword.trim().toLowerCase()
  const matched = query
    ? experiences.filter((item) => [item.conclusion, ...item.conditions, ...item.counterexamples]
      .some((text) => text.toLowerCase().includes(query)))
    : experiences
  // 选中项跟着筛选结果走：搜索把当前选中那条筛掉之后，右侧退回第一条。
  const selected = matched.find((item) => item.id === selectedId) || matched[0]

  return <div className="insight-panel prelaunch-workspace">
    <section className="prelaunch-main">
      <InsightToolbar
        description="这些是已经确认过、可以直接引用的结论。每条都写明了在什么情况下成立、什么情况下不该用。"
        label="投放之前"
        project={project}
        title="让过去的投放结果参与这次决策"
      />
      <div className="prelaunch-filterbar">
        <label className="prelaunch-search">
          <span aria-hidden="true">⌕</span>
          <input
            aria-label="搜索结论"
            onChange={(event) => setKeyword(event.target.value)}
            placeholder="搜索结论、适用条件或反例"
            value={keyword}
          />
        </label>
        <span>{matched.length} 条可引用结论</span>
      </div>
      <div className="prelaunch-table">
        <div className="prelaunch-row prelaunch-row--header">
          <span>结论</span><span>状态</span><span>来自哪次投放</span><span>版本</span>
        </div>
        {matched.map((item) => <button
          className={item.id === selected?.id ? 'prelaunch-row prelaunch-row--active' : 'prelaunch-row'}
          key={item.id}
          onClick={() => setSelectedId(item.id)}
          type="button"
        >
          <span><b>{item.conclusion}</b><small>{item.conditions.join(' · ') || '未限定适用条件'}</small></span>
          <span>{experienceStatusLabels[item.status]}</span>
          <span>{evidence.get(item.report_id) || '来源报告不在当前项目内'}</span>
          <span>第 {item.revision} 版</span>
        </button>)}
        {matched.length === 0 ? <p className="prelaunch-empty">
          {experiences.length === 0
            ? '还没有可引用经验。完成投后报告确认和经验沉淀后，下一轮投前会自动看到引用。'
            : '没有匹配的结论，换个词试试。'}
        </p> : null}
      </div>
      <div className="insight-panel__body"><Disclosure>{disclosure}</Disclosure></div>
    </section>
    <aside className="prelaunch-detail">
      {selected ? <>
        <span className="insight-section-label">当前结论</span>
        <h3>{selected.conclusion}</h3>
        <p>{experienceStatusLabels[selected.status]} · 第 {selected.revision} 版</p>
        <div className="prelaunch-fact">
          <span aria-hidden="true">◇</span>
          <span><small>什么情况下可以用</small><b>{selected.conditions.join('；') || '没有限定条件，引用前请自行判断。'}</b></span>
        </div>
        <div className="prelaunch-fact">
          <span aria-hidden="true">▤</span>
          <span><small>这条结论来自哪次投放</small><b>{evidence.get(selected.report_id) || '来源报告不在当前项目的报告列表里，无法展示证据说明。'}</b></span>
        </div>
        <div className="prelaunch-boundary" role="note">
          <span aria-hidden="true">！</span>
          <span><small>什么时候不该用</small>{selected.counterexamples.join('；') || '沉淀时没有填写反例。'}</span>
        </div>
        <Link className="button button--secondary button--compact" to={insightViewPath(projectId, 'experiences', 'references')}>查看这条经验被谁用过</Link>
      </> : <p className="prelaunch-empty">左边选一条结论，这里会显示它的完整说明。</p>}
    </aside>
  </div>
}

function ExecutionCard({ item }: { item: DeliveryExecutionSnapshot }) {
  const metrics = item.metric_snapshot?.raw_metrics
  return <article className="outcome-card">
    <div className="outcome-card__top">
      <span className="status-chip status-chip--active">本地模拟</span>
      {item.metric_snapshot ? <span className="status-chip">数据集 {item.metric_snapshot.dataset_version}</span> : null}
    </div>
    <h2>投放证据快照</h2>
    <p>{item.evidence_summary}</p>
    {metrics ? <dl>
      <div><dt>曝光</dt><dd>{metrics.impressions.toLocaleString()}</dd></div>
      <div><dt>点击</dt><dd>{metrics.clicks.toLocaleString()}</dd></div>
      <div><dt>转化</dt><dd>{metrics.conversions.toLocaleString()}</dd></div>
      <div><dt>花费</dt><dd>¥{(metrics.spend_cents / 100).toLocaleString('zh-CN', { minimumFractionDigits: 2 })}</dd></div>
    </dl> : <p>这次执行还没有指标快照。</p>}
  </article>
}

function Disclosure({ children }: { children: string }) {
  return <div className="simulation-banner" role="note"><strong>证据与能力披露</strong><span>{children}</span></div>
}

function EmptyState({ title, description }: { title: string, description: string }) {
  return <div className="outcome-empty"><span aria-hidden="true">◎</span><h2>{title}</h2><p>{description}</p></div>
}
