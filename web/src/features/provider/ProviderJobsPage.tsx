import { useCallback, useEffect, useState } from 'react'
import { Link, useParams, useSearchParams } from 'react-router-dom'
import { createImageJob, getProjectContext, getProviderJob } from '../platform/api'
import type { ProviderJob } from '../platform/types'

const terminal = new Set<ProviderJob['provider_status']>(['succeeded', 'partially_succeeded', 'failed', 'cancelled', 'expired'])

const statusLabel: Record<ProviderJob['provider_status'], string> = {
  submitted: '已提交', running: '模型生成中', outputs_ready: '产物已就绪', ingesting: '素材入库中',
  succeeded: '已完成', partially_succeeded: '部分完成', failed: '失败', cancelled: '已取消', expired: '已过期',
}

export function ProviderJobsPage() {
  const { projectId = '' } = useParams()
  const [searchParams] = useSearchParams()
  const requestedJobId = searchParams.get('job')
  const [prompt, setPrompt] = useState('')
  const [size, setSize] = useState('1024x1024')
  const [job, setJob] = useState<ProviderJob | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')
  const jobId = job?.id ?? ''
  const jobStatus = job?.provider_status

  const refresh = useCallback(async (jobId: string, signal?: AbortSignal) => {
    try {
      setJob(await getProviderJob(projectId, jobId, signal))
    } catch (caught) {
      if (caught instanceof DOMException && caught.name === 'AbortError') return
      setError(caught instanceof Error ? caught.message : '无法加载任务状态。')
    }
  }, [projectId])

  useEffect(() => {
    if (!requestedJobId) return
    const controller = new AbortController()
    void getProviderJob(projectId, requestedJobId, controller.signal)
      .then(setJob)
      .catch((caught) => {
        if (!(caught instanceof DOMException && caught.name === 'AbortError')) setError(caught instanceof Error ? caught.message : '无法读取指定 Provider 作业。')
      })
    return () => controller.abort()
  }, [projectId, requestedJobId])

  useEffect(() => {
    if (!jobId || !jobStatus || terminal.has(jobStatus)) return
    const controller = new AbortController()
    const timer = window.setInterval(() => void refresh(jobId, controller.signal), 2000)
    return () => { window.clearInterval(timer); controller.abort() }
  }, [jobId, jobStatus, refresh])

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
      <div><h1>图像生成</h1><p>{requestedJobId ? '正在查看来自创意任务的 Provider 作业；状态和入库结果会自动刷新。' : '创建独立 Provider 作业；模型产物会经过校验和素材入库后，才出现在项目素材库中。'}</p></div>
      <Link className="button button--secondary" to={`/projects/${encodeURIComponent(projectId)}/assets`}>查看项目素材</Link>
    </header>
    <div className="provider-layout">
      <form className="provider-form" onSubmit={(event) => { event.preventDefault(); void submit() }}>
        <label>提示词<textarea value={prompt} maxLength={4000} onChange={(event) => setPrompt(event.target.value)} placeholder="描述你希望生成的图片…" /></label>
        <label>画面尺寸<select value={size} onChange={(event) => setSize(event.target.value)}><option value="1024x1024">1024 × 1024</option><option value="1024x768">1024 × 768</option><option value="768x1024">768 × 1024</option><option value="1365x1024">1365 × 1024</option><option value="1024x1365">1024 × 1365</option></select></label>
        <button className="button button--primary" disabled={!prompt.trim() || submitting} type="submit">{submitting ? '正在提交…' : '创建独立生成任务'}</button>
        {requestedJobId ? <p className="provider-form__hint">独立任务不会替代当前关联作业。返回创意页可继续查看完整生产链路。</p> : null}
      </form>
      <aside className="provider-job" aria-live="polite">
        <h2>{requestedJobId ? '关联作业' : '当前任务'}</h2>
        {!job ? <p className="provider-job__empty">{requestedJobId ? '正在读取关联作业…' : '尚未创建任务。提交后会在这里持续显示 Provider 与素材入库状态。'}</p> : <>
          <div className={`provider-status provider-status--${job.provider_status}`}><i />{statusLabel[job.provider_status]} <span>{job.progress}%</span></div>
          <div className="provider-progress"><span style={{ width: `${job.progress}%` }} /></div>
          <dl><div><dt>任务 ID</dt><dd>{job.id}</dd></div><div><dt>执行尝试</dt><dd>{job.attempt_count} / {job.max_attempts}</dd></div><div><dt>已入库素材</dt><dd>{job.project_asset_refs.length}</dd></div></dl>
          {job.error ? <div className="error-note"><strong>{job.error.code}</strong><span>{job.error.message}</span></div> : null}
          <div className="provider-job__links"><Link className="text-button" to={`/projects/${encodeURIComponent(projectId)}/assets?provider_job_id=${encodeURIComponent(job.id)}`}>查看本作业素材</Link>{terminal.has(job.provider_status) ? <button className="text-button" onClick={() => void refresh(job.id)} type="button">刷新状态</button> : <p className="provider-job__polling">每 2 秒刷新一次状态</p>}</div>
        </>}
      </aside>
    </div>
    {error ? <div className="library-error" role="alert"><div><strong>Provider 操作失败</strong><span>{error}</span></div><button className="text-button" onClick={() => setError('')} type="button">关闭</button></div> : null}
  </section>
}
