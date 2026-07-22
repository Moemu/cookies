import { useEffect, useState } from 'react'
import { Check, ChevronRight, CircleAlert, Eye, EyeOff, KeyRound, LockKeyhole, RotateCcw, Save, ShieldCheck, Trash2 } from 'lucide-react'
import { useModelConfig, type ModelProviderId } from '../context/ModelConfigContext'

export function ModelSettingsPage() {
  const { providers, configuredCount, saveProvider, verifyProvider, removeProvider } = useModelConfig()
  const [selectedId, setSelectedId] = useState<ModelProviderId>('openai')
  const selected = providers.find(provider => provider.id === selectedId) ?? providers[0]
  const [endpoint, setEndpoint] = useState(selected.endpoint)
  const [defaultModel, setDefaultModel] = useState(selected.defaultModel)
  const [apiKey, setApiKey] = useState('')
  const [showKey, setShowKey] = useState(false)
  const [notice, setNotice] = useState('')

  useEffect(() => {
    setEndpoint(selected.endpoint)
    setDefaultModel(selected.defaultModel)
    setApiKey('')
    setShowKey(false)
    setNotice('')
  }, [selected.id, selected.defaultModel, selected.endpoint])

  const save = () => {
    if (!endpoint.trim() || !defaultModel.trim() || (!apiKey.trim() && !selected.keyHint)) {
      setNotice('请补全 API 地址、默认模型和密钥后再保存。')
      return
    }
    const result = saveProvider(selected.id, { endpoint, defaultModel, apiKey: apiKey.trim() })
    setApiKey('')
    setNotice(`${result.name} 配置已保存，密钥明文不会在页面中回显。`)
  }

  const verify = () => setNotice(verifyProvider(selected.id) ? `${selected.name} 连接校验通过。` : '尚未保存完整配置，无法校验连接。')
  const remove = () => {
    removeProvider(selected.id)
    setApiKey('')
    setNotice(`${selected.name} 密钥已移除，依赖该模型的任务将保持暂停。`)
  }

  return <div className="model-settings-page">
    <header className="model-settings-heading">
      <div><span>组织级配置</span><h1>模型与密钥</h1><p>按需连接模型服务。平台安装、启动和构建过程不读取任何模型密钥。</p></div>
      <div className="provider-summary"><b>{configuredCount} / {providers.length}</b><span>服务已配置</span></div>
    </header>

    <div className="model-settings-layout">
      <aside className="provider-index" aria-label="模型服务商">
        <div className="provider-index-title"><KeyRound size={16}/><b>模型服务</b></div>
        {providers.map(provider => <button key={provider.id} className={provider.id === selected.id ? 'active' : ''} onClick={() => setSelectedId(provider.id)}>
          <span className={`provider-status-dot ${provider.status === '已配置' ? 'configured' : ''}`}/>
          <span><b>{provider.name}</b><small>{provider.status}</small></span><ChevronRight size={15}/>
        </button>)}
        <div className="provider-index-note"><ShieldCheck size={16}/><span><b>凭据隔离</b><small>密钥不属于 Project 数据，也不会进入导出包。</small></span></div>
      </aside>

      <section className="provider-form">
        <div className="provider-form-title"><div><h2>{selected.name}</h2><p>{selected.description}</p></div><span className={selected.status === '已配置' ? 'config-status configured' : 'config-status'}>{selected.status === '已配置' ? <Check size={14}/> : <CircleAlert size={14}/>} {selected.status}</span></div>

        <div className="secret-policy"><LockKeyhole size={18}/><div><b>仅在此页面配置</b><p>密钥不会写入代码、环境变量或前端构建产物。保存后只显示末四位，生产环境应由后端密钥保险库托管。</p></div></div>

        <div className="provider-fields">
          <label>API 地址<input value={endpoint} onChange={event => setEndpoint(event.target.value)} placeholder="https://api.example.com/v1" autoComplete="url"/></label>
          <label>默认模型<input value={defaultModel} onChange={event => setDefaultModel(event.target.value)} placeholder="输入模型 ID" autoComplete="off"/></label>
          <label className="key-field">API Key<div><input type={showKey ? 'text' : 'password'} value={apiKey} onChange={event => setApiKey(event.target.value)} placeholder={selected.keyHint || '输入后保存，页面不会再次显示明文'} autoComplete="new-password"/><button type="button" aria-label={showKey ? '隐藏密钥' : '显示密钥'} onClick={() => setShowKey(value => !value)}>{showKey ? <EyeOff size={16}/> : <Eye size={16}/>}</button></div><small>{selected.keyHint ? `当前密钥：${selected.keyHint}` : '尚未保存密钥'}</small></label>
        </div>

        <div className="provider-metadata">
          <div><span>生效范围</span><b>当前组织的全部 Project</b></div>
          <div><span>最近校验</span><b>{selected.lastVerifiedAt || '尚未校验'}</b></div>
          <div><span>调用策略</span><b>未配置时暂停任务，不阻断平台启动</b></div>
        </div>

        {notice ? <div className={notice.includes('请补全') || notice.includes('无法') ? 'config-notice error' : 'config-notice'} role="status">{notice}</div> : null}
        <div className="provider-actions">
          {selected.keyHint ? <button className="secondary-button danger-text" onClick={remove}><Trash2 size={15}/>移除密钥</button> : null}
          <button className="secondary-button" onClick={verify}><RotateCcw size={15}/>校验连接</button>
          <button className="primary-button" onClick={save}><Save size={15}/>保存配置</button>
        </div>
      </section>

      <aside className="model-settings-guide">
        <h3>接入规则</h3>
        <ol><li><span>01</span><p><b>平台先可用</b>没有密钥时仍可浏览、编辑和使用 Mock 数据。</p></li><li><span>02</span><p><b>任务按需检查</b>只有调用模型的动作会检查对应服务是否已配置。</p></li><li><span>03</span><p><b>密钥不回显</b>保存后仅展示末四位和最近校验时间。</p></li></ol>
        <div className="model-settings-audit"><span>配置审计</span><b>保存、校验和移除操作都会进入组织审计记录。</b></div>
      </aside>
    </div>
  </div>
}
