import { useEffect, useState } from 'react'
import { Link, useSearchParams, useParams } from 'react-router-dom'
import { ApiProblem } from '../../shared/api/client'
import { getProjectContext } from '../platform/api'
import type { ProjectContext } from '../platform/types'
import { createCreativeImageJob, createCreativePlan } from './api'
import type { CreativeImageJob, CreativePlan } from './types'

function message(error: unknown) {
  if (error instanceof ApiProblem) return `${error.message}（${error.problem.error.code}）`
  return error instanceof Error ? error.message : '创意操作失败，请稍后重试。'
}

export function CreativeWorkspacePage() {
  const { projectId = '' } = useParams()
  const [searchParams] = useSearchParams()
  const strategyId = searchParams.get('strategy') || ''
  const [context, setContext] = useState<ProjectContext | null>(null)
  const [plan, setPlan] = useState<CreativePlan | null>(null)
  const [job, setJob] = useState<CreativeImageJob | null>(null)
  const [busy, setBusy] = useState<'plan' | 'image' | null>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    const controller = new AbortController()
    getProjectContext(projectId, controller.signal).then(setContext).catch((caught: unknown) => {
      if (!(caught instanceof DOMException && caught.name === 'AbortError')) setError(message(caught))
    })
    return () => controller.abort()
  }, [projectId])

  async function createPlan() {
    if (!strategyId || busy) return
    setBusy('plan')
    setError('')
    try {
      setPlan(await createCreativePlan(projectId, { strategy_output_id: strategyId }))
    } catch (caught) {
      setError(message(caught))
    } finally {
      setBusy(null)
    }
  }

  async function createImage() {
    if (!plan || !context || busy) return
    setBusy('image')
    setError('')
    try {
      setJob(await createCreativeImageJob(projectId, plan.id, {
        project_context_version: context.project_context_version,
        width: 1024,
        height: 1024,
      }))
    } catch (caught) {
      setError(message(caught))
    } finally {
      setBusy(null)
    }
  }

  return <section className="workflow-page">
    <header className="page-header">
      <div><h1>项目创意工作区</h1><p>创意计划由已审批策略派生。图像生成仍由既有 Provider Jobs 执行，完成入库后可在素材库查看。</p></div>
      <Link className="button button--secondary" to={`/projects/${encodeURIComponent(projectId)}/assets`}>查看项目素材</Link>
    </header>

    {!strategyId ? <div className="context-warning" role="status"><strong>需要已审批策略</strong><span>请先在策略工作区审批策略，再进入本页创建创意计划。</span><Link to={`/projects/${encodeURIComponent(projectId)}/strategy`}>前往策略工作区</Link></div> : null}

    <div className="workflow-layout">
      <article className="workflow-card workflow-output">
        <div className="workflow-card__heading"><div><h2>创意计划</h2><p>策略输出 ID：{strategyId || '未选择'}</p></div><span className="workflow-version">{context ? `上下文 v${context.project_context_version}` : '加载中'}</span></div>
        {!plan ? <p className="workflow-empty">根据已审批策略创建计划，生成图像与视频提示词变体。</p> : <>
          <dl className="workflow-facts"><div><dt>计划 ID</dt><dd>{plan.id}</dd></div><div><dt>模型别名</dt><dd>{plan.model_alias}</dd></div></dl>
          <div className="prompt-variant"><strong>图像提示词</strong><p>{plan.image_prompt}</p></div>
          <div className="prompt-variant"><strong>视频提示词</strong><p>{plan.video_prompt}</p></div>
        </>}
        <button className="button button--primary" disabled={!strategyId || Boolean(plan) || busy !== null} onClick={() => void createPlan()} type="button">{busy === 'plan' ? '正在创建…' : '创建创意计划'}</button>
      </article>

      <article className="workflow-card workflow-output" aria-live="polite">
        <div className="workflow-card__heading"><div><h2>图像 Provider Job</h2><p>仅提交计划 ID 与项目上下文版本；Provider 凭据与任务句柄不在浏览器显示。</p></div></div>
        {!job ? <p className="workflow-empty">创意计划就绪后，可将图像提示词提交到现有 Provider Jobs。</p> : <dl className="workflow-facts"><div><dt>任务 ID</dt><dd>{job.id}</dd></div><div><dt>Provider 状态</dt><dd>{job.provider_status}</dd></div><div><dt>进度</dt><dd>{job.progress}%</dd></div><div><dt>已入库素材</dt><dd>{job.project_asset_refs.length}</dd></div></dl>}
        <div className="workflow-actions">
          <button className="button button--primary" disabled={!plan || !context || busy !== null} onClick={() => void createImage()} type="button">{busy === 'image' ? '正在提交…' : '创建图像任务'}</button>
          <Link className="button button--secondary" to={`/projects/${encodeURIComponent(projectId)}/provider-jobs`}>打开 Provider Jobs</Link>
        </div>
      </article>
    </div>
    {error ? <div className="library-error" role="alert"><div><strong>创意工作区操作失败</strong><span>{error}</span></div><button className="text-button" onClick={() => setError('')} type="button">关闭</button></div> : null}
  </section>
}
