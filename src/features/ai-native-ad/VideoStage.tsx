import { useState } from 'react'
import { AlertTriangle, Check, Download, Film, ImageOff, LoaderCircle, RotateCcw, Sparkles, Square } from 'lucide-react'
import { StageEmpty } from './ScriptStage'
import type { AINativeStageStatus, ProductionReferenceFailure, VideoRenderState } from './types'

type VideoStageProps = {
  status: AINativeStageStatus
  video: VideoRenderState | null
  referenceFailure: ProductionReferenceFailure | null
  onRetry: () => void
  onFitVoiceover: () => void
  voiceoverFitBusy?: boolean
  onCancel: () => void
  onReviewReference: (assetId: string) => void
}

export function VideoStage({ status, video, referenceFailure, onRetry, onFitVoiceover, voiceoverFitBusy = false, onCancel, onReviewReference }: VideoStageProps) {
  const [feedbackCopied, setFeedbackCopied] = useState(false)
  const copyReferenceFeedback = async () => {
    if (!referenceFailure || !navigator.clipboard) return
    try {
      await navigator.clipboard.writeText(referenceFailure.recommended_feedback)
      setFeedbackCopied(true)
    } catch {
      setFeedbackCopied(false)
    }
  }
  if (!video) return <StageEmpty icon={<Film size={24}/>} title="尚未开始视频生成" detail="确认故事板后，系统将在这里展示视频素材、旁白与最终成片进度。" />
  const completed = video.status === 'completed'
  const assetsReady = video.status === 'assets_ready'
  const failed = status === 'failed' || video.status === 'failed' || video.status === 'render_failed' || video.status === 'cancelled'
  const renderFailed = video.status === 'render_failed'

  if (completed) return <section className="video-completed">
    <div className="ai-native-stage-heading"><div><h3>广告视频已生成</h3><p>字幕、旁白与音轨已完成对齐，最终 MP4 已进入项目素材库。</p></div><span className="completed-icon"><Check size={18}/></span></div>
    <div className="video-result final-video-result">
      <div className="video-result-preview final-preview">{video.output_url ? <video controls playsInline preload="metadata" src={video.output_url}/> : <><Film size={34}/><span>正在获取安全预览地址…</span></>}</div>
      <div><h4>抖音效果广告 · 9:16</h4><p>H.264 / AAC · 720×1280 · 字幕安全区 · 旁白自动压低 BGM</p>{video.output_url ? <a className="primary-button" href={video.output_url} download><Download size={14}/>下载 MP4</a> : null}</div>
    </div>
  </section>

  return <section>
    <div className="ai-native-stage-heading"><div><h3>{assetsReady ? '视频素材已生成' : failed ? (renderFailed ? '最终视频渲染失败' : '部分素材生成失败') : video.status === 'rendering' ? '正在合成最终视频' : '正在生成视频素材'}</h3><p>{assetsReady ? '视频片段与旁白已就绪。' : renderFailed ? '成功片段与旁白已经保留，可只重试最终合成。' : failed ? '成功素材已经保留；请先处理明确的失败原因，再决定是否重试。' : '进度来自服务端；离开页面不会中断任务，稍后返回仍可恢复。'}</p></div></div>
    {referenceFailure ? <div className="video-reference-failure" role="alert"><span><ImageOff size={20}/></span><div><b>故事板参考图片未通过视频模型检查</b><p><strong>{referenceFailure.asset_name}</strong> 用于 {referenceFailure.unit_id}。{referenceFailure.reason}</p>{referenceFailure.asset_source === 'ai_generated' ? <div className="video-reference-suggestion"><small>建议反馈词</small><p>{referenceFailure.recommended_feedback}</p><button className="secondary-button" type="button" onClick={() => { void copyReferenceFeedback() }}>{feedbackCopied ? '已复制' : '复制反馈词'}</button></div> : <small>这是商品链接导入图片，需要返回需求阶段更换合规商品图；原样重试仍会失败。</small>}</div><button className="secondary-button" type="button" onClick={() => onReviewReference(referenceFailure.asset_id)}><AlertTriangle size={13}/>查看问题素材</button></div> : null}
    {!referenceFailure && failed && video.failed_unit_id ? <div className="video-production-failure" role="alert"><AlertTriangle size={18}/><div><b>{video.failed_unit_id}{video.failed_shot_id ? ` · ${video.failed_shot_id}` : ''}</b><p>{video.failure_reason || '该素材生成失败，请重试。'}</p>{video.failure_code === 'SPEECH_DURATION_EXCEEDED' ? <button className="secondary-button voiceover-fit-button" type="button" disabled={voiceoverFitBusy} onClick={onFitVoiceover}>{voiceoverFitBusy ? <LoaderCircle className="spin" size={13}/> : <Sparkles size={13}/>} {voiceoverFitBusy ? '正在压缩旁白…' : '智能压缩旁白'}</button> : null}</div></div> : null}
    {assetsReady ? <div className="video-result"><div className="video-result-preview"><Film size={34}/><span>9:16 · {video.total_shots} 个视频 Unit · 静音片段</span></div><div><span className="completed-icon"><Check size={18}/></span><h4>素材生产完成</h4><p>系统将继续完成字幕、混音与最终成片。</p></div></div> : <div className="render-progress-layout"><div className="render-poster"><div style={{ height: `${Math.max(video.progress, 8)}%` }}/><span>{video.progress}%</span></div><div className="render-progress-detail"><div>{failed ? <RotateCcw size={18}/> : <span className="spin"><LoaderCircle size={18}/></span>}<span><b>{video.current_step}</b><small>{failed ? '可从失败阶段继续' : `预计剩余 ${Math.max(1, Math.ceil(video.eta_seconds / 60))} 分钟`}</small></span></div><div className="render-progress-track"><span style={{ width: `${video.progress}%` }}/></div><dl><div><dt>服务端进度</dt><dd>{video.progress}%</dd></div><div><dt>视频片段</dt><dd>{video.completed_shots} / {video.total_shots}</dd></div><div><dt>旁白</dt><dd>{video.completed_speech_units} / {video.total_speech_units}</dd></div><div><dt>视频规格</dt><dd>9:16 · 简体中文</dd></div></dl><ol><li className={video.progress >= 5 ? 'active' : ''}>生成视频 Unit</li><li className={video.progress >= 60 ? 'active' : ''}>生成并入库旁白</li><li className={video.progress >= 70 ? 'active' : ''}>字幕、BGM 与音画对齐</li><li className={video.progress >= 90 ? 'active' : ''}>最终 MP4 校验并入库</li></ol><div>{failed ? <button className="secondary-button" disabled={Boolean(referenceFailure)} onClick={onRetry}><RotateCcw size={14}/>{referenceFailure ? '请先处理问题素材' : renderFailed ? '只重试最终合成' : video.failed_unit_id ? `重试 ${video.failed_unit_id}` : '只重试失败片段'}</button> : <button className="secondary-button" onClick={onCancel}><Square size={13}/>取消生产</button>}</div></div></div>}
  </section>
}
