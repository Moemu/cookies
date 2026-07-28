import type { ProjectAsset } from './types'

export type RemixSegment = 'opening' | 'middle' | 'ending'
export type RemixPace = 'balanced' | 'fast' | 'story'

export type RemixSelection = Record<RemixSegment, ProjectAsset[]>

export type RemixClip = {
  id: string
  segment: RemixSegment
  assetId: string
  version: number
  label: string
  sourceType: string
  mimeType: string
  aspect: 'vertical' | 'horizontal' | 'square' | 'unknown'
  startSeconds: number
  durationSeconds: number
  inPointSeconds: number
  outPointSeconds: number
  score: number
  reason: string
}

export type RemixSegmentPlan = {
  segment: RemixSegment
  label: string
  targetSeconds: number
  actualSeconds: number
  clips: RemixClip[]
}

export type BulkRemixPlan = {
  id: string
  targetSeconds: number
  actualSeconds: number
  pace: RemixPace
  segments: RemixSegmentPlan[]
  warnings: string[]
  summary: {
    selectedAssets: number
    usedAssets: number
    coveragePercent: number
    strategy: string
  }
}

export type BuildBulkRemixPlanInput = {
  selection: RemixSelection
  targetSeconds: number
  pace: RemixPace
  maxClipsPerSegment?: number
  now?: Date
}

const segmentLabels: Record<RemixSegment, string> = {
  opening: '前段',
  middle: '中段',
  ending: '后段',
}

const segmentWeights: Record<RemixSegment, number> = {
  opening: 0.25,
  middle: 0.5,
  ending: 0.25,
}

const paceClipBounds: Record<RemixPace, { min: number; max: number; strategy: string }> = {
  fast: { min: 1.6, max: 3.2, strategy: '快节奏：短切、高密度、优先强钩子素材' },
  balanced: { min: 2.4, max: 4.8, strategy: '均衡节奏：兼顾钩子、叙事和收束' },
  story: { min: 3.6, max: 6.8, strategy: '叙事节奏：保留更长镜头，优先连续表达' },
}

export function buildBulkRemixPlan(input: BuildBulkRemixPlanInput): BulkRemixPlan {
  const targetSeconds = clamp(Math.round(input.targetSeconds || 30), 9, 180)
  const maxClipsPerSegment = clamp(input.maxClipsPerSegment ?? 24, 1, 80)
  const now = input.now ?? new Date()
  const segments: RemixSegmentPlan[] = []
  const warnings: string[] = ['当前版本使用文件大小估算素材时长，接入视频 duration 字段后会自动替换为真实片长。']
  let cursor = 0

  for (const segment of segmentOrder()) {
    const target = Math.round(targetSeconds * segmentWeights[segment])
    const candidates = dedupeAssets(input.selection[segment]).filter(isVideoAsset)
    if (candidates.length === 0) {
      warnings.push(`${segmentLabels[segment]}没有可用视频素材。`)
    }
    const clips = pickSegmentClips(segment, candidates, target, input.pace, maxClipsPerSegment, cursor, now)
    const actual = roundSeconds(clips.reduce((sum, clip) => sum + clip.durationSeconds, 0))
    if (actual < target * 0.65 && candidates.length > 0) {
      warnings.push(`${segmentLabels[segment]}素材不足，当前只覆盖目标时长的 ${Math.round((actual / target) * 100)}%。`)
    }
    segments.push({ segment, label: segmentLabels[segment], targetSeconds: target, actualSeconds: actual, clips })
    cursor = roundSeconds(cursor + actual)
  }

  const selectedAssets = segmentOrder().reduce((sum, segment) => sum + dedupeAssets(input.selection[segment]).filter(isVideoAsset).length, 0)
  const usedAssets = new Set(segments.flatMap((segment) => segment.clips.map((clip) => clip.assetId))).size
  const actualSeconds = roundSeconds(segments.reduce((sum, segment) => sum + segment.actualSeconds, 0))
  const coveragePercent = targetSeconds > 0 ? Math.min(100, Math.round((actualSeconds / targetSeconds) * 100)) : 0

  return {
    id: `remix_${stableHash(`${targetSeconds}:${input.pace}:${selectedAssets}:${actualSeconds}`)}`,
    targetSeconds,
    actualSeconds,
    pace: input.pace,
    segments,
    warnings,
    summary: {
      selectedAssets,
      usedAssets,
      coveragePercent,
      strategy: paceClipBounds[input.pace].strategy,
    },
  }
}

export function isVideoAsset(asset: ProjectAsset) {
  return asset.asset.asset_kind === 'video' || asset.version.mime_type.startsWith('video/')
}

export function estimateAssetDurationSeconds(asset: ProjectAsset, pace: RemixPace) {
  const bounds = paceClipBounds[pace]
  const sizeMB = Math.max(0.1, asset.version.size_bytes / 1024 / 1024)
  const estimated = 1.8 + Math.log2(sizeMB + 1) * 1.25
  return roundSeconds(clamp(estimated, bounds.min, bounds.max))
}

function pickSegmentClips(segment: RemixSegment, candidates: ProjectAsset[], targetSeconds: number, pace: RemixPace, maxClips: number, startCursor: number, now: Date) {
  const ranked = candidates
    .map((asset) => ({ asset, score: scoreAssetForSegment(asset, segment, now) }))
    .sort((left, right) => right.score - left.score || left.asset.asset.id.localeCompare(right.asset.asset.id))
  const clips: RemixClip[] = []
  const usedAspects = new Set<string>()
  const usedSources = new Set<string>()
  let cursor = startCursor

  for (const candidate of ranked) {
    if (clips.length >= maxClips) break
    const currentDuration = clips.reduce((sum, clip) => sum + clip.durationSeconds, 0)
    if (currentDuration >= targetSeconds * 0.98) break
    const aspect = assetAspect(candidate.asset)
    const source = candidate.asset.version.source_type
    const diversityPenalty = (usedAspects.has(aspect) ? 0.05 : 0) + (usedSources.has(source) ? 0.04 : 0)
    const duration = Math.min(estimateAssetDurationSeconds(candidate.asset, pace), Math.max(1.2, targetSeconds - currentDuration))
    const score = roundScore(candidate.score - diversityPenalty)
    clips.push({
      id: `${segment}_${candidate.asset.asset.id}_${candidate.asset.version.version}`,
      segment,
      assetId: candidate.asset.asset.id,
      version: candidate.asset.version.version,
      label: `${candidate.asset.asset.id} · v${candidate.asset.version.version}`,
      sourceType: source,
      mimeType: candidate.asset.version.mime_type,
      aspect,
      startSeconds: roundSeconds(cursor),
      durationSeconds: roundSeconds(duration),
      inPointSeconds: 0,
      outPointSeconds: roundSeconds(duration),
      score,
      reason: explainSelection(segment, candidate.asset, score),
    })
    cursor = roundSeconds(cursor + duration)
    usedAspects.add(aspect)
    usedSources.add(source)
  }
  return clips
}

function scoreAssetForSegment(asset: ProjectAsset, segment: RemixSegment, now: Date) {
  let score = 0.45
  const ageDays = Math.max(0, (now.getTime() - new Date(asset.created_at).getTime()) / 86400000)
  score += Math.max(0, 0.18 - ageDays * 0.006)
  if (asset.version.width_pixels && asset.version.height_pixels) score += 0.1
  if (asset.version.source_type === 'captured') score += segment === 'opening' ? 0.18 : 0.08
  if (asset.version.source_type === 'provider_generated') score += segment === 'middle' ? 0.12 : 0.08
  if (asset.version.source_type === 'upload') score += segment === 'ending' ? 0.12 : 0.05
  const aspect = assetAspect(asset)
  if (aspect === 'vertical' && segment === 'opening') score += 0.12
  if (aspect === 'horizontal' && segment === 'middle') score += 0.08
  if (aspect === 'square' && segment === 'ending') score += 0.06
  score += Math.min(0.1, Math.log2(Math.max(1, asset.version.size_bytes / 1024 / 1024)) * 0.02)
  return roundScore(clamp(score, 0, 1))
}

function explainSelection(segment: RemixSegment, asset: ProjectAsset, score: number) {
  const signals = [`${segmentLabels[segment]}匹配分 ${Math.round(score * 100)}`]
  const aspect = assetAspect(asset)
  if (aspect !== 'unknown') signals.push(`${aspectLabel(aspect)}画幅`)
  signals.push(sourceLabel(asset.version.source_type))
  if (asset.version.width_pixels && asset.version.height_pixels) signals.push('尺寸完整')
  return signals.join(' · ')
}

function dedupeAssets(assets: ProjectAsset[]) {
  const seen = new Set<string>()
  return assets.filter((asset) => {
    const key = `${asset.asset.id}:${asset.version.version}`
    if (seen.has(key)) return false
    seen.add(key)
    return true
  })
}

function assetAspect(asset: ProjectAsset): RemixClip['aspect'] {
  const width = asset.version.width_pixels
  const height = asset.version.height_pixels
  if (!width || !height) return 'unknown'
  const ratio = width / height
  if (ratio > 1.18) return 'horizontal'
  if (ratio < 0.85) return 'vertical'
  return 'square'
}

function aspectLabel(aspect: RemixClip['aspect']) {
  if (aspect === 'vertical') return '竖版'
  if (aspect === 'horizontal') return '横版'
  if (aspect === 'square') return '方版'
  return '未知'
}

function sourceLabel(source: string) {
  if (source === 'captured') return '采集素材'
  if (source === 'provider_generated') return '生成素材'
  if (source === 'upload') return '上传素材'
  return '导入素材'
}

function segmentOrder(): RemixSegment[] {
  return ['opening', 'middle', 'ending']
}

function clamp(value: number, min: number, max: number) {
  return Math.min(max, Math.max(min, value))
}

function roundSeconds(value: number) {
  return Math.round(value * 10) / 10
}

function roundScore(value: number) {
  return Math.round(value * 1000) / 1000
}

function stableHash(value: string) {
  let hash = 0
  for (let index = 0; index < value.length; index += 1) {
    hash = (hash * 31 + value.charCodeAt(index)) >>> 0
  }
  return hash.toString(36)
}
