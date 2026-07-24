import { useEffect, useState } from 'react'
import { Link, Navigate, Route, Routes, useLocation, useNavigate, useParams } from 'react-router-dom'
import { ProjectAssetsPage } from '../features/assets/ProjectAssetsPage'
import { ProviderJobsPage } from '../features/provider/ProviderJobsPage'
import { IdentityOrganizationPage } from '../features/identity/IdentityOrganizationPage'
import { CreativeImageTextPage } from '../features/creative/CreativeImageTextPage'
import { getWorkspaceBootstrap } from '../features/platform/api'
import type { CurrentIdentity, Project } from '../features/platform/types'
import { NewProjectDialog } from '../features/projects/NewProjectDialog'
import { StrategyLanding, StrategyProjectHome } from '../features/strategy/StrategyProjectHome'
import { StrategyWorkspacePage } from '../features/strategy/StrategyWorkspacePage'
import { StrategyReviewPage } from '../features/strategy/StrategyReviewPage'
import { StrategyPackagePage } from '../features/strategy/StrategyPackagePage'
import { Icon } from './Icon'
import { adminModule, shellModules } from './modules'
import { logout } from '../auth/api'

const modules = [...shellModules, adminModule]

function routeProjectId(pathname: string) {
  return pathname.match(/^\/projects\/([^/]+)/)?.[1]
    || pathname.match(/^\/strategy\/projects\/([^/]+)/)?.[1]
    || ''
}

function destinationForProject(pathname: string, projectId: string) {
  if (pathname.includes('/strategy')) return `/projects/${projectId}/strategy/workspaces`
  if (pathname.includes('/provider-jobs')) return `/projects/${projectId}/provider-jobs`
  if (pathname.includes('/creative')) return `/projects/${projectId}/creative/tasks`
  return `/projects/${projectId}/assets`
}

function activeModuleKey(pathname: string) {
  if (pathname.includes('/strategy')) return 'strategy'
  if (pathname.includes('/creative')) return 'creative'
  if (pathname.startsWith('/admin')) return 'admin'
  return 'strategy'
}

export function Workspace() {
  const location = useLocation()
  const navigate = useNavigate()
  const projectId = routeProjectId(location.pathname)
  const moduleKey = activeModuleKey(location.pathname)
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
  const currentProjectId = projectId || currentProject?.id || ''
  const displayName = identity?.user?.display_name || identity?.actor.principal.id || '本地用户'
  const initial = displayName.slice(0, 1).toUpperCase()

  return (
    <div className="app-shell">
      <aside className="sidebar" aria-label="系统导航">
        <Link className="wordmark" to="/strategy" aria-label="cookies 首页">cookies</Link>
        <nav className="module-nav" aria-label="业务系统">
          {shellModules.map((module) => (
            <Link className={module.key === activeModule.key ? 'nav-item nav-item--active' : 'nav-item'} key={module.key} to={currentProject ? `/projects/${currentProject.id}/${module.key}${module.key === 'creative' ? '/tasks' : '/workspaces'}` : `/${module.key}`}>
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
            {projectMenuOpen ? <div aria-label="项目与工作区导航" className="project-menu" role="menu">
              <div className="project-menu__section-label"><span>切换项目</span><small>{projects.length} 个项目</small></div>
              <div className="project-menu__projects">
                {projects.map((project) => (
                  <Link className={project.id === currentProjectId ? 'project-menu__project project-menu__project--current' : 'project-menu__project'} key={project.id} onClick={() => setProjectMenuOpen(false)} role="menuitem" to={destinationForProject(location.pathname, project.id)}>
                    <span>{project.name}</span><small>{project.id === currentProjectId ? '当前项目' : project.status === 'active' ? '活跃项目' : project.status === 'draft' ? '草稿项目' : '已归档'}</small>
                  </Link>
                ))}
              </div>
              {projects.length === 0 ? <p className="project-menu__empty">还没有项目</p> : null}
              {currentProject ? <>
                <div className="project-menu__divider" />
                <div className="project-menu__section-label"><span>当前项目 · {currentProject.name}</span><small>工作区</small></div>
                <div className="project-menu__workspace-links">
                  <Link onClick={() => setProjectMenuOpen(false)} role="menuitem" to={`/projects/${currentProjectId}/strategy/workspaces`}><span>策略工作区</span><small>Brief 与策略包</small></Link>
                  <Link onClick={() => setProjectMenuOpen(false)} role="menuitem" to={`/projects/${currentProjectId}/creative/tasks`}><span>创意创作</span><small>图文任务与初稿</small></Link>
                  <Link onClick={() => setProjectMenuOpen(false)} role="menuitem" to={`/projects/${currentProjectId}/assets`}><span>项目素材库</span><small>上传与已入库素材</small></Link>
                  <Link onClick={() => setProjectMenuOpen(false)} role="menuitem" to={`/projects/${currentProjectId}/provider-jobs`}><span>模型作业（排障）</span><small>调用、失败原因与入库核查</small></Link>
                </div>
              </> : null}
              <div className="project-menu__divider" />
              <button className="project-menu__create" onClick={() => {
                setProjectMenuOpen(false)
                setProjectDialogOpen(true)
              }} role="menuitem" type="button">
                <svg aria-hidden="true" fill="none" height="18" stroke="currentColor" strokeLinecap="round" strokeWidth="1.8" viewBox="0 0 24 24" width="18"><path d="M12 5v14M5 12h14" /></svg>
                <span>新建项目</span>
              </button>
            </div> : null}
          </div>
          <div className="identity-summary" title={identity?.organization.name || ''}>
            <span className="identity-summary__org">{identity?.organization.name || '本地组织'}</span>
            <span className="avatar" aria-hidden="true">{initial}</span>
            <span className="identity-summary__name">{displayName}</span>
            <button className="identity-summary__logout" onClick={() => {
              logout().finally(() => navigate('/login', { replace: true }))
            }} type="button">退出</button>
          </div>
        </header>
        {bootstrapError ? <div className="workspace-alert" role="status">身份与项目列表暂不可用：{bootstrapError}</div> : null}
        <main className="workspace" aria-live="polite">
          <Routes>
            <Route path="/strategy" element={<StrategyLanding project={currentProject} />} />
            <Route path="/projects/:projectId/strategy" element={<ProjectStrategyRedirect />} />
            <Route path="/projects/:projectId/strategy/workspaces" element={<StrategyProjectHome project={currentProject} />} />
            <Route path="/projects/:projectId/strategy/workspaces/:workspaceId/:stage" element={<StrategyWorkspacePage project={currentProject} />} />
            <Route path="/projects/:projectId/strategy/reviews/:reviewId" element={<StrategyReviewPage />} />
            <Route path="/projects/:projectId/strategy/packages/:packageId" element={<StrategyPackagePage />} />
            <Route path="/strategy/projects/:projectId" element={<LegacyStrategyRedirect />} />
            <Route path="/strategy/projects/:projectId/workspaces/:workspaceId/*" element={<LegacyStrategyWorkspaceRedirect />} />
            <Route path="/projects/:projectId/assets" element={<ProjectAssetsPage project={currentProject} />} />
            <Route path="/projects/:projectId/provider-jobs" element={<ProviderJobsPage />} />
            <Route path="/projects/:projectId/creative/tasks" element={<CreativeImageTextPage />} />
            <Route path="/projects/:projectId/creative" element={<LegacyCreativeRedirect />} />
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

function LegacyStrategyRedirect() {
  const { projectId = '' } = useParams()
  return <Navigate replace to={`/projects/${projectId}/strategy/workspaces`} />
}

function ProjectStrategyRedirect() {
  const { projectId = '' } = useParams()
  return <Navigate replace to={`/projects/${projectId}/strategy/workspaces`} />
}

function LegacyStrategyWorkspaceRedirect() {
  const { projectId = '', workspaceId = '', '*': legacyStage = '' } = useParams()
  const stage = ['conversation', 'brief', 'strategy'].includes(legacyStage) ? legacyStage : 'conversation'
  return <Navigate replace to={`/projects/${projectId}/strategy/workspaces/${workspaceId}/${stage}`} />
}

function LegacyCreativeRedirect() {
  const { projectId = '' } = useParams()
  return <Navigate replace to={`/projects/${projectId}/creative/tasks`} />
}

function ModulePlaceholder({ label, description }: { label: string; description: string }) {
  return <section className="placeholder-page"><div className="empty-state"><div className="module-mark" aria-hidden="true"><span /><span /><span /><span /></div><h1>{label}工作区</h1><p>{description}</p><p className="empty-state__hint">共享 Shell 已就绪，模块通过真实 URL 独立挂载。</p></div></section>
}

export function LegacyProviderJobsPlaceholder() {
  const projectId = routeProjectId(useLocation().pathname)
  return <section className="placeholder-page"><div className="empty-state"><h1>Provider Jobs</h1><p>Project: {projectId}</p><p className="empty-state__hint">Provider 模块将在此路由挂载生成请求与任务状态。</p></div></section>
}
