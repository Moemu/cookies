import { useCallback, useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { createCoverImageJob, createCreativeIntake, createCreativeTask, getCreativeTask, listCreativeIntakes, listCreativeTasks } from './api'
import type { CreativeIntakeInput, CreativeTask, CreativeTaskDetail } from './types'
import type { ProviderJob } from '../platform/types'

const emptyInput: CreativeIntakeInput = {
  source: 'manual', channel: 'xiaohongshu', objective: '', audience: '', core_message: '', call_to_action: '', concept: '', tone: [], visual_keywords: [], mandatory_elements: [], prohibited_claims: [],
}

function values(value: string) {
  return value.split(/[，,]/).map((item) => item.trim()).filter(Boolean)
}

export function CreativeImageTextPage() {
  const { projectId = '' } = useParams()
  const [input, setInput] = useState<CreativeIntakeInput>(emptyInput)
  const [tasks, setTasks] = useState<CreativeTask[]>([])
  const [selected, setSelected] = useState<CreativeTaskDetail | null>(null)
  const [loading, setLoading] = useState(true)
  const [submitting, setSubmitting] = useState(false)
  const [generating, setGenerating] = useState(false)
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  const [latestJob, setLatestJob] = useState<ProviderJob | null>(null)

  const selectTask = useCallback(async (taskId: string, signal?: AbortSignal) => {
    const detail = await getCreativeTask(projectId, taskId, signal)
    setSelected(detail)
  }, [projectId])

  const load = useCallback(async (signal?: AbortSignal) => {
    setError('')
    try {
      const [taskResponse] = await Promise.all([listCreativeTasks(projectId, signal), listCreativeIntakes(projectId, signal)])
      setTasks(taskResponse.items)
      if (taskResponse.items.length > 0) await selectTask(taskResponse.items[0].id, signal)
    } catch (caught) {
      if (caught instanceof DOMException && caught.name === 'AbortError') return
      setError(caught instanceof Error ? caught.message : '无法加载创意工作区。')
    } finally { setLoading(false) }
  }, [projectId, selectTask])

  useEffect(() => {
    const controller = new AbortController()
    const timer = window.setTimeout(() => void load(controller.signal), 0)
    return () => { window.clearTimeout(timer); controller.abort() }
  }, [load])

  function change<K extends keyof CreativeIntakeInput>(key: K, value: CreativeIntakeInput[K]) {
    setInput((current) => ({ ...current, [key]: value }))
  }

  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (submitting) return
    setSubmitting(true); setError(''); setMessage('')
    try {
      const intake = await createCreativeIntake(projectId, input)
      if (intake.status === 'needs_clarification') {
        setMessage(`输入已保存，还需要补充：${intake.missing_fields.join('、')}。`)
        return
      }
      const task = await createCreativeTask(projectId, intake.id)
      setTasks((current) => [task, ...current])
      await selectTask(task.id)
      setMessage('已创建小红书图文任务，并生成可编辑的图文初稿。')
      setInput(emptyInput)
    } catch (caught) { setError(caught instanceof Error ? caught.message : '无法创建创意任务。')
    } finally { setSubmitting(false) }
  }

  async function generateCover() {
    if (!selected || generating) return
    setGenerating(true); setError(''); setMessage('')
    try {
      const job = await createCoverImageJob(projectId, selected.task.id)
      setLatestJob(job)
      await selectTask(selected.task.id)
      setMessage('封面生图任务已提交。Provider 产物校验入库后，会显示在项目素材库中。')
    } catch (caught) { setError(caught instanceof Error ? caught.message : '无法创建封面生成任务。')
    } finally { setGenerating(false) }
  }

  return <section className="creative-page">
    <header className="page-header creative-page__header">
      <div><span className="creative-kicker">Creative / Image & Text</span><h1>小红书图文创作</h1><p>从独立的 Creative Intake 开始，产出可审阅图文初稿；策略接入与素材生成都保留可追溯边界。</p></div>
      <Link className="button button--secondary" to={`/projects/${encodeURIComponent(projectId)}/assets`}>项目素材库</Link>
    </header>

    <div className="creative-workspace">
      <form className="creative-intake" onSubmit={(event) => void submit(event)}>
        <div className="creative-panel__heading"><div><span>01 · 创意输入</span><h2>先说清楚要表达什么</h2></div><small>手工入口 · 小红书图文</small></div>
        <label>传播目标<input value={input.objective} onChange={(event) => change('objective', event.target.value)} placeholder="例如：建立新品认知" /></label>
        <label>目标人群<input value={input.audience} onChange={(event) => change('audience', event.target.value)} placeholder="例如：关注生活方式的年轻上班族" /></label>
        <label>核心信息<textarea value={input.core_message} onChange={(event) => change('core_message', event.target.value)} placeholder="希望用户记住的一句话" /></label>
        <div className="creative-form-grid"><label>创意概念<input value={input.concept} onChange={(event) => change('concept', event.target.value)} placeholder="例如：晨光中的蓝白咖啡桌" /></label><label>行动引导<input value={input.call_to_action} onChange={(event) => change('call_to_action', event.target.value)} placeholder="例如：收藏这份晨间灵感" /></label></div>
        <div className="creative-form-grid"><label>语气（逗号分隔）<input value={input.tone.join('，')} onChange={(event) => change('tone', values(event.target.value))} placeholder="自然，克制，可信" /></label><label>视觉关键词（逗号分隔）<input value={input.visual_keywords.join('，')} onChange={(event) => change('visual_keywords', values(event.target.value))} placeholder="蓝白，晨光，留白" /></label></div>
        <button className="button button--primary" disabled={submitting} type="submit">{submitting ? '正在建立任务…' : '保存输入并创建图文任务'}</button>
        <p className="creative-intake__hint">缺少传播目标、目标人群或核心信息时，系统只会保存为“等待补充”，不会创建半成品任务。</p>
      </form>

      <aside className="creative-task-list" aria-label="创意任务列表">
        <div className="creative-panel__heading"><div><span>02 · 任务</span><h2>图文初稿</h2></div><small>{tasks.length} 项</small></div>
        {loading ? <p className="creative-empty">正在加载任务…</p> : tasks.length === 0 ? <p className="creative-empty">还没有图文任务。完成左侧输入后，会在这里出现一份可编辑的初稿。</p> : <div className="creative-task-items">{tasks.map((task) => <button className={selected?.task.id === task.id ? 'creative-task-item creative-task-item--selected' : 'creative-task-item'} key={task.id} onClick={() => void selectTask(task.id)} type="button"><span>{task.direction.concept || '未命名方向'}</span><small>{task.channel === 'xiaohongshu' ? '小红书图文' : task.channel} · {task.status === 'draft' ? '初稿' : task.status}</small></button>)}</div>}
      </aside>
    </div>

    {selected ? <section className="creative-draft" aria-label="图文内容初稿">
      <div className="creative-draft__top"><div><span>03 · 内容初稿 v{selected.draft.version}</span><h2>{selected.task.direction.concept || '小红书图文方向'}</h2><p>{selected.intake.request.objective} · 面向 {selected.intake.request.audience}</p></div><button className="button button--primary" disabled={generating} onClick={() => void generateCover()} type="button">{generating ? '正在提交封面…' : '生成封面图片'}</button></div>
      <div className="creative-draft__grid"><article className="creative-copy"><h3>标题候选</h3><ol>{selected.draft.title_candidates.map((title) => <li key={title}>{title}</li>)}</ol><h3>正文</h3><p>{selected.draft.body}</p><div className="creative-topics">{selected.draft.topics.map((topic) => <span key={topic}>{topic}</span>)}</div></article><article className="creative-cover"><div className="creative-cover__canvas"><span>封面文字</span><strong>{selected.draft.cover_copy}</strong><small>{selected.task.direction.tone.join(' · ') || '小红书图文'}</small></div><h3>图组结构</h3><ol className="creative-image-plan">{selected.draft.image_plan.map((item) => <li key={item.order}><b>{item.order}</b><div><strong>{item.purpose}</strong><span>{item.visual_brief}</span></div></li>)}</ol></article></div>
      {selected.production_jobs.length > 0 ? <div className="creative-production"><span>封面生产血缘</span>{selected.production_jobs.map((job) => <code key={job.provider_job_id}>{job.provider_job_id}</code>)}</div> : null}
    </section> : null}
    {latestJob ? <div className="success-note creative-message"><span>✓</span><p>Provider Job：<code>{latestJob.id}</code>，当前状态：{latestJob.provider_status}。</p></div> : null}
    {message ? <div className="success-note creative-message"><span>✓</span><p>{message}</p></div> : null}
    {error ? <div className="library-error" role="alert"><div><strong>创意操作失败</strong><span>{error}</span></div><button className="text-button" onClick={() => setError('')} type="button">关闭</button></div> : null}
  </section>
}
