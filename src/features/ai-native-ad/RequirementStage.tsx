import { useEffect, useRef, useState } from 'react'
import { AlertTriangle, CheckCircle2, Image, Link2, LoaderCircle, Plus, RefreshCw, Send, Trash2 } from 'lucide-react'
import { AINativeApiError, getAssetPreview, resolveProductPreview } from './api'
import type { AINativeProductPreview, AINativeRequirement, AINativeStageStatus, RequirementMedia } from './types'

type ProductRecognitionState =
  | { status: 'idle' }
  | { status: 'checking' }
  | { status: 'success'; product: AINativeProductPreview }
  | { status: 'failed'; message: string; retryable: boolean }

function productRecognitionFailure(cause: unknown): { message: string; retryable: boolean } {
  if (cause instanceof AINativeApiError) {
    if (cause.code === 'AI_NATIVE_PRODUCT_LINK_INCOMPLETE') return { message: '复制内容不完整，商品参数在中途被截断。请回到抖音商品页重新复制并完整粘贴。', retryable: false }
    if (cause.code === 'AI_NATIVE_PRODUCT_LINK_UNSUPPORTED') return { message: '没有识别到受支持的抖音商品链接，请粘贴抖音商城分享口令或商品链接。', retryable: false }
    if (cause.code === 'AI_NATIVE_PRODUCT_DETAIL_MISSING') return { message: '这是抖音链接，但没有找到完整商品信息，可能复制的是视频链接而不是商品详情链接。', retryable: false }
    if (cause.code === 'CLIENT_TIMEOUT') return { message: '商品信息获取超时，链接格式可能正常，可以重新识别。', retryable: true }
  }
  const detail = cause instanceof Error ? cause.message.toLowerCase() : ''
  if (detail.includes('incomplete product link') || detail.includes('unexpected end')) {
    return { message: '复制内容不完整，商品参数在中途被截断。请回到抖音商品页重新复制并完整粘贴。', retryable: false }
  }
  if (detail.includes('host') || detail.includes('unsupported product link') || detail.includes('https link is required')) {
    return { message: '没有识别到受支持的抖音商品链接，请粘贴抖音商城分享口令或商品链接。', retryable: false }
  }
  if (detail.includes('goods_detail is absent') || detail.includes('product information is missing')) {
    return { message: '这是抖音链接，但没有找到完整商品信息，可能复制的是视频链接而不是商品详情链接。', retryable: false }
  }
  if (detail.includes('timeout') || detail.includes('超时') || detail.includes('network')) {
    return { message: '商品信息获取超时，链接格式可能正常，可以重新识别。', retryable: true }
  }
  return { message: '暂时无法获取商品信息，请确认链接完整后重新识别。', retryable: true }
}

export function RequirementMediaGallery({ media, previews, status, onReanalyze }: {
  media: RequirementMedia[]
  previews: Record<string, string | null>
  status: AINativeStageStatus
  onReanalyze: () => void
}) {
  const importedCount = media.filter(item => item.asset_ref).length
  const legacyCount = media.length - importedCount

  return <div className="requirement-media wide">
    <div><span>商品图片媒介</span><small>{media.length} 个素材 · {importedCount} 个已入库</small></div>
    <div className="requirement-media-items">{media.length ? media.map(item => {
      const preview = item.asset_ref ? previews[item.id] : ''
      const previewResolved = Object.hasOwn(previews, item.id)
      return <figure key={item.id}>
        {preview ? <img src={preview} alt={`${item.role} 商品素材`}/> : <div className="requirement-media-placeholder"><Image size={18}/><span>{item.asset_ref ? previewResolved ? '素材预览暂不可用' : '素材预览加载中' : '旧版链接素材，需重新提取'}</span></div>}
        <figcaption>{item.role}</figcaption>
      </figure>
    }) : <div className="media-empty"><Image size={20}/>商品链接暂未提取到图片</div>}</div>
    {legacyCount > 0 ? <div className="requirement-media-recovery"><AlertTriangle size={15}/><span>当前有 {legacyCount} 张旧版链接素材尚未进入项目素材库，源站地址可能无法稳定显示。</span><button className="secondary-button" disabled={status === 'generating'} onClick={onReanalyze}>{status === 'generating' ? <LoaderCircle className="spin" size={13}/> : null}重新提取商品素材</button></div> : null}
  </div>
}

export function RequirementStage({
  projectId,
  status,
  productLink,
  supplementalRequirement,
  requirement,
  error,
  onProductLinkChange,
  onSupplementalRequirementChange,
  onAnalyze,
  onReanalyze,
  onChange,
  onSave,
  onConfirm,
  onEdit,
}: {
  projectId: string
  status: AINativeStageStatus
  productLink: string
  supplementalRequirement: string
  requirement: AINativeRequirement | null
  error: string
  onProductLinkChange: (value: string) => void
  onSupplementalRequirementChange: (value: string) => void
  onAnalyze: () => void
  onReanalyze: () => void
  onChange: (requirement: AINativeRequirement) => void
  onSave: () => void
  onConfirm: () => void
  onEdit: () => void
}) {
  const [mediaPreviews, setMediaPreviews] = useState<Record<string, string | null>>({})
  const [recognition, setRecognition] = useState<ProductRecognitionState>({ status: 'idle' })
  const [recognitionRetry, setRecognitionRetry] = useState(0)
  const recognitionRequest = useRef(0)
  const hasRequirement = Boolean(requirement)
  const mediaPreviewKey = requirement?.media.map(item => item.asset_ref ? `${item.id}:${item.asset_ref.asset_id}:${item.asset_ref.version}` : `${item.id}:legacy`).join('|') ?? ''

  useEffect(() => {
    const input = productLink.trim()
    const requestID = ++recognitionRequest.current
    if (hasRequirement || !input) {
      setRecognition({ status: 'idle' })
      return
    }
    setRecognition({ status: 'checking' })
    const timer = window.setTimeout(() => {
      void resolveProductPreview(projectId, input).then(product => {
        if (recognitionRequest.current === requestID) setRecognition({ status: 'success', product })
      }).catch(cause => {
        if (recognitionRequest.current !== requestID) return
        setRecognition({ status: 'failed', ...productRecognitionFailure(cause) })
      })
    }, 450)
    return () => window.clearTimeout(timer)
  }, [hasRequirement, productLink, projectId, recognitionRetry])

  useEffect(() => {
    let active = true
    const importedMedia = requirement?.media.filter(item => item.asset_ref) ?? []
    setMediaPreviews({})
    void Promise.all(importedMedia.map(async item => {
      try {
        return [item.id, await getAssetPreview(projectId, item.asset_ref!)] as const
      } catch {
        return [item.id, null] as const
      }
    })).then(entries => {
      if (active) setMediaPreviews(Object.fromEntries(entries))
    })
    return () => { active = false }
  }, [projectId, mediaPreviewKey])

  const locked = status === 'confirmed'
  if (!requirement) {
    return <section className="ai-native-stage-panel requirement-entry" role="tabpanel" id="ai-native-panel-requirement" aria-labelledby="ai-native-stage-requirement">
      <div className="ai-native-stage-heading"><div><h3>从商品链接开始创作</h3><p>粘贴抖音商城商品链接并补充生成需求，AI 将自动整理商品名称、受众、素材和核心卖点。</p></div><span className="ai-native-channel">抖音 · 已支持</span></div>
      <div className="requirement-conversation">
        <div className="conversation-intro"><span>AI</span><p>请发送商品链接，并告诉我这条广告希望强调什么。其他渠道暂时只做入口展示，本阶段仅生成抖音 9:16 广告。</p></div>
        <div className="conversation-composer">
          <label><Link2 size={15}/><input aria-label="商品链接" aria-describedby="product-link-recognition" value={productLink} onChange={event => onProductLinkChange(event.target.value)} placeholder="粘贴抖音商城商品链接或完整分享口令"/></label>
          {recognition.status !== 'idle' ? <div id="product-link-recognition" className={`product-link-recognition ${recognition.status}`} role={recognition.status === 'failed' ? 'alert' : 'status'}>
            {recognition.status === 'checking' ? <><LoaderCircle className="spin" size={15}/><span><b>正在识别商品链接</b><small>正在提取并清理抖音商品信息…</small></span></> : recognition.status === 'success' ? <><CheckCircle2 size={16}/><span><b>识别成功</b><small>{recognition.product.product_name}</small></span></> : <><AlertTriangle size={16}/><span><b>商品链接识别失败</b><small>{recognition.message}</small></span>{recognition.retryable ? <button type="button" onClick={() => setRecognitionRetry(value => value + 1)}><RefreshCw size={12}/>重新识别</button> : null}</>}
          </div> : null}
          <textarea aria-label="补充生成需求" value={supplementalRequirement} onChange={event => onSupplementalRequirementChange(event.target.value)} placeholder="补充生成需求，例如：面向通勤人群，突出轻便和容量，整体节奏自然真实。" maxLength={2000}/>
          <div className="conversation-composer-footer"><span>{supplementalRequirement.length} / 2000</span><button className="primary-button" disabled={recognition.status !== 'success' || status === 'generating'} onClick={onAnalyze}>{status === 'generating' ? <LoaderCircle className="spin" size={15}/> : <Send size={15}/>}发送并分析</button></div>
        </div>
        {error ? <div className="ai-native-error" role="alert"><AlertTriangle size={15}/>{error}</div> : null}
      </div>
    </section>
  }

  const updateListItem = (field: 'target_audiences' | 'core_selling_points', index: number, text: string) => onChange({
    ...requirement,
    [field]: requirement[field].map((item, itemIndex) => itemIndex === index ? { ...item, text } : item),
  })
  const removeListItem = (field: 'target_audiences' | 'core_selling_points', index: number) => onChange({
    ...requirement,
    [field]: requirement[field].filter((_, itemIndex) => itemIndex !== index),
  })
  const addListItem = (field: 'target_audiences' | 'core_selling_points') => onChange({
    ...requirement,
    [field]: [...requirement[field], { id: `${field}-${Date.now()}`, text: field === 'target_audiences' ? '新增目标受众' : '新增核心卖点' }],
  })

  return <section className="ai-native-stage-panel" role="tabpanel" id="ai-native-panel-requirement" aria-labelledby="ai-native-stage-requirement">
    <div className="ai-native-stage-heading"><div><h3>需求分析</h3><p>AI 已根据商品链接和补充要求生成可编辑结果。确认后，本阶段会被冻结并进入脚本生成。</p></div>{locked ? <button className="secondary-button" onClick={onEdit}>编辑需求</button> : null}</div>
    <div className="requirement-form-grid">
      <label className="wide">商品名称<input disabled={locked} value={requirement.product_name} onChange={event => onChange({ ...requirement, product_name: event.target.value })}/></label>
      <label className="wide">商品描述<textarea disabled={locked} value={requirement.product_description} onChange={event => onChange({ ...requirement, product_description: event.target.value })}/></label>
      <div className="editable-list wide"><div><span>目标受众 <small>{requirement.target_audiences.length} / 10</small></span>{!locked ? <button onClick={() => addListItem('target_audiences')} disabled={requirement.target_audiences.length >= 10}><Plus size={13}/>添加</button> : null}</div><div className="editable-chips">{requirement.target_audiences.map((item, index) => <label key={item.id}><input disabled={locked} value={item.text} onChange={event => updateListItem('target_audiences', index, event.target.value)}/>{!locked ? <button aria-label={`删除目标受众 ${item.text}`} onClick={() => removeListItem('target_audiences', index)} disabled={requirement.target_audiences.length <= 1}><Trash2 size={12}/></button> : null}</label>)}</div></div>
      <RequirementMediaGallery media={requirement.media} previews={mediaPreviews} status={status} onReanalyze={onReanalyze}/>
      <div className="editable-list wide"><div><span>核心卖点 <small>{requirement.core_selling_points.length} / 20</small></span>{!locked ? <button onClick={() => addListItem('core_selling_points')} disabled={requirement.core_selling_points.length >= 20}><Plus size={13}/>添加</button> : null}</div><div className="editable-chips">{requirement.core_selling_points.map((item, index) => <label key={item.id}><input disabled={locked} value={item.text} onChange={event => updateListItem('core_selling_points', index, event.target.value)}/>{!locked ? <button aria-label={`删除核心卖点 ${item.text}`} onClick={() => removeListItem('core_selling_points', index)} disabled={requirement.core_selling_points.length <= 1}><Trash2 size={12}/></button> : null}</label>)}</div></div>
      <label>视频比例<select disabled={locked} value={requirement.aspect_ratio} onChange={() => undefined}><option>9:16</option></select></label>
      <label>视频时长<select disabled={locked} value={requirement.duration_seconds} onChange={event => onChange({ ...requirement, duration_seconds: Number(event.target.value) })}>{[15, 20, 25, 30].map(value => <option key={value} value={value}>{value} 秒</option>)}</select></label>
      <label>视频语言<select disabled={locked} value={requirement.language} onChange={() => undefined}><option value="zh-CN">简体中文</option></select></label>
      <label className="wide">补充生成需求<textarea disabled={locked} value={requirement.supplemental_requirement} onChange={event => onChange({ ...requirement, supplemental_requirement: event.target.value })}/></label>
    </div>
    <div className="requirement-notes requirement-confirmation-note"><AlertTriangle size={15}/><div><b>请确认 AI 提取的商品信息</b><span>商品名称、目标受众和核心卖点均可编辑</span><span>商品图片已从链接提取，可补充或替换</span><span>确认后将进入脚本生成；后续如返回修改，已生成的脚本、故事板和视频将失效</span></div></div>
    {error ? <div className="ai-native-error" role="alert"><AlertTriangle size={15}/>{error}</div> : null}
    <footer className="ai-native-actions">{locked ? <span className="confirmed-note">需求已确认并冻结</span> : <><button className="secondary-button" disabled={status === 'generating'} onClick={onSave}>保存修改</button><button className="primary-button" disabled={status === 'generating'} onClick={onConfirm}>{status === 'generating' ? <LoaderCircle className="spin" size={15}/> : null}确认并生成脚本</button></>}</footer>
  </section>
}
