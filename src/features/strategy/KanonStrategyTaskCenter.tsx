import { Archive, ArrowRight, Check, ClipboardList, Plus, RefreshCw, RotateCcw, Target, X } from 'lucide-react'
import { useEffect, useMemo, useState, type FormEvent } from 'react'
import { useProject } from '../../context/ProjectContext'
import { createMutationKey, strategyApi } from './api'
import type { StrategyTaskBundle, StrategyTaskListItem } from './types'

export function KanonStrategyTaskCenter({ activeView, onOpenWorkspace, onRequestCreate }: {
  activeView: string
  onOpenWorkspace: (workspaceId: string) => void
  onRequestCreate: () => void
}) {
  const { currentProject } = useProject()
  const [tasks, setTasks] = useState<StrategyTaskListItem[]>([])
  const [selectedId, setSelectedId] = useState('')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [lifecycleItem, setLifecycleItem] = useState<StrategyTaskListItem | null>(null)

  const load = async (signal?: AbortSignal) => {
    setLoading(true)
    try {
      const result = await strategyApi.listTasks(currentProject.id, 'all', signal)
      setTasks(result.items)
      setSelectedId(current => result.items.some(item => item.task.id === current) ? current : result.items[0]?.task.id ?? '')
      setError('')
    } catch (cause) {
      if (!(cause instanceof DOMException && cause.name === 'AbortError')) {
        setError(cause instanceof Error ? cause.message : '暂时无法读取策略任务')
      }
    } finally {
      if (!signal?.aborted) setLoading(false)
    }
  }

  useEffect(() => {
    const controller = new AbortController()
    void load(controller.signal)
    return () => controller.abort()
  }, [currentProject.id])

  const visible = useMemo(() => tasks.filter(item => {
    const archived = isArchived(item)
    if (activeView === '已归档') return archived
    if (archived) return false
    const phase = taskPhase(item)
    if (activeView === '进行中') return phase !== '待评审' && phase !== '已完成'
    if (activeView === '待评审') return phase === '待评审'
    if (activeView === '已完成') return phase === '已完成'
    return true
  }), [activeView, tasks])
  const selected = visible.find(item => item.task.id === selectedId) ?? visible[0]

  if (loading) return <div className="kanon-task-state" role="status"><RefreshCw className="spin" size={22}/><b>正在读取策略任务</b></div>
  if (error) return <div className="kanon-task-state" role="alert"><ClipboardList size={24}/><b>暂时无法读取策略任务</b><p>{error}</p><button className="secondary-button" onClick={() => void load()}><RefreshCw size={14}/>重新加载</button></div>
  if (!visible.length) return <div className="kanon-task-empty"><ClipboardList size={30}/><h2>{tasks.length ? `没有“${activeView}”任务` : '创建第一个策略任务'}</h2><p>{activeView === '已归档' ? '废弃任务和归档策略会保留全部 Brief、版本、评审及审计记录，并可在这里恢复。' : '任务会真实创建工作区、需求对话与 Brief 草稿，并持续关联策略和评审。'}</p>{activeView !== '已归档' ? <button className="primary-button" onClick={onRequestCreate}><Plus size={16}/>新建策略任务</button> : null}</div>

  return <div className="kanon-task-center">
    <aside className="kanon-task-list" aria-label="策略任务列表">
      <header><div><span>STRATEGY TASKS</span><h3>策略任务</h3></div><strong>{visible.length}<small> / {tasks.length}</small></strong></header>
      {visible.map(item => <button className={item.task.id === selected?.task.id ? 'active' : ''} key={item.task.id} onClick={() => setSelectedId(item.task.id)}>
        <Target size={17}/><span><b>{item.name}</b><small>{item.objective}</small></span><em className={`phase-${taskPhase(item)}`}>{taskPhase(item)}</em>
      </button>)}
    </aside>
    <main className="kanon-task-detail">
      {selected ? <>
        <header><div><span>strategytask · 策略任务</span><h2>{selected.name}</h2><p>{selected.objective}</p></div><em>{taskPhase(selected)}</em></header>
        <section className="kanon-task-context" aria-label="策略任务上下文">
          <article><ClipboardList size={18}/><span><small>策略工作区</small><b>对话与 Brief 已创建</b><em>刷新后可以从当前进度继续</em></span></article>
          <article><Check size={18}/><span><small>Brief 状态</small><b>{selected.brief_status === 'confirmed' ? '已确认并冻结' : selected.brief_ready ? '可以确认' : '等待补充关键信息'}</b><em>{selected.brief_status === 'open' ? '当前为可编辑草稿' : '不可变输入已保存'}</em></span></article>
          <article><RefreshCw size={18}/><span><small>最近更新</small><b>{new Date(selected.task.updated_at).toLocaleDateString('zh-CN')}</b><em>{new Date(selected.task.updated_at).toLocaleTimeString('zh-CN', { hour12: false })}</em></span></article>
        </section>
        <ol className="kanon-task-stages">
          {['需求与 Brief', '策略形成', '策略评审', '完成归档'].map((label, index) => <li className={taskStageIndex(selected) >= index ? 'active' : ''} key={label}><span>{taskStageIndex(selected) > index ? <Check size={13}/> : String(index + 1).padStart(2, '0')}</span>{label}</li>)}
        </ol>
        <footer><small>{isArchived(selected) ? lifecycleReason(selected) : '所有状态来自 Strategy 真实领域数据'}</small><div className="kanon-task-actions">
          <button className="secondary-button" onClick={() => setLifecycleItem(selected)}>{isArchived(selected) ? <><RotateCcw size={14}/>恢复</> : <><Archive size={14}/>{selected.strategy_revision > 0 ? '归档策略' : '废弃任务'}</>}</button>
          <button className={isArchived(selected) ? 'secondary-button' : 'primary-button'} onClick={() => onOpenWorkspace(selected.task.workspace_id)}>{isArchived(selected) ? '查看只读工作区' : '进入策略工作区'}<ArrowRight size={15}/></button>
        </div></footer>
      </> : null}
    </main>
    {lifecycleItem ? <StrategyLifecycleDialog item={lifecycleItem} onClose={() => setLifecycleItem(null)} onDone={async () => {
      setLifecycleItem(null)
      await load()
    }}/> : null}
  </div>
}

function StrategyLifecycleDialog({ item, onClose, onDone }: {
  item: StrategyTaskListItem
  onClose: () => void
  onDone: () => Promise<void>
}) {
  const archived = isArchived(item)
  const archivesStrategy = item.strategy_revision > 0
  const [reason, setReason] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const action = archived ? '恢复' : archivesStrategy ? '归档策略' : '废弃任务'

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    if (!archived && !reason.trim()) return
    setBusy(true)
    setError('')
    try {
      if (item.task.discarded_at) {
        await strategyApi.restoreTask(item.task.id, item.task.version, createMutationKey('strategy-task-restore'))
      } else if (item.strategy_archived_at && item.task.current_strategy_id) {
        await strategyApi.restoreStrategy(item.task.current_strategy_id, item.strategy_version, createMutationKey('strategy-restore'))
      } else if (archivesStrategy && item.task.current_strategy_id) {
        await strategyApi.archiveStrategy(item.task.current_strategy_id, item.strategy_version, reason.trim(), createMutationKey('strategy-archive'))
      } else {
        await strategyApi.discardTask(item.task.id, item.task.version, reason.trim(), createMutationKey('strategy-task-discard'))
      }
      await onDone()
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : `${action}失败`)
      setBusy(false)
    }
  }

  return <div className="kanon-dialog-backdrop" onMouseDown={event => { if (event.target === event.currentTarget && !busy) onClose() }}>
    <form className="kanon-task-dialog kanon-lifecycle-dialog" aria-labelledby="strategy-lifecycle-title" onSubmit={submit} role="dialog">
      <header><div><span>任务生命周期</span><h2 id="strategy-lifecycle-title">{action}</h2><p>{archived ? '恢复后任务会重新出现在默认列表，并回到归档前的业务状态。' : archivesStrategy ? '归档后停止编辑和评审操作，但保留策略版本、评审与策略包。' : '废弃适用于尚未形成策略版本的任务，Brief、对话和审计记录仍会保留。'}</p></div><button aria-label="关闭" disabled={busy} onClick={onClose} type="button"><X size={18}/></button></header>
      <div className="kanon-task-dialog-note"><Archive size={18}/><div><b>{item.name}</b><small>{archived ? lifecycleReason(item) : '此操作不会物理删除任何业务数据'}</small></div></div>
      {!archived ? <div className="kanon-lifecycle-reason"><label>原因（必填）<textarea autoFocus maxLength={500} rows={3} placeholder={`请说明${action}原因，供后续审计追溯`} value={reason} onChange={event => setReason(event.target.value)}/><small>{reason.length} / 500</small></label></div> : null}
      {error ? <p className="kanon-dialog-error" role="alert">{error}</p> : null}
      <footer><button className="secondary-button" disabled={busy} onClick={onClose} type="button">取消</button><button className={archived ? 'primary-button' : 'danger-button'} disabled={busy || (!archived && !reason.trim())} type="submit">{busy ? '正在处理…' : `确认${action}`}</button></footer>
    </form>
  </div>
}

export function KanonStrategyTaskDialog({ onClose, onCreated }: {
  onClose: () => void
  onCreated: (bundle: StrategyTaskBundle) => void
}) {
  const { currentProject } = useProject()
  const [name, setName] = useState(`${currentProject.name} · 策略任务`)
  const [objective, setObjective] = useState(currentProject.goal || '')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    const close = (event: KeyboardEvent) => { if (event.key === 'Escape' && !busy) onClose() }
    window.addEventListener('keydown', close)
    return () => window.removeEventListener('keydown', close)
  }, [busy, onClose])

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    if (!name.trim() || !objective.trim()) return
    setBusy(true)
    setError('')
    try {
      onCreated(await strategyApi.createTask(
        currentProject.id, name.trim(), objective.trim(), createMutationKey('strategy-task'),
      ))
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : '创建策略任务失败')
      setBusy(false)
    }
  }

  return <div className="kanon-dialog-backdrop" onMouseDown={event => { if (event.target === event.currentTarget && !busy) onClose() }}>
    <form className="kanon-task-dialog" aria-labelledby="strategy-task-dialog-title" onSubmit={submit} role="dialog">
      <header><div><span>当前 Project · {currentProject.name}</span><h2 id="strategy-task-dialog-title">新建策略任务</h2><p>一次创建真实工作区、需求对话和 Brief 草稿，后续策略版本与评审持续写入同一任务。</p></div><button aria-label="关闭新建策略任务" disabled={busy} onClick={onClose} type="button"><X size={18}/></button></header>
      <div className="kanon-task-dialog-note"><Target size={18}/><div><b>完整策略工作链路</b><small>需求澄清 → Brief → 策略版本 → 评审与发布</small></div></div>
      <div className="kanon-task-dialog-fields">
        <label>任务名称<input autoFocus maxLength={255} value={name} onChange={event => setName(event.target.value)}/></label>
        <label>任务目标<textarea maxLength={4096} rows={4} value={objective} onChange={event => setObjective(event.target.value)}/></label>
      </div>
      {error ? <p className="kanon-dialog-error" role="alert">{error}</p> : null}
      <footer><button className="secondary-button" disabled={busy} onClick={onClose} type="button">取消</button><button className="primary-button" disabled={busy || !name.trim() || !objective.trim()} type="submit">{busy ? '正在创建…' : '创建并进入工作区'}</button></footer>
    </form>
  </div>
}

function taskPhase(item: StrategyTaskListItem) {
  if (item.task.discarded_at) return '已废弃'
  if (item.strategy_archived_at) return '已归档'
  if (item.review_status === 'approved') return '已完成'
  if (item.review_status === 'open' || item.strategy_status === 'ready_for_review') return '待评审'
  if (item.strategy_status === 'failed') return '生成失败'
  if (item.strategy_status) return '策略形成'
  if (item.brief_status === 'confirmed') return '生成策略'
  return '完善 Brief'
}

function taskStageIndex(item: StrategyTaskListItem) {
  if (taskPhase(item) === '已完成') return 4
  if (taskPhase(item) === '待评审') return 2
  if (item.strategy_status) return 1
  return 0
}

function isArchived(item: StrategyTaskListItem) {
  return Boolean(item.task.discarded_at || item.strategy_archived_at)
}

function lifecycleReason(item: StrategyTaskListItem) {
  const reason = item.task.discard_reason || item.strategy_archive_reason
  return reason ? `原因：${reason}` : '记录已保留，可随时恢复'
}
