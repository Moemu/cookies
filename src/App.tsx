import { useEffect } from 'react'
import { Shell } from './components/Shell'
import { DashboardPage, HomePage, ModulePage } from './components/Pages'
import { useProject } from './context/ProjectContext'
import { systems } from './data/navigation'
import { projectPath, useAppRoute } from './lib/router'
import type { SystemKey } from './types'

export default function App() {
  const { route, navigate } = useAppRoute()
  const { currentProject, selectProject } = useProject()
  const system = systems.find(item => item.key === route.systemKey) ?? systems[0]
  const navItem = system.nav.find(item => item.id === route.navId) ?? system.nav[0]

  useEffect(() => {
    if (route.projectId) selectProject(route.projectId)
  }, [route.projectId, selectProject])

  const changeSystem = (next: SystemKey) => navigate(projectPath(currentProject.id, next))
  const openProject = (projectId: string, next: SystemKey = 'strategy', navId = 'home') => {
    selectProject(projectId)
    navigate(projectPath(projectId, next, navId))
  }

  return <Shell system={system} activeNav={navItem.id} isHome={route.isHome} onHome={() => navigate('/')} onSystemChange={changeSystem} onProjectChange={openProject} onNavChange={id => navigate(projectPath(currentProject.id, system.key, id))}>
    {route.isHome ? <HomePage onSystemChange={changeSystem} onOpenProject={openProject}/> : navItem.id === 'home' ? <DashboardPage system={system} onSystemChange={changeSystem}/> : <ModulePage key={`${system.key}-${navItem.id}`} system={system} item={navItem}/>}
  </Shell>
}
