import { useCallback, useEffect, useRef, useState, type FormEvent } from 'react'
import { Check, CircleAlert, LockKeyhole, RefreshCw, Save } from 'lucide-react'
import { useProject } from '../context/ProjectContext'
import { ApiRequestError, api, type ApiOceanEngineSession } from '../data/api'

type LoadState = 'loading' | 'ready' | 'error'
const formatter = new Intl.DateTimeFormat('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
const formatTime = (value?: string | Date | null) => value ? formatter.format(new Date(value)) : '暂无记录'

const statusCopy: Record<ApiOceanEngineSession['status'], { title: string; detail: string }> = {
  unverified: { title: '已保存，尚未验证', detail: '保存后执行一次只读验证，确认 Web API 会话仍可用。' },
  ready: { title: '连接正常', detail: '最近一次只读验证通过，可以继续进行授权范围内的数据读取。' },
  auth_required: { title: '需要更新会话', detail: '会话已失效或需要重新复制 Cookie，请更新后再次验证。' },
  disabled: { title: '服务端未启用', detail: '当前环境未启用巨量引擎 Web API 连接能力。' },
}

function errorMessage(error: unknown) {
  if (error instanceof ApiRequestError && error.code === 'VERSION_CONFLICT') return '数据已更新，请刷新后重试。'
  if (error instanceof ApiRequestError && error.status === 403) return '你没有管理当前 Project 巨量会话的权限。'
  return error instanceof Error ? error.message : '连接操作失败，请稍后重试。'
}

export function OceanEngineSessionSettings() {
  const { currentProject } = useProject()
  const inputRef = useRef<HTMLInputElement>(null)
  const requestRef = useRef(0)
  const [session, setSession] = useState<ApiOceanEngineSession | null>(null)
  const [loadState, setLoadState] = useState<LoadState>('loading')
  const [busy, setBusy] = useState(false)
  const [syncing, setSyncing] = useState(false)
  const [lastSyncedAt, setLastSyncedAt] = useState<Date | null>(null)
  const [notice, setNotice] = useState('')

  const load = useCallback(async (background = false) => {
    const requestId = ++requestRef.current
    if (background) setSyncing(true)
    else { setLoadState('loading'); setNotice('') }
    try {
      const next = await api.getOceanEngineSession(currentProject.id)
      if (requestId !== requestRef.current) return
      setSession(next); setLoadState('ready'); setLastSyncedAt(new Date())
    } catch (error) {
      if (requestId !== requestRef.current) return
      if (error instanceof ApiRequestError && error.status === 404) { setSession(null); setLoadState('ready') }
      else { if (!background) setLoadState('error'); setNotice(background ? `状态自动同步暂未完成：${errorMessage(error)}` : errorMessage(error)) }
    } finally { if (requestId === requestRef.current) setSyncing(false) }
  }, [currentProject.id])

  useEffect(() => { void load() }, [load])
  useEffect(() => {
    const refresh = () => { if (document.visibilityState === 'visible') void load(true) }
    window.addEventListener('focus', refresh); document.addEventListener('visibilitychange', refresh)
    return () => { window.removeEventListener('focus', refresh); document.removeEventListener('visibilitychange', refresh) }
  }, [load])

  const save = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const value = inputRef.current?.value.trim() ?? ''
    if (value.length < 8) return setNotice('请粘贴完整的 Cookie 值后再保存。')
    if (value.length > 16356) return setNotice('Cookie 值超过 16 KiB，无法安全保存；请确认只复制 Cookie 的值。')
    setBusy(true); setNotice('')
    try {
      const next = await api.updateOceanEngineSession(currentProject.id, { session: value, expected_version: session?.version ?? 0 })
      if (inputRef.current) inputRef.current.value = ''
      setSession(next); setLoadState('ready'); setLastSyncedAt(new Date())
      setNotice('会话已保存到服务端，输入框已清空。系统不会在本页回显凭据，请继续执行只读验证。')
    } catch (error) { setNotice(errorMessage(error)) } finally { setBusy(false) }
  }

  const verify = async () => {
    if (!session) return
    setBusy(true); setNotice('')
    try {
      const next = await api.verifyOceanEngineSession(currentProject.id, session.version)
      setSession(next); setLastSyncedAt(new Date())
      setNotice(next.status === 'ready' ? '已完成只读验证，连接正常。' : statusCopy[next.status].detail)
    } catch (error) { setNotice(errorMessage(error)) } finally { setBusy(false) }
  }

  const copy = session ? statusCopy[session.status] : { title: '尚未配置', detail: '请保存一个经授权的巨量引擎 Web API 会话。' }
  return <section className="miyun-connection-settings" aria-labelledby="ocean-engine-session-title">
    <div className="miyun-settings-main">
      <header><div><span>项目级安全配置</span><h2 id="ocean-engine-session-title">巨量会话凭据</h2><p>当前 Project：{currentProject.name}</p></div><div className="miyun-settings-status-group" aria-live="polite"><span className={`miyun-connection-status ${session?.status === 'ready' ? 'ready' : ''}`}>{session?.status === 'ready' ? <Check size={14} aria-hidden="true" /> : <CircleAlert size={14} aria-hidden="true" />}{loadState === 'loading' ? '正在读取' : copy.title}</span><small>{syncing ? '正在同步状态…' : `页面同步于 ${formatTime(lastSyncedAt)}`}</small></div></header>
      <div className="miyun-settings-secret-policy"><LockKeyhole size={18} /><p><b>只保存到服务端</b>{copy.detail} 这是 <code>ocean_engine / web_api</code> 连接，不是 RPA 或官方 Marketing API。Cookie 不会回显。</p></div>
      <form className="miyun-session-form" onSubmit={save}>
        <label htmlFor="ocean-engine-session">巨量引擎 Cookie 请求头</label>
        <input id="ocean-engine-session" name="ocean-engine-session" ref={inputRef} type="password" autoComplete="off" placeholder="粘贴完整 Cookie 值，仅用于本次保存…" aria-describedby="ocean-engine-session-help" />
        <small id="ocean-engine-session-help">保存后输入框会立即清空；刷新页面也无法查看已保存内容。</small>
        {session ? <dl className="miyun-connection-metadata"><div><dt>最近验证</dt><dd>{formatTime(session.last_verified_at)}</dd></div><div><dt>最近成功请求</dt><dd>{formatTime(session.last_successful_request_at)}</dd></div></dl> : null}
        <div className="miyun-settings-actions"><button className="secondary-button" type="button" onClick={() => void load(true)} disabled={busy || syncing || loadState === 'loading'}><RefreshCw size={15} aria-hidden="true" />{syncing ? '同步中' : '立即同步'}</button><button className="secondary-button" type="button" onClick={() => void verify()} disabled={busy || syncing || !session || session.status === 'disabled'}><RefreshCw size={15} aria-hidden="true" />只读验证</button><button className="primary-button" type="submit" disabled={busy}><Save size={15} aria-hidden="true" />{busy ? '处理中' : '保存会话'}</button></div>
      </form>
      {notice ? <p className="miyun-settings-notice" role="status" aria-live="polite">{notice}</p> : null}
    </div>
    <aside className="miyun-cookie-guide"><h3>获取 Cookie 的步骤</h3><ol><li><span>01</span><p>在已授权的巨量引擎控制台登录企业账号。</p></li><li><span>02</span><p>打开需要读取的页面，按 <b>F12</b> 进入 Network 面板并刷新。</p></li><li><span>03</span><p>选择一个只读请求，在请求标头中找到 <b>Cookie</b>，仅复制值。</p></li><li><span>04</span><p>只粘贴到左侧表单并保存，然后点击“只读验证”。不要把值发给他人、截图或写入文档。</p></li></ol></aside>
  </section>
}
