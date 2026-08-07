import type { EditingTimeline } from './api'

// Timeline command/history semantics are adapted from OpenCut Classic's
// MoveElementCommand, SplitElementsCommand, resize controller, and
// CommandManager at cf5e79e919144200294fb9fed22a222592a0aeea. The cookies
// implementation deliberately uses its own AssetVersionRef and millisecond
// timeline contract. See third_party/opencut-timeline/NOTICE.md.

export type TimelineAsset = {
  assetId: string
  assetVersion: number
  durationMs: number
  name: string
  previewUrl: string
}

export type EditorClip = {
  id: string
  assetId: string
  assetVersion: number
  name: string
  previewUrl: string
  timelineStartMs: number
  timelineEndMs: number
  sourceInMs: number
  sourceOutMs: number
  sourceDurationMs: number
}

export type EditorTimeline = {
  clips: EditorClip[]
  durationMs: number
}

export type TimelineHistory = {
  past: EditorTimeline[]
  present: EditorTimeline
  future: EditorTimeline[]
}

export function createEmptyEditorTimeline(): EditorTimeline {
  return { clips: [], durationMs: 0 }
}

export function createTimelineHistory(timeline: EditorTimeline): TimelineHistory {
  return { past: [], present: timeline, future: [] }
}

export function commitTimeline(history: TimelineHistory, timeline: EditorTimeline): TimelineHistory {
  if (timeline === history.present) return history
  return { past: [...history.past, history.present], present: timeline, future: [] }
}

export function undoTimeline(history: TimelineHistory): TimelineHistory {
  const previous = history.past.at(-1)
  if (!previous) return history
  return { past: history.past.slice(0, -1), present: previous, future: [history.present, ...history.future] }
}

export function redoTimeline(history: TimelineHistory): TimelineHistory {
  const next = history.future[0]
  if (!next) return history
  return { past: [...history.past, history.present], present: next, future: history.future.slice(1) }
}

export function addClip(timeline: EditorTimeline, asset: TimelineAsset): EditorTimeline {
  const ordinal = timeline.clips.filter(clip => clip.assetId === asset.assetId && clip.assetVersion === asset.assetVersion).length + 1
  const start = timeline.durationMs
  return {
    clips: [...timeline.clips, {
      id: `clip-${asset.assetId}-v${asset.assetVersion}-${ordinal}`,
      assetId: asset.assetId,
      assetVersion: asset.assetVersion,
      name: asset.name,
      previewUrl: asset.previewUrl,
      timelineStartMs: start,
      timelineEndMs: start + asset.durationMs,
      sourceInMs: 0,
      sourceOutMs: asset.durationMs,
      sourceDurationMs: asset.durationMs,
    }],
    durationMs: start + asset.durationMs,
  }
}

export function moveClip(timeline: EditorTimeline, clipId: string, targetIndex: number): EditorTimeline {
  const currentIndex = timeline.clips.findIndex(clip => clip.id === clipId)
  if (currentIndex < 0) return timeline
  const clips = [...timeline.clips]
  const [clip] = clips.splice(currentIndex, 1)
  const boundedIndex = Math.max(0, Math.min(targetIndex, clips.length))
  clips.splice(boundedIndex, 0, clip)
  return normalizeTimeline(clips)
}

export function trimClip(timeline: EditorTimeline, clipId: string, sourceInMs: number, sourceOutMs: number): EditorTimeline {
  const clip = timeline.clips.find(item => item.id === clipId)
  if (!clip) return timeline
  if (sourceInMs < 0 || sourceOutMs > clip.sourceDurationMs || sourceOutMs <= sourceInMs) {
    throw new RangeError('裁切范围必须位于素材时长内。')
  }
  return normalizeTimeline(timeline.clips.map(item => item.id === clipId ? { ...item, sourceInMs, sourceOutMs } : item))
}

export function splitClip(timeline: EditorTimeline, clipId: string, playheadMs: number): EditorTimeline {
  const index = timeline.clips.findIndex(clip => clip.id === clipId)
  if (index < 0) return timeline
  const clip = timeline.clips[index]
  if (playheadMs <= clip.timelineStartMs || playheadMs >= clip.timelineEndMs) return timeline
  const sourceSplitMs = clip.sourceInMs + playheadMs - clip.timelineStartMs
  const clips = [...timeline.clips]
  clips.splice(index, 1,
    { ...clip, id: `${clip.id}-a`, sourceOutMs: sourceSplitMs },
    { ...clip, id: `${clip.id}-b`, sourceInMs: sourceSplitMs },
  )
  return normalizeTimeline(clips)
}

export function deleteClip(timeline: EditorTimeline, clipId: string): EditorTimeline {
  if (!timeline.clips.some(clip => clip.id === clipId)) return timeline
  return normalizeTimeline(timeline.clips.filter(clip => clip.id !== clipId))
}

export function toEditingTimeline(timeline: EditorTimeline): EditingTimeline {
  return {
    schema_version: 'editing-timeline/v1',
    output_profile: { id: 'cookies-editing-vertical-v1', width: 720, height: 1280, frame_rate: 30, sample_rate: 48000 },
    duration_ms: timeline.durationMs,
    tracks: [{
      id: 'video-primary',
      role: 'primary_video',
      clips: timeline.clips.map(clip => ({
        id: clip.id,
        asset_ref: { asset_id: clip.assetId, version: clip.assetVersion },
        timeline_start_ms: clip.timelineStartMs,
        timeline_end_ms: clip.timelineEndMs,
        source_in_ms: clip.sourceInMs,
        source_out_ms: clip.sourceOutMs,
      })),
    }],
  }
}

export function restoreEditorTimeline(timeline: EditingTimeline, assets: TimelineAsset[]): EditorTimeline {
  const assetsByRef = new Map(assets.map(asset => [`${asset.assetId}:v${asset.assetVersion}`, asset]))
  const primary = timeline.tracks.find(track => track.role === 'primary_video')
  if (!primary) return createEmptyEditorTimeline()
  const clips = primary.clips.map(clip => {
    if (!clip.asset_ref) throw new Error(`镜头 ${clip.id} 缺少素材引用。`)
    const asset = assetsByRef.get(`${clip.asset_ref.asset_id}:v${clip.asset_ref.version}`)
    if (!asset) throw new Error(`镜头 ${clip.id} 引用的素材版本当前不可用。`)
    return {
      id: clip.id,
      assetId: asset.assetId,
      assetVersion: asset.assetVersion,
      name: asset.name,
      previewUrl: asset.previewUrl,
      timelineStartMs: clip.timeline_start_ms,
      timelineEndMs: clip.timeline_end_ms,
      sourceInMs: clip.source_in_ms ?? 0,
      sourceOutMs: clip.source_out_ms ?? clip.timeline_end_ms - clip.timeline_start_ms,
      sourceDurationMs: asset.durationMs,
    }
  })
  return { clips, durationMs: timeline.duration_ms }
}

function normalizeTimeline(clips: EditorClip[]): EditorTimeline {
  let cursor = 0
  const normalized = clips.map(clip => {
    const duration = clip.sourceOutMs - clip.sourceInMs
    const value = { ...clip, timelineStartMs: cursor, timelineEndMs: cursor + duration }
    cursor += duration
    return value
  })
  return { clips: normalized, durationMs: cursor }
}

export type { EditingTimeline }
