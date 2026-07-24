import { useEffect, useState } from 'react'
import { Link, Navigate, useNavigate } from 'react-router-dom'
import type { Project } from '../platform/types'
import { createWorkspace, listWorkspaces } from './api'
import type { Workspace } from './types'
import './strategy.css'

export function StrategyProjectHome({ project }: { project?: Project }) {
  const navigate = useNavigate()
  const [workspaces, setWorkspaces] = useState<Workspace[]>([])
  const [name, setName] = useState('主策略工作区')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (!project) return
    const controller = new AbortController()
    listWorkspaces(project.id, controller.signal).then(({ items }) => setWorkspaces(items)).catch((cause: unknown) => {
      if (!(cause instanceof DOMException && cause.name === 'AbortError')) setError(cause instanceof Error ? cause.message : '工作区加载失败')
    })
    return () => controller.abort()
  }, [project])

  if (!project) return <section className="strategy-empty"><h1>策略工作区</h1><p>请先创建或选择一个项目。</p></section>

  return <section className="strategy-home">
    <header><span className="eyebrow">STRATEGY</span><h1>{project.name} · 策略</h1><p>把自然语言需求沉淀为可确认、可评审、可交付给 Creative 的策略版本。</p></header>
    {error ? <div className="strategy-alert" role="alert">{error}</div> : null}
    {workspaces.length ? <div className="workspace-cards">
      {workspaces.map((workspace) => <Link className="workspace-card" key={workspace.id} to={`/projects/${project.id}/strategy/workspaces/${workspace.id}/conversation`}>
        <span>{workspace.is_primary ? '主工作区' : '工作区'}</span><h2>{workspace.name}</h2><p>继续需求梳理、Brief 确认与策略评审</p><strong>打开工作区 →</strong>
      </Link>)}
    </div> : <form className="create-workspace-card" onSubmit={async (event) => {
      event.preventDefault()
      setBusy(true)
      setError('')
      try {
        const workspace = await createWorkspace(project.id, name)
        navigate(`/projects/${project.id}/strategy/workspaces/${workspace.id}/conversation`)
      } catch (cause) {
        setError(cause instanceof Error ? cause.message : '创建失败')
      } finally {
        setBusy(false)
      }
    }}>
      <div><span className="eyebrow">新项目第一步</span><h2>创建策略工作区</h2><p>一个项目首期默认使用一个主工作区，历史版本会持续保留。</p></div>
      <label htmlFor="workspace-name">工作区名称</label>
      <input id="workspace-name" maxLength={255} onChange={(event) => setName(event.target.value)} value={name} />
      <button className="button button--primary" disabled={busy || !name.trim()} type="submit">{busy ? '正在创建…' : '创建并开始'}</button>
    </form>}
  </section>
}
// The shell already resolves the active project. Keeping this entry route
// project-aware prevents /strategy from stranding users on a static empty page.
export function StrategyLanding({ project }: { project?: Project }) {
  if (project) return <Navigate replace to={`/projects/${project.id}/strategy/workspaces`} />
  return <section className="strategy-empty"><span className="eyebrow">STRATEGY</span><h1>策略工作区</h1><p>从顶部选择项目，开始广告需求梳理。</p></section>
}
