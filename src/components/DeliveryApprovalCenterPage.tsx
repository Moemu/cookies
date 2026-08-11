import { useCallback, useEffect, useMemo, useState } from 'react'
import { CircleAlert, CircleCheck, FileCheck2, Play, RotateCcw, ShieldCheck, ThumbsUp } from 'lucide-react'
import {
  deliveryPlanApi,
  type DeliveryApproval,
  type DeliveryControlChangeSet,
} from '../api/delivery'
import { useProject } from '../context/ProjectContext'
import type { DataState } from '../types'
import { StateBoundary } from './StateBoundary'

const invalidReasonLabels: Record<NonNullable<DeliveryApproval['invalidReason']>, string> = {
  APPROVAL_EXPIRED: '审批已超过 24 小时有效期，需要重新预检并审批。',
  APPROVAL_CONTENT_MISMATCH: 'ChangeSet 或批准内容与审批快照不一致。',
  APPROVAL_SCOPE_EXCEEDED: '执行范围或预算超过批准快照。',
  STALE_PLAN_VERSION: '计划已产生新版本，旧版本审批永久失效。',
}

const preflightPassedStatuses = new Set<DeliveryControlChangeSet['status']>([
  'preflight_passed',
  'approved',
  'executed',
  'rolled_back',
])

export function DeliveryApprovalCenterPage({ state }: { state: DataState }) {
  const { currentProject } = useProject()
  const projectId = currentProject.id
  const [changeSets, setChangeSets] = useState<DeliveryControlChangeSet[]>([])
  const [selectedId, setSelectedId] = useState('')
  const [busy, setBusy] = useState(false)
  const [notice, setNotice] = useState('')

  const selected = useMemo(
    () => changeSets.find(changeSet => changeSet.id === selectedId),
    [changeSets, selectedId],
  )

  const refresh = useCallback(async () => {
    if (!projectId) return
    setBusy(true)
    try {
      const records = await deliveryPlanApi.listChangeSets(projectId)
      setChangeSets(records)
      setSelectedId(current => records.some(record => record.id === current) ? current : records[0]?.id ?? '')
      setNotice(records.length ? `已加载 ${records.length} 个 Delivery ChangeSet。` : '当前 Project 暂无 Delivery ChangeSet。')
    } catch (error) {
      setNotice(error instanceof Error ? error.message : '加载审批队列失败')
    } finally {
      setBusy(false)
    }
  }, [projectId])

  useEffect(() => {
    void refresh()
  }, [refresh])

  const apply = async (action: 'approve' | 'execute') => {
    if (!selected) return
    setBusy(true)
    try {
      const updated = action === 'approve'
        ? await deliveryPlanApi.approveChangeSet(projectId, selected.id, selected.version)
        : await deliveryPlanApi.executeChangeSet(projectId, selected.id, selected.version)
      setChangeSets(current => current.map(item => item.id === updated.id ? updated : item))
      setNotice(action === 'approve'
        ? `已批准 Plan V${updated.planVersion}；审批将在 24 小时后过期。`
        : '模拟执行完成，未写入真实广告平台。')
    } catch (error) {
      setNotice(error instanceof Error ? error.message : `${action === 'approve' ? '审批' : '执行'}失败`)
    } finally {
      setBusy(false)
    }
  }

  const approval = selected?.approval
  const approvalValid = selected?.status === 'approved' && approval?.valid === true
  const preflightPassed = selected ? preflightPassedStatuses.has(selected.status) : false

  return <StateBoundary
    state={state}
    contextLabel="智能投放 / 审批中心"
    emptyTitle="当前 Project 暂无待审批 ChangeSet"
    emptyDetail="在投放计划页完成服务端预检并创建 ChangeSet 后，审批快照会出现在这里。"
    errorDetail="审批队列暂时无法读取。请确认 Delivery 服务可用后刷新。"
    retryLabel="刷新审批队列"
  >
    <div className="approval-workspace">
      <aside className="approval-queue">
        <div className="surface-toolbar">
          <div><span className="section-label">Delivery A03</span><h3>审批队列</h3></div>
          <button onClick={() => void refresh()} disabled={busy} aria-label="刷新审批队列"><RotateCcw size={15}/></button>
        </div>
        <div className="mock-contract-banner"><b>source=mock</b><span>scope=execute_mock</span></div>
        {changeSets.map(item => <button
          key={item.id}
          className={selectedId === item.id ? 'active' : ''}
          aria-label={`${item.id} ${item.planName} Plan V${item.planVersion} ${item.status}`}
          onClick={() => setSelectedId(item.id)}
        >
          <span title={item.id}>{item.id.slice(-12)}</span>
          <b>{item.planName}</b>
          <small>Plan V{item.planVersion} · {item.status} · ¥{formatMinor(item.budgetLimit.totalMinor)}</small>
        </button>)}
      </aside>

      <section className="approval-detail">
        {selected ? <>
          <div className="approval-heading">
            <div>
              <span title={selected.id}>{selected.id.slice(-12)} · ChangeSet v{selected.version}</span>
              <h2>{selected.planName}</h2>
              <p>Plan V{selected.planVersion} 执行审批 · 批准内容由 canonical hash、ChangeSetVersion、执行范围和预算快照共同绑定。</p>
            </div>
            <div className="mock-source-stack">
              <span className={`approval-status ${selected.status}`}>{selected.status}</span>
              <span className="source-chip">source={selected.source}</span>
              <span className="source-chip">scenario={selected.scenario}</span>
            </div>
          </div>

          {approval && !approval.valid ? <div className="inline-notice danger approval-invalid-notice" role="alert">
            <CircleAlert size={16}/>
            <span><b>旧审批已失效</b><small>{approval.invalidReason ? invalidReasonLabels[approval.invalidReason] : '审批快照不再满足执行门禁。'}</small></span>
          </div> : null}

          <div className="execution-confirmation">
            <h3>批准内容快照</h3>
            <div><b>PlanVersion</b><span>V{selected.planVersion}</span></div>
            <div><b>ChangeSetVersion</b><span>v{approval?.changeSetVersion ?? selected.version}</span></div>
            <div><b>内容 Hash</b><span title={approval?.planCanonicalHash ?? selected.planCanonicalHash}>{approval?.hashSummary ?? selected.planCanonicalHash.slice(0, 12)}</span></div>
            <div><b>Action Hash</b><span title={approval?.actionHash}>{approval?.actionHash.slice(0, 12) ?? '批准后生成'}</span></div>
            <div><b>执行范围</b><span>{approval?.scope ?? 'execute_mock'}</span></div>
            <div><b>预算上限</b><span>¥{formatMinor(approval?.budgetLimit.totalMinor ?? selected.budgetLimit.totalMinor)} {approval?.budgetLimit.currency ?? selected.budgetLimit.currency}</span></div>
            <div><b>审批人</b><span>{approval?.approvedBy ?? '等待有 delivery.approve 权限的用户'}</span></div>
            <div><b>批准时间</b><span>{approval ? formatTime(approval.approvedAt) : '—'}</span></div>
            <div><b>有效期</b><span>{approval ? `至 ${formatTime(approval.expiresAt)}` : '批准后 24 小时'}</span></div>
          </div>

          <div className="approval-evidence">
            <h3>执行门禁</h3>
            <GateRow passed={preflightPassed} label="服务端预检" detail={preflightPassed ? '复用 RunPreflight' : '尚未通过'} />
            <GateRow passed={Boolean(approval)} label="不可变审批记录" detail={approval?.approvalId ?? '等待审批'} />
            <GateRow passed={approval?.valid === true} label="审批有效性" detail={approval?.valid ? '内容、有效期、范围和预算匹配' : approval?.invalidReason ?? '尚未批准'} />
          </div>

          <div className="approval-actions">
            <button
              className="secondary-button"
              onClick={() => void apply('execute')}
              disabled={busy || !approvalValid}
            ><Play size={15}/>模拟执行</button>
            <button
              className="primary-button"
              onClick={() => void apply('approve')}
              disabled={busy || selected.status !== 'preflight_passed'}
            ><ThumbsUp size={15}/>批准当前内容快照</button>
          </div>
        </> : <div className="panel-empty"><FileCheck2 size={24}/>没有可显示的 Delivery ChangeSet。</div>}
        {notice ? <div className="inline-notice" role="status">{notice}</div> : null}
      </section>

      <aside className="approval-audit">
        <div className="approval-audit-heading">
          <ShieldCheck size={18}/>
          <span className="section-label">权限与审计</span>
        </div>
        <div><time>身份来源</time><span>ActorContext</span></div>
        <div><time>批准动作</time><span>execute</span></div>
        <div><time>执行范围</time><span>execute_mock</span></div>
        <div><time>有效期</time><span>固定 24 小时</span></div>
        <div><time>审计策略</time><span>旧 Approval 保留，不覆盖</span></div>
      </aside>
    </div>
  </StateBoundary>
}

function GateRow({ passed, label, detail }: { passed: boolean; label: string; detail: string }) {
  return <div className={`gate-row ${passed ? 'passed' : 'failed'}`}>
    {passed ? <CircleCheck size={16}/> : <CircleAlert size={16}/>}
    <span><b>{label}</b><small>{detail}</small></span>
  </div>
}

function formatMinor(value: number) {
  return (value / 100).toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
}

function formatTime(value: string) {
  return new Date(value).toLocaleString('zh-CN')
}
