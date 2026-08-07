import { useEffect, useMemo, useRef, useState } from 'react'
import { Check, Download, Film, Play, Save, Scissors, Upload } from 'lucide-react'

import { useProject } from '../../context/ProjectContext'
import { api, type ApiProjectMediaAsset } from '../../data/api'
import { shortId } from '../../data/shortId'
import { EditingApiError, editingApi, type ApiEditTask, type ApiEditingRenderJob } from './api'
import { VideoPreviewPlayer } from './VideoPreviewPlayer'
import { VideoTimeline } from './VideoTimeline'
import {
  addClip,
  commitTimeline,
  createEmptyEditorTimeline,
  createTimelineHistory,
  deleteClip,
  moveClip,
  redoTimeline,
  restoreEditorTimeline,
  splitClip,
  toEditingTimeline,
  trimClip,
  undoTimeline,
  type EditorTimeline,
  type TimelineAsset,
  type TimelineHistory,
} from './timeline'

type SaveState = 'clean' | 'dirty' | 'saving' | 'error' | 'conflict'

export function VideoEditingWorkspaceV2({ onNotice, editTaskId, onOpenEditTask }: {
  onNotice: (message: string) => void
  onCreate: () => void
  editTaskId?: string
  onOpenEditTask: (id: string) => void
}) {
  const { currentProject } = useProject()
  const [assets, setAssets] = useState<ApiProjectMediaAsset[]>([])
  const [assetState, setAssetState] = useState<'loading' | 'ready' | 'error'>('loading')
  const [uploading, setUploading] = useState(false)
  const [previewAssetId, setPreviewAssetId] = useState('')
  const [editTask, setEditTask] = useState<ApiEditTask | null>(null)
  const [history, setHistory] = useState<TimelineHistory>(() => createTimelineHistory(createEmptyEditorTimeline()))
  const [selectedClipId, setSelectedClipId] = useState('')
  const [playheadMs, setPlayheadMs] = useState(0)
  const [zoom, setZoom] = useState(1)
  const [saveState, setSaveState] = useState<SaveState>('clean')
  const [editingRender, setEditingRender] = useState<ApiEditingRenderJob | null>(null)
  const [renderNotice, setRenderNotice] = useState('选择素材加入真实时间线，然后可排序、裁切、分割、删除、保存和导出。')
  const restoredVersionRef = useRef('')
  const timeline = history.present

  useEffect(() => {
    let active = true
    setAssetState('loading')
    void api.listProjectMediaAssets(currentProject.id).then(projectAssets => {
      if (!active) return
      const videos = projectAssets.filter(asset => asset.kind === 'video' && (asset.durationSeconds ?? 0) >= 0.25)
      setAssets(videos)
      setPreviewAssetId(current => current && videos.some(asset => asset.id === current) ? current : (videos[0]?.id ?? ''))
      setAssetState('ready')
    }).catch(() => {
      if (!active) return
      setAssets([])
      setAssetState('error')
    })
    return () => { active = false }
  }, [currentProject.id])

  useEffect(() => {
    let active = true
    restoredVersionRef.current = ''
    if (!editTaskId) {
      setEditTask(null)
      setHistory(createTimelineHistory(createEmptyEditorTimeline()))
      setSelectedClipId('')
      setPlayheadMs(0)
      setSaveState('clean')
      return () => { active = false }
    }
    void editingApi.get(currentProject.id, editTaskId).then(value => {
      if (active) setEditTask(value)
    }).catch(cause => {
      if (active) onNotice(cause instanceof Error ? cause.message : '素材剪辑任务读取失败')
    })
    return () => { active = false }
  }, [currentProject.id, editTaskId, onNotice])

  const timelineAssets = useMemo(() => assets.map(toTimelineAsset), [assets])
  useEffect(() => {
    if (!editTask || !timelineAssets.length) return
    const versionKey = `${editTask.id}:v${editTask.current_timeline.version}`
    if (restoredVersionRef.current === versionKey) return
    try {
      const restored = restoreEditorTimeline(editTask.current_timeline.timeline, timelineAssets)
      restoredVersionRef.current = versionKey
      setHistory(createTimelineHistory(restored))
      setSelectedClipId(restored.clips[0]?.id ?? '')
      setPlayheadMs(0)
      setSaveState('clean')
    } catch (cause) {
      onNotice(cause instanceof Error ? cause.message : '时间线恢复失败')
    }
  }, [editTask, onNotice, timelineAssets])

  const activePreview = assets.find(asset => asset.id === previewAssetId) ?? assets[0]
  const selectedClip = timeline.clips.find(clip => clip.id === selectedClipId)
  const selectedClipIndex = timeline.clips.findIndex(clip => clip.id === selectedClipId)
  const selectedAssetRefs = new Set(timeline.clips.map(clip => `${clip.assetId}:v${clip.assetVersion}`))

  useEffect(() => {
    if (selectedClipId && !timeline.clips.some(clip => clip.id === selectedClipId)) {
      setSelectedClipId(timeline.clips[0]?.id ?? '')
    }
    if (playheadMs > timeline.durationMs) setPlayheadMs(timeline.durationMs)
  }, [playheadMs, selectedClipId, timeline.clips, timeline.durationMs])

  const commitEdit = (change: (current: EditorTimeline) => EditorTimeline) => {
    setHistory(current => {
      const next = change(current.present)
      if (next === current.present) return current
      setSaveState('dirty')
      return commitTimeline(current, next)
    })
  }

  const addAssetToTimeline = (asset: ApiProjectMediaAsset) => {
    const timelineAsset = toTimelineAsset(asset)
    setHistory(current => {
      const next = addClip(current.present, timelineAsset)
      setSelectedClipId(next.clips.at(-1)?.id ?? '')
      setPlayheadMs(next.clips.at(-1)?.timelineStartMs ?? 0)
      setSaveState('dirty')
      return commitTimeline(current, next)
    })
    setPreviewAssetId(asset.id)
    onNotice(`${assetLabel(asset)} 已加入主视频轨。`)
  }

  const saveSnapshot = async (snapshot: EditorTimeline): Promise<ApiEditTask> => {
    if (!snapshot.clips.length) throw new Error('请先把至少一段视频加入时间线。')
    setSaveState('saving')
    try {
      const contract = toEditingTimeline(snapshot)
      const saved = editTask
        ? await editingApi.saveTimeline(currentProject.id, editTask.id, editTask.current_timeline.version, contract)
        : await editingApi.create(currentProject.id, { display_name: '素材剪辑', timeline: contract })
      restoredVersionRef.current = `${saved.id}:v${saved.current_timeline.version}`
      setEditTask(saved)
      setSaveState('clean')
      onNotice(`EditTask ${saved.id} 的时间线 v${saved.current_timeline.version} 已保存。`)
      if (!editTask) onOpenEditTask(saved.id)
      return saved
    } catch (cause) {
      const message = cause instanceof Error ? cause.message : '素材剪辑任务保存失败'
      setSaveState(cause instanceof EditingApiError && cause.status === 409 ? 'conflict' : 'error')
      throw cause
    }
  }

  const loadServerVersion = async () => {
    if (!editTask) return
    try {
      const serverTask = await editingApi.get(currentProject.id, editTask.id)
      const restored = restoreEditorTimeline(serverTask.current_timeline.timeline, timelineAssets)
      restoredVersionRef.current = `${serverTask.id}:v${serverTask.current_timeline.version}`
      setEditTask(serverTask)
      setHistory(createTimelineHistory(restored))
      setSelectedClipId(restored.clips[0]?.id ?? '')
      setPlayheadMs(0)
      setSaveState('clean')
      onNotice(`已载入服务端时间线 v${serverTask.current_timeline.version}。`)
    } catch (cause) {
      onNotice(cause instanceof Error ? cause.message : '服务端版本载入失败')
    }
  }

  const saveAsNewTask = async () => {
    if (!timeline.clips.length) return
    setSaveState('saving')
    try {
      const saved = await editingApi.create(currentProject.id, { display_name: `${editTask?.display_name ?? '素材剪辑'}（冲突副本）`, timeline: toEditingTimeline(timeline) })
      restoredVersionRef.current = `${saved.id}:v${saved.current_timeline.version}`
      setEditTask(saved)
      setSaveState('clean')
      onOpenEditTask(saved.id)
      onNotice(`冲突内容已另存为 EditTask ${saved.id}。`)
    } catch (cause) {
      setSaveState('error')
      onNotice(cause instanceof Error ? cause.message : '另存 EditTask 失败')
    }
  }

  useEffect(() => {
    if (!editTask || saveState !== 'dirty' || !timeline.clips.length) return
    const timer = window.setTimeout(() => {
      void saveSnapshot(timeline).catch(cause => onNotice(cause instanceof Error ? cause.message : '自动保存失败'))
    }, 1200)
    return () => window.clearTimeout(timer)
  }, [editTask, saveState, timeline])

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      const target = event.target as HTMLElement | null
      if (target?.closest('input, textarea, select')) return
      if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'z') {
        event.preventDefault()
        if (event.shiftKey) {
          setHistory(current => { const next = redoTimeline(current); if (next !== current) setSaveState('dirty'); return next })
        } else {
          setHistory(current => { const next = undoTimeline(current); if (next !== current) setSaveState('dirty'); return next })
        }
      }
      if ((event.key === 'Delete' || event.key === 'Backspace') && selectedClipId) {
        event.preventDefault()
        commitEdit(current => deleteClip(current, selectedClipId))
        setSelectedClipId('')
      }
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [selectedClipId])

  useEffect(() => {
    if (!editingRender || !['queued', 'running'].includes(editingRender.status)) return
    const timer = window.setInterval(() => {
      void editingApi.getRender(currentProject.id, editingRender.id).then(next => {
        setEditingRender(next)
        if (next.status === 'succeeded') setRenderNotice(`${next.kind === 'preview' ? '低清预览' : '正式导出'}已完成，成片已回流素材库。`)
        if (next.status === 'failed') setRenderNotice(`渲染失败${next.error_message ? `：${next.error_message}` : '，可以重试。'}`)
        if (next.status === 'cancelled') setRenderNotice('渲染任务已取消。')
      }).catch(cause => setRenderNotice(cause instanceof Error ? cause.message : '剪辑渲染状态读取失败'))
    }, 1200)
    return () => window.clearInterval(timer)
  }, [currentProject.id, editingRender])

  const createRender = async (kind: 'preview' | 'export') => {
    try {
      const task = !editTask || saveState !== 'clean' ? await saveSnapshot(timeline) : editTask
      const job = await editingApi.createRender(currentProject.id, task.id, kind)
      setEditingRender(job)
      setRenderNotice(`${kind === 'preview' ? '低清预览' : '正式导出'}已排队，绑定时间线 v${job.timeline.version}。`)
    } catch (cause) {
      setRenderNotice(cause instanceof Error ? cause.message : '渲染任务创建失败')
    }
  }

  const uploadVideo = async (file?: File) => {
    if (!file) return
    setUploading(true)
    try {
      const ref = await api.uploadProjectAsset(currentProject.id, file)
      const videos = (await api.listProjectMediaAssets(currentProject.id)).filter(asset => asset.kind === 'video')
      setAssets(videos)
      const uploaded = videos.find(asset => asset.id === ref.asset_id && asset.version === ref.version)
      if (uploaded) {
        setPreviewAssetId(uploaded.id)
        addAssetToTimeline(uploaded)
      }
      onNotice('视频已上传到项目素材库并加入时间线。')
    } catch (cause) {
      onNotice(cause instanceof Error ? cause.message : '视频上传失败')
    } finally {
      setUploading(false)
    }
  }

  const renderBusy = editingRender?.status === 'queued' || editingRender?.status === 'running'
  const saveLabel = !editTask && saveState === 'clean' ? '未创建' : saveState === 'clean' ? '已保存' : saveState === 'dirty' ? '有未保存修改' : saveState === 'saving' ? '保存中…' : saveState === 'conflict' ? '版本冲突' : '保存失败'

  return <div className="video-editing-workspace real-editor-workspace">
    <div className="editing-toolbar"><div><span className="section-label">EditTask · {editTask?.id ?? '未创建'}</span><b>{editTask?.display_name ?? '素材剪辑'}</b><small>时间线 {editTask ? `v${editTask.current_timeline.version}` : 'v0'} · {saveLabel}</small></div><div><button className="secondary-button" disabled={!timeline.clips.length || renderBusy} onClick={() => void createRender('preview')}><Play size={14} fill="currentColor"/>低清预览</button><button className="primary-button" disabled={!timeline.clips.length || renderBusy} onClick={() => void createRender('export')}><Download size={15}/>正式导出</button></div></div>
    <div className="editing-shell">
      <aside className="editing-assets video-asset-library">
        <div className="surface-toolbar"><h3>视频素材箱</h3><span>{assets.length} 个已入库</span></div>
        <label className="video-editor-upload"><Upload size={14}/>{uploading ? '上传处理中…' : '上传视频'}<input type="file" accept="video/mp4,video/quicktime,video/webm" disabled={uploading} onChange={event => { void uploadVideo(event.target.files?.[0]); event.currentTarget.value = '' }}/></label>
        <div className="video-library-scope"><span>全部项目视频</span><small>预览与加入时间线分开操作</small></div>
        <div className="asset-group video-asset-stage">
          {assetState === 'loading' ? <div className="panel-empty">正在加载项目视频…</div> : null}
          {assetState === 'error' ? <div className="panel-empty">素材箱加载失败，请刷新重试。</div> : null}
          {assetState === 'ready' && !assets.length ? <div className="panel-empty">暂无视频，可使用上方按钮上传。</div> : null}
          <div className="asset-card-flow">{assets.map((asset, index) => {
            const inTimeline = selectedAssetRefs.has(`${asset.id}:v${asset.version}`)
            return <article key={`${asset.id}:v${asset.version}`} className={`video-editor-asset-card poster-${index % 6}${previewAssetId === asset.id ? ' previewing' : ''}`}>
              <button type="button" className="asset-preview-button" onClick={() => setPreviewAssetId(asset.id)} aria-label={`预览 ${assetLabel(asset)}`}><span className="video-poster-frame"><span className="poster-glow"/><span className="poster-play"><Play size={13} fill="currentColor"/></span><b>{assetLabel(asset)}</b><small>{asset.durationSeconds?.toFixed(1)} 秒 · v{asset.version}</small></span></button>
              <button type="button" className="asset-add-button" onClick={() => addAssetToTimeline(asset)}><Scissors size={13}/>{inTimeline ? '再次加入时间线' : '加入时间线'}</button>
            </article>
          })}</div>
        </div>
        <div className="video-library-preview"><span>素材预览</span><b>{activePreview ? assetLabel(activePreview) : '请选择素材'}</b>{activePreview ? <video className="project-asset-preview" controls preload="metadata" src={activePreview.contentUrl}/> : null}</div>
      </aside>
      <section className="editing-center">
        <VideoPreviewPlayer timeline={timeline} playheadMs={playheadMs} onSeek={setPlayheadMs}/>
        <VideoTimeline
          timeline={timeline}
          selectedClipId={selectedClipId}
          playheadMs={playheadMs}
          zoom={zoom}
          canUndo={history.past.length > 0}
          canRedo={history.future.length > 0}
          onSelectClip={setSelectedClipId}
          onSeek={setPlayheadMs}
          onZoomChange={setZoom}
          onMoveClip={(clipId, targetIndex) => commitEdit(current => moveClip(current, clipId, targetIndex))}
          onTrimClip={(clipId, sourceInMs, sourceOutMs) => commitEdit(current => trimClip(current, clipId, sourceInMs, sourceOutMs))}
          onSplitClip={() => { if (selectedClipId) { const rightId = `${selectedClipId}-b`; commitEdit(current => splitClip(current, selectedClipId, playheadMs)); setSelectedClipId(rightId) } }}
          onDeleteClip={() => { if (selectedClipId) commitEdit(current => deleteClip(current, selectedClipId)); setSelectedClipId('') }}
          onUndo={() => setHistory(current => { const next = undoTimeline(current); if (next !== current) setSaveState('dirty'); return next })}
          onRedo={() => setHistory(current => { const next = redoTimeline(current); if (next !== current) setSaveState('dirty'); return next })}
        />
      </section>
      <aside className="editing-inspector">
        <div className="surface-toolbar"><h3>剪辑任务</h3><span className={`status ${saveState === 'clean' ? 'success' : saveState === 'error' || saveState === 'conflict' ? 'danger' : 'pending'}`}><span/>{saveLabel}</span></div>
        <div className="inspector-section"><span>固定输出规格</span><b>720 × 1280 · 9:16</b><small>MP4 / H.264 / AAC · 30fps · 48kHz</small></div>
        {selectedClip ? <div className="inspector-section selected-clip-inspector"><span>当前片段</span><b>{selectedClip.name}</b><small>素材 {shortId(selectedClip.assetId)} · v{selectedClip.assetVersion}</small><dl><div><dt>时间线</dt><dd>{formatTime(selectedClip.timelineStartMs)}–{formatTime(selectedClip.timelineEndMs)}</dd></div><div><dt>源片裁切</dt><dd>{formatTime(selectedClip.sourceInMs)}–{formatTime(selectedClip.sourceOutMs)}</dd></div></dl><div className="clip-nudge-actions"><button type="button" disabled={selectedClipIndex <= 0} onClick={() => commitEdit(current => moveClip(current, selectedClip.id, selectedClipIndex - 1))}>向左移动</button><button type="button" disabled={selectedClipIndex < 0 || selectedClipIndex >= timeline.clips.length - 1} onClick={() => commitEdit(current => moveClip(current, selectedClip.id, selectedClipIndex + 1))}>向右移动</button><button type="button" disabled={selectedClip.sourceInMs <= 0} onClick={() => commitEdit(current => trimClip(current, selectedClip.id, Math.max(0, selectedClip.sourceInMs - 500), selectedClip.sourceOutMs))}>左扩 0.5s</button><button type="button" disabled={selectedClip.sourceOutMs >= selectedClip.sourceDurationMs} onClick={() => commitEdit(current => trimClip(current, selectedClip.id, selectedClip.sourceInMs, Math.min(selectedClip.sourceDurationMs, selectedClip.sourceOutMs + 500)))}>右扩 0.5s</button></div></div> : <div className="inspector-section"><span>当前片段</span><b>请在时间线上选择片段</b><small>选中后可裁切、分割和删除。</small></div>}
        <div className="editing-checks"><span><Check size={14}/>{timeline.clips.length} 个主视频片段</span><span><Check size={14}/>原素材使用 Asset ID/Version</span><span><Check size={14}/>自动保存与版本快照</span><span>{editingRender ? `渲染 ${editingRender.progress_percent}% · ${editingRender.kind}` : '尚未创建渲染任务'}</span></div>
        {saveState === 'conflict' ? <div className="timeline-conflict-actions" role="alert"><b>服务器已有更新</b><small>可放弃本地改动载入最新版，或保留当前内容另存为新任务。</small><button className="secondary-button full" type="button" onClick={() => void loadServerVersion()}>载入服务端版本</button><button className="secondary-button full" type="button" onClick={() => void saveAsNewTask()}>另存为新 EditTask</button></div> : null}
        <button className="primary-button full" disabled={!timeline.clips.length || saveState === 'saving'} onClick={() => void saveSnapshot(timeline).catch(cause => onNotice(cause instanceof Error ? cause.message : '保存失败'))}><Save size={15}/>{editTask ? '保存时间线新版本' : '创建 EditTask'}</button>
        <button className="secondary-button full" disabled={!timeline.clips.length || renderBusy} onClick={() => void createRender('preview')}><Play size={15}/>创建低清预览</button>
        <button className="secondary-button full" disabled={!timeline.clips.length || renderBusy} onClick={() => void createRender('export')}><Download size={15}/>创建正式导出</button>
        {editingRender?.status === 'failed' ? <button className="secondary-button full" onClick={() => void editingApi.retryRender(currentProject.id, editingRender.id).then(setEditingRender)}>重试渲染</button> : null}
        {renderBusy ? <button className="secondary-button full" onClick={() => void editingApi.cancelRender(currentProject.id, editingRender!.id).then(setEditingRender)}>取消渲染</button> : null}
        {editingRender?.output_asset ? <a className="secondary-button full" href={`/platform/v1/projects/${currentProject.id}/assets/${editingRender.output_asset.asset_version.asset_id}/versions/${editingRender.output_asset.asset_version.version}/preview`} target="_blank" rel="noreferrer">打开成片预览</a> : null}
        <div className="inline-notice" role="status">{renderNotice}</div>
      </aside>
    </div>
  </div>
}

function toTimelineAsset(asset: ApiProjectMediaAsset): TimelineAsset {
  return {
    assetId: asset.id,
    assetVersion: asset.version,
    durationMs: Math.max(250, Math.round((asset.durationSeconds ?? 0) * 1000)),
    name: assetLabel(asset),
    previewUrl: asset.contentUrl,
  }
}

function assetLabel(asset: ApiProjectMediaAsset): string {
  return `${asset.sourceType === 'provider_generated' ? '生成视频' : asset.sourceType === 'rendered' ? '导出视频' : '导入视频'} · ${shortId(asset.id)}`
}

function formatTime(timeMs: number): string {
  return `${(timeMs / 1000).toFixed(1)}s`
}
