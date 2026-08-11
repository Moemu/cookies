import { AlertTriangle, FileText, LoaderCircle, RefreshCw } from 'lucide-react'
import type { AdScriptDraft, AINativeStageStatus } from './types'

export function ScriptStage({ status, script, error, onChange, onRegenerate, onRetry, onConfirm, onEdit }: {
  status: AINativeStageStatus
  script: AdScriptDraft | null
  error: string
  onChange: (script: AdScriptDraft) => void
  onRegenerate: () => void
  onRetry: () => void
  onConfirm: () => void
  onEdit: () => void
}) {
  const locked = status === 'confirmed'
  if (status === 'generating') return <StageLoading icon={<LoaderCircle className="spin" size={24}/>} title="正在生成完整营销脚本" detail="AI 正在组合痛点引入、卖点展示和转化收束，通常需要 1–2 分钟。"/>
  if (status === 'failed') return <StageEmpty icon={<AlertTriangle size={24}/>} title="脚本生成失败" detail={error || '后台任务未能完成，请重新生成脚本。'} action={<button className="secondary-button" onClick={onRetry}><RefreshCw size={14}/>重新生成脚本</button>}/>
  if (!script) return <StageEmpty icon={<FileText size={24}/>} title={status === 'invalidated' ? '脚本已因需求修改而作废' : '尚未生成脚本'} detail="确认需求分析后，系统将在这里生成一个完整脚本。你仍可先查看脚本字段结构。"/>

  return <section className="ai-native-stage-panel" role="tabpanel" id="ai-native-panel-script" aria-labelledby="ai-native-stage-script">
    <div className="ai-native-stage-heading"><div><h3>广告方向与脚本</h3><p>当前只保留一份完整脚本。不满意可填写调整要求后重新生成整版。</p></div>{locked ? <button className="secondary-button" onClick={onEdit}>编辑脚本</button> : <span className="frontend-demo-label">服务端草稿</span>}</div>
    <div className="script-summary">
      <label>脚本标题<input disabled={locked} value={script.title} onChange={event => onChange({ ...script, title: event.target.value })}/></label>
      <label className="wide">创意方向<textarea disabled={locked} value={script.creative_summary} onChange={event => onChange({ ...script, creative_summary: event.target.value })}/></label>
    </div>
    <div className="script-segment-list">{script.segments.map((segment, index) => <article key={segment.id}><header><span>{String(index + 1).padStart(2, '0')}</span><div><b>{segment.purpose}</b><small>{segment.start_ms / 1000}s–{segment.end_ms / 1000}s · {(segment.end_ms - segment.start_ms) / 1000} 秒</small></div></header><div className="script-segment-fields"><label className="wide">画面内容<textarea disabled={locked} value={segment.visual_intent} onChange={event => onChange({ ...script, segments: script.segments.map((item, itemIndex) => itemIndex === index ? { ...item, visual_intent: event.target.value } : item) })}/></label><label>旁白<textarea disabled={locked} value={segment.voiceover} onChange={event => onChange({ ...script, segments: script.segments.map((item, itemIndex) => itemIndex === index ? { ...item, voiceover: event.target.value } : item) })}/></label><label>字幕<textarea disabled={locked} value={segment.subtitle} onChange={event => onChange({ ...script, segments: script.segments.map((item, itemIndex) => itemIndex === index ? { ...item, subtitle: event.target.value } : item) })}/></label><label>使用卖点 ID<input disabled={locked} value={segment.selling_point_ids.join(', ')} onChange={event => onChange({ ...script, segments: script.segments.map((item, itemIndex) => itemIndex === index ? { ...item, selling_point_ids: event.target.value.split(',').map(value => value.trim()).filter(Boolean) } : item) })}/></label><label>转化动作<input disabled={locked} value={segment.conversion_action ?? ''} onChange={event => onChange({ ...script, segments: script.segments.map((item, itemIndex) => itemIndex === index ? { ...item, conversion_action: event.target.value } : item) })}/></label></div></article>)}</div>
    <footer className="ai-native-actions">{locked ? <span className="confirmed-note">脚本已确认并冻结</span> : <><label className="regeneration-note">重新生成要求<input value={script.regeneration_note ?? ''} onChange={event => onChange({ ...script, regeneration_note: event.target.value })} placeholder="可选，例如：开头更直接，旁白更口语化"/></label><button className="secondary-button" onClick={onRegenerate}><RefreshCw size={14}/>重新生成整版</button><button className="primary-button" onClick={onConfirm}>确认并生成故事板</button></>}</footer>
  </section>
}

function StageEmpty({ icon, title, detail, action }: { icon: React.ReactNode; title: string; detail: string; action?: React.ReactNode }) {
  return <section className="ai-native-stage-panel stage-empty" role="tabpanel"><div>{icon}<h3>{title}</h3><p>{detail}</p>{action}</div><div className="empty-structure"><span/><span/><span/><span/></div></section>
}

function StageLoading({ icon, title, detail }: { icon: React.ReactNode; title: string; detail: string }) {
  return <section className="ai-native-stage-panel stage-loading" role="tabpanel"><div>{icon}<h3>{title}</h3><p>{detail}</p></div></section>
}

export { StageEmpty, StageLoading }
