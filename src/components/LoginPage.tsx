import { useState, type FormEvent } from 'react'
import { LockKeyhole, ShieldCheck } from 'lucide-react'
import { Navigate, useLocation, useNavigate } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import { CookiesMark } from './Icons'

function safeReturnTo(value: unknown) {
  return typeof value === 'string' && value.startsWith('/') && !value.startsWith('//') ? value : '/'
}

export function LoginPage() {
  const location = useLocation()
  const navigate = useNavigate()
  const { isLoading, login, session } = useAuth()
  const [username, setUsername] = useState('Admin')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    setSubmitting(true)
    setError('')
    try {
      await login(username, password)
      const state = location.state as { returnTo?: unknown } | null
      navigate(safeReturnTo(state?.returnTo), { replace: true })
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : '登录失败')
    } finally {
      setSubmitting(false)
    }
  }

  if (session.authenticated) {
    const state = location.state as { returnTo?: unknown } | null
    return <Navigate replace to={safeReturnTo(state?.returnTo)} />
  }

  return <main className="auth-login-page">
    <section className="auth-login-panel" aria-label="登录 cookies">
      <div className="auth-login-brand"><CookiesMark className="brand-mark"/><span>cookies</span></div>
      <div className="auth-login-copy">
        <span>ONE PROJECT · ONE DELIVERY LOOP</span>
        <h1>把需求、决策和交付证据放回同一个项目。</h1>
        <p>登录后继续当前 Project：从需求澄清开始，经过策略评审，衔接创意、素材和投放执行。</p>
        <ol className="auth-login-flow">
          <li><b>01</b><span><strong>需求澄清</strong><small>确认目标、边界与引用证据</small></span></li>
          <li><b>02</b><span><strong>策略评审</strong><small>保留版本、意见与批准记录</small></span></li>
          <li><b>03</b><span><strong>创意与投放</strong><small>沿用同一项目上下文完成交付</small></span></li>
        </ol>
      </div>
      <form className="auth-login-form" onSubmit={submit}>
        <header><small>WORKSPACE ACCESS</small><h2>登录工作台</h2><p>使用当前环境配置的平台账号。</p></header>
        <label>账号<input value={username} onChange={event => setUsername(event.target.value)} autoComplete="username"/></label>
        <label>密码<input value={password} onChange={event => setPassword(event.target.value)} type="password" autoComplete="current-password" autoFocus/></label>
        {error ? <div className="config-notice error">{error}</div> : null}
        <button className="primary-button full" disabled={submitting || isLoading}>{submitting ? '登录中…' : '登录工作台'}</button>
        <div className="auth-login-security"><ShieldCheck size={15}/><span><b>安全会话</b><small>HttpOnly · SameSite=Strict</small></span><LockKeyhole size={15}/></div>
      </form>
    </section>
  </main>
}
