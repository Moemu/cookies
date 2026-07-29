import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import { api, type ApiAuthSession } from '../data/api'

interface AuthValue {
  session: ApiAuthSession
  isLoading: boolean
  login: (email: string, password: string) => Promise<void>
  logout: () => Promise<void>
}

const AuthContext = createContext<AuthValue | null>(null)

const anonymousSession: ApiAuthSession = { authenticated: false }

export function AuthProvider({ children }: { children: ReactNode }) {
  const [session, setSession] = useState<ApiAuthSession>(anonymousSession)
  const [isLoading, setIsLoading] = useState(true)

  const refresh = useCallback(async () => {
    try {
      setSession(await api.getSession())
    } catch {
      setSession(anonymousSession)
    } finally {
      setIsLoading(false)
    }
  }, [])

  useEffect(() => { void refresh() }, [refresh])

  const login = useCallback(async (email: string, password: string) => {
    setSession(await api.login({ email, password }))
  }, [])

  const logout = useCallback(async () => {
    setSession(await api.logout())
  }, [])

  const value = useMemo(() => ({ session, isLoading, login, logout }), [session, isLoading, login, logout])
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth() {
  const value = useContext(AuthContext)
  if (!value) throw new Error('useAuth must be used inside AuthProvider')
  return value
}
