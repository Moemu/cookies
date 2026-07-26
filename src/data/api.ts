export type ApiProject = {
  id: string
  name: string
  brand: string
  objective: string
  runtime: {
    code: string
    product: string
    stage: string
    progress: number
    status: 'active' | 'completed'
    owner: string
    budget: number
    currency: 'CNY'
    timezone: 'Asia/Shanghai'
  }
  version: number
  createdAt: string
  updatedAt: string
}

export type ApiArtifact = {
  id: string
  projectId: string
  kind: 'brief' | 'image' | 'video' | 'document'
  purpose?: ApiVideoPurpose
  prerollType?: ApiPrerollType
  shortDramaPreroll?: ApiShortDramaPrerollSnapshot
  status: 'draft' | 'ready' | 'archived'
  content: string
  sourceJobId?: string
  version: number
  createdAt: string
  updatedAt: string
}

export type ApiAssetFeature = {
  id: string
  organizationId: string
  projectId: string
  assetId: string
  assetVersion: number
  schemaVersion: 'asset_feature_v1'
  featureVersion: string
  hookStrength: number
  productVisibility: number
  sceneTags: string[]
  productTags: string[]
  personTags: string[]
  actionTags: string[]
  emotionTags: string[]
  sellingPoints: string[]
  ctaPresence: boolean
  similarityGroup?: string
  similarityRisk: 'low' | 'medium' | 'high'
  evidence: string[]
  version: number
  createdAt: string
  updatedAt: string
}

export type ApiGenerationJob = {
  id: string
  projectId: string
  artifactKind: ApiArtifact['kind']
  purpose?: ApiVideoPurpose
  prerollType?: ApiPrerollType
  briefArtifactId?: string
  status: 'queued' | 'running' | 'succeeded' | 'failed' | 'cancelled'
  model?: string
  diagnostic?: string
  artifactId?: string
  version: number
  createdAt: string
  updatedAt: string
}

export type ApiVideoPurpose = 'preroll'
export type ApiPrerollType = 'short_drama' | 'game' | 'commerce'

export type ApiShortDramaStoryContext = {
  title: string
  synopsis: string
  reviewedSellingPoints: string[]
  openingLine?: string
}

export type ApiShortDramaPrerollCandidate = {
  id: string
  hookType: 'conflict' | 'reversal' | 'suspense' | 'selling_point_bridge'
  score: number
  scoreMeaning: 'hook_relevance'
  evidence: string[]
  voiceover: string
  visualIntent: string
  transitionLine: string
}

export type ApiShortDramaPrerollPlan = {
  version: 'short_drama_preroll_v1'
  candidates: ApiShortDramaPrerollCandidate[]
}

export type ApiShortDramaPrerollSnapshot = {
  planVersion: ApiShortDramaPrerollPlan['version']
  storyContext: Omit<ApiShortDramaStoryContext, 'openingLine'>
  selectedCandidate: ApiShortDramaPrerollCandidate
  prompt: string
}

export type ApiPrerollScope = {
  projectId: string
  purpose: 'preroll'
  prerollType: ApiPrerollType
}

export type ApiBusinessTaskType =
  | 'strategy'
  | 'creative'
  | 'video'
  | 'brand_video'
  | 'short_drama_preroll'
  | 'game_preroll'
  | 'commerce_preroll'
  | 'viral_remake'
  | 'video_edit'

export type ApiBusinessTask = {
  id: string
  projectId: string
  type: ApiBusinessTaskType
  name: string
  objective: string
  status: 'draft' | 'in_progress' | 'ready' | 'completed' | 'failed'
  sourceTaskIds: string[]
  sourceArtifactIds: string[]
  outputArtifactIds: string[]
  version: number
  createdAt: string
  updatedAt: string
}

export type ApiAuditEvent = {
  id: string
  projectId: string
  actor: string
  action: string
  entityType: 'project' | 'business_task' | 'artifact' | 'generation_job' | 'change_set'
  entityId: string
  metadata: Record<string, unknown>
  createdAt: string
}

export type ApiOperationalRecordKind =
  | 'work_item'
  | 'evidence'
  | 'activity'
  | 'metric'
  | 'performance_ad'
  | 'audience_mix'
  | 'method'
  | 'delivery_diagnostic'
  | 'delivery_action'
  | 'unified_record'

export type ApiOperationalRecord = {
  id: string
  projectId: string
  kind: ApiOperationalRecordKind
  title: string
  status: string
  occurredAt: string
  fields: Record<string, string | number>
  createdAt: string
  updatedAt: string
}

export type ApiRemixEvalCase = {
  id: string
  type: 'mcq' | 'rubric'
  title: string
  prompt: string
  planner_version: string
  prompt_version: string
  choices?: Array<{ id: string; label: string }>
  expected_choice?: string
  rubric?: Array<{ id: string; label: string; signal: string; weight: number; required: boolean }>
  passing_score: number
  created_at: string
}

export type ApiRemixEvalSubmission = {
  case_id: string
  choice_id?: string
  answer_text?: string
  rubric_evidence?: string[]
}

export type ApiRemixEvalRun = {
  id: string
  status: 'succeeded'
  planner_version: string
  prompt_version: string
  score: number
  total_cases: number
  passed_cases: number
  failed_cases: string[]
  results: Array<{
    id: string
    case_id: string
    case_type: string
    score: number
    passed: boolean
    expected: string
    actual: string
    reason: string
  }>
  created_at: string
}

export type ApiKnowledgeDocument = {
  id: string
  organization_id: string
  project_id: string
  title: string
  source_uri: string
  source_type: string
  chunk_count: number
  created_at: string
  updated_at: string
}

export type ApiKnowledgeCitation = {
  document_id: string
  chunk_id: string
  title: string
  source_uri: string
  section: string
  start_line: number
  end_line: number
  snippet: string
}

export type ApiKnowledgeSearchResult = {
  chunk: {
    id: string
    document_id: string
    project_id: string
    index: number
    text: string
    source_uri: string
    section: string
    start_line: number
    end_line: number
  }
  score: number
  citations: ApiKnowledgeCitation[]
}

export type ApiRemixRenderJob = {
  id: string
  organization_id: string
  project_id: string
  plan_id: string
  status: 'queued' | 'running' | 'requires_review' | 'succeeded' | 'failed'
  progress: number
  target_format: 'mp4'
  target_quality: 'draft' | 'standard' | 'high'
  requires_review: boolean
  quality_report_id?: string
  output_asset?: {
    project_id: string
    asset_version: {
      asset_id: string
      version: number
    }
  }
  output_preview?: {
    url: string
  }
  provenance?: {
    plan_id: string
    render_job_id: string
    input_assets: ApiAssetVersionRef[]
  }
  error_code?: string
  error_message?: string
  created_at: string
  updated_at: string
}

export type ApiFeedbackEventType = 'rating' | 'comment' | 'asset_selected' | 'render_succeeded'
export type ApiFeedbackTargetType = 'remix_plan' | 'render_job' | 'asset'

export type ApiCreateFeedbackEventInput = {
  event_type: ApiFeedbackEventType
  target_type: ApiFeedbackTargetType
  target_id: string
  asset_version?: ApiAssetVersionRef
  rating?: number
  comment?: string
}

export type ApiFeedbackEvent = ApiCreateFeedbackEventInput & {
  id: string
  organization_id: string
  project_id: string
  created_at: string
}

export type ApiAssetPerformance = {
  asset_version: ApiAssetVersionRef
  selected_count: number
  render_succeeded_count: number
  feedback_count: number
  average_rating: number
  updated_at: string
}

export type ApiPlannerWeightSnapshot = {
  id: string
  organization_id: string
  project_id: string
  asset_weights: Array<{
    asset_version: ApiAssetVersionRef
    weight: number
    reasons: string[]
  }>
  created_at: string
}

export type ApiQualityVerdict = 'pass' | 'major' | 'critical'
export type ApiRemixHookType = 'conflict' | 'reversal' | 'suspense' | 'selling_point_bridge' | 'product_demo' | 'offer'
export type ApiPrerollMode = 'prompt_only' | 'generate_video'
export type ApiPrerollStatus = 'draft' | 'ready' | 'failed' | 'applied'

export type ApiQualityReport = {
  id: string
  organization_id: string
  project_id: string
  render_job_id: string
  output_asset?: {
    project_id: string
    asset_version: ApiAssetVersionRef
  }
  verdict: ApiQualityVerdict
  score: number
  dimensions: Array<{
    name: string
    score: number
    verdict: ApiQualityVerdict
    summary: string
  }>
  issues: Array<{
    code: string
    severity: 'major' | 'critical'
    dimension: string
    start_seconds: number
    end_seconds: number
    description: string
    repair_suggestion: string
  }>
  evidence: Array<{
    kind: string
    timestamp_sec: number
    summary: string
  }>
  repair_suggestions: string[]
  created_at: string
  updated_at: string
}

export type ApiAssetVersionRef = {
  asset_id: string
  version: number
}

export type ApiHitSegmentRole = 'hook' | 'problem' | 'proof' | 'offer' | 'cta'

export type ApiHitAnalysis = {
  id: string
  organization_id: string
  project_id: string
  source_asset: ApiAssetVersionRef
  title: string
  video_meta: {
    duration_seconds: number
    language: string
  }
  segments: Array<{
    id: string
    start_seconds: number
    end_seconds: number
    role: ApiHitSegmentRole
    summary: string
    script: string
    visual_element: string
    conversion_cue: string
    replication_hint: string
  }>
  scripts: Array<{ segment_id: string; text: string }>
  visual_elements: string[]
  conversion_nodes: Array<{ segment_id: string; cue: string }>
  replication_insights: string[]
  created_at: string
  updated_at: string
}

export type ApiCreateHitAnalysisInput = {
  source_asset: ApiAssetVersionRef
  title: string
  duration_seconds: number
  language?: string
  notes?: string
}

export type ApiProductProfile = {
  name: string
  selling_points: string[]
  cta: string
}

export type ApiReplacementRule = {
  role: ApiHitSegmentRole
  target_asset: ApiAssetVersionRef
  message: string
}

export type ApiCreateProductMappingInput = {
  hit_analysis_id: string
  target_product: ApiProductProfile
  required_assets: ApiAssetVersionRef[]
  replacement_rules: ApiReplacementRule[]
  constraints?: string[]
  target_seconds?: number
  pace?: 'fast' | 'balanced' | 'story'
}

export type ApiProductMapping = ApiCreateProductMappingInput & {
  id: string
  organization_id: string
  project_id: string
  created_at: string
  updated_at: string
}

export type ApiCreateRemixPrerollInput = {
  plan_id: string
  hook_type: ApiRemixHookType
  reference_asset: ApiAssetVersionRef
  style_constraints: string[]
  duration_seconds: number
  mode: ApiPrerollMode
}

export type ApiRemixPreroll = ApiCreateRemixPrerollInput & {
  id: string
  organization_id: string
  project_id: string
  prompt_draft: string
  output_asset?: {
    project_id: string
    asset_version: ApiAssetVersionRef
  }
  quality_verdict: ApiQualityVerdict
  status: ApiPrerollStatus
  error_code?: string
  error_message?: string
  applied_plan_id?: string
  created_at: string
  updated_at: string
}

export type ApiRemixPlan = {
  id: string
  organization_id: string
  project_id: string
  schema_version: 'remix_plan_v1' | 'remix_plan_v2'
  client_plan_id: string
  target_seconds: number
  actual_seconds: number
  pace: 'fast' | 'balanced' | 'story'
  segments: Array<{
    segment: 'opening' | 'middle' | 'ending'
    label: string
    target_seconds: number
    actual_seconds: number
    shots: Array<{
      id: string
      source: 'existing_asset'
      asset_version: ApiAssetVersionRef
      timeline?: {
        start_seconds: number
        duration_seconds: number
        in_point_seconds: number
        out_point_seconds: number
      }
      creative: {
        scene: string
        shot_type: string
        dialogue_or_narration: string
        subtitle: string
        cta_element: string
      }
    }>
  }>
  warnings: string[]
  summary: {
    selected_assets: number
    used_assets: number
    coverage_percent: number
    strategy: string
  }
}

export type ApiAgentRun = {
  id: string
  organization_id: string
  project_id: string
  workflow: 'render_diagnosis'
  status: 'queued' | 'running' | 'succeeded' | 'failed' | 'cancelled'
  target: {
    render_job_id: string
  }
  steps: Array<{
    id: string
    label: string
    status: ApiAgentRun['status']
    summary: string
    started_at: string
    ended_at?: string
  }>
  tool_calls: Array<{
    id: string
    name: string
    status: 'pending' | 'running' | 'succeeded' | 'failed'
    input: Record<string, unknown>
    output?: Record<string, unknown>
    error_message?: string
    references?: Array<{ type: string; id: string; version?: number }>
    started_at: string
    ended_at?: string
  }>
  trace_spans: Array<{
    id: string
    parent_id?: string
    name: string
    kind: 'agent' | 'tool' | 'model'
    status: 'running' | 'succeeded' | 'failed' | 'cancelled'
    model?: string
    input_tokens?: number
    output_tokens?: number
    error_message?: string
    started_at: string
    ended_at?: string
  }>
  output?: Record<string, unknown>
  error_message?: string
  created_at: string
  updated_at: string
  completed_at?: string
}

export type ApiProviderCapabilities = {
  provider: string
  status: 'configured' | 'not_configured'
  capabilities: Array<{
    capability: string
    model: string
    available: boolean
  }>
  checkedAt: string
}

const viteEnv = (import.meta as unknown as { env?: { VITE_API_BASE_URL?: string } }).env
const apiBase = `${viteEnv?.VITE_API_BASE_URL ?? 'http://127.0.0.1:8787'}/api`
const platformBase = `${viteEnv?.VITE_API_BASE_URL ?? 'http://127.0.0.1:8787'}/platform/v1`

async function request<T>(path: string, method = 'GET', body?: unknown): Promise<T> {
  const response = await fetch(`${apiBase}${path}`, {
    method,
    headers: body === undefined ? undefined : { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  const payload = await response.json() as T | { error?: { message?: string } }
  if (!response.ok) {
    const error = payload as { error?: { message?: string } }
    throw new Error(error.error?.message ?? 'API 请求失败')
  }
  return payload as T
}

async function platformRequest<T>(path: string, method = 'GET', body?: unknown, headers?: Record<string, string>): Promise<T> {
  const response = await fetch(`${platformBase}${path}`, {
    method,
    headers: {
      ...(body === undefined ? {} : { 'Content-Type': 'application/json' }),
      ...(headers ?? {}),
    },
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  const payload = await response.json() as T | { error?: { message?: string } }
  if (!response.ok) {
    const error = payload as { error?: { message?: string } }
    throw new Error(error.error?.message ?? '平台 API 请求失败')
  }
  return payload as T
}

function projectQuery(projectId?: string): string {
  return projectId ? `?projectId=${encodeURIComponent(projectId)}` : ''
}

function prerollQuery(scope: ApiPrerollScope): string {
  const search = new URLSearchParams({
    projectId: scope.projectId,
    purpose: scope.purpose,
    prerollType: scope.prerollType,
  })
  return `?${search.toString()}`
}

function assetFeatureQuery(projectId: string, organizationId: string): string {
  const search = new URLSearchParams({ organizationId, projectId })
  return `?${search.toString()}`
}

export function buildHitAnalysisInput(sourceAsset: ApiAssetVersionRef, title: string, durationSeconds: number): ApiCreateHitAnalysisInput {
  return {
    source_asset: sourceAsset,
    title,
    duration_seconds: durationSeconds,
    language: 'zh-CN',
    notes: '由爆款复刻流程提交，输出结构用于产品映射和 Shot-based RemixPlan。',
  }
}

export function buildProductMappingInput(
  analysis: Pick<ApiHitAnalysis, 'id'>,
  targetProduct: ApiProductProfile,
  targetAssets: {
    hook: ApiAssetVersionRef
    proof: ApiAssetVersionRef
    cta: ApiAssetVersionRef
  },
): ApiCreateProductMappingInput {
  return {
    hit_analysis_id: analysis.id,
    target_product: targetProduct,
    required_assets: [targetAssets.hook, targetAssets.proof, targetAssets.cta],
    replacement_rules: [
      { role: 'hook', target_asset: targetAssets.hook, message: `${targetProduct.name}：先用最高差异卖点制造停留。` },
      { role: 'proof', target_asset: targetAssets.proof, message: `${targetProduct.selling_points[0] ?? targetProduct.name}：用授权素材重建证明段。` },
      { role: 'cta', target_asset: targetAssets.cta, message: targetProduct.cta },
    ],
    constraints: ['不得默认复用原视频二进制', '仅引用当前 Project 授权素材'],
    target_seconds: 30,
    pace: 'balanced',
  }
}

export function buildRemixPrerollInput(
	planId: string,
	hookType: ApiRemixHookType,
	referenceAsset: ApiAssetVersionRef,
	mode: ApiPrerollMode,
	durationSeconds = 6,
	styleConstraints: string[] = ['9:16 竖版', '静音可理解', '保留 opening 拼接点'],
): ApiCreateRemixPrerollInput {
	return {
		plan_id: planId,
		hook_type: hookType,
		reference_asset: referenceAsset,
		style_constraints: styleConstraints,
		duration_seconds: durationSeconds,
		mode,
	}
}

export const api = {
  getCapabilities: () => request<ApiProviderCapabilities>('/provider/capabilities'),
  listProjects: () => request<ApiProject[]>('/projects'),
  createProject: (input: Pick<ApiProject, 'name' | 'brand' | 'objective'>) =>
    request<ApiProject>('/projects', 'POST', input),
  updateProject: (id: string, input: Partial<Pick<ApiProject, 'name' | 'brand' | 'objective'>>) =>
    request<ApiProject>(`/projects/${encodeURIComponent(id)}`, 'PATCH', input),
  listArtifacts: (projectId?: string) =>
    request<ApiArtifact[]>(`/artifacts${projectQuery(projectId)}`),
  listPrerollArtifacts: (scope: ApiPrerollScope) =>
    request<ApiArtifact[]>(`/artifacts${prerollQuery(scope)}`),
  listAssetFeatures: (projectId: string, organizationId = 'demo-org') =>
    request<{ items: ApiAssetFeature[] }>(`/asset-features${assetFeatureQuery(projectId, organizationId)}`),
  listTasks: (projectId?: string) =>
    request<ApiBusinessTask[]>(`/tasks${projectQuery(projectId)}`),
  getTask: (id: string) =>
    request<ApiBusinessTask>(`/tasks/${encodeURIComponent(id)}`),
  createTask: (input: {
    projectId: string
    type: ApiBusinessTaskType
    name: string
    objective: string
    sourceTaskIds?: string[]
    sourceArtifactIds?: string[]
  }) => request<ApiBusinessTask>('/tasks', 'POST', input),
  updateTask: (
    id: string,
    input: Partial<Pick<ApiBusinessTask, 'name' | 'objective' | 'status' | 'sourceTaskIds' | 'sourceArtifactIds' | 'outputArtifactIds'>>,
  ) => request<ApiBusinessTask>(`/tasks/${encodeURIComponent(id)}`, 'PATCH', input),
  createArtifact: (input: {
    projectId: string
    kind: ApiArtifact['kind']
    content: string
    status?: ApiArtifact['status']
    sourceJobId?: string
  }) => request<ApiArtifact>('/artifacts', 'POST', input),
  updateArtifact: (
    id: string,
    input: Partial<Pick<ApiArtifact, 'content' | 'status' | 'sourceJobId'>>,
  ) => request<ApiArtifact>(`/artifacts/${encodeURIComponent(id)}`, 'PATCH', input),
  listJobs: (projectId?: string) =>
    request<ApiGenerationJob[]>(`/generation-jobs${projectQuery(projectId)}`),
  listPrerollJobs: (scope: ApiPrerollScope) =>
    request<ApiGenerationJob[]>(`/generation-jobs${prerollQuery(scope)}`),
  getJob: (id: string) =>
    request<ApiGenerationJob>(`/generation-jobs/${encodeURIComponent(id)}`),
  getPrerollJob: (id: string, scope: ApiPrerollScope) =>
    request<ApiGenerationJob>(`/generation-jobs/${encodeURIComponent(id)}${prerollQuery(scope)}`),
  cancelJob: (id: string, scope?: ApiPrerollScope) =>
    request<ApiGenerationJob>(
      `/generation-jobs/${encodeURIComponent(id)}/cancel${scope ? prerollQuery(scope) : ''}`,
      'POST',
    ),
  generateBrief: (projectId: string, prompt: string) =>
    request<{ job: ApiGenerationJob; artifact: ApiArtifact }>('/generation/text', 'POST', {
      projectId,
      prompt,
    }),
  createMedia: (
    projectId: string,
    kind: 'image' | 'video',
    prompt: string,
    briefId: string,
  ) => request<ApiGenerationJob>('/generation/media', 'POST', {
    projectId,
    kind,
    prompt,
    briefId,
  }),
  createPrerollVideo: (
    scope: ApiPrerollScope,
    prompt: string,
    briefId: string,
  ) => request<ApiGenerationJob>('/generation/media', 'POST', {
    projectId: scope.projectId,
    kind: 'video',
    purpose: scope.purpose,
    prerollType: scope.prerollType,
    prompt,
    briefId,
  }),
  planShortDramaPreroll: (
    projectId: string,
    briefId: string,
    storyContext: ApiShortDramaStoryContext,
  ) => request<ApiShortDramaPrerollPlan>('/short-drama-preroll-plans', 'POST', {
    projectId,
    briefId,
    storyContext,
  }),
  createShortDramaPrerollVideo: (
    scope: ApiPrerollScope & { prerollType: 'short_drama' },
    briefId: string,
    planVersion: ApiShortDramaPrerollPlan['version'],
    candidateId: string,
    storyContext: ApiShortDramaStoryContext,
  ) => request<ApiGenerationJob>('/generation/media', 'POST', {
    projectId: scope.projectId,
    kind: 'video',
    purpose: scope.purpose,
    prerollType: scope.prerollType,
    briefId,
    shortDramaPlanVersion: planVersion,
    shortDramaCandidateId: candidateId,
    storyContext,
  }),
  listAuditEvents: (projectId?: string) =>
    request<ApiAuditEvent[]>(`/audit-events${projectQuery(projectId)}`),
  listOperations: (projectId: string) =>
    request<ApiOperationalRecord[]>(`/projects/${encodeURIComponent(projectId)}/operations`),
  listRemixEvalCases: (projectId: string) =>
    platformRequest<{ items: ApiRemixEvalCase[] }>(`/projects/${encodeURIComponent(projectId)}/remix-eval-cases`),
  createRemixEvalRun: (projectId: string, input: {
    planner_version: string
    prompt_version: string
    submissions: ApiRemixEvalSubmission[]
  }) => platformRequest<ApiRemixEvalRun>(`/projects/${encodeURIComponent(projectId)}/remix-eval-runs`, 'POST', input),
  getRemixEvalRun: (projectId: string, runId: string) =>
    platformRequest<ApiRemixEvalRun>(`/projects/${encodeURIComponent(projectId)}/remix-eval-runs/${encodeURIComponent(runId)}`),
  createRemixRenderJob: (projectId: string, input: {
    plan_id: string
    target_format?: 'mp4'
    target_quality?: 'draft' | 'standard' | 'high'
  }, idempotencyKey: string) =>
    platformRequest<ApiRemixRenderJob>(
      `/projects/${encodeURIComponent(projectId)}/remix-render-jobs`,
      'POST',
      input,
      { 'Idempotency-Key': idempotencyKey },
    ),
  getRemixRenderJob: (projectId: string, jobId: string) =>
    platformRequest<ApiRemixRenderJob>(`/projects/${encodeURIComponent(projectId)}/remix-render-jobs/${encodeURIComponent(jobId)}`),
  createRemixQualityReport: (projectId: string, input: {
    render_job_id: string
    output_asset?: ApiQualityReport['output_asset']
    policy?: 'fail_critical' | 'review_all_issues'
  }) => platformRequest<ApiQualityReport>(`/projects/${encodeURIComponent(projectId)}/remix-quality-reports`, 'POST', input),
  getRemixRenderJobQualityReport: (projectId: string, jobId: string) =>
    platformRequest<{ quality_report: ApiQualityReport | null }>(`/projects/${encodeURIComponent(projectId)}/remix-render-jobs/${encodeURIComponent(jobId)}/quality-report`),
  createHitAnalysis: (projectId: string, input: ApiCreateHitAnalysisInput) =>
    platformRequest<ApiHitAnalysis>(`/projects/${encodeURIComponent(projectId)}/remix-hit-analyses`, 'POST', input),
  getHitAnalysis: (projectId: string, analysisId: string) =>
    platformRequest<ApiHitAnalysis>(`/projects/${encodeURIComponent(projectId)}/remix-hit-analyses/${encodeURIComponent(analysisId)}`),
  createProductMapping: (projectId: string, input: ApiCreateProductMappingInput) =>
    platformRequest<ApiProductMapping>(`/projects/${encodeURIComponent(projectId)}/remix-product-mappings`, 'POST', input),
  getProductMapping: (projectId: string, mappingId: string) =>
    platformRequest<ApiProductMapping>(`/projects/${encodeURIComponent(projectId)}/remix-product-mappings/${encodeURIComponent(mappingId)}`),
  generatePlanFromProductMapping: (projectId: string, mappingId: string) =>
    platformRequest<ApiRemixPlan>(`/projects/${encodeURIComponent(projectId)}/remix-product-mappings/${encodeURIComponent(mappingId)}/plans`, 'POST'),
  createRemixPreroll: (projectId: string, input: ApiCreateRemixPrerollInput) =>
    platformRequest<ApiRemixPreroll>(`/projects/${encodeURIComponent(projectId)}/remix-prerolls`, 'POST', input),
  getRemixPreroll: (projectId: string, prerollId: string) =>
    platformRequest<ApiRemixPreroll>(`/projects/${encodeURIComponent(projectId)}/remix-prerolls/${encodeURIComponent(prerollId)}`),
  applyRemixPreroll: (projectId: string, prerollId: string) =>
    platformRequest<ApiRemixPlan>(`/projects/${encodeURIComponent(projectId)}/remix-prerolls/${encodeURIComponent(prerollId)}/apply`, 'POST'),
  createRemixFeedbackEvent: (projectId: string, input: ApiCreateFeedbackEventInput) =>
    platformRequest<ApiFeedbackEvent>(`/projects/${encodeURIComponent(projectId)}/remix-feedback-events`, 'POST', input),
  listRemixFeedbackEvents: (projectId: string, targetType?: ApiFeedbackTargetType, targetId?: string, limit = 20) => {
    const search = new URLSearchParams({ limit: String(limit) })
    if (targetType) search.set('target_type', targetType)
    if (targetId) search.set('target_id', targetId)
    return platformRequest<{ items: ApiFeedbackEvent[] }>(`/projects/${encodeURIComponent(projectId)}/remix-feedback-events?${search.toString()}`)
  },
  getRemixAssetPerformance: (projectId: string) =>
    platformRequest<{ items: ApiAssetPerformance[] }>(`/projects/${encodeURIComponent(projectId)}/remix-asset-performance`),
  createPlannerWeightSnapshot: (projectId: string) =>
    platformRequest<ApiPlannerWeightSnapshot>(`/projects/${encodeURIComponent(projectId)}/remix-planner-weight-snapshots`, 'POST'),
  listAgentRuns: (projectId: string, limit = 20) =>
    platformRequest<{ items: ApiAgentRun[] }>(`/projects/${encodeURIComponent(projectId)}/agent-runs?limit=${limit}`),
  createAgentRun: (projectId: string, renderJobId: string) =>
    platformRequest<ApiAgentRun>(`/projects/${encodeURIComponent(projectId)}/agent-runs`, 'POST', {
      workflow: 'render_diagnosis',
      target: { render_job_id: renderJobId },
    }),
  getAgentRun: (projectId: string, runId: string) =>
    platformRequest<ApiAgentRun>(`/projects/${encodeURIComponent(projectId)}/agent-runs/${encodeURIComponent(runId)}`),
  cancelAgentRun: (projectId: string, runId: string) =>
    platformRequest<ApiAgentRun>(`/projects/${encodeURIComponent(projectId)}/agent-runs/${encodeURIComponent(runId)}/cancel`, 'POST'),
  importKnowledgeDocument: (projectId: string, input: { title: string; source_uri?: string; source_type?: string; text: string }) =>
    platformRequest<ApiKnowledgeDocument>(`/projects/${encodeURIComponent(projectId)}/knowledge/documents`, 'POST', input),
  listKnowledgeDocuments: (projectId: string, limit = 20) =>
    platformRequest<{ items: ApiKnowledgeDocument[] }>(`/projects/${encodeURIComponent(projectId)}/knowledge/documents?limit=${limit}`),
  searchKnowledge: (projectId: string, query: string, limit = 10) =>
    platformRequest<{ query: string; items: ApiKnowledgeSearchResult[] }>(
      `/projects/${encodeURIComponent(projectId)}/knowledge/search?q=${encodeURIComponent(query)}&limit=${limit}`,
    ),
}
