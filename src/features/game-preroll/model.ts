export type GamePrerollStep = 'upload' | 'analysis' | 'brief' | 'candidates' | 'generate'
export type Provenance = 'video_evidence' | 'ai_inference' | 'manual'
export type RunStatus = 'idle' | 'running' | 'succeeded' | 'failed'

export type SourceVideo = {
  id: string
  assetId?: string
  assetVersion?: number
  name: string
  sizeBytes: number
  durationSeconds: number
  previewUrl?: string
  rightsConfirmed: boolean
}

export type EvidenceMoment = {
  id: string
  kind: 'gameplay' | 'operation' | 'result' | 'reward' | 'ui'
  label: string
  description: string
  startMs: number
  endMs: number
  provenance: Exclude<Provenance, 'manual'>
  thumbnailUrl?: string
  verifiedCopy?: string[]
}

export type AnalysisFact = {
  id: string
  label: string
  value: string
  provenance: Exclude<Provenance, 'manual'>
  evidenceRefs: string[]
}

export type BriefField = {
  id: string
  label: string
  value: string
  provenance: Provenance
  required: boolean
  evidenceRefs: string[]
}

export type HookCandidate = {
  id: string
  mechanism: 'question' | 'reversal' | 'impact'
  name: string
  hookLine: string
  audienceFit: string
  recommendation: string
  recommended: boolean
  score: number
  evidenceRefs: string[]
  risk: string
  beats: Array<{ id: string; range: string; copy: string; evidenceRef: string }>
}

export type GenerationConfig = {
  channel: 'douyin'
  aspectRatio: '9:16'
  durationSeconds: 6 | 7 | 8 | 9 | 10
  subtitleStyle: 'high_contrast_dynamic' | 'minimal_centered'
  pace: 'balanced' | 'punchy' | 'intense'
  cta: string
}

export type GenerationJob = {
  id: string
  status: RunStatus
  progress: number
  outputUrl?: string
  diagnostic?: string
  assetId?: string
  assetVersion?: number
}

export type GamePrerollState = {
  projectId: string
  taskId: string
  revision: number
  step: GamePrerollStep
  source?: SourceVideo
  analysisStatus: RunStatus
  analysisFacts: AnalysisFact[]
  evidence: EvidenceMoment[]
  brief: BriefField[]
  briefConfirmed: boolean
  candidates: HookCandidate[]
  selectedCandidateId?: string
  config: GenerationConfig
  generation: GenerationJob
  notice: string
}

export const stepOrder: GamePrerollStep[] = ['upload', 'analysis', 'brief', 'candidates', 'generate']

export function createInitialGamePrerollState(projectId: string): GamePrerollState {
  return {
    projectId,
    taskId: '',
    revision: 0,
    step: 'upload',
    analysisStatus: 'idle',
    analysisFacts: [],
    evidence: [],
    brief: [],
    briefConfirmed: false,
    candidates: [],
    config: {
      channel: 'douyin', aspectRatio: '9:16', durationSeconds: 8,
      subtitleStyle: 'high_contrast_dynamic', pace: 'punchy', cta: '立即下载',
    },
    generation: { id: '', status: 'idle', progress: 0 },
    notice: '上传游戏原视频后，AI 会先理解素材，不会立即生成视频。',
  }
}

export function stepAccessible(state: GamePrerollState, step: GamePrerollStep) {
  const target = stepOrder.indexOf(step)
  if (target === 0) return true
  if (target === 1) return Boolean(state.source)
  if (target === 2) return state.analysisStatus === 'succeeded'
  if (target === 3) return state.briefConfirmed
  return Boolean(state.selectedCandidateId)
}

export function generationBlockers(state: GamePrerollState) {
  const blockers: string[] = []
  if (!state.source) blockers.push('未上传原视频')
  if (state.analysisStatus !== 'succeeded') blockers.push('素材尚未完成拆解')
  if (!state.briefConfirmed) blockers.push('广告简报尚未确认')
  if (!state.selectedCandidateId) blockers.push('尚未人工选择钩子方案')
  if (state.evidence.length < 3) blockers.push('证据帧不足 3 张')
  return blockers
}

export function briefCompleteness(fields: BriefField[]) {
  const required = fields.filter(field => field.required)
  if (!required.length) return 0
  return Math.round(required.filter(field => field.value.trim()).length / required.length * 100)
}
