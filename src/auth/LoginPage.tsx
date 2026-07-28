import { useState, type FormEvent } from 'react'
import { Navigate, useLocation, useNavigate } from 'react-router-dom'
import { apiRequest } from '../backend/platform'
import './auth.css'

function safeReturnTo(value: unknown) {
  return typeof value === 'string' && value.startsWith('/') && !value.startsWith('//') ? value : '/'
}

export function LoginPage() {
  const location = useLocation()
  const navigate = useNavigate()
  const [username, setUsername] = useState('Admin')
  const [password, setPassword] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  if (location.pathname !== '/login') return <Navigate replace to="/login" />

  async function submit(event: FormEvent) {
    event.preventDefault()
    setBusy(true)
    setError('')
    try {
      await apiRequest('/platform/v1/auth/login', {
        method: 'POST',
        body: JSON.stringify({ username, password }),
      })
      const state = location.state as { returnTo?: unknown } | null
      navigate(safeReturnTo(state?.returnTo), { replace: true })
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : '登录失败，请稍后重试。')
    } finally {
      setBusy(false)
    }
  }

  return <main className="login-page">
    <section className="login-brand">
      <span className="login-wordmark">cookies</span>
      <div><p className="login-eyebrow">ADVERTISING STRATEGY WORKSPACE</p><h1>从一句需求，<br />到可评审的<br />多平台策略。</h1><p>对话梳理需求、引用品牌资料，并保留每次生成与修改的依据。</p></div>
    </section>
    <section className="login-panel" aria-labelledby="login-title">
      <form onSubmit={submit}>
        <div><p className="login-eyebrow">管理端登录</p><h2 id="login-title">欢迎回来</h2><p>使用管理员账户进入工作区。</p></div>
        <label>账号<input autoComplete="username" disabled={busy} onChange={(event) => setUsername(event.target.value)} required value={username} /></label>
        <label>密码<input autoComplete="current-password" autoFocus disabled={busy} onChange={(event) => setPassword(event.target.value)} required type="password" value={password} /></label>
        {error ? <p className="form-error" role="alert">{error}</p> : null}
        <button className="primary-button" disabled={busy} type="submit">{busy ? '正在登录…' : '登录'}</button>
      </form>
    </section>
  </main>
}
