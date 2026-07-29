import { useCallback, useMemo } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
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
  accountPage?: 'profile' | 'security' | 'organization' | 'organization-members'
}

const systemKeys = new Set<SystemKey>(['strategy', 'creative', 'insight', 'delivery'])

export function parseRoute(location = `${window.location.pathname}${window.location.search}`): AppRoute {
  const url = new URL(location, window.location.origin)
  const parts = url.pathname.split('/').filter(Boolean)
  if (parts[0] === 'account' && parts[1] === 'profile') return { isHome: false, isProjectHome: false, isProjectManagement: false, isModelSettings: false, isLegacyProjectSystemRoute: false, systemKey: 'strategy', navId: 'tasks', accountPage: 'profile' }
  if (parts[0] === 'account' && parts[1] === 'security') return { isHome: false, isProjectHome: false, isProjectManagement: false, isModelSettings: false, isLegacyProjectSystemRoute: false, systemKey: 'strategy', navId: 'tasks', accountPage: 'security' }
  if (parts[0] === 'organization') return { isHome: false, isProjectHome: false, isProjectManagement: false, isModelSettings: false, isLegacyProjectSystemRoute: false, systemKey: 'strategy', navId: 'tasks', accountPage: parts[1] === 'members' ? 'organization-members' : 'organization' }
  if (parts[0] === 'settings') return { isHome: false, isProjectHome: false, isProjectManagement: false, isModelSettings: true, isLegacyProjectSystemRoute: false, systemKey: 'strategy', navId: 'tasks' }
  if (parts[0] !== 'projects' || !parts[1]) return { isHome: true, isProjectHome: false, isProjectManagement: false, isModelSettings: false, isLegacyProjectSystemRoute: false, systemKey: 'strategy', navId: 'tasks' }
  if (!parts[2] || parts[2] === 'home') return { isHome: false, isProjectHome: true, isProjectManagement: false, isModelSettings: false, isLegacyProjectSystemRoute: false, projectId: parts[1], systemKey: 'strategy', navId: 'tasks' }
  if (parts[2] === 'manage') return { isHome: false, isProjectHome: false, isProjectManagement: true, isModelSettings: false, isLegacyProjectSystemRoute: false, projectId: parts[1], systemKey: 'strategy', navId: 'tasks' }
  if (parts[2] === 'assets') {
    return { isHome: false, isProjectHome: false, isProjectManagement: false, isModelSettings: false, isLegacyProjectSystemRoute: false, projectId: parts[1], systemKey: 'creative', navId: 'assets', objectId: parts[3], view: url.searchParams.get('view') ?? undefined }
  }
  if (parts[2] === 'provider-jobs') {
    return { isHome: false, isProjectHome: false, isProjectManagement: false, isModelSettings: false, isLegacyProjectSystemRoute: false, projectId: parts[1], systemKey: 'creative', navId: 'production', view: url.searchParams.get('view') ?? undefined }
  }
  const normalizedSystem = parts[2] === 'insights' ? 'insight' : parts[2]
  const systemKey = systemKeys.has(normalizedSystem as SystemKey) ? normalizedSystem as SystemKey : 'strategy'
  return { isHome: false, isProjectHome: false, isProjectManagement: false, isModelSettings: false, isLegacyProjectSystemRoute: false, projectId: parts[1], systemKey, navId: parts[3] || 'tasks', objectId: parts[4], view: url.searchParams.get('view') ?? undefined }
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
  const location = useLocation()
  const routerNavigate = useNavigate()
  const route = useMemo(
    () => parseRoute(`${location.pathname}${location.search}`),
    [location.pathname, location.search],
  )
  const navigate = useCallback((path: string, replace = false) => {
    routerNavigate(path, { replace })
    window.scrollTo({ top: 0 })
  }, [routerNavigate])
  return { route, navigate }
}
