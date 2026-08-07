import { useEffect, useMemo, useState } from 'react'
import { AlertTriangle, Image, LoaderCircle, Maximize2, RefreshCw, X } from 'lucide-react'
import { getAssetPreview } from './api'
import { StageEmpty, StageLoading } from './ScriptStage'
import type { AINativeStageStatus, StoryboardAsset, StoryboardDraft } from './types'

const assetGroupLabels: Record<StoryboardAsset['role'], string> = {
  person_identity: '人物图片',
  product_identity: '商品图片',
  scene_reference: '场景图',
  composition_reference: '构图参考',
  audio_reference: '音频素材',
  brand_element: '品牌元素',
}

type StoryboardStageProps = {
  projectId: string
  status: AINativeStageStatus
  storyboard: StoryboardDraft | null
  canGenerate: boolean
  error: string
  onChange: (storyboard: StoryboardDraft) => void
  onSave: () => void
  onConfirm: () => void
  onEdit: () => void
  onRetry: () => void
  onRegenerateAsset: (assetId: string) => void
  onReplaceSourceAsset: (assetId: string) => void
}

export function StoryboardStage({ projectId, status, storyboard, canGenerate, error, onChange, onSave, onConfirm, onEdit, onRetry, onRegenerateAsset, onReplaceSourceAsset }: StoryboardStageProps) {
  const locked = status === 'confirmed'
  const editable = status === 'draft'
  const [previews, setPreviews] = useState<Record<string, string>>({})
  const [selectedAssetId, setSelectedAssetId] = useState('')
  const previewKey = storyboard?.assets.map(asset => `${asset.id}:${asset.asset_ref?.asset_id ?? ''}:${asset.asset_ref?.version ?? 0}`).join('|') ?? ''
  const selectedAsset = useMemo(() => storyboard?.assets.find(asset => asset.id === selectedAssetId) ?? null, [selectedAssetId, storyboard])

  useEffect(() => {
    let active = true
    const readyAssets = storyboard?.assets.filter(asset => asset.asset_ref) ?? []
    void Promise.all(readyAssets.map(async asset => {
      try {
        return [asset.id, await getAssetPreview(projectId, asset.asset_ref!)] as const
      } catch {
        return [asset.id, ''] as const
      }
    })).then(entries => {
      if (active) setPreviews(Object.fromEntries(entries))
    })
    return () => { active = false }
  }, [projectId, previewKey])

  useEffect(() => {
    if (!selectedAsset) return
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setSelectedAssetId('')
    }
    window.addEventListener('keydown', closeOnEscape)
    return () => window.removeEventListener('keydown', closeOnEscape)
  }, [selectedAsset])

  if (status === 'generating' && !storyboard) return <StageLoading icon={<LoaderCircle className="spin" size={24}/>} title="正在生成故事板" detail="AI 正在整理人物、商品和场景素材，并把脚本转换为完整分镜；生成图片会先进入项目 Assets。"/>
  if (!storyboard) {
    const failed = status === 'failed'
    const action = canGenerate ? <button className="secondary-button" onClick={onRetry}><RefreshCw size={14}/>重新生成故事板</button> : undefined
    return <StageEmpty
      icon={failed ? <AlertTriangle size={24}/> : <Image size={24}/>}
      title={failed ? '故事板生成未启动' : status === 'invalidated' ? '故事板已因上游修改而作废' : '尚未生成故事板'}
      detail={failed ? error || '故事板生成任务启动失败，请稍后重试。' : '确认脚本后，这里将展示素材板和每个镜头的完整制作信息。'}
      action={action}
    />
  }

  const roles = Object.keys(assetGroupLabels) as StoryboardAsset['role'][]
  const groups = roles.map(role => ({ role, assets: storyboard.assets.filter(asset => asset.role === role) })).filter(group => group.assets.length)
  const failedAssets = storyboard.assets.filter(asset => asset.status === 'failed')
  const readyAssetCount = storyboard.assets.filter(asset => asset.status === 'ready').length
  const updateShot = (index: number, patch: Partial<StoryboardDraft['shots'][number]>) => onChange({
    ...storyboard,
    shots: storyboard.shots.map((shot, shotIndex) => shotIndex === index ? { ...shot, ...patch } : shot),
  })

  return <section className="ai-native-stage-panel" role="tabpanel" id="ai-native-panel-storyboard" aria-labelledby="ai-native-stage-storyboard">
    <div className="ai-native-stage-heading"><div><h3>故事板与分镜</h3><p>点击图片可放大核对；AI 生成人物、场景和构图素材可逐张重新生成，商品原图继续使用链接导入素材。</p></div>{locked ? <button className="secondary-button" onClick={onEdit}>编辑故事板</button> : <span className="ai-native-contract-label">Storyboard r{storyboard.revision}</span>}</div>
    {status === 'generating' ? <div className="storyboard-generation-status"><LoaderCircle className="spin" size={16}/><span>正在生成素材，已完成 {readyAssetCount}/{storyboard.assets.length}；成功素材会立即保存。</span></div> : null}
    {status === 'failed' ? <div className="ai-native-error"><AlertTriangle size={16}/><div><b>部分故事板素材生成失败</b><p>{error || `已保留 ${readyAssetCount} 项成功素材，可仅重试 ${failedAssets.length} 项失败素材。`}</p></div></div> : null}

    <div className="storyboard-assets">{groups.map(group => <section key={group.role}><header><b>{assetGroupLabels[group.role]}</b><small>{group.assets.length} 项</small></header><div>{group.assets.map(asset => {
      const preview = previews[asset.id]
      const canRegenerate = asset.source === 'ai_generated' && asset.status !== 'generating'
      return <figure className={`storyboard-asset-${asset.status}`} key={asset.id}>
        {preview ? <button className="storyboard-preview-button" type="button" onClick={() => setSelectedAssetId(asset.id)} aria-label={`放大查看${asset.name}`}><img src={preview} alt={asset.name}/><span><Maximize2 size={13}/>点击放大</span></button> : <span>{asset.status === 'failed' ? <AlertTriangle size={19}/> : asset.status === 'generating' ? <LoaderCircle className="spin" size={19}/> : <Image size={19}/>} {asset.status === 'ready' ? `Asset ${asset.asset_ref?.asset_id ?? ''}` : asset.status === 'failed' ? '生成失败' : asset.status === 'generating' ? 'AI 素材生成中' : 'AI 素材待生成'}</span>}
        <figcaption><b>{asset.name}</b><small>{asset.source === 'product_import' ? '商品链接导入' : asset.source === 'project_asset' ? '项目素材' : asset.status === 'ready' ? 'AI 生成 · 已入库' : asset.status === 'failed' ? `第 ${asset.generation_attempt ?? 1} 次生成失败` : asset.status === 'generating' ? 'AI 正在生成' : 'AI 生成计划'}</small>{asset.error_message ? <em title={asset.error_message}>{asset.error_message}</em> : null}{canRegenerate ? <button className="storyboard-regenerate-button" type="button" onClick={() => onRegenerateAsset(asset.id)}><RefreshCw size={12}/>重新生成</button> : asset.source !== 'ai_generated' ? <button className="storyboard-regenerate-button" type="button" onClick={() => onReplaceSourceAsset(asset.id)}><RefreshCw size={12}/>{asset.source === 'product_import' ? '更换商品图' : '更换素材'}</button> : null}</figcaption>
      </figure>
    })}</div></section>)}</div>

    <div className="storyboard-shot-list">{storyboard.shots.map((shot, index) => <article key={shot.id}><header><div><span>分镜 {String(index + 1).padStart(2, '0')}</span><b>{(shot.duration_ms / 1000).toFixed(1)} 秒</b></div><small>{shot.start_ms / 1000}s–{shot.end_ms / 1000}s · 参考素材 {shot.reference_asset_ids.length} 项</small></header><div className="storyboard-shot-fields">
      <label>开始（毫秒）<input type="number" min={0} disabled={!editable} value={shot.start_ms} onChange={event => updateShot(index, { start_ms: Number(event.target.value), duration_ms: shot.end_ms - Number(event.target.value) })}/></label>
      <label>结束（毫秒）<input type="number" min={1} disabled={!editable} value={shot.end_ms} onChange={event => updateShot(index, { end_ms: Number(event.target.value), duration_ms: Number(event.target.value) - shot.start_ms })}/></label>
      <label className="wide">画面内容<textarea disabled={!editable} value={shot.visual_content} onChange={event => updateShot(index, { visual_content: event.target.value })}/></label>
      <label className="wide">人物、商品和动作<textarea disabled={!editable} value={shot.subjects_products_actions} onChange={event => updateShot(index, { subjects_products_actions: event.target.value })}/></label>
      <label>景别<textarea disabled={!editable} value={shot.shot_size} onChange={event => updateShot(index, { shot_size: event.target.value })}/></label>
      <label>运镜<textarea disabled={!editable} value={shot.camera_movement} onChange={event => updateShot(index, { camera_movement: event.target.value })}/></label>
      <label className="wide">参考图片<input disabled={!editable} value={shot.reference_asset_ids.join('、')} onChange={event => updateShot(index, { reference_asset_ids: event.target.value.split('、').map(value => value.trim()).filter(Boolean) })}/></label>
      <label>旁白<textarea disabled={!editable} value={shot.voiceover} onChange={event => updateShot(index, { voiceover: event.target.value })}/></label>
      <label>字幕<textarea disabled={!editable} value={shot.subtitle} onChange={event => updateShot(index, { subtitle: event.target.value })}/></label>
      <label>音效<textarea disabled={!editable} value={shot.sound_effect} onChange={event => updateShot(index, { sound_effect: event.target.value })}/></label>
      <label>BGM<textarea disabled={!editable} value={shot.bgm_direction} onChange={event => updateShot(index, { bgm_direction: event.target.value })}/></label>
      <label className="wide">转场<textarea disabled={!editable} value={shot.transition} onChange={event => updateShot(index, { transition: event.target.value })}/></label>
    </div></article>)}</div>
    <footer className="ai-native-actions">{locked ? <span className="confirmed-note">故事板已确认并冻结</span> : status === 'failed' ? <button className="primary-button" disabled={!canGenerate} onClick={onRetry}><RefreshCw size={14}/>仅重试失败素材</button> : status === 'generating' ? <span className="confirmed-note">素材生成中，完成后可编辑分镜</span> : <><button className="secondary-button" onClick={onSave}>保存故事板</button><button className="primary-button" onClick={onConfirm}>确认并一键成片</button></>}</footer>

    {selectedAsset && previews[selectedAsset.id] ? <div className="storyboard-lightbox" role="presentation" onMouseDown={event => { if (event.target === event.currentTarget) setSelectedAssetId('') }}><section role="dialog" aria-modal="true" aria-labelledby="storyboard-lightbox-title"><header><div><b id="storyboard-lightbox-title">{selectedAsset.name}</b><small>{assetGroupLabels[selectedAsset.role]} · {selectedAsset.source === 'ai_generated' ? 'AI 生成' : '商品/项目素材'}</small></div><button type="button" onClick={() => setSelectedAssetId('')} aria-label="关闭大图"><X size={18}/></button></header><div><img src={previews[selectedAsset.id]} alt={selectedAsset.name}/></div><footer><button className="secondary-button" type="button" onClick={() => { setSelectedAssetId(''); selectedAsset.source === 'ai_generated' ? onRegenerateAsset(selectedAsset.id) : onReplaceSourceAsset(selectedAsset.id) }}><RefreshCw size={13}/>{selectedAsset.source === 'ai_generated' ? '不满意，重新生成' : selectedAsset.source === 'product_import' ? '更换商品图' : '更换素材'}</button></footer></section></div> : null}
  </section>
}
