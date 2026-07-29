import { useState, type FormEvent } from 'react'
import { KeyRound, LockKeyhole, ShieldCheck } from 'lucide-react'
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

  return <main className="login-page">
    <section className="login-panel" aria-label="登录 cookies">
      <div className="login-brand"><CookiesMark className="brand-mark"/><span>cookies</span></div>
      <div className="login-copy">
        <span>COOKIES WORKSPACE ACCESS</span>
        <h1>登录后进入广告增长工作台。</h1>
        <p>统一访问需求与策略、创意创作、素材洞察和智能投放；身份与项目权限由 Go 平台服务校验。</p>
      </div>
      <form className="login-form" onSubmit={submit}>
        <label>账号<input value={username} onChange={event => setUsername(event.target.value)} autoComplete="username"/></label>
        <label>密码<input value={password} onChange={event => setPassword(event.target.value)} type="password" autoComplete="current-password" autoFocus/></label>
        {error ? <div className="config-notice error">{error}</div> : null}
        <button className="primary-button full" disabled={submitting || isLoading}>{submitting ? '登录中…' : '登录工作台'}</button>
      </form>
      <div className="login-guardrails">
        <span><ShieldCheck size={15}/><b>HttpOnly 会话</b><small>浏览器脚本不能读取登录 cookie。</small></span>
        <span><LockKeyhole size={15}/><b>Go 身份校验</b><small>登录由平台 API 统一验证，不依赖前端假会话。</small></span>
        <span><KeyRound size={15}/><b>项目级权限</b><small>登录后只展示当前身份有权访问的 Project。</small></span>
      </div>
    </section>
  </main>
}
