import { useState } from 'react'
import { BrowserRouter, Link, Navigate, Route, Routes, useLocation, useParams } from 'react-router-dom'
import { Icon } from './shell/Icon'
import { adminModule, shellModules } from './shell/modules'

const modules = [...shellModules, adminModule]

function Workspace() {
  const location = useLocation()
  const moduleKey = location.pathname.split('/')[1]
  const activeModule = modules.find(({ key }) => key === moduleKey) ?? shellModules[0]
  const [projectMenuOpen, setProjectMenuOpen] = useState(false)

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
              <span className="project-folder" aria-hidden="true" /><span>示例项目</span><span className="chevron" aria-hidden="true">⌄</span>
            </button>
            {projectMenuOpen ? <div className="project-menu" role="menu">
              <Link role="menuitem" to="/projects/project_demo/assets">项目素材库</Link>
              <Link role="menuitem" to="/projects/project_demo/provider-jobs">Provider Jobs</Link>
            </div> : null}
          </div>
          <div className="global-status" aria-label="全局入口"><span>待办</span><span>审批</span><span>通知</span><span className="avatar" aria-label="当前用户">U</span></div>
        </header>
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
  return <section className="empty-state"><div className="module-mark" aria-hidden="true"><span /><span /><span /><span /></div><h1>{label}工作区</h1><p>{description}</p><p className="empty-state__hint">共享 Shell 已就绪，模块通过真实 URL 独立挂载。</p></section>
}

function ProjectAssetsPage() {
  const { projectId } = useParams()
  return <section className="empty-state"><h1>项目素材库</h1><p>Project: {projectId}</p><p className="empty-state__hint">Assets 模块将在此路由挂载上传、列表和预览。</p></section>
}

function ProviderJobsPage() {
  const { projectId } = useParams()
  return <section className="empty-state"><h1>Provider Jobs</h1><p>Project: {projectId}</p><p className="empty-state__hint">Provider 模块将在此路由挂载生成请求与任务状态。</p></section>
}

export default function App() {
  return <BrowserRouter><Routes><Route path="/" element={<Navigate replace to="/strategy" />} /><Route path="/*" element={<Workspace />} /></Routes></BrowserRouter>
}
