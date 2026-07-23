import { useCallback, useEffect, useMemo, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { listProjectAssets } from '../assets/api'
import type { ProjectAsset } from '../assets/types'
import { getProviderJob } from '../platform/api'
import type { ProviderJob } from '../platform/types'
import { createCoverImageJob, createCreativeIntake, createCreativeTask, getCreativeTask, listCreativeIntakes, listCreativeTasks } from './api'
import type { CreativeIntake, CreativeIntakeInput, CreativeTask, CreativeTaskDetail } from './types'

const emptyInput: CreativeIntakeInput = {
  source: 'manual', channel: 'xiaohongshu', objective: '', audience: '', core_message: '', call_to_action: '', concept: '', tone: [], visual_keywords: [], mandatory_elements: [], prohibited_claims: [],
}

const terminalProviderStatuses = new Set<ProviderJob['provider_status']>(['succeeded', 'partially_succeeded', 'failed', 'cancelled', 'expired'])

const providerStatusLabels: Record<ProviderJob['provider_status'], string> = {
  submitted: '已提交', running: '模型生成中', outputs_ready: '产物已就绪', ingesting: '素材入库中',
  succeeded: '已完成', partially_succeeded: '部分完成', failed: '失败', cancelled: '已取消', expired: '已过期',
}

function values(value: string) {
  return value.split(/[，,]/).map((item) => item.trim()).filter(Boolean)
}

function IntakeHandoffCard({ intake, task, busy, onCreate, onOpen }: {
  intake: CreativeIntake
  task?: CreativeTask
  busy: boolean
  onCreate: () => void
  onOpen: () => void
}) {
  const packageRef = intake.request.strategy_package
  return <article className="creative-intake-item">
    <div>
      <strong>策略包已接入</strong>
      <small>{packageRef ? `策略包 ${packageRef.package_id} · v${packageRef.package_version}` : '已冻结策略上下文'}</small>
      <small>{task ? '图文任务已创建，可直接继续生产。' : intake.status === 'ready' ? '已就绪，下一步创建图文任务。' : `待补充：${intake.missing_fields.join('、')}`}</small>
    </div>
    {task
      ? <button className="button button--secondary" onClick={onOpen} type="button">打开图文任务</button>
      : intake.status === 'ready' ? <button className="button button--primary" disabled={busy} onClick={onCreate} type="button">创建图文任务</button> : null}
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

  const tasksByIntake = useMemo(() => new Map(tasks.map((task) => [task.intake_id, task])), [tasks])
  const strategyIntakes = useMemo(() => intakes.filter((intake) => intake.source === 'strategy_package'), [intakes])
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
      const task = await createCreativeTask(projectId, intake.id)
      setTasks((current) => [task, ...current])
      await selectTask(task.id)
      setMessage('已创建图文任务，并生成可编辑的图文初稿。')
      setInput(emptyInput)
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : '无法创建创意任务。')
    } finally { setSubmitting(false) }
  }

  async function createTaskFromIntake(intake: CreativeIntake) {
    if (submitting) return
    setSubmitting(true); setError(''); setMessage('')
    try {
      const task = await createCreativeTask(projectId, intake.id)
      setTasks((current) => current.some((item) => item.id === task.id) ? current : [task, ...current])
      await selectTask(task.id)
      setMessage('已从策略交接的创意输入创建图文任务，并生成可编辑初稿。')
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : '无法从创意输入创建任务。')
    } finally { setSubmitting(false) }
  }

  async function generateCover() {
    if (!selected || generating) return
    setGenerating(true); setError(''); setMessage('')
    try {
      const job = await createCoverImageJob(projectId, selected.task.id)
      setLatestJob(job)
      setProviderJobs((current) => ({ ...current, [job.id]: job }))
      await selectTask(selected.task.id)
      setMessage('封面生图任务已提交；Provider 产物校验并入库后会出现在下方链路与项目素材库中。')
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : '无法创建封面生成任务。')
    } finally { setGenerating(false) }
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
        <div className="creative-panel__heading"><div><span>策略接入与任务</span><h2>下一步</h2></div><small>{strategyIntakes.length} 个策略输入 · {tasks.length} 项任务</small></div>
        {strategyIntakes.length > 0 ? <div className="creative-intake-items">{strategyIntakes.map((intake) => <IntakeHandoffCard busy={submitting} intake={intake} key={intake.id} onCreate={() => void createTaskFromIntake(intake)} onOpen={() => { const task = tasksByIntake.get(intake.id); if (task) void selectTask(task.id) }} task={tasksByIntake.get(intake.id)} />)}</div> : <p className="creative-empty">还没有策略交接。你可以从策略工作区发布策略包后创建创意输入，或直接使用左侧手工入口。</p>}
        {loading ? <p className="creative-empty">正在加载任务…</p> : tasks.length === 0 ? <p className="creative-empty">尚未创建图文任务。完成策略交接或手工输入后，初稿会显示在这里。</p> : <div className="creative-task-items">{tasks.map((task) => <button className={selected?.task.id === task.id ? 'creative-task-item creative-task-item--selected' : 'creative-task-item'} key={task.id} onClick={() => void selectTask(task.id)} type="button"><span>{task.direction.concept || '未命名方向'}</span><small>{task.channel === 'xiaohongshu' ? '小红书图文' : task.channel} · {task.status === 'draft' ? '初稿' : task.status}</small></button>)}</div>}
      </aside>
    </div>

    {selected ? <section className="creative-draft" aria-label="图文内容初稿">
      <div className="creative-draft__top"><div><span>内容初稿 v{selected.draft.version}</span><h2>{selected.task.direction.concept || '小红书图文方向'}</h2><p>{selected.intake.request.objective} · 面向 {selected.intake.request.audience}</p></div><button className="button button--primary" disabled={generating} onClick={() => void generateCover()} type="button">{generating ? '正在提交封面…' : '生成封面图片'}</button></div>
      <div className="creative-lineage" aria-label="当前任务链路">
        <div><span>创意输入</span><strong>{selected.intake.source === 'strategy_package' ? '来自策略包' : '手工输入'}</strong><small>{selected.intake.request.strategy_package ? `${selected.intake.request.strategy_package.package_id} · v${selected.intake.request.strategy_package.package_version}` : `Intake · ${selected.intake.id}`}</small></div>
        <div><span>图文任务</span><strong>当前初稿</strong><small>{selected.task.id}</small></div>
        <div><span>Provider 作业</span><strong>{selected.production_jobs.length ? `${selected.production_jobs.length} 个生产任务` : '尚未提交'}</strong><small>{selected.production_jobs.length ? '可查看执行和入库状态' : '点击“生成封面图片”开始'}</small></div>
        <div><span>项目素材</span><strong>{currentProductionAssets.length ? `${currentProductionAssets.length} 个已入库` : '等待生成结果'}</strong><small>素材由 Provider 校验入库</small></div>
      </div>
      <div className="creative-draft__grid"><article className="creative-copy"><h3>标题候选</h3><ol>{selected.draft.title_candidates.map((title) => <li key={title}>{title}</li>)}</ol><h3>正文</h3><p>{selected.draft.body}</p><div className="creative-topics">{selected.draft.topics.map((topic) => <span key={topic}>{topic}</span>)}</div></article><article className="creative-cover"><div className="creative-cover__canvas"><span>封面文字</span><strong>{selected.draft.cover_copy}</strong><small>{selected.task.direction.tone.join(' · ') || '小红书图文'}</small></div><h3>图组结构</h3><ol className="creative-image-plan">{selected.draft.image_plan.map((item) => <li key={item.order}><b>{item.order}</b><div><strong>{item.purpose}</strong><span>{item.visual_brief}</span></div></li>)}</ol></article></div>
      {selected.production_jobs.length > 0 ? <section className="creative-production"><span>封面生产状态</span>{selected.production_jobs.map((production) => {
        const job = providerJobs[production.provider_job_id]
        const assetCount = currentProductionAssets.filter((asset) => asset.version.provider_job_id === production.provider_job_id).length
        return <article key={production.provider_job_id}><div><strong>{job ? providerStatusLabels[job.provider_status] : '正在读取状态'}</strong><code>{production.provider_job_id}</code><small>{job && !terminalProviderStatuses.has(job.provider_status) ? `进度 ${job.progress}% · 正在自动刷新` : assetCount ? `${assetCount} 个素材已入库` : '尚未入库'}</small></div><div className="creative-production__actions"><Link className="text-button" to={`/projects/${encodeURIComponent(projectId)}/provider-jobs?job=${encodeURIComponent(production.provider_job_id)}`}>查看 Provider</Link><Link className="text-button" to={`/projects/${encodeURIComponent(projectId)}/assets?provider_job_id=${encodeURIComponent(production.provider_job_id)}`}>查看素材</Link></div></article>
      })}</section> : null}
    </section> : null}
    {latestJob ? <div className="success-note creative-message"><span>✓</span><p>Provider Job：<code>{latestJob.id}</code>，当前状态：{providerStatusLabels[latestJob.provider_status]}。</p></div> : null}
    {message ? <div className="success-note creative-message"><span>✓</span><p>{message}</p></div> : null}
    {error ? <div className="library-error" role="alert"><div><strong>创意操作失败</strong><span>{error}</span></div><button className="text-button" onClick={() => setError('')} type="button">关闭</button></div> : null}
  </section>
}
