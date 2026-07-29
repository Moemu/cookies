import type { ReactNode } from 'react'
import { Navigate, useLocation } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'

export function AuthBoundary({ children }: { children: ReactNode }) {
  const location = useLocation()
  const { session, isLoading, error } = useAuth()

  if (error) {
    return <main className="auth-status"><h1>身份服务暂不可用</h1><p>{error}</p></main>
  }
  if (!isLoading && !session.authenticated) {
    const returnTo = `${location.pathname}${location.search}${location.hash}`
    return <Navigate replace state={{ returnTo }} to="/login" />
  }
  if (isLoading) {
    return <main className="auth-status"><p>正在验证登录状态…</p></main>
  }
  return children
}
