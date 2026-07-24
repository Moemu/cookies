import { useEffect } from 'react'
import { Shell } from './components/Shell'
import { HomePage, ModulePage } from './components/Pages'
import { ProjectFlowDashboard } from './components/ProjectWorkflow'
import { ProjectManagementPage } from './components/ProjectManagementPage'
import { ModelSettingsPage } from './components/ModelSettingsPage'
import { useProject } from './context/ProjectContext'
import { systems } from './data/navigation'
import { projectHomePath, projectManagePath, projectPath, useAppRoute } from './lib/router'
import type { SystemKey } from './types'

export default function App() {
  const { route, navigate } = useAppRoute()
  const { currentProject, selectProject } = useProject()
  const system = systems.find(item => item.key === route.systemKey) ?? systems[0]
  const navItem = system.nav.find(item => item.id === route.navId) ?? system.nav[0]

  useEffect(() => {
    if (route.projectId) selectProject(route.projectId)
  }, [route.projectId, selectProject])

  const systemLanding: Record<SystemKey, string> = { strategy: 'tasks', creative: 'tasks', insight: 'prelaunch', delivery: 'plans' }
  const changeSystem = (next: SystemKey) => navigate(projectPath(currentProject.id, next, systemLanding[next]))
  const openProject = (projectId: string, next?: SystemKey, navId?: string, objectId?: string, view?: string) => {
    selectProject(projectId)
    navigate(next ? projectPath(projectId, next, navId ?? systemLanding[next], objectId, view) : projectHomePath(projectId))
  }

  const manageProject = (projectId: string) => {
    selectProject(projectId)
    navigate(projectManagePath(projectId))
  }

  return <Shell system={system} activeNav={navItem.id} isHome={route.isHome} isProjectHome={route.isProjectHome} isProjectManagement={route.isProjectManagement} isGlobalSettings={route.isModelSettings} onHome={() => navigate('/')} onModelSettings={() => navigate('/settings/models')} onSystemChange={changeSystem} onProjectChange={openProject} onProjectManage={manageProject} onNavChange={id => navigate(projectPath(currentProject.id, system.key, id))}>
    {route.isModelSettings ? <ModelSettingsPage/> : route.isHome ? <HomePage onSystemChange={changeSystem} onOpenProject={openProject} onManageProject={manageProject}/> : route.isProjectHome ? <ProjectFlowDashboard onOpenProject={openProject} onManageProject={manageProject}/> : route.isProjectManagement ? <ProjectManagementPage onOpenWorkbench={id => openProject(id)} onOpenProject={openProject}/> : <ModulePage key={`${system.key}-${navItem.id}`} system={system} item={navItem} objectId={route.objectId} routeView={route.view} onOpenProject={openProject}/>}
  </Shell>
}
