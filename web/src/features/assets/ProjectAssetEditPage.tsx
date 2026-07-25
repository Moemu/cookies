import { useCallback, useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { createImageJob, getProjectContext, getProviderJob } from '../platform/api'
import type { ProviderJob } from '../platform/types'
import { getAssetPreview } from './api'

const terminal = new Set<ProviderJob['provider_status']>(['succeeded', 'partially_succeeded', 'failed', 'cancelled', 'expired'])

const statusLabel: Record<ProviderJob['provider_status'], string> = {
  submitted: '已提交',
  running: '模型编辑中',
  outputs_ready: '产物已就绪',
  ingesting: '素材入库中',
  succeeded: '已完成',
  partially_succeeded: '部分完成',
  failed: '失败',
  cancelled: '已取消',
  expired: '已过期',
}

export function ProjectAssetEditPage() {
  const { projectId = '', assetId = '', version = '1' } = useParams()
  const assetVersion = Number(version)
  const [previewUrl, setPreviewUrl] = useState('')
  const [previewKey, setPreviewKey] = useState('')
  const [prompt, setPrompt] = useState('')
  const [size, setSize] = useState('1024x1024')
  const [job, setJob] = useState<ProviderJob | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const currentPreviewKey = `${projectId}:${assetId}:${assetVersion}`
  const previewMatches = previewKey === currentPreviewKey
  const [error, setError] = useState('')

  useEffect(() => {
    const controller = new AbortController()
    getAssetPreview(projectId, assetId, assetVersion, controller.signal).then((signed) => {
      setPreviewKey(currentPreviewKey)
      setPreviewUrl(signed.url)
      setError('')
    }).catch((caught: unknown) => {
      if (caught instanceof DOMException && caught.name === 'AbortError') return
      setPreviewUrl('')
      setPreviewKey(currentPreviewKey)
      setError(caught instanceof Error ? caught.message : '源素材预览加载失败。')
    })
    return () => controller.abort()
  }, [assetId, assetVersion, currentPreviewKey, projectId])

  const refresh = useCallback(async (signal?: AbortSignal) => {
    if (!job?.id) return
    try {
      setJob(await getProviderJob(projectId, job.id, signal))
    } catch (caught) {
      if (caught instanceof DOMException && caught.name === 'AbortError') return
      setError(caught instanceof Error ? caught.message : '无法加载任务状态。')
    }
  }, [job, projectId])

  useEffect(() => {
    if (!job || terminal.has(job.provider_status)) return
    const controller = new AbortController()
    const timer = window.setInterval(() => void refresh(controller.signal), 2000)
    return () => {
      window.clearInterval(timer)
      controller.abort()
    }
  }, [job, refresh])

  async function submit() {
    if (!prompt.trim() || submitting || !assetId || !assetVersion) return
    setSubmitting(true)
    setError('')
    try {
      const context = await getProjectContext(projectId)
      const [width, height] = size.split('x').map(Number)
      const source = { project_id: projectId, asset_version: { asset_id: assetId, version: assetVersion } }
      setJob(await createImageJob(projectId, context.project_context_version, { prompt: prompt.trim(), width, height, source_assets: [source] }, 'image.edit'))
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : '无法提交图片编辑任务。')
    } finally {
      setSubmitting(false)
    }
  }

  return <section className="asset-edit-page">
    <header className="page-header">
      <div>
        <h1>图片编辑</h1>
        <p>基于项目素材创建编辑任务；源图内容只在服务端读取，模型结果完成入库后会回到素材库。</p>
      </div>
      <Link className="button button--secondary" to={`/projects/${encodeURIComponent(projectId)}/assets`}>返回素材库</Link>
    </header>

    <div className="asset-edit-layout">
      <div className="asset-edit-preview">
        <div className="asset-edit-preview__stage">
          {previewMatches && previewUrl ? <img alt="编辑源素材预览" src={previewUrl} /> : <span>{previewMatches ? '源图不可预览' : '正在加载源图…'}</span>}
        </div>
        <dl>
          <div><dt>素材 ID</dt><dd>{assetId}</dd></div>
          <div><dt>版本</dt><dd>v{assetVersion || '-'}</dd></div>
        </dl>
      </div>

      <form className="provider-form asset-edit-form" onSubmit={(event) => { event.preventDefault(); void submit() }}>
        <label>编辑指令<textarea value={prompt} maxLength={4000} onChange={(event) => setPrompt(event.target.value)} placeholder="例如：保留主体构图，把背景换成清晨咖啡店，增加柔和自然光…" /></label>
        <label>输出尺寸<select value={size} onChange={(event) => setSize(event.target.value)}><option value="1024x1024">1024 × 1024</option><option value="1024x768">1024 × 768</option><option value="768x1024">768 × 1024</option><option value="1365x1024">1365 × 1024</option><option value="1024x1365">1024 × 1365</option></select></label>
        <button className="button button--primary" disabled={!prompt.trim() || submitting || !assetVersion} type="submit">{submitting ? '正在提交…' : '创建编辑任务'}</button>
      </form>

      <aside className="provider-job asset-edit-job" aria-live="polite">
        <h2>编辑任务</h2>
        {!job ? <p className="provider-job__empty">提交后会在这里显示模型编辑、输出校验和素材入库状态。</p> : <>
          <div className={`provider-status provider-status--${job.provider_status}`}><i />{statusLabel[job.provider_status]} <span>{job.progress}%</span></div>
          <div className="provider-progress"><span style={{ width: `${job.progress}%` }} /></div>
          <dl><div><dt>任务 ID</dt><dd>{job.id}</dd></div><div><dt>已入库素材</dt><dd>{job.project_asset_refs.length}</dd></div></dl>
          {job.error ? <div className="error-note"><strong>{job.error.code}</strong><span>{job.error.message}</span></div> : null}
          {terminal.has(job.provider_status) ? <Link className="text-button" to={`/projects/${encodeURIComponent(projectId)}/assets`}>查看素材库结果</Link> : <p className="provider-job__polling">每 2 秒刷新一次状态</p>}
        </>}
      </aside>
    </div>

    {error ? <div className="library-error" role="alert"><div><strong>图片编辑失败</strong><span>{error}</span></div><button className="text-button" onClick={() => setError('')} type="button">关闭</button></div> : null}
  </section>
}
