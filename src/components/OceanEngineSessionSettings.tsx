import { useCallback, useEffect, useRef, useState, type FormEvent } from 'react'
import { Check, CircleAlert, Database, LockKeyhole, RefreshCw, Save } from 'lucide-react'
import { ApiRequestError, api, type ApiConnectorAccount, type ApiConnectorAccountSession } from '../data/api'

type LoadState = 'loading' | 'ready' | 'error'
const formatter = new Intl.DateTimeFormat('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
const formatTime = (value?: string | Date | null) => value ? formatter.format(new Date(value)) : '暂无记录'

const statusCopy: Record<ApiConnectorAccountSession['status'], { title: string; detail: string }> = {
  unverified: { title: '会话尚未验证', detail: '执行只读验证后，系统才允许同步。' },
  ready: { title: '只读连接正常', detail: '账号可以执行授权范围内的只读同步。' },
  auth_required: { title: '需要更新会话', detail: '请更新 Cookie，然后重新验证。' },
  disabled: { title: '连接已停用', detail: '当前会话不能用于同步。' },
}

const accountStatusCopy: Record<ApiConnectorAccount['status'], string> = {
  pending: '待验证',
  verified: '已验证',
  revoked: '已停用',
  blocked: '已阻止',
}

function errorMessage(error: unknown) {
  if (error instanceof ApiRequestError && error.status === 409) return '数据版本已变化。请刷新后重试。'
  if (error instanceof ApiRequestError && error.status === 422) return 'Cookie 已失效，或缺少该投放账号上下文。请从该账号成功的广告列表请求中重新复制完整 Cookie。'
  if (error instanceof ApiRequestError && error.status === 502) return '巨量引擎验证接口当前不可用。会话未改变，请稍后重试。'
  if (error instanceof ApiRequestError && error.status === 403) return '你没有管理 Organization 级 Connector 的权限。'
  return error instanceof Error ? error.message : '连接操作失败。请稍后重试。'
}

function initialSyncWindow() {
  const end = new Date()
  const start = new Date(end)
  start.setUTCDate(start.getUTCDate() - 180)
  return { start: start.toISOString(), end: end.toISOString() }
}

const wait = (milliseconds: number) => new Promise(resolve => setTimeout(resolve, milliseconds))

export function OceanEngineSessionSettings() {
  const externalIDRef = useRef<HTMLInputElement>(null)
  const labelRef = useRef<HTMLInputElement>(null)
  const sessionInputRef = useRef<HTMLInputElement>(null)
  const requestRef = useRef(0)
  const [accounts, setAccounts] = useState<ApiConnectorAccount[]>([])
  const [selectedAccountID, setSelectedAccountID] = useState('')
  const [session, setSession] = useState<ApiConnectorAccountSession | null>(null)
  const [loadState, setLoadState] = useState<LoadState>('loading')
  const [busy, setBusy] = useState(false)
  const [lastSyncedAt, setLastSyncedAt] = useState<Date | null>(null)
  const [notice, setNotice] = useState('')

  const load = useCallback(async (preferredAccountID = '') => {
    const requestID = ++requestRef.current
    setLoadState('loading')
    try {
      const response = await api.listConnectorAccounts()
      if (requestID !== requestRef.current) return
      setAccounts(response.items)
      const accountID = response.items.some(item => item.id === preferredAccountID)
        ? preferredAccountID
        : response.items[0]?.id ?? ''
      setSelectedAccountID(accountID)
      if (!accountID) {
        setSession(null)
        setLoadState('ready')
        return
      }
      try {
        const nextSession = await api.getConnectorAccountSession(accountID)
        if (requestID !== requestRef.current) return
        setSession(nextSession)
      } catch (error) {
        if (!(error instanceof ApiRequestError && error.status === 404)) throw error
        setSession(null)
      }
      setLoadState('ready')
      setLastSyncedAt(new Date())
    } catch (error) {
      if (requestID !== requestRef.current) return
      setLoadState('error')
      setNotice(errorMessage(error))
    }
  }, [])

  useEffect(() => { void load() }, [load])

  const selectAccount = async (accountID: string) => {
    setSelectedAccountID(accountID)
    setNotice('')
    if (!accountID) return setSession(null)
    setLoadState('loading')
    try {
      setSession(await api.getConnectorAccountSession(accountID))
    } catch (error) {
      if (error instanceof ApiRequestError && error.status === 404) setSession(null)
      else setNotice(errorMessage(error))
    } finally {
      setLoadState('ready')
    }
  }

  const register = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const externalID = externalIDRef.current?.value.trim() ?? ''
    const displayLabel = labelRef.current?.value.trim() ?? ''
    if (!externalID) return setNotice('请输入投放账号 ID。')
    setBusy(true); setNotice('')
    try {
      const account = await api.registerConnectorAccount({ external_id: externalID, display_label: displayLabel })
      if (externalIDRef.current) externalIDRef.current.value = ''
      if (labelRef.current) labelRef.current.value = ''
      await load(account.id)
      setNotice('账号已登记。原始账号 ID 不会在页面回显。请继续保存只读会话。')
    } catch (error) {
      setNotice(errorMessage(error))
    } finally {
      setBusy(false)
    }
  }

  const saveSession = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const value = sessionInputRef.current?.value.trim() ?? ''
    if (!selectedAccountID) return setNotice('请先登记投放账号。')
    if (value.length < 8) return setNotice('请粘贴完整 Cookie 值。')
    if (value.length > 16384) return setNotice('Cookie 超过 16 KiB。请确认只复制 Cookie 值。')
    setBusy(true); setNotice('')
    try {
      const next = await api.updateConnectorAccountSession(selectedAccountID, { session: value, expected_version: session?.version ?? 0 })
      if (sessionInputRef.current) sessionInputRef.current.value = ''
      setSession(next)
      setNotice('会话已加密保存，输入框已清空。请执行只读验证。')
    } catch (error) {
      setNotice(errorMessage(error))
    } finally {
      setBusy(false)
    }
  }

  const verify = async () => {
    if (!selectedAccountID || !session) return
    setBusy(true); setNotice('')
    try {
      await api.verifyConnectorAccount(selectedAccountID)
      const next = await api.getConnectorAccountSession(selectedAccountID)
      setSession(next)
      setAccounts(current => current.map(account => account.id === selectedAccountID ? { ...account, status: 'verified', verified_at: next.last_verified_at } : account))
      setNotice('只读验证通过。现在可以同步历史数据。')
    } catch (error) {
      setNotice(errorMessage(error))
    } finally {
      setBusy(false)
    }
  }

  const syncHistory = async () => {
    if (!selectedAccountID || session?.status !== 'ready') return
    setBusy(true); setNotice('')
    try {
      const window = initialSyncWindow()
      const idempotencyKey = `organization-history-${selectedAccountID}-${crypto.randomUUID()}`
      const result = await api.syncConnectorAccount(selectedAccountID, { ...window, time_zone: 'Asia/Shanghai', currency: 'CNY' }, idempotencyKey)
      setNotice('只读同步已进入后台。页面将持续读取同步状态。')
      for (let attempt = 0; attempt < 1_350; attempt += 1) {
        await wait(2_000)
        try {
          const status = await api.getConnectorSync(selectedAccountID, result.run_id)
          if (status.status === 'completed') {
            setNotice('只读同步已完成。对象快照和指标窗口已经写入 Connector。')
            return
          }
          if (status.status === 'failed') {
            setNotice(`只读同步失败。最后阶段：${status.cursor || '尚未取得平台数据'}。`)
            return
          }
          setNotice(`只读同步正在后台运行。当前阶段：${status.cursor || '准备平台读取'}。`)
        } catch (error) {
          if (!(error instanceof ApiRequestError && error.status === 404)) throw error
        }
      }
      setNotice('同步仍在后台运行。请稍后刷新页面查看状态。')
    } catch (error) {
      setNotice(errorMessage(error))
    } finally {
      setBusy(false)
    }
  }

  const selectedAccount = accounts.find(account => account.id === selectedAccountID)
  const copy = session ? statusCopy[session.status] : { title: '尚未保存会话', detail: '账号和会话归属 Organization，不需要业务 Project 或 Plan。' }

  return <section className="miyun-connection-settings" aria-labelledby="ocean-engine-session-title">
    <div className="miyun-settings-main">
      <header><div><span>Organization 级只读 Connector</span><h2 id="ocean-engine-session-title">巨量投放账号</h2><p>历史校准不创建占位 Project，也不建立虚假 Plan。</p></div><div className="miyun-settings-status-group" aria-live="polite"><span className={`miyun-connection-status ${session?.status === 'ready' ? 'ready' : ''}`}>{session?.status === 'ready' ? <Check size={14} aria-hidden="true" /> : <CircleAlert size={14} aria-hidden="true" />}{loadState === 'loading' ? '正在读取…' : copy.title}</span><small>页面同步于 {formatTime(lastSyncedAt)}</small></div></header>
      <div className="miyun-settings-secret-policy"><LockKeyhole size={18} aria-hidden="true" /><p><b>账号和凭据分开保存</b>{copy.detail} 原始账号 ID 和 Cookie 都不会进入校准导出。</p></div>

      <div className="oe-connector-flow">
        <section className="oe-settings-card" aria-labelledby="oe-account-section-title">
          <header className="oe-settings-card-header"><span>01</span><div><h3 id="oe-account-section-title">投放账号</h3><p>选择已有账号，或登记另一个账号。</p></div>{selectedAccount ? <strong className={`oe-account-state ${selectedAccount.status}`}>{accountStatusCopy[selectedAccount.status]}</strong> : null}</header>
          {accounts.length > 0 ? <div className="oe-account-picker">
            <label htmlFor="ocean-engine-local-account">当前账号</label>
            <select id="ocean-engine-local-account" name="ocean_engine_local_account" value={selectedAccountID} onChange={event => void selectAccount(event.target.value)} disabled={busy}>
              {accounts.map(account => <option key={account.id} value={account.id}>{account.display_label || '未命名账号'} · {accountStatusCopy[account.status]}</option>)}
            </select>
            <div className="oe-account-reference"><span>本地账号引用</span><code translate="no" title={selectedAccount?.id}>{selectedAccount?.id ?? '无'}</code></div>
          </div> : <div className="oe-account-empty"><CircleAlert size={18} aria-hidden="true" /><p><b>尚未登记账号</b><span>填写下方信息后，系统会创建匿名本地引用。</span></p></div>}
          <form className="oe-register-form" onSubmit={register}>
            <div className="oe-form-heading"><b>{accounts.length > 0 ? '登记其他账号' : '登记第一个账号'}</b></div>
            <div className="oe-register-fields"><label htmlFor="ocean-engine-account-id">投放账号 ID<input id="ocean-engine-account-id" name="ocean_engine_account_id" ref={externalIDRef} type="password" autoComplete="off" spellCheck={false} placeholder="仅用于登记，保存后不回显…" /></label><label htmlFor="ocean-engine-account-label">本地显示名称<input id="ocean-engine-account-label" name="ocean_engine_account_label" ref={labelRef} autoComplete="off" maxLength={255} placeholder="例如：历史校准账号…" /></label></div>
            <button className="secondary-button" type="submit" disabled={busy}><Save size={15} aria-hidden="true" />登记账号</button>
          </form>
        </section>

        <section className={`oe-settings-card ${selectedAccountID ? '' : 'disabled'}`} aria-labelledby="oe-session-section-title">
          <header className="oe-settings-card-header"><span>02</span><div><h3 id="oe-session-section-title">只读会话</h3><p>保存 Cookie，并验证账号读取权限。</p></div><LockKeyhole size={18} aria-hidden="true" /></header>
          <form className="miyun-session-form" onSubmit={saveSession}>
            <label htmlFor="ocean-engine-account-session">巨量引擎 Cookie 请求头</label>
            <input id="ocean-engine-account-session" name="ocean_engine_cookie" ref={sessionInputRef} type="password" autoComplete="off" spellCheck={false} placeholder={selectedAccountID ? '粘贴完整 Cookie 值，仅用于本次保存…' : '请先登记投放账号…'} disabled={!selectedAccountID || busy} />
            <small>从该账号成功的广告列表请求中复制完整 Cookie。保存后，系统立即清空输入框。</small>
            <dl className="miyun-connection-metadata"><div><dt>会话状态</dt><dd>{session ? statusCopy[session.status].title : '尚未保存'}</dd></div><div><dt>最近验证</dt><dd>{formatTime(session?.last_verified_at)}</dd></div></dl>
            <div className="miyun-settings-actions"><button className="secondary-button" type="button" onClick={() => void load(selectedAccountID)} disabled={busy || !selectedAccountID}><RefreshCw size={15} aria-hidden="true" />刷新状态</button><button className="secondary-button" type="button" onClick={() => void verify()} disabled={busy || !session}><Check size={15} aria-hidden="true" />只读验证</button><button className="primary-button" type="submit" disabled={busy || !selectedAccountID}><Save size={15} aria-hidden="true" />保存会话</button></div>
          </form>
        </section>

        <section className={`oe-sync-card ${session?.status === 'ready' ? 'ready' : ''}`} aria-labelledby="oe-sync-section-title">
          <div className="oe-sync-icon"><Database size={20} aria-hidden="true" /></div><div className="oe-sync-copy"><span>03 · 数据读取</span><h3 id="oe-sync-section-title">历史数据同步</h3><p>{session?.status === 'ready' ? '连接已就绪。同步会读取最近 180 天的完整自然日数据。' : '完成只读验证后，系统才允许同步历史数据。'}</p><div className="oe-sync-facts"><span>只读请求</span><span>7 天分窗</span><span>后台运行</span></div></div><button className="primary-button" type="button" onClick={() => void syncHistory()} disabled={busy || session?.status !== 'ready'}><Database size={15} aria-hidden="true" />同步最近 180 天</button>
        </section>
      </div>
      {notice ? <p className="miyun-settings-notice" role="status" aria-live="polite">{notice}</p> : null}
    </div>
    <aside className="miyun-cookie-guide"><h3>安全边界</h3><ol><li><span>01</span><p>账号归属 Organization，不归属业务 Project。</p></li><li><span>02</span><p>Cookie 只提交到服务端加密存储。</p></li><li><span>03</span><p>验证和同步只使用读取请求。</p></li><li><span>04</span><p>校准导出使用独立匿名密钥。</p></li></ol></aside>
  </section>
}
