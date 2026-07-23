import { useCallback, useEffect, useState } from 'react'
import type { SystemKey } from '../types'

export interface AppRoute {
  isHome: boolean
  isModelSettings: boolean
  projectId?: string
  systemKey: SystemKey
  navId: string
  objectId?: string
  view?: string
}

const systemKeys = new Set<SystemKey>(['strategy', 'creative', 'insight', 'delivery'])

export function parseRoute(pathname = window.location.pathname): AppRoute {
  const parts = pathname.split('/').filter(Boolean)
  if (parts[0] === 'settings' && parts[1] === 'models') return { isHome: false, isModelSettings: true, systemKey: 'strategy', navId: 'home' }
  if (parts[0] !== 'projects' || !parts[1]) return { isHome: true, isModelSettings: false, systemKey: 'strategy', navId: 'home' }
  const systemKey = systemKeys.has(parts[2] as SystemKey) ? parts[2] as SystemKey : 'strategy'
  return { isHome: false, isModelSettings: false, projectId: parts[1], systemKey, navId: parts[3] || 'home', objectId: parts[4], view: new URLSearchParams(window.location.search).get('view') ?? undefined }
}

export function projectPath(projectId: string, systemKey: SystemKey, navId = 'home', objectId?: string, view?: string) {
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
