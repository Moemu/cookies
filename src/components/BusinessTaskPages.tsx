import { useEffect, useMemo, useState, type FormEvent } from 'react'
import {
  ArrowRight,
  Archive,
  Check,
  CircleCheck,
  Clock3,
  FileInput,
  Film,
  Link2,
  ListChecks,
  Plus,
  Search,
  ShieldCheck,
  Target,
  X,
} from 'lucide-react'
import { useProject } from '../context/ProjectContext'
import type { BusinessTaskRecord, BusinessTaskType, DataState, SystemKey } from '../types'
import type { ApiAgencyWorkbench, ApiAssetVersionPointer, ApiMaterialConfirmation, ApiQualityCheckRun } from '../data/api'
import { StateBoundary } from './StateBoundary'
import { shortId } from '../data/shortId'

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

type CreativeQuickView = '全部' | '我的任务' | '今日到期' | '质检未通过' | '待人工确认' | '需要修改' | '生成失败'
type CreativeBatchAction = 'assign' | 'add_specs' | 'start_generation' | 'run_quality' | 'human_review' | 'export_confirmed' | 'archive'
type StatusTone = 'success' | 'warning' | 'danger' | 'info'

interface CreativeOpsRow {
  task: BusinessTaskRecord
  projectId: string
  projectName: string
  clientName: string
  brandName: string
  assetName: string
  channelSpec: string
  owner: string
  priority: '高' | '中' | '低'
  dueLabel: string
  qualityLabel: string
  qualityTone: StatusTone
  confirmationLabel: string
  confirmationTone: StatusTone
  deliveryLabel: string
  deliveryTone: StatusTone
  latestVersion: string
  updatedLabel: string
  supportedActions: Set<CreativeBatchAction>
  searchText: string
}

const creativeQuickViews: CreativeQuickView[] = ['全部', '我的任务', '今日到期', '质检未通过', '待人工确认', '需要修改', '生成失败']
const batchActionLabels: Record<CreativeBatchAction, string> = {
  assign: '分配负责人/截止',
  add_specs: '添加渠道规格',
  start_generation: '开始/重新生成',
  run_quality: '运行大模型质检',
  human_review: '进入人工检查',
  export_confirmed: '导出确认版本',
  archive: '归档',
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
        <div><span>当前 Project · {currentProject.name}</span><h2 id="task-dialog-title">新建{domain === 'strategy' ? '策略' : '创意'}任务</h2><p>{domain === 'creative' ? '先确定任务类型，再进入对应创意工作区；所有生成、素材和交付仍写入同一条 Project 链路。' : '任务会写入服务端，并自动关联当前项目的 Brief、策略和后续产物。'}</p></div>
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
  onContinueTask,
  onOpenProject,
}: {
  state: DataState
  domain: 'strategy' | 'creative'
  activeView: string
  selectedId?: string
  onOpenTask: (id: string) => void
  onRequestCreate: () => void
  onContinueTask?: (task: BusinessTaskRecord) => void
  onOpenProject?: (projectId: string, system?: SystemKey, navId?: string, objectId?: string, view?: string) => void
}) {
  const { agencyWorkbench, currentProject, projects, updateTask } = useProject()
  const [search, setSearch] = useState('')
  const [quickView, setQuickView] = useState<CreativeQuickView>(() => creativeQuickViews.includes(activeView as CreativeQuickView) ? activeView as CreativeQuickView : '全部')
  const [selectedIds, setSelectedIds] = useState<string[]>([])
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
  const creativeRows = useMemo(
    () => domain === 'creative' ? buildCreativeOpsRows(projects, agencyWorkbench, currentProject.id) : [],
    [agencyWorkbench, currentProject.id, domain, projects],
  )
  const visibleCreativeRows = useMemo(() => {
    const query = search.trim().toLowerCase()
    return creativeRows.filter(row => matchesCreativeQuickView(row, quickView, currentProject.owner))
      .filter(row => !query || row.searchText.includes(query))
  }, [creativeRows, currentProject.owner, quickView, search])
  const selectedCreativeRows = visibleCreativeRows.filter(row => selectedIds.includes(row.task.id))
  const commonActions = commonBatchActions(selectedCreativeRows)

  useEffect(() => {
    if (domain !== 'creative') return
    setSelectedIds(current => current.filter(id => visibleCreativeRows.some(row => row.task.id === id)))
  }, [domain, visibleCreativeRows])

  useEffect(() => {
    if (domain === 'creative' && creativeQuickViews.includes(activeView as CreativeQuickView)) {
      setQuickView(activeView as CreativeQuickView)
    }
  }, [activeView, domain])

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

  const selectedCreative = visibleCreativeRows.find(row => row.task.id === selectedId)?.task ?? visibleCreativeRows[0]?.task
  const toggleCreativeRow = (id: string) => {
    setSelectedIds(current => current.includes(id) ? current.filter(item => item !== id) : [...current, id])
  }
  const toggleAllVisibleCreativeRows = () => {
    setSelectedIds(current => current.length === visibleCreativeRows.length ? [] : visibleCreativeRows.map(row => row.task.id))
  }
  const openCreativeRow = (row: CreativeOpsRow) => {
    if (row.projectId === currentProject.id) {
      onOpenTask(row.task.id)
    } else {
      onOpenProject?.(row.projectId, 'creative', 'tasks', row.task.id, quickView)
    }
  }
  const continueCreativeTask = (task: BusinessTaskRecord) => {
    if (task.projectId === currentProject.id) onContinueTask?.(task)
    else onOpenProject?.(task.projectId, 'creative', 'tasks', task.id, quickView)
  }
  const announceBatchAction = (action: CreativeBatchAction) => {
    setNotice(`已选择 ${selectedCreativeRows.length} 个任务，可执行“${batchActionLabels[action]}”。混合状态下只展示共同支持动作。`)
  }

  if (domain === 'creative') {
    const selectedRow = visibleCreativeRows.find(row => row.task.id === selectedCreative?.id)
    return <StateBoundary state={state} onCreate={onRequestCreate}><div className="business-task-center creative-ops-center">
      <section className="creative-ops-toolbar" aria-label="创意任务批量运营筛选">
        <div><span className="section-label">CREATIVE OPS</span><h3>创意任务批量运营</h3><p>按客户、Project、渠道规格、质检与人工确认状态筛选，批量动作只显示选中对象共同支持的操作。</p></div>
        <label className="creative-task-search"><Search size={15}/><input value={search} onChange={event => setSearch(event.target.value)} placeholder="搜索任务名称、ID 或素材名称"/></label>
      </section>
      <nav className="creative-quick-views" aria-label="创意任务快捷视图">
        {creativeQuickViews.map(view => <button key={view} className={quickView === view ? 'active' : ''} onClick={() => setQuickView(view)}>{view}<small>{creativeRows.filter(row => matchesCreativeQuickView(row, view, currentProject.owner)).length}</small></button>)}
      </nav>
      <section className="creative-batch-toolbar" aria-live="polite" aria-label="批量工具栏">
        <span>{selectedCreativeRows.length ? `已选择 ${selectedCreativeRows.length} 个任务` : '选择任务后显示共同批量动作'}</span>
        <div>{(Object.keys(batchActionLabels) as CreativeBatchAction[]).map(action => <button key={action} className="secondary-button" disabled={!commonActions.includes(action)} onClick={() => announceBatchAction(action)}>{batchActionLabels[action]}</button>)}</div>
      </section>
      <div className="creative-ops-layout">
        <section className="creative-task-table" aria-label="创意任务列表">
          <div className="creative-task-head">
            <label><input type="checkbox" checked={visibleCreativeRows.length > 0 && selectedIds.length === visibleCreativeRows.length} onChange={toggleAllVisibleCreativeRows}/></label>
            <span>任务 / 素材</span><span>客户 / Project</span><span>渠道规格</span><span>负责人</span><span>优先级 / 截止</span><span>质检</span><span>人工确认</span><span>交付</span><span>版本</span>
          </div>
          {visibleCreativeRows.map(row => <div key={row.task.id} className={selectedCreative?.id === row.task.id ? 'creative-task-row active' : 'creative-task-row'}>
            <label aria-label={`选择 ${row.task.name}`} onClick={event => event.stopPropagation()}><input type="checkbox" checked={selectedIds.includes(row.task.id)} onChange={() => toggleCreativeRow(row.task.id)}/></label>
            <button className="creative-task-main" onClick={() => openCreativeRow(row)}><b>{row.task.name}</b><small>{row.task.id} · {row.assetName}</small></button>
            <button className="creative-task-context" onClick={() => openCreativeRow(row)}><b>{row.clientName}</b><small>{row.projectName}</small></button>
            <span>{row.channelSpec}</span>
            <span>{row.owner}</span>
            <span><b className={`priority-dot ${priorityTone(row.priority)}`}>{row.priority}</b><small>{row.dueLabel}</small></span>
            <StatusPill label={row.qualityLabel} tone={row.qualityTone}/>
            <StatusPill label={row.confirmationLabel} tone={row.confirmationTone}/>
            <StatusPill label={row.deliveryLabel} tone={row.deliveryTone}/>
            <span><b>{row.latestVersion}</b><small>{row.updatedLabel}</small></span>
          </div>)}
          {!visibleCreativeRows.length ? <div className="task-list-empty"><ListChecks size={24}/><b>没有匹配的创意任务</b><p>可切换快捷视图或清空搜索条件。</p><button className="primary-button" onClick={onRequestCreate}><Plus size={15}/>新建任务</button></div> : null}
        </section>
        <aside className="creative-task-detail" aria-label="创意任务运营详情">
          {selectedRow ? <>
            <span className="section-label">{selectedRow.task.id}</span>
            <h3>{selectedRow.task.name}</h3>
            <p>{selectedRow.task.objective}</p>
            <dl>
              <div><dt>客户 / 品牌</dt><dd>{selectedRow.clientName} / {selectedRow.brandName}</dd></div>
              <div><dt>Project</dt><dd>{selectedRow.projectName}</dd></div>
              <div><dt>素材</dt><dd>{selectedRow.assetName} · {selectedRow.latestVersion}</dd></div>
              <div><dt>状态链路</dt><dd>{statusLabels[selectedRow.task.status]} · {selectedRow.qualityLabel} · {selectedRow.confirmationLabel} · {selectedRow.deliveryLabel}</dd></div>
            </dl>
            <div className="creative-detail-actions">
              <button className="secondary-button full" onClick={() => announceBatchAction('run_quality')} disabled={!selectedRow.supportedActions.has('run_quality')}><ShieldCheck size={15}/>运行质检</button>
              <button className="secondary-button full" onClick={() => announceBatchAction('archive')} disabled={!selectedRow.supportedActions.has('archive')}><Archive size={15}/>归档任务</button>
              <button className="primary-button full" onClick={() => continueCreativeTask(selectedRow.task)} disabled={!onContinueTask}>继续制作<ArrowRight size={15}/></button>
            </div>
            {notice ? <div className="inline-notice" role="status">{notice}</div> : null}
          </> : <div className="task-work-empty"><ListChecks size={30}/><h2>创建第一个创意任务</h2><p>任务将绑定当前 Project，并进入批量运营视图。</p></div>}
        </aside>
      </div>
    </div></StateBoundary>
  }

  return <StateBoundary state={state} onCreate={onRequestCreate}><div className="business-task-center">
    <div className="task-center-grid">
      <aside className="task-list-rail">
        <div className="surface-toolbar"><h3>{domain === 'strategy' ? '策略任务' : '统一创意入口'}</h3><span>{tasks.length}/{allTasks.length}</span></div>
        {tasks.map(task => <button className={selected?.id === task.id ? 'active' : ''} key={task.id} onClick={() => onOpenTask(task.id)}>
          <span className="task-type-icon">{task.type === 'strategy' ? <Target size={15}/> : <Film size={15}/>}</span>
          <span><b>{task.name}</b><small>{taskTypeMeta[task.type].label} · v{task.version}</small></span>
          <em>{statusLabels[task.status]}</em>
        </button>)}
        {!tasks.length ? <div className="task-list-empty"><ListChecks size={24}/><b>当前项目还没有任务</b><p>新建后会永久保存并关联项目来源。</p><button className="primary-button" onClick={onRequestCreate}><Plus size={15}/>新建任务</button></div> : null}
      </aside>
      <section className="task-work-area">
        {selected ? <>
          <header><div><span>{shortId(selected.id)} · {taskTypeMeta[selected.type].label}</span><h2>{selected.name}</h2><p>{selected.objective}</p></div><span className={`task-state ${selected.status}`}>{statusLabels[selected.status]}</span></header>
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

function buildCreativeOpsRows(projects: ReturnType<typeof useProject>['projects'], workbench: ApiAgencyWorkbench | null, currentProjectId: string): CreativeOpsRow[] {
  const agencyProjects = new Map(workbench?.projects.map(project => [project.id, project]) ?? [])
  const clients = new Map(workbench?.clients.map(client => [client.id, client]) ?? [])
  const brands = new Map(workbench?.brands.map(brand => [brand.id, brand]) ?? [])
  const allRows = projects.flatMap(project => project.tasks
    .filter(task => taskTypeMeta[task.type].domain === 'creative')
    .map((task, index) => {
      const agencyProject = agencyProjects.get(project.id)
      const brand = brands.get(agencyProject?.brandId ?? '')
      const client = clients.get(agencyProject?.clientId ?? brand?.clientId ?? '')
      const pointers = workbench?.assetVersionPointers.filter(pointer => pointer.projectId === project.id) ?? []
      const pointer = pointers.find(item => task.outputArtifactIds.includes(item.assetId)) ?? pointers[index % Math.max(pointers.length, 1)]
      return toCreativeOpsRow(task, project, pointer, workbench, {
        clientName: client?.name ?? '客户未分配',
        brandName: brand?.name ?? project.brand,
        projectName: project.name,
        projectOwner: agencyProject?.runtime.owner ?? project.owner,
      })
    }))
  return allRows.sort((left, right) => Number(right.projectId === currentProjectId) - Number(left.projectId === currentProjectId)
    || right.task.updatedAt.localeCompare(left.task.updatedAt))
}

function toCreativeOpsRow(
  task: BusinessTaskRecord,
  project: ReturnType<typeof useProject>['projects'][number],
  pointer: ApiAssetVersionPointer | undefined,
  workbench: ApiAgencyWorkbench | null,
  context: { clientName: string; brandName: string; projectName: string; projectOwner: string },
): CreativeOpsRow {
  const qualityRun = pointer ? latestQualityRun(pointer, workbench) : undefined
  const confirmation = pointer ? latestConfirmation(pointer, workbench) : undefined
  const quality = creativeQualityState(task, pointer, qualityRun)
  const confirmationState = creativeConfirmationState(confirmation, quality)
  const delivery = creativeDeliveryState(pointer, confirmation, quality)
  const priority = creativePriority(task, quality, confirmation)
  const dueLabel = creativeDueLabel(task, priority)
  const latestVersion = pointer ? `v${pointer.workingVersion}` : `v${task.version}`
  const assetName = pointer ? materialTitle(pointer.assetId) : `${taskTypeMeta[task.type].label}素材`
  const supportedActions = creativeSupportedActions(task, quality, confirmation, delivery)
  const searchText = [
    task.name,
    task.id,
    task.objective,
    assetName,
    context.clientName,
    context.brandName,
    context.projectName,
    context.projectOwner,
  ].join(' ').toLowerCase()
  return {
    task,
    projectId: project.id,
    projectName: context.projectName,
    clientName: context.clientName,
    brandName: context.brandName,
    assetName,
    channelSpec: channelSpecLabel(task.type),
    owner: pointer?.owner ?? context.projectOwner,
    priority,
    dueLabel,
    qualityLabel: quality.label,
    qualityTone: quality.tone,
    confirmationLabel: confirmationState.label,
    confirmationTone: confirmationState.tone,
    deliveryLabel: delivery.label,
    deliveryTone: delivery.tone,
    latestVersion,
    updatedLabel: formatTaskDate(pointer?.updatedAt ?? task.updatedAt),
    supportedActions,
    searchText,
  }
}

function latestQualityRun(pointer: ApiAssetVersionPointer, workbench: ApiAgencyWorkbench | null): ApiQualityCheckRun | undefined {
  return workbench?.qualityCheckRuns
    .filter(run => run.projectId === pointer.projectId && run.assetId === pointer.assetId && run.assetVersion === pointer.workingVersion)
    .sort((left, right) => right.createdAt.localeCompare(left.createdAt))[0]
}

function latestConfirmation(pointer: ApiAssetVersionPointer, workbench: ApiAgencyWorkbench | null): ApiMaterialConfirmation | undefined {
  return workbench?.materialConfirmations
    .filter(item => item.projectId === pointer.projectId && item.assetId === pointer.assetId && item.assetVersion === pointer.workingVersion)
    .sort((left, right) => right.createdAt.localeCompare(left.createdAt))[0]
}

function creativeQualityState(task: BusinessTaskRecord, pointer: ApiAssetVersionPointer | undefined, run: ApiQualityCheckRun | undefined): { label: string; tone: StatusTone; failed: boolean; passed: boolean } {
  if (task.status === 'failed') return { label: '生成失败', tone: 'danger', failed: true, passed: false }
  if (!pointer) return { label: '待生成', tone: 'warning', failed: false, passed: false }
  if (!run) return { label: pointer.workingVersion > (pointer.qualityCheckedVersion ?? 0) ? '待质检' : '无质检记录', tone: 'warning', failed: false, passed: false }
  if (run.status === 'failed') return { label: '质检未通过', tone: 'danger', failed: true, passed: false }
  if (run.status === 'passed') return { label: '质检通过', tone: 'success', failed: false, passed: true }
  return { label: run.status === 'running' ? '质检中' : '排队中', tone: 'info', failed: false, passed: false }
}

function creativeConfirmationState(confirmation: ApiMaterialConfirmation | undefined, quality: ReturnType<typeof creativeQualityState>): { label: string; tone: StatusTone } {
  if (confirmation?.status === 'confirmed') return { label: '已人工确认', tone: 'success' }
  if (confirmation?.status === 'changes_requested') return { label: '需要修改', tone: 'danger' }
  if (quality.passed) return { label: '待人工确认', tone: 'info' }
  return { label: '未就绪', tone: quality.failed ? 'danger' : 'warning' }
}

function creativeDeliveryState(pointer: ApiAssetVersionPointer | undefined, confirmation: ApiMaterialConfirmation | undefined, quality: ReturnType<typeof creativeQualityState>): { label: string; tone: StatusTone; ready: boolean } {
  if (pointer?.deliveryVersion === pointer?.workingVersion) return { label: '已交付', tone: 'success', ready: true }
  if (confirmation?.status === 'confirmed') return { label: '可交付', tone: 'info', ready: true }
  if (confirmation?.status === 'changes_requested' || quality.failed) return { label: '交付阻塞', tone: 'danger', ready: false }
  return { label: '未交付', tone: 'warning', ready: false }
}

function creativeSupportedActions(
  task: BusinessTaskRecord,
  quality: ReturnType<typeof creativeQualityState>,
  confirmation: ApiMaterialConfirmation | undefined,
  delivery: ReturnType<typeof creativeDeliveryState>,
): Set<CreativeBatchAction> {
  const actions = new Set<CreativeBatchAction>(['assign', 'add_specs'])
  if (task.status === 'draft' || task.status === 'failed' || confirmation?.status === 'changes_requested') actions.add('start_generation')
  if (task.status !== 'draft' && task.status !== 'failed' && !delivery.ready) actions.add('run_quality')
  if (quality.passed && !confirmation) actions.add('human_review')
  if (confirmation?.status === 'confirmed') actions.add('export_confirmed')
  if (task.status === 'completed' || confirmation?.status === 'confirmed') actions.add('archive')
  return actions
}

function commonBatchActions(rows: CreativeOpsRow[]): CreativeBatchAction[] {
  if (!rows.length) return []
  return (Object.keys(batchActionLabels) as CreativeBatchAction[])
    .filter(action => rows.every(row => row.supportedActions.has(action)))
}

function matchesCreativeQuickView(row: CreativeOpsRow, view: CreativeQuickView, currentOwner: string): boolean {
  if (view === '我的任务') return row.owner === currentOwner
  if (view === '今日到期') return row.dueLabel.startsWith('今日')
  if (view === '质检未通过') return row.qualityLabel === '质检未通过'
  if (view === '待人工确认') return row.confirmationLabel === '待人工确认'
  if (view === '需要修改') return row.confirmationLabel === '需要修改'
  if (view === '生成失败') return row.task.status === 'failed' || row.qualityLabel === '生成失败'
  return true
}

function creativePriority(task: BusinessTaskRecord, quality: ReturnType<typeof creativeQualityState>, confirmation: ApiMaterialConfirmation | undefined): CreativeOpsRow['priority'] {
  if (task.status === 'failed' || quality.failed || confirmation?.status === 'changes_requested') return '高'
  if (task.status === 'ready' || quality.passed) return '中'
  return '低'
}

function creativeDueLabel(task: BusinessTaskRecord, priority: CreativeOpsRow['priority']): string {
  if (task.status === 'completed') return '已完成'
  if (priority === '高') return '今日 18:00'
  if (priority === '中') return '明日 12:00'
  return '本周五 18:00'
}

function channelSpecLabel(type: BusinessTaskType): string {
  const labels: Record<BusinessTaskType, string> = {
    strategy: '策略输出',
    creative: '巨量/腾讯 · 图文 3:4',
    video: '全渠道 · 视频 9:16',
    brand_video: '品牌广告 · 16:9 / 9:16',
    short_drama_preroll: '短剧前贴 · 9:16 ≤6s',
    game_preroll: '游戏前贴 · 9:16 ≤6s',
    commerce_preroll: '电商前贴 · 9:16 ≤6s',
    viral_remake: '爆款复刻 · 多规格',
    video_edit: '素材剪辑 · 9:16 / 1:1',
  }
  return labels[type]
}

function materialTitle(assetId: string) {
  return assetId.replace(/^asset-/, '').split('-').map(part => part.charAt(0).toUpperCase() + part.slice(1)).join(' ')
}

function formatTaskDate(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit' })
}

function priorityTone(priority: CreativeOpsRow['priority']): StatusTone {
  return priority === '高' ? 'danger' : priority === '中' ? 'warning' : 'info'
}

function StatusPill({ label, tone }: { label: string; tone: StatusTone }) {
  return <span className={`status ${tone}`}><span />{label}</span>
}
