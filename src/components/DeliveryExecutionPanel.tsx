import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { CircleAlert, CircleCheck, ListRestart, Play, RefreshCw, ShieldAlert } from 'lucide-react'
import {
  deliveryExecutionApi,
  type DeliveryControlChangeSet,
  type DeliveryExecutionRecord,
  type DeliveryExecutionScenario,
} from '../api/delivery'

const scenarios: Array<{ value: DeliveryExecutionScenario; label: string }> = [
  { value: 'success', label: 'success · 已确认完成' },
  { value: 'failed', label: 'failed · 确认未产生目标效果' },
  { value: 'partial', label: 'partial · 仅部分完成' },
  { value: 'result_unknown', label: 'result_unknown · 结果待核验' },
]

type Props = {
  projectId: string
  changeSet: DeliveryControlChangeSet
  canExecute: boolean
  onExecutionCreated: (changeSet: DeliveryControlChangeSet) => void
}

export function DeliveryExecutionPanel({ projectId, changeSet, canExecute, onExecutionCreated }: Props) {
  const [records, setRecords] = useState<DeliveryExecutionRecord[]>([])
  const [selectedId, setSelectedId] = useState('')
  const [selectedRecord, setSelectedRecord] = useState<DeliveryExecutionRecord>()
  const [scenario, setScenario] = useState<DeliveryExecutionScenario>('success')
  const [busy, setBusy] = useState(false)
  const [notice, setNotice] = useState('')
  const idempotencyKeys = useRef(new Map<string, string>())

  const refresh = useCallback(async () => {
    if (!projectId) return
    setBusy(true)
    try {
      const values = await deliveryExecutionApi.list(projectId)
      setRecords(values)
      setSelectedId(current => {
        if (values.some(value => value.execution.id === current)) return current
        return values.find(value => value.execution.changeSetId === changeSet.id)?.execution.id ?? values[0]?.execution.id ?? ''
      })
      setNotice(values.length ? `已从 Go 权威 API 加载 ${values.length} 条 Execution 记录。` : '当前 Project 暂无 Execution 记录。')
    } catch (error) {
      setNotice(error instanceof Error ? error.message : '读取 Execution 记录失败。')
    } finally {
      setBusy(false)
    }
  }, [changeSet.id, projectId])

  useEffect(() => {
    void refresh()
  }, [refresh])

  useEffect(() => {
    if (!projectId || !selectedId) {
      setSelectedRecord(undefined)
      return
    }
    let cancelled = false
    void deliveryExecutionApi.get(projectId, selectedId)
      .then(value => {
        if (!cancelled) setSelectedRecord(value)
      })
      .catch(error => {
        if (!cancelled) setNotice(error instanceof Error ? error.message : '读取 Execution 明细失败。')
      })
    return () => { cancelled = true }
  }, [projectId, selectedId])

  const selectedSummary = useMemo(
    () => records.find(value => value.execution.id === selectedId),
    [records, selectedId],
  )
  const record = selectedRecord?.execution.id === selectedId ? selectedRecord : selectedSummary
  const unresolvedExecution = records.find(value => (
    value.execution.changeSetId === changeSet.id && value.execution.status === 'result_unknown'
  ))
  const canStart = canExecute && !unresolvedExecution && !busy

  const startExecution = async () => {
    if (!canStart) return
    const keyScope = `${changeSet.id}:${scenario}`
    const idempotencyKey = idempotencyKeys.current.get(keyScope) ?? createIdempotencyKey()
    idempotencyKeys.current.set(keyScope, idempotencyKey)
    setBusy(true)
    try {
      const value = await deliveryExecutionApi.execute(projectId, changeSet.id, changeSet.version, scenario, idempotencyKey)
      setRecords(current => [value, ...current.filter(item => item.execution.id !== value.execution.id)])
      setSelectedId(value.execution.id)
      setSelectedRecord(value)
      onExecutionCreated(value.changeSet)
      setNotice(`已创建或复用 Execution ${value.execution.id.slice(-12)}。所有结果来自 source=mock。`)
    } catch (error) {
      setNotice(error instanceof Error ? error.message : '启动模拟执行失败。')
    } finally {
      setBusy(false)
    }
  }

  return <section className="delivery-execution-panel" aria-label="模拟执行记录">
    <header className="delivery-execution-header">
      <div>
        <span className="section-label">Delivery A04 · Execution</span>
        <h3>持久执行与恢复判断</h3>
        <p>Execution 是一次模拟执行尝试，独立于不可变的 ChangeSet 审批证据；不会写入真实平台。</p>
      </div>
      <button className="secondary-button" onClick={() => void refresh()} disabled={busy}>
        <RefreshCw size={15}/>从 Go API 刷新
      </button>
    </header>

    <div className="mock-contract-banner execution-provenance">
      <b>source=mock</b><span>adapter=mock_ocean_engine</span><span>mode=local_simulation</span>
    </div>

    {unresolvedExecution ? <div className="execution-recovery-alert" role="alert">
      <ShieldAlert size={18}/><span><b>禁止盲目重试</b><small>Execution {unresolvedExecution.execution.id.slice(-12)} 的结果未知。请先查询并重新核验，再生成恢复决定；不得复用此 ChangeSet 直接重试。</small></span>
    </div> : null}

    <div className="execution-start-controls">
      <label>模拟场景
        <select value={scenario} onChange={event => setScenario(event.target.value as DeliveryExecutionScenario)} disabled={!canExecute || busy}>
          {scenarios.map(item => <option key={item.value} value={item.value}>{item.label}</option>)}
        </select>
      </label>
      <button className="secondary-button" onClick={() => void startExecution()} disabled={!canStart}>
        <Play size={15}/>启动模拟执行
      </button>
      {!canExecute ? <small>需要有效审批后才能开始模拟执行。</small> : null}
    </div>

    <div className="execution-records">
      <div className="execution-list" aria-label="Execution 列表">
        {records.length ? records.map(item => <button
          key={item.execution.id}
          className={item.execution.id === selectedId ? 'active' : ''}
          onClick={() => setSelectedId(item.execution.id)}
        >
          <span>{item.execution.id.slice(-12)}</span>
          <b>{item.execution.status}</b>
          <small>scenario={item.execution.scenario} · {formatTime(item.execution.startedAt)}</small>
        </button>) : <div className="panel-empty"><ListRestart size={20}/>刷新后将在这里显示服务端执行记录。</div>}
      </div>
      <div className="execution-detail">
        {record ? <ExecutionDetail record={record}/> : <div className="panel-empty">请选择一条 Execution 查看权威明细。</div>}
      </div>
    </div>
    {notice ? <div className="inline-notice" role="status">{notice}</div> : null}
  </section>
}

function ExecutionDetail({ record }: { record: DeliveryExecutionRecord }) {
  const { execution, evidence } = record
  const completed = execution.steps.filter(step => step.status === 'succeeded')
  const incomplete = execution.steps.filter(step => step.status !== 'succeeded')
  return <>
    <div className="execution-detail-heading">
      <div><span>{execution.id}</span><h4>{execution.status}</h4></div>
      <div className="execution-chips"><span>source={execution.source}</span><span>scenario={execution.scenario}</span></div>
    </div>

    {execution.status === 'failed' ? <div className="execution-outcome failed"><CircleCheck size={16}/><span><b>未产生目标效果</b><small>failed 仅表示服务端已确认目标效果没有产生，并非结果未知。</small></span></div> : null}
    {execution.status === 'result_unknown' ? <div className="execution-outcome unknown"><CircleAlert size={16}/><span><b>结果未知，不能盲目重试</b><small>{execution.recoveryReason || '先查询并重新核验目标状态，再创建恢复决定。'}</small></span></div> : null}
    {execution.status === 'partial' ? <div className="execution-outcome partial"><CircleAlert size={16}/><span><b>部分完成，需要受控恢复</b><small>{execution.recoveryReason || '补偿不是自动回滚，需作为新的受控动作处理。'}</small></span></div> : null}

    <dl className="execution-meta">
      <div><dt>执行模式</dt><dd>{execution.mode}</dd></div>
      <div><dt>适配器</dt><dd>{execution.adapter}</dd></div>
      <div><dt>请求 Hash</dt><dd title={execution.requestHash}>{shortHash(execution.requestHash)}</dd></div>
      <div><dt>恢复动作</dt><dd>{execution.recoveryAction}</dd></div>
      <div><dt>重试许可</dt><dd>{execution.retryAllowed ? '服务端允许' : '服务端禁止'}</dd></div>
    </dl>
    <div className="execution-recovery-reason"><b>恢复原因</b><p>{execution.recoveryReason || '服务端未要求恢复动作。'}</p></div>

    {execution.status === 'partial' ? <div className="execution-scope">
      <div><b>已完成范围</b>{completed.length ? completed.map(step => <span key={step.id}>{step.sequence}. {step.action}</span>) : <small>尚无已完成步骤。</small>}</div>
      <div><b>未完成范围</b>{incomplete.length ? incomplete.map(step => <span key={step.id}>{step.sequence}. {step.action} · {step.status}</span>) : <small>没有未完成步骤。</small>}</div>
      <div><b>补偿候选</b>{execution.compensationCandidates.length ? execution.compensationCandidates.map(candidate => <span key={candidate}>{candidate}</span>) : <small>服务端未提供补偿候选。</small>}</div>
    </div> : null}

    <h5>持久步骤</h5>
    <div className="execution-step-table">
      {execution.steps.length ? execution.steps.map(step => <article key={step.id}>
        <b>{step.sequence}. {step.action}</b><span>{step.status} · effect={step.effect}</span><small>{step.outcomeSummary}</small><code>{step.evidenceRef ?? '无 Evidence reference'}</code>
      </article>) : <small>服务端尚未返回步骤。</small>}
    </div>
    <h5>脱敏 Evidence references</h5>
    <div className="execution-evidence"><p>{evidence.summary}</p>{evidence.references.length ? evidence.references.map(reference => <code key={reference}>{reference}</code>) : <small>服务端尚未返回 Evidence reference。</small>}</div>
  </>
}

function createIdempotencyKey() {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') return crypto.randomUUID()
  return `delivery-execution-${Date.now()}-${Math.random().toString(16).slice(2)}`
}

function shortHash(value: string) {
  return value.length > 16 ? `${value.slice(0, 16)}…` : value
}

function formatTime(value: string) {
  return new Date(value).toLocaleString('zh-CN')
}
