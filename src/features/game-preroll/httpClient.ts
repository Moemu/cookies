import { uploadProjectAssetFile } from '../../data/projectAssetUpload'
import {
  createInitialGamePrerollState,
  type BriefField,
  type EvidenceMoment,
  type GamePrerollState,
  type GenerationConfig,
  type HookCandidate,
  type Provenance,
} from './model'

type AssetVersionRef = { asset_id: string; version: number }
type ProjectAssetRef = { project_id: string; asset_version: AssetVersionRef }
type ApiEvidence = {
  id: string
  kind: string
  start_milliseconds: number
  end_milliseconds: number
  description: string
  verified_copy?: string[]
}
type ApiBriefField = {
  id: string
  key: string
  label: string
  value: string
  provenance: Provenance
  evidence_refs: string[]
  required: boolean
}
type ApiCandidate = {
  id: string
  hook_mechanism: string
  execution_angle: string
  primary_test_variable: string
  variant_hypothesis: string
  score: number
  score_meaning: string
  hook_line: string
  evidence_moment_ids: string[]
  storyboard: Array<{ start_milliseconds: number; end_milliseconds: number; visual: string; copy: string; evidence_moment_id: string }>
  prompt_package?: { negative_constraints?: string[] }
}
type ApiGenerationConfig = {
  subtitle_style: string
  hook_strength: number
  pace_profile: string
  duration_seconds: number
  channel: string
  aspect_ratio: string
  resolution: string
  audio_policy: string
  call_to_action: string
}
type ApiWorkspace = {
  contract_version: string
  task_id: string
  revision: number
  stage?: string
  input_snapshot: { source_video: AssetVersionRef; evidence_moments?: ApiEvidence[] }
  source_metadata?: { DurationMS?: number; duration_ms?: number }
  analysis?: {
    status: string
    error_message?: string
    facts?: Array<{ id: string; label: string; value: string; provenance: Exclude<Provenance, 'manual'>; evidence_refs: string[] }>
    evidence?: ApiEvidence[]
    suggested_brief?: ApiBriefField[]
  }
  confirmed_brief?: { fields: ApiBriefField[] }
  evidence_assets?: {
    status: string
    frames: Array<{
      evidence_moment_id: string
      source_start_milliseconds: number
      source_end_milliseconds: number
      representative_frame_milliseconds: number
      frame_asset: ProjectAssetRef
    }>
  }
  generation_config?: ApiGenerationConfig
  candidates?: ApiCandidate[]
  selected_candidate_id?: string
  latest_video_attempt_id?: string
  video_error?: { message?: string }
  output_asset?: ProjectAssetRef
}

export type ApiGamePrerollTaskDetail = {
  task: { id: string; display_name: string; status: string }
  video_draft: { revision: number; game_preroll: ApiWorkspace }
}

type PreviewURLs = {
  sourceUrl?: string
  frameUrls?: Record<string, string>
  outputUrl?: string
}

type ProviderJob = {
  id: string
  execution_status: string
  provider_status: string
  progress: number
  error?: { message?: string }
}

const backendOrigin = (import.meta.env?.VITE_API_BASE_URL as string | undefined) ?? ''
const terminalProviderStatuses = new Set(['succeeded', 'partially_succeeded', 'failed', 'cancelled', 'expired'])
const wait = (ms: number) => new Promise<void>(resolve => globalThis.setTimeout(resolve, ms))

function idempotencyKey(kind: string) {
  return `game-preroll-v2-${kind}-${Date.now()}-${Math.random().toString(36).slice(2)}`
}

async function request<T>(path: string, method = 'GET', body?: unknown): Promise<T> {
  const response = await fetch(`${backendOrigin}${path}`, {
    method,
    credentials: 'include',
    headers: {
      ...(body === undefined ? {} : { 'Content-Type': 'application/json' }),
      ...(method === 'GET' ? {} : { 'Idempotency-Key': idempotencyKey(method.toLowerCase()) }),
    },
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  const text = await response.text()
  let payload: unknown = {}
  try { payload = text ? JSON.parse(text) : {} } catch { payload = {} }
  if (!response.ok) {
    const message = (payload as { error?: { message?: string } }).error?.message
    throw new Error(message || `请求失败（HTTP ${response.status}）`)
  }
  return payload as T
}

function evidenceKind(kind: string): EvidenceMoment['kind'] {
  if (kind === 'operation' || kind === 'skill_choice') return 'operation'
  if (kind === 'result') return 'result'
  if (kind === 'reward') return 'reward'
  if (kind === 'ui') return 'ui'
  return 'gameplay'
}

function mechanism(value: string): HookCandidate['mechanism'] {
  if (value === 'choice_challenge') return 'question'
  if (value === 'tactical_tradeoff' || value === 'failure_reversal') return 'reversal'
  return 'impact'
}

function workspaceStep(stage = ''): GamePrerollState['step'] {
  if (stage === 'video_generating' || stage === 'video_ready' || stage === 'candidate_selected') return 'generate'
  if (stage === 'candidates_ready') return 'candidates'
  if (stage === 'brief_confirmed') return 'candidates'
  if (stage === 'analysis_ready') return 'analysis'
  return stage === 'source_ready' ? 'analysis' : 'upload'
}

export function mapGamePrerollWorkspace(projectId: string, detail: ApiGamePrerollTaskDetail, urls: PreviewURLs = {}): GamePrerollState {
  const workspace = detail.video_draft.game_preroll
  const analysis = workspace.analysis
  const rawEvidence = analysis?.evidence ?? workspace.input_snapshot.evidence_moments ?? []
  const candidates = (workspace.candidates ?? []).map<HookCandidate>(candidate => ({
    id: candidate.id,
    mechanism: mechanism(candidate.hook_mechanism),
    name: candidate.execution_angle,
    hookLine: candidate.hook_line,
    audienceFit: candidate.primary_test_variable,
    recommendation: candidate.variant_hypothesis || candidate.score_meaning,
    recommended: false,
    score: candidate.score,
    evidenceRefs: candidate.evidence_moment_ids,
    risk: candidate.prompt_package?.negative_constraints?.[0] ?? '仅使用原视频中可验证的玩法与画面',
    beats: candidate.storyboard.map((beat, index) => ({
      id: `${candidate.id}-beat-${index + 1}`,
      range: `${formatSeconds(beat.start_milliseconds)}–${formatSeconds(beat.end_milliseconds)}`,
      copy: beat.copy || beat.visual,
      evidenceRef: beat.evidence_moment_id,
    })),
  }))
  const bestScore = Math.max(...candidates.map(candidate => candidate.score), -1)
  for (const candidate of candidates) candidate.recommended = candidate.score === bestScore
  const durationMS = workspace.source_metadata?.DurationMS ?? workspace.source_metadata?.duration_ms ?? 0
  const sourceRef = workspace.input_snapshot.source_video
  const config = workspace.generation_config
  const hasOutput = Boolean(workspace.output_asset && urls.outputUrl)
  const hasVideoError = Boolean(workspace.video_error)
  const initial = createInitialGamePrerollState(projectId)
  return {
    ...initial,
    taskId: detail.task.id,
    revision: detail.video_draft.revision,
    step: workspaceStep(workspace.stage),
    source: {
      id: `${sourceRef.asset_id}:${sourceRef.version}`,
      assetId: sourceRef.asset_id,
      assetVersion: sourceRef.version,
      name: detail.task.display_name || '游戏原视频',
      sizeBytes: 0,
      durationSeconds: durationMS / 1000,
      previewUrl: urls.sourceUrl,
      rightsConfirmed: true,
    },
    analysisStatus: analysis?.status === 'ready' ? 'succeeded' : analysis?.status === 'failed' ? 'failed' : analysis?.status === 'running' ? 'running' : 'idle',
    analysisFacts: (analysis?.facts ?? []).map(fact => ({
      id: fact.id,
      label: fact.label,
      value: fact.value,
      provenance: fact.provenance,
      evidenceRefs: fact.evidence_refs,
    })),
    evidence: rawEvidence.map(item => ({
      id: item.id,
      kind: evidenceKind(item.kind),
      label: item.verified_copy?.[0] || item.description.slice(0, 16),
      description: item.description,
      startMs: item.start_milliseconds,
      endMs: item.end_milliseconds,
      provenance: 'video_evidence',
      thumbnailUrl: urls.frameUrls?.[item.id],
      verifiedCopy: item.verified_copy ?? [],
    })),
    brief: (workspace.confirmed_brief?.fields ?? analysis?.suggested_brief ?? []).map(field => ({
      id: field.key || field.id,
      label: field.label,
      value: field.value,
      provenance: field.provenance,
      required: field.required,
      evidenceRefs: field.evidence_refs,
    })),
    briefConfirmed: Boolean(workspace.confirmed_brief),
    candidates,
    selectedCandidateId: workspace.selected_candidate_id,
    config: config ? {
      channel: 'douyin',
      aspectRatio: '9:16',
      durationSeconds: clampDuration(config.duration_seconds),
      subtitleStyle: config.subtitle_style === 'brand_minimal' ? 'minimal_centered' : 'high_contrast_dynamic',
      pace: config.pace_profile === 'balanced' ? 'balanced' : 'punchy',
      cta: config.call_to_action || '立即下载',
    } : initial.config,
    generation: {
      id: workspace.latest_video_attempt_id ?? '',
      status: hasOutput ? 'succeeded' : hasVideoError ? 'failed' : workspace.stage === 'video_generating' ? 'running' : 'idle',
      progress: hasOutput ? 100 : workspace.stage === 'video_generating' ? 25 : 0,
      outputUrl: urls.outputUrl,
      diagnostic: workspace.video_error?.message,
      assetId: workspace.output_asset?.asset_version.asset_id,
      assetVersion: workspace.output_asset?.asset_version.version,
    },
    notice: analysis?.error_message || (hasOutput ? '真实游戏前贴已生成并入库为 Project Asset。' : '工作区已从服务端恢复。'),
  }
}

function formatSeconds(milliseconds: number) {
  return `${(milliseconds / 1000).toFixed(milliseconds % 1000 === 0 ? 0 : 1)}秒`
}

function clampDuration(value: number): GenerationConfig['durationSeconds'] {
  return Math.min(10, Math.max(6, Math.round(value || 8))) as GenerationConfig['durationSeconds']
}

export function shouldPauseEvidenceAt(currentTimeSeconds: number, endMilliseconds?: number) {
  return endMilliseconds !== undefined && currentTimeSeconds * 1000 >= endMilliseconds
}

export class HttpGamePrerollClient {
  private detail: ApiGamePrerollTaskDetail | null = null
  private sourcePresentation: { name: string; sizeBytes: number } | null = null

  constructor(private readonly projectId: string) {}

  private workspace() {
    if (!this.detail) throw new Error('游戏前贴工作区尚未创建')
    return this.detail.video_draft.game_preroll
  }

  private path(action?: string) {
    if (!this.detail) throw new Error('游戏前贴工作区尚未创建')
    const base = `/api/creative/v1/projects/${encodeURIComponent(this.projectId)}/game-preroll-workspaces/${encodeURIComponent(this.detail.task.id)}`
    return action ? `${base}/actions/${action}` : base
  }

  private async command(action: string, body: Record<string, unknown>) {
    this.detail = await request<ApiGamePrerollTaskDetail>(this.path(action), 'POST', body)
    return this.detail
  }

  async create(file: File, durationSeconds: GenerationConfig['durationSeconds']) {
    this.sourcePresentation = { name: file.name, sizeBytes: file.size }
    const source = await uploadProjectAssetFile(backendOrigin, this.projectId, file, 'game-preroll-source')
    this.detail = await request<ApiGamePrerollTaskDetail>(
      `/api/creative/v1/projects/${encodeURIComponent(this.projectId)}/game-preroll-workspaces`,
      'POST',
      { source_video: source, rights_confirmed: true, duration_seconds: durationSeconds },
    )
    return this.hydrate()
  }

  async restore(taskId: string, sourcePresentation?: { name: string; sizeBytes: number }) {
    if (sourcePresentation) this.sourcePresentation = sourcePresentation
    this.detail = await request<ApiGamePrerollTaskDetail>(`/api/creative/v1/projects/${encodeURIComponent(this.projectId)}/game-preroll-workspaces/${encodeURIComponent(taskId)}`)
    if (this.workspace().analysis?.status === 'running') await this.pollAnalysis()
    if (this.workspace().stage === 'video_generating') await this.pollGeneration()
    return this.hydrate()
  }

  async analyze() {
    await this.command('analyze-source', { expected_revision: this.detail!.video_draft.revision })
    await this.pollAnalysis()
    if (this.workspace().analysis?.status === 'ready' && this.workspace().evidence_assets?.status !== 'ready') {
      await this.command('prepare-evidence', { expected_revision: this.detail!.video_draft.revision })
    }
    return this.hydrate()
  }

  async confirmBriefAndPlan(fields: BriefField[]) {
    const payload = fields.map(field => ({
      id: field.id,
      key: field.id,
      label: field.label,
      value: field.value,
      provenance: field.provenance,
      evidence_refs: field.evidenceRefs,
      required: field.required,
    }))
    await this.command('confirm-brief', { expected_revision: this.detail!.video_draft.revision, fields: payload })
    await this.command('plan-candidates', { expected_revision: this.detail!.video_draft.revision })
    return this.hydrate()
  }

  async replan() {
    await this.command('plan-candidates', { expected_revision: this.detail!.video_draft.revision })
    return this.hydrate()
  }

  async select(candidateId: string) {
    await this.command('select-candidate', { expected_revision: this.detail!.video_draft.revision, candidate_id: candidateId })
    return this.hydrate()
  }

  async generate(candidate: HookCandidate, config: GenerationConfig) {
    const candidateId = await this.syncConfig(candidate, config)
    if (this.workspace().selected_candidate_id !== candidateId) {
      await this.command('select-candidate', { expected_revision: this.detail!.video_draft.revision, candidate_id: candidateId })
    }
    if (this.workspace().evidence_assets?.status !== 'ready') {
      await this.command('prepare-evidence', { expected_revision: this.detail!.video_draft.revision })
    }
    await this.command('generate-video', { expected_revision: this.detail!.video_draft.revision })
    await this.pollGeneration()
    return this.hydrate()
  }

  private async syncConfig(candidate: HookCandidate, config: GenerationConfig) {
    const current = this.workspace().generation_config
    const subtitle = config.subtitleStyle === 'minimal_centered' ? 'brand_minimal' : 'high_contrast_dynamic'
    const pace = config.pace === 'balanced' ? 'balanced' : 'punchy'
    if (current && current.duration_seconds === config.durationSeconds && current.subtitle_style === subtitle && current.pace_profile === pace && current.call_to_action === config.cta) return candidate.id
    const oldMechanism = candidate.mechanism
    await this.command('update-generation-config', {
      expected_revision: this.detail!.video_draft.revision,
      config: { subtitle_style: subtitle, hook_strength: 4, pace_profile: pace, duration_seconds: config.durationSeconds, channel: 'douyin', aspect_ratio: '9:16', resolution: '720p', audio_policy: 'source_audio', call_to_action: config.cta },
    })
    await this.command('plan-candidates', { expected_revision: this.detail!.video_draft.revision })
    const replacement = (this.workspace().candidates ?? []).find(item => mechanism(item.hook_mechanism) === oldMechanism) ?? this.workspace().candidates?.[0]
    if (!replacement) throw new Error('更新配置后没有生成可用候选')
    return replacement.id
  }

  private async pollAnalysis() {
    for (let attempt = 0; attempt < 180; attempt++) {
      this.detail = await request<ApiGamePrerollTaskDetail>(this.path())
      const status = this.workspace().analysis?.status
      if (status === 'ready') return
      if (status === 'failed') throw new Error(this.workspace().analysis?.error_message || '素材拆解失败')
      await wait(1000)
    }
    throw new Error('素材拆解超时，请稍后重试')
  }

  private async pollGeneration() {
    const jobId = this.workspace().latest_video_attempt_id
    if (!jobId) throw new Error('服务端没有返回视频生成任务 ID')
    for (let attempt = 0; attempt < 600; attempt++) {
      const job = await request<ProviderJob>(`/platform/v1/projects/${encodeURIComponent(this.projectId)}/model/jobs/${encodeURIComponent(jobId)}`)
      if (terminalProviderStatuses.has(job.provider_status) || ['succeeded', 'failed', 'cancelled'].includes(job.execution_status)) {
        this.detail = await request<ApiGamePrerollTaskDetail>(this.path('reconcile-video'), 'POST', { expected_revision: this.detail!.video_draft.revision, provider_job_id: jobId })
        if (this.workspace().output_asset) return
        throw new Error(this.workspace().video_error?.message || job.error?.message || '视频生成失败')
      }
      await wait(1500)
    }
    throw new Error('视频生成仍在进行，可刷新页面继续查看')
  }

  private async hydrate() {
    const workspace = this.workspace()
    const sourceUrl = await previewUrl(this.projectId, { project_id: this.projectId, asset_version: workspace.input_snapshot.source_video })
    const frameEntries = await Promise.all((workspace.evidence_assets?.frames ?? []).map(async frame => [frame.evidence_moment_id, await previewUrl(this.projectId, frame.frame_asset)] as const))
    const outputUrl = workspace.output_asset ? await previewUrl(this.projectId, workspace.output_asset) : undefined
    const state = mapGamePrerollWorkspace(this.projectId, this.detail!, { sourceUrl, frameUrls: Object.fromEntries(frameEntries), outputUrl })
    return this.sourcePresentation && state.source
      ? { ...state, source: { ...state.source, ...this.sourcePresentation } }
      : state
  }
}

async function previewUrl(projectId: string, ref: ProjectAssetRef) {
  const version = ref.asset_version
  const payload = await request<{ url?: string }>(`/platform/v1/projects/${encodeURIComponent(projectId)}/assets/${encodeURIComponent(version.asset_id)}/versions/${version.version}/preview`)
  if (!payload.url) throw new Error('素材预览地址不可用')
  return payload.url.startsWith('/') ? `${backendOrigin}${payload.url}` : payload.url
}
