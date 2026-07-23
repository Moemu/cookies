import { Check, CircleAlert, KeyRound, LockKeyhole, RotateCcw, ShieldCheck } from 'lucide-react'
import { useModelConfig } from '../context/ModelConfigContext'

export function ModelSettingsPage() {
  const { providers, configuredCount, isLoading, refresh } = useModelConfig()
  const selected = providers[0]

  return <div className="model-settings-page">
    <header className="model-settings-heading">
      <div><span>组织级能力</span><h1>模型能力</h1><p>浏览器只读取服务端能力状态。凭据仅由服务端的 <code>ARK_API_KEY</code> 环境变量提供。</p></div>
      <div className="provider-summary"><b>{configuredCount} / {providers.length || 1}</b><span>服务已配置</span></div>
    </header>

    <div className="model-settings-layout">
      <aside className="provider-index" aria-label="模型服务商">
          <div className="provider-index-title"><KeyRound size={16}/><b>服务端 Provider</b></div>
        {providers.map(provider => <div key={provider.id} className="menu-item">
          <span className={`provider-status-dot ${provider.status === '已配置' ? 'configured' : ''}`}/>
          <span><b>{provider.name}</b><small>{provider.status}</small></span>
        </div>)}
        <div className="provider-index-note"><ShieldCheck size={16}/><span><b>凭据隔离</b><small>API Key 不进入浏览器、Project 数据或导出包。</small></span></div>
      </aside>

      <section className="provider-form">
        <div className="provider-form-title"><div><h2>{selected?.name ?? '正在读取能力'}</h2><p>{selected?.description ?? '等待服务端响应'}</p></div><span className={selected?.status === '已配置' ? 'config-status configured' : 'config-status'}>{selected?.status === '已配置' ? <Check size={14}/> : <CircleAlert size={14}/>} {selected?.status ?? '读取中'}</span></div>
        <div className="secret-policy"><LockKeyhole size={18}/><div><b>只读安全边界</b><p>此页面没有 API Key 输入、保存、掩码或浏览器存储。请在启动服务端的环境中配置 <code>ARK_API_KEY</code>。</p></div></div>

        <div className="provider-metadata">
          <div><span>生效范围</span><b>服务端全部 Project</b></div>
          <div><span>最近检查</span><b>{selected?.lastVerifiedAt || '尚未连接'}</b></div>
          <div><span>调用策略</span><b>未配置时返回可诊断失败</b></div>
        </div>
        <div className="provider-actions">
          <button className="secondary-button" onClick={() => void refresh()} disabled={isLoading}><RotateCcw size={15}/>{isLoading ? '检查中…' : '刷新状态'}</button>
        </div>
      </section>

      <aside className="model-settings-guide">
        <h3>接入规则</h3>
        <ol><li><span>01</span><p><b>平台先可用</b>未配置 Provider 时仍可浏览项目和已有产物。</p></li><li><span>02</span><p><b>任务按需检查</b>生成请求由服务端统一校验环境配置。</p></li><li><span>03</span><p><b>没有浏览器密钥</b>响应和本地存储都不含密钥或其片段。</p></li></ol>
        <div className="model-settings-audit"><span>安全边界</span><b>模型目录与健康状态是公开能力；凭据不会发送到浏览器。</b></div>
      </aside>
    </div>
  </div>
}
