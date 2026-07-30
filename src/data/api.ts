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

// 复盘报告（03 AM-015）。一次投放执行对应一份报告：汇总素材表现、结论，
// 确认之后从它沉淀经验。后端权威定义在 internal/systems/insights/service.go。
export type ApiReportStatus = 'draft' | 'confirmed'

export type ApiInsightReport = {
  id: string
  organization_id: string
  project_id: string
  execution_id: string
  delivery_mode: string
  evidence_id: string
  evidence_summary: string
  metric_snapshot_id: string
  creative_package_id: string
  // 模拟投放的报告不能和真实投放的报告混着看：结论的分量不一样。
  is_simulated: boolean
  dataset_version: string
  status: ApiReportStatus
  summary: string
  findings: string[]
  version: number
  created_by: string
  confirmed_by?: string
  confirmed_at?: string
  created_at: string
  updated_at: string
}

export type ApiExperienceStatus = 'pending' | 'confirmed' | 'needs_review' | 'retired'

export type ApiExperience = {
  id: string
  organization_id: string
  project_id: string
  lineage_id: string
  revision: number
  supersedes_id?: string
  superseded_by_id?: string
  report_id: string
  source_execution_id: string
  source_evidence_id: string
  source_metric_snapshot_id: string
  conclusion: string
  conditions: string[]
  counterexamples: string[]
  status: ApiExperienceStatus
  status_reason: string
  status_changed_by: string
  status_changed_at?: string
  confirmed_by?: string
  confirmed_at?: string
  version: number
  created_by: string
  created_at: string
  updated_at: string
}

export type ApiExperienceAudit = {
  id: string
  organization_id: string
  project_id: string
  experience_id: string
  from_status: string
  to_status: ApiExperienceStatus
  reason: string
  actor_id: string
  created_at: string
}

// 引用之后发生了什么。四挡是有序的：只是引用 → 照做 → 改了之后用 → 没采纳。
export type ApiExperienceReferenceOutcome = 'referenced' | 'adopted' | 'modified' | 'rejected'

export type ApiExperienceReference = {
  id: string
  organization_id: string
  project_id: string
  experience_id: string
  consumer_kind: string
  consumer_id: string
  outcome: string
  note: string
  version: number
  created_by: string
  created_at: string
  updated_at: string
}

// 投前洞察卡（03 §8.1 九字段）。洞察卡不是一张新表，是经验库的投影：
// 后端权威定义在 internal/systems/insights/prelaunch.go，契约见 api/openapi/insights-v1.yaml。

// 类型承担 03 §2 目标⑥：把事实、计算、推断和人的结论分开。
// 只有 fact 和 statistic 能被下游当证据引用——拿假设当证据是循环论证。
export type ApiInsightCardType = 'fact' | 'statistic' | 'hypothesis' | 'recommendation'

export type ApiApplicability = {
  brands?: string[]
  products?: string[]
  channels?: string[]
  creative_types?: string[]
  objectives?: string[]
  audiences?: string[]
  time_range_note?: string
}

export type ApiDataBasis = {
  asset_count?: number
  sample_size?: number
  window_start?: string
  window_end?: string
  metrics?: string[]
  baseline?: string
}

export type ApiContentBasis = {
  features?: string[]
  example_asset_versions?: string[]
  note?: string
}

export type ApiInsightCard = {
  experience_id: string
  lineage_id: string
  revision: number
  conclusion: string
  type: ApiInsightCardType
  type_label: string
  applicability: ApiApplicability
  conditions: string[]
  data_basis: ApiDataBasis
  content_basis: ApiContentBasis
  confidence: ApiConfidenceLevel
  confidence_hint: string
  counterexamples: string[]
  recommended_action: string
  status: ApiExperienceStatus
  // 九字段没填全的卡照样返回，缺哪几项写在这里。藏起来会让人以为经验库是空的。
  missing_fields: string[]
  reference_count: number
  updated_at: string
}

export type ApiFeaturePattern = {
  feature: string
  card_count: number
  channels: string[]
  // 取最强置信而不是平均：一条充分证据和一条样本不足不该被平均成方向性。
  best_confidence: ApiConfidenceLevel
  conclusions: string[]
}

export type ApiPreLaunchFacets = {
  channels: string[]
  creative_types: string[]
  objectives: string[]
}

export type ApiPreLaunchInsight = {
  project_id: string
  cards: ApiInsightCard[]
  patterns: ApiFeaturePattern[]
  facets: ApiPreLaunchFacets
  // 筛选期间被排除的「没写适用范围」的经验条数。不静默丢弃。
  unscoped_excluded: number
  mixed_channels: string[]
  cross_channel_comparison: boolean
  // quality_checked=false 表示这一次没能做数据质量校验，不是「校验通过」。
  quality_checked: boolean
  strong_conclusions_allowed: boolean
  quality_blockers: string[]
  experience_references: ApiExperience[]
  disclosure: string
}

export type ApiPreLaunchFilter = {
  channel?: string
  creative_type?: string
  objective?: string
  q?: string
  cross_channel?: boolean
}

// 分析素材库与内容分析（03 §9 AM-001~006）。后端权威定义在
// internal/systems/insights/assets.go 与 features.go，契约见 api/openapi/insights-v1.yaml。

export type ApiAnalysisStatus =
  | 'awaiting_data' | 'awaiting_match' | 'analysable' | 'analysing'
  | 'pending_confirmation' | 'confirmed' | 'needs_review' | 'retired'

export type ApiInsightAssetType =
  | 'xiaohongshu_note' | 'wechat_article' | 'brand_ad'
  | 'digital_human_ad' | 'preroll_ad' | 'hit_replica_ad'

export type ApiAssetSourceKind = 'creative' | 'upload' | 'external'

/** AI 推断与人工结论是两层，互不覆盖（03 §14）。 */
export type ApiFeatureSource = 'ai' | 'human'

export type ApiConfidence = 'low' | 'medium' | 'high'

// authored 是「AI 没提过这一项，人第一个填的」，和 rejected（有推断但人不认）分开。
// 混成一个值会让人手填的特征被投后分析当成被否掉的推断丢掉。
export type ApiReviewState = 'pending' | 'confirmed' | 'rejected' | 'authored'

export type ApiMappingStatus = 'unmatched' | 'matched' | 'ignored'

export type ApiFeatureValueKind =
  | 'text' | 'tags' | 'enum' | 'enum_multi' | 'number' | 'bool' | 'duration_seconds'

export type ApiFeatureValue = {
  kind: ApiFeatureValueKind
  text?: string
  terms?: string[]
  number?: number
  bool?: boolean
}

export type ApiInsightAsset = {
  id: string
  organization_id: string
  project_id: string
  lineage_id: string
  revision: number
  title: string
  source_kind: ApiAssetSourceKind
  source_ref?: string
  source_job_id?: string
  platform_asset_id?: string
  platform_asset_version?: number
  asset_type?: ApiInsightAssetType
  asset_type_source?: ApiFeatureSource
  asset_type_confidence?: ApiConfidence
  analysis_status: ApiAnalysisStatus
  analysis_status_reason?: string
  analysis_status_changed_at?: string
  version: number
  created_by: string
  created_at: string
  updated_at: string
}

export type ApiInsightAssetMapping = {
  id: string
  organization_id: string
  project_id: string
  platform: string
  platform_object_kind: string
  platform_object_id: string
  platform_object_name?: string
  asset_id?: string
  status: ApiMappingStatus
  match_source?: string
  matched_by?: string
  matched_at?: string
  note?: string
  version: number
  created_at: string
  updated_at: string
}

export type ApiInsightAssetFeature = {
  id: string
  organization_id: string
  project_id: string
  asset_id: string
  asset_type: ApiInsightAssetType
  key: string
  value: ApiFeatureValue
  source: ApiFeatureSource
  confidence?: ApiConfidence
  review_state: ApiReviewState
  skill_id?: string
  skill_version?: string
  extracted_at?: string
  version: number
  created_by: string
  created_at: string
  updated_at: string
}

// 写入用的特征条目：AI 层必须带 confidence，人工层必须不带（internal/systems/insights/assets.go FeatureInput）。
export type ApiFeatureInput = {
  key: string
  value: ApiFeatureValue
  confidence?: ApiConfidence
  review_state?: ApiReviewState
}

// 一次分析任务的留痕。这里没有输入正文，只有它的指纹和规模——
// 外部返回的内容和输入全文都不入日志（doc09 §7）。
export type ApiAnalysisRun = {
  id: string
  kind: 'feature_extraction'
  asset_id: string
  status: 'running' | 'succeeded' | 'failed'
  asset_type: ApiInsightAssetType
  skill_id?: string
  skill_version?: string
  skill_content_hash?: string
  prompt_version?: string
  provider_code?: string
  model_alias?: string
  model_version?: string
  generation_mode?: 'model' | 'template'
  input_hash?: string
  result_hash?: string
  feature_count: number
  dropped_fields?: string[]
  data_through?: string
  prompt_tokens: number
  completion_tokens: number
  latency_ms: number
  error_code?: string
  error_message?: string
  started_at: string
  finished_at?: string
  created_by: string
}

export type ApiAnalyzeAssetResult = {
  run: ApiAnalysisRun
  features: ApiInsightAssetFeature[]
  dropped_fields?: string[]
}

export type ApiFeatureField = {
  key: string
  label: string
  group: string
  kind: ApiFeatureValueKind
  vocabulary?: string[]
  unit?: string
  note?: string
}

export type ApiFeatureSchema = {
  asset_type: ApiInsightAssetType
  label: string
  source: string
  fields: ApiFeatureField[]
}

export type ApiFeatureMatrixCell = {
  asset_id: string
  source: ApiFeatureSource
  confidence?: ApiConfidence
  review_state: ApiReviewState
  value: ApiFeatureValue
}

export type ApiFeatureMatrixRow = {
  key: string
  label: string
  group: string
  kind: ApiFeatureValueKind
  cells: ApiFeatureMatrixCell[]
}

export type ApiFeatureMatrix = {
  assets: ApiInsightAsset[]
  asset_types: ApiInsightAssetType[]
  rows: ApiFeatureMatrixRow[]
  disclosure: string
}

export type ApiInsightAssetFilter = {
  statuses?: ApiAnalysisStatus[]
  assetTypes?: ApiInsightAssetType[]
  sourceKinds?: ApiAssetSourceKind[]
  lineageId?: string
  limit?: number
}

// 数据接入与投后分析指标（10-ad-data-connectors.md）。后端权威定义在
// internal/systems/insights/connectors.go，契约见 api/openapi/insights-v1.yaml。

export type ApiPlatform = 'douyin' | 'kuaishou' | 'xiaohongshu' | 'wechat' | 'tencent_ads' | 'other'

export type ApiIngestMode = 'api' | 'service_account' | 'file_import' | 'computer_use' | 'business'

export type ApiDataSourceStatus = 'draft' | 'active' | 'paused' | 'revoked'

/** doc10 §11。healthy 以外的任何取值都会阻止这个数据源的数字生成强结论（§12.4）。 */
export type ApiQualityStatus =
  | 'healthy' | 'delayed' | 'partial' | 'mapping_incomplete'
  | 'tracking_broken' | 'reconciling' | 'blocked'

export type ApiImportKind = 'sync' | 'backfill' | 'file' | 'correction'

export type ApiImportStatus = 'pending' | 'running' | 'succeeded' | 'partial' | 'failed'

/** 置信提示的四个档位（03 §9），刻意不是数字。 */
export type ApiConfidenceLevel = 'sufficient' | 'directional' | 'low_sample' | 'confounded'

/** 口径：两个数字并排放之前必须一致的东西（doc10 §6）。 */
export type ApiMetricCaliber = {
  time_zone: string
  currency: string
  attribution_window: string
  metric_schema_version: string
}

export type ApiMetricCounts = {
  impressions: number
  clicks: number
  conversions: number
  video_views: number
  video_completions: number
  spend_cents: number
  revenue_cents: number
}

/**
 * 派生指标。字段缺失表示「不可用」——分母为零时后端不返回该字段（doc10 §6）。
 * 前端必须显示「不可用」，不能当成 0 参与排序或比较。
 */
export type ApiMetricRates = {
  ctr?: number
  cvr?: number
  completion_rate?: number
  cpa_cents?: number
  cpm_cents?: number
  roas?: number
}

export type ApiRateInterval = { low: number; high: number }

export type ApiDataSource = {
  id: string
  organization_id: string
  project_id: string
  platform: ApiPlatform
  account_label?: string
  account_ref?: string
  ingest_mode: ApiIngestMode
  credential_ref?: string
  status: ApiDataSourceStatus
  quality_status: ApiQualityStatus
  quality_note?: string
  caliber: ApiMetricCaliber
  field_mapping?: Record<string, string>
  data_through?: string
  last_synced_at?: string
  version: number
  created_by: string
  created_at: string
  updated_at: string
}

export type ApiImportBatch = {
  id: string
  organization_id: string
  project_id: string
  data_source_id: string
  kind: ApiImportKind
  status: ApiImportStatus
  source_label?: string
  window_start?: string
  window_end?: string
  content_hash?: string
  requested_rows: number
  accepted_rows: number
  rejected_rows: number
  error_summary?: string
  errors?: string[]
  corrects_batch_id?: string
  started_at?: string
  finished_at?: string
  version: number
  created_by: string
  created_at: string
  updated_at: string
}

export type ApiAssetMetricPerformance = {
  asset_id?: string
  asset_title: string
  asset_type?: ApiInsightAssetType
  objects: number
  counts: ApiMetricCounts
  rates: ApiMetricRates
  attributable: boolean
  confidence: ApiConfidenceLevel
}

export type ApiPerformancePoint = {
  date: string
  counts: ApiMetricCounts
  rates: ApiMetricRates
}

export type ApiPlatformTotal = {
  platform: ApiPlatform
  label: string
  counts: ApiMetricCounts
  rates: ApiMetricRates
}

export type ApiSourceHealth = {
  data_source_id: string
  platform: ApiPlatform
  label: string
  quality_status: ApiQualityStatus
  quality_note?: string
  data_through?: string
  freshness_days: number
}

export type ApiMetricOverview = {
  window: { start: string; end: string }
  caliber: ApiMetricCaliber
  caliber_conflicts?: string[]
  comparable: boolean
  comparable_reason?: string
  totals: ApiMetricCounts
  rates: ApiMetricRates
  ctr_interval?: ApiRateInterval
  confidence: ApiConfidenceLevel
  confidence_note: string
  series: ApiPerformancePoint[]
  assets: ApiAssetMetricPerformance[]
  unmatched_objects: number
  unmatched_spend_cents: number
  sources: ApiSourceHealth[]
  warnings?: string[]
  platforms: ApiPlatformTotal[]
}

/**
 * AM-009 的判定阶梯，从严到松。**只有 attributable 是「能归到这个变量」**，
 * 其余四档都是「不能」，只是不能的理由不同。
 */
export type ApiVariantVerdict = 'attributable' | 'directional' | 'confounded' | 'low_sample' | 'no_features'

/** 两个素材之间某个特征的取值差异。未记录的一侧写作「（未记录）」。 */
export type ApiFeatureDiff = {
  key: string
  label: string
  group: string
  baseline: string
  variant: string
  /** 该特征只能人工判定（AM-006），AI 不产出，缺失属正常。 */
  human_only: boolean
}

/**
 * 素材对比（03 §7.3 / AM-009）。changed_features 长度 > 1 时 verdict 一定是
 * confounded——差异归不到其中任何一个变量上，哪怕数字差得很多。
 */
export type ApiVariantComparison = {
  baseline_asset_id: string
  baseline_title: string
  variant_asset_id: string
  variant_title: string
  asset_type?: ApiInsightAssetType
  changed_features: ApiFeatureDiff[]
  /** 两侧取值相同的特征数量——受控变量越多，单变量判定越可信。 */
  controlled_count: number
  baseline_counts: ApiMetricCounts
  variant_counts: ApiMetricCounts
  baseline_rates: ApiMetricRates
  variant_rates: ApiMetricRates
  baseline_ctr_interval?: ApiRateInterval
  variant_ctr_interval?: ApiRateInterval
  /** 任一侧区间算不出来时为 true——不知道差异是否显著，就不能说它显著。 */
  intervals_overlap: boolean
  ctr_lift?: number
  verdict: ApiVariantVerdict
  confidence: ApiConfidenceLevel
  note: string
}

/** direction 为 unknown 表示天数不足或前半段无曝光，不能当成持平。 */
export type ApiAssetTrend = {
  asset_id: string
  asset_title: string
  asset_type?: ApiInsightAssetType
  points: ApiPerformancePoint[]
  /** 真正有数据的天数，不是窗口长度。 */
  active_days: number
  direction: 'rising' | 'flat' | 'declining' | 'unknown'
  ctr_change?: number
  confidence: ApiConfidenceLevel
  note: string
}

/** likely 需要两项条件同时成立；单项恶化只到 watch。 */
export type ApiFatigueSeverity = 'none' | 'watch' | 'likely'

export type ApiFatigueSignal = {
  asset_id: string
  asset_title: string
  asset_type?: ApiInsightAssetType
  first_half: ApiMetricCounts
  second_half: ApiMetricCounts
  first_rates: ApiMetricRates
  last_rates: ApiMetricRates
  ctr_change?: number
  cpa_change?: number
  impression_change?: number
  severity: ApiFatigueSeverity
  /** 没能排除的其他解释。这里列的是「排除不了」，不是「已排除」。 */
  alternative_explanations?: string[]
  confidence: ApiConfidenceLevel
  note: string
}

export type ApiAnomalyKind = 'spike' | 'drop' | 'gap'

/** 用中位数与 MAD 判定偏离（阈值 3.5），不用均值与标准差：一次大促尖峰会把标准差本身抬高。 */
export type ApiMetricAnomaly = {
  date: string
  scope: 'project' | 'asset'
  asset_id?: string
  asset_title?: string
  metric: string
  kind: ApiAnomalyKind
  observed: number
  median: number
  /** 偏离中位数多少个 MAD。 */
  deviation: number
  note: string
}

/**
 * 驱动因素：某个特征取值的素材组与其余素材的对比。**这不是因果**——
 * covarying_features 非空时 confidence 一律降为 confounded。
 */
export type ApiFeatureDriver = {
  asset_type?: ApiInsightAssetType
  key: string
  label: string
  group: string
  value: string
  assets: number
  rest_assets: number
  counts: ApiMetricCounts
  rest_counts: ApiMetricCounts
  rates: ApiMetricRates
  rest_rates: ApiMetricRates
  ctr_interval?: ApiRateInterval
  rest_ctr_interval?: ApiRateInterval
  intervals_overlap: boolean
  ctr_lift?: number
  /** 与本特征完全同向变化的其他特征，分不开谁在起作用。 */
  covarying_features?: string[]
  confidence: ApiConfidenceLevel
  note: string
}

/**
 * 投后分析五个二级视图（素材对比 / 趋势 / 疲劳 / 异常 / 驱动因素）的共用载荷。
 * 五个视图共用一次返回，否则「趋势里看到的」和「疲劳里算的」会来自两次不同的读取。
 */
export type ApiPerformanceAnalysis = {
  window: { start: string; end: string }
  caliber: ApiMetricCaliber
  /** 口径不一致时为 false，所有 attributable 会被降级为 directional。 */
  comparable: boolean
  comparable_reason?: string
  comparisons: ApiVariantComparison[]
  trends: ApiAssetTrend[]
  fatigue: ApiFatigueSignal[]
  anomalies: ApiMetricAnomaly[]
  drivers: ApiFeatureDriver[]
  assets_in_window: number
  /** 其中有内容特征的素材数。远小于 assets_in_window 时，对比和驱动因素都会大面积空着。 */
  assets_with_features: number
  notes?: string[]
}

/** 质量问题的五个类别，对应数据质量的前五个二级视图。第六个「修复队列」是跨类别的队列视图。 */
export type ApiDataQualityIssueKind = 'freshness' | 'missing' | 'anomaly' | 'caliber' | 'reconciliation'

/** blocking 会让整个 Project 暂停强结论与自动优化（PRD §10.3、doc10 §12.4）。 */
export type ApiDataQualitySeverity = 'blocking' | 'warning' | 'info'

/**
 * 问题当前的处置状态。open 与 reopened 不入库，由后端把实时检测结果和处置记录比对算出。
 * reopened = 有人报了修但问题在那之后又被观测到——这件事必须被看见，
 * 否则点一次「已修复」就能让问题永久消失。
 */
export type ApiDataQualityIssueState = 'open' | 'acknowledged' | 'resolved' | 'ignored' | 'reopened'

/** 能被写进库的三种处置，是 ApiDataQualityIssueState 的子集。 */
export type ApiDataQualityDispositionState = 'acknowledged' | 'resolved' | 'ignored'

export type ApiDataQualityDisposition = {
  id: string
  organization_id: string
  project_id: string
  fingerprint: string
  issue_kind: ApiDataQualityIssueKind
  state: ApiDataQualityDispositionState
  note: string
  observed_through: string
  version: number
  decided_by: string
  created_at: string
  updated_at: string
}

export type ApiDataQualityIssue = {
  /** 同一个问题反复出现时保持不变，且不含数据窗口——换窗口看到的还是同一条。 */
  fingerprint: string
  kind: ApiDataQualityIssueKind
  severity: ApiDataQualitySeverity
  title: string
  detail: string
  /** 下一步该查什么。PRD §10.3 要求不能只报警不给动作。 */
  suggestion: string
  scope_label: string
  data_source_id?: string
  platform?: ApiPlatform
  affected_spend_cents: number
  affected_days?: number
  stat_date?: string
  /** 处置时要原样回传，后端靠它判断处置之后问题有没有再出现。 */
  last_observed_at: string
  state: ApiDataQualityIssueState
  disposition?: ApiDataQualityDisposition
}

export type ApiDataQualityReport = {
  window: { start: string; end: string }
  generated_at: string
  /** 已按「在队列里 → 严重度 → 影响花费」排好序（20 §4.1 错误与延迟置顶）。 */
  issues: ApiDataQualityIssue[]
  by_kind: Partial<Record<ApiDataQualityIssueKind, number>>
  open_count: number
  queue_count: number
  strong_conclusions_allowed: boolean
  blocked_reason?: string
  sources?: ApiSourceHealth[]
}

// ---- 能力运营（03 §一级导航；20 §4.1）----
// 五个二级视图共用一次请求。这个模块一张表都不建：特征体系读后端的六套 schema，
// 指标字典是后端声明的常量，Skill 与评测集从已有的特征行现算。

/** `fact` 可跨天相加；`derived` 只能用「总量除总量」重算，日均比率是另一个数。 */
export type ApiMetricKind = 'fact' | 'derived'

export type ApiCaliberFactor = 'time_zone' | 'currency' | 'attribution_window' | 'metric_schema_version'

export type ApiMetricDictionaryEntry = {
  key: string
  label: string
  kind: ApiMetricKind
  unit?: string
  definition: string
  formula?: string
  /** 这个指标的口径依赖哪些要素；只有依赖的要素冲突了才会影响它。 */
  caliber_factors?: ApiCaliberFactor[]
  source: string
  /** 本项目实际有多少天导过。0 表示一条都没有，和「有值但为 0」不是一回事。 */
  day_count: number
  total: number
  /** false 表示各数据源在它依赖的口径要素上不一致，跨源对比前要先展示差异（03 §7）。 */
  comparable: boolean
  conflict_notes?: ApiCaliberFactor[]
}

export type ApiCaliberConflict = {
  factor: ApiCaliberFactor
  label: string
  values: string[]
  note: string
}

export type ApiFeatureValueUsage = {
  value: string
  asset_count: number
}

export type ApiFeatureFieldUsage = ApiFeatureField & {
  /** 词表已发布。false 的枚举字段目前接受任何取值，这正是碎片化的敞口。 */
  governed: boolean
  asset_count: number
  distinct_values: number
  /** 按使用量降序，最多 12 个。统计用生效值：人工结论覆盖机器结论，机器旧值不再计入。 */
  values?: ApiFeatureValueUsage[]
  /** 全项目只有一条素材用过的取值。是候选不是结论——系统不做语义猜测。 */
  merge_candidates?: string[]
  off_vocabulary?: string[]
}

export type ApiFeatureSystemHealth = {
  asset_type: ApiInsightAssetType
  label: string
  source: string
  asset_count: number
  field_count: number
  used_field_count: number
  open_enum_count: number
  fields: ApiFeatureFieldUsage[]
}

export type ApiSkillHealth = {
  skill_id: string
  skill_version: string
  extraction_count: number
  asset_count: number
  field_keys: string[]
  high_confidence: number
  medium_confidence: number
  low_confidence: number
  first_extracted_at?: string
  last_extracted_at?: string
  /** 按最近一次提取时间判在用，不按版本号字符串排序（否则 v9 会排在 v10 后面）。 */
  latest: boolean
}

export type ApiEvaluationExample = {
  asset_id: string
  asset_title?: string
  feature_key: string
  label: string
  ai_value: string
  human_value: string
}

export type ApiFieldEvaluation = {
  key: string
  label: string
  reviewed: number
  agreed: number
}

/**
 * 一个 Skill 版本的人机一致率。**不是独立评测集**：样本全部来自人工复核记录，
 * 回答的是「被人看过的地方机器错了多少」，不是整体准确率。没人复核过的提取一条都
 * 不计入——算成「机器对了」，准确率会随提取量自动上涨。
 */
export type ApiSkillEvaluation = {
  skill_id: string
  skill_version: string
  reviewed: number
  agreed: number
  disagreed: number
  /** 样本少于 10 条时恒为 0，此时要藏起这个数而不是显示 0%。 */
  accuracy: number
  confidence: ApiConfidenceLevel
  note: string
  fields?: ApiFieldEvaluation[]
  examples?: ApiEvaluationExample[]
}

export type ApiOperationsTodo = {
  kind: string
  severity: string
  title: string
  detail: string
  asset_type?: ApiInsightAssetType
  feature_key?: string
}

export type ApiOperationsDashboard = {
  feature_field_total: number
  feature_field_used: number
  open_vocabulary_fields: number
  merge_candidate_count: number
  off_vocabulary_count: number
  caliber_conflict_count: number
  skill_version_count: number
  evaluation_samples: number
  /** 词表待办只覆盖已经有人用过的开放枚举字段，否则真正在散的那几个会被埋掉。 */
  todos: ApiOperationsTodo[]
}

export type ApiCapabilityOperations = {
  window: { start: string; end: string }
  generated_at: string
  feature_systems: ApiFeatureSystemHealth[]
  metrics: ApiMetricDictionaryEntry[]
  caliber_conflicts: ApiCaliberConflict[]
  skills: ApiSkillHealth[]
  evaluations: ApiSkillEvaluation[]
  dashboard: ApiOperationsDashboard
}

/**
 * 一条设置。effect 和 recommended 不是可选注释：20 §121 要求「重要阈值显示影响说明
 * 和默认推荐」，22 §239 记的问题正是「缺少实际阈值影响说明」。
 */
export type ApiSettingItem = {
  key: string
  label: string
  /** 现在生效的值，后端已格式化好（含单位），前端不要再拼。 */
  value: string
  effect: string
  recommended: string
  /**
   * 当前值偏离了推荐值，页面要提示「有人调过它」。由后端判定，前端不要自己拿
   * value 和 recommended 比字符串——确认权限那组两边说的是不同的事（管到哪些操作
   * vs 该发给谁），字面永远不同，一比就会给每一条都打上凭空造出来的告警。
   */
  deviates: boolean
  /** 值在代码里的位置，例如 internal/systems/insights/connectors.go:267。 */
  source: string
  /** 文档依据；没有依据会显式写「无文档指定值」，不会留空。 */
  basis: string
}

export type ApiSettingGroup = {
  key: string
  label: string
  /** in_effect 现在真的在生效；not_built 还没有任何东西，此时 items 为空。 */
  state: 'in_effect' | 'not_built'
  summary: string
  /** 只在 not_built 时有内容。 */
  missing: string[]
  items: ApiSettingItem[]
}

export type ApiInsightSettings = {
  generated_at: string
  /** 恒为 false。不要因此渲染禁用输入框——改不动的输入框比一句「这里改不了」更恼人。 */
  editable: boolean
  editable_note: string
  /** 恒为 false：这些值对整个部署生效，路径上的 project 只用于鉴权。 */
  project_scoped: boolean
  groups: ApiSettingGroup[]
}

/** 一行 canonical 日指标。stat_date 是数据源时区下的当地日期 YYYY-MM-DD。 */
export type ApiMetricRow = {
  platform_object_kind: string
  platform_object_id: string
  platform_object_name?: string
  stat_date: string
  counts: ApiMetricCounts
  raw?: Record<string, unknown>
}

export type ApiImportResult = {
  batch: ApiImportBatch
  new_mappings: number
}

export type ApiDataSourceFilter = {
  statuses?: ApiDataSourceStatus[]
  platforms?: ApiPlatform[]
  limit?: number
}

export type ApiImportBatchFilter = {
  dataSourceId?: string
  statuses?: ApiImportStatus[]
  limit?: number
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

// 后端对同一类冲突只回一句「当前状态不允许该操作」，具体原因不在响应体里。
// 把状态码和错误码带出来，页面才有机会按场景说人话；message 保持原样，老的 catch 不受影响。
export class ApiRequestError extends Error {
  readonly status: number
  readonly code: string

  constructor(message: string, status: number, code: string) {
    super(message)
    this.name = 'ApiRequestError'
    this.status = status
    this.code = code
  }
}

async function request<T>(path: string, method = 'GET', body?: unknown): Promise<T> {
  const response = await fetch(`${apiBase}${path}`, {
    method,
    headers: body === undefined ? undefined : { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  const payload = await response.json() as T | { error?: { message?: string; code?: string } }
  if (!response.ok) {
    const error = payload as { error?: { message?: string; code?: string } }
    throw new ApiRequestError(error.error?.message ?? 'API 请求失败', response.status, error.error?.code ?? '')
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
  listReports: (projectId: string, limit = 100) =>
    request<{ items: ApiInsightReport[] }>(`${insightProjectPath(projectId)}/reports?limit=${limit}`),
  confirmReport: (projectId: string, reportId: string, expectedVersion: number) =>
    request<ApiInsightReport>(
      `${insightProjectPath(projectId)}/reports/${encodeURIComponent(reportId)}:confirm`, 'POST',
      { expected_version: expectedVersion },
    ),
  // 从复盘沉淀经验。九字段能填多少填多少：复盘是最有依据的一次，
  // 这里少填一个字段，后面投前洞察里那张卡就永远缺一格。
  createExperienceFromReport: (
    projectId: string,
    reportId: string,
    body: {
      expected_report_version: number
      conclusion: string
      conditions?: string[]
      counterexamples?: string[]
      card_type?: ApiInsightCardType
      confidence?: ApiConfidenceLevel
      recommended_action?: string
      applicability?: ApiApplicability
      data_basis?: ApiDataBasis
      content_basis?: ApiContentBasis
    },
  ) => request<ApiExperience>(
    `${insightProjectPath(projectId)}/reports/${encodeURIComponent(reportId)}:create-experience`, 'POST', body,
  ),
  listExperiences: (projectId: string, status?: ApiExperienceStatus, limit = 100) => {
    const search = new URLSearchParams({ limit: String(limit) })
    if (status) search.set('status', status)
    return request<{ items: ApiExperience[] }>(
      `${insightProjectPath(projectId)}/experiences?${search.toString()}`,
    )
  },
  listExperienceAudits: (projectId: string, experienceId: string, limit = 50) =>
    request<{ items: ApiExperienceAudit[] }>(
      `${insightExperiencePath(projectId, experienceId)}/audits?limit=${limit}`,
    ),
  listExperienceReferences: (projectId: string, experienceId: string, limit = 50) =>
    request<{ items: ApiExperienceReference[] }>(
      `${insightExperiencePath(projectId, experienceId)}/references?limit=${limit}`,
    ),
  listProjectExperienceReferences: (projectId: string, limit = 100) =>
    request<{ items: ApiExperienceReference[] }>(
      `${insightProjectPath(projectId)}/experience-references?limit=${limit}`,
    ),
  listExperienceLineage: (projectId: string, experienceId: string) =>
    request<{ items: ApiExperience[] }>(
      `${insightExperiencePath(projectId, experienceId)}/lineage`,
    ),
  confirmExperience: (projectId: string, experienceId: string, expectedVersion: number) =>
    request<ApiExperience>(
      `${insightExperiencePath(projectId, experienceId)}:confirm`, 'POST',
      { expected_version: expectedVersion },
    ),
  rejectExperience: (projectId: string, experienceId: string, expectedVersion: number, reason: string) =>
    request<ApiExperience>(
      `${insightExperiencePath(projectId, experienceId)}:reject`, 'POST',
      { expected_version: expectedVersion, reason },
    ),
  requestExperienceReview: (projectId: string, experienceId: string, expectedVersion: number, reason: string) =>
    request<ApiExperience>(
      `${insightExperiencePath(projectId, experienceId)}:request-review`, 'POST',
      { expected_version: expectedVersion, reason },
    ),
  retireExperience: (projectId: string, experienceId: string, expectedVersion: number, reason: string) =>
    request<ApiExperience>(
      `${insightExperiencePath(projectId, experienceId)}:retire`, 'POST',
      { expected_version: expectedVersion, reason },
    ),
  // AM-014 的闭环：下游引用了哪条结论、最后是照做还是改了还是没采纳，
  // 记在经验自己身上，而不是只在 Brief 里留一句话。
  recordExperienceReference: (
    projectId: string,
    experienceId: string,
    body: { consumer_kind: string; consumer_id: string; outcome: ApiExperienceReferenceOutcome; note?: string },
  ) => request<ApiExperienceReference>(
    `${insightExperiencePath(projectId, experienceId)}:record-reference`, 'POST', body,
  ),
  // 分析素材库的每个视图都是一次不同的查询，而不是同一批数据换个标签（22 §8.3）。
  listInsightAssets: (projectId: string, filter: ApiInsightAssetFilter = {}) => {
    const search = new URLSearchParams({ limit: String(filter.limit ?? 100) })
    filter.statuses?.forEach(status => search.append('status', status))
    filter.assetTypes?.forEach(assetType => search.append('asset_type', assetType))
    filter.sourceKinds?.forEach(sourceKind => search.append('source_kind', sourceKind))
    if (filter.lineageId) search.set('lineage_id', filter.lineageId)
    return request<{ items: ApiInsightAsset[] }>(
      `${insightProjectPath(projectId)}/assets?${search.toString()}`,
    )
  },
  getInsightAsset: (projectId: string, assetId: string) =>
    request<ApiInsightAsset>(`${insightAssetPath(projectId, assetId)}`),
  listInsightAssetLineage: (projectId: string, assetId: string) =>
    request<{ items: ApiInsightAsset[] }>(`${insightAssetPath(projectId, assetId)}/lineage`),
  listInsightAssetFeatures: (projectId: string, assetId: string) =>
    request<{ items: ApiInsightAssetFeature[] }>(`${insightAssetPath(projectId, assetId)}/features`),
  // 人工结论另起一行写入，不改 AI 那一层，后台再跑也不会盖掉（03 AM-006、§14）。
  patchInsightAssetFeatures: (
    projectId: string,
    assetId: string,
    body: { expected_version: number; features: ApiFeatureInput[]; reason: string },
  ) => request<{ items: ApiInsightAssetFeature[] }>(`${insightAssetPath(projectId, assetId)}/features`, 'PATCH', body),
  // AI 提特征。**只有人点按钮才会调到这里**：登记素材时自动排队会把复核队列
  // 灌满没人要看的结果，而复核是这套东西唯一的质量闸门（03 AM-005）。
  // content 必须由调用方带上——素材库存的是素材的身份和状态，不存正文。
  analyzeInsightAsset: (
    projectId: string,
    assetId: string,
    body: { expected_version: number; content: string; note?: string },
  ) => request<ApiAnalyzeAssetResult>(`${insightAssetPath(projectId, assetId)}:analyze`, 'POST', body),
  // 分析历史。失败的也在里面：只列成功的话，成功率永远是 100%。
  listInsightAssetAnalysisRuns: (projectId: string, assetId: string, limit = 20) =>
    request<{ items: ApiAnalysisRun[] }>(
      `${insightAssetPath(projectId, assetId)}/analysis-runs?limit=${limit}`,
    ),
  identifyInsightAssetType: (
    projectId: string,
    assetId: string,
    body: { expected_version: number; asset_type: ApiInsightAssetType; source: ApiFeatureSource; confidence?: ApiConfidence; reason: string },
  ) => request<ApiInsightAsset>(`${insightAssetPath(projectId, assetId)}:identify-type`, 'POST', body),
  listInsightAssetMappings: (projectId: string, status?: ApiMappingStatus, limit = 100) => {
    const search = new URLSearchParams({ limit: String(limit) })
    if (status) search.set('status', status)
    return request<{ items: ApiInsightAssetMapping[] }>(
      `${insightProjectPath(projectId)}/asset-mappings?${search.toString()}`,
    )
  },
  // 认领：把一个平台对象认到某个素材版本上，或者明确忽略它。认领之后它的花费
  // 才算得到这一版素材头上（doc10 §5）。
  resolveInsightAssetMapping: (
    projectId: string,
    mappingId: string,
    body: { expected_version: number; status: ApiMappingStatus; asset_id?: string; note: string },
  ) => request<ApiInsightAssetMapping>(
    `${insightProjectPath(projectId)}/asset-mappings/${encodeURIComponent(mappingId)}:resolve`, 'POST', body),
  listFeatureSchemas: (projectId: string) =>
    request<{ items: ApiFeatureSchema[] }>(`${insightProjectPath(projectId)}/feature-schemas`),
  getFeatureMatrix: (projectId: string, assetIds: string[]) =>
    request<ApiFeatureMatrix>(
      `${insightProjectPath(projectId)}/feature-matrix?asset_ids=${encodeURIComponent(assetIds.join(','))}`,
    ),
  // 数据接入（doc10）。五个视图各自是一次不同的查询：数据源与字段映射读同一批行
  // 但看不同字段，导入任务与同步记录是同一张表按 kind 过滤（22 §8.3）。
  listDataSources: (projectId: string, filter: ApiDataSourceFilter = {}) => {
    const search = new URLSearchParams({ limit: String(filter.limit ?? 100) })
    filter.statuses?.forEach(status => search.append('status', status))
    filter.platforms?.forEach(platform => search.append('platform', platform))
    return request<{ items: ApiDataSource[] }>(
      `${insightProjectPath(projectId)}/data-sources?${search.toString()}`,
    )
  },
  getDataSource: (projectId: string, dataSourceId: string) =>
    request<ApiDataSource>(insightDataSourcePath(projectId, dataSourceId)),
  registerDataSource: (
    projectId: string,
    body: {
      platform: ApiPlatform
      ingest_mode: ApiIngestMode
      account_label?: string
      account_ref?: string
      credential_ref?: string
      caliber?: ApiMetricCaliber
      field_mapping?: Record<string, string>
    },
  ) => request<ApiDataSource>(`${insightProjectPath(projectId)}/data-sources`, 'POST', body),
  updateDataSource: (
    projectId: string,
    dataSourceId: string,
    body: {
      expected_version: number
      status?: ApiDataSourceStatus
      account_label?: string
      caliber?: ApiMetricCaliber
      field_mapping?: Record<string, string>
    },
  ) => request<ApiDataSource>(insightDataSourcePath(projectId, dataSourceId), 'PATCH', body),
  setDataSourceQuality: (
    projectId: string,
    dataSourceId: string,
    body: { expected_version: number; quality_status: ApiQualityStatus; note?: string },
  ) => request<ApiDataSource>(`${insightDataSourcePath(projectId, dataSourceId)}:set-quality`, 'POST', body),
  listImportBatches: (projectId: string, filter: ApiImportBatchFilter = {}) => {
    const search = new URLSearchParams({ limit: String(filter.limit ?? 100) })
    if (filter.dataSourceId) search.set('data_source_id', filter.dataSourceId)
    filter.statuses?.forEach(status => search.append('status', status))
    return request<{ items: ApiImportBatch[] }>(
      `${insightProjectPath(projectId)}/import-batches?${search.toString()}`,
    )
  },
  // 部分成功也会正常返回：批次建出来了，被拒的行在 batch.errors 里逐条说明。
  importMetrics: (
    projectId: string,
    body: {
      data_source_id: string
      kind: ApiImportKind
      rows: ApiMetricRow[]
      source_label?: string
      content_hash?: string
      corrects_batch_id?: string
      register_objects?: boolean
    },
  ) => request<ApiImportResult>(`${insightProjectPath(projectId)}/import-batches`, 'POST', body),
  // 窗口写在 URL 里而不是让后端偷偷默认，20 §4.1 要求数据窗口必须能被看到。
  getMetricOverview: (projectId: string, start?: string, end?: string) => {
    const search = new URLSearchParams()
    if (start) search.set('start', start)
    if (end) search.set('end', end)
    const query = search.toString()
    return request<ApiMetricOverview>(
      `${insightProjectPath(projectId)}/metric-overview${query ? `?${query}` : ''}`,
    )
  },
  // 和 getMetricOverview 分成两个请求，但五个视图共用这一次返回：
  // 拆开的话「趋势里看到的」和「疲劳里算的」会来自两次读取，对不上时没人解释得清。
  getPerformanceAnalysis: (projectId: string, start?: string, end?: string) => {
    const search = new URLSearchParams()
    if (start) search.set('start', start)
    if (end) search.set('end', end)
    const query = search.toString()
    return request<ApiPerformanceAnalysis>(
      `${insightProjectPath(projectId)}/performance-analysis${query ? `?${query}` : ''}`,
    )
  },
  // cross_channel 只认字面 true：跨渠道比较默认关闭（03 §10.3②），
  // 缺参数、空串、"false" 都是关闭，不做 truthy 判断。
  getPreLaunch: (projectId: string, filter: ApiPreLaunchFilter = {}) => {
    const search = new URLSearchParams()
    if (filter.channel) search.set('channel', filter.channel)
    if (filter.creative_type) search.set('creative_type', filter.creative_type)
    if (filter.objective) search.set('objective', filter.objective)
    if (filter.q) search.set('q', filter.q)
    if (filter.cross_channel) search.set('cross_channel', 'true')
    const query = search.toString()
    return request<ApiPreLaunchInsight>(
      `${insightProjectPath(projectId)}/prelaunch${query ? `?${query}` : ''}`,
    )
  },
  // 六个二级视图共用这一次请求：它们共用同一套排序和队列判断，
  // 拆成六个请求会让「队列里还有几条」在不同视图里算出不同的数。
  getDataQuality: (projectId: string, start?: string, end?: string) => {
    const search = new URLSearchParams()
    if (start) search.set('start', start)
    if (end) search.set('end', end)
    const query = search.toString()
    return request<ApiDataQualityReport>(
      `${insightProjectPath(projectId)}/data-quality${query ? `?${query}` : ''}`,
    )
  },
  // 五个二级视图共用这一次请求：它们算的是同一批素材、特征、数据源和日指标，
  // 拆开会让治理面上「特征数」和「待办数」在不同视图里对不上。
  getCapabilityOperations: (projectId: string, start?: string, end?: string) => {
    const search = new URLSearchParams()
    if (start) search.set('start', start)
    if (end) search.set('end', end)
    const query = search.toString()
    return request<ApiCapabilityOperations>(
      `${insightProjectPath(projectId)}/capability-operations${query ? `?${query}` : ''}`,
    )
  },
  // 系统设置整页只读，所以只有 get 没有 put。这些值不来自数据库，全部是代码常量本身，
  // 每次请求现算——中间隔一层存储，就会有页面和代码对不上的那一天。
  getInsightSettings: (projectId: string) =>
    request<ApiInsightSettings>(`${insightProjectPath(projectId)}/settings`),
  // observed_through 要回传界面上那条问题的 last_observed_at，不要用当前时间：
  // 「你处置的是你看到的那个版本」靠它成立，中间问题若又恶化不会被一并盖掉。
  resolveQualityIssue: (
    projectId: string,
    body: {
      fingerprint: string
      issue_kind: ApiDataQualityIssueKind
      state: ApiDataQualityDispositionState
      note: string
      observed_through: string
    },
  ) => request<ApiDataQualityDisposition>(
    `${insightProjectPath(projectId)}/data-quality/dispositions`, 'POST', body,
  ),
}

// Insights 走 /api/insights/v1；request() 已经带上 /api 前缀。
function insightProjectPath(projectId: string): string {
  return `/insights/v1/projects/${encodeURIComponent(projectId)}`
}

// 动作端点形如 .../experiences/{id}:confirm，冒号是路径的一部分，不参与编码。
function insightExperiencePath(projectId: string, experienceId: string): string {
  return `${insightProjectPath(projectId)}/experiences/${encodeURIComponent(experienceId)}`
}

function insightAssetPath(projectId: string, assetId: string): string {
  return `${insightProjectPath(projectId)}/assets/${encodeURIComponent(assetId)}`
}

function insightDataSourcePath(projectId: string, dataSourceId: string): string {
  return `${insightProjectPath(projectId)}/data-sources/${encodeURIComponent(dataSourceId)}`
}
