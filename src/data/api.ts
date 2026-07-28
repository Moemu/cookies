import {
  createKanonMedia,
  createKanonProject,
  getKanonCapabilities,
  getKanonJob,
  listKanonArtifacts,
  listKanonJobs,
  listKanonProjects,
  listKanonTasks,
  loadKanonAgencyWorkbench,
  unsupportedKanonWrite,
} from '../backend/kanon-api'

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

export type ApiAgencyHealthStatus = 'healthy' | 'watch' | 'blocked'
export type ApiAdPlatform = '巨量引擎' | '腾讯广告' | '快手磁力'
export type ApiBindingHealthStatus = 'normal' | 'warning' | 'expired'
export type ApiProjectProgressStage = 'intake' | 'strategy' | 'creative' | 'quality_check' | 'human_review' | 'delivery' | 'completed'
export type ApiQualityCheckStatus = 'queued' | 'running' | 'passed' | 'failed'
export type ApiQualityIssueSeverity = 'minor' | 'major' | 'critical'
export type ApiMaterialConfirmationStatus = 'confirmed' | 'changes_requested'

export type ApiOrganization = {
  id: string
  code: string
  name: string
  owner: string
  currency: 'CNY'
  timezone: 'Asia/Shanghai'
  updatedAt: string
}

export type ApiClient = {
  id: string
  organizationId: string
  code: string
  name: string
  industry: string
  owner: string
  healthStatus: ApiAgencyHealthStatus
  updatedAt: string
}

export type ApiBrand = {
  id: string
  organizationId: string
  clientId: string
  code: string
  name: string
  category: string
  productLines: string[]
  owner: string
  guidelineStatus: 'ready' | 'missing' | 'outdated'
  updatedAt: string
}

export type ApiAdAccountBinding = {
  id: string
  organizationId: string
  clientId: string
  brandId: string
  projectIds: string[]
  platform: ApiAdPlatform
  accountName: string
  accountDisplayId: string
  currency: 'CNY'
  timezone: 'Asia/Shanghai'
  permissionStatus: ApiBindingHealthStatus
  loginStatus: ApiBindingHealthStatus
  trackingStatus: ApiBindingHealthStatus
  owner: string
  boundAssetIds: string[]
  lastSyncedAt: string
}

export type ApiProjectProgress = {
  stage: ApiProjectProgressStage
  stageLabel: string
  stagePercent: number
  taskPercent: number
  riskStatus: ApiAgencyHealthStatus
  blocker?: string
  updatedAt: string
}

export type ApiAgencyProject = ApiProject & {
  organizationId: string
  clientId: string
  brandId: string
  progressDetail: ApiProjectProgress
}

export type ApiQualityCheckIssue = {
  id: string
  severity: ApiQualityIssueSeverity
  rule: string
  evidence: string
  suggestion: string
}

export type ApiQualityCheckRun = {
  id: string
  organizationId: string
  projectId: string
  assetId: string
  assetVersion: number
  status: ApiQualityCheckStatus
  model: string
  ruleVersion: string
  promptVersion: string
  summary: string
  issues: ApiQualityCheckIssue[]
  createdAt: string
  completedAt?: string
}

export type ApiMaterialConfirmation = {
  id: string
  organizationId: string
  projectId: string
  qualityCheckRunId: string
  assetId: string
  assetVersion: number
  status: ApiMaterialConfirmationStatus
  scope: string
  confirmedBy: string
  note: string
  createdAt: string
}

export type ApiAssetVersionRecord = {
  version: number
  createdBy: string
  sourceTaskId: string
  sourceType: 'model_generation' | 'manual_edit'
  sourceLabel: string
  createdAt: string
  changeSummary: string
}

export type ApiAssetAuthorizationScope = {
  platforms: ApiAdPlatform[]
  regions: string[]
  rightsHolder: string
  expiresAt: string
  note: string
}

export type ApiAssetVersionPointer = {
  id: string
  organizationId: string
  projectId: string
  assetId: string
  workingVersion: number
  qualityCheckedVersion?: number
  humanConfirmedVersion?: number
  deliveryVersion?: number
  versions: ApiAssetVersionRecord[]
  authorization: ApiAssetAuthorizationScope
  deliveryTarget: {
    platform: ApiAdPlatform
    region: string
  }
  owner: string
  updatedAt: string
}

export type ApiAgencyWorkbench = {
  organizations: ApiOrganization[]
  clients: ApiClient[]
  brands: ApiBrand[]
  projects: ApiAgencyProject[]
  adAccountBindings: ApiAdAccountBinding[]
  qualityCheckRuns: ApiQualityCheckRun[]
  materialConfirmations: ApiMaterialConfirmation[]
  assetVersionPointers: ApiAssetVersionPointer[]
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

export type ApiVideoPromptDimensionId =
  | 'task_goal_type'
  | 'quality_style_lighting'
  | 'environment_atmosphere'
  | 'camera_content'
  | 'music_sound'

export type ApiVideoPromptDimension = {
  id: ApiVideoPromptDimensionId
  label: string
  prompt: string
  evidence: string
}

export type ApiVideoReplicationPrompt = {
  source_asset: ApiAssetVersionRef
  source_title: string
  source_file_name?: string
  reference_image_name?: string
  user_instruction?: string
  dimensions: ApiVideoPromptDimension[]
  composite_prompt: string
  model_directive: string
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
const backendOrigin = viteEnv?.VITE_API_BASE_URL ?? ''
const apiBase = `${backendOrigin}/api`
const platformBase = `${backendOrigin}/platform/v1`

export const agencyWorkbenchSample: ApiAgencyWorkbench = {
  organizations: [{
    id: 'org-demo-agency',
    code: 'AGY',
    name: '星河增长代理商',
    owner: 'Mia Chen',
    currency: 'CNY',
    timezone: 'Asia/Shanghai',
    updatedAt: '2026-07-27T08:00:00.000Z',
  }],
  clients: [
    {
      id: 'client-nova-lifestyle',
      organizationId: 'org-demo-agency',
      code: 'NOVA',
      name: '诺瓦生活科技',
      industry: '智能家居',
      owner: 'Amelia Meng',
      healthStatus: 'healthy',
      updatedAt: '2026-07-27T07:40:00.000Z',
    },
    {
      id: 'client-orbit-health',
      organizationId: 'org-demo-agency',
      code: 'ORBIT',
      name: '环域健康',
      industry: '消费医疗',
      owner: 'Noah Xu',
      healthStatus: 'watch',
      updatedAt: '2026-07-27T07:35:00.000Z',
    },
  ],
  brands: [
    {
      id: 'brand-nova-home',
      organizationId: 'org-demo-agency',
      clientId: 'client-nova-lifestyle',
      code: 'NOVA-HOME',
      name: 'Nova Home',
      category: '智能清洁',
      productLines: ['扫拖机器人', '空气护理'],
      owner: 'Lin Wei',
      guidelineStatus: 'ready',
      updatedAt: '2026-07-27T07:20:00.000Z',
    },
    {
      id: 'brand-nova-kids',
      organizationId: 'org-demo-agency',
      clientId: 'client-nova-lifestyle',
      code: 'NOVA-KIDS',
      name: 'Nova Kids',
      category: '儿童陪伴硬件',
      productLines: ['学习灯', '陪伴音箱'],
      owner: 'Sofia Chen',
      guidelineStatus: 'outdated',
      updatedAt: '2026-07-26T09:10:00.000Z',
    },
    {
      id: 'brand-orbit-care',
      organizationId: 'org-demo-agency',
      clientId: 'client-orbit-health',
      code: 'ORBIT-CARE',
      name: 'Orbit Care',
      category: '健康管理',
      productLines: ['睡眠监测', '康复训练'],
      owner: 'Noah Xu',
      guidelineStatus: 'ready',
      updatedAt: '2026-07-27T06:50:00.000Z',
    },
  ],
  projects: [
    {
      id: 'project-nova-home-launch',
      organizationId: 'org-demo-agency',
      clientId: 'client-nova-lifestyle',
      brandId: 'brand-nova-home',
      name: 'Nova Home 夏季清洁增长',
      brand: 'Nova Home',
      objective: '验证家庭清洁痛点素材，提升搜索与信息流线索效率。',
      runtime: {
        code: 'NOVA-HOME-2607',
        product: '全屋扫拖机器人 S8',
        stage: '素材人工确认',
        progress: 64,
        status: 'active',
        owner: 'Lin Wei',
        budget: 260000,
        currency: 'CNY',
        timezone: 'Asia/Shanghai',
      },
      progressDetail: {
        stage: 'human_review',
        stageLabel: '素材人工确认',
        stagePercent: 60,
        taskPercent: 64,
        riskStatus: 'watch',
        blocker: '2 条视频素材仍需处理质检问题',
        updatedAt: '2026-07-27T07:50:00.000Z',
      },
      version: 3,
      createdAt: '2026-07-18T02:00:00.000Z',
      updatedAt: '2026-07-27T07:50:00.000Z',
    },
    {
      id: 'project-nova-kids-presale',
      organizationId: 'org-demo-agency',
      clientId: 'client-nova-lifestyle',
      brandId: 'brand-nova-kids',
      name: 'Nova Kids 开学季预售',
      brand: 'Nova Kids',
      objective: '围绕护眼与陪伴场景准备预售素材和账户测试计划。',
      runtime: {
        code: 'NOVA-KIDS-2608',
        product: 'AI 学习灯 L2',
        stage: '创意制作',
        progress: 38,
        status: 'active',
        owner: 'Sofia Chen',
        budget: 180000,
        currency: 'CNY',
        timezone: 'Asia/Shanghai',
      },
      progressDetail: {
        stage: 'creative',
        stageLabel: '创意制作',
        stagePercent: 45,
        taskPercent: 38,
        riskStatus: 'healthy',
        updatedAt: '2026-07-27T06:20:00.000Z',
      },
      version: 2,
      createdAt: '2026-07-20T03:10:00.000Z',
      updatedAt: '2026-07-27T06:20:00.000Z',
    },
    {
      id: 'project-orbit-care-sleep',
      organizationId: 'org-demo-agency',
      clientId: 'client-orbit-health',
      brandId: 'brand-orbit-care',
      name: 'Orbit Care 睡眠健康线索',
      brand: 'Orbit Care',
      objective: '验证睡眠监测教育素材，建立可扩量的获客计划。',
      runtime: {
        code: 'ORBIT-SLEEP-2607',
        product: '睡眠监测贴片 Pro',
        stage: '投放预检',
        progress: 76,
        status: 'active',
        owner: 'Noah Xu',
        budget: 320000,
        currency: 'CNY',
        timezone: 'Asia/Shanghai',
      },
      progressDetail: {
        stage: 'delivery',
        stageLabel: '投放预检',
        stagePercent: 35,
        taskPercent: 76,
        riskStatus: 'blocked',
        blocker: '腾讯广告账户追踪状态异常',
        updatedAt: '2026-07-27T07:10:00.000Z',
      },
      version: 4,
      createdAt: '2026-07-12T01:30:00.000Z',
      updatedAt: '2026-07-27T07:10:00.000Z',
    },
  ],
  adAccountBindings: [
    {
      id: 'binding-nova-home-juliang',
      organizationId: 'org-demo-agency',
      clientId: 'client-nova-lifestyle',
      brandId: 'brand-nova-home',
      projectIds: ['project-nova-home-launch'],
      platform: '巨量引擎',
      accountName: 'Nova Home 巨量演示账户',
      accountDisplayId: 'JLY-DEMO-NOVA-001',
      currency: 'CNY',
      timezone: 'Asia/Shanghai',
      permissionStatus: 'normal',
      loginStatus: 'normal',
      trackingStatus: 'normal',
      owner: 'Noah Xu',
      boundAssetIds: ['asset-nova-home-hook', 'asset-nova-home-proof'],
      lastSyncedAt: '2026-07-27T07:55:00.000Z',
    },
    {
      id: 'binding-nova-kids-kuaishou',
      organizationId: 'org-demo-agency',
      clientId: 'client-nova-lifestyle',
      brandId: 'brand-nova-kids',
      projectIds: ['project-nova-kids-presale'],
      platform: '快手磁力',
      accountName: 'Nova Kids 快手演示账户',
      accountDisplayId: 'KS-DEMO-NOVA-018',
      currency: 'CNY',
      timezone: 'Asia/Shanghai',
      permissionStatus: 'normal',
      loginStatus: 'warning',
      trackingStatus: 'normal',
      owner: 'Sofia Chen',
      boundAssetIds: ['asset-nova-kids-scene'],
      lastSyncedAt: '2026-07-27T06:05:00.000Z',
    },
    {
      id: 'binding-orbit-care-tencent',
      organizationId: 'org-demo-agency',
      clientId: 'client-orbit-health',
      brandId: 'brand-orbit-care',
      projectIds: ['project-orbit-care-sleep'],
      platform: '腾讯广告',
      accountName: 'Orbit Care 腾讯演示账户',
      accountDisplayId: 'TX-DEMO-ORBIT-026',
      currency: 'CNY',
      timezone: 'Asia/Shanghai',
      permissionStatus: 'normal',
      loginStatus: 'normal',
      trackingStatus: 'expired',
      owner: 'Noah Xu',
      boundAssetIds: ['asset-orbit-sleep-education', 'asset-orbit-sleep-proof'],
      lastSyncedAt: '2026-07-26T23:30:00.000Z',
    },
  ],
  qualityCheckRuns: [
    {
      id: 'qc-nova-home-hook-v2',
      organizationId: 'org-demo-agency',
      projectId: 'project-nova-home-launch',
      assetId: 'asset-nova-home-hook',
      assetVersion: 2,
      status: 'failed',
      model: 'demo-quality-vision-v1',
      ruleVersion: 'agency-material-rules-2026-07',
      promptVersion: 'material-check-2026-07-15',
      summary: '画面卖点清晰，但价格权益口径与 Brief 不一致，需要修改。',
      issues: [{
        id: 'issue-nova-home-price',
        severity: 'major',
        rule: '价格权益一致性',
        evidence: '第 6 秒字幕出现未在 Brief 中确认的限时优惠。',
        suggestion: '删除价格承诺，改为“预约领取清洁方案”。',
      }],
      createdAt: '2026-07-27T05:10:00.000Z',
      completedAt: '2026-07-27T05:12:00.000Z',
    },
    {
      id: 'qc-nova-home-proof-v1',
      organizationId: 'org-demo-agency',
      projectId: 'project-nova-home-launch',
      assetId: 'asset-nova-home-proof',
      assetVersion: 1,
      status: 'passed',
      model: 'demo-quality-vision-v1',
      ruleVersion: 'agency-material-rules-2026-07',
      promptVersion: 'material-check-2026-07-15',
      summary: '品牌露出、产品画面和 CTA 均满足当前项目要求。',
      issues: [],
      createdAt: '2026-07-26T09:00:00.000Z',
      completedAt: '2026-07-26T09:02:00.000Z',
    },
    {
      id: 'qc-orbit-sleep-proof-v3',
      organizationId: 'org-demo-agency',
      projectId: 'project-orbit-care-sleep',
      assetId: 'asset-orbit-sleep-proof',
      assetVersion: 3,
      status: 'passed',
      model: 'demo-quality-vision-v1',
      ruleVersion: 'agency-material-rules-2026-07',
      promptVersion: 'material-check-2026-07-15',
      summary: '医学表述使用“辅助观察”，未出现诊断承诺。',
      issues: [],
      createdAt: '2026-07-27T03:40:00.000Z',
      completedAt: '2026-07-27T03:44:00.000Z',
    },
  ],
  materialConfirmations: [
    {
      id: 'confirm-nova-home-proof-v1',
      organizationId: 'org-demo-agency',
      projectId: 'project-nova-home-launch',
      qualityCheckRunId: 'qc-nova-home-proof-v1',
      assetId: 'asset-nova-home-proof',
      assetVersion: 1,
      status: 'confirmed',
      scope: 'Nova Home 夏季清洁增长 / 信息流素材',
      confirmedBy: 'Lin Wei',
      note: '可进入投放计划，需保留当前字幕版本。',
      createdAt: '2026-07-26T09:30:00.000Z',
    },
    {
      id: 'confirm-nova-home-hook-v2',
      organizationId: 'org-demo-agency',
      projectId: 'project-nova-home-launch',
      qualityCheckRunId: 'qc-nova-home-hook-v2',
      assetId: 'asset-nova-home-hook',
      assetVersion: 2,
      status: 'changes_requested',
      scope: 'Nova Home 夏季清洁增长 / 短视频钩子',
      confirmedBy: 'Amelia Meng',
      note: '请移除未经确认的优惠口径后再提交新版本。',
      createdAt: '2026-07-27T05:40:00.000Z',
    },
    {
      id: 'confirm-orbit-sleep-proof-v3',
      organizationId: 'org-demo-agency',
      projectId: 'project-orbit-care-sleep',
      qualityCheckRunId: 'qc-orbit-sleep-proof-v3',
      assetId: 'asset-orbit-sleep-proof',
      assetVersion: 3,
      status: 'confirmed',
      scope: 'Orbit Care 睡眠健康线索 / 腾讯广告教育素材',
      confirmedBy: 'Noah Xu',
      note: '允许用于预检，但需先修复账户追踪异常。',
      createdAt: '2026-07-27T04:10:00.000Z',
    },
  ],
  assetVersionPointers: [
    {
      id: 'pointer-nova-home-hook',
      organizationId: 'org-demo-agency',
      projectId: 'project-nova-home-launch',
      assetId: 'asset-nova-home-hook',
      workingVersion: 3,
      qualityCheckedVersion: 2,
      versions: [
        {
          version: 3,
          createdBy: 'Lin Wei',
          sourceTaskId: 'creative-nova-home-hook-fix',
          sourceType: 'manual_edit',
          sourceLabel: '人工字幕返修',
          createdAt: '2026-07-27T06:00:00.000Z',
          changeSummary: '移除未经确认的限时优惠口径，保留清洁痛点钩子。',
        },
        {
          version: 2,
          createdBy: 'demo-quality-vision-v1',
          sourceTaskId: 'creative-nova-home-hook-gen',
          sourceType: 'model_generation',
          sourceLabel: 'AI 生成短视频钩子',
          createdAt: '2026-07-27T05:00:00.000Z',
          changeSummary: '生成 9:16 钩子视频，新增权益字幕。',
        },
        {
          version: 1,
          createdBy: 'Lin Wei',
          sourceTaskId: 'creative-nova-home-hook-draft',
          sourceType: 'manual_edit',
          sourceLabel: '脚本初稿',
          createdAt: '2026-07-26T10:30:00.000Z',
          changeSummary: '建立家庭清洁痛点开场与产品露出。',
        },
      ],
      authorization: {
        platforms: ['巨量引擎'],
        regions: ['中国大陆'],
        rightsHolder: 'Nova Home',
        expiresAt: '2026-09-30T15:59:59.000Z',
        note: '仅限 Nova Home 夏季清洁增长项目的信息流素材测试。',
      },
      deliveryTarget: {
        platform: '巨量引擎',
        region: '中国大陆',
      },
      owner: 'Lin Wei',
      updatedAt: '2026-07-27T06:00:00.000Z',
    },
    {
      id: 'pointer-nova-home-proof',
      organizationId: 'org-demo-agency',
      projectId: 'project-nova-home-launch',
      assetId: 'asset-nova-home-proof',
      workingVersion: 1,
      qualityCheckedVersion: 1,
      humanConfirmedVersion: 1,
      deliveryVersion: 1,
      versions: [
        {
          version: 1,
          createdBy: 'Lin Wei',
          sourceTaskId: 'creative-nova-home-proof-edit',
          sourceType: 'manual_edit',
          sourceLabel: '产品证明段剪辑',
          createdAt: '2026-07-26T08:40:00.000Z',
          changeSummary: '保留产品清洁前后对比和标准 CTA。',
        },
      ],
      authorization: {
        platforms: ['巨量引擎', '腾讯广告'],
        regions: ['中国大陆', '中国香港'],
        rightsHolder: 'Nova Home',
        expiresAt: '2026-10-31T15:59:59.000Z',
        note: '授权覆盖清洁产品证明段，可用于当前项目交付包。',
      },
      deliveryTarget: {
        platform: '巨量引擎',
        region: '中国大陆',
      },
      owner: 'Lin Wei',
      updatedAt: '2026-07-26T09:30:00.000Z',
    },
    {
      id: 'pointer-orbit-sleep-proof',
      organizationId: 'org-demo-agency',
      projectId: 'project-orbit-care-sleep',
      assetId: 'asset-orbit-sleep-proof',
      workingVersion: 3,
      qualityCheckedVersion: 3,
      humanConfirmedVersion: 3,
      versions: [
        {
          version: 3,
          createdBy: 'Noah Xu',
          sourceTaskId: 'creative-orbit-proof-medical-copy',
          sourceType: 'manual_edit',
          sourceLabel: '医学口径人工修订',
          createdAt: '2026-07-27T03:30:00.000Z',
          changeSummary: '将诊断承诺改为辅助观察表述。',
        },
        {
          version: 2,
          createdBy: 'demo-quality-vision-v1',
          sourceTaskId: 'creative-orbit-proof-gen',
          sourceType: 'model_generation',
          sourceLabel: 'AI 生成教育素材',
          createdAt: '2026-07-26T11:20:00.000Z',
          changeSummary: '生成睡眠监测教育素材初版。',
        },
        {
          version: 1,
          createdBy: 'Noah Xu',
          sourceTaskId: 'creative-orbit-proof-script',
          sourceType: 'manual_edit',
          sourceLabel: '脚本导入',
          createdAt: '2026-07-25T09:00:00.000Z',
          changeSummary: '导入客户提供的睡眠健康教育脚本。',
        },
      ],
      authorization: {
        platforms: ['巨量引擎'],
        regions: ['中国大陆'],
        rightsHolder: 'Orbit Care',
        expiresAt: '2026-08-31T15:59:59.000Z',
        note: '当前授权未覆盖腾讯广告，因此不可设为腾讯广告交付版本。',
      },
      deliveryTarget: {
        platform: '腾讯广告',
        region: '中国大陆',
      },
      owner: 'Noah Xu',
      updatedAt: '2026-07-27T04:10:00.000Z',
    },
  ],
}

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
    notes: '由爆款复刻流程提交，先用视觉理解模型拆解五维视频提示词，再输入视频生成模型生成复刻视频。',
  }
}

export function buildLocalHitAnalysis(projectId: string, input: ApiCreateHitAnalysisInput): ApiHitAnalysis {
  const now = new Date().toISOString()
  const duration = Math.max(9, input.duration_seconds)
  const openingEnd = Math.max(3, Math.round(duration * 0.16))
  const problemEnd = Math.max(openingEnd + 3, Math.round(duration * 0.36))
  const proofEnd = Math.max(problemEnd + 3, Math.round(duration * 0.68))
  const offerEnd = Math.max(proofEnd + 2, Math.round(duration * 0.86))
  return {
    id: `local-hit-${input.source_asset.asset_id}-${Date.now()}`,
    organization_id: 'demo-org',
    project_id: projectId,
    source_asset: input.source_asset,
    title: input.title,
    video_meta: {
      duration_seconds: duration,
      language: input.language ?? 'zh-CN',
    },
    segments: [
      {
        id: 'seg-hook',
        start_seconds: 0,
        end_seconds: openingEnd,
        role: 'hook',
        summary: '用强冲突或高反差画面在开头建立停留理由。',
        script: '先别划走，这个结果和你想的不一样。',
        visual_element: '近景人物停顿、屏幕弹窗、快速推入主体。',
        conversion_cue: '建立注意力和问题意识。',
        replication_hint: '复刻开头停顿、反差和字幕强调，不复用原片人物和构图。',
      },
      {
        id: 'seg-problem',
        start_seconds: openingEnd,
        end_seconds: problemEnd,
        role: 'problem',
        summary: '放大受众痛点，让观众理解为什么需要继续看。',
        script: '如果你也遇到这个问题，先看这一步。',
        visual_element: '问题场景、失败瞬间、对比字幕。',
        conversion_cue: '让痛点与目标产品建立关联。',
        replication_hint: '保留问题升级节奏，替换为当前 Project 的业务场景。',
      },
      {
        id: 'seg-proof',
        start_seconds: problemEnd,
        end_seconds: proofEnd,
        role: 'proof',
        summary: '用过程、数据或细节镜头证明解决方案有效。',
        script: '关键不是更复杂，而是更稳定地做到。',
        visual_element: '产品特写、流程证明、结果对比、数据角标。',
        conversion_cue: '展示可信证据和卖点。',
        replication_hint: '复刻证明段功能，用授权素材和新产品卖点重建。',
      },
      {
        id: 'seg-offer',
        start_seconds: proofEnd,
        end_seconds: offerEnd,
        role: 'offer',
        summary: '将卖点转成可感知的利益点。',
        script: '现在就能把这套方法用到你的场景里。',
        visual_element: '利益点字幕、轻微加速剪辑、正向反馈画面。',
        conversion_cue: '降低行动成本。',
        replication_hint: '复刻利益转译方式，避免照搬原口播。',
      },
      {
        id: 'seg-cta',
        start_seconds: offerEnd,
        end_seconds: duration,
        role: 'cta',
        summary: '用清晰行动指令完成转化收口。',
        script: '点击预约，获取你的专属方案。',
        visual_element: 'CTA 定格、品牌露出、按钮式字幕。',
        conversion_cue: '引导点击或留资。',
        replication_hint: '保留清晰收口和停顿，不复制原片品牌资产。',
      },
    ],
    scripts: [
      { segment_id: 'seg-hook', text: '先别划走，这个结果和你想的不一样。' },
      { segment_id: 'seg-proof', text: '关键不是更复杂，而是更稳定地做到。' },
      { segment_id: 'seg-cta', text: '点击预约，获取你的专属方案。' },
    ],
    visual_elements: ['高反差开场', '问题场景', '产品证明', '数据角标', 'CTA 定格'],
    conversion_nodes: [
      { segment_id: 'seg-hook', cue: '停留' },
      { segment_id: 'seg-proof', cue: '信任' },
      { segment_id: 'seg-cta', cue: '行动' },
    ],
    replication_insights: ['复刻结构与镜头功能，不复刻原片具体表达。', '先建立冲突，再给证据，最后用明确 CTA 收口。'],
    created_at: now,
    updated_at: now,
  }
}

export function buildVideoReplicationPrompt(
  analysis: ApiHitAnalysis,
  input: {
    productName: string
    sellingPoints: string[]
    cta: string
    sourceFileName?: string
    referenceImageName?: string
    userInstruction?: string
  },
): ApiVideoReplicationPrompt {
  const opening = analysis.segments.find(segment => segment.role === 'hook') ?? analysis.segments[0]
  const proof = analysis.segments.find(segment => segment.role === 'proof') ?? analysis.segments[Math.min(2, analysis.segments.length - 1)]
  const ctaSegment = analysis.segments.find(segment => segment.role === 'cta') ?? analysis.segments.at(-1)
  const sellingPoints = input.sellingPoints.filter(Boolean)
  const sellingPointText = sellingPoints.length ? sellingPoints.join('、') : input.productName
  const rhythm = analysis.segments.map(segment => `${segment.start_seconds}-${segment.end_seconds}s ${segment.role}`).join('；')
  const instructionText = input.userInstruction?.trim()
  const imageText = input.referenceImageName?.trim()
  const multimodalReference = [
    '源视频用于复刻节奏、镜头功能和声音结构',
    instructionText ? `文本指令优先约束内容改写：${instructionText}` : '',
    imageText ? `参考图片用于约束主体外观、产品形态、色彩或构图气质：${imageText}` : '',
  ].filter(Boolean).join('；')
  const dimensions: ApiVideoPromptDimension[] = [
    {
      id: 'task_goal_type',
      label: '任务目标类型',
      prompt: `生成一条 ${analysis.video_meta.duration_seconds}s 左右的爆款复刻广告视频，目标是复刻源视频的停留结构、节奏推进和转化节点，同时将主题替换为「${input.productName}」。核心转化目标：${input.cta}。${instructionText ? `必须遵守文本指令：${instructionText}。` : ''}`,
      evidence: `源视频结构：${rhythm}`,
    },
    {
      id: 'quality_style_lighting',
      label: '画质&风格&光影规范',
      prompt: `保持高质感商业广告画质，画面干净锐利，主体边缘清晰；参考源视频的强对比停留点与 ${proof?.visual_element ?? '产品证明镜头'}，使用真实材质、高光轮廓、局部微距和可读字幕。${imageText ? `参考图片「${imageText}」用于校准主体形态、材质、配色和视觉气质。` : ''}避免低清、闪烁、畸变和过度相似的原片复制。`,
      evidence: [proof?.visual_element ?? analysis.visual_elements.join(' / '), imageText ? `参考图片：${imageText}` : ''].filter(Boolean).join('；'),
    },
    {
      id: 'environment_atmosphere',
      label: '环境氛围',
      prompt: `营造与「${input.productName}」匹配的可信商业场景：先制造问题压力，再进入解决方案展示，最后转向明确行动氛围；整体情绪从紧张、好奇推进到信任、确定。${instructionText ? `氛围表达需贴合文本指令中的限制和创意方向。` : ''}`,
      evidence: analysis.replication_insights.join('；'),
    },
    {
      id: 'camera_content',
      label: '镜头画面内容',
      prompt: `按源视频镜头功能复刻而非逐帧照抄：开场 ${opening?.summary ?? '用强钩子建立停留'}；中段展示 ${sellingPointText} 的证据镜头；结尾呈现 ${ctaSegment?.conversion_cue ?? input.cta}。镜头包含人物/产品近景、过程证明、字幕强调、CTA 定格。`,
      evidence: analysis.segments.map(segment => `${segment.role}: ${segment.summary}`).join('；'),
    },
    {
      id: 'music_sound',
      label: '音乐&音效',
      prompt: `音乐使用短视频平台高留存节奏：前 2 秒有清晰冲击音效，中段用递进鼓点承接证明信息，CTA 前加入短暂停顿和确认音；旁白与字幕语义一致，静音观看也能理解。`,
      evidence: analysis.scripts.map(script => script.text).join(' / '),
    },
  ]
  const compositePrompt = [
    `源视频参考：${analysis.title}${input.sourceFileName ? `（${input.sourceFileName}）` : ''}，Asset ${analysis.source_asset.asset_id} v${analysis.source_asset.version}。`,
    `多模态输入：${multimodalReference}。`,
    ...dimensions.map(dimension => `【${dimension.label}】${dimension.prompt}`),
    '生成要求：视频参考负责节奏和镜头功能，图片参考负责主体视觉，文本指令负责内容改写和约束；三者冲突时以文本指令和版权安全为最高优先级。不复制原视频人物、商标、字幕、画面构图或受版权保护的表达。',
  ].join('\n')
  return {
    source_asset: analysis.source_asset,
    source_title: analysis.title,
    source_file_name: input.sourceFileName,
    reference_image_name: input.referenceImageName,
    user_instruction: instructionText,
    dimensions,
    composite_prompt: compositePrompt,
    model_directive: `multimodal video remake: source video + reference image + text instruction；输出 ${analysis.video_meta.duration_seconds}s，9:16 竖版，目标产品 ${input.productName}，CTA ${input.cta}`,
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
  listAgencyWorkbench: loadKanonAgencyWorkbench,
  getCapabilities: getKanonCapabilities,
  listProjects: listKanonProjects,
  createProject: createKanonProject,
  updateProject: async (_id: string, _input: Partial<Pick<ApiProject, 'name' | 'brand' | 'objective'>>) =>
    Promise.reject<ApiProject>(unsupportedKanonWrite('项目编辑')),
  listArtifacts: (projectId?: string) =>
    projectId ? listKanonArtifacts(projectId) : Promise.resolve([]),
  listPrerollArtifacts: async (scope: ApiPrerollScope) =>
    (await listKanonArtifacts(scope.projectId))
      .filter(artifact => artifact.kind === 'video')
      .map(artifact => ({ ...artifact, purpose: scope.purpose, prerollType: scope.prerollType })),
  listAssetFeatures: (_projectId: string, _organizationId = 'demo-org') =>
    Promise.resolve<{ items: ApiAssetFeature[] }>({ items: [] }),
  listTasks: (projectId?: string) =>
    projectId ? listKanonTasks(projectId) : Promise.resolve([]),
  getTask: (id: string) =>
    request<ApiBusinessTask>(`/tasks/${encodeURIComponent(id)}`),
  createTask: (input: {
    projectId: string
    type: ApiBusinessTaskType
    name: string
    objective: string
    sourceTaskIds?: string[]
    sourceArtifactIds?: string[]
  }) => Promise.reject<ApiBusinessTask>(unsupportedKanonWrite(`“${input.name}”任务创建`)),
  updateTask: (
    id: string,
    input: Partial<Pick<ApiBusinessTask, 'name' | 'objective' | 'status' | 'sourceTaskIds' | 'sourceArtifactIds' | 'outputArtifactIds'>>,
  ) => Promise.reject<ApiBusinessTask>(unsupportedKanonWrite(`任务 ${id} 更新`)),
  createArtifact: (input: {
    projectId: string
    kind: ApiArtifact['kind']
    content: string
    status?: ApiArtifact['status']
    sourceJobId?: string
  }) => Promise.reject<ApiArtifact>(unsupportedKanonWrite('通用产物创建')),
  updateArtifact: (
    id: string,
    input: Partial<Pick<ApiArtifact, 'content' | 'status' | 'sourceJobId'>>,
  ) => Promise.reject<ApiArtifact>(unsupportedKanonWrite(`产物 ${id} 更新`)),
  listJobs: (projectId?: string) =>
    projectId ? listKanonJobs(projectId) : Promise.resolve([]),
  listPrerollJobs: async (scope: ApiPrerollScope) =>
    (await listKanonJobs(scope.projectId))
      .filter(job => job.artifactKind === 'video')
      .map(job => ({ ...job, purpose: scope.purpose, prerollType: scope.prerollType })),
  getJob: getKanonJob,
  getPrerollJob: async (id: string, scope: ApiPrerollScope) => ({
    ...await getKanonJob(id),
    purpose: scope.purpose,
    prerollType: scope.prerollType,
  }),
  cancelJob: async (id: string, _scope?: ApiPrerollScope) =>
    Promise.reject<ApiGenerationJob>(unsupportedKanonWrite(`模型作业 ${id} 取消`)),
  generateBrief: async (_projectId: string, _prompt: string) =>
    Promise.reject<{ job: ApiGenerationJob; artifact: ApiArtifact }>(
      unsupportedKanonWrite('一键生成 Brief'),
    ),
  createMedia: (
    projectId: string,
    kind: 'image' | 'video',
    prompt: string,
    briefId: string,
  ) => createKanonMedia(projectId, kind, prompt, briefId),
  createPrerollVideo: (
    scope: ApiPrerollScope,
    prompt: string,
    briefId: string,
  ) => createKanonMedia(scope.projectId, 'video', prompt, briefId)
    .then(job => ({ ...job, purpose: scope.purpose, prerollType: scope.prerollType })),
  planShortDramaPreroll: async (
    _projectId: string,
    _briefId: string,
    _storyContext: ApiShortDramaStoryContext,
  ) => Promise.reject<ApiShortDramaPrerollPlan>(
    unsupportedKanonWrite('短剧前贴候选规划'),
  ),
  createShortDramaPrerollVideo: (
    scope: ApiPrerollScope & { prerollType: 'short_drama' },
    briefId: string,
    planVersion: ApiShortDramaPrerollPlan['version'],
    candidateId: string,
    storyContext: ApiShortDramaStoryContext,
  ) => createKanonMedia(
    scope.projectId,
    'video',
    [
      `短剧：${storyContext.title}`,
      storyContext.synopsis,
      `已审核卖点：${storyContext.reviewedSellingPoints.join('；')}`,
      storyContext.openingLine ? `开场台词：${storyContext.openingLine}` : '',
      `候选方案：${candidateId}（${planVersion}）`,
      '生成 9:16、6 秒、静音可理解的短剧广告前贴，并保留稳定拼接点。',
    ].filter(Boolean).join('。'),
    briefId,
  ).then(job => ({ ...job, purpose: scope.purpose, prerollType: scope.prerollType })),
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
