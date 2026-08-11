import { createContext, useContext, useMemo, type ReactNode } from 'react'
import type { StrategyWorkspaceLocation } from './workspaceRoute'

type WorkspaceRouteContextValue = {
  projectId: string
  workspaceId?: string
  location: StrategyWorkspaceLocation
  navigate: (location: StrategyWorkspaceLocation, replace?: boolean) => void
  navigateWorkspace: (workspaceId: string, location?: StrategyWorkspaceLocation, replace?: boolean) => void
}

const WorkspaceRouteContext = createContext<WorkspaceRouteContextValue | null>(null)

export function WorkspaceProvider({
  children,
  location,
  onNavigate,
  projectId,
  workspaceId,
}: {
  children: ReactNode
  location: StrategyWorkspaceLocation
  onNavigate: (workspaceId: string, location: StrategyWorkspaceLocation, replace?: boolean) => void
  projectId: string
  workspaceId?: string
}) {
  const value = useMemo<WorkspaceRouteContextValue>(() => ({
    projectId,
    workspaceId,
    location,
    navigate: (nextLocation, replace = false) => {
      if (workspaceId) onNavigate(workspaceId, nextLocation, replace)
    },
    navigateWorkspace: (nextWorkspaceId, nextLocation = location, replace = false) => {
      onNavigate(nextWorkspaceId, nextLocation, replace)
    },
  }), [location, onNavigate, projectId, workspaceId])

  return <WorkspaceRouteContext.Provider value={value}>{children}</WorkspaceRouteContext.Provider>
}

export function useStrategyWorkspaceRoute(): WorkspaceRouteContextValue {
  const value = useContext(WorkspaceRouteContext)
  if (!value) throw new Error('Strategy workspace route context is required')
  return value
}
