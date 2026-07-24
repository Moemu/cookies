import { useEffect, useState, type ReactNode } from 'react'
import { Navigate, useLocation } from 'react-router-dom'
import { ApiProblem } from '../shared/api/client'
import { getCurrentIdentity } from './api'

export function AuthBoundary({ children }: { children: ReactNode }) {
  const location = useLocation()
  const [status, setStatus] = useState<'checking' | 'authenticated' | 'anonymous' | 'failed'>('checking')

  useEffect(() => {
    const controller = new AbortController()
    getCurrentIdentity(controller.signal).then(() => setStatus('authenticated')).catch((error: unknown) => {
      if (error instanceof DOMException && error.name === 'AbortError') return
      if (error instanceof ApiProblem && error.problem.error.code === 'UNAUTHENTICATED') {
        setStatus('anonymous')
        return
      }
      setStatus('failed')
    })
    return () => controller.abort()
  }, [])

  if (status === 'anonymous') {
    const returnTo = `${location.pathname}${location.search}${location.hash}`
    return <Navigate replace state={{ returnTo }} to="/login" />
  }
  if (status === 'failed') {
    return <main className="auth-status"><h1>身份服务暂不可用</h1><p>请稍后刷新页面重试。</p></main>
  }
  if (status === 'checking') {
    return <main className="auth-status"><span className="strategy-spinner" /><p>正在验证登录状态…</p></main>
  }
  return children
}
