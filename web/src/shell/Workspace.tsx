import { useEffect, useState } from 'react'
import { Link, Route, Routes, useLocation } from 'react-router-dom'
import { ProjectAssetsPage } from '../features/assets/ProjectAssetsPage'
import { getWorkspaceBootstrap } from '../features/platform/api'
import type { CurrentIdentity, Project } from '../features/platform/types'
import { Icon } from './Icon'
import { adminModule, shellModules } from './modules'

const modules = [...shellModules, adminModule]

function routeProjectId(pathname: string) {
  return pathname.match(/^\/projects\/([^/]+)/)?.[1] || ''
}

export function Workspace() {
  const location = useLocation()
  const projectId = routeProjectId(location.pathname)
  const moduleKey = location.pathname.startsWith('/projects/') ? 'creative' : location.pathname.split('/')[1]
  const activeModule = modules.find(({ key }) => key === moduleKey) ?? shellModules[0]
  const [projectMenuOpen, setProjectMenuOpen] = useState(false)
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
  const projectOptions: Array<Pick<Project, 'id' | 'name' | 'status'>> = projects.length > 0
    ? projects
    : [{ id: 'project_demo', name: '示例项目', status: 'active' }]
  const displayName = identity?.user?.display_name || identity?.actor.principal.id || '本地用户'
  const initial = displayName.slice(0, 1).toUpperCase()

  return (
    <div className="app-shell">
      <aside className="sidebar" aria-label="系统导航">
        <Link className="wordmark" to="/strategy" aria-label="cookies 首页">cookies</Link>
        <nav className="module-nav" aria-label="业务系统">
          {shellModules.map((module) => (
            <Link className={module.key === activeModule.key ? 'nav-item nav-item--active' : 'nav-item'} key={module.key} to={`/${module.key}`}>
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
              {projectOptions.map((project) => (
                <Link key={project.id} onClick={() => setProjectMenuOpen(false)} role="menuitem" to={`/projects/${project.id}/assets`}>
                  <span>{project.name}</span><small>{project.status === 'active' ? '活跃项目' : project.status === 'draft' ? '草稿项目' : '已归档'}</small>
                </Link>
              ))}
              <div className="project-menu__divider" />
              <Link onClick={() => setProjectMenuOpen(false)} role="menuitem" to={`/projects/${projectId || currentProject?.id || 'project_demo'}/provider-jobs`}>Provider Jobs</Link>
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
            <Route path="/projects/:projectId/assets" element={<ProjectAssetsPage />} />
            <Route path="/projects/:projectId/provider-jobs" element={<ProviderJobsPage />} />
            <Route path="*" element={<ModulePlaceholder label={activeModule.label} description={activeModule.description} />} />
          </Routes>
        </main>
      </div>
    </div>
  )
}

function ModulePlaceholder({ label, description }: { label: string; description: string }) {
  return <section className="placeholder-page"><div className="empty-state"><div className="module-mark" aria-hidden="true"><span /><span /><span /><span /></div><h1>{label}工作区</h1><p>{description}</p><p className="empty-state__hint">共享 Shell 已就绪，模块通过真实 URL 独立挂载。</p></div></section>
}

function ProviderJobsPage() {
  const projectId = routeProjectId(useLocation().pathname)
  return <section className="placeholder-page"><div className="empty-state"><h1>Provider Jobs</h1><p>Project: {projectId}</p><p className="empty-state__hint">Provider 模块将在此路由挂载生成请求与任务状态。</p></div></section>
}
