import { useCallback, useEffect, useMemo, useRef, useState, useTransition } from 'react'
import { CircleAlert, CircleCheck, Clock3, FileCheck2, Hand, Pause, RefreshCw, ShieldAlert, XCircle } from 'lucide-react'
import { controlledExecutionApi, ControlledExecutionApiError } from './api'
import type { ComputerUseEvidence, ComputerUseRun, ComputerUseRunEvent, ControlledExecutionTransportState } from './model'
import { isTerminalControlledExecutionState, presentControlledExecution, shortHash } from './presentation'
import './controlled-execution.css'

type Props = {
  projectId: string
  /** The route should supply a server-issued ComputerUseRun id; an absent id is a real empty state, not a fixture. */
  runId?: string
}

export function ControlledExecutionWorkspace({ projectId, runId }: Props) {
  const [transport, setTransport] = useState<ControlledExecutionTransportState>(() => runId ? { kind: 'loading' } : { kind: 'empty' })
  const [notice, setNotice] = useState('')
  const [actionPending, setActionPending] = useState(false)
  const [isRefreshPending, startRefreshTransition] = useTransition()
  const requestId = useRef(0)

  const load = useCallback(async (signal?: AbortSignal) => {
    if (!runId) {
      setTransport({ kind: 'empty' })
      return
    }
    const currentRequest = ++requestId.current
    setTransport(current => current.kind === 'ready' ? current : { kind: 'loading' })
    try {
      const workspace = await controlledExecutionApi.getWorkspace(projectId, runId, signal)
      if (currentRequest !== requestId.current || signal?.aborted) return
      startRefreshTransition(() => setTransport({ kind: 'ready', workspace }))
    } catch (error) {
      if (currentRequest !== requestId.current || signal?.aborted) return
      const message = error instanceof Error ? error.message : '读取受控执行中心失败。'
      setTransport(error instanceof ControlledExecutionApiError && error.status === 404
        ? { kind: 'empty' }
        : error instanceof ControlledExecutionApiError && error.status === 403
          ? { kind: 'forbidden', message }
          : { kind: 'error', message })
    }
  }, [projectId, runId, startRefreshTransition])

  useEffect(() => {
    const controller = new AbortController()
    void load(controller.signal)
    return () => {
      requestId.current += 1
      controller.abort()
    }
  }, [load])

  const refresh = useCallback(() => {
    setNotice('')
    void load()
  }, [load])

  const runControl = useCallback(async (action: 'pause' | 'resume' | 'cancel' | 'takeover' | 'release_takeover') => {
    if (transport.kind !== 'ready') return
    const { run } = transport.workspace
    setActionPending(true)
    setNotice('')
    try {
      const updated = await controlledExecutionApi[
        action === 'takeover' ? 'takeOver' : action === 'release_takeover' ? 'releaseTakeover' : action
      ](projectId, run.id, run.version)
      setTransport(current => current.kind === 'ready' && current.workspace.run.id === run.id
        ? { kind: 'ready', workspace: { ...current.workspace, run: updated } }
        : current)
      setNotice(controlNotice(action))
    } catch (error) {
      setNotice(error instanceof Error ? error.message : '运行控制请求失败。')
    } finally {
      setActionPending(false)
    }
  }, [projectId, transport])

  if (transport.kind === 'loading') return <WorkspaceState kind="loading" />
  if (transport.kind === 'empty') return <WorkspaceState kind="empty" />
  if (transport.kind === 'forbidden') return <WorkspaceState kind="forbidden" message={transport.message} />
  if (transport.kind === 'error') return <WorkspaceState kind="error" message={transport.message} onRetry={refresh} />

  return <WorkspaceReady
    workspace={transport.workspace}
    busy={actionPending || isRefreshPending}
    notice={notice}
    onRefresh={refresh}
    onControl={runControl}
  />
}

function WorkspaceReady({ workspace, busy, notice, onRefresh, onControl }: {
  workspace: Extract<ControlledExecutionTransportState, { kind: 'ready' }>['workspace']
  busy: boolean
  notice: string
  onRefresh: () => void
  onControl: (action: 'pause' | 'resume' | 'cancel' | 'takeover' | 'release_takeover') => void
}) {
  const { run, events, evidence } = workspace
  const presentation = useMemo(() => presentControlledExecution(run), [run])
  const terminal = isTerminalControlledExecutionState(run.state)
  // A Kill Switch blocks new writes, never the operator's ability to take over,
  // pause, cancel, or inspect the run.
  const showTakeover = !terminal

  return <section className="controlled-execution-workspace" aria-label="受控执行中心">
    <header className="controlled-execution-header">
      <div>
        <span className="section-label">Controlled Computer Use</span>
        <h2>受控执行中心</h2>
        <p>服务端 Run 是权威状态；本页不保存本地终态，也不提供直接远端提交按钮。</p>
      </div>
      <button className="secondary-button" onClick={onRefresh} disabled={busy}><RefreshCw size={15} />从服务端刷新</button>
    </header>

    <AuthorityChain run={run} />
    <StatusBanner presentation={presentation} />
    {run.authority.promotion_mutation || run.authority.promotion_control || run.authority.promotion_restart ? <PromotionChangeDiff run={run} /> : null}

    <div className="controlled-execution-layout">
      <section className="controlled-execution-main" aria-label="运行状态与步骤">
        <RunTimeline run={run} />
        <ControlPanel run={run} busy={busy} terminal={terminal} showTakeover={showTakeover} onControl={onControl} />
        <RecoveryPanel kind={presentation.kind} />
        <EvidencePanel evidence={evidence} events={events} />
      </section>
      <aside className="controlled-execution-audit" aria-label="授权与审计摘要">
        <dl>
          <div><dt>Run</dt><dd title={run.id}>{run.id}</dd></div>
          <div><dt>账户</dt><dd>{run.account_id}</dd></div>
          <div><dt>ChangeSet</dt><dd title={run.authority.change_set_id}>{run.authority.change_set_id}</dd></div>
          <div><dt>正式 Approval</dt><dd title={run.authority.approval_id}>{run.authority.approval_id}</dd></div>
          {run.authority.target_mapping_id ? <div><dt>目标映射版本</dt><dd title={run.authority.target_mapping_id}>{shortHash(run.authority.target_mapping_id)} · v{run.authority.target_mapping_version}</dd></div> : null}
          {run.authority.target_platform_object_id ? <div><dt>目标推广单元</dt><dd title={run.authority.target_platform_object_id}>{shortHash(run.authority.target_platform_object_id)}</dd></div> : null}
          {run.authority.operator_principal_id ? <div><dt>绑定操作人</dt><dd title={run.authority.operator_principal_id}>{run.authority.operator_principal_id}</dd></div> : null}
          <div><dt>预算上限</dt><dd>¥{formatMinor(run.authority.budget_limit_minor)} {run.authority.currency}</dd></div>
          <div><dt>Workflow</dt><dd title={run.authority.workflow_canonical_hash}>{shortHash(run.authority.workflow_canonical_hash)}</dd></div>
          <div><dt>Platform Skill</dt><dd>{run.authority.skill_id && run.authority.skill_version ? <>{run.authority.skill_id} · {run.authority.skill_version}<small>仅代表已校准路径；执行当轮仍须复核页面和字段。</small></> : '未绑定；真实执行不可用'}</dd></div>
          <div><dt>租约</dt><dd title={run.lease_id}>{run.lease_id}</dd></div>
          <div><dt>策略</dt><dd title={run.policy_id}>{run.policy_id}</dd></div>
        </dl>
      </aside>
    </div>
    {notice ? <div className="controlled-execution-notice" role="status">{notice}</div> : null}
  </section>
}

function AuthorityChain({ run }: { run: ComputerUseRun }) {
  const changing = Boolean(run.authority.promotion_mutation || run.authority.promotion_control || run.authority.promotion_restart)
  const emergencyPause = run.authority.action === 'pause_promotion'
  const controlledRestart = run.authority.action === 'resume_promotion'
  const formalApproved = run.blocking_reason !== 'APPROVAL_INVALID'
  const confirmationReady = ['submitting', 'verifying', 'succeeded', 'failed', 'partial', 'result_unknown'].includes(run.state)
    && run.blocking_reason !== 'FINAL_CONFIRMATION_INVALID'
  return <ol className="controlled-execution-authority-chain" aria-label="受控写入授权链">
    <li className="complete"><span>1</span><div><b>{emergencyPause ? '核对投放状态并创建紧急暂停' : controlledRestart ? '完成全部重检并创建受控重启' : changing ? '读取当前值并创建新变更' : '接受优化方案'}</b><small>{changing ? '当前值、目标值、对象、操作人和 Mapping 版本已经冻结；创建时的审批不可复用。' : '已接受/修改的反馈才可创建 ChangeSet；这不是写入批准。'}</small></div></li>
    <li className={formalApproved ? 'complete' : 'blocked'}><span>2</span><div><b>批准平台写入</b><small>正式 Approval 绑定账户、预算、配置、Workflow 与阶段 B Skill 校准版本；这不代表实时 DOM 已复核。</small></div></li>
    <li className={confirmationReady ? 'complete' : 'waiting'}><span>3</span><div><b>一次性最终确认</b><small>仅对当前 Run 有效；签发或过期都不等于已经提交。</small></div></li>
  </ol>
}

function PromotionChangeDiff({ run }: { run: ComputerUseRun }) {
  const mutation = run.authority.promotion_mutation
  const control = run.authority.promotion_control
  const restart = run.authority.promotion_restart
  if (!mutation && !control && !restart) return null
  const emergencyPause = run.authority.action === 'pause_promotion' && Boolean(control)
  const controlledRestart = run.authority.action === 'resume_promotion' && Boolean(restart)
  const actionLabel = ({
    update_promotion_budget: '修改推广单元日预算',
    update_promotion_materials: '更换或增减授权素材',
    pause_promotion: '紧急暂停推广单元',
    resume_promotion: '受控重启推广单元',
  } as Record<string, string>)[run.authority.action] ?? '修改推广单元'
  const rows = restart
    ? [
        { label: '投放状态', current: statusLabel(restart.current_platform_status), target: statusLabel(restart.target_platform_status) },
        { label: '每日预算（重新核准）', current: `¥${formatMinor(restart.current_daily_budget_minor)}`, target: `¥${formatMinor(restart.approved_daily_budget_minor)}` },
        { label: '当前有效排期', current: formatSchedule(restart.schedule), target: '校验通过后保持不变' },
        { label: '授权素材可用性', current: `${restart.materials.length} 项`, target: '最终点击前再次核对' },
        { label: '授权落地页可用性', current: '已绑定 1 个授权落地页', target: '最终点击前再次核对' },
      ]
    : control
    ? [
        { label: '投放状态', current: statusLabel(control.current_platform_status), target: statusLabel(control.target_platform_status) },
        { label: '每日预算（保持不变）', current: `¥${formatMinor(control.current_daily_budget_minor)}`, target: `¥${formatMinor(control.current_daily_budget_minor)}` },
      ]
    : mutationRows(run.authority.action, mutation!)
  return <section className={`controlled-execution-mutation${emergencyPause ? ' emergency' : ''}${controlledRestart ? ' restart' : ''}`} aria-label="当前值与目标值">
    <header><div><span className="section-label">{emergencyPause ? '高优先级止损动作' : controlledRestart ? '严格重检后的独立动作' : '本次受控变更'}</span><h3>{actionLabel}</h3></div><small>{emergencyPause ? '这是平台推广单元的状态变更，不是暂停本页执行流程。只允许单击一次，随后必须回读“已暂停”。' : controlledRestart ? '重启不是自动补偿。账户、父项目、对象、预算、有效排期、素材、落地页和 Kill Switch 任一不通过都会阻止最终确认。' : '提交前必须逐字段回读一致；任何差异都会阻止一次性确认。'}</small></header>
    <div className="controlled-execution-mutation-table" role="table" aria-label="变更差异">
      <div className="heading" role="row"><b role="columnheader">字段</b><b role="columnheader">当前值</b><b role="columnheader">目标值</b></div>
      {rows.map(row => <div key={row.label} role="row"><span role="cell">{row.label}</span><strong role="cell">{row.current}</strong><strong role="cell" className="target">{row.target}</strong></div>)}
    </div>
  </section>
}

function statusLabel(status: string) {
  return ({ delivering: '投放中', paused: '已暂停' } as Record<string, string>)[status] ?? status
}

function mutationRows(action: string, mutation: NonNullable<ComputerUseRun['authority']['promotion_mutation']>) {
  if (action === 'update_promotion_materials') {
    return [{ label: '已授权素材', current: `${mutation.current_materials?.length ?? 0} 个`, target: `${mutation.target_materials?.length ?? 0} 个` }]
  }
  return [{ label: '每日预算', current: `¥${formatMinor(mutation.current_daily_budget_minor)}`, target: `¥${formatMinor(mutation.target_daily_budget_minor)}` }]
}

function formatSchedule(value?: { start_at: string; end_at: string; timezone: string }) {
  if (!value) return '未提供'
  return `${formatTime(value.start_at)} 至 ${formatTime(value.end_at)}（${value.timezone}）`
}

function StatusBanner({ presentation }: { presentation: ReturnType<typeof presentControlledExecution> }) {
  const Icon = presentation.tone === 'success' ? CircleCheck : presentation.tone === 'neutral' ? Clock3 : ShieldAlert
  return <div className={`controlled-execution-status ${presentation.tone}`} role={presentation.tone === 'danger' ? 'alert' : 'status'}>
    <Icon size={20} />
    <span><b>{presentation.title}</b><small>{presentation.detail}</small></span>
  </div>
}

function RunTimeline({ run }: { run: ComputerUseRun }) {
  const steps: Array<{ state: ComputerUseRun['state']; label: string }> = [
    { state: 'environment_check', label: '环境检查' },
    { state: 'preparing', label: '准备表单' },
    { state: 'awaiting_confirmation', label: '核对差异与等待确认' },
    { state: 'submitting', label: '受控提交' },
    { state: 'verifying', label: '写后验证' },
  ]
  const activeIndex = isTerminalControlledExecutionState(run.state)
    ? steps.length
    : Math.max(0, steps.findIndex(step => step.state === run.state))
  return <section className="controlled-execution-timeline" aria-label="执行阶段">
    <h3>执行阶段</h3>
    <ol>{steps.map((step, index) => <li key={step.state} className={index < activeIndex ? 'complete' : index === activeIndex ? 'active' : ''}><span>{index + 1}</span>{step.label}</li>)}</ol>
    {run.paused ? <p><Pause size={14} />执行流程已暂停；这不代表平台推广单元已经暂停。恢复流程时必须重新识别页面与账户。</p> : null}
  </section>
}

function ControlPanel({ run, busy, terminal, showTakeover, onControl }: {
  run: ComputerUseRun
  busy: boolean
  terminal: boolean
  showTakeover: boolean
  onControl: (action: 'pause' | 'resume' | 'cancel' | 'takeover' | 'release_takeover') => void
}) {
  return <section className="controlled-execution-controls" aria-label="运行控制">
    <div><span className="section-label">运行控制</span><h3>执行流程的暂停、接管和取消</h3><p>这里仅控制 Computer Use 执行流程，不会暂停广告平台上的推广单元；平台紧急暂停必须使用独立 ChangeSet 和 Approval。</p></div>
    <div className="controlled-execution-control-actions">
      {run.paused ? <button className="secondary-button" onClick={() => onControl('resume')} disabled={busy || terminal}>恢复并重新识别</button> : <button className="secondary-button" onClick={() => onControl('pause')} disabled={busy || terminal}>暂停执行流程</button>}
      {showTakeover ? <button className="secondary-button" onClick={() => onControl(run.takeover_active ? 'release_takeover' : 'takeover')} disabled={busy || terminal}><Hand size={15} />{run.takeover_active ? '释放接管' : '人工接管'}</button> : null}
      <button className="danger-button" onClick={() => onControl('cancel')} disabled={busy || terminal}><XCircle size={15} />取消运行</button>
    </div>
  </section>
}

function RecoveryPanel({ kind }: { kind: ReturnType<typeof presentControlledExecution>['kind'] }) {
  if (kind === 'result_unknown') return <section className="controlled-execution-recovery danger"><CircleAlert size={18} /><div><b>仅允许查询、重新识别或人工接管</b><p>没有“重试提交”按钮。结果未知时再次提交可能创建重复对象。</p></div></section>
  if (kind === 'partial') return <section className="controlled-execution-recovery"><CircleAlert size={18} /><div><b>已完成范围保留，补偿需重新获批</b><p>请以事件与证据确认完成范围；不要把补偿或回滚伪装为普通恢复。</p></div></section>
  if (kind === 'approval_expired' || kind === 'confirmation_expired') return <section className="controlled-execution-recovery danger"><FileCheck2 size={18} /><div><b>重新生成授权链</b><p>从差异与正式 Approval 重新开始；旧审批或确认不能用于当前 Run。</p></div></section>
  return null
}

function EvidencePanel({ evidence, events }: { evidence: ComputerUseEvidence[]; events: ComputerUseRunEvent[] }) {
  return <section className="controlled-execution-evidence" aria-label="证据与事件">
    <header><div><span className="section-label">Evidence & Audit</span><h3>脱敏证据与事件</h3></div><small>仅显示脱敏键、引用和版本，不渲染原始页面事实或凭据。</small></header>
    <div className="controlled-execution-evidence-grid">
      <div><h4>字段差异</h4>{evidence.length ? evidence.map(item => <article key={item.id}><b>{item.step_id}</b><span>{item.diff_keys.length ? item.diff_keys.join('、') : '无字段差异键'}</span><small>redaction={item.redaction_version} · selector={item.selector_version}</small></article>) : <p>服务端尚未返回 Evidence。</p>}</div>
      <div><h4>事件时间线</h4>{events.length ? events.map(item => <article key={item.id}><b>{item.kind}</b><span>{item.summary}</span><small>{formatTime(item.created_at)} · {item.actor}</small></article>) : <p>服务端尚未返回 Run Event。</p>}</div>
    </div>
  </section>
}

function WorkspaceState({ kind, message, onRetry }: { kind: 'loading' | 'empty' | 'forbidden' | 'error'; message?: string; onRetry?: () => void }) {
  if (kind === 'loading') return <section className="controlled-execution-state loading" role="status" aria-busy="true"><span /><span /><span /><span /></section>
  const forbidden = kind === 'forbidden'
  return <section className={`controlled-execution-state ${kind}`} role={forbidden || kind === 'error' ? 'alert' : 'status'}>
    {forbidden || kind === 'error' ? <CircleAlert size={28} /> : <FileCheck2 size={28} />}
    <h2>{forbidden ? '无权查看此受控执行' : kind === 'error' ? '无法读取受控执行' : '暂无受控执行 Run'}</h2>
    <p>{message ?? '路由需要提供服务端创建的 Run ID；此页不会用本地示例填充执行记录。'}</p>
    {onRetry ? <button className="secondary-button" onClick={onRetry}><RefreshCw size={15} />重新加载</button> : null}
  </section>
}

function controlNotice(action: 'pause' | 'resume' | 'cancel' | 'takeover' | 'release_takeover') {
  return ({ pause: '已请求暂停执行流程；这不会改变平台推广单元状态。', resume: '已请求恢复执行流程；服务端将先重新识别页面与账户。', cancel: '已请求取消；证据与审计记录保持可读。', takeover: '已请求人工接管；请等待服务端确认租约状态。', release_takeover: '已请求释放人工接管。' })[action]
}

function formatMinor(value: number) {
  return (value / 100).toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
}

function formatTime(value: string) {
  return new Date(value).toLocaleString('zh-CN')
}
