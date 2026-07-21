import { useState } from 'react'
import { Icon } from './shell/Icon'
import { adminModule, shellModules, type ModuleKey } from './shell/modules'

const projectNames = ['示例项目', '等待 Project 模块接入']

function App() {
  const [selectedModule, setSelectedModule] = useState<ModuleKey>('strategy')
  const [projectMenuOpen, setProjectMenuOpen] = useState(false)
  const [projectName, setProjectName] = useState(projectNames[0])
  const activeModule = [...shellModules, adminModule].find(({ key }) => key === selectedModule) ?? shellModules[0]

  function selectProject(name: string) {
    setProjectName(name)
    setProjectMenuOpen(false)
  }

  return (
    <div className="app-shell">
      <aside className="sidebar" aria-label="系统导航">
        <a className="wordmark" href="/" aria-label="cookies 首页">cookies</a>
        <nav className="module-nav" aria-label="业务系统">
          {shellModules.map((module) => (
            <button
              className={module.key === selectedModule ? 'nav-item nav-item--active' : 'nav-item'}
              key={module.key}
              onClick={() => setSelectedModule(module.key)}
              type="button"
            >
              <Icon name={module.icon} />
              <span>{module.label}</span>
            </button>
          ))}
        </nav>
        <button
          className={selectedModule === adminModule.key ? 'nav-item nav-item--active nav-item--bottom' : 'nav-item nav-item--bottom'}
          onClick={() => setSelectedModule(adminModule.key)}
          type="button"
        >
          <Icon name={adminModule.icon} />
          <span>{adminModule.label}</span>
        </button>
      </aside>

      <div className="workspace-frame">
        <header className="topbar">
          <div className="project-switcher">
            <button
              aria-expanded={projectMenuOpen}
              className="project-trigger"
              onClick={() => setProjectMenuOpen((open) => !open)}
              type="button"
            >
              <span className="project-folder" aria-hidden="true" />
              <span>{projectName}</span>
              <span className="chevron" aria-hidden="true">⌄</span>
            </button>
            {projectMenuOpen ? (
              <div className="project-menu" role="menu">
                {projectNames.map((name) => (
                  <button key={name} onClick={() => selectProject(name)} role="menuitem" type="button">
                    {name}
                  </button>
                ))}
              </div>
            ) : null}
          </div>
          <div className="global-status" aria-label="全局入口">
            <span>待办</span>
            <span>审批</span>
            <span>通知</span>
            <span className="avatar" aria-label="当前用户">U</span>
          </div>
        </header>

        <main className="workspace" aria-live="polite">
          <section className="empty-state">
            <div className="module-mark" aria-hidden="true"><span /><span /><span /><span /></div>
            <h1>{activeModule.label}工作区</h1>
            <p>{activeModule.description}</p>
            <p className="empty-state__hint">共享 Shell 已就绪，模块可独立接入或由项目链路串联。</p>
          </section>
        </main>
      </div>
    </div>
  )
}

export default App
