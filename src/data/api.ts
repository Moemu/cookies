import {
  attachKanonBriefProductAsset,
  createKanonCommercePrerollVideo,
  createKanonBrief,
  createKanonPreparedCommercePrerollVideo,
  createKanonMedia,
  createKanonProject,
  confirmKanonBrief,
  ensureKanonGuerlainCommerceFixtureAssets,
  getKanonCapabilities,
  getKanonJob,
  listKanonArtifacts,
  listKanonCommercePrerollSources,
  listKanonJobs,
  listKanonProjects,
  listKanonTasks,
  loadKanonAgencyWorkbench,
  prepareKanonCommercePreroll,
  unsupportedKanonWrite,
} from '../backend/kanon-api.js'
import { platformClient } from './platformClient.js'

export type ApiProject = {
  id: string
  name: string
  brand: string
  objective: string
  industry?: 'short_drama' | 'game' | 'ecommerce' | 'automotive_brand'
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

export type ApiPublicInsightIndustryStat = {
  name: string
  count: number
  views: number
}

export type ApiPublicInsightOverview = {
  total_videos: number
  total_views: number
  average_like_rate: number
  average_finish_rate: number
  ai_ratio: number
  industries: ApiPublicInsightIndustryStat[]
  files: Array<{ filename: string; row_count: number; modified_at: string }>
  loaded_at: string
  data_dir: string
}

export type ApiPublicInsightFilterOption = {
  value: string
  count: number
}

export type ApiPublicInsightFilters = {
  industries: ApiPublicInsightFilterOption[]
  visual_styles: ApiPublicInsightFilterOption[]
  ai_types: string[]
  date_range: { min: string; max: string }
}

export type ApiPublicInsightVideoListItem = {
  item_id: string
  url: string
  frame_first: string
  item_title: string
  item_create_day: string
  author_cert_type: string
  vv_all: number
  like_cnt_all: number
  comment_cnt_all: number
  share_cnt_all: number
  favourite_cnt_all: number
  finish_vv_all: number
  ctr: string
  bounce_rate_map: string
  has_ai_generated: string
  industry: string
  date: string
  finish_rate: number
  like_rate: number
  playback_url: string
}

export type ApiPublicInsightVideoDetail = ApiPublicInsightVideoListItem & {
  storyboard_structure: string
  ai_creative_type: string
  item_asr: string
  item_ocr: string
  first3s_visual_creative_type: string
  main_visual_elements: string
  shooting_scene: string
  characters_relation: string
  mentioned_brand: string
  oral_product_desc: string
  bgm_style: string
  bgm_bpm: string
  bgm_emotion: string
  voice_type: string
  speech_speed: string
  oral_script: string
  storyboard_prompt: string
  visual_style: string
  creative_highlight: string
  source_file: string
  storyboard: unknown[]
}

export type ApiPublicInsightVideoPage = {
  items: ApiPublicInsightVideoListItem[]
  total: number
  page: number
  page_size: number
  pages: number
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
  mediaKind?: 'image' | 'video'
  contentUrl?: string
  sourceJobId?: string
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
  briefTaskId?: string
  briefDraftVersion?: number
  version: number
  createdAt: string
  updatedAt: string
}

export type ApiProjectMediaAsset = {
  id: string
  projectId: string
  version: number
  kind: 'video' | 'image' | 'document'
  sourceType?: 'upload' | 'provider_generated' | 'imported' | 'captured' | 'rendered'
  mimeType: string
  sizeBytes: number
  durationSeconds?: number
  width?: number
  height?: number
  createdAt: string
  contentUrl: string
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

export type ApiCommerceTemplateId =
  | 'commerce.product-cut'
  | 'commerce.window-reveal'
  | 'commerce.one-click'
  | 'commerce.miniature'
  | 'commerce.device-summon'

export type ApiCreativeSourceRef = {
  kind: 'confirmed_brief' | 'strategy_package'
  id: string
  version: number
  content_hash: string
}

export type ApiCommerceProductFacts = {
  brand_name: string
  product_name: string
  product_category?: string
  selling_points: string[]
  tone: string[]
  visual_keywords: string[]
  mandatory_elements: string[]
  prohibited_claims: string[]
  product_asset_refs: Array<{ asset_id: string; version: number }>
}

export type ApiCreativeSourceOption = {
  source_ref: ApiCreativeSourceRef
  status: 'confirmed' | 'approved'
  product: ApiCommerceProductFacts
  confirmed_at: string
  preferred: boolean
}

export type ApiPreparedCommercePreroll = {
  contract_version: 'creative-commerce-preroll-preparation/v1'
  source_ref: ApiCreativeSourceRef
  product: ApiCommerceProductFacts
  plan: {
    template: {
      template_id: ApiCommerceTemplateId
      template_version: 1
    }
    frame_plan: {
      start_frame_kind: string
      tail_frame_kind: string
    }
    prompt: {
      fidelity: string
      camera: string
      environment: string
      timeline: Array<{
        start_seconds: number
        end_seconds: number
        purpose: 'information_gap' | 'single_transformation' | 'product_hold'
        instruction: string
      }>
      guardrails: string[]
      compiled_prompt: string
      prompt_hash: string
    }
  }
  readiness: {
    planning_ready: boolean
    generation_ready: boolean
    blockers: string[]
    warnings: string[]
  }
  prepared_at: string
}

export type ApiCommercePrerollWorkspace = {
  task: {
    id: string
    status: 'draft' | 'in_progress' | 'ready_for_review' | 'generating' | 'generated' | 'archived'
    performance_mode: 'commerce_preroll'
    updated_at: string
  }
  intake: {
    id: string
    version: number
    request: {
      objective: string
      audience: string
      core_message: string
      call_to_action: string
      manual_commerce_preroll: {
        fixture_id: string
        fixture_version: number
        fixture_content_hash: string
        brand_name: string
        product_name: string
        product_category?: string
        selling_points: string[]
        visual_keywords: string[]
        product_asset_ref: ApiAssetVersionRef
        first_frame_asset_ref: ApiAssetVersionRef
        last_frame_asset_ref: ApiAssetVersionRef
        template_ref: { template_id: ApiCommerceTemplateId; template_version: 1 }
      }
    }
  }
  video_draft: {
    revision: number
    prompt: string
    commerce_preroll: {
      contract_version: 'creative-commerce-preroll-draft/v1'
      revision: number
      input_hash: string
      input_snapshot: {
        fixture_id: string
        fixture_version: number
        fixture_content_hash: string
        brand_name: string
        product_name: string
        product_category?: string
        selling_points: string[]
        visual_keywords: string[]
        mandatory_elements: string[]
        prohibited_claims: string[]
        product_asset_ref: ApiAssetVersionRef
        first_frame_asset_ref: ApiAssetVersionRef
        last_frame_asset_ref: ApiAssetVersionRef
      }
      plan: {
        template: { template_id: ApiCommerceTemplateId; template_version: 1 }
        frame_plan: {
          start_frame_kind: string
          tail_frame_kind: string
          product_asset_ref: ApiAssetVersionRef
        }
        prompt: {
          prompt_version: number
          fidelity: string
          camera: string
          environment: string
          timeline: Array<{
            start_seconds: number
            end_seconds: number
            purpose: 'information_gap' | 'single_transformation' | 'product_hold'
            instruction: string
          }>
          guardrails: string[]
          compiled_prompt: string
          prompt_hash: string
        }
        generation_spec: {
          generation_spec_hash: string
          duration_seconds: 6
          aspect_ratio: '9:16'
          resolution: '720p'
          generation_ready: boolean
          production_ready: boolean
        }
      }
      approval?: {
        generation_spec_hash: string
        confirmed_by: string
        confirmed_at: string
      }
      readiness: {
        planning_ready: boolean
        generation_ready: boolean
        production_ready: boolean
        missing_fields: string[]
        blockers: string[]
      }
    }
  }
  commerce_preroll_generation_attempts?: Array<{
    id: string
    task_id: string
    draft_revision: number
    template_ref: { template_id: ApiCommerceTemplateId; template_version: 1 }
    prompt_hash: string
    generation_spec_hash: string
    provider_job_id: string
    retry_of_attempt_id?: string
    output_asset_version?: ApiAssetVersionRef
    created_at: string
  }>
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
  executionAngle: 'dialogue_confrontation' | 'action_reveal' | 'reaction_escalation' | 'result_first'
  executionAngleLabel: string
  score: number
  scoreMeaning: 'editorial_quality_heuristic'
  evidence: string[]
  primaryTestVariable: string
  pacingProfile: string
  visualGrammar: string
  variantHypothesis: string
  hookLine: string
  voiceover: string
  storyboard: Array<{ startSeconds: number; endSeconds: number; visual: string; copy: string }>
  visualIntent: string
  transitionLine: string
  promptPackage: {
    compiledPrompt: string
    contentHash: string
    directorSpec: Record<string, string>
    candidateBatchId?: string
    promptCompilerVersion?: string
    generationConfig: ApiShortDramaGenerationConfig
    subtitleSpec: {
      mode: string
      max_lines: number
      safe_area: string
      keyword_emphasis: boolean
      animation_density: string
      contrast_policy: string
    }
  }
}

export type ApiShortDramaPrerollPlan = {
  version: 'short_drama_preroll_v1'
  candidates: ApiShortDramaPrerollCandidate[]
}

export type ApiShortDramaHookStrategy = 'conflict_reversal' | 'suspense_reveal' | 'identity_contrast' | 'selling_point_bridge'
export type ApiShortDramaSubtitleStyle = 'high_contrast_dynamic' | 'brand_minimal'
export type ApiShortDramaPaceProfile = 'auto' | 'punchy' | 'balanced' | 'suspense_hold'
export type ApiShortDramaVariationIntent = 'balanced' | 'more_visual' | 'more_dialogue' | 'more_suspense'

export type ApiShortDramaGenerationConfig = {
  subtitle_style: ApiShortDramaSubtitleStyle
  hook_strength: number
  pace_profile: ApiShortDramaPaceProfile
}

type ApiShortDramaCandidateWire = {
  id: string
  hook_strategy: ApiShortDramaHookStrategy
  execution_angle: ApiShortDramaPrerollCandidate['executionAngle']
  primary_test_variable?: string
  pacing_profile?: string
  visual_grammar?: string
  variant_hypothesis?: string
  score: number
  score_meaning: 'editorial_quality_heuristic'
  evidence: string[]
  hook_line: string
  voiceover: string
  storyboard: Array<{ start_seconds: number; end_seconds: number; visual: string; copy: string }>
  visual_intent: string
  transition_line: string
  prompt_package: {
    compiled_prompt: string
    content_hash: string
    director_spec: Record<string, string>
    candidate_batch_id?: string
    prompt_compiler_version?: string
    generation_config?: ApiShortDramaGenerationConfig
    subtitle_spec?: ApiShortDramaPrerollCandidate['promptPackage']['subtitleSpec']
  }
}

export type ApiShortDramaPrerollWorkspace = {
  task: { id: string; performance_mode: 'short_drama_preroll'; status: string }
  video_draft: {
    revision: number
    short_drama_preroll: {
      revision: number
      selected_candidate_id?: string
      input_snapshot: {
        brief_id: string
        brief_version: number
        brief_name: string
        story_title: string
        synopsis: string
        reviewed_selling_points: string[]
        opening_line?: string
        hook_strategy: ApiShortDramaHookStrategy
        subtitle_style: ApiShortDramaSubtitleStyle
        transition: 'hard_cut' | 'action_match' | 'audio_bridge'
        hook_strength: number
        pace_profile?: ApiShortDramaPaceProfile
        call_to_action: string
      }
      readiness: { planning_ready: boolean; generation_ready: boolean; production_ready: boolean; blockers: string[] }
      active_candidate_batch?: {
        id: string
        revision: number
        planner_version: string
        prompt_compiler_version: string
        diversity_nonce: string
        generation_config: ApiShortDramaGenerationConfig
        variation_intent: ApiShortDramaVariationIntent
        generated_candidate_count: number
        candidates: ApiShortDramaCandidateWire[]
        created_at: string
      }
      candidates: ApiShortDramaCandidateWire[]
    }
  }
  short_drama_generation_attempts?: Array<{
    id: string
    task_id: string
    draft_revision: number
    candidate_batch_id: string
    candidate_id: string
    prompt_package_hash: string
    generation_spec_hash: string
    provider_job_id: string
    output_asset_version?: ApiAssetVersionRef
    created_at: string
  }>
}

export type ApiCreateManualShortDramaPrerollInput = {
  briefId: string
  briefVersion: number
  briefName: string
  title: string
  synopsis: string
  reviewedSellingPoints: string[]
  openingLine?: string
  hookStrategy: ApiShortDramaHookStrategy
  subtitleStyle: ApiShortDramaSubtitleStyle
  transition: 'hard_cut' | 'action_match' | 'audio_bridge'
  hookStrength: number
  paceProfile: ApiShortDramaPaceProfile
  objective: string
  audience: string
  prohibitedClaims: string[]
  callToAction: string
}

export type ApiShortDramaPrerollSnapshot = {
  planVersion: ApiShortDramaPrerollPlan['version']
  storyContext: Omit<ApiShortDramaStoryContext, 'openingLine'>
  selectedCandidate: ApiShortDramaPrerollCandidate
  prompt: string
}

export type ApiGameEvidenceMoment = {
  id: string
  kind: 'skill_choice' | 'wave_progress' | 'battle'
  start_milliseconds: number
  end_milliseconds: number
  description: string
  verified_copy: string[]
}

export type ApiGameHookMechanism =
  | 'choice_challenge'
  | 'tactical_tradeoff'
  | 'wave_escalation'
  | 'failure_reversal'
  | 'merge_upgrade'
  | 'reward_reveal'

export type ApiGamePrerollCandidate = {
  id: string
  hook_mechanism: ApiGameHookMechanism
  execution_angle: string
  primary_test_variable: string
  variant_hypothesis: string
  score: number
  score_meaning: 'evidence_grounded_hook_relevance'
  hook_line: string
  evidence_moment_ids: string[]
  storyboard: Array<{
    start_milliseconds: number
    end_milliseconds: number
    visual: string
    copy: string
    evidence_moment_id: string
  }>
  prompt_package: {
    prompt_compiler_version: string
    input_snapshot_hash: string
    candidate_batch_id: string
    candidate_id: string
    generation_config: {
      subtitle_style: 'high_contrast_dynamic' | 'brand_minimal'
      hook_strength: number
      pace_profile: 'punchy' | 'balanced'
    }
    director_spec: Record<string, string>
    negative_constraints: string[]
    compiled_prompt: string
    content_hash: string
  }
}

export type ApiGamePrerollWorkspace = {
  task: { id: string; performance_mode: 'game_preroll'; status: string }
  video_draft: {
    revision: number
    game_preroll: {
      revision: number
      selected_candidate_id?: string
      input_snapshot: {
        brief_id: string
        brief_version: number
        brief_name: string
        game_name: string
        gameplay_summary: string
        source_video: ApiAssetVersionRef
        source_video_rights: 'confirmed'
        call_to_action: string
        evidence_moments: ApiGameEvidenceMoment[]
        allowed_mechanisms: ApiGameHookMechanism[]
        prohibited_mechanisms: ApiGameHookMechanism[]
      }
      readiness: {
        planning_ready: boolean
        generation_ready: boolean
        production_ready: boolean
        blockers: string[]
      }
      active_candidate_batch: {
        id: string
        revision: number
        planner_version: string
        prompt_compiler_version: string
        generation_config: {
          subtitle_style: 'high_contrast_dynamic' | 'brand_minimal'
          hook_strength: number
          pace_profile: 'punchy' | 'balanced'
        }
        generated_candidate_count: number
        candidates: ApiGamePrerollCandidate[]
        created_at: string
      }
      candidates: ApiGamePrerollCandidate[]
    }
  }
  game_preroll_generation_attempts?: Array<{
    id: string
    task_id: string
    draft_revision: number
    candidate_batch_id: string
    candidate_id: string
    prompt_package_hash: string
    generation_spec_hash: string
    provider_job_id: string
    output_asset_version?: ApiAssetVersionRef
    created_at: string
  }>
}

export type ApiCreateManualGamePrerollInput = {
  briefId: string
  briefVersion: number
  briefName: string
  gameName: string
  gameplaySummary: string
  sourceVideo: ApiAssetVersionRef
  evidenceMoments: ApiGameEvidenceMoment[]
  allowedMechanisms: ApiGameHookMechanism[]
  prohibitedMechanisms: ApiGameHookMechanism[]
  subtitleStyle: 'high_contrast_dynamic' | 'brand_minimal'
  hookStrength: number
  paceProfile: 'punchy' | 'balanced'
  objective: string
  audience: string
  coreMessage: string
  callToAction: string
  mandatoryElements: string[]
  prohibitedClaims: string[]
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
  entityType: 'project' | 'business_task' | 'artifact' | 'generation_job' | 'change_set' | 'operational_record'
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
  fields: Record<string, unknown>
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

export type ApiViralRemakeWorkspace = {
  task: {
    id: string
    performance_mode: 'viral_remake'
    status: string
  }
  intake: {
    id: string
    request: {
      call_to_action: string
      manual_viral_remake: {
        product_name: string
        selling_points: string[]
        user_instruction: string
        reference_video: ApiAssetVersionRef
        reference_image?: ApiAssetVersionRef
        reference_video_rights: 'pending' | 'confirmed'
        reference_image_rights?: 'pending' | 'confirmed'
      }
    }
  }
  video_draft: {
    revision: number
    viral_remake: {
      revision: number
      status:
        | 'waiting_for_analysis'
        | 'analysis_ready'
        | 'generation_ready'
        | 'generating'
        | 'candidate_ready'
        | 'provider_failed'
        | 'ready_for_review'
      selected_route_id: 'route_manual_viral_remake_v1'
      input_snapshot: {
        reference_video: ApiAssetVersionRef
        reference_image?: ApiAssetVersionRef
        product_name: string
        selling_points: string[]
        call_to_action: string
        user_instruction: string
        reference_video_rights: 'pending' | 'confirmed'
        reference_image_rights?: 'pending' | 'confirmed'
      }
      readiness: {
        planning_ready: boolean
        generation_ready: boolean
        production_ready: boolean
        missing_fields: string[]
        blockers: string[]
      }
      analysis_snapshot?: {
        contract_version: 'creative-viral-analysis-snapshot/v1'
        task_id: string
        source_asset_ref: ApiAssetVersionRef
        dimensions: Array<{
          id: ApiVideoPromptDimension['id']
          prompt: string
          evidence_refs: string[]
          confidence: number
          source: 'ai_extracted'
        }>
        preserve_rules: string[]
        replace_rules: string[]
        transcript?: string
        confidence: number
        evidence_refs: string[]
        model_lineage: {
          model_alias: string
          route_revision_id: string
          prompt_version: string
        }
        content_hash: string
        created_at: string
      }
      prompt_draft?: {
        revision: number
        dimensions: Record<ApiVideoPromptDimension['id'], string>
        composite_prompt: string
        updated_at: string
      }
      prompt_package?: {
        contract_version: 'creative-viral-prompt-package/v1'
        prompt_version: number
        content_hash: string
        composite_prompt: string
        generation_spec: {
          model_alias: string
          duration_seconds: number
          aspect_ratio: string
          resolution: string
          candidate_count: number
        }
        confirmed_by: string
        confirmed_at: string
      }
      candidates: Array<{
        id: string
        provider_job_id: string
        prompt_hash: string
        status: 'queued' | 'running' | 'succeeded' | 'failed' | 'reviewed'
        output_asset_ref?: ApiAssetVersionRef
        checks: Array<{ code: string; passed: boolean; message: string }>
        error_code?: string
        error_message?: string
        created_at: string
        updated_at: string
      }>
    }
  }
  production_jobs: Array<{ provider_job_id: string; kind: string }>
}

export type ApiCreateManualViralRemakeInput = {
  sourceVideo: ApiAssetVersionRef
  referenceImage?: ApiAssetVersionRef
  productName: string
  sellingPoints: string[]
  callToAction: string
  userInstruction: string
  objective: string
  audience: string
  coreMessage: string
  durationSeconds: number
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
  credential?: {
    source?: 'environment' | 'workspace'
    maskedApiKey?: string
    updatedAt?: string
  }
  checkedAt: string
}

export type ApiAuthSession = {
  authenticated: boolean
  user?: {
    id: string
    email?: string
    displayName: string
  }
  organization?: {
    id: string
    name: string
    status: string
  }
  membership?: {
    role: 'owner' | 'admin' | 'member' | 'auditor'
    status: string
    updatedAt: string
  }
  scopes?: string[]
}

type PlatformActor = {
  organization_id: string
  principal: { kind: 'user' | 'service'; id: string }
  scopes: string[]
}

type PlatformRequestContext = {
  actor: PlatformActor
}

type PlatformLoginResult = PlatformRequestContext & {
  session_id: string
}

export type ApiProviderConfiguration = {
  provider: 'ark'
  status: 'configured' | 'not_configured'
  baseUrl: string
  source?: 'environment' | 'workspace'
  maskedApiKey?: string
  updatedAt?: string
  capabilities: ApiProviderCapabilities
}

const viteEnv = (import.meta as unknown as { env?: { VITE_API_BASE_URL?: string } }).env
const backendOrigin = viteEnv?.VITE_API_BASE_URL ?? ''
const apiBase = `${backendOrigin}/api`
const platformBase = `${backendOrigin}/platform/v1`

type AgencyWorkbenchOptions = {
  projectIds?: string[]
  includeDemoProject?: boolean
}

const emptyAgencyWorkbench: ApiAgencyWorkbench = {
  organizations: [],
  clients: [],
  brands: [],
  projects: [],
  adAccountBindings: [],
  qualityCheckRuns: [],
  materialConfirmations: [],
  assetVersionPointers: [],
}

async function loadPersistedAgencyWorkbench(projectIds: string[]): Promise<ApiAgencyWorkbench> {
  const results = await Promise.all([...new Set(projectIds)].map(async projectId => {
    const [snapshot, workbench, mediaAssets] = await Promise.all([
      platformClient.getProjectSnapshot(projectId),
      platformClient.getWorkbench(projectId),
      platformClient.listProjectMediaAssets(projectId),
    ])
    return workbench ? workbenchFromResponse(snapshot.project, workbench, mediaAssets) : emptyAgencyWorkbench
  }))
  return results.reduce<ApiAgencyWorkbench>((all, current) => ({
    organizations: [...all.organizations, ...current.organizations], clients: [...all.clients, ...current.clients], brands: [...all.brands, ...current.brands], projects: [...all.projects, ...current.projects], adAccountBindings: [...all.adAccountBindings, ...current.adAccountBindings], qualityCheckRuns: [...all.qualityCheckRuns, ...current.qualityCheckRuns], materialConfirmations: [...all.materialConfirmations, ...current.materialConfirmations], assetVersionPointers: [...all.assetVersionPointers, ...current.assetVersionPointers],
  }), emptyAgencyWorkbench)
}

function workbenchFromResponse(
  project: ApiProject,
  response: import('./platformClient.js').PlatformProjectWorkbench,
  mediaAssets: ApiProjectMediaAsset[],
): ApiAgencyWorkbench {
  const { organization, client, brand, project: progress } = response
  const mediaByAssetVersion = new Map(mediaAssets.map(asset => [`${asset.id}:${asset.version}`, asset]))
  return {
    organizations: [{ id: organization.id, code: organization.code, name: organization.name, owner: organization.owner, currency: organization.currency as 'CNY', timezone: organization.timezone as 'Asia/Shanghai', updatedAt: organization.updated_at }],
    clients: [{ id: client.id, organizationId: client.organization_id, code: client.code, name: client.name, industry: client.industry, owner: client.owner, healthStatus: client.health_status as ApiAgencyHealthStatus, updatedAt: client.updated_at }],
    brands: [{ id: brand.id, organizationId: brand.organization_id, clientId: brand.client_id, code: brand.code, name: brand.name, category: brand.category, productLines: brand.product_lines, owner: brand.owner, guidelineStatus: brand.guideline_status as ApiBrand['guidelineStatus'], updatedAt: brand.updated_at }],
    projects: [{ ...project, organizationId: progress.organization_id, clientId: progress.client_id, brandId: progress.brand_id, progressDetail: { stage: progress.stage as ApiProjectProgressStage, stageLabel: progress.stage_label, stagePercent: progress.stage_percent, taskPercent: progress.task_percent, riskStatus: progress.risk_status as ApiAgencyHealthStatus, blocker: progress.blocker || undefined, updatedAt: progress.updated_at } }],
    adAccountBindings: response.ad_account_bindings.map(item => ({ id: item.id, organizationId: item.organization_id, clientId: item.client_id, brandId: item.brand_id, projectIds: [project.id], platform: item.platform as ApiAdPlatform, accountName: item.account_name, accountDisplayId: item.account_display_id, currency: item.currency as 'CNY', timezone: item.timezone as 'Asia/Shanghai', permissionStatus: item.permission_status as ApiBindingHealthStatus, loginStatus: item.login_status as ApiBindingHealthStatus, trackingStatus: item.tracking_status as ApiBindingHealthStatus, owner: item.owner, boundAssetIds: item.bound_asset_ids, lastSyncedAt: item.last_synced_at })),
    qualityCheckRuns: response.quality_check_runs.map(item => ({ id: item.id, organizationId: item.organization_id, projectId: item.project_id, assetId: item.asset_id, assetVersion: item.asset_version, status: item.status as ApiQualityCheckStatus, model: item.model, ruleVersion: item.rule_version, promptVersion: item.prompt_version, summary: item.summary, issues: item.issues.map(issue => ({ id: issue.id, severity: issue.severity as ApiQualityIssueSeverity, rule: issue.rule, evidence: issue.evidence, suggestion: issue.suggestion })), createdAt: item.created_at, completedAt: item.completed_at ?? undefined })),
    materialConfirmations: response.material_confirmations.map(item => ({ id: item.id, organizationId: item.organization_id, projectId: item.project_id, qualityCheckRunId: item.quality_check_run_id, assetId: item.asset_id, assetVersion: item.asset_version, status: item.status as ApiMaterialConfirmationStatus, scope: item.scope, confirmedBy: item.confirmed_by, note: item.note, createdAt: item.created_at })),
    assetVersionPointers: response.asset_version_pointers.map(item => {
      const media = mediaByAssetVersion.get(`${item.asset_id}:${item.working_version}`)
      return {
        id: item.id,
        organizationId: item.organization_id,
        projectId: item.project_id,
        assetId: item.asset_id,
        mediaKind: media?.kind === 'document' ? undefined : media?.kind,
        contentUrl: media?.contentUrl,
        workingVersion: item.working_version,
        qualityCheckedVersion: item.quality_checked_version ?? undefined,
        humanConfirmedVersion: item.human_confirmed_version ?? undefined,
        deliveryVersion: item.delivery_version ?? undefined,
        versions: item.versions.map(version => ({
          version: version.version,
          createdBy: version.created_by,
          sourceTaskId: version.source_task_id,
          sourceType: version.source_type as ApiAssetVersionRecord['sourceType'],
          sourceLabel: version.source_label,
          createdAt: version.created_at,
          changeSummary: version.change_summary,
        })),
        authorization: {
          platforms: item.authorization.platforms as ApiAdPlatform[],
          regions: item.authorization.regions,
          rightsHolder: item.authorization.rights_holder,
          expiresAt: item.authorization.expires_at,
          note: item.authorization.note,
        },
        deliveryTarget: {
          platform: item.delivery_target.platform as ApiAdPlatform,
          region: item.delivery_target.region,
        },
        owner: item.owner,
        updatedAt: item.updated_at,
      }
    }),
  }
}

async function request<T>(path: string, method = 'GET', body?: unknown): Promise<T> {
  const response = await fetch(`${apiBase}${path}`, {
    method,
    credentials: 'include',
    headers: body === undefined ? undefined : { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  const payloadText = await response.text()
  const payload = payloadText ? JSON.parse(payloadText) as T | { error?: { message?: string } } : undefined
  if (!response.ok) {
    const error = (payload ?? {}) as { error?: { message?: string } }
    throw new Error(error.error?.message ?? 'API 请求失败')
  }
  return payload as T
}

function authSessionFromActor(actor: PlatformActor, username?: string): ApiAuthSession {
  const identity = username?.trim() || actor.principal.id
  return {
    authenticated: true,
    user: { id: actor.principal.id, email: '', displayName: identity },
  }
}

async function platformRequest<T>(path: string, method = 'GET', body?: unknown, headers?: Record<string, string>): Promise<T> {
  const response = await fetch(`${platformBase}${path}`, {
    method,
    credentials: 'include',
    headers: {
      ...(body === undefined ? {} : { 'Content-Type': 'application/json' }),
      ...(headers ?? {}),
    },
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  const payloadText = await response.text()
  const payload = payloadText ? JSON.parse(payloadText) as T | { error?: { message?: string } } : undefined
  if (!response.ok) {
    const error = (payload ?? {}) as { error?: { message?: string } }
    throw new Error(error.error?.message ?? '平台 API 请求失败')
  }
  return payload as T
}

class CreativeApiError extends Error {
  constructor(message: string, readonly status: number) {
    super(message)
    this.name = 'CreativeApiError'
  }
}

async function creativeRequest<T>(path: string, method = 'GET', body?: unknown, headers?: Record<string, string>): Promise<T> {
  const response = await fetch(`${backendOrigin}/api/creative/v1${path}`, {
    method,
    headers: {
      ...(body === undefined ? {} : { 'Content-Type': 'application/json' }),
      ...(headers ?? {}),
    },
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  const responseText = await response.text()
  let payload: T | { error?: { message?: string; request_id?: string } }
  try {
    payload = responseText ? JSON.parse(responseText) as T | { error?: { message?: string; request_id?: string } } : {}
  } catch {
    throw new Error(`Creative API 返回了无法解析的响应（HTTP ${response.status}）`)
  }
  if (!response.ok) {
    const error = payload as { error?: { message?: string; request_id?: string } }
    const requestId = error.error?.request_id ? `（request_id: ${error.error.request_id}）` : ''
    throw new CreativeApiError(`${error.error?.message ?? `Creative API 请求失败（HTTP ${response.status}）`}${requestId}`, response.status)
  }
  return payload as T
}

async function putUploadedAsset(url: string, headers: Record<string, string>, file: File) {
  const requestHeaders = new Headers()
  for (const [name, value] of Object.entries(headers)) {
    const normalized = name.toLowerCase()
    if (normalized !== 'host' && normalized !== 'content-length') requestHeaders.set(name, value)
  }
  if (!requestHeaders.has('Content-Type')) requestHeaders.set('Content-Type', file.type)
  const target = url.startsWith('/') ? `${backendOrigin}${url}` : url
  const response = await fetch(target, { method: 'PUT', headers: requestHeaders, body: file })
  if (!response.ok) throw new Error(`素材上传失败（HTTP ${response.status}）`)
}

async function uploadProjectAsset(projectId: string, file: File): Promise<ApiAssetVersionRef> {
  const path = `/projects/${encodeURIComponent(projectId)}/assets/uploads`
  const created = await platformRequest<{
    session: { id: string; project_asset_ref: null | { asset_version: ApiAssetVersionRef } }
    upload: null | { url: string; method: 'PUT'; headers: Record<string, string> }
  }>(path, 'POST', {
    filename: file.name,
    declared_mime_type: file.type,
    declared_size_bytes: file.size,
    declared_sha256: null,
  }, { 'Idempotency-Key': `viral-upload-${Date.now()}-${Math.random().toString(36).slice(2)}` })
  const existing = created.session.project_asset_ref?.asset_version
  if (existing) return existing
  if (!created.upload) throw new Error('素材上传会话没有返回可用的上传地址。')
  await putUploadedAsset(created.upload.url, created.upload.headers, file)
  const completed = await platformRequest<{
    project_asset_ref: null | { asset_version: ApiAssetVersionRef }
  }>(`${path}/${encodeURIComponent(created.session.id)}:finalize`, 'POST')
  const result = completed.project_asset_ref?.asset_version
  if (!result) throw new Error('素材已经上传，但没有生成可用的 AssetVersionRef。')
  return result
}

async function createManualViralRemakeWorkspace(
  projectId: string,
  input: ApiCreateManualViralRemakeInput,
): Promise<ApiViralRemakeWorkspace> {
  const duration = Math.min(60, Math.max(4, Math.round(input.durationSeconds)))
  const intake = await creativeRequest<{ id: string }>(
    `/projects/${encodeURIComponent(projectId)}/creative-intakes`,
    'POST',
    {
      source: 'manual',
      format: 'video',
      performance_mode: 'viral_remake',
      channel: 'douyin',
      objective: input.objective,
      audience: input.audience,
      core_message: input.coreMessage,
      call_to_action: input.callToAction,
      concept: input.userInstruction,
      tone: ['清晰', '高节奏'],
      visual_keywords: ['高停留开场', '产品证明', '原创表达'],
      mandatory_elements: [],
      prohibited_claims: ['不得复用原片人物、商标、字幕、音乐或逐字台词'],
      creative_routes: [{
        route_id: 'route_manual_viral_remake_v1',
        route_type: 'viral_remake',
        video_purpose: 'performance',
        channels: ['douyin'],
        reason: '用户在 Creative 爆款复刻工作区明确选择该路线',
        target_duration_seconds: duration,
        aspect_ratio: '9:16',
        source_asset_refs: [input.sourceVideo, ...(input.referenceImage ? [input.referenceImage] : [])],
        evidence_refs: [],
        requires_human_confirmation: true,
      }],
      manual_viral_remake: {
        product_name: input.productName,
        selling_points: input.sellingPoints.filter(Boolean),
        user_instruction: input.userInstruction,
        reference_video: input.sourceVideo,
        reference_image: input.referenceImage,
        reference_video_rights: 'pending',
        reference_image_rights: input.referenceImage ? 'pending' : undefined,
      },
    },
    { 'Idempotency-Key': `manual-viral-${Date.now()}-${Math.random().toString(36).slice(2)}` },
  )
  const task = await creativeRequest<{ id: string }>(
    `/projects/${encodeURIComponent(projectId)}/creative-intakes/${encodeURIComponent(intake.id)}:create-video-task`,
    'POST',
    {
      selected_route_id: 'route_manual_viral_remake_v1',
      channel: 'douyin',
      source_video: input.sourceVideo,
      concept: input.userInstruction,
      prompt: '等待 Phase 2 视频理解后编译五维提示词',
      call_to_action: input.callToAction,
      mandatory_elements: [],
      prohibited_claims: ['不得复制原片受保护表达'],
      confirm_route: true,
    },
  )
  return creativeRequest<ApiViralRemakeWorkspace>(
    `/projects/${encodeURIComponent(projectId)}/creative-tasks/${encodeURIComponent(task.id)}/viral-remake`,
  )
}

async function createManualShortDramaPrerollWorkspace(
  projectId: string,
  input: ApiCreateManualShortDramaPrerollInput,
): Promise<ApiShortDramaPrerollWorkspace> {
  const intake = await creativeRequest<{ id: string }>(
    `/projects/${encodeURIComponent(projectId)}/creative-intakes`,
    'POST',
    {
      source: 'manual', format: 'video', performance_mode: 'short_drama_preroll', channel: 'douyin',
      objective: input.objective, audience: input.audience,
      core_message: input.reviewedSellingPoints.filter(Boolean).join('；'), call_to_action: input.callToAction,
      concept: '短剧导流广告前贴', tone: ['紧凑', '悬念'], visual_keywords: ['人物连续', '高对比字幕', 'CTA 收束'],
      mandatory_elements: [], prohibited_claims: input.prohibitedClaims,
      creative_routes: [{
        route_id: 'route_manual_short_drama_preroll_v1', route_type: 'short_drama_preroll', video_purpose: 'performance',
        channels: ['douyin'], reason: '用户在短剧前贴工作区明确选择本地预置 Brief', target_duration_seconds: 6,
        aspect_ratio: '9:16', source_asset_refs: [], evidence_refs: [], requires_human_confirmation: true,
      }],
      manual_short_drama_preroll: {
        brief_id: input.briefId, brief_version: input.briefVersion, brief_name: input.briefName,
        story_title: input.title, synopsis: input.synopsis, reviewed_selling_points: input.reviewedSellingPoints.filter(Boolean),
        opening_line: input.openingLine || undefined, hook_strategy: input.hookStrategy, subtitle_style: input.subtitleStyle,
        transition: input.transition, hook_strength: input.hookStrength, pace_profile: input.paceProfile, character_references: [],
      },
    },
    { 'Idempotency-Key': `manual-short-drama-${Date.now()}-${Math.random().toString(36).slice(2)}` },
  )
  const task = await creativeRequest<{ id: string }>(
    `/projects/${encodeURIComponent(projectId)}/creative-intakes/${encodeURIComponent(intake.id)}:create-video-task`,
    'POST',
    {
      selected_route_id: 'route_manual_short_drama_preroll_v1', channel: 'douyin',
      concept: '短剧导流广告前贴', prompt: '等待人工选择短剧候选后由服务端编译 PromptPackage',
      call_to_action: input.callToAction, mandatory_elements: [], prohibited_claims: ['不得虚构未确认剧情事实'], confirm_route: true,
    },
  )
  return creativeRequest<ApiShortDramaPrerollWorkspace>(
    `/projects/${encodeURIComponent(projectId)}/creative-tasks/${encodeURIComponent(task.id)}/short-drama-preroll`,
  )
}

async function selectShortDramaPrerollCandidate(
  projectId: string,
  taskId: string,
  expectedRevision: number,
  candidateId: string,
): Promise<ApiShortDramaPrerollWorkspace> {
  return creativeRequest<ApiShortDramaPrerollWorkspace>(
    `/projects/${encodeURIComponent(projectId)}/creative-tasks/${encodeURIComponent(taskId)}/short-drama-preroll:select-candidate`,
    'POST',
    { expected_revision: expectedRevision, candidate_id: candidateId },
    { 'Idempotency-Key': `short-drama-select-${taskId}-${expectedRevision}-${candidateId}` },
  )
}

async function regenerateShortDramaPrerollCandidates(
  projectId: string,
  taskId: string,
  expectedRevision: number,
  generationConfig: ApiShortDramaGenerationConfig,
  variationIntent: ApiShortDramaVariationIntent = 'balanced',
): Promise<ApiShortDramaPrerollWorkspace> {
  return creativeRequest<ApiShortDramaPrerollWorkspace>(
    `/projects/${encodeURIComponent(projectId)}/creative-tasks/${encodeURIComponent(taskId)}/short-drama-preroll:regenerate-candidates`,
    'POST',
    {
      expected_revision: expectedRevision,
      generation_config: generationConfig,
      variation_intent: variationIntent,
    },
    {
      'Idempotency-Key': [
        'short-drama-regenerate',
        taskId,
        expectedRevision,
        generationConfig.subtitle_style,
        generationConfig.hook_strength,
        generationConfig.pace_profile,
        variationIntent,
      ].join('-'),
    },
  )
}

async function createShortDramaPrerollVideoJob(
  projectId: string,
  taskId: string,
  draftRevision: number,
  candidateId: string,
): Promise<ApiGenerationJob> {
  const job = await creativeRequest<ApiProviderJobWire>(
    `/projects/${encodeURIComponent(projectId)}/creative-tasks/${encodeURIComponent(taskId)}:video-job`,
    'POST',
    { model_alias: 'cookies.video.standard' },
    { 'Idempotency-Key': `short-drama-video-${taskId}-${draftRevision}-${candidateId}` },
  )
  return mapViralProviderJob(job)
}

async function getShortDramaPrerollVideoJob(projectId: string, jobId: string): Promise<ApiGenerationJob> {
  const job = await platformRequest<ApiProviderJobWire>(
    `/projects/${encodeURIComponent(projectId)}/model/jobs/${encodeURIComponent(jobId)}`,
  )
  return mapViralProviderJob(job)
}

async function getLatestShortDramaPrerollWorkspace(projectId: string): Promise<ApiShortDramaPrerollWorkspace | null> {
  try {
    return await creativeRequest<ApiShortDramaPrerollWorkspace>(
      `/projects/${encodeURIComponent(projectId)}/creative-workspaces/short-drama-preroll`,
    )
  } catch (cause) {
    if (cause instanceof CreativeApiError && cause.status === 404) return null
    throw cause
  }
}

function commerceRequestFingerprint(value: unknown) {
  const text = JSON.stringify(value)
  let hash = 2166136261
  for (let index = 0; index < text.length; index += 1) {
    hash ^= text.charCodeAt(index)
    hash = Math.imul(hash, 16777619)
  }
  return (hash >>> 0).toString(36)
}

async function ensureCommercePrerollFixtureWorkspace(
  projectId: string,
): Promise<ApiCommercePrerollWorkspace> {
  const assets = await ensureKanonGuerlainCommerceFixtureAssets(projectId)
  return creativeRequest<ApiCommercePrerollWorkspace>(
    `/projects/${encodeURIComponent(projectId)}/creative-workspaces/commerce-preroll:ensure-fixture`,
    'POST',
    {
      template_ref: {
        template_id: 'commerce.window-reveal',
        template_version: 1,
      },
      product_asset_ref: assets.productAsset,
      first_frame_asset_ref: assets.firstFrame,
      last_frame_asset_ref: assets.lastFrame,
    },
    { 'Idempotency-Key': `commerce-fixture-${projectId}-guerlain-v1` },
  )
}

async function getLatestCommercePrerollWorkspace(
  projectId: string,
): Promise<ApiCommercePrerollWorkspace | null> {
  try {
    return await creativeRequest<ApiCommercePrerollWorkspace>(
      `/projects/${encodeURIComponent(projectId)}/creative-workspaces/commerce-preroll`,
    )
  } catch (cause) {
    if (cause instanceof CreativeApiError && cause.status === 404) return null
    throw cause
  }
}

async function updateCommercePrerollDraft(
  projectId: string,
  taskId: string,
  input: {
    expected_revision: number
    template_ref: { template_id: ApiCommerceTemplateId; template_version: 1 }
    fidelity?: string
    camera?: string
    motion?: string
    environment?: string
    result?: string
    guardrails?: string[]
  },
): Promise<ApiCommercePrerollWorkspace> {
  return creativeRequest<ApiCommercePrerollWorkspace>(
    `/projects/${encodeURIComponent(projectId)}/creative-tasks/${encodeURIComponent(taskId)}/commerce-preroll-draft`,
    'PATCH',
    input,
    {
      'Idempotency-Key': [
        'commerce-draft',
        taskId,
        input.expected_revision,
        commerceRequestFingerprint(input),
      ].join('-'),
    },
  )
}

async function confirmCommercePrerollGeneration(
  projectId: string,
  taskId: string,
  expectedRevision: number,
): Promise<ApiCommercePrerollWorkspace> {
  return creativeRequest<ApiCommercePrerollWorkspace>(
    `/projects/${encodeURIComponent(projectId)}/creative-tasks/${encodeURIComponent(taskId)}/commerce-preroll:confirm-generation`,
    'POST',
    { expected_revision: expectedRevision },
    { 'Idempotency-Key': `commerce-confirm-${taskId}-${expectedRevision}` },
  )
}

async function createCommercePrerollWorkspaceVideoJob(
  projectId: string,
  workspace: ApiCommercePrerollWorkspace,
): Promise<ApiGenerationJob> {
  const taskId = workspace.task.id
  const draft = workspace.video_draft.commerce_preroll
  const attemptOrdinal = (workspace.commerce_preroll_generation_attempts?.length ?? 0) + 1
  const job = await creativeRequest<ApiProviderJobWire>(
    `/projects/${encodeURIComponent(projectId)}/creative-tasks/${encodeURIComponent(taskId)}:video-job`,
    'POST',
    { model_alias: 'cookies.video.standard' },
    {
      'Idempotency-Key': [
        'commerce-video',
        taskId,
        draft.revision,
        attemptOrdinal,
        draft.plan.generation_spec.generation_spec_hash.slice(-12),
      ].join('-'),
    },
  )
  return mapViralProviderJob(job)
}

async function createManualGamePrerollWorkspace(
  projectId: string,
  input: ApiCreateManualGamePrerollInput,
): Promise<ApiGamePrerollWorkspace> {
  const intake = await creativeRequest<{ id: string }>(
    `/projects/${encodeURIComponent(projectId)}/creative-intakes`,
    'POST',
    {
      source: 'manual',
      format: 'video',
      performance_mode: 'game_preroll',
      channel: 'douyin',
      objective: input.objective,
      audience: input.audience,
      core_message: input.coreMessage,
      call_to_action: input.callToAction,
      concept: input.briefName,
      tone: ['紧张', '清晰', '真实玩法'],
      visual_keywords: ['技能三选一', '波次推进', '竖屏塔防'],
      mandatory_elements: input.mandatoryElements,
      prohibited_claims: input.prohibitedClaims,
      creative_routes: [{
        route_id: 'route_manual_game_preroll_v1',
        route_type: 'game_preroll',
        video_purpose: 'performance',
        channels: ['douyin'],
        reason: '用户确认使用《保卫向日葵》授权实录固定样例跑通游戏前贴',
        target_duration_seconds: 6,
        aspect_ratio: '9:16',
        source_asset_refs: [input.sourceVideo],
        evidence_refs: input.evidenceMoments.map(moment => moment.id),
        requires_human_confirmation: true,
      }],
      manual_game_preroll: {
        brief_id: input.briefId,
        brief_version: input.briefVersion,
        brief_name: input.briefName,
        game_name: input.gameName,
        gameplay_summary: input.gameplaySummary,
        source_video: input.sourceVideo,
        source_video_rights: 'confirmed',
        evidence_moments: input.evidenceMoments,
        allowed_mechanisms: input.allowedMechanisms,
        prohibited_mechanisms: input.prohibitedMechanisms,
        subtitle_style: input.subtitleStyle,
        hook_strength: input.hookStrength,
        pace_profile: input.paceProfile,
      },
    },
    { 'Idempotency-Key': `manual-game-preroll-${Date.now()}-${Math.random().toString(36).slice(2)}` },
  )
  const task = await creativeRequest<{ id: string }>(
    `/projects/${encodeURIComponent(projectId)}/creative-intakes/${encodeURIComponent(intake.id)}:create-video-task`,
    'POST',
    {
      selected_route_id: 'route_manual_game_preroll_v1',
      channel: 'douyin',
      source_video: input.sourceVideo,
      concept: input.briefName,
      prompt: '等待人工选择游戏前贴候选后由服务端编译 PromptPackage',
      call_to_action: input.callToAction,
      mandatory_elements: input.mandatoryElements,
      prohibited_claims: input.prohibitedClaims,
      confirm_route: true,
    },
  )
  return creativeRequest<ApiGamePrerollWorkspace>(
    `/projects/${encodeURIComponent(projectId)}/creative-tasks/${encodeURIComponent(task.id)}`,
  )
}

async function selectGamePrerollCandidate(
  projectId: string,
  taskId: string,
  expectedRevision: number,
  candidateId: string,
): Promise<ApiGamePrerollWorkspace> {
  return creativeRequest<ApiGamePrerollWorkspace>(
    `/projects/${encodeURIComponent(projectId)}/creative-tasks/${encodeURIComponent(taskId)}/game-preroll:select-candidate`,
    'POST',
    { expected_revision: expectedRevision, candidate_id: candidateId },
    { 'Idempotency-Key': `game-preroll-select-${taskId}-${expectedRevision}-${candidateId}` },
  )
}

async function regenerateGamePrerollCandidates(
  projectId: string,
  taskId: string,
  expectedRevision: number,
): Promise<ApiGamePrerollWorkspace> {
  return creativeRequest<ApiGamePrerollWorkspace>(
    `/projects/${encodeURIComponent(projectId)}/creative-tasks/${encodeURIComponent(taskId)}/game-preroll:regenerate-candidates`,
    'POST',
    {
      expected_revision: expectedRevision,
      generation_config: {
        subtitle_style: 'high_contrast_dynamic',
        hook_strength: 4,
        pace_profile: 'punchy',
      },
    },
    { 'Idempotency-Key': `game-preroll-regenerate-${taskId}-${expectedRevision}` },
  )
}

async function createGamePrerollVideoJob(
  projectId: string,
  taskId: string,
  draftRevision: number,
  candidateId: string,
): Promise<ApiGenerationJob> {
  const job = await creativeRequest<ApiProviderJobWire>(
    `/projects/${encodeURIComponent(projectId)}/creative-tasks/${encodeURIComponent(taskId)}:video-job`,
    'POST',
    { model_alias: 'cookies.video.standard' },
    { 'Idempotency-Key': `game-preroll-video-${taskId}-${draftRevision}-${candidateId}` },
  )
  return mapViralProviderJob(job)
}

async function getLatestGamePrerollWorkspace(projectId: string): Promise<ApiGamePrerollWorkspace | null> {
  try {
    return await creativeRequest<ApiGamePrerollWorkspace>(
      `/projects/${encodeURIComponent(projectId)}/creative-workspaces/game-preroll`,
    )
  } catch (cause) {
    if (cause instanceof CreativeApiError && cause.status === 404) return null
    throw cause
  }
}

async function getGamePrerollVideoJob(projectId: string, jobId: string): Promise<ApiGenerationJob> {
  const job = await platformRequest<ApiProviderJobWire>(
    `/projects/${encodeURIComponent(projectId)}/model/jobs/${encodeURIComponent(jobId)}`,
  )
  return mapViralProviderJob(job)
}

async function getLatestViralRemakeWorkspace(projectId: string): Promise<ApiViralRemakeWorkspace | null> {
  const result = await creativeRequest<{ items: Array<{ id: string; performance_mode?: string; status: string }> }>(
    `/projects/${encodeURIComponent(projectId)}/creative-tasks?limit=100`,
  )
  const task = result.items.find(item => item.performance_mode === 'viral_remake' && item.status !== 'archived')
  if (!task) return null
  return creativeRequest<ApiViralRemakeWorkspace>(
    `/projects/${encodeURIComponent(projectId)}/creative-tasks/${encodeURIComponent(task.id)}/viral-remake`,
  )
}

async function getViralRemakeWorkspace(projectId: string, taskId: string): Promise<ApiViralRemakeWorkspace> {
  return creativeRequest<ApiViralRemakeWorkspace>(
    `/projects/${encodeURIComponent(projectId)}/creative-tasks/${encodeURIComponent(taskId)}/viral-remake`,
  )
}

async function analyzeViralRemake(projectId: string, taskId: string): Promise<ApiViralRemakeWorkspace> {
  return creativeRequest<ApiViralRemakeWorkspace>(
    `/projects/${encodeURIComponent(projectId)}/creative-tasks/${encodeURIComponent(taskId)}/viral-remake:analyze-reference`,
    'POST',
    undefined,
    { 'Idempotency-Key': `viral-analysis-${taskId}-${Date.now()}` },
  )
}

async function updateViralPrompt(
  projectId: string,
  taskId: string,
  expectedRevision: number,
  dimensions: Record<ApiVideoPromptDimension['id'], string>,
): Promise<ApiViralRemakeWorkspace> {
  return creativeRequest<ApiViralRemakeWorkspace>(
    `/projects/${encodeURIComponent(projectId)}/creative-tasks/${encodeURIComponent(taskId)}/viral-remake/prompt-draft`,
    'PATCH',
    { expected_revision: expectedRevision, dimensions },
  )
}

async function confirmViralGeneration(
  projectId: string,
  taskId: string,
  expectedRevision: number,
  confirmReferenceImageRights: boolean,
): Promise<ApiViralRemakeWorkspace> {
  return creativeRequest<ApiViralRemakeWorkspace>(
    `/projects/${encodeURIComponent(projectId)}/creative-tasks/${encodeURIComponent(taskId)}/viral-remake:confirm-generation`,
    'POST',
    {
      expected_revision: expectedRevision,
      confirm_reference_video_rights: true,
      confirm_reference_image_rights: confirmReferenceImageRights,
    },
    { 'Idempotency-Key': `viral-confirm-${taskId}-${expectedRevision}` },
  )
}

type ApiProviderJobWire = {
  id: string
  project_id: string
  kind: string
  execution_status: 'pending' | 'running' | 'succeeded' | 'failed' | 'cancelled'
  provider_status: string
  project_asset_refs: Array<{ asset_version: ApiAssetVersionRef }>
  error?: { message?: string }
  version: number
  created_at: string
  updated_at: string
}

function mapViralProviderJob(job: ApiProviderJobWire): ApiGenerationJob {
  const providerStatus = job.provider_status
  let status: ApiGenerationJob['status'] = 'queued'
  if (job.execution_status === 'failed' || providerStatus === 'failed' || providerStatus === 'expired') status = 'failed'
  else if (job.execution_status === 'cancelled' || providerStatus === 'cancelled') status = 'cancelled'
  else if (job.execution_status === 'succeeded' || providerStatus === 'succeeded' || providerStatus === 'partially_succeeded') status = 'succeeded'
  else if (job.execution_status === 'running' || providerStatus !== 'submitted') status = 'running'
  return {
    id: job.id,
    projectId: job.project_id,
    artifactKind: 'video',
    status,
    model: 'cookies.video.standard',
    diagnostic: job.error?.message,
    artifactId: job.project_asset_refs.at(-1)?.asset_version.asset_id,
    version: job.version,
    createdAt: job.created_at,
    updatedAt: job.updated_at,
  }
}

async function createViralVideoJob(projectId: string, taskId: string): Promise<ApiGenerationJob> {
  const job = await creativeRequest<ApiProviderJobWire>(
    `/projects/${encodeURIComponent(projectId)}/creative-tasks/${encodeURIComponent(taskId)}:video-job`,
    'POST',
    {},
    { 'Idempotency-Key': `viral-video-${taskId}-${Date.now()}` },
  )
  return mapViralProviderJob(job)
}

async function getViralVideoJob(projectId: string, jobId: string): Promise<ApiGenerationJob> {
  const job = await platformRequest<ApiProviderJobWire>(
    `/projects/${encodeURIComponent(projectId)}/model/jobs/${encodeURIComponent(jobId)}`,
  )
  return mapViralProviderJob(job)
}

async function submitViralCandidateReview(
  projectId: string,
  taskId: string,
  candidateId: string,
): Promise<ApiViralRemakeWorkspace> {
  return creativeRequest<ApiViralRemakeWorkspace>(
    `/projects/${encodeURIComponent(projectId)}/creative-tasks/${encodeURIComponent(taskId)}/viral-remake/candidates/${encodeURIComponent(candidateId)}:submit-review`,
    'POST',
  )
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
  listAgencyWorkbench: async (options: AgencyWorkbenchOptions = {}) => {
    // Workbench data always follows the caller's accessible Project scope.
    // includeDemoProject is retained as a compatibility hint, but it must not
    // introduce a hard-coded Project outside the current identity.
    const projectIds = options.projectIds ?? []
    return loadPersistedAgencyWorkbench(projectIds)
  },
  getSession: async () => authSessionFromActor((await platformRequest<PlatformRequestContext>('/context')).actor),
  login: async (input: { username: string; password: string }) => {
    const result = await platformRequest<PlatformLoginResult>('/auth/login', 'POST', {
      username: input.username,
      password: input.password,
    })
    return authSessionFromActor(result.actor, input.username)
  },
  logout: async () => {
    await platformRequest<void>('/auth/logout', 'POST')
    return { authenticated: false }
  },
  getCapabilities: getKanonCapabilities,
  getProviderConfiguration: () => request<ApiProviderConfiguration>('/provider/configuration'),
  updateProviderConfiguration: (input: { apiKey: string; baseUrl?: string }) =>
    request<ApiProviderConfiguration>('/provider/configuration', 'PUT', input),
  deleteProviderConfiguration: () => request<ApiProviderConfiguration>('/provider/configuration', 'DELETE'),
  getPublicInsightOverview: () => request<ApiPublicInsightOverview>('/public-insights/overview'),
  getPublicInsightFilters: () => request<ApiPublicInsightFilters>('/public-insights/filters'),
  listPublicInsightVideos: (input: {
    page?: number
    pageSize?: number
    keyword?: string
    industry?: string
    aiGenerated?: string
    visualStyle?: string
    sortBy?: string
    sortOrder?: 'asc' | 'desc'
  } = {}) => {
    const search = new URLSearchParams({
      page: String(input.page ?? 1),
      page_size: String(input.pageSize ?? 20),
      keyword: input.keyword ?? '',
      industry: input.industry ?? '',
      ai_generated: input.aiGenerated ?? '全部',
      visual_style: input.visualStyle ?? '',
      sort_by: input.sortBy ?? 'vv_all',
      sort_order: input.sortOrder ?? 'desc',
    })
    return request<ApiPublicInsightVideoPage>(`/public-insights/videos?${search.toString()}`)
  },
  getPublicInsightVideo: (itemId: string) =>
    request<ApiPublicInsightVideoDetail>(`/public-insights/videos/${encodeURIComponent(itemId)}`),
  listProjects: () => platformClient.listProjects(),
  getProjectSnapshot: (projectId: string) => platformClient.getProjectSnapshot(projectId),
  listProjectMediaAssets: (projectId: string) => platformClient.listProjectMediaAssets(projectId),
  runMaterialQualityCheck: async (projectId: string, assetId: string, version: number) => {
    const item = await platformRequest<{
      id: string
      organization_id: string
      project_id: string
      asset_id: string
      asset_version: number
      status: string
      model: string
      rule_version: string
      prompt_version: string
      summary: string
      issues: Array<{ id: string; severity: string; rule: string; evidence: string; suggestion: string }>
      created_at: string
      completed_at?: string
    }>(
      `/projects/${encodeURIComponent(projectId)}/assets/${encodeURIComponent(assetId)}/versions/${version}/quality-checks`,
      'POST',
      {},
      { 'Idempotency-Key': `material-qc-${projectId}-${assetId}-${version}` },
    )
    return {
      id: item.id,
      organizationId: item.organization_id,
      projectId: item.project_id,
      assetId: item.asset_id,
      assetVersion: item.asset_version,
      status: item.status as ApiQualityCheckStatus,
      model: item.model,
      ruleVersion: item.rule_version,
      promptVersion: item.prompt_version,
      summary: item.summary,
      issues: item.issues.map(issue => ({ ...issue, severity: issue.severity as ApiQualityIssueSeverity })),
      createdAt: item.created_at,
      completedAt: item.completed_at,
    } satisfies ApiQualityCheckRun
  },
  recordMaterialConfirmation: async (projectId: string, assetId: string, version: number, input: { status: ApiMaterialConfirmationStatus; scope: string; note: string }) => {
    const item = await platformRequest<{
      id: string
      organization_id: string
      project_id: string
      quality_check_run_id: string
      asset_id: string
      asset_version: number
      status: string
      scope: string
      confirmed_by: string
      note: string
      created_at: string
    }>(
      `/projects/${encodeURIComponent(projectId)}/assets/${encodeURIComponent(assetId)}/versions/${version}/confirmations`,
      'POST',
      input,
      { 'Idempotency-Key': `material-confirm-${projectId}-${assetId}-${version}-${input.status}` },
    )
    return {
      id: item.id,
      organizationId: item.organization_id,
      projectId: item.project_id,
      qualityCheckRunId: item.quality_check_run_id,
      assetId: item.asset_id,
      assetVersion: item.asset_version,
      status: item.status as ApiMaterialConfirmationStatus,
      scope: item.scope,
      confirmedBy: item.confirmed_by,
      note: item.note,
      createdAt: item.created_at,
    } satisfies ApiMaterialConfirmation
  },
  setMaterialDeliveryVersion: (projectId: string, assetId: string, version: number) =>
    platformRequest(
      `/projects/${encodeURIComponent(projectId)}/assets/${encodeURIComponent(assetId)}/version-pointer`,
      'PATCH',
      { delivery_version: version },
      { 'Idempotency-Key': `material-delivery-${projectId}-${assetId}-${version}` },
    ),
  createProject: (input: Pick<ApiProject, 'name' | 'brand' | 'objective' | 'industry'>) =>
    platformClient.createProject(input),
  updateProject: (id: string, input: Partial<Pick<ApiProject, 'name' | 'brand' | 'objective' | 'industry'>> & { expectedContextVersion?: number }) =>
    platformClient.updateProject(id, input),
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
    projectId: string,
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
    createKanonBrief(_projectId, _prompt),
  confirmBrief: (artifact: ApiArtifact) => confirmKanonBrief(artifact),
  attachBriefProductAsset: (artifact: ApiArtifact, asset: ApiAssetVersionRef) =>
    attachKanonBriefProductAsset(artifact, asset),
  createMedia: (
    projectId: string,
    kind: 'image' | 'video',
    prompt: string,
    briefId: string,
  ) => createKanonMedia(projectId, kind, prompt, briefId),
  createCommercePrerollVideo: (
    projectId: string,
    prompt: string,
    briefId: string,
  ) => createKanonCommercePrerollVideo(projectId, prompt, briefId),
  createPreparedCommercePrerollVideo: (
    projectId: string,
    prompt: string,
    sourceId: string,
    productAsset: { asset_id: string; version: number },
  ) => createKanonPreparedCommercePrerollVideo(projectId, prompt, sourceId, productAsset),
  listCommercePrerollSources: (projectId: string) =>
    listKanonCommercePrerollSources(projectId),
  prepareCommercePreroll: (
    projectId: string,
    source: ApiCreativeSourceOption,
    templateId: ApiCommerceTemplateId,
  ) => prepareKanonCommercePreroll(projectId, source, templateId),
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
      '生成 9:16、独立 6 秒、静音可理解的短剧广告前贴，并以清晰 CTA 收束。',
    ].filter(Boolean).join('。'),
    briefId,
  ).then(job => ({ ...job, purpose: scope.purpose, prerollType: scope.prerollType })),
  listAuditEvents: (projectId?: string) =>
    projectId ? platformClient.listAuditEvents(projectId) : Promise.resolve([]),
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
  uploadProjectAsset,
  createManualViralRemakeWorkspace,
  createManualShortDramaPrerollWorkspace,
  selectShortDramaPrerollCandidate,
  regenerateShortDramaPrerollCandidates,
  createShortDramaPrerollVideoJob,
  getShortDramaPrerollVideoJob,
  getLatestShortDramaPrerollWorkspace,
  ensureCommercePrerollFixtureWorkspace,
  getLatestCommercePrerollWorkspace,
  updateCommercePrerollDraft,
  confirmCommercePrerollGeneration,
  createCommercePrerollWorkspaceVideoJob,
  createManualGamePrerollWorkspace,
  selectGamePrerollCandidate,
  regenerateGamePrerollCandidates,
  createGamePrerollVideoJob,
  getLatestGamePrerollWorkspace,
  getGamePrerollVideoJob,
  getLatestViralRemakeWorkspace,
  getViralRemakeWorkspace,
  analyzeViralRemake,
  updateViralPrompt,
  confirmViralGeneration,
  createViralVideoJob,
  getViralVideoJob,
  submitViralCandidateReview,
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
