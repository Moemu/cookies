import { useMemo, useState, type FormEvent } from 'react'
import {
  ArrowRight,
  Check,
  CircleCheck,
  Clock3,
  FileInput,
  Film,
  Link2,
  ListChecks,
  Plus,
  Target,
  X,
} from 'lucide-react'
import { useProject } from '../context/ProjectContext'
import type { BusinessTaskRecord, BusinessTaskType, DataState, SystemKey } from '../types'
import { StateBoundary } from './StateBoundary'

export const taskTypeMeta: Record<BusinessTaskType, { label: string; domain: 'strategy' | 'creative'; detail: string }> = {
  strategy: { label: '策略任务', domain: 'strategy', detail: '从 Brief、研究证据与业务目标形成可评审策略。' },
  creative: { label: '创意任务', domain: 'creative', detail: '基于已批准策略定义创意命题、渠道和交付规格。' },
  video: { label: '视频创作', domain: 'creative', detail: '创建通用视频创作任务并继承项目策略与品牌约束。' },
  brand_video: { label: '品牌广告', domain: 'creative', detail: '从 Brief 到剧本、资产、生成、剪辑和交付。' },
  short_drama_preroll: { label: '短剧前贴', domain: 'creative', detail: '以冲突与反转建立短剧开场的继续观看理由。' },
  game_preroll: { label: '游戏前贴', domain: 'creative', detail: '以挑战、失败和即时反馈建立游戏开场吸引力。' },
  commerce_preroll: { label: '电商前贴', domain: 'creative', detail: '突出商品动作、利益点与正片稳定衔接。' },
  viral_remake: { label: '爆款复刻', domain: 'creative', detail: '提取高表现结构并完成品牌化原创改写。' },
  video_edit: { label: '视频包装', domain: 'creative', detail: '选择已生成视频，完成混剪、字幕、声音和品牌包装。' },
}

const creativeTaskTypes: BusinessTaskType[] = [
  'creative',
  'video',
  'brand_video',
  'short_drama_preroll',
  'game_preroll',
  'commerce_preroll',
  'viral_remake',
  'video_edit',
]

const statusLabels: Record<BusinessTaskRecord['status'], string> = {
  draft: '草稿',
  in_progress: '进行中',
  ready: '待评审',
  completed: '已完成',
  failed: '失败',
}

export function ProjectLineage({ compact = false }: { compact?: boolean }) {
  const { currentProject } = useProject()
  const strategyTasks = currentProject.tasks.filter(task => task.type === 'strategy')
  const creativeTasks = currentProject.tasks.filter(task => taskTypeMeta[task.type].domain === 'creative')
  const stages = [
    { label: 'Brief', value: currentProject.artifacts.brief.version, ready: currentProject.artifacts.brief.status === '已确认' },
    { label: '策略任务', value: `${strategyTasks.length} 个`, ready: strategyTasks.length > 0 },
    { label: '创意任务', value: `${creativeTasks.length} 个`, ready: creativeTasks.length > 0 },
    { label: '素材洞察', value: currentProject.artifacts.insight.version, ready: Boolean(currentProject.artifacts.insight.id) },
    { label: '智能投放', value: `${currentProject.changeSets.length} 个`, ready: currentProject.changeSets.length > 0 },
  ]
  return <div className={compact ? 'project-lineage compact' : 'project-lineage'} aria-label="当前 Project 业务链路">
    {stages.map((stage, index) => <div className={stage.ready ? 'ready' : ''} key={stage.label}>
      <span>{stage.ready ? <Check size={13}/> : String(index + 1).padStart(2, '0')}</span>
      <small>{stage.label}</small>
      <b>{stage.value}</b>
      {index < stages.length - 1 ? <ArrowRight size={14}/> : null}
    </div>)}
  </div>
}

export function TaskCreateDialog({
  domain,
  initialType,
  onClose,
  onCreated,
}: {
  domain: 'strategy' | 'creative'
  initialType?: BusinessTaskType
  onClose: () => void
  onCreated: (task: BusinessTaskRecord) => void
}) {
  const { currentProject, createTask } = useProject()
  const choices = domain === 'strategy' ? ['strategy'] as BusinessTaskType[] : creativeTaskTypes
  const [type, setType] = useState<BusinessTaskType>(
    initialType && choices.includes(initialType) ? initialType : choices[0],
  )
  const [name, setName] = useState(() => `${currentProject.name} · ${taskTypeMeta[initialType ?? choices[0]].label}`)
  const [objective, setObjective] = useState(currentProject.goal)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  const chooseType = (next: BusinessTaskType) => {
    setType(next)
    setName(`${currentProject.name} · ${taskTypeMeta[next].label}`)
  }
  const submit = async (event: FormEvent) => {
    event.preventDefault()
    if (!name.trim() || !objective.trim()) return
    setBusy(true)
    setError('')
    try {
      const task = await createTask({ type, name: name.trim(), objective: objective.trim() })
      onCreated(task)
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : '任务创建失败')
    } finally {
      setBusy(false)
    }
  }

  return <div className="task-dialog-backdrop" role="presentation" onMouseDown={event => {
    if (event.target === event.currentTarget) onClose()
  }}>
    <form className="task-create-dialog" role="dialog" aria-modal="true" aria-labelledby="task-dialog-title" onSubmit={submit}>
      <header>
        <div><span>当前 Project · {currentProject.name}</span><h2 id="task-dialog-title">新建{domain === 'strategy' ? '策略' : '创意'}任务</h2><p>任务会写入服务端，并自动关联当前项目的 Brief、策略和后续产物。</p></div>
        <button type="button" aria-label="关闭新建任务" onClick={onClose}><X size={18}/></button>
      </header>
      {domain === 'creative' ? <div className="task-type-picker" role="radiogroup" aria-label="创意任务类型">
        {choices.map(choice => <button type="button" role="radio" aria-checked={type === choice} className={type === choice ? 'active' : ''} key={choice} onClick={() => chooseType(choice)}>
          {choice.includes('video') || choice.includes('preroll') || choice === 'viral_remake' ? <Film size={16}/> : <ListChecks size={16}/>}
          <span><b>{taskTypeMeta[choice].label}</b><small>{taskTypeMeta[choice].detail}</small></span>
        </button>)}
      </div> : <div className="strategy-task-definition"><Target size={18}/><div><b>策略任务</b><p>{taskTypeMeta.strategy.detail}</p></div></div>}
      <div className="task-form-fields">
        <label>任务名称<input value={name} onChange={event => setName(event.target.value)} autoFocus/></label>
        <label>任务目标<textarea value={objective} onChange={event => setObjective(event.target.value)}/></label>
      </div>
      <div className="task-source-preview">
        <FileInput size={16}/>
        <span><b>自动关联项目来源</b><small>Brief {currentProject.artifacts.brief.version} · 策略 {currentProject.artifacts.strategy.version} · Project {currentProject.code}</small></span>
      </div>
      {error ? <div className="inline-notice" role="alert">{error}</div> : null}
      <footer><button type="button" className="secondary-button" onClick={onClose}>取消</button><button type="submit" className="primary-button" disabled={busy || !name.trim() || !objective.trim()}>{busy ? '正在创建…' : '创建并打开任务'}</button></footer>
    </form>
  </div>
}

export function TaskCenterPage({
  state,
  domain,
  activeView,
  selectedId,
  onOpenTask,
  onRequestCreate,
}: {
  state: DataState
  domain: 'strategy' | 'creative'
  activeView: string
  selectedId?: string
  onOpenTask: (id: string) => void
  onRequestCreate: () => void
}) {
  const { currentProject, updateTask } = useProject()
  const [notice, setNotice] = useState('')
  const allTasks = useMemo(
    () => currentProject.tasks
      .filter(task => taskTypeMeta[task.type].domain === domain)
      .slice()
      .sort((left, right) => right.updatedAt.localeCompare(left.updatedAt)),
    [currentProject.tasks, domain],
  )
  const tasks = useMemo(
    () => allTasks.filter(task => matchesTaskView(task, activeView)),
    [activeView, allTasks],
  )
  const selected = tasks.find(task => task.id === selectedId) ?? tasks[0]
  const advance = async () => {
    if (!selected) return
    const status = selected.status === 'draft' ? 'in_progress' : selected.status === 'in_progress' ? 'ready' : 'completed'
    try {
      await updateTask(selected.id, { status })
      setNotice(`${selected.name} 已更新为${statusLabels[status]}`)
    } catch (cause) {
      setNotice(cause instanceof Error ? cause.message : '任务状态更新失败')
    }
  }

  return <StateBoundary state={state} onCreate={onRequestCreate}><div className="business-task-center">
    <div className="task-center-grid">
      <aside className="task-list-rail">
        <div className="surface-toolbar"><h3>{domain === 'strategy' ? '策略任务' : '创意任务'}</h3><span>{tasks.length}/{allTasks.length}</span></div>
        {tasks.map(task => <button className={selected?.id === task.id ? 'active' : ''} key={task.id} onClick={() => onOpenTask(task.id)}>
          <span className="task-type-icon">{task.type === 'strategy' ? <Target size={15}/> : <Film size={15}/>}</span>
          <span><b>{task.name}</b><small>{taskTypeMeta[task.type].label} · v{task.version}</small></span>
          <em>{statusLabels[task.status]}</em>
        </button>)}
        {!tasks.length ? <div className="task-list-empty"><ListChecks size={24}/><b>当前项目还没有任务</b><p>新建后会永久保存并关联项目来源。</p><button className="primary-button" onClick={onRequestCreate}><Plus size={15}/>新建任务</button></div> : null}
      </aside>
      <section className="task-work-area">
        {selected ? <>
          <header><div><span>{selected.id.slice(0, 8)} · {taskTypeMeta[selected.type].label}</span><h2>{selected.name}</h2><p>{selected.objective}</p></div><span className={`task-state ${selected.status}`}>{statusLabels[selected.status]}</span></header>
          <div className="task-context-grid">
            <article><Link2 size={17}/><small>所属 Project</small><b>{currentProject.name}</b><p>{currentProject.brand} · {currentProject.goal}</p></article>
            <article><FileInput size={17}/><small>上游输入</small><b>{selected.sourceTaskIds.length} 个任务 · {selected.sourceArtifactIds.length} 个产物</b><p>Brief、策略任务、版本和证据均保持项目内关联。</p></article>
            <article><CircleCheck size={17}/><small>输出与版本</small><b>{selected.outputArtifactIds.length} 个产物 · v{selected.version}</b><p>输出将在生成或保存后回写到该任务。</p></article>
          </div>
          <div className="task-stage-list">
            {(domain === 'strategy'
              ? ['校验 Brief 完整度', '组织受众与研究证据', '形成渠道和创意策略', '提交策略评审']
              : ['继承已批准策略', '配置渠道和生成规格', '制作与生成版本', '品牌检查和交付']
            ).map((stage, index) => <div className={index === 0 || selected.status !== 'draft' ? 'done' : ''} key={stage}><span>{index === 0 || selected.status !== 'draft' ? <Check size={13}/> : String(index + 1).padStart(2, '0')}</span><b>{stage}</b><small>{index === 0 ? '已继承当前 Project 上下文' : '等待任务推进'}</small></div>)}
          </div>
          <footer><span><Clock3 size={14}/>更新于 {new Date(selected.updatedAt).toLocaleString('zh-CN', { hour12: false })}</span><button className="primary-button" onClick={() => void advance()} disabled={selected.status === 'completed'}>{selected.status === 'draft' ? '开始任务' : selected.status === 'in_progress' ? '提交评审' : selected.status === 'ready' ? '确认完成' : '任务已完成'}</button></footer>
          {notice ? <div className="inline-notice" role="status">{notice}</div> : null}
        </> : <div className="task-work-empty"><ListChecks size={30}/><h2>创建第一个{domain === 'strategy' ? '策略' : '创意'}任务</h2><p>任务将绑定当前 Project，并成为后续模块的数据来源。</p></div>}
      </section>
    </div>
  </div></StateBoundary>
}

export function moduleTaskType(system: SystemKey, view?: string): BusinessTaskType {
  if (system === 'strategy') return 'strategy'
  if (view === '品牌广告') return 'brand_video'
  if (view === '素材剪辑') return 'video_edit'
  return 'creative'
}

function matchesTaskView(task: BusinessTaskRecord, view: string): boolean {
  if (view === '进行中') return task.status === 'in_progress'
  if (view === '待评审') return task.status === 'ready'
  if (view === '已完成') return task.status === 'completed'
  if (view === '失败') return task.status === 'failed'
  if (view === '等待输入') return task.status === 'draft'
  if (view === '生成中') return task.status === 'in_progress' && task.type !== 'strategy'
  if (view === '归档') return false
  return true
}
