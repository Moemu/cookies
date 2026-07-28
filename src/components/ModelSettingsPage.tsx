import { useState, type FormEvent } from 'react'
import { Check, CircleAlert, KeyRound, LockKeyhole, RotateCcw, Save, ShieldCheck, Trash2 } from 'lucide-react'
import { useModelConfig } from '../context/ModelConfigContext'

export function ModelSettingsPage() {
  const { providers, configuredCount, isLoading, refresh, saveProvider, clearProvider } = useModelConfig()
  const selected = providers[0]
  const [apiKey, setApiKey] = useState('')
  const [baseUrl, setBaseUrl] = useState('https://ark.cn-beijing.volces.com/api/v3')
  const [notice, setNotice] = useState('')
  const [error, setError] = useState('')
  const [isSaving, setIsSaving] = useState(false)

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    setIsSaving(true)
    setNotice('')
    setError('')
    try {
      await saveProvider({ apiKey, baseUrl })
      setApiKey('')
      setNotice('模型密钥已保存到服务端，本页面只保留掩码状态。')
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : '保存失败')
    } finally {
      setIsSaving(false)
    }
  }

  const clear = async () => {
    setNotice('')
    setError('')
    try {
      await clearProvider()
      setNotice('已清除页面配置的工作区密钥，服务端将回退到环境变量配置。')
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : '清除失败')
    }
  }

  return <div className="model-settings-page">
    <header className="model-settings-heading">
      <div><span>组织级能力</span><h1>模型密钥与能力</h1><p>登录后可在页面直接配置模型 API Key。明文只提交到服务端一次，浏览器响应只展示掩码。</p></div>
      <div className="provider-summary"><b>{configuredCount} / {providers.length || 1}</b><span>服务已配置</span></div>
    </header>

    <div className="model-settings-layout">
      <aside className="provider-index" aria-label="模型服务商">
          <div className="provider-index-title"><KeyRound size={16}/><b>服务端 Provider</b></div>
        {providers.map(provider => <div key={provider.id} className="menu-item">
          <span className={`provider-status-dot ${provider.status === '已配置' ? 'configured' : ''}`}/>
          <span><b>{provider.name}</b><small>{provider.status}</small></span>
        </div>)}
        <div className="provider-index-note"><ShieldCheck size={16}/><span><b>凭据隔离</b><small>保存后只返回掩码；Project、导出包和审计事件不包含密钥。</small></span></div>
      </aside>

      <section className="provider-form">
        <div className="provider-form-title"><div><h2>{selected?.name ?? '正在读取能力'}</h2><p>{selected?.description ?? '等待服务端响应'}</p></div><span className={selected?.status === '已配置' ? 'config-status configured' : 'config-status'}>{selected?.status === '已配置' ? <Check size={14}/> : <CircleAlert size={14}/>} {selected?.status ?? '读取中'}</span></div>
        <div className="secret-policy"><LockKeyhole size={18}/><div><b>服务端密钥边界</b><p>输入的 API Key 会写入本地 MVP 服务端 store，并覆盖当前进程的环境变量配置。页面永远不展示完整密钥。</p></div></div>

        <form className="provider-fields" onSubmit={submit}>
          <label>Provider API Key
            <input value={apiKey} onChange={event => setApiKey(event.target.value)} type="password" placeholder={selected?.maskedApiKey ?? '输入新的 API Key'} autoComplete="off" required/>
          </label>
          <label>Base URL
            <input value={baseUrl} onChange={event => setBaseUrl(event.target.value)} placeholder="https://ark.cn-beijing.volces.com/api/v3"/>
          </label>
          <div className="provider-actions inline">
            <button className="secondary-button danger-text" type="button" onClick={() => void clear()} disabled={isSaving}><Trash2 size={15}/>清除页面配置</button>
            <button className="secondary-button" type="button" onClick={() => void refresh()} disabled={isLoading || isSaving}><RotateCcw size={15}/>{isLoading ? '检查中…' : '刷新状态'}</button>
            <button className="primary-button" type="submit" disabled={isSaving}><Save size={15}/>{isSaving ? '保存中…' : '保存密钥'}</button>
          </div>
        </form>

        {notice ? <div className="config-notice">{notice}</div> : null}
        {error ? <div className="config-notice error">{error}</div> : null}

        <div className="provider-metadata">
          <div><span>生效范围</span><b>服务端全部 Project</b></div>
          <div><span>配置来源</span><b>{selected?.source === 'workspace' ? '页面工作区配置' : selected?.source === 'environment' ? '服务端环境变量' : '尚未配置'}</b></div>
          <div><span>密钥状态</span><b>{selected?.maskedApiKey ?? '未保存'}</b></div>
          <div><span>最近检查</span><b>{selected?.lastVerifiedAt || '尚未连接'}</b></div>
          <div><span>调用策略</span><b>未配置时返回可诊断失败</b></div>
        </div>
      </section>

      <aside className="model-settings-guide">
        <h3>接入规则</h3>
        <ol><li><span>01</span><p><b>登录后配置</b>未登录不能读取或写入密钥配置。</p></li><li><span>02</span><p><b>任务按需检查</b>生成请求由服务端统一校验 Provider 状态。</p></li><li><span>03</span><p><b>掩码返回</b>响应和本地浏览器存储都不含完整密钥。</p></li></ol>
        <div className="model-settings-audit"><span>默认账号</span><b>本地 MVP 默认使用 demo@cookies.local / cookies-demo，可通过服务端环境变量覆盖。</b></div>
      </aside>
    </div>
  </div>
}
