import { useEffect, useState } from 'react'
import { AlertTriangle, Image, LoaderCircle, RefreshCw } from 'lucide-react'
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

export function StoryboardStage({ projectId, status, storyboard, canGenerate, error, onChange, onSave, onConfirm, onEdit, onRetry }: {
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
}) {
  const locked = status === 'confirmed'
  const [previews, setPreviews] = useState<Record<string, string>>({})

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
  }, [projectId, storyboard?.revision])

  if (status === 'generating') return <StageLoading icon={<LoaderCircle className="spin" size={24}/>} title="正在生成故事板" detail="AI 正在整理人物、商品和场景素材，并把脚本转换为完整分镜；生成图片会先进入项目 Assets。"/>
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
  const updateShot = (index: number, patch: Partial<StoryboardDraft['shots'][number]>) => onChange({
    ...storyboard,
    shots: storyboard.shots.map((shot, shotIndex) => shotIndex === index ? { ...shot, ...patch } : shot),
  })

  return <section className="ai-native-stage-panel" role="tabpanel" id="ai-native-panel-storyboard" aria-labelledby="ai-native-stage-storyboard">
    <div className="ai-native-stage-heading"><div><h3>故事板与分镜</h3><p>商品图固定复用链接导入的真实素材；人物、场景和构图缺失时由 AI 生成并进入项目素材库。</p></div>{locked ? <button className="secondary-button" onClick={onEdit}>编辑故事板</button> : <span className="ai-native-contract-label">Storyboard r{storyboard.revision}</span>}</div>
    <div className="storyboard-assets">{groups.map(group => <section key={group.role}><header><b>{assetGroupLabels[group.role]}</b><small>{group.assets.length} 项</small></header><div>{group.assets.map(asset => <figure key={asset.id}>{previews[asset.id] ? <img src={previews[asset.id]} alt={asset.name}/> : <span><Image size={19}/>{asset.status === 'ready' ? `Asset ${asset.asset_ref?.asset_id ?? ''}` : 'AI 素材准备中'}</span>}<figcaption><b>{asset.name}</b><small>{asset.source === 'product_import' ? '商品链接导入' : asset.source === 'project_asset' ? '项目素材' : asset.status === 'ready' ? 'AI 生成 · 已入库' : 'AI 生成计划'}</small></figcaption></figure>)}</div></section>)}</div>
    <div className="storyboard-shot-list">{storyboard.shots.map((shot, index) => <article key={shot.id}><header><div><span>分镜 {String(index + 1).padStart(2, '0')}</span><b>{(shot.duration_ms / 1000).toFixed(1)} 秒</b></div><small>{shot.start_ms / 1000}s–{shot.end_ms / 1000}s · 参考素材 {shot.reference_asset_ids.length} 项</small></header><div className="storyboard-shot-fields">
      <label>开始（毫秒）<input type="number" min={0} disabled={locked} value={shot.start_ms} onChange={event => updateShot(index, { start_ms: Number(event.target.value), duration_ms: shot.end_ms - Number(event.target.value) })}/></label>
      <label>结束（毫秒）<input type="number" min={1} disabled={locked} value={shot.end_ms} onChange={event => updateShot(index, { end_ms: Number(event.target.value), duration_ms: Number(event.target.value) - shot.start_ms })}/></label>
      <label className="wide">画面内容<textarea disabled={locked} value={shot.visual_content} onChange={event => updateShot(index, { visual_content: event.target.value })}/></label>
      <label className="wide">人物、商品和动作<textarea disabled={locked} value={shot.subjects_products_actions} onChange={event => updateShot(index, { subjects_products_actions: event.target.value })}/></label>
      <label>景别<textarea disabled={locked} value={shot.shot_size} onChange={event => updateShot(index, { shot_size: event.target.value })}/></label>
      <label>运镜<textarea disabled={locked} value={shot.camera_movement} onChange={event => updateShot(index, { camera_movement: event.target.value })}/></label>
      <label className="wide">参考图片<input disabled={locked} value={shot.reference_asset_ids.join('、')} onChange={event => updateShot(index, { reference_asset_ids: event.target.value.split('、').map(value => value.trim()).filter(Boolean) })}/></label>
      <label>旁白<textarea disabled={locked} value={shot.voiceover} onChange={event => updateShot(index, { voiceover: event.target.value })}/></label>
      <label>字幕<textarea disabled={locked} value={shot.subtitle} onChange={event => updateShot(index, { subtitle: event.target.value })}/></label>
      <label>音效<textarea disabled={locked} value={shot.sound_effect} onChange={event => updateShot(index, { sound_effect: event.target.value })}/></label>
      <label>BGM<textarea disabled={locked} value={shot.bgm_direction} onChange={event => updateShot(index, { bgm_direction: event.target.value })}/></label>
      <label className="wide">转场<textarea disabled={locked} value={shot.transition} onChange={event => updateShot(index, { transition: event.target.value })}/></label>
    </div></article>)}</div>
    <footer className="ai-native-actions">{locked ? <span className="confirmed-note">故事板已确认并冻结</span> : <><button className="secondary-button" onClick={onSave}>保存故事板</button><button className="primary-button" onClick={onConfirm}>确认并一键成片</button></>}</footer>
  </section>
}
