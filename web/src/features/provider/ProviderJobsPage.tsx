import { useCallback, useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { createImageJob, getProjectContext, getProviderJob } from '../platform/api'
import type { ProviderJob } from '../platform/types'

const terminal = new Set<ProviderJob['provider_status']>(['succeeded', 'partially_succeeded', 'failed', 'cancelled', 'expired'])

const statusLabel: Record<ProviderJob['provider_status'], string> = {
  submitted: '已提交', running: '模型生成中', outputs_ready: '产物已就绪', ingesting: '素材入库中',
  succeeded: '已完成', partially_succeeded: '部分完成', failed: '失败', cancelled: '已取消', expired: '已过期',
}

export function ProviderJobsPage() {
  const { projectId = '' } = useParams()
  const [prompt, setPrompt] = useState('')
  const [size, setSize] = useState('1024x1024')
  const [job, setJob] = useState<ProviderJob | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')

  const refresh = useCallback(async (signal?: AbortSignal) => {
    if (!job?.id) return
    try {
      const current = await getProviderJob(projectId, job.id, signal)
      setJob(current)
    } catch (caught) {
      if (caught instanceof DOMException && caught.name === 'AbortError') return
      setError(caught instanceof Error ? caught.message : '无法加载任务状态。')
    }
  }, [job, projectId])

  useEffect(() => {
    if (!job || terminal.has(job.provider_status)) return
    const controller = new AbortController()
    const timer = window.setInterval(() => void refresh(controller.signal), 2000)
    return () => { window.clearInterval(timer); controller.abort() }
  }, [job, refresh])

  async function submit() {
    if (!prompt.trim() || submitting) return
    setSubmitting(true)
    setError('')
    try {
      const context = await getProjectContext(projectId)
      const [width, height] = size.split('x').map(Number)
      setJob(await createImageJob(projectId, context.project_context_version, { prompt: prompt.trim(), width, height }))
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : '无法提交生成任务。')
    } finally {
      setSubmitting(false)
    }
  }

  return <section className="provider-page">
    <header className="page-header">
      <div><h1>图像生成</h1><p>创建 Provider 作业；模型产物会经过校验和素材入库后，才出现在项目素材库中。</p></div>
      <Link className="button button--secondary" to={`/projects/${encodeURIComponent(projectId)}/assets`}>查看项目素材</Link>
    </header>
    <div className="provider-layout">
      <form className="provider-form" onSubmit={(event) => { event.preventDefault(); void submit() }}>
        <label>提示词<textarea value={prompt} maxLength={4000} onChange={(event) => setPrompt(event.target.value)} placeholder="描述你希望生成的图片…" /></label>
        <label>画面尺寸<select value={size} onChange={(event) => setSize(event.target.value)}><option value="1024x1024">1024 × 1024</option><option value="1024x768">1024 × 768</option><option value="768x1024">768 × 1024</option><option value="1365x1024">1365 × 1024</option><option value="1024x1365">1024 × 1365</option></select></label>
        <button className="button button--primary" disabled={!prompt.trim() || submitting} type="submit">{submitting ? '正在提交…' : '创建生成任务'}</button>
      </form>
      <aside className="provider-job" aria-live="polite">
        <h2>当前任务</h2>
        {!job ? <p className="provider-job__empty">尚未创建任务。提交后会在这里持续显示 Provider 与素材入库状态。</p> : <>
          <div className={`provider-status provider-status--${job.provider_status}`}><i />{statusLabel[job.provider_status]} <span>{job.progress}%</span></div>
          <div className="provider-progress"><span style={{ width: `${job.progress}%` }} /></div>
          <dl><div><dt>任务 ID</dt><dd>{job.id}</dd></div><div><dt>执行尝试</dt><dd>{job.attempt_count} / {job.max_attempts}</dd></div><div><dt>已入库素材</dt><dd>{job.project_asset_refs.length}</dd></div></dl>
          {job.error ? <div className="error-note"><strong>{job.error.code}</strong><span>{job.error.message}</span></div> : null}
          {terminal.has(job.provider_status) ? <button className="text-button" onClick={() => void refresh()} type="button">刷新状态</button> : <p className="provider-job__polling">每 2 秒刷新一次状态</p>}
        </>}
      </aside>
    </div>
    {error ? <div className="library-error" role="alert"><div><strong>Provider 操作失败</strong><span>{error}</span></div><button className="text-button" onClick={() => setError('')} type="button">关闭</button></div> : null}
  </section>
}
