import { useState, type ReactNode } from 'react'
import { Bell, CheckCircle2, ChevronDown, CircleHelp, Command, Home, Menu, Search, X } from 'lucide-react'
import { systems } from '../data/navigation'
import type { SystemDefinition, SystemKey } from '../types'
import { CookiesMark } from './Icons'

interface ShellProps {
  system: SystemDefinition
  activeNav: string
  isHome: boolean
  onHome: () => void
  onSystemChange: (key: SystemKey) => void
  onNavChange: (id: string) => void
  children: ReactNode
}

export function Shell({ system, activeNav, isHome, onHome, onSystemChange, onNavChange, children }: ShellProps) {
  const [projectMenu, setProjectMenu] = useState(false)
  const [searchOpen, setSearchOpen] = useState(false)
  const groups = [...new Set(system.nav.map(item => item.group))]

  return (
    <div className={isHome ? 'app-shell home-shell' : 'app-shell'}>
      <a className="skip-link" href="#main-content">跳到主内容</a>
      <header className="topbar">
        <button className="brand" onClick={onHome} aria-label="返回 Home 首页">
          <CookiesMark className="brand-mark" />
          <span>cookies</span>
        </button>
        <nav className="system-nav" aria-label="业务系统">
          <button className={isHome ? 'system-nav-item active' : 'system-nav-item'} onClick={onHome}><Home size={15}/><span>Home</span></button>
          {systems.map(item => <button key={item.key} className={!isHome && item.key === system.key ? 'system-nav-item active' : 'system-nav-item'} onClick={() => onSystemChange(item.key)}><item.icon size={15}/><span>{item.label}</span></button>)}
        </nav>
        <span className="top-divider" />
        <div className="top-switcher-wrap">
          <button className="top-switcher project" onClick={() => setProjectMenu(v => !v)} aria-expanded={projectMenu}>
            春季新品上市<ChevronDown size={14} />
          </button>
          {projectMenu && (
            <div className="menu project-menu">
              <div className="menu-label">当前 Project</div>
              <button className="project-choice"><span className="project-code">SP</span><span><b>春季新品上市</b><small>夏季增长实验室 · 5 个活跃任务</small></span><CheckCircle2 size={16} /></button>
              <button className="menu-link">查看项目总览</button>
              <button className="menu-link">切换其他项目</button>
            </div>
          )}
        </div>
        <div className="top-spacer" />
        <div className={searchOpen ? 'global-search expanded' : 'global-search'}>
          <Search size={16} />
          <input aria-label="全局搜索" placeholder="搜索项目、内容或数据" onFocus={() => setSearchOpen(true)} />
          {searchOpen ? <button aria-label="关闭搜索" onClick={() => setSearchOpen(false)}><X size={15} /></button> : <kbd>/</kbd>}
        </div>
        <button className="icon-button" aria-label="命令中心"><Command size={18} /></button>
        <button className="icon-button" aria-label="帮助"><CircleHelp size={18} /></button>
        <button className="icon-button has-dot" aria-label="通知"><Bell size={18} /></button>
        <button className="avatar" aria-label="个人菜单">AM</button>
      </header>
      {!isHome && <aside className="sidebar" aria-label={`${system.label}导航`}>
        <div className="side-title"><system.icon size={18} /><span>{system.label}</span></div>
        <nav>
          {groups.map(group => (
            <div className="nav-group" key={group}>
              <div className="nav-group-label">{group}</div>
              {system.nav.filter(item => item.group === group).map(item => {
                const Icon = item.icon
                return <button key={item.id} className={activeNav === item.id ? 'nav-item active' : 'nav-item'} onClick={() => onNavChange(item.id)}><Icon size={17} /><span>{item.label}</span></button>
              })}
            </div>
          ))}
        </nav>
        <button className="collapse-button"><Menu size={17} /><span>收起侧栏</span></button>
      </aside>}
      <main id="main-content" className="main-content">{children}</main>
    </div>
  )
}
