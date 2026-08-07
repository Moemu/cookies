import { useEffect, useMemo, useRef, useState } from 'react'
import { Pause, Play } from 'lucide-react'

import type { EditorTimeline } from './timeline'

export function VideoPreviewPlayer({ timeline, playheadMs, onSeek }: {
  timeline: EditorTimeline
  playheadMs: number
  onSeek: (timeMs: number) => void
}) {
  const videoRef = useRef<HTMLVideoElement | null>(null)
  const [playing, setPlaying] = useState(false)
  const activeClip = useMemo(() => timeline.clips.find(clip => playheadMs >= clip.timelineStartMs && playheadMs < clip.timelineEndMs) ?? timeline.clips.at(-1), [playheadMs, timeline.clips])
  const activeClipId = activeClip?.id ?? ''

  useEffect(() => {
    const video = videoRef.current
    if (!video || !activeClip) return
    const targetSeconds = (activeClip.sourceInMs + Math.max(0, playheadMs - activeClip.timelineStartMs)) / 1000
    if (Math.abs(video.currentTime - targetSeconds) > 0.35) video.currentTime = targetSeconds
  }, [activeClip, activeClipId, playheadMs])

  useEffect(() => {
    if (!playing || !activeClip) return
    const video = videoRef.current
    if (!video) return
    void video.play().catch(() => setPlaying(false))
  }, [activeClipId, activeClip, playing])

  if (!activeClip) {
    return <div className="editing-preview real-preview empty"><div className="editing-safe-frame"><span>9:16</span><b>把项目视频加入时间线</b><small>READY TO EDIT</small></div><time>00:00.0 / 00:00.0</time></div>
  }

  const togglePlayback = () => {
    const video = videoRef.current
    if (!video) return
    if (playing) {
      video.pause()
      setPlaying(false)
      return
    }
    setPlaying(true)
    void video.play().catch(() => setPlaying(false))
  }

  return <div className="editing-preview real-preview">
    <video
      ref={videoRef}
      key={activeClip.id}
      src={activeClip.previewUrl}
      preload="metadata"
      playsInline
      onLoadedMetadata={event => {
        event.currentTarget.currentTime = (activeClip.sourceInMs + Math.max(0, playheadMs - activeClip.timelineStartMs)) / 1000
        if (playing) void event.currentTarget.play()
      }}
      onTimeUpdate={event => {
        if (!playing) return
        const sourceTimeMs = event.currentTarget.currentTime * 1000
        const sourceOffsetMs = Math.max(0, sourceTimeMs - activeClip.sourceInMs)
        const nextTime = Math.min(activeClip.timelineEndMs, activeClip.timelineStartMs + sourceOffsetMs)
        onSeek(Math.round(nextTime))
        if (sourceTimeMs >= activeClip.sourceOutMs - 80) {
          const nextClip = timeline.clips[timeline.clips.findIndex(clip => clip.id === activeClip.id) + 1]
          if (nextClip) onSeek(nextClip.timelineStartMs)
          else setPlaying(false)
        }
      }}
      onPause={() => setPlaying(false)}
      onPlay={() => setPlaying(true)}
    />
    <button type="button" aria-label={playing ? '暂停时间线预览' : '播放时间线预览'} onClick={togglePlayback}>{playing ? <Pause size={18} fill="currentColor"/> : <Play size={18} fill="currentColor"/>}</button>
    <div className="preview-clip-label"><b>{activeClip.name}</b><small>源片 {formatTime(activeClip.sourceInMs)}–{formatTime(activeClip.sourceOutMs)}</small></div>
    <time>{formatTime(playheadMs)} / {formatTime(timeline.durationMs)}</time>
  </div>
}

function formatTime(timeMs: number): string {
  const totalSeconds = Math.max(0, timeMs) / 1000
  const minutes = Math.floor(totalSeconds / 60)
  return `${String(minutes).padStart(2, '0')}:${(totalSeconds - minutes * 60).toFixed(1).padStart(4, '0')}`
}
