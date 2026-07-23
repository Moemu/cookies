import { useEffect, useState } from 'react'
import { Link, Route, Routes, useLocation, useNavigate } from 'react-router-dom'
import { ProjectAssetsPage } from '../features/assets/ProjectAssetsPage'
import { CreativeWorkspacePage } from '../features/creative/CreativeWorkspacePage'
import { ProviderJobsPage } from '../features/provider/ProviderJobsPage'
import { IdentityOrganizationPage } from '../features/identity/IdentityOrganizationPage'
import { getWorkspaceBootstrap } from '../features/platform/api'
import type { CurrentIdentity, Project } from '../features/platform/types'
import { NewProjectDialog } from '../features/projects/NewProjectDialog'
import { StrategyWorkspacePage } from '../features/strategy/StrategyWorkspacePage'
import { Icon } from './Icon'
import { adminModule, shellModules } from './modules'

const modules = [...shellModules, adminModule]

function routeProjectId(pathname: string) {
  return pathname.match(/^\/projects\/([^/]+)/)?.[1] || ''
}

export function Workspace() {
  const location = useLocation()
  const navigate = useNavigate()
  const projectId = routeProjectId(location.pathname)
  const moduleKey = location.pathname.startsWith('/projects/') ? 'creative' : location.pathname.split('/')[1]
  const activeModule = modules.find(({ key }) => key === moduleKey) ?? shellModules[0]
  const [projectMenuOpen, setProjectMenuOpen] = useState(false)
  const [projectDialogOpen, setProjectDialogOpen] = useState(false)
  const [identity, setIdentity] = useState<CurrentIdentity | null>(null)
  const [projects, setProjects] = useState<Project[]>([])
  const [bootstrapError, setBootstrapError] = useState('')

  useEffect(() => {
    const controller = new AbortController()
    getWorkspaceBootstrap(controller.signal).then((result) => {
      setIdentity(result.identity)
      setProjects(result.projects)
    }).catch((error: unknown) => {
      if (error instanceof DOMException && error.name === 'AbortError') return
      setBootstrapError(error instanceof Error ? error.message : '工作区信息加载失败。')
    })
    return () => controller.abort()
  }, [])

  const currentProject = projects.find((project) => project.id === projectId) ?? projects[0]
  const displayName = identity?.user?.display_name || identity?.actor.principal.id || '本地用户'
  const initial = displayName.slice(0, 1).toUpperCase()

  return (
    <div className="app-shell">
      <aside className="sidebar" aria-label="系统导航">
        <Link className="wordmark" to="/strategy" aria-label="cookies 首页">cookies</Link>
        <nav className="module-nav" aria-label="业务系统">
          {shellModules.map((module) => (
            <Link className={module.key === activeModule.key ? 'nav-item nav-item--active' : 'nav-item'} key={module.key} to={currentProject && (module.key === 'strategy' || module.key === 'creative') ? `/projects/${currentProject.id}/${module.key}` : `/${module.key}`}>
              <Icon name={module.icon} /><span>{module.label}</span>
            </Link>
          ))}
        </nav>
        <Link className={activeModule.key === adminModule.key ? 'nav-item nav-item--active nav-item--bottom' : 'nav-item nav-item--bottom'} to="/admin">
          <Icon name={adminModule.icon} /><span>{adminModule.label}</span>
        </Link>
      </aside>
      <div className="workspace-frame">
        <header className="topbar">
          <div className="project-switcher">
            <button aria-expanded={projectMenuOpen} className="project-trigger" onClick={() => setProjectMenuOpen((open) => !open)} type="button">
              <span className="project-folder" aria-hidden="true" /><span>{currentProject?.name || '选择项目'}</span><span className="chevron" aria-hidden="true">⌄</span>
            </button>
            {projectMenuOpen ? <div className="project-menu" role="menu">
              {projects.map((project) => (
                <Link key={project.id} onClick={() => setProjectMenuOpen(false)} role="menuitem" to={`/projects/${project.id}/assets`}>
                  <span>{project.name}</span><small>{project.status === 'active' ? '活跃项目' : project.status === 'draft' ? '草稿项目' : '已归档'}</small>
                </Link>
              ))}
              {projects.length === 0 ? <p className="project-menu__empty">还没有项目</p> : null}
              <div className="project-menu__divider" />
              <button className="project-menu__create" onClick={() => {
                setProjectMenuOpen(false)
                setProjectDialogOpen(true)
              }} role="menuitem" type="button">
                <svg aria-hidden="true" fill="none" height="18" stroke="currentColor" strokeLinecap="round" strokeWidth="1.8" viewBox="0 0 24 24" width="18"><path d="M12 5v14M5 12h14" /></svg>
                <span>新建项目</span>
              </button>
              {currentProject ? <>
                <Link onClick={() => setProjectMenuOpen(false)} role="menuitem" to={`/projects/${currentProject.id}/strategy`}>项目策略</Link>
                <Link onClick={() => setProjectMenuOpen(false)} role="menuitem" to={`/projects/${currentProject.id}/creative`}>项目创意</Link>
                <Link onClick={() => setProjectMenuOpen(false)} role="menuitem" to={`/projects/${projectId || currentProject.id}/provider-jobs`}>Provider Jobs</Link>
              </> : null}
            </div> : null}
          </div>
          <div className="identity-summary" title={identity?.organization.name || ''}>
            <span className="identity-summary__org">{identity?.organization.name || '本地组织'}</span>
            <span className="avatar" aria-hidden="true">{initial}</span>
            <span className="identity-summary__name">{displayName}</span>
          </div>
        </header>
        {bootstrapError ? <div className="workspace-alert" role="status">身份与项目列表暂不可用：{bootstrapError}</div> : null}
        <main className="workspace" aria-live="polite">
          <Routes>
            <Route path="/projects/:projectId/assets" element={<ProjectAssetsPage project={currentProject} />} />
            <Route path="/projects/:projectId/strategy" element={<StrategyWorkspacePage />} />
            <Route path="/projects/:projectId/creative" element={<CreativeWorkspacePage />} />
            <Route path="/projects/:projectId/provider-jobs" element={<ProviderJobsPage />} />
            <Route path="/admin" element={<IdentityOrganizationPage identity={identity} projects={projects} />} />
            <Route path="*" element={<ModulePlaceholder label={activeModule.label} description={activeModule.description} />} />
          </Routes>
        </main>
      </div>
      <NewProjectDialog
        onClose={() => setProjectDialogOpen(false)}
        onCreated={(project) => {
          setProjects((current) => [...current, project])
          setProjectDialogOpen(false)
          navigate(`/projects/${project.id}/assets`)
        }}
        open={projectDialogOpen}
      />
    </div>
  )
}

function ModulePlaceholder({ label, description }: { label: string; description: string }) {
  return <section className="placeholder-page"><div className="empty-state"><div className="module-mark" aria-hidden="true"><span /><span /><span /><span /></div><h1>{label}工作区</h1><p>{description}</p><p className="empty-state__hint">共享 Shell 已就绪，模块通过真实 URL 独立挂载。</p></div></section>
}

export function LegacyProviderJobsPlaceholder() {
  const projectId = routeProjectId(useLocation().pathname)
  return <section className="placeholder-page"><div className="empty-state"><h1>Provider Jobs</h1><p>Project: {projectId}</p><p className="empty-state__hint">Provider 模块将在此路由挂载生成请求与任务状态。</p></div></section>
}
