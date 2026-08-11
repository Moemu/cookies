import type {
  ApiGameEvidenceMoment,
  ApiGamePrerollCandidate,
} from '../../data/api'

export type GameShotView = {
  id: string
  outputRangeLabel: string
  sourceRangeLabel: string
  seekSeconds: number
  thumbnailTimestampSeconds: number
  evidenceDescription: string
  verifiedCopy: string[]
  beat: ApiGamePrerollCandidate['storyboard'][number]
}

export type ComparedGamePrerollCandidate = {
  candidate: ApiGamePrerollCandidate
  recommended: boolean
  recommendationLabel: string
}

export function buildGameShotViews(
  candidate: ApiGamePrerollCandidate | undefined,
  evidenceMoments: ApiGameEvidenceMoment[],
): GameShotView[] {
  if (!candidate) return []

  const evidenceById = new Map(evidenceMoments.map(moment => [moment.id, moment]))
  return candidate.storyboard.map(beat => {
    const evidence = evidenceById.get(beat.evidence_moment_id)
    const sourceStart = evidence?.start_milliseconds ?? 0
    const sourceEnd = evidence?.end_milliseconds ?? sourceStart
    return {
      id: `${candidate.id}:${beat.start_milliseconds}`,
      outputRangeLabel: `${formatOutputSeconds(beat.start_milliseconds)}–${formatOutputSeconds(beat.end_milliseconds)} 秒`,
      sourceRangeLabel: `${formatSourceSeconds(sourceStart)}–${formatSourceSeconds(sourceEnd)}s`,
      seekSeconds: beat.start_milliseconds / 1000,
      thumbnailTimestampSeconds: roundMilliseconds((sourceStart + sourceEnd) / 2),
      evidenceDescription: evidence?.description ?? '未找到对应的源实录证据',
      verifiedCopy: evidence?.verified_copy ?? [],
      beat,
    }
  })
}

export function compareGamePrerollCandidates(
  candidates: ApiGamePrerollCandidate[],
): ComparedGamePrerollCandidate[] {
  return candidates
    .map((candidate, originalIndex) => ({ candidate, originalIndex }))
    .sort((left, right) => right.candidate.score - left.candidate.score || left.originalIndex - right.originalIndex)
    .map(({ candidate }, index) => ({
      candidate,
      recommended: index === 0,
      recommendationLabel: index === 0 ? '证据匹配度最高' : '备选创意方向',
    }))
}

function formatOutputSeconds(milliseconds: number) {
  return String(milliseconds / 1000).replace(/\.0+$/, '')
}

function formatSourceSeconds(milliseconds: number) {
  return (milliseconds / 1000).toFixed(3)
}

function roundMilliseconds(milliseconds: number) {
  return Math.round(milliseconds) / 1000
}
