import { useCallback, useEffect, useMemo, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { listProjectAssets, removeProjectAsset } from '../assets/api'
import { RemoveAssetDialog } from '../assets/RemoveAssetDialog'
import type { ProjectAsset } from '../assets/types'
import { getProviderJob } from '../platform/api'
import type { ProviderJob } from '../platform/types'
import { approveCreativeVersion, archiveCreativeTask, bindCreativeImageAsset, checkCreativeVersion, createImagePlanJob, createCreativeIntake, createCreativeTask, deliverCreativeVersion, freezeCreativeVersion, getCreativeTask, listCreativeIntakes, listCreativeTasks, reviseCreativeDraft } from './api'
import type { CreateCreativeTaskInput, CreativeContentType, CreativeIntake, CreativeIntakeInput, CreativePackage, CreativeTask, CreativeTaskDetail, CreativeVersion, ImageTextDraft, ReviseDraftInput } from './types'

const emptyInput: CreativeIntakeInput = {
  source: 'manual', channel: 'xiaohongshu', objective: '', audience: '', core_message: '', call_to_action: '', concept: '', tone: [], visual_keywords: [], mandatory_elements: [], prohibited_claims: [],
}

const terminalProviderStatuses = new Set<ProviderJob['provider_status']>(['succeeded', 'partially_succeeded', 'failed', 'cancelled', 'expired'])
const taskTypeLabels: Record<CreativeContentType, string> = { lifestyle: '生活方式种草', ingredient_explanation: '成分解释', usage_scenario: '使用场景', list_guide: '清单攻略', comparison: '对比选择', custom: '自定义方向' }

const providerStatusLabels: Record<ProviderJob['provider_status'], string> = {
  submitted: '已提交', running: '模型生成中', outputs_ready: '产物已就绪', ingesting: '素材入库中',
  succeeded: '已完成', partially_succeeded: '部分完成', failed: '失败', cancelled: '已取消', expired: '已过期',
}

function values(value: string) {
  return value.split(/[，,]/).map((item) => item.trim()).filter(Boolean)
}

function DraftEditor({ draft, busy, onCancel, onSave }: { draft: ImageTextDraft, busy: boolean, onCancel: () => void, onSave: (input: ReviseDraftInput) => void }) {
  const [input, setInput] = useState<ReviseDraftInput>({
    expected_version: draft.version, title_candidates: [...draft.title_candidates], body: draft.body, topics: [...draft.topics], cover_copy: draft.cover_copy,
    image_plan: draft.image_plan.map((item) => ({ ...item })),
  })
  const updatePlan = (index: number, visualBrief: string) => setInput((current) => ({ ...current, image_plan: current.image_plan.map((item, itemIndex) => itemIndex === index ? { ...item, visual_brief: visualBrief } : item) }))
  return <form className="creative-editor" onSubmit={(event) => { event.preventDefault(); onSave(input) }}>
    <div className="creative-editor__heading"><div><span>可编辑草稿</span><h3>修改会创建 v{draft.version + 1}</h3></div><small>旧版不会被覆盖；已冻结版本保持不变。</small></div>
    <label>标题候选（每行一个）<textarea value={input.title_candidates.join('\n')} onChange={(event) => setInput((current) => ({ ...current, title_candidates: event.target.value.split('\n').map((item) => item.trim()).filter(Boolean) }))} /></label>
    <label>正文<textarea value={input.body} onChange={(event) => setInput((current) => ({ ...current, body: event.target.value }))} /></label>
    <div className="creative-form-grid"><label>话题（逗号分隔）<input value={input.topics.join('，')} onChange={(event) => setInput((current) => ({ ...current, topics: values(event.target.value) }))} /></label><label>封面文字<input value={input.cover_copy} onChange={(event) => setInput((current) => ({ ...current, cover_copy: event.target.value }))} /></label></div>
    <fieldset><legend>图组画面说明</legend>{input.image_plan.map((item, index) => <label key={item.order}>{item.order}. {item.purpose}<input value={item.visual_brief} onChange={(event) => updatePlan(index, event.target.value)} /></label>)}</fieldset>
    <div className="creative-editor__actions"><button className="button button--secondary" disabled={busy} onClick={onCancel} type="button">取消编辑</button><button className="button button--primary" disabled={busy} type="submit">{busy ? '正在保存…' : '保存为下一版草稿'}</button></div>
  </form>
}

function TaskDirectionForm({ intake, busy, onCancel, onCreate }: { intake: CreativeIntake, busy: boolean, onCancel: () => void, onCreate: (input: CreateCreativeTaskInput) => void }) {
  const [contentType, setContentType] = useState<CreativeContentType>('lifestyle')
  const [focus, setFocus] = useState('')
  return <form className="creative-task-brief" onSubmit={(event) => { event.preventDefault(); onCreate({ content_type: contentType, focus }) }}>
    <div><span>02 · 二次创作意图</span><h2>这篇图文要怎么讲？</h2><p>同一策略可以创建多篇任务；每篇都必须选择不同的内容类型和表达角度。</p></div>
    <label>图文类型<select value={contentType} onChange={(event) => setContentType(event.target.value as CreativeContentType)}>{Object.entries(taskTypeLabels).map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select></label>
    <label>本次创作角度<input autoFocus onChange={(event) => setFocus(event.target.value)} placeholder="例如：通勤早晨如何把产品融入日常" required value={focus} /></label>
    <small>策略目标：{intake.request.objective} · 目标人群：{intake.request.audience}</small>
    <div className="creative-editor__actions"><button className="button button--secondary" disabled={busy} onClick={onCancel} type="button">暂不创建</button><button className="button button--primary" disabled={busy} type="submit">{busy ? '正在创建…' : '创建这篇图文任务'}</button></div>
  </form>
}

type StrategyIntakeGroup = {
  key: string
  intake: CreativeIntake
  duplicateCount: number
  tasks: CreativeTask[]
}

function StrategyPackageHandoffCard({ group, busy, onCreate, onOpen }: {
  group: StrategyIntakeGroup
  busy: boolean
  onCreate: () => void
  onOpen: () => void
}) {
  const { intake, duplicateCount, tasks } = group
  const packageRef = intake.request.strategy_package
  const packageLabel = packageRef ? `策略包版本 v${packageRef.package_version}` : '策略包版本未知'
  const task = tasks[0]
  return <article className="creative-intake-item creative-intake-item--strategy">
    <div>
      <span className="creative-intake-item__eyebrow">策略输入</span>
      <strong>{packageLabel}</strong>
      <small>{intake.request.objective || '未填写传播目标'} · {intake.request.audience || '未填写目标人群'}</small>
      <small>{packageRef ? `来源 ${packageRef.package_id}` : '已冻结策略上下文'}{duplicateCount > 1 ? ` · 检测到 ${duplicateCount} 次重复交接，已合并显示` : ''}</small>
    </div>
    {task
      ? <button className="button button--secondary" onClick={onOpen} type="button">打开 {tasks.length} 个图文任务</button>
      : intake.status === 'ready' ? <button className="button button--primary" disabled={busy} onClick={onCreate} type="button">基于此策略创建图文任务</button> : null}
  </article>
}

export function CreativeImageTextPage() {
  const { projectId = '' } = useParams()
  const [input, setInput] = useState<CreativeIntakeInput>(emptyInput)
  const [intakes, setIntakes] = useState<CreativeIntake[]>([])
  const [tasks, setTasks] = useState<CreativeTask[]>([])
  const [selected, setSelected] = useState<CreativeTaskDetail | null>(null)
  const [loading, setLoading] = useState(true)
  const [submitting, setSubmitting] = useState(false)
  const [generating, setGenerating] = useState(false)
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  const [latestJob, setLatestJob] = useState<ProviderJob | null>(null)
  const [providerJobs, setProviderJobs] = useState<Record<string, ProviderJob>>({})
  const [productionAssets, setProductionAssets] = useState<ProjectAsset[]>([])
  const [manualOpen, setManualOpen] = useState(false)
  const [assetToRemove, setAssetToRemove] = useState<ProjectAsset | null>(null)
  const [removingAsset, setRemovingAsset] = useState(false)
  const [removeError, setRemoveError] = useState('')

  const [editingDraft, setEditingDraft] = useState(false)
  const [savingDraft, setSavingDraft] = useState(false)
  const [freezingVersion, setFreezingVersion] = useState(false)
  const [frozenVersion, setFrozenVersion] = useState<CreativeVersion | null>(null)
  const [transitioningVersion, setTransitioningVersion] = useState(false)
  const [creativePackage, setCreativePackage] = useState<CreativePackage | null>(null)
  const [intakeForTask, setIntakeForTask] = useState<CreativeIntake | null>(null)
  const [archivePending, setArchivePending] = useState(false)
  const [archiveConfirm, setArchiveConfirm] = useState(false)

  const selectTask = useCallback(async (taskId: string, signal?: AbortSignal) => {
    const detail = await getCreativeTask(projectId, taskId, signal)
    setSelected(detail)
  }, [projectId])

  const load = useCallback(async (signal?: AbortSignal) => {
    setError('')
    try {
      const [taskResponse, intakeResponse] = await Promise.all([listCreativeTasks(projectId, signal), listCreativeIntakes(projectId, signal)])
      setTasks(taskResponse.items)
      setIntakes(intakeResponse.items)
      if (taskResponse.items.length > 0) await selectTask(taskResponse.items[0].id, signal)
      else setSelected(null)
    } catch (caught) {
      if (caught instanceof DOMException && caught.name === 'AbortError') return
      setError(caught instanceof Error ? caught.message : '无法加载创意工作区。')
    } finally {
      if (!signal?.aborted) setLoading(false)
    }
  }, [projectId, selectTask])

  useEffect(() => {
    const controller = new AbortController()
    const timer = window.setTimeout(() => void load(controller.signal), 0)
    return () => { window.clearTimeout(timer); controller.abort() }
  }, [load])

  const strategyIntakes = useMemo(() => intakes.filter((intake) => intake.source === 'strategy_package'), [intakes])
  const strategyIntakeGroups = useMemo(() => {
    const groups = new Map<string, StrategyIntakeGroup>()
	const groupKeyByIntakeID = new Map<string, string>()
    for (const intake of strategyIntakes) {
      const reference = intake.request.strategy_package
      const key = reference ? `${reference.package_id}:${reference.package_version}:${reference.expected_content_hash}` : intake.id
		groupKeyByIntakeID.set(intake.id, key)
      const existing = groups.get(key)
      if (existing) {
        existing.duplicateCount += 1
        continue
      }
      groups.set(key, { key, intake, duplicateCount: 1, tasks: [] })
    }
    for (const task of tasks) {
		const group = groups.get(groupKeyByIntakeID.get(task.intake_id) ?? '')
		if (group) group.tasks.push(task)
    }
    return [...groups.values()]
  }, [strategyIntakes, tasks])
  const intakesByID = useMemo(() => new Map(intakes.map((intake) => [intake.id, intake])), [intakes])
  const productionJobKey = (selected?.production_jobs.map((job) => job.provider_job_id) ?? []).join(',')

  useEffect(() => {
    const jobIds = productionJobKey ? productionJobKey.split(',') : []
    if (jobIds.length === 0) return
    const controller = new AbortController()
    const refreshProduction = async () => {
      try {
        const [jobs, assets] = await Promise.all([
          Promise.all(jobIds.map((jobId) => getProviderJob(projectId, jobId, controller.signal))),
          listProjectAssets(projectId, controller.signal),
        ])
        if (controller.signal.aborted) return
        setProviderJobs(Object.fromEntries(jobs.map((job) => [job.id, job])))
        setProductionAssets(assets.items.filter((asset) => jobIds.includes(asset.version.provider_job_id ?? '')))
      } catch (caught) {
        if (caught instanceof DOMException && caught.name === 'AbortError') return
        if (!controller.signal.aborted) setError(caught instanceof Error ? caught.message : '无法读取生产任务状态。')
      }
    }
    void refreshProduction()
    const timer = window.setInterval(() => void refreshProduction(), 2000)
    return () => { window.clearInterval(timer); controller.abort() }
  }, [projectId, productionJobKey])

  function change<K extends keyof CreativeIntakeInput>(key: K, value: CreativeIntakeInput[K]) {
    setInput((current) => ({ ...current, [key]: value }))
  }

  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (submitting) return
    setSubmitting(true); setError(''); setMessage('')
    try {
      const intake = await createCreativeIntake(projectId, input)
      setIntakes((current) => [intake, ...current])
      if (intake.status === 'needs_clarification') {
        setMessage(`输入已保存，还需要补充：${intake.missing_fields.join('、')}。`)
        return
      }
      setIntakeForTask(intake)
      setMessage('创意输入已保存。请填写本次图文的类型与创作角度，再创建任务。')
      setInput(emptyInput)
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : '无法创建创意任务。')
    } finally { setSubmitting(false) }
  }

  async function createTaskFromIntake(intake: CreativeIntake, direction: CreateCreativeTaskInput) {
    if (submitting) return
    setSubmitting(true); setError(''); setMessage('')
    try {
      const task = await createCreativeTask(projectId, intake.id, direction)
      setTasks((current) => [task, ...current])
      await selectTask(task.id)
      setIntakeForTask(null)
      setMessage(`已创建“${taskTypeLabels[direction.content_type]}”图文任务，并生成可编辑初稿。`)
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : '无法从创意输入创建任务。')
    } finally { setSubmitting(false) }
  }

  async function generateImage(imagePlanOrder: number) {
    if (!selected || generating) return
    setGenerating(true); setError(''); setMessage('')
    try {
      const job = await createImagePlanJob(projectId, selected.task.id, imagePlanOrder)
      setLatestJob(job)
      setProviderJobs((current) => ({ ...current, [job.id]: job }))
      await selectTask(selected.task.id)
      setMessage('封面生图任务已提交；Provider 产物校验并入库后会出现在下方链路与项目素材库中。')
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : '无法创建封面生成任务。')
    } finally { setGenerating(false) }
  }

  async function bindImageAsset(imagePlanOrder: number, asset: ProjectAsset) {
    if (!selected || savingDraft) return
    setSavingDraft(true); setError(''); setMessage('')
    try {
      await bindCreativeImageAsset(projectId, selected.task.id, selected.draft.version, imagePlanOrder, asset.asset.id, asset.version.version)
      await selectTask(selected.task.id)
      setMessage(`素材已绑定到第 ${imagePlanOrder} 张图，并创建新的草稿版本。`)
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : '素材绑定失败。')
    } finally { setSavingDraft(false) }
  }

  async function saveDraft(input: ReviseDraftInput) {
    if (!selected || savingDraft) return
    setSavingDraft(true); setError(''); setMessage('')
    try {
      await reviseCreativeDraft(projectId, selected.task.id, input)
      await selectTask(selected.task.id)
      setEditingDraft(false)
      setMessage(`草稿已保存为 v${input.expected_version + 1}；此前冻结的创意版本不会被改写。`)
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : '无法保存创意草稿。')
    } finally { setSavingDraft(false) }
  }

  async function freezeCurrentDraft() {
    if (!selected || freezingVersion) return
    setFreezingVersion(true); setError(''); setMessage('')
    try {
      const version = await freezeCreativeVersion(projectId, selected.task.id, selected.draft.version)
      setFrozenVersion(version)
      setMessage(`已冻结创意版本 v${version.version}。接下来将进入检查与评审，不会再引用可编辑草稿。`)
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : '无法冻结创意版本。')
    } finally { setFreezingVersion(false) }
  }

  async function transitionVersion(action: 'check' | 'approve' | 'deliver') {
    if (!frozenVersion || transitioningVersion) return
    setTransitioningVersion(true); setError(''); setMessage('')
    try {
      if (action === 'check') {
        const version = await checkCreativeVersion(projectId, frozenVersion.id)
        setFrozenVersion(version)
        setMessage(version.check?.passed ? '版本检查通过，可以批准。' : `版本检查发现 ${version.check?.blockers.length ?? 0} 个阻塞项。`)
      } else if (action === 'approve') {
        const version = await approveCreativeVersion(projectId, frozenVersion.id)
        setFrozenVersion(version)
        setMessage('版本已批准，并写入 creative.approved.v1 事件。')
      } else {
        const value = await deliverCreativeVersion(projectId, frozenVersion.id)
        setCreativePackage(value)
        setMessage('交付包已创建，并写入 creative.delivered.v1 事件。')
      }
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : 'Creative 版本流转失败。')
    } finally { setTransitioningVersion(false) }
  }

  async function archiveSelectedTask() {
    if (!selected || archivePending) return
    setArchivePending(true); setError(''); setMessage('')
    try {
      await archiveCreativeTask(projectId, selected.task.id)
      const remaining = tasks.filter((task) => task.id !== selected.task.id)
      setTasks(remaining)
      setArchiveConfirm(false)
      setFrozenVersion(null)
      if (remaining.length > 0) await selectTask(remaining[0].id)
      else setSelected(null)
      setMessage('图文任务已归档，不再出现在当前工作队列；草稿、创意版本、模型作业和已入库素材仍保留，可用于追溯。')
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : '图文任务归档失败，请稍后重试。')
    } finally { setArchivePending(false) }
  }

  async function removeCreativeAsset() {
    if (!assetToRemove || removingAsset) return
    setRemovingAsset(true)
    setRemoveError('')
    try {
      await removeProjectAsset(projectId, assetToRemove.asset.id, assetToRemove.version.version)
      setProductionAssets((current) => current.filter((asset) => asset.asset.id !== assetToRemove.asset.id || asset.version.version !== assetToRemove.version.version))
      setAssetToRemove(null)
      setMessage('素材已从当前项目中移除；原始文件、版本与 Provider 溯源记录仍会保留。')
    } catch (caught) {
      setRemoveError(caught instanceof Error ? caught.message : '素材删除失败，请稍后重试。')
    } finally {
      setRemovingAsset(false)
    }
  }

  const showManualForm = strategyIntakes.length === 0 || manualOpen
  const currentProductionAssets = productionAssets.filter((asset) => productionJobKey.split(',').includes(asset.version.provider_job_id ?? ''))

  return <section className="creative-page">
    <header className="page-header creative-page__header">
      <div><span className="creative-kicker">Creative / Image & Text</span><h1>小红书图文创作</h1><p>策略交接、创意初稿、Provider 生成和项目素材在同一条可追溯链路中完成。</p></div>
      <Link className="button button--secondary" to={`/projects/${encodeURIComponent(projectId)}/assets`}>查看项目素材</Link>
    </header>

    <ol className="creative-journey" aria-label="创意生产链路">
      <li className={strategyIntakes.length > 0 ? 'creative-journey__done' : ''}><b>1</b><span>策略包</span><small>{strategyIntakes.length > 0 ? '已交接' : '可选'}</small></li>
      <li className={tasks.length > 0 ? 'creative-journey__done' : ''}><b>2</b><span>图文任务</span><small>{tasks.length > 0 ? `${tasks.length} 项` : '待创建'}</small></li>
      <li className={selected?.production_jobs.length ? 'creative-journey__done' : ''}><b>3</b><span>Provider 生图</span><small>{selected?.production_jobs.length ? '已提交' : '待提交'}</small></li>
      <li className={currentProductionAssets.length ? 'creative-journey__done' : ''}><b>4</b><span>项目素材</span><small>{currentProductionAssets.length ? `${currentProductionAssets.length} 个已入库` : '待入库'}</small></li>
    </ol>

    <div className="creative-workspace">
      {showManualForm ? <form className="creative-intake" onSubmit={(event) => void submit(event)}>
        <div className="creative-panel__heading"><div><span>手工入口</span><h2>新建独立创意</h2></div>{strategyIntakes.length > 0 ? <button className="text-button" onClick={() => setManualOpen(false)} type="button">收起</button> : <small>小红书图文</small>}</div>
        <label>传播目标<input value={input.objective} onChange={(event) => change('objective', event.target.value)} placeholder="例如：建立新品认知" /></label>
        <label>目标人群<input value={input.audience} onChange={(event) => change('audience', event.target.value)} placeholder="例如：关注生活方式的年轻上班族" /></label>
        <label>核心信息<textarea value={input.core_message} onChange={(event) => change('core_message', event.target.value)} placeholder="希望用户记住的一句话" /></label>
        <div className="creative-form-grid"><label>创意概念<input value={input.concept} onChange={(event) => change('concept', event.target.value)} placeholder="例如：晨光中的蓝白咖啡桌" /></label><label>行动引导<input value={input.call_to_action} onChange={(event) => change('call_to_action', event.target.value)} placeholder="例如：收藏这份晨间灵感" /></label></div>
        <div className="creative-form-grid"><label>语气（逗号分隔）<input value={input.tone.join('，')} onChange={(event) => change('tone', values(event.target.value))} placeholder="自然，柔和，可信" /></label><label>视觉关键词（逗号分隔）<input value={input.visual_keywords.join('，')} onChange={(event) => change('visual_keywords', values(event.target.value))} placeholder="蓝白，晨光，留白" /></label></div>
        <button className="button button--primary" disabled={submitting} type="submit">{submitting ? '正在建立任务…' : '保存输入并创建图文任务'}</button>
        <p className="creative-intake__hint">缺少传播目标、目标人群或核心信息时，系统只会保存为“等待补充”，不会创建半成品任务。</p>
      </form> : <section className="creative-manual-entry"><span>手工入口</span><h2>需要一条独立创意吗？</h2><p>当前已有来自策略的输入。只有不基于该策略时，才新建独立创意。</p><button className="button button--secondary" onClick={() => setManualOpen(true)} type="button">新建手工创意</button></section>}

      <aside className="creative-task-list" aria-label="策略接入与创意任务">
        <div className="creative-panel__heading"><div><span>策略交接与创意任务</span><h2>下一步</h2></div><small>{strategyIntakeGroups.length} 个策略包 · {tasks.length} 项图文任务</small></div>
        <section className="creative-handoff-section" aria-label="策略输入">
          <div className="creative-handoff-section__heading"><strong>策略输入</strong><small>已批准策略的只读交接，不是创意任务。</small></div>
          {strategyIntakeGroups.length > 0 ? <div className="creative-intake-items">{strategyIntakeGroups.map((group) => <StrategyPackageHandoffCard busy={submitting} group={group} key={group.key} onCreate={() => setIntakeForTask(group.intake)} onOpen={() => { if (group.tasks[0]) void selectTask(group.tasks[0].id) }} />)}</div> : <p className="creative-empty">还没有策略交接。你可以从策略工作区发布策略包后创建创意输入，或直接使用左侧手工入口。</p>}
        </section>
        <section className="creative-handoff-section" aria-label="图文任务">
          <div className="creative-handoff-section__heading"><strong>图文任务</strong><small>Creative 自己拥有的可编辑生产任务。</small></div>
          {loading ? <p className="creative-empty">正在加载任务…</p> : tasks.length === 0 ? <p className="creative-empty">尚未创建图文任务。完成策略交接或手工输入后，初稿会显示在这里。</p> : <div className="creative-task-items">{tasks.map((task) => {
            const source = intakesByID.get(task.intake_id)
            const reference = source?.request.strategy_package
            return <button className={selected?.task.id === task.id ? 'creative-task-item creative-task-item--selected' : 'creative-task-item'} key={task.id} onClick={() => void selectTask(task.id)} type="button"><span>{task.direction.focus || task.direction.concept || source?.request.core_message || '未命名方向'}</span><small>{taskTypeLabels[task.direction.content_type] ?? '图文方向'} · {task.channel === 'xiaohongshu' ? '小红书图文' : task.channel} · {task.status === 'draft' ? '初稿' : task.status}</small><em>{reference ? `来自策略包 v${reference.package_version}` : '手工创意输入'}</em></button>
          })}</div>}
        </section>
      </aside>
    </div>
    {intakeForTask ? <TaskDirectionForm busy={submitting} intake={intakeForTask} onCancel={() => setIntakeForTask(null)} onCreate={(direction) => void createTaskFromIntake(intakeForTask, direction)} /> : null}

    {selected ? <section className="creative-draft" aria-label="图文内容初稿">
      <div className="creative-draft__top"><div><span>{taskTypeLabels[selected.task.direction.content_type] ?? '图文创作'} · 内容初稿 v{selected.draft.version}</span><h2>{selected.task.direction.focus || selected.task.direction.concept || '小红书图文方向'}</h2><p>{selected.intake.request.objective} · 面向 {selected.task.direction.audience || selected.intake.request.audience}</p></div><div className="creative-draft__actions"><button className="button button--secondary" disabled={savingDraft || freezingVersion} onClick={() => setEditingDraft(true)} type="button">编辑草稿</button><button className="button button--secondary" disabled={generating} onClick={() => void generateImage(1)} type="button">{generating ? '正在提交图片…' : '生成第 1 张图'}</button><button className="button button--primary" disabled={freezingVersion || editingDraft} onClick={() => void freezeCurrentDraft()} type="button">{freezingVersion ? '正在冻结…' : '冻结为创意版本'}</button><button className="text-button text-button--danger" disabled={archivePending} onClick={() => setArchiveConfirm(true)} type="button">归档图文任务</button></div></div>
      <div className="creative-lineage" aria-label="当前任务链路">
        <div><span>创意输入</span><strong>{selected.intake.source === 'strategy_package' ? '来自策略包' : '手工输入'}</strong><small>{selected.intake.request.strategy_package ? `${selected.intake.request.strategy_package.package_id} · v${selected.intake.request.strategy_package.package_version}` : `Intake · ${selected.intake.id}`}</small></div>
        <div><span>图文任务</span><strong>当前初稿</strong><small>{selected.task.id}</small></div>
        <div><span>Provider 作业</span><strong>{selected.production_jobs.length ? `${selected.production_jobs.length} 个生产任务` : '尚未提交'}</strong><small>{selected.production_jobs.length ? '可查看执行和入库状态' : '点击“生成封面图片”开始'}</small></div>
        <div><span>项目素材</span><strong>{currentProductionAssets.length ? `${currentProductionAssets.length} 个已入库` : '等待生成结果'}</strong><small>素材由 Provider 校验入库</small></div>
      </div>
      <div className="creative-draft__grid"><article className="creative-copy"><h3>标题候选</h3><ol>{selected.draft.title_candidates.map((title) => <li key={title}>{title}</li>)}</ol><h3>正文</h3><p>{selected.draft.body}</p><div className="creative-topics">{selected.draft.topics.map((topic) => <span key={topic}>{topic}</span>)}</div></article><article className="creative-cover"><div className="creative-cover__canvas"><span>封面文字</span><strong>{selected.draft.cover_copy}</strong><small>{selected.task.direction.tone.join(' · ') || '小红书图文'}</small></div><h3>图组结构</h3><ol className="creative-image-plan">{selected.draft.image_plan.map((item) => {
        const production = [...selected.production_jobs].reverse().find((job) => job.kind.startsWith(`image_plan_${item.order}_job_`) || (item.order === 1 && job.kind === 'cover_image'))
        const generatedAssets = production ? currentProductionAssets.filter((asset) => asset.version.provider_job_id === production.provider_job_id) : []
        return <li key={item.order}><b>{item.order}</b><div><strong>{item.purpose}</strong><span>{item.visual_brief}</span><small>{item.asset_ref ? `已绑定素材 ${item.asset_ref.asset_id} · v${item.asset_ref.version}` : '尚未绑定项目素材'}</small><div className="creative-image-plan__actions"><button className="text-button" disabled={generating} onClick={() => void generateImage(item.order)} type="button">生成此图</button>{generatedAssets.map((asset) => <button className="text-button" disabled={savingDraft} key={`${asset.asset.id}:${asset.version.version}`} onClick={() => void bindImageAsset(item.order, asset)} type="button">绑定已入库素材</button>)}</div></div></li>
      })}</ol></article></div>
      {editingDraft ? <DraftEditor key={`${selected.task.id}:${selected.draft.version}`} busy={savingDraft} draft={selected.draft} onCancel={() => setEditingDraft(false)} onSave={(input) => void saveDraft(input)} /> : null}
      {archiveConfirm ? <section className="creative-archive-confirm" aria-label="归档图文任务确认"><div><strong>归档这篇图文任务？</strong><p>{selected.production_jobs.length > 0 ? '任务已关联模型作业。归档只会从当前工作队列移除它，不会删除草稿、创意版本、模型作业或项目素材。' : '任务会从当前工作队列移除；已有草稿仍会保留，方便后续追溯。'}</p></div><div><button className="button button--secondary" disabled={archivePending} onClick={() => setArchiveConfirm(false)} type="button">取消</button><button className="button button--danger" disabled={archivePending} onClick={() => void archiveSelectedTask()} type="button">{archivePending ? '正在归档…' : '确认归档'}</button></div></section> : null}
      {frozenVersion?.creative_task_id === selected.task.id ? <div className="creative-version-note"><strong>已冻结 CreativeVersion v{frozenVersion.version}</strong><code>{frozenVersion.id}</code><span>内容哈希 {frozenVersion.content_hash}</span><div className="creative-version-note__actions"><button className="text-button" disabled={transitioningVersion || frozenVersion.status === 'approved'} onClick={() => void transitionVersion('check')} type="button">检查版本</button><button className="text-button" disabled={transitioningVersion || frozenVersion.status !== 'checked' || !frozenVersion.check?.passed} onClick={() => void transitionVersion('approve')} type="button">批准版本</button><button className="text-button" disabled={transitioningVersion || frozenVersion.status !== 'approved'} onClick={() => void transitionVersion('deliver')} type="button">创建交付包</button></div>{frozenVersion.check ? <small>{frozenVersion.check.passed ? '检查通过' : `检查阻塞：${frozenVersion.check.blockers.join('；')}`}</small> : null}</div> : null}
      {creativePackage && frozenVersion && creativePackage.creative_version_id === frozenVersion.id ? <div className="success-note creative-message"><span>✓</span><p>CreativePackage：<code>{creativePackage.id}</code></p></div> : null}
      {selected.production_jobs.length > 0 ? <section className="creative-production"><span>封面生产状态</span>{selected.production_jobs.map((production) => {
        const job = providerJobs[production.provider_job_id]
        const jobAssets = currentProductionAssets.filter((asset) => asset.version.provider_job_id === production.provider_job_id)
        const assetCount = jobAssets.length
        return <section className="creative-production__job" key={production.provider_job_id}><article><div><strong>{job ? providerStatusLabels[job.provider_status] : '正在读取状态'}</strong><code>{production.provider_job_id}</code><small>{job && !terminalProviderStatuses.has(job.provider_status) ? `进度 ${job.progress}% · 正在自动刷新` : assetCount ? `${assetCount} 个素材已入库` : '尚未入库'}</small></div><div className="creative-production__actions"><Link className="text-button" to={`/projects/${encodeURIComponent(projectId)}/assets?provider_job_id=${encodeURIComponent(production.provider_job_id)}`}>查看项目素材</Link></div></article><details className="creative-provider-details"><summary>模型作业详情（排障用）</summary><p>这里用于核对模型调用、失败原因和入库结果，不是创作主流程。</p><code>{production.provider_job_id}</code>{job?.error ? <small>{job.error.code} · {job.error.message}</small> : null}</details>{jobAssets.length > 0 ? <div className="creative-production__assets" aria-label="已入库创意素材">{jobAssets.map((asset) => <div key={`${asset.asset.id}:${asset.version.version}`}><div><strong>已入库素材</strong><small>{asset.asset.id} · v{asset.version.version} · {asset.version.mime_type}</small></div><button className="text-button text-button--danger" onClick={() => { setRemoveError(''); setAssetToRemove(asset) }} type="button">从项目中删除</button></div>)}</div> : null}</section>
      })}</section> : null}
    </section> : null}
    {latestJob ? <div className="success-note creative-message"><span>✓</span><p>Provider Job：<code>{latestJob.id}</code>，当前状态：{providerStatusLabels[latestJob.provider_status]}。</p></div> : null}
    {message ? <div className="success-note creative-message"><span>✓</span><p>{message}</p></div> : null}
    {error ? <div className="library-error" role="alert"><div><strong>创意操作失败</strong><span>{error}</span></div><button className="text-button" onClick={() => setError('')} type="button">关闭</button></div> : null}
    <RemoveAssetDialog asset={assetToRemove} busy={removingAsset} error={removeError} onClose={() => { if (!removingAsset) { setAssetToRemove(null); setRemoveError('') } }} onConfirm={() => void removeCreativeAsset()} />
  </section>
}
