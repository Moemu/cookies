import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import { apiRequest, BackendApiError, type BackendIdentity } from '../backend/platform'
import type { ApiAuthSession } from '../data/api'

interface AuthValue {
  session: ApiAuthSession
  isLoading: boolean
  error: string | null
  login: (username: string, password: string) => Promise<void>
  logout: () => Promise<void>
  refresh: () => Promise<void>
  switchOrganization: (organizationId: string) => Promise<void>
}

const AuthContext = createContext<AuthValue | null>(null)

const anonymousSession: ApiAuthSession = { authenticated: false }

export function AuthProvider({ children }: { children: ReactNode }) {
  const [session, setSession] = useState<ApiAuthSession>(anonymousSession)
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const refresh = useCallback(async () => {
    setError(null)
    try {
      setSession(toSession(await apiRequest<BackendIdentity>('/platform/v1/me')))
    } catch (cause) {
      if (!(cause instanceof BackendApiError) || (cause.code !== 'UNAUTHENTICATED' && cause.status !== 401)) {
        setError(cause instanceof Error ? cause.message : '身份服务暂不可用')
        return
      }
      setSession(anonymousSession)
    } finally {
      setIsLoading(false)
    }
  }, [])

  useEffect(() => {
    void refresh()
  }, [refresh])

  const login = useCallback(async (username: string, password: string) => {
    setError(null)
    await apiRequest('/platform/v1/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    })
    setSession(toSession(await apiRequest<BackendIdentity>('/platform/v1/me')))
  }, [])

  const logout = useCallback(async () => {
    setError(null)
    await apiRequest('/platform/v1/auth/logout', { method: 'POST' })
    setSession(anonymousSession)
  }, [])

  const switchOrganization = useCallback(async (organizationId: string) => {
    setError(null)
    await apiRequest('/platform/v1/auth/switch-organization', {
      method: 'POST',
      body: JSON.stringify({ organization_id: organizationId }),
    })
    setSession(toSession(await apiRequest<BackendIdentity>('/platform/v1/me')))
    window.localStorage.removeItem('cookies.recent-project-ids.v1')
    window.localStorage.removeItem('cookies.project-system-paths.v1')
    window.location.replace('/')
  }, [])

  const value = useMemo(() => ({ session, isLoading, error, login, logout, refresh, switchOrganization }), [session, isLoading, error, login, logout, refresh, switchOrganization])
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth() {
  const value = useContext(AuthContext)
  if (!value) throw new Error('useAuth must be used inside AuthProvider')
  return value
}

function toSession(identity: BackendIdentity): ApiAuthSession {
  const displayName = identity.user?.display_name ?? identity.organization.name
  return {
    authenticated: true,
    user: {
      id: identity.user?.id ?? identity.actor.organization_id,
      displayName,
    },
    organization: {
      id: identity.organization.id,
      name: identity.organization.name,
      status: identity.organization.status,
    },
    membership: identity.membership ? {
      role: identity.membership.role,
      status: identity.membership.status,
      updatedAt: identity.membership.updated_at,
    } : undefined,
    scopes: identity.actor.scopes,
  }
}
