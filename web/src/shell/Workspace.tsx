import { useEffect, useState } from 'react'
import { Link, Navigate, NavLink, Route, Routes, useLocation, useNavigate, useParams } from 'react-router-dom'
import {
  activeBusinessModule,
  creativeTasksPath,
  deliveryPath,
  destinationForProject,
  insightsPath,
  projectHomePath,
  projectManagePath,
  routeProjectId,
  strategyPath,
} from '../app/routes'
import { AccountPage } from '../account/AccountPage'
import { ProjectAssetsPage } from '../features/assets/ProjectAssetsPage'
import { ProjectAssetEditPage } from '../features/assets/ProjectAssetEditPage'
import { ProjectAssetRemixPage } from '../features/assets/ProjectAssetRemixPage'
import { CreativeImageTextPage } from '../features/creative/CreativeImageTextPage'
import { DeliveryWorkspacePage } from '../features/delivery/DeliveryWorkspacePage'
import { IdentityOrganizationPage } from '../features/identity/IdentityOrganizationPage'
import { InsightsWorkspacePage } from '../features/insights/InsightsWorkspacePage'
import { getWorkspaceBootstrap } from '../features/platform/api'
import type { CurrentIdentity, Project } from '../features/platform/types'
import { ProjectHomePage, ProjectManagePage, ProjectsPage } from '../features/projects/ProjectPages'
import { NewProjectDialog } from '../features/projects/NewProjectDialog'
import { ProviderJobsPage } from '../features/provider/ProviderJobsPage'
import { StrategyPackagePage } from '../features/strategy/StrategyPackagePage'
import { StrategyLanding, StrategyProjectHome } from '../features/strategy/StrategyProjectHome'
import { StrategyReviewPage } from '../features/strategy/StrategyReviewPage'
import { StrategyWorkspacePage } from '../features/strategy/StrategyWorkspacePage'
import { Icon } from './Icon'
import { UserMenu } from './UserMenu'

type ShellSidebarProps = {
  module: ReturnType<typeof activeBusinessModule>
  currentProjectId: string
  canDelivery: boolean
  canInsights: boolean
}

function navClass({ isActive }: { isActive: boolean }) {
  return isActive ? 'nav-item nav-item--active' : 'nav-item'
}

function ShellSidebar({ module, currentProjectId, canDelivery, canInsights }: ShellSidebarProps) {
  const fallback = '/projects'
  const home = currentProjectId ? projectHomePath(currentProjectId) : fallback
  const strategy = currentProjectId ? strategyPath(currentProjectId) : fallback
  const creative = currentProjectId ? creativeTasksPath(currentProjectId) : fallback
  const delivery = currentProjectId ? deliveryPath(currentProjectId) : fallback
  const insights = currentProjectId ? insightsPath(currentProjectId) : fallback

  if (module === 'admin') {
    return <aside className="sidebar" aria-label="管理导航">
      <div className="sidebar__module"><Icon name="settings" /><strong>管理</strong></div>
      <span className="sidebar__section-label">组织</span>
      <nav className="module-nav">
        <NavLink className={navClass} end to="/admin"><Icon name="settings" /><span>组织与访问</span></NavLink>
        <NavLink className={navClass} to="/account/profile"><Icon name="user" /><span>个人资料</span></NavLink>
      </nav>
    </aside>
  }

  if (module === 'creative') {
    return <aside className="sidebar" aria-label="创意创作导航">
      <div className="sidebar__module"><Icon name="pen" /><strong>创意创作</strong></div>
      <span className="sidebar__section-label">工作</span>
      <nav className="module-nav">
        <NavLink className={navClass} end to={creative}><Icon name="list" /><span>创意任务</span></NavLink>
      </nav>
      <span className="sidebar__section-label">项目上下文</span>
      <nav className="module-nav">
        <NavLink className={navClass} to={home}><Icon name="home" /><span>项目进度</span></NavLink>
      </nav>
      <div className="sidebar__footer"><span>PROJECT</span><strong>{currentProjectId || '未选择'}</strong></div>
    </aside>
  }

  if (module === 'strategy') {
    return <aside className="sidebar" aria-label="需求与策略导航">
      <div className="sidebar__module"><Icon name="target" /><strong>需求与策略</strong></div>
      <span className="sidebar__section-label">工作</span>
      <nav className="module-nav">
        <NavLink className={navClass} to={strategy}><Icon name="target" /><span>策略工作区</span></NavLink>
      </nav>
      <span className="sidebar__section-label">项目上下文</span>
      <nav className="module-nav">
        <NavLink className={navClass} to={home}><Icon name="home" /><span>项目进度</span></NavLink>
      </nav>
      <div className="sidebar__footer"><span>PROJECT</span><strong>{currentProjectId || '未选择'}</strong></div>
    </aside>
  }

  if (module === 'delivery' && canDelivery) {
    return <aside className="sidebar" aria-label="智能投放导航">
      <div className="sidebar__module"><Icon name="send" /><strong>智能投放</strong></div>
      <span className="sidebar__section-label">投放</span>
      <nav className="module-nav">
        <NavLink className={navClass} to={delivery}><Icon name="list" /><span>投放计划</span></NavLink>
        <NavLink className={navClass} to={deliveryPath(currentProjectId, 'monitoring')}><Icon name="chart" /><span>投放监控</span></NavLink>
        <NavLink className={navClass} to={deliveryPath(currentProjectId, 'accounts')}><Icon name="user" /><span>广告账户</span></NavLink>
        <NavLink className={navClass} to={deliveryPath(currentProjectId, 'optimization')}><Icon name="target" /><span>优化建议</span></NavLink>
      </nav>
      <div className="sidebar__footer"><span>PROJECT</span><strong>{currentProjectId || '未选择'}</strong></div>
    </aside>
  }

  if (module === 'insights' && canInsights) {
    return <aside className="sidebar" aria-label="素材洞察导航">
      <div className="sidebar__module"><Icon name="chart" /><strong>素材洞察</strong></div>
      <span className="sidebar__section-label">分析</span>
      <nav className="module-nav">
        <NavLink className={navClass} to={insights}><Icon name="target" /><span>投前洞察</span></NavLink>
        <NavLink className={navClass} to={insightsPath(currentProjectId, 'performance')}><Icon name="chart" /><span>投后分析</span></NavLink>
        <NavLink className={navClass} to={insightsPath(currentProjectId, 'reports')}><Icon name="list" /><span>复盘报告</span></NavLink>
        <NavLink className={navClass} to={insightsPath(currentProjectId, 'experiences')}><Icon name="database" /><span>经验沉淀</span></NavLink>
      </nav>
      <div className="sidebar__footer"><span>PROJECT</span><strong>{currentProjectId || '未选择'}</strong></div>
    </aside>
  }

  return <aside className="sidebar" aria-label="项目导航">
    <div className="sidebar__module"><Icon name="home" /><strong>项目中心</strong></div>
    <span className="sidebar__section-label">项目</span>
    <nav className="module-nav">
      <NavLink className={navClass} end to="/projects"><Icon name="list" /><span>全部项目</span></NavLink>
      {currentProjectId ? <>
        <NavLink className={navClass} to={home}><Icon name="home" /><span>项目概览</span></NavLink>
        <NavLink className={navClass} to={projectManagePath(currentProjectId)}><Icon name="settings" /><span>项目管理</span></NavLink>
      </> : null}
    </nav>
    <div className="sidebar__footer"><span>PROJECT</span><strong>{currentProjectId || '未选择'}</strong></div>
  </aside>
}

function ProjectMenu({
  canDelivery,
  canInsights,
  currentProject,
  currentProjectId,
  location,
  onClose,
  onCreate,
  projects,
}: {
  currentProject?: Project
  currentProjectId: string
  location: string
  onClose: () => void
  onCreate: () => void
  projects: Project[]
  canDelivery?: boolean
  canInsights?: boolean
}) {
  return <div aria-label="项目与工作区导航" className="project-menu" role="menu">
    <div className="project-menu__section-label"><span>切换项目</span><small>{projects.length} 个项目</small></div>
    <div className="project-menu__projects">
      {projects.map((project) => <Link
        className={project.id === currentProjectId ? 'project-menu__project project-menu__project--current' : 'project-menu__project'}
        key={project.id}
        onClick={onClose}
        role="menuitem"
        to={destinationForProject(location, project.id)}
      >
        <span>{project.name}</span>
        <small>{project.id === currentProjectId ? '当前项目' : project.status === 'active' ? '进行中' : project.status === 'draft' ? '草稿' : '已归档'}</small>
      </Link>)}
    </div>
    {projects.length === 0 ? <p className="project-menu__empty">还没有项目</p> : null}
    {currentProject ? <>
      <div className="project-menu__divider" />
      <div className="project-menu__section-label"><span>{currentProject.name}</span><small>项目工作区</small></div>
      <div className="project-menu__workspace-links">
        <Link onClick={onClose} role="menuitem" to={projectHomePath(currentProjectId)}><span>项目首页</span><small>八阶段真实进度</small></Link>
        <Link onClick={onClose} role="menuitem" to={projectManagePath(currentProjectId)}><span>项目管理</span><small>身份与共享上下文</small></Link>
        <Link onClick={onClose} role="menuitem" to={strategyPath(currentProjectId)}><span>需求与策略</span><small>Brief 与策略交付</small></Link>
        <Link onClick={onClose} role="menuitem" to={creativeTasksPath(currentProjectId)}><span>创意创作</span><small>图文任务与交付</small></Link>
        {canInsights ? <Link onClick={onClose} role="menuitem" to={insightsPath(currentProjectId)}><span>素材洞察</span><small>投前引用与投后复盘</small></Link> : null}
        {canDelivery ? <Link onClick={onClose} role="menuitem" to={deliveryPath(currentProjectId)}><span>智能投放</span><small>计划、审批与执行证据</small></Link> : null}
      </div>
    </> : null}
    <div className="project-menu__divider" />
    <button className="project-menu__create" onClick={onCreate} role="menuitem" type="button">
      <span aria-hidden="true">＋</span><span>新建项目</span>
    </button>
  </div>
}

export function Workspace() {
  const location = useLocation()
  const navigate = useNavigate()
  const projectId = routeProjectId(location.pathname)
  const activeModule = activeBusinessModule(location.pathname)
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
      setBootstrapError(error instanceof Error ? error.message : '身份与项目列表加载失败。')
    })
    return () => controller.abort()
  }, [])

  const currentProject = projects.find((project) => project.id === projectId) ?? (projectId ? undefined : projects[0])
  const currentProjectId = projectId || currentProject?.id || ''
  const currentStrategyPath = currentProjectId ? strategyPath(currentProjectId) : '/projects'
  const currentCreativePath = currentProjectId ? creativeTasksPath(currentProjectId) : '/projects'
  const currentInsightsPath = currentProjectId ? insightsPath(currentProjectId) : '/projects'
  const currentDeliveryPath = currentProjectId ? deliveryPath(currentProjectId) : '/projects'
  const canInsights = Boolean(identity?.actor.scopes.includes('insights.read'))
  const canDelivery = Boolean(identity?.actor.scopes.includes('delivery.read'))

  return <div className="app-shell">
    <header className="topbar">
      <div className="topbar__primary">
        <Link className="wordmark" to="/projects" aria-label="cookies 项目中心"><span className="wordmark__symbol" aria-hidden="true">◎</span>cookies</Link>
        <nav className="global-nav" aria-label="全局业务系统">
          <NavLink className={activeModule === 'projects' ? 'global-nav__item global-nav__item--active' : 'global-nav__item'} to="/projects"><Icon name="home" /><span>项目</span></NavLink>
          <NavLink className={activeModule === 'strategy' ? 'global-nav__item global-nav__item--active' : 'global-nav__item'} to={currentStrategyPath}><Icon name="target" /><span>需求与策略</span></NavLink>
          <NavLink aria-label="创意" className={activeModule === 'creative' ? 'global-nav__item global-nav__item--active' : 'global-nav__item'} to={currentCreativePath}><Icon name="pen" /><span>创意创作</span></NavLink>
          {canInsights ? <NavLink aria-label="洞察" className={activeModule === 'insights' ? 'global-nav__item global-nav__item--active' : 'global-nav__item'} to={currentInsightsPath}><Icon name="chart" /><span>素材洞察</span></NavLink> : null}
          {canDelivery ? <NavLink aria-label="投放" className={activeModule === 'delivery' ? 'global-nav__item global-nav__item--active' : 'global-nav__item'} to={currentDeliveryPath}><Icon name="send" /><span>智能投放</span></NavLink> : null}
        </nav>
        <div className="project-switcher">
          <button aria-expanded={projectMenuOpen} className="project-trigger" onClick={() => setProjectMenuOpen((open) => !open)} type="button">
            <span>{currentProject?.name || '选择项目'}</span><span className="chevron" aria-hidden="true">⌄</span>
          </button>
          {projectMenuOpen ? <ProjectMenu
            currentProject={currentProject}
            currentProjectId={currentProjectId}
            canDelivery={canDelivery}
            canInsights={canInsights}
            location={location.pathname}
            onClose={() => setProjectMenuOpen(false)}
            onCreate={() => {
              setProjectMenuOpen(false)
              setProjectDialogOpen(true)
            }}
            projects={projects}
          /> : null}
        </div>
      </div>
      <div className="topbar__utilities">
        <Link aria-label="管理" className="utility-button" title="组织与访问" to="/admin"><Icon name="settings" /></Link>
        <UserMenu identity={identity} />
      </div>
    </header>
    <div className="shell-body">
      <ShellSidebar canDelivery={canDelivery} canInsights={canInsights} currentProjectId={currentProjectId} module={activeModule} />
      <div className="workspace-frame">
        {bootstrapError ? <div className="workspace-alert" role="alert">身份与项目列表暂不可用：{bootstrapError}</div> : null}
        <main className="workspace">
          <Routes>
            <Route path="/projects" element={<ProjectsPage projects={projects} />} />
            <Route path="/projects/:projectId/home" element={<ProjectHomePage project={currentProject} />} />
            <Route path="/projects/:projectId/manage" element={<ProjectManagePage project={currentProject} />} />
            <Route path="/strategy" element={<StrategyLanding project={currentProject} />} />
            <Route path="/projects/:projectId/strategy" element={<ProjectStrategyRedirect />} />
            <Route path="/projects/:projectId/strategy/workspaces" element={<StrategyProjectHome project={currentProject} />} />
            <Route path="/projects/:projectId/strategy/workspaces/:workspaceId/:stage" element={<StrategyWorkspacePage project={currentProject} />} />
            <Route path="/projects/:projectId/strategy/reviews/:reviewId" element={<StrategyReviewPage />} />
            <Route path="/projects/:projectId/strategy/packages/:packageId" element={<StrategyPackagePage />} />
            <Route path="/strategy/projects/:projectId" element={<LegacyStrategyRedirect />} />
            <Route path="/strategy/projects/:projectId/workspaces/:workspaceId/*" element={<LegacyStrategyWorkspaceRedirect />} />
            <Route path="/projects/:projectId/assets" element={<ProjectAssetsPage project={currentProject} />} />
            <Route path="/projects/:projectId/assets/remix" element={<ProjectAssetRemixPage />} />
            <Route path="/projects/:projectId/assets/:assetId/versions/:version/edit" element={<ProjectAssetEditPage />} />
            <Route path="/projects/:projectId/provider-jobs" element={<ProviderJobsPage />} />
            <Route path="/projects/:projectId/creative/tasks" element={<CreativeImageTextPage />} />
            <Route path="/projects/:projectId/creative/tasks/:taskId/:creativeStage" element={<CreativeImageTextPage />} />
            <Route path="/projects/:projectId/creative" element={<LegacyCreativeRedirect />} />
            <Route path="/projects/:projectId/delivery/plans" element={canDelivery ? <DeliveryWorkspacePage project={currentProject} view="plans" /> : <ForbiddenPage />} />
            <Route path="/projects/:projectId/delivery/plans/:planId" element={canDelivery ? <DeliveryWorkspacePage project={currentProject} view="plans" /> : <ForbiddenPage />} />
            <Route path="/projects/:projectId/delivery/monitoring" element={canDelivery ? <DeliveryWorkspacePage project={currentProject} view="monitoring" /> : <ForbiddenPage />} />
            <Route path="/projects/:projectId/delivery/accounts" element={canDelivery ? <DeliveryWorkspacePage project={currentProject} view="accounts" /> : <ForbiddenPage />} />
            <Route path="/projects/:projectId/delivery/optimization" element={canDelivery ? <DeliveryWorkspacePage project={currentProject} view="optimization" /> : <ForbiddenPage />} />
            <Route path="/projects/:projectId/insights/prelaunch" element={canInsights ? <InsightsWorkspacePage project={currentProject} view="prelaunch" /> : <ForbiddenPage />} />
            <Route path="/projects/:projectId/insights/performance" element={canInsights ? <InsightsWorkspacePage project={currentProject} view="performance" /> : <ForbiddenPage />} />
            <Route path="/projects/:projectId/insights/reports" element={canInsights ? <InsightsWorkspacePage project={currentProject} view="reports" /> : <ForbiddenPage />} />
            <Route path="/projects/:projectId/insights/experiences" element={canInsights ? <InsightsWorkspacePage project={currentProject} view="experiences" /> : <ForbiddenPage />} />
            <Route path="/account/profile" element={<AccountPage identity={identity} view="profile" />} />
            <Route path="/account/security" element={<AccountPage identity={identity} view="security" />} />
            <Route path="/account/preferences" element={<AccountPage identity={identity} view="preferences" />} />
            <Route path="/admin" element={<IdentityOrganizationPage identity={identity} projects={projects} />} />
            <Route path="*" element={<NotFoundPage />} />
          </Routes>
        </main>
      </div>
    </div>
    <NewProjectDialog
      onClose={() => setProjectDialogOpen(false)}
      onCreated={(project) => {
        setProjects((current) => [...current, project])
        setProjectDialogOpen(false)
        navigate(projectHomePath(project.id))
      }}
      open={projectDialogOpen}
    />
  </div>
}

function LegacyStrategyRedirect() {
  const { projectId = '' } = useParams()
  return <Navigate replace to={strategyPath(projectId)} />
}

function ProjectStrategyRedirect() {
  const { projectId = '' } = useParams()
  return <Navigate replace to={strategyPath(projectId)} />
}

function LegacyStrategyWorkspaceRedirect() {
  const { projectId = '', workspaceId = '', '*': legacyStage = '' } = useParams()
  const stage = ['conversation', 'brief', 'strategy'].includes(legacyStage) ? legacyStage : 'conversation'
  return <Navigate replace to={`/projects/${projectId}/strategy/workspaces/${workspaceId}/${stage}`} />
}

function LegacyCreativeRedirect() {
  const { projectId = '' } = useParams()
  return <Navigate replace to={creativeTasksPath(projectId)} />
}

function NotFoundPage() {
  return <section className="not-found-page">
    <span className="page-eyebrow">404</span>
    <h1>这个页面不存在</h1>
    <p>当前 URL 没有对应的真实业务页面。请从项目中心重新进入。</p>
    <Link className="button button--primary button--compact" to="/projects">返回项目中心</Link>
  </section>
}

function ForbiddenPage() {
  const { projectId = '' } = useParams()
  return <section className="not-found-page">
    <span className="page-eyebrow">403</span>
    <h1>当前账号没有访问权限</h1>
    <p>导航已按身份范围收敛；如果这是工作需要，请联系组织管理员补充对应模块权限。</p>
    <Link className="button button--primary button--compact" to={projectId ? projectHomePath(projectId) : '/projects'}>{projectId ? '返回项目' : '返回项目中心'}</Link>
  </section>
}

export function LegacyProviderJobsPlaceholder() {
  const projectId = routeProjectId(useLocation().pathname)
  return <section className="not-found-page"><h1>Provider Jobs</h1><p>Project: {projectId}</p></section>
}
