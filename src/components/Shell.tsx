import { useEffect, useMemo, useState, type ReactNode } from 'react'
import { Bell, CheckCircle2, ChevronDown, CircleHelp, Command, Home, KeyRound, LogOut, Menu, Search, X } from 'lucide-react'
import { useAuth } from '../context/AuthContext'
import { useModelConfig } from '../context/ModelConfigContext'
import { useProject } from '../context/ProjectContext'
import { systems } from '../data/navigation'
import type { ApiAgencyWorkbench } from '../data/api'
import type { ProjectRecord, SystemDefinition, SystemKey } from '../types'
import { CookiesMark } from './Icons'

interface ShellProps {
  system: SystemDefinition
  activeNav: string
  isHome: boolean
  isProjectHome: boolean
  isProjectManagement: boolean
  isGlobalSettings: boolean
  onHome: () => void
  onModelSettings: () => void
  onSystemChange: (key: SystemKey) => void
  onProjectChange: (id: string, system?: SystemKey, navId?: string, objectId?: string, view?: string) => void
  onProjectManage: (id: string) => void
  onNavChange: (id: string) => void
  children: ReactNode
}

export function Shell({ system, activeNav, isHome, isProjectHome, isProjectManagement, isGlobalSettings, onHome, onModelSettings, onSystemChange, onProjectChange, onProjectManage, onNavChange, children }: ShellProps) {
  const { projects, currentProject, agencyWorkbench } = useProject()
  const { session, logout } = useAuth()
  const { configuredCount } = useModelConfig()
  const [projectMenu, setProjectMenu] = useState(false)
  const [projectMenuSearch, setProjectMenuSearch] = useState('')
  const [searchOpen, setSearchOpen] = useState(false)
  const [search, setSearch] = useState('')
  const [collapsed, setCollapsed] = useState(false)
  const [recentProjectIds, setRecentProjectIds] = useState<string[]>(() => readRecentProjectIds())
  const groups = [...new Set(system.nav.map(item => item.group))]
  const projectMenuItems = useMemo(() => buildProjectMenuItems(projects, agencyWorkbench), [projects, agencyWorkbench])
  const currentContext = projectMenuItems.find(item => item.project.id === currentProject.id)
  const projectMenuQuery = projectMenuSearch.trim().toLowerCase()
  const filteredProjectItems = useMemo(() => {
    if (!projectMenuQuery) return projectMenuItems
    return projectMenuItems.filter(item => item.searchText.includes(projectMenuQuery))
  }, [projectMenuItems, projectMenuQuery])
  const recentProjectItems = useMemo(() => recentProjectIds
    .map(id => projectMenuItems.find(item => item.project.id === id))
    .filter((item): item is ProjectMenuItem => Boolean(item))
    .slice(0, 4), [projectMenuItems, recentProjectIds])
  const groupedProjectItems = useMemo(() => groupProjectMenuItems(filteredProjectItems), [filteredProjectItems])
  const searchResults = useMemo(() => {
    const query = search.trim().toLowerCase()
    if (!query) return []
    return projectMenuItems.flatMap(item => [
      { id: `${item.project.id}-project`, projectId: item.project.id, title: item.project.name, meta: `${item.clientName} · ${item.brandName} · ${item.project.owner}`, projectHome: true as const },
      ...Object.values(item.project.artifacts).map(artifact => ({ id: `${item.project.id}-${artifact.key}`, projectId: item.project.id, objectId: artifact.key, title: `${artifact.label} ${artifact.version}`, meta: `${item.project.name} · ${artifact.status}`, system: artifact.key === 'creative' ? 'creative' as const : artifact.key === 'insight' ? 'insight' as const : artifact.key === 'delivery' ? 'delivery' as const : 'strategy' as const, navId: artifact.key === 'brief' ? 'briefs' : artifact.key === 'strategy' ? 'strategies' : artifact.key === 'creative' ? 'tasks' : artifact.key === 'insight' ? 'knowledge' : 'plans' })),
    ]).filter(result => {
      const item = projectMenuItems.find(candidate => candidate.project.id === result.projectId)
      return `${result.title} ${result.meta} ${item?.searchText ?? ''}`.toLowerCase().includes(query)
    }).slice(0, 6)
  }, [projectMenuItems, search])

  useEffect(() => {
    if (!currentProject.id) return
    setRecentProjectIds(current => {
      const next = [currentProject.id, ...current.filter(id => id !== currentProject.id)].slice(0, 6)
      writeRecentProjectIds(next)
      return next
    })
  }, [currentProject.id])

  const chooseProject = (projectId: string) => {
    onProjectChange(projectId, system.key)
    setProjectMenu(false)
    setProjectMenuSearch('')
  }
  const userLabel = session.user?.displayName ?? session.user?.email ?? 'Local User'
  const userInitials = userLabel.split(/\s+|@/).filter(Boolean).slice(0, 2).map(part => part[0]?.toUpperCase()).join('') || 'LU'

  const withoutSidebar = isHome || isProjectHome || isProjectManagement || isGlobalSettings
  return <div className={`${withoutSidebar ? 'app-shell home-shell' : 'app-shell'}${collapsed && !withoutSidebar ? ' sidebar-collapsed' : ''}`}>
    <a className="skip-link" href="#main-content">跳到主内容</a>
    <header className="topbar">
      <button className="brand" onClick={onHome} aria-label="返回 Home 首页"><CookiesMark className="brand-mark"/><span>cookies</span></button>
      <nav className="system-nav" aria-label="业务系统">
        <button className={isHome || isProjectHome || isProjectManagement ? 'system-nav-item active' : 'system-nav-item'} onClick={onHome}><Home size={15}/><span>Home</span></button>
        {systems.map(item => <button key={item.key} className={!isHome && !isProjectHome && !isProjectManagement && item.key === system.key ? 'system-nav-item active' : 'system-nav-item'} onClick={() => onSystemChange(item.key)}><item.icon size={15}/><span>{item.label}</span></button>)}
      </nav>
      <span className="top-divider"/>
      {!isGlobalSettings ? <div className="top-switcher-wrap">
        <button className="top-switcher project" onClick={() => setProjectMenu(value => !value)} aria-expanded={projectMenu} aria-haspopup="menu"><span>{currentProject.name}</span><ChevronDown size={14}/></button>
        <div className="top-context-chain" aria-label="当前组织、客户、品牌、Project 和系统上下文">
          <span>{currentContext?.organizationName ?? '组织未连接'}</span>
          <span>{currentContext?.clientName ?? '客户未分配'}</span>
          <span>{currentContext?.brandName ?? currentProject.brand}</span>
          <span>{currentProject.code}</span>
          <span>{system.shortLabel}</span>
        </div>
        {projectMenu ? <div className="menu project-menu" role="menu">
          <div className="menu-label">切换 Project</div>
          <label className="project-menu-search"><Search size={14}/><input value={projectMenuSearch} onChange={event => setProjectMenuSearch(event.target.value)} placeholder="搜索客户、品牌、Project、代码或负责人"/></label>
          {!projectMenuQuery && recentProjectItems.length ? <div className="project-menu-section">
            <div className="project-menu-section-label">最近访问</div>
            {recentProjectItems.map(item => <ProjectMenuButton key={`recent-${item.project.id}`} item={item} activeProjectId={currentProject.id} onChoose={chooseProject}/>)}
          </div> : null}
          {groupedProjectItems.length ? groupedProjectItems.map(clientGroup => <div className="project-client-group" key={clientGroup.clientName}>
            <div className="project-client-title"><b>{clientGroup.clientName}</b><small>{clientGroup.clientCode}</small></div>
            {clientGroup.brands.map(brandGroup => <div className="project-brand-group" key={`${clientGroup.clientName}-${brandGroup.brandName}`}>
              <div className="project-brand-title">{brandGroup.brandName}<span>{brandGroup.brandCode}</span></div>
              {brandGroup.items.map(item => <ProjectMenuButton key={item.project.id} item={item} activeProjectId={currentProject.id} onChoose={chooseProject}/>)}
            </div>)}
          </div>) : <div className="project-menu-empty">没有匹配的 Project</div>}
          <button className="menu-link" role="menuitem" onClick={() => { onProjectChange(currentProject.id); setProjectMenu(false); setProjectMenuSearch('') }}>查看项目工作台</button>
          <button className="menu-link" role="menuitem" onClick={() => { onProjectManage(currentProject.id); setProjectMenu(false) }}>管理当前项目</button>
        </div> : null}
      </div> : <div className="top-context-label"><KeyRound size={15}/>组织级配置</div>}
      <div className="top-spacer"/>
      <div className={searchOpen ? 'global-search expanded' : 'global-search'}>
        <Search size={16}/><input aria-label="全局搜索" placeholder="搜索项目、内容或数据" value={search} onChange={event => setSearch(event.target.value)} onFocus={() => setSearchOpen(true)}/>
        {searchOpen ? <button aria-label="关闭搜索" onClick={() => { setSearchOpen(false); setSearch('') }}><X size={15}/></button> : <kbd>/</kbd>}
        {searchOpen && search ? <div className="search-results" role="listbox" aria-label="全局搜索结果">{searchResults.length ? searchResults.map(result => <button key={result.id} role="option" onClick={() => { if ('projectHome' in result) onProjectChange(result.projectId); else onProjectChange(result.projectId, result.system, result.navId, result.objectId); setSearch(''); setSearchOpen(false) }}><b>{result.title}</b><small>{result.meta}</small></button>) : <div className="search-empty">没有匹配结果</div>}</div> : null}
      </div>
      <button className={isGlobalSettings ? 'icon-button active' : configuredCount ? 'icon-button' : 'icon-button has-warning'} aria-label="模型与密钥设置" onClick={onModelSettings}><KeyRound size={18}/></button><button className="icon-button" aria-label="命令中心"><Command size={18}/></button><button className="icon-button" aria-label="帮助"><CircleHelp size={18}/></button><button className="icon-button has-dot" aria-label="通知"><Bell size={18}/></button><button className="avatar" aria-label={`当前用户：${userLabel}`}>{userInitials}</button><button className="icon-button" aria-label="退出登录" onClick={() => void logout()}><LogOut size={17}/></button>
    </header>
    {!withoutSidebar ? <aside className="sidebar" aria-label={`${system.label}导航`}>
      <div className="side-title"><system.icon size={18}/><span>{system.label}</span></div>
      <nav>{groups.map(group => <div className="nav-group" key={group}><div className="nav-group-label">{group}</div>{system.nav.filter(item => item.group === group).map(item => <button key={item.id} className={activeNav === item.id ? 'nav-item active' : 'nav-item'} onClick={() => onNavChange(item.id)} aria-label={collapsed ? item.label : undefined}><item.icon size={17}/><span>{item.label}</span></button>)}</div>)}</nav>
      <button className="collapse-button" aria-label={collapsed ? '展开侧栏' : '收起侧栏'} onClick={() => setCollapsed(value => !value)}><Menu size={17}/><span>{collapsed ? '展开侧栏' : '收起侧栏'}</span></button>
    </aside> : null}
    <main id="main-content" className="main-content">{children}</main>
  </div>
}

interface ProjectMenuItem {
  project: ProjectRecord
  organizationName: string
  clientName: string
  clientCode: string
  brandName: string
  brandCode: string
  searchText: string
}

interface ProjectClientGroup {
  clientName: string
  clientCode: string
  brands: Array<{
    brandName: string
    brandCode: string
    items: ProjectMenuItem[]
  }>
}

function ProjectMenuButton({ item, activeProjectId, onChoose }: { item: ProjectMenuItem; activeProjectId: string; onChoose: (projectId: string) => void }) {
  const { project } = item
  return <button role="menuitem" className={project.id === activeProjectId ? 'project-choice' : 'menu-item'} onClick={() => onChoose(project.id)}>
    <span className="project-code">{project.code}</span>
    <span>
      <b>{project.name}</b>
      <small>{project.status} · {project.stage} · {project.owner} · {project.updatedAt.slice(5)}</small>
    </span>
    {project.id === activeProjectId ? <CheckCircle2 size={16}/> : null}
  </button>
}

function buildProjectMenuItems(projects: ProjectRecord[], workbench: ApiAgencyWorkbench | null): ProjectMenuItem[] {
  return projects.map(project => {
    const agencyProject = workbench?.projects.find(item => item.id === project.id)
      ?? workbench?.projects.find(item => item.runtime.code === project.code)
      ?? workbench?.projects.find(item => item.name === project.name && item.brand === project.brand)
    const organization = workbench?.organizations.find(item => item.id === agencyProject?.organizationId) ?? workbench?.organizations[0]
    const brand = workbench?.brands.find(item => item.id === agencyProject?.brandId)
      ?? workbench?.brands.find(item => item.name === project.brand)
    const client = workbench?.clients.find(item => item.id === agencyProject?.clientId)
      ?? workbench?.clients.find(item => item.id === brand?.clientId)
    const organizationName = organization?.name ?? '组织未连接'
    const clientName = client?.name ?? '客户未分配'
    const clientCode = client?.code ?? 'UNASSIGNED'
    const brandName = brand?.name ?? project.brand
    const brandCode = brand?.code ?? project.brand
    return {
      project,
      organizationName,
      clientName,
      clientCode,
      brandName,
      brandCode,
      searchText: `${organizationName} ${clientName} ${clientCode} ${brandName} ${brandCode} ${project.name} ${project.code} ${project.owner}`.toLowerCase(),
    }
  })
}

function groupProjectMenuItems(items: ProjectMenuItem[]): ProjectClientGroup[] {
  return items.reduce<ProjectClientGroup[]>((clientGroups, item) => {
    let clientGroup = clientGroups.find(group => group.clientName === item.clientName)
    if (!clientGroup) {
      clientGroup = { clientName: item.clientName, clientCode: item.clientCode, brands: [] }
      clientGroups.push(clientGroup)
    }
    let brandGroup = clientGroup.brands.find(group => group.brandName === item.brandName)
    if (!brandGroup) {
      brandGroup = { brandName: item.brandName, brandCode: item.brandCode, items: [] }
      clientGroup.brands.push(brandGroup)
    }
    brandGroup.items.push(item)
    return clientGroups
  }, [])
}

const recentProjectIdsKey = 'cookies.recent-project-ids.v1'

function readRecentProjectIds(): string[] {
  try {
    const parsed = JSON.parse(window.localStorage.getItem(recentProjectIdsKey) ?? '[]') as unknown
    return Array.isArray(parsed) ? parsed.filter((item): item is string => typeof item === 'string') : []
  } catch {
    return []
  }
}

function writeRecentProjectIds(ids: string[]) {
  try {
    window.localStorage.setItem(recentProjectIdsKey, JSON.stringify(ids))
  } catch {
    // Recent access is non-critical and should never block project switching.
  }
}
