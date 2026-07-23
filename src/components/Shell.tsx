import { useMemo, useState, type ReactNode } from 'react'
import { Bell, CheckCircle2, ChevronDown, CircleHelp, Command, Home, KeyRound, Menu, Search, X } from 'lucide-react'
import { useModelConfig } from '../context/ModelConfigContext'
import { useProject } from '../context/ProjectContext'
import { systems } from '../data/navigation'
import type { SystemDefinition, SystemKey } from '../types'
import { CookiesMark } from './Icons'

interface ShellProps {
  system: SystemDefinition
  activeNav: string
  isHome: boolean
  isGlobalSettings: boolean
  onHome: () => void
  onModelSettings: () => void
  onSystemChange: (key: SystemKey) => void
  onProjectChange: (id: string, system?: SystemKey, navId?: string, objectId?: string, view?: string) => void
  onNavChange: (id: string) => void
  children: ReactNode
}

export function Shell({ system, activeNav, isHome, isGlobalSettings, onHome, onModelSettings, onSystemChange, onProjectChange, onNavChange, children }: ShellProps) {
  const { projects, currentProject } = useProject()
  const { configuredCount } = useModelConfig()
  const [projectMenu, setProjectMenu] = useState(false)
  const [searchOpen, setSearchOpen] = useState(false)
  const [search, setSearch] = useState('')
  const [collapsed, setCollapsed] = useState(false)
  const groups = [...new Set(system.nav.map(item => item.group))]
  const searchResults = useMemo(() => {
    const query = search.trim().toLowerCase()
    if (!query) return []
    return projects.flatMap(project => [
      { id: `${project.id}-project`, projectId: project.id, title: project.name, meta: `${project.brand} · Project`, system: 'strategy' as const, navId: 'home' },
      ...Object.values(project.artifacts).map(artifact => ({ id: `${project.id}-${artifact.key}`, projectId: project.id, objectId: artifact.key, title: `${artifact.label} ${artifact.version}`, meta: `${project.name} · ${artifact.status}`, system: artifact.key === 'creative' ? 'creative' as const : artifact.key === 'insight' ? 'insight' as const : artifact.key === 'delivery' ? 'delivery' as const : 'strategy' as const, navId: artifact.key === 'brief' ? 'briefs' : artifact.key === 'strategy' ? 'strategies' : artifact.key === 'creative' ? 'tasks' : artifact.key === 'insight' ? 'knowledge' : 'plans' })),
    ]).filter(item => `${item.title} ${item.meta}`.toLowerCase().includes(query)).slice(0, 6)
  }, [projects, search])

  const withoutSidebar = isHome || isGlobalSettings
  return <div className={`${withoutSidebar ? 'app-shell home-shell' : 'app-shell'}${collapsed && !withoutSidebar ? ' sidebar-collapsed' : ''}`}>
    <a className="skip-link" href="#main-content">跳到主内容</a>
    <header className="topbar">
      <button className="brand" onClick={onHome} aria-label="返回 Home 首页"><CookiesMark className="brand-mark"/><span>cookies</span></button>
      <nav className="system-nav" aria-label="业务系统">
        <button className={isHome ? 'system-nav-item active' : 'system-nav-item'} onClick={onHome}><Home size={15}/><span>Home</span></button>
        {systems.map(item => <button key={item.key} className={!isHome && item.key === system.key ? 'system-nav-item active' : 'system-nav-item'} onClick={() => onSystemChange(item.key)}><item.icon size={15}/><span>{item.label}</span></button>)}
      </nav>
      <span className="top-divider"/>
      {!isGlobalSettings ? <div className="top-switcher-wrap">
        <button className="top-switcher project" onClick={() => setProjectMenu(value => !value)} aria-expanded={projectMenu} aria-haspopup="menu">{currentProject.name}<ChevronDown size={14}/></button>
        {projectMenu ? <div className="menu project-menu" role="menu">
          <div className="menu-label">切换 Project</div>
          {projects.map(project => <button role="menuitem" key={project.id} className={project.id === currentProject.id ? 'project-choice' : 'menu-item'} onClick={() => { onProjectChange(project.id, system.key); setProjectMenu(false) }}><span className="project-code">{project.code}</span><span><b>{project.name}</b><small>{project.stage} · {project.updatedAt.slice(5)}</small></span>{project.id === currentProject.id ? <CheckCircle2 size={16}/> : null}</button>)}
          <button className="menu-link" role="menuitem" onClick={() => { onProjectChange(currentProject.id, 'strategy', 'home'); setProjectMenu(false) }}>查看项目总览</button>
        </div> : null}
      </div> : <div className="top-context-label"><KeyRound size={15}/>组织级配置</div>}
      <div className="top-spacer"/>
      <div className={searchOpen ? 'global-search expanded' : 'global-search'}>
        <Search size={16}/><input aria-label="全局搜索" placeholder="搜索项目、内容或数据" value={search} onChange={event => setSearch(event.target.value)} onFocus={() => setSearchOpen(true)}/>
        {searchOpen ? <button aria-label="关闭搜索" onClick={() => { setSearchOpen(false); setSearch('') }}><X size={15}/></button> : <kbd>/</kbd>}
        {searchOpen && search ? <div className="search-results" role="listbox" aria-label="全局搜索结果">{searchResults.length ? searchResults.map(result => <button key={result.id} role="option" onClick={() => { onProjectChange(result.projectId, result.system, result.navId, 'objectId' in result ? result.objectId : undefined); setSearch(''); setSearchOpen(false) }}><b>{result.title}</b><small>{result.meta}</small></button>) : <div className="search-empty">没有匹配结果</div>}</div> : null}
      </div>
      <button className={isGlobalSettings ? 'icon-button active' : configuredCount ? 'icon-button' : 'icon-button has-warning'} aria-label="模型与密钥设置" onClick={onModelSettings}><KeyRound size={18}/></button><button className="icon-button" aria-label="命令中心"><Command size={18}/></button><button className="icon-button" aria-label="帮助"><CircleHelp size={18}/></button><button className="icon-button has-dot" aria-label="通知"><Bell size={18}/></button><button className="avatar" aria-label="个人菜单">AM</button>
    </header>
    {!withoutSidebar ? <aside className="sidebar" aria-label={`${system.label}导航`}>
      <div className="side-title"><system.icon size={18}/><span>{system.label}</span></div>
      <nav>{groups.map(group => <div className="nav-group" key={group}><div className="nav-group-label">{group}</div>{system.nav.filter(item => item.group === group).map(item => <button key={item.id} className={activeNav === item.id ? 'nav-item active' : 'nav-item'} onClick={() => onNavChange(item.id)} aria-label={collapsed ? item.label : undefined}><item.icon size={17}/><span>{item.label}</span></button>)}</div>)}</nav>
      <button className="collapse-button" aria-label={collapsed ? '展开侧栏' : '收起侧栏'} onClick={() => setCollapsed(value => !value)}><Menu size={17}/><span>{collapsed ? '展开侧栏' : '收起侧栏'}</span></button>
    </aside> : null}
    <main id="main-content" className="main-content">{children}</main>
  </div>
}
