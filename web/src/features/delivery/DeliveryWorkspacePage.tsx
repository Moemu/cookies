import { useCallback, useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { deliveryPlanPath, projectHomePath } from '../../app/routes'
import { ApiProblem } from '../../shared/api/client'
import { listCreativePackages } from '../creative/api'
import type { CreativePackage } from '../creative/types'
import type { Project } from '../platform/types'
import {
  createDeliveryChangeSet,
  createDemoMetricSnapshot,
  createDeliveryPlan,
  executeDeliveryChangeSet,
  getDeliveryPlan,
  listDeliveryExecutions,
  listDeliveryPlans,
  transitionDeliveryChangeSet,
} from './api'
import type { DeliveryChangeSet, DeliveryExecutionResult, DeliveryPlan, DeliveryPlanDetail } from './types'
import '../outcomes/outcome-workspace.css'

export type DeliveryView = 'plans' | 'monitoring' | 'accounts' | 'optimization'

export function DeliveryWorkspacePage({ project, view = 'plans' }: { project?: Project, view?: DeliveryView }) {
  const { projectId = '', planId = '' } = useParams()
  const navigate = useNavigate()
  const [plans, setPlans] = useState<DeliveryPlan[]>([])
  const [packages, setPackages] = useState<CreativePackage[]>([])
  const [executions, setExecutions] = useState<DeliveryExecutionResult[]>([])
  const [detail, setDetail] = useState<DeliveryPlanDetail | null>(null)
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [showCreate, setShowCreate] = useState(false)

  const load = useCallback(async (signal?: AbortSignal) => {
    setError('')
    try {
      const [planResponse, packageResponse, executionResponse] = await Promise.all([
        listDeliveryPlans(projectId, signal),
        listCreativePackages(projectId, signal),
        listDeliveryExecutions(projectId, signal),
      ])
      setPlans(planResponse.items)
      setPackages(packageResponse.items)
      setExecutions(executionResponse.items)
      const selectedPlanId = planId || planResponse.items[0]?.id
      setDetail(selectedPlanId ? await getDeliveryPlan(projectId, selectedPlanId, signal) : null)
    } catch (caught) {
      if (caught instanceof DOMException && caught.name === 'AbortError') return
      setError(caught instanceof Error ? caught.message : '无法读取投放工作区。')
    } finally {
      if (!signal?.aborted) setLoading(false)
    }
  }, [planId, projectId])

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
        setError('该变更已被其他操作更新，页面已刷新为最新版本，请确认后重试。')
        await load()
      } else {
        setError(caught instanceof Error ? caught.message : '投放操作未完成。')
      }
    } finally {
      setBusy(false)
    }
  }

  const latestChangeSet = detail?.change_sets[0]
  const hasPackage = packages.length > 0

  return <section className="outcome-workspace">
    <header className="outcome-header">
      <div>
        <nav className="breadcrumb" aria-label="面包屑"><Link to="/projects">项目</Link><span>/</span><Link to={projectHomePath(projectId)}>{project?.name || projectId}</Link><span>/</span><span>智能投放</span></nav>
        <span className="page-eyebrow">DELIVERY</span>
        <h1>{view === 'plans' ? '投放计划与变更控制' : view === 'monitoring' ? '投放监控' : view === 'accounts' ? '广告账户' : '优化建议'}</h1>
        <p>以 CreativePackage 为稳定输入，通过预检、审批和可回滚证据控制每一次执行。</p>
      </div>
      {view === 'plans' ? <button className="button button--primary button--compact" disabled={!hasPackage} onClick={() => setShowCreate(true)} type="button">新建投放计划</button> : null}
    </header>

    <div className="simulation-banner" role="note">
      <strong>本地模拟模式</strong>
      <span>当前执行不会写入真实广告平台；所有页面和证据都会明确保留这项披露。</span>
    </div>
    {error ? <div className="workspace-alert" role="alert"><span>{error}</span><button className="text-action" onClick={() => void load()} type="button">重试</button></div> : null}
    {loading ? <div className="outcome-loading" role="status">正在读取投放计划、交付包与执行证据…</div> : null}

    {!loading && view === 'plans' ? <>
      {!hasPackage && !loading ? <EmptyState title="等待创意交付包" description="先在创意评审中通过版本并生成 CreativePackage，才能创建投放计划。" /> : null}
      <div className="outcome-grid">
        <aside className="outcome-list" aria-label="投放计划列表">
          <div className="outcome-list__heading"><strong>投放计划</strong><span>{plans.length}</span></div>
          {plans.map((plan) => <Link className={detail?.plan.id === plan.id ? 'outcome-list__item outcome-list__item--active' : 'outcome-list__item'} key={plan.id} to={deliveryPlanPath(projectId, plan.id)}>
            <strong>{plan.name}</strong><span>¥{(plan.budget_cents / 100).toLocaleString()} · v{plan.version}</span>
          </Link>)}
          {plans.length === 0 ? <p className="outcome-list__empty">暂无计划</p> : null}
        </aside>
        <main className="outcome-detail">
          {detail ? <PlanDetail
            busy={busy}
            detail={detail}
            latestChangeSet={latestChangeSet}
            onAction={(action, changeSet) => act(() => action === 'execute'
              ? executeDeliveryChangeSet(projectId, changeSet.id, changeSet.version).then(async (result) => {
                  await createDemoMetricSnapshot(projectId, result.execution.id)
                  return result
                })
              : transitionDeliveryChangeSet(projectId, changeSet.id, action, changeSet.version))}
            onCreateChangeSet={() => act(() => createDeliveryChangeSet(projectId, detail.plan.id, detail.plan.version))}
            onCreateMetric={(executionId) => act(() => createDemoMetricSnapshot(projectId, executionId))}
          /> : <EmptyState title="选择一个投放计划" description="计划详情会展示版本、变更集、审批状态和执行证据。" />}
        </main>
      </div>
    </> : null}

    {!loading && view === 'monitoring' ? <ExecutionCards
      busy={busy}
      executions={executions}
      onCreateMetric={(executionId) => act(() => createDemoMetricSnapshot(projectId, executionId))}
    /> : null}
    {!loading && view === 'accounts' ? <EmptyState title="尚未连接真实广告账户" description="Phase 1—3 使用本地模拟执行。未来账户接入必须经过独立授权、最小权限和审计设计。" /> : null}
    {!loading && view === 'optimization' ? <section className="outcome-cards">
      {executions.length ? executions.map((item) => <article className="outcome-card" key={item.execution.id}>
        <span className="status-chip status-chip--active">证据驱动</span>
        <h2>复查 {item.change_set.id}</h2>
        <p>{item.evidence.summary}</p>
        <small>当前没有真实效果指标，因此不会自动给出调价或扩量建议。</small>
      </article>) : <EmptyState title="还没有可优化的执行" description="完成一次受控模拟执行后，这里会显示可追溯证据；真实优化需要后续平台指标。" />}
    </section> : null}

    {showCreate ? <CreatePlanDialog
      packages={packages}
      onClose={() => setShowCreate(false)}
      onSubmit={(input) => act(async () => {
        const plan = await createDeliveryPlan(projectId, input)
        setShowCreate(false)
        navigate(deliveryPlanPath(projectId, plan.id))
      })}
    /> : null}
  </section>
}

function PlanDetail({ busy, detail, latestChangeSet, onAction, onCreateChangeSet, onCreateMetric }: {
  busy: boolean
  detail: DeliveryPlanDetail
  latestChangeSet?: DeliveryChangeSet
  onAction: (action: 'preflight' | 'approve' | 'execute' | 'rollback', changeSet: DeliveryChangeSet) => void
  onCreateChangeSet: () => void
  onCreateMetric: (executionId: string) => void
}) {
  const nextAction = !latestChangeSet
    ? null
    : latestChangeSet.status === 'draft'
      ? 'preflight'
      : latestChangeSet.status === 'preflight_passed'
        ? 'approve'
        : latestChangeSet.status === 'approved'
          ? 'execute'
          : latestChangeSet.status === 'executed'
            ? 'rollback'
            : null
  const labels = { preflight: '执行预检', approve: '批准变更', execute: '本地模拟执行', rollback: '回滚模拟变更' }
  return <>
    <div className="outcome-detail__hero">
      <div><span className="page-eyebrow">DELIVERY PLAN</span><h2>{detail.plan.name}</h2><p>{detail.plan.objective}</p></div>
      <span className="status-chip status-chip--active">计划 v{detail.plan.version}</span>
    </div>
    <dl className="outcome-facts">
      <div><dt>预算</dt><dd>¥{(detail.plan.budget_cents / 100).toLocaleString()}</dd></div>
      <div><dt>CreativePackage</dt><dd><code>{detail.plan.creative_package_id}</code></dd></div>
      <div><dt>内容哈希</dt><dd><code>{detail.plan.creative_package_hash}</code></dd></div>
    </dl>
    <section className="outcome-timeline">
      <div className="project-section-heading"><div><span className="page-eyebrow">CONTROLLED CHANGE</span><h3>变更与审批</h3></div>
        {!latestChangeSet ? <button className="button button--primary button--compact" disabled={busy} onClick={onCreateChangeSet} type="button">创建变更集</button> : null}
      </div>
      {latestChangeSet ? <article className="timeline-card">
        <div><strong>{latestChangeSet.id}</strong><span>风险 {latestChangeSet.risk_level} · v{latestChangeSet.version}</span></div>
        <span className="status-chip status-chip--active">{statusLabel(latestChangeSet.status)}</span>
        {nextAction ? <button className={nextAction === 'execute' ? 'button button--primary button--compact' : 'button button--secondary button--compact'} disabled={busy} onClick={() => onAction(nextAction, latestChangeSet)} type="button">{labels[nextAction]}</button> : null}
      </article> : <p className="outcome-list__empty">计划已创建，下一步需要创建不可变的 ChangeSet。</p>}
    </section>
    <ExecutionCards busy={busy} executions={detail.executions} onCreateMetric={onCreateMetric} />
  </>
}

function ExecutionCards({ busy = false, executions, onCreateMetric }: {
  busy?: boolean
  executions: DeliveryExecutionResult[]
  onCreateMetric?: (executionId: string) => void
}) {
  return <section className="outcome-cards" aria-label="执行证据">
    {executions.map((item) => <article className="outcome-card" key={item.execution.id}>
      <div className="outcome-card__top"><span className="status-chip status-chip--active">模拟执行成功</span><code>{item.execution.id}</code></div>
      <h2>执行证据</h2>
      <p>{item.evidence.summary}</p>
      <dl><div><dt>模式</dt><dd>{item.execution.mode}</dd></div><div><dt>可回滚</dt><dd>{item.evidence.reversible ? '是' : '否'}</dd></div></dl>
      {onCreateMetric ? <button
        className="button button--secondary button--compact"
        disabled={busy}
        onClick={() => onCreateMetric(item.execution.id)}
        type="button"
      >生成或恢复模拟指标</button> : null}
    </article>)}
    {executions.length === 0 ? <EmptyState title="还没有执行证据" description="只有通过预检和审批的 ChangeSet 才能进入本地模拟执行。" /> : null}
  </section>
}

function CreatePlanDialog({ packages, onClose, onSubmit }: {
  packages: CreativePackage[]
  onClose: () => void
  onSubmit: (input: { creative_package_id: string, name: string, objective: string, budget_cents: number, start_at: string, end_at: string }) => void
}) {
  const [input, setInput] = useState({ creative_package_id: packages[0]?.id || '', name: '', objective: '', budget: '1000', start_at: '', end_at: '' })
  return <div className="dialog-backdrop" role="presentation"><form className="outcome-dialog" onSubmit={(event) => {
    event.preventDefault()
    onSubmit({
      creative_package_id: input.creative_package_id,
      name: input.name,
      objective: input.objective,
      budget_cents: Math.round(Number(input.budget) * 100),
      start_at: new Date(input.start_at).toISOString(),
      end_at: new Date(input.end_at).toISOString(),
    })
  }}>
    <span className="page-eyebrow">NEW DELIVERY PLAN</span><h2>新建投放计划</h2>
    <label>创意交付包<select value={input.creative_package_id} onChange={(event) => setInput({ ...input, creative_package_id: event.target.value })}>{packages.map((item) => <option key={item.id} value={item.id}>{item.id}</option>)}</select></label>
    <label>计划名称<input required value={input.name} onChange={(event) => setInput({ ...input, name: event.target.value })} /></label>
    <label>验证目标<textarea required value={input.objective} onChange={(event) => setInput({ ...input, objective: event.target.value })} /></label>
    <div className="outcome-form-grid"><label>预算（元）<input min="1" required type="number" value={input.budget} onChange={(event) => setInput({ ...input, budget: event.target.value })} /></label><label>开始<input required type="datetime-local" value={input.start_at} onChange={(event) => setInput({ ...input, start_at: event.target.value })} /></label><label>结束<input required type="datetime-local" value={input.end_at} onChange={(event) => setInput({ ...input, end_at: event.target.value })} /></label></div>
    <div className="outcome-dialog__actions"><button className="button button--secondary" onClick={onClose} type="button">取消</button><button className="button button--primary" type="submit">创建计划</button></div>
  </form></div>
}

function EmptyState({ title, description }: { title: string, description: string }) {
  return <div className="outcome-empty"><span aria-hidden="true">◎</span><h2>{title}</h2><p>{description}</p></div>
}

function statusLabel(status: DeliveryChangeSet['status']) {
  return {
    draft: '草稿', preflight_passed: '预检通过', preflight_failed: '预检失败', approved: '已批准',
    rejected: '已拒绝', executed: '已执行', rolled_back: '已回滚',
  }[status]
}
