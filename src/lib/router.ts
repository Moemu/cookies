import { useCallback, useEffect, useState } from 'react'
import type { SystemKey } from '../types'

export interface AppRoute {
  isHome: boolean
  isProjectHome: boolean
  isProjectManagement: boolean
  isModelSettings: boolean
  isLegacyProjectSystemRoute: boolean
  projectId?: string
  systemKey: SystemKey
  navId: string
  objectId?: string
  view?: string
}

const systemKeys = new Set<SystemKey>(['strategy', 'creative', 'insight', 'delivery'])

export function parseRoute(location = `${window.location.pathname}${window.location.search}`): AppRoute {
  const url = new URL(location, window.location.origin)
  const parts = url.pathname.split('/').filter(Boolean)
  if (parts[0] === 'settings' && parts[1] === 'models') return { isHome: false, isProjectHome: false, isProjectManagement: false, isModelSettings: true, isLegacyProjectSystemRoute: false, systemKey: 'strategy', navId: 'tasks' }
  if (parts[0] !== 'projects' || !parts[1]) return { isHome: true, isProjectHome: false, isProjectManagement: false, isModelSettings: false, isLegacyProjectSystemRoute: false, systemKey: 'strategy', navId: 'tasks' }
  if (systemKeys.has(parts[1] as SystemKey)) {
    const systemKey = parts[1] as SystemKey
    return { isHome: false, isProjectHome: false, isProjectManagement: false, isModelSettings: false, isLegacyProjectSystemRoute: true, systemKey, navId: parts[2] || defaultNavForSystem(systemKey), objectId: parts[3], view: url.searchParams.get('view') ?? undefined }
  }
  if (!parts[2] || parts[2] === 'home') return { isHome: false, isProjectHome: true, isProjectManagement: false, isModelSettings: false, isLegacyProjectSystemRoute: false, projectId: parts[1], systemKey: 'strategy', navId: 'tasks' }
  if (parts[2] === 'manage') return { isHome: false, isProjectHome: false, isProjectManagement: true, isModelSettings: false, isLegacyProjectSystemRoute: false, projectId: parts[1], systemKey: 'strategy', navId: 'tasks' }
  const systemKey = systemKeys.has(parts[2] as SystemKey) ? parts[2] as SystemKey : 'strategy'
  return { isHome: false, isProjectHome: false, isProjectManagement: false, isModelSettings: false, isLegacyProjectSystemRoute: false, projectId: parts[1], systemKey, navId: parts[3] || defaultNavForSystem(systemKey), objectId: parts[4], view: url.searchParams.get('view') ?? undefined }
}

function defaultNavForSystem(systemKey: SystemKey) {
  if (systemKey === 'insight') return 'prelaunch'
  if (systemKey === 'delivery') return 'plans'
  return 'tasks'
}

export function projectHomePath(projectId: string) {
  return `/projects/${projectId}/home`
}

export function projectManagePath(projectId: string) {
  return `/projects/${projectId}/manage`
}

export function projectPath(projectId: string, systemKey: SystemKey, navId: string, objectId?: string, view?: string) {
  const path = `/projects/${projectId}/${systemKey}/${navId}${objectId ? `/${objectId}` : ''}`
  return view ? `${path}?view=${encodeURIComponent(view)}` : path
}

export function useAppRoute() {
  const [route, setRoute] = useState<AppRoute>(() => parseRoute())
  useEffect(() => {
    const sync = () => setRoute(parseRoute())
    window.addEventListener('popstate', sync)
    return () => window.removeEventListener('popstate', sync)
  }, [])
  const navigate = useCallback((path: string, replace = false) => {
    window.history[replace ? 'replaceState' : 'pushState']({}, '', path)
    setRoute(parseRoute(path))
    window.scrollTo({ top: 0 })
  }, [])
  return { route, navigate }
}
