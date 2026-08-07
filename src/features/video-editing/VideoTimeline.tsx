import { useEffect, useMemo, useRef, useState, type PointerEvent as ReactPointerEvent } from 'react'
import { Redo2, Scissors, Trash2, Undo2, ZoomIn, ZoomOut } from 'lucide-react'

import type { EditorClip, EditorTimeline } from './timeline'

type TrimDraft = { clipId: string; sourceInMs: number; sourceOutMs: number } | null

export type VideoTimelineProps = {
  timeline: EditorTimeline
  selectedClipId: string
  playheadMs: number
  zoom: number
  canUndo: boolean
  canRedo: boolean
  onSelectClip: (clipId: string) => void
  onSeek: (timeMs: number) => void
  onZoomChange: (zoom: number) => void
  onMoveClip: (clipId: string, targetIndex: number) => void
  onTrimClip: (clipId: string, sourceInMs: number, sourceOutMs: number) => void
  onSplitClip: () => void
  onDeleteClip: () => void
  onUndo: () => void
  onRedo: () => void
}

const BASE_PIXELS_PER_SECOND = 44
const MIN_CLIP_DURATION_MS = 250

export function VideoTimeline({
  timeline,
  selectedClipId,
  playheadMs,
  zoom,
  canUndo,
  canRedo,
  onSelectClip,
  onSeek,
  onZoomChange,
  onMoveClip,
  onTrimClip,
  onSplitClip,
  onDeleteClip,
  onUndo,
  onRedo,
}: VideoTimelineProps) {
  const laneRef = useRef<HTMLDivElement | null>(null)
  const trimSession = useRef<{ clip: EditorClip; side: 'left' | 'right'; startX: number } | null>(null)
  const trimDraftRef = useRef<TrimDraft>(null)
  const draggingPlayheadRef = useRef(false)
  const [trimDraft, setTrimDraft] = useState<TrimDraft>(null)
  const [draggingClipId, setDraggingClipId] = useState('')
  const pixelsPerMs = BASE_PIXELS_PER_SECOND * zoom / 1000
  const laneWidth = Math.max(620, timeline.durationMs * pixelsPerMs + 80)
  const selectedClip = timeline.clips.find(clip => clip.id === selectedClipId)

  const displayClips = useMemo(() => timeline.clips.map(clip => {
    if (!trimDraft || trimDraft.clipId !== clip.id) return clip
    const durationMs = trimDraft.sourceOutMs - trimDraft.sourceInMs
    return {
      ...clip,
      sourceInMs: trimDraft.sourceInMs,
      sourceOutMs: trimDraft.sourceOutMs,
      timelineEndMs: clip.timelineStartMs + durationMs,
    }
  }), [timeline.clips, trimDraft])

  useEffect(() => {
    const handlePointerMove = (event: PointerEvent) => {
      const session = trimSession.current
      if (!session) return
      const deltaMs = Math.round((event.clientX - session.startX) / pixelsPerMs)
      if (session.side === 'left') {
        const sourceInMs = Math.max(0, Math.min(session.clip.sourceOutMs - MIN_CLIP_DURATION_MS, session.clip.sourceInMs + deltaMs))
        const draft = { clipId: session.clip.id, sourceInMs, sourceOutMs: session.clip.sourceOutMs }
        trimDraftRef.current = draft
        setTrimDraft(draft)
      } else {
        const sourceOutMs = Math.max(session.clip.sourceInMs + MIN_CLIP_DURATION_MS, Math.min(session.clip.sourceDurationMs, session.clip.sourceOutMs + deltaMs))
        const draft = { clipId: session.clip.id, sourceInMs: session.clip.sourceInMs, sourceOutMs }
        trimDraftRef.current = draft
        setTrimDraft(draft)
      }
    }
    const handlePointerUp = () => {
      const draft = trimDraftRef.current
      trimSession.current = null
      if (draft) onTrimClip(draft.clipId, draft.sourceInMs, draft.sourceOutMs)
      trimDraftRef.current = null
      setTrimDraft(null)
    }
    window.addEventListener('pointermove', handlePointerMove)
    window.addEventListener('pointerup', handlePointerUp)
    return () => {
      window.removeEventListener('pointermove', handlePointerMove)
      window.removeEventListener('pointerup', handlePointerUp)
    }
  }, [onTrimClip, pixelsPerMs])

  const startTrim = (event: ReactPointerEvent, clip: EditorClip, side: 'left' | 'right') => {
    event.preventDefault()
    event.stopPropagation()
    onSelectClip(clip.id)
    trimSession.current = { clip, side, startX: event.clientX }
    const draft = { clipId: clip.id, sourceInMs: clip.sourceInMs, sourceOutMs: clip.sourceOutMs }
    trimDraftRef.current = draft
    setTrimDraft(draft)
  }

  const seekFromPointer = (clientX: number) => {
    const lane = laneRef.current
    if (!lane) return
    const timeMs = (clientX - lane.getBoundingClientRect().left + lane.scrollLeft) / pixelsPerMs
    onSeek(Math.max(0, Math.min(timeline.durationMs, Math.round(timeMs))))
  }

  useEffect(() => {
    const handlePointerMove = (event: PointerEvent) => {
      if (draggingPlayheadRef.current) seekFromPointer(event.clientX)
    }
    const handlePointerUp = () => { draggingPlayheadRef.current = false }
    window.addEventListener('pointermove', handlePointerMove)
    window.addEventListener('pointerup', handlePointerUp)
    return () => {
      window.removeEventListener('pointermove', handlePointerMove)
      window.removeEventListener('pointerup', handlePointerUp)
    }
  }, [pixelsPerMs, timeline.durationMs])

  return <section className="real-video-timeline" aria-label="视频剪辑时间线">
    <div className="real-timeline-toolbar">
      <div className="timeline-edit-actions">
        <button type="button" onClick={onUndo} disabled={!canUndo} title="撤销 Ctrl+Z"><Undo2 size={14}/>撤销</button>
        <button type="button" onClick={onRedo} disabled={!canRedo} title="重做 Ctrl+Shift+Z"><Redo2 size={14}/>重做</button>
        <button type="button" onClick={() => onSeek(Math.max(0, playheadMs - 500))} disabled={!timeline.clips.length || playheadMs <= 0}>后退 0.5s</button>
        <button type="button" onClick={() => onSeek(Math.min(timeline.durationMs, playheadMs + 500))} disabled={!timeline.clips.length || playheadMs >= timeline.durationMs}>前进 0.5s</button>
        <button type="button" onClick={onSplitClip} disabled={!selectedClip || playheadMs <= selectedClip.timelineStartMs || playheadMs >= selectedClip.timelineEndMs}><Scissors size={14}/>在播放头分割</button>
        <button type="button" onClick={onDeleteClip} disabled={!selectedClip}><Trash2 size={14}/>删除片段</button>
      </div>
      <label className="timeline-zoom"><ZoomOut size={13}/><input aria-label="时间线缩放" type="range" min="0.55" max="2.4" step="0.05" value={zoom} onChange={event => onZoomChange(Number(event.target.value))}/><ZoomIn size={13}/><span>{Math.round(zoom * 100)}%</span></label>
    </div>
    <div className="timeline-scroll" ref={laneRef} onClick={event => {
      if ((event.target as HTMLElement).closest('.real-timeline-clip')) return
      seekFromPointer(event.clientX)
    }}>
      <div className="timeline-canvas" style={{ width: laneWidth }}>
        <div className="timeline-ruler" aria-hidden="true">{Array.from({ length: Math.ceil(timeline.durationMs / 5000) + 1 }, (_, index) => <span key={index} style={{ left: index * 5000 * pixelsPerMs }}>{index * 5}s</span>)}</div>
        <div className="timeline-track-label">主视频轨</div>
        <div className="timeline-primary-lane">
          {displayClips.map((clip, index) => <button
            type="button"
            key={clip.id}
            className={`real-timeline-clip${selectedClipId === clip.id ? ' selected' : ''}${draggingClipId === clip.id ? ' dragging' : ''}`}
            style={{ left: clip.timelineStartMs * pixelsPerMs, width: Math.max(44, (clip.sourceOutMs - clip.sourceInMs) * pixelsPerMs) }}
            draggable
            onClick={event => { event.stopPropagation(); onSelectClip(clip.id); onSeek(clip.timelineStartMs) }}
            onDragStart={event => { setDraggingClipId(clip.id); event.dataTransfer.setData('text/plain', clip.id); event.dataTransfer.effectAllowed = 'move' }}
            onDragEnd={() => setDraggingClipId('')}
            onDragOver={event => { event.preventDefault(); event.dataTransfer.dropEffect = 'move' }}
            onDrop={event => { event.preventDefault(); const sourceId = event.dataTransfer.getData('text/plain'); if (sourceId) onMoveClip(sourceId, index); setDraggingClipId('') }}
            aria-pressed={selectedClipId === clip.id}
            aria-label={`${clip.name}，${formatTime(clip.sourceOutMs - clip.sourceInMs)}`}
          >
            <span className="clip-trim-handle left" role="slider" aria-label={`${clip.name} 左裁切手柄`} onPointerDown={event => startTrim(event, clip, 'left')}/>
            <span className="clip-thumbnail" style={{ backgroundImage: `linear-gradient(90deg, rgba(7,22,40,.2), rgba(7,22,40,.8)), url(${JSON.stringify(clip.previewUrl).slice(1, -1)})` }}/>
            <span className="clip-copy"><b>{clip.name}</b><small>{formatTime(clip.sourceOutMs - clip.sourceInMs)} · v{clip.assetVersion}</small></span>
            <span className="clip-trim-handle right" role="slider" aria-label={`${clip.name} 右裁切手柄`} onPointerDown={event => startTrim(event, clip, 'right')}/>
          </button>)}
          {!timeline.clips.length ? <div className="timeline-empty">从左侧选择或拖入视频，开始剪辑</div> : null}
        </div>
        <button type="button" className="timeline-playhead" aria-label={`播放头 ${formatTime(playheadMs)}`} style={{ left: playheadMs * pixelsPerMs }} onPointerDown={event => { event.preventDefault(); event.stopPropagation(); draggingPlayheadRef.current = true; seekFromPointer(event.clientX) }}><span/></button>
      </div>
    </div>
    <footer className="timeline-status"><span>{timeline.clips.length} 个片段 · {formatTime(timeline.durationMs)}</span><span>拖动排序 · 拖片段两端裁切 · 拖动播放头定位</span></footer>
  </section>
}

function formatTime(timeMs: number): string {
  const totalSeconds = Math.max(0, timeMs) / 1000
  const minutes = Math.floor(totalSeconds / 60)
  const seconds = totalSeconds - minutes * 60
  return `${String(minutes).padStart(2, '0')}:${seconds.toFixed(1).padStart(4, '0')}`
}
