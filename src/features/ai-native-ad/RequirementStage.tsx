import { useEffect, useRef, useState } from 'react'
import { AlertTriangle, CheckCircle2, ChevronDown, Image, Link2, LoaderCircle, Plus, RefreshCw, Send, Trash2 } from 'lucide-react'
import { AINativeApiError, getAssetPreview, resolveProductPreview, uploadRequirementProductImage } from './api'
import type { AINativeDeliveryTreatment, AINativeOutputPreset, AINativeOutputPresetSnapshot, AINativeProductPreview, AINativeRequirement, AINativeStageStatus, RequirementMedia } from './types'

type ProductRecognitionState =
  | { status: 'idle' }
  | { status: 'checking' }
  | { status: 'success'; product: AINativeProductPreview }
  | { status: 'failed'; message: string; retryable: boolean }

function productRecognitionFailure(cause: unknown): { message: string; retryable: boolean } {
  if (cause instanceof AINativeApiError) {
    if (cause.code === 'AI_NATIVE_PRODUCT_LINK_INCOMPLETE') return { message: '复制内容不完整，商品参数在中途被截断。请回到商品页重新复制并完整粘贴。', retryable: false }
    if (cause.code === 'AI_NATIVE_PRODUCT_LINK_UNSUPPORTED') return { message: '没有识别到受支持的商品链接，请粘贴抖音、淘宝、天猫或 1688 的商品链接。', retryable: false }
    if (cause.code === 'AI_NATIVE_PRODUCT_DETAIL_MISSING') return { message: '已识别平台，但链接中没有商品或内容 ID。请确认复制的是商品详情链接。', retryable: false }
    if (cause.code === 'CLIENT_TIMEOUT') return { message: '商品信息获取超时，链接格式可能正常，可以重新识别。', retryable: true }
  }
  const detail = cause instanceof Error ? cause.message.toLowerCase() : ''
  if (detail.includes('incomplete product link') || detail.includes('unexpected end')) {
    return { message: '复制内容不完整，商品参数在中途被截断。请回到商品页重新复制并完整粘贴。', retryable: false }
  }
  if (detail.includes('host') || detail.includes('unsupported product link') || detail.includes('https link is required')) {
    return { message: '没有识别到受支持的商品链接，请粘贴抖音、淘宝、天猫或 1688 的商品链接。', retryable: false }
  }
  if (detail.includes('goods_detail is absent') || detail.includes('product information is missing')) {
    return { message: '已识别平台，但链接中没有商品或内容 ID。请确认复制的是商品详情链接。', retryable: false }
  }
  if (detail.includes('timeout') || detail.includes('超时') || detail.includes('network')) {
    return { message: '商品信息获取超时，链接格式可能正常，可以重新识别。', retryable: true }
  }
  return { message: '暂时无法获取商品信息，请确认链接完整后重新识别。', retryable: true }
}

export function RequirementMediaGallery({ media, previews, status, onReanalyze, onUpload, uploadBusy = false, locked = false }: {
  media: RequirementMedia[]
  previews: Record<string, string | null>
  status: AINativeStageStatus
  onReanalyze: () => void
  onUpload?: (file: File) => void
  uploadBusy?: boolean
  locked?: boolean
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
    }) : <div className="media-empty"><Image size={20}/><span>商品链接暂未提取到图片</span></div>}</div>
    {!locked && onUpload ? <label className="requirement-media-upload"><input type="file" accept="image/jpeg,image/png,image/webp" disabled={uploadBusy || media.length >= 20} onChange={event => { const file = event.target.files?.[0]; if (file) onUpload(file); event.currentTarget.value = '' }}/>{uploadBusy ? <LoaderCircle className="spin" size={14}/> : <Plus size={14}/>}上传商品主图</label> : null}
    {!locked && media.length === 0 ? <small className="requirement-media-help">链接没有权限提取，需要用户手动上传。支持 JPG、PNG、WebP；上传后自动保存到项目素材库。</small> : null}
    {legacyCount > 0 ? <div className="requirement-media-recovery"><AlertTriangle size={15}/><span>当前有 {legacyCount} 张旧版链接素材尚未进入项目素材库，源站地址可能无法稳定显示。</span><button className="secondary-button" disabled={status === 'generating'} onClick={onReanalyze}>{status === 'generating' ? <LoaderCircle className="spin" size={13}/> : null}重新提取商品素材</button></div> : null}
  </div>
}

export function RequirementStage({
  projectId,
  status,
  productLink,
  supplementalRequirement,
  requirement,
  outputPresets,
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
  outputPresets: AINativeOutputPreset[]
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
  const [uploadBusy, setUploadBusy] = useState(false)
  const [uploadError, setUploadError] = useState('')
  const [advancedDeliveryOpen, setAdvancedDeliveryOpen] = useState(false)
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
      <div className="ai-native-stage-heading"><div><h3>从商品链接开始创作</h3><p>粘贴抖音、淘宝、天猫或 1688 商品链接，AI 将整理公开页面中可获得的商品信息。</p></div><span className="ai-native-channel">4 类来源 · 已支持</span></div>
      <div className="requirement-conversation">
        <div className="conversation-intro"><span>AI</span><p>请发送商品链接，并告诉我这条广告希望强调什么。商品来源与视频使用场景相互独立，分析后可选择平台场景和对应比例。</p></div>
        <div className="conversation-composer">
          <label><Link2 size={15}/><input aria-label="商品链接" aria-describedby="product-link-recognition" value={productLink} onChange={event => onProductLinkChange(event.target.value)} placeholder="粘贴抖音、淘宝、天猫或 1688 商品链接/分享文案"/></label>
          {recognition.status !== 'idle' ? <div id="product-link-recognition" className={`product-link-recognition ${recognition.status}`} role={recognition.status === 'failed' ? 'alert' : 'status'}>
            {recognition.status === 'checking' ? <><LoaderCircle className="spin" size={15}/><span><b>正在识别商品链接</b><small>正在识别平台、商品类型并清理追踪参数…</small></span></> : recognition.status === 'success' ? <><CheckCircle2 size={16}/><span><b>识别成功</b><small>{recognition.product.product_name || `已识别${recognition.product.source}商品，商品名称需在下一步补充`}</small></span></> : <><AlertTriangle size={16}/><span><b>商品链接识别失败</b><small>{recognition.message}</small></span>{recognition.retryable ? <button type="button" onClick={() => setRecognitionRetry(value => value + 1)}><RefreshCw size={12}/>重新识别</button> : null}</>}
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
  const uploadProductImage = async (file: File) => {
    setUploadBusy(true)
    setUploadError('')
    try {
      const assetRef = await uploadRequirementProductImage(projectId, file)
      const item: RequirementMedia = { id: `media_upload_${Date.now()}`, url: '', role: file.name, source: 'user_upload', asset_ref: assetRef }
      onChange({ ...requirement, media: [...requirement.media, item] })
    } catch (cause) {
      setUploadError(cause instanceof Error ? cause.message : '商品图片上传失败，请重试。')
    } finally {
      setUploadBusy(false)
    }
  }
  const selectOutputPreset = (presetId: string) => {
    const selected = availableOutputPresets.find(item => item.id === presetId)
    if (!selected) return
    const { status: _status, ...snapshot } = selected
    onChange({ ...requirement, output_preset: snapshot as AINativeOutputPresetSnapshot, channel: snapshot.channel, aspect_ratio: snapshot.aspect_ratio })
  }
  const visibleOutputPresets = outputPresets.filter(preset => preset.channel !== 'xiaohongshu')
  const availableOutputPresets: AINativeOutputPreset[] = visibleOutputPresets.length > 0
    ? visibleOutputPresets
    : requirement.output_preset && requirement.output_preset.channel !== 'xiaohongshu' ? [{ ...requirement.output_preset, status: 'available' }] : []
  const selectedOutputPresetID = availableOutputPresets.some(preset => preset.id === requirement.output_preset?.id)
    ? requirement.output_preset!.id
    : availableOutputPresets[0]?.id || ''
  const delivery = requirement.delivery_treatment ?? deliveryTreatmentForPreset('full_ad')
  const selectDeliveryPreset = (preset: AINativeDeliveryTreatment['preset']) => {
    if (preset === 'custom') return
    onChange({ ...requirement, delivery_treatment: deliveryTreatmentForPreset(preset) })
  }
  const updateDelivery = (patch: Partial<AINativeDeliveryTreatment>) => {
    const next = { ...delivery, ...patch, preset: 'custom' as const }
    if (next.voiceover_mode === 'none' && next.caption_mode === 'from_voiceover') next.caption_mode = 'editorial'
    onChange({ ...requirement, delivery_treatment: next })
  }

  return <section className="ai-native-stage-panel" role="tabpanel" id="ai-native-panel-requirement" aria-labelledby="ai-native-stage-requirement">
    <div className="ai-native-stage-heading"><div><h3>需求分析</h3><p>AI 已根据商品链接和补充要求生成可编辑结果。确认后，本阶段会被冻结并进入脚本生成。</p></div>{locked ? <button className="secondary-button" onClick={onEdit}>编辑需求</button> : null}</div>
    <div className="requirement-form-grid">
      <label className="wide">商品名称<input disabled={locked} value={requirement.product_name} onChange={event => onChange({ ...requirement, product_name: event.target.value })}/></label>
      <label className="wide">商品描述<textarea disabled={locked} value={requirement.product_description} onChange={event => onChange({ ...requirement, product_description: event.target.value })}/></label>
      <div className="editable-list wide"><div><span>目标受众 <small>{requirement.target_audiences.length} / 10</small></span>{!locked ? <button onClick={() => addListItem('target_audiences')} disabled={requirement.target_audiences.length >= 10}><Plus size={13}/>添加</button> : null}</div><div className="editable-chips">{requirement.target_audiences.map((item, index) => <label key={item.id}><input disabled={locked} value={item.text} onChange={event => updateListItem('target_audiences', index, event.target.value)}/>{!locked ? <button aria-label={`删除目标受众 ${item.text}`} onClick={() => removeListItem('target_audiences', index)} disabled={requirement.target_audiences.length <= 1}><Trash2 size={12}/></button> : null}</label>)}</div></div>
      <RequirementMediaGallery media={requirement.media} previews={mediaPreviews} status={status} onReanalyze={onReanalyze} onUpload={uploadProductImage} uploadBusy={uploadBusy} locked={locked}/>
      {uploadError ? <div className="ai-native-error wide" role="alert"><AlertTriangle size={15}/>{uploadError}</div> : null}
      <div className="editable-list wide"><div><span>核心卖点 <small>{requirement.core_selling_points.length} / 20</small></span>{!locked ? <button onClick={() => addListItem('core_selling_points')} disabled={requirement.core_selling_points.length >= 20}><Plus size={13}/>添加</button> : null}</div><div className="editable-chips">{requirement.core_selling_points.map((item, index) => <label key={item.id}><input disabled={locked} value={item.text} onChange={event => updateListItem('core_selling_points', index, event.target.value)}/>{!locked ? <button aria-label={`删除核心卖点 ${item.text}`} onClick={() => removeListItem('core_selling_points', index)} disabled={requirement.core_selling_points.length <= 1}><Trash2 size={12}/></button> : null}</label>)}</div></div>
      <label>视频使用场景与比例<select disabled={locked || availableOutputPresets.length === 0} value={selectedOutputPresetID} onChange={event => selectOutputPreset(event.target.value)}>{availableOutputPresets.length ? availableOutputPresets.map(preset => <option key={preset.id} value={preset.id}>{preset.label}</option>) : <option value="">暂无可用创作规格</option>}</select><small>仅决定视频创作规格，不会自动投放或连接广告账户。</small></label>
      <label>视频时长<select disabled={locked} value={requirement.duration_seconds} onChange={event => onChange({ ...requirement, duration_seconds: Number(event.target.value) })}>{[15, 20, 25, 30].map(value => <option key={value} value={value}>{value} 秒</option>)}</select></label>
      <label>视频语言<select disabled={locked} value={requirement.language} onChange={() => undefined}><option value="zh-CN">简体中文</option></select></label>
      <section className="delivery-treatment wide" aria-label="交付方式">
        <header><div><b>交付方式</b><small>声音和文字轨道会随需求一起冻结</small></div></header>
        <div className="delivery-preset-grid">
          {([
            ['full_ad', '完整广告', '旁白 + 字幕 + 卖点叠字 + BGM/音效'],
            ['no_voiceover', '无旁白成片', '叙事字幕 + 卖点叠字 + BGM/音效'],
            ['clean_material', '纯净视频素材', '无旁白、无文字、无音频'],
          ] as const).map(([preset, label, description]) => <button type="button" disabled={locked} className={delivery.preset === preset ? 'active' : ''} onClick={() => selectDeliveryPreset(preset)} key={preset}><span>{delivery.preset === preset ? '●' : '○'}</span><div><b>{label}{preset === 'full_ad' ? '（推荐）' : ''}</b><small>{description}</small></div></button>)}
        </div>
        <button className="delivery-advanced-toggle" type="button" onClick={() => setAdvancedDeliveryOpen(open => !open)} aria-expanded={advancedDeliveryOpen}><span>高级设置{delivery.preset === 'custom' ? ' · 自定义' : ''}</span><ChevronDown size={14}/></button>
        {advancedDeliveryOpen ? <div className="delivery-advanced-grid">
          <label>旁白<select disabled={locked} value={delivery.voiceover_mode} onChange={event => updateDelivery({ voiceover_mode: event.target.value as AINativeDeliveryTreatment['voiceover_mode'] })}><option value="generated">生成旁白</option><option value="none">无旁白</option></select></label>
          <label>字幕<select disabled={locked} value={delivery.caption_mode} onChange={event => updateDelivery({ caption_mode: event.target.value as AINativeDeliveryTreatment['caption_mode'] })}><option value="from_voiceover" disabled={delivery.voiceover_mode === 'none'}>跟随旁白</option><option value="editorial">编辑型字幕</option><option value="none">无字幕</option></select></label>
          <label>卖点叠字<select disabled={locked} value={delivery.sales_overlay_mode} onChange={event => updateDelivery({ sales_overlay_mode: event.target.value as AINativeDeliveryTreatment['sales_overlay_mode'] })}><option value="key_points">卖点与 CTA</option><option value="minimal">仅关键卖点</option><option value="none">无叠字</option></select></label>
          <label>BGM/音效<select disabled={locked} value={delivery.music_sfx_mode} onChange={event => updateDelivery({ music_sfx_mode: event.target.value as AINativeDeliveryTreatment['music_sfx_mode'] })}><option value="auto">自动规划（需授权素材）</option><option value="none">无音频</option></select></label>
        </div> : null}
      </section>
      <label className="wide">补充生成需求<textarea disabled={locked} value={requirement.supplemental_requirement} onChange={event => onChange({ ...requirement, supplemental_requirement: event.target.value })}/></label>
    </div>
    <div className="requirement-notes requirement-confirmation-note"><AlertTriangle size={15}/><div><b>请确认 AI 整理的商品信息</b><span>商品名称、目标受众和核心卖点均可编辑</span><span>商品图片可由链接提取，也可手动上传补充</span><span>确认后将进入脚本生成；后续如返回修改，已生成的脚本、故事板和视频将失效</span></div></div>
    {error ? <div className="ai-native-error" role="alert"><AlertTriangle size={15}/>{error}</div> : null}
    <footer className="ai-native-actions">{locked ? <span className="confirmed-note">需求已确认并冻结</span> : <><button className="secondary-button" disabled={status === 'generating'} onClick={onSave}>保存修改</button><button className="primary-button" disabled={status === 'generating'} onClick={onConfirm}>{status === 'generating' ? <LoaderCircle className="spin" size={15}/> : null}确认并生成脚本</button></>}</footer>
  </section>
}

function deliveryTreatmentForPreset(preset: Exclude<AINativeDeliveryTreatment['preset'], 'custom'>): AINativeDeliveryTreatment {
  if (preset === 'no_voiceover') return { preset, voiceover_mode: 'none', caption_mode: 'editorial', sales_overlay_mode: 'key_points', music_sfx_mode: 'auto' }
  if (preset === 'clean_material') return { preset, voiceover_mode: 'none', caption_mode: 'none', sales_overlay_mode: 'none', music_sfx_mode: 'none' }
  return { preset, voiceover_mode: 'generated', caption_mode: 'from_voiceover', sales_overlay_mode: 'key_points', music_sfx_mode: 'auto' }
}
