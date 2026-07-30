import { useState, type FormEvent } from 'react'
import { KeyRound, LockKeyhole, ShieldCheck } from 'lucide-react'
import { useAuth } from '../context/AuthContext'
import { CookiesMark } from './Icons'

export function LoginPage() {
  const { isLoading, login } = useAuth()
  const [email, setEmail] = useState('demo@cookies.local')
  const [password, setPassword] = useState('cookies-demo')
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    setSubmitting(true)
    setError('')
    try {
      await login(email, password)
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : '登录失败')
    } finally {
      setSubmitting(false)
    }
  }

  return <main className="login-page">
    <section className="login-panel" aria-label="登录 cookies">
      <div className="login-brand"><CookiesMark className="brand-mark"/><span>cookies</span></div>
      <div className="login-copy">
        <span>Local MVP Access</span>
        <h1>先登录，再配置模型密钥。</h1>
        <p>登录用于保护页面上的 API Key 写入操作。密钥只提交到本地 MVP 服务端，前端只展示掩码状态。</p>
      </div>
      <form className="login-form" onSubmit={submit}>
        <label>邮箱<input value={email} onChange={event => setEmail(event.target.value)} autoComplete="username"/></label>
        <label>密码<input value={password} onChange={event => setPassword(event.target.value)} type="password" autoComplete="current-password"/></label>
        {error ? <div className="config-notice error">{error}</div> : null}
        <button className="primary-button full" disabled={submitting || isLoading}>{submitting ? '登录中…' : '登录工作台'}</button>
      </form>
      <div className="login-guardrails">
        <span><ShieldCheck size={15}/><b>HttpOnly 会话</b><small>浏览器脚本不能读取登录 cookie。</small></span>
        <span><LockKeyhole size={15}/><b>密钥不回显</b><small>保存后只返回末 4 位掩码。</small></span>
        <span><KeyRound size={15}/><b>本地演示账号</b><small>可用环境变量覆盖默认邮箱和密码。</small></span>
      </div>
    </section>
  </main>
}
