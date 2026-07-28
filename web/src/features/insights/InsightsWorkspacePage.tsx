import { useCallback, useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { projectHomePath } from '../../app/routes'
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
import type { DeliveryExecutionSnapshot, Experience, InsightReport, PerformanceOverview, PreLaunchInsight } from './types'
import '../outcomes/outcome-workspace.css'

export type InsightsView = 'prelaunch' | 'performance' | 'reports' | 'experiences'

export function InsightsWorkspacePage({ project, view }: { project?: Project, view: InsightsView }) {
  const { projectId = '' } = useParams()
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
        listExperiences(projectId, signal),
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
        setError('报告已被其他操作更新，页面已刷新为最新版本，请确认后重试。')
        await load()
      } else {
        setError(caught instanceof Error ? caught.message : '洞察操作未完成。')
      }
    } finally {
      setBusy(false)
    }
  }

  return <section className="outcome-workspace">
    <header className="outcome-header">
      <div>
        <nav className="breadcrumb" aria-label="面包屑"><Link to="/projects">项目</Link><span>/</span><Link to={projectHomePath(projectId)}>{project?.name || projectId}</Link><span>/</span><span>素材洞察</span></nav>
        <span className="page-eyebrow">INSIGHTS</span>
        <h1>{view === 'prelaunch' ? '投前洞察' : view === 'performance' ? '投后分析' : view === 'reports' ? '复盘报告' : '经验沉淀'}</h1>
        <p>从 Delivery Evidence 形成可确认报告，再把结论沉淀为下一轮可引用的 Experience。</p>
      </div>
    </header>
    {error ? <div className="workspace-alert" role="alert"><span>{error}</span><button className="text-action" onClick={() => void load()} type="button">重试</button></div> : null}
    {loading ? <div className="outcome-loading" role="status">正在读取项目洞察与证据…</div> : null}

    {!loading && view === 'prelaunch' ? <section className="outcome-cards">
      <Disclosure>{preLaunch?.disclosure || '正在读取已确认经验。'}</Disclosure>
      {preLaunch?.experience_references.map((item) => <ExperienceCard item={item} key={item.id} />)}
      {preLaunch?.experience_references.length === 0 ? <EmptyState title="还没有可引用经验" description="完成投后报告确认和经验沉淀后，下一轮投前会自动看到引用。" /> : null}
    </section> : null}

    {!loading && view === 'performance' ? <section className="outcome-cards">
      <Disclosure>{performance?.disclosure || '正在读取执行证据。'}</Disclosure>
      {performance?.executions.map((item) => <article className="outcome-card" key={item.id}>
        <div className="outcome-card__top"><span className="status-chip status-chip--active">本地模拟</span><code>{item.id}</code></div>
        <h2>投放证据快照</h2><p>{item.evidence_summary}</p>
        <dl><div><dt>计划</dt><dd>{item.plan_id}</dd></div><div><dt>Evidence</dt><dd>{item.evidence_id}</dd></div></dl>
      </article>)}
      {performance?.executions.length === 0 ? <EmptyState title="还没有投后数据" description="当前只接收受控模拟执行证据；真实指标接入留给后续 Insights 数据连接器。" /> : null}
    </section> : null}

    {!loading && view === 'reports' ? <ReportsView busy={busy} executions={performance?.executions || []} experiences={experiences} onAction={act} projectId={projectId} reports={reports} /> : null}
    {!loading && view === 'experiences' ? <section className="outcome-cards">
      {experiences.map((item) => <ExperienceCard item={item} key={item.id} />)}
      {experiences.length === 0 ? <EmptyState title="还没有沉淀经验" description="确认复盘报告后，填写适用条件和反例，才能形成可复用 Experience。" /> : null}
    </section> : null}
  </section>
}

function ReportsView({ busy, executions, experiences, onAction, projectId, reports }: {
  busy: boolean
  executions: DeliveryExecutionSnapshot[]
  experiences: Experience[]
  onAction: (operation: () => Promise<unknown>) => void
  projectId: string
  reports: InsightReport[]
}) {
  const reported = new Set(reports.map((item) => item.execution_id))
  const experienced = new Set(experiences.map((item) => item.report_id))
  return <div className="outcome-grid">
    <aside className="outcome-list">
      <div className="outcome-list__heading"><strong>待复盘执行</strong><span>{executions.length}</span></div>
      {executions.map((item) => <div className="outcome-list__item" key={item.id}><strong>{item.id}</strong><span>{reported.has(item.id) ? '已建立报告' : '等待报告'}</span></div>)}
    </aside>
    <main className="outcome-detail">
      {executions.filter((item) => !reported.has(item.id)).map((item) => <AutomatedReportComposer busy={busy} execution={item} key={item.id} onCreate={() => onAction(() => createInsightReport(projectId, { execution_id: item.id }))} />)}
      {reports.map((report) => <article className="report-card" key={report.id}>
        <div className="outcome-card__top"><span className="status-chip status-chip--active">{report.status === 'confirmed' ? '已确认' : '待确认'}</span><code>{report.id} · v{report.version}</code></div>
        <h2>{report.summary}</h2>
        <ul>{report.findings.map((item) => <li key={item}>{item}</li>)}</ul>
        <Disclosure>{report.evidence_summary}</Disclosure>
        <div className="report-card__actions">
          {report.status === 'draft' ? <button className="button button--primary button--compact" disabled={busy} onClick={() => onAction(() => confirmInsightReport(projectId, report.id, report.version))} type="button">确认报告</button> : null}
          {report.status === 'confirmed' && !experienced.has(report.id) ? <button className="button button--secondary button--compact" disabled={busy} onClick={() => onAction(() => createExperience(projectId, report.id, {
            expected_report_version: report.version,
            conclusion: report.summary,
            conditions: ['小红书图文', '当前项目上下文'],
            counterexamples: ['尚无真实平台指标，结论需后续复核'],
          }))} type="button">沉淀为经验</button> : null}
          {experienced.has(report.id) ? <span className="status-chip status-chip--active">已沉淀经验</span> : null}
        </div>
      </article>)}
      {executions.length === 0 && reports.length === 0 ? <EmptyState title="等待投放执行" description="完成 Delivery 模拟执行后，才能创建有证据来源的复盘报告。" /> : null}
    </main>
  </div>
}

function AutomatedReportComposer({ busy, execution, onCreate }: { busy: boolean, execution: DeliveryExecutionSnapshot, onCreate: () => void }) {
  const metrics = execution.metric_snapshot?.raw_metrics
  return <section className="report-composer">
    <span className="page-eyebrow">EVIDENCE-BASED REPORT</span>
    <h2>为 {execution.id} 生成模拟复盘</h2>
    {metrics ? <dl className="outcome-facts">
      <div><dt>曝光</dt><dd>{metrics.impressions.toLocaleString()}</dd></div>
      <div><dt>点击</dt><dd>{metrics.clicks.toLocaleString()}</dd></div>
      <div><dt>转化</dt><dd>{metrics.conversions.toLocaleString()}</dd></div>
    </dl> : <p>请先在智能投放中生成模拟指标快照。</p>}
    <Disclosure>报告摘要和 CTR/CVR 由服务端根据 demo_fixture 自动计算；模拟数据不代表真实广告效果。</Disclosure>
    <button className="button button--primary button--compact" disabled={busy || !metrics} onClick={onCreate} type="button">生成复盘草稿</button>
  </section>
}

export function ReportComposer({ busy, execution, onCreate }: { busy: boolean, execution: DeliveryExecutionSnapshot, onCreate: (summary: string, findings: string[]) => void }) {
  const [summary, setSummary] = useState('')
  const [findings, setFindings] = useState('')
  return <form className="report-composer" onSubmit={(event) => { event.preventDefault(); onCreate(summary, findings.split('\n').map((item) => item.trim()).filter(Boolean)) }}>
    <span className="page-eyebrow">NEW REPORT</span><h2>为 {execution.id} 创建复盘</h2>
    <label>报告摘要<textarea required value={summary} onChange={(event) => setSummary(event.target.value)} /></label>
    <label>关键发现（每行一条）<textarea required value={findings} onChange={(event) => setFindings(event.target.value)} /></label>
    <button className="button button--primary button--compact" disabled={busy} type="submit">创建报告草稿</button>
  </form>
}

function ExperienceCard({ item }: { item: Experience }) {
  return <article className="outcome-card">
    <div className="outcome-card__top"><span className="status-chip status-chip--active">已确认经验</span><code>{item.id}</code></div>
    <h2>{item.conclusion}</h2>
    <dl><div><dt>适用条件</dt><dd>{item.conditions.join(' · ') || '未限定'}</dd></div><div><dt>反例/限制</dt><dd>{item.counterexamples.join(' · ') || '暂无'}</dd></div><div><dt>来源证据</dt><dd>{item.source_evidence_id}</dd></div></dl>
  </article>
}

function Disclosure({ children }: { children: string }) {
  return <div className="simulation-banner" role="note"><strong>证据与能力披露</strong><span>{children}</span></div>
}

function EmptyState({ title, description }: { title: string, description: string }) {
  return <div className="outcome-empty"><span aria-hidden="true">◎</span><h2>{title}</h2><p>{description}</p></div>
}
