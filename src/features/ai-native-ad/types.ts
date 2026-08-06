export type AINativeStageId = 'requirement' | 'script' | 'storyboard' | 'video'

export type AINativeStageStatus = 'empty' | 'generating' | 'draft' | 'confirmed' | 'failed' | 'invalidated'

export type EditableTextItem = {
  id: string
  text: string
}

export type RequirementMedia = {
  id: string
  url: string
  role: string
  source: string
  asset_ref?: { asset_id: string; version: number }
}

export type AINativeProductPreview = {
  product_id: string
  product_name: string
  source: 'douyin_mall'
  source_url: string
}

export type AINativeRequirement = {
  contract_version: 'creative.ai-native.requirement/v1'
  revision: number
  status: 'draft'
  product: {
    source: 'douyin_mall'
    product_id: string
    name: string
    description: string
	images: Array<{ url: string; role: string; asset_ref?: { asset_id: string; version: number } }>
    price: {
      min_raw: number
      max_raw: number
      currency: 'CNY'
      display_unconfirmed: boolean
    }
    sales: number
    source_url: string
  }
  product_name: string
  product_description: string
  target_audiences: EditableTextItem[]
  media: RequirementMedia[]
  core_selling_points: EditableTextItem[]
  supplemental_requirement: string
  channel: 'douyin'
  aspect_ratio: '9:16'
  duration_seconds: number
  language: 'zh-CN'
  needs_confirmation: string[]
  generation: {
    mode: 'model' | 'deterministic_fallback'
    model_alias: string
    model_version: string
    route_revision_id?: string
    prompt_version: 'ai-native-requirement/douyin-v1'
  }
}

export type AINativeRequirementWorkspace = {
  workspace_id: string
  display_name: string
  creative_intake_id: string
  creative_task_id: string
  organization_id: string
  project_id: string
  status: 'draft' | 'confirmed'
  current_stage: 'requirement' | 'script' | 'storyboard' | 'production'
  workspace_version: number
  active_operation_id?: string
  active_operation_version?: number
  current_revision: number
  confirmed_revision?: number
  requirement: AINativeRequirement
  script_status?: 'generating' | 'draft' | 'confirmed' | 'failed'
  current_script_revision?: number
  confirmed_script_revision?: number
  script?: AdScriptDraft
  script_error_code?: string
  script_error_message?: string
  storyboard_status?: 'generating' | 'draft' | 'confirmed' | 'failed'
  current_storyboard_revision?: number
  confirmed_storyboard_revision?: number
  storyboard?: StoryboardDraft
  storyboard_plan?: StoryboardDraft
  storyboard_error_code?: string
  storyboard_error_message?: string
  production_status?: 'running' | 'assets_ready' | 'rendering' | 'completed' | 'render_failed' | 'failed' | 'cancelled'
  current_production_revision?: number
  production_plan?: ProductionPlan
  production_progress?: ProductionProgress
  created_by: string
  confirmed_by?: string
  created_at: string
  updated_at: string
}

export type AINativeAdWorkspaceSummary = {
  workspace_id: string
  display_name: string
  product_name: string
  current_stage: 'requirement' | 'script' | 'storyboard' | 'production'
  status: 'draft' | 'confirmed'
  script_status?: 'generating' | 'draft' | 'confirmed' | 'failed'
  storyboard_status?: 'generating' | 'draft' | 'confirmed' | 'failed'
  production_status?: 'running' | 'assets_ready' | 'rendering' | 'completed' | 'render_failed' | 'failed' | 'cancelled'
  created_at: string
  updated_at: string
}

export type AINativeInvalidatedResource = {
  type: string
  id: string
  status: string
}

export type AINativeReopenImpact = {
  workspace_id: string
  stage: 'requirement' | 'script'
  expected_workspace_version: number
  superseded_requirement_revisions: number[]
  superseded_script_revisions: number[]
  invalidated_resources: AINativeInvalidatedResource[]
}

export type AdScriptSegment = {
  id: string
  start_ms: number
  end_ms: number
  purpose: 'hook' | 'pain' | 'proof' | 'benefit' | 'cta'
  visual_intent: string
  voiceover: string
  subtitle: string
  selling_point_ids: string[]
  conversion_action?: string
}

export type AdScriptDraft = {
  contract_version: 'creative.ai-native.script/v1'
  revision: number
  status: 'draft' | 'confirmed' | 'superseded'
  title: string
  creative_summary: string
  channel_profile_id: string
  channel_profile_hash: string
  duration_seconds: number
  segments: AdScriptSegment[]
  regeneration_note?: string
  based_on_revision?: number
  based_on_requirement_revision: number
  based_on_requirement_hash: string
  generation: {
    model_alias: string
    model_version: string
    route_revision_id?: string
    prompt_version: 'ai-ad-script/douyin/v1'
    profile_hash: string
    input_tokens?: number
    output_tokens?: number
    total_tokens?: number
    latency_ms?: number
  }
}

export type StoryboardAsset = {
  id: string
  role: 'product_identity' | 'person_identity' | 'scene_reference' | 'composition_reference' | 'audio_reference' | 'brand_element'
  name: string
  source: 'product_import' | 'project_asset' | 'ai_generated'
  asset_ref?: { asset_id: string; version: number }
  generation_brief?: string
  status: 'ready' | 'planned' | 'generating' | 'failed'
  generation_attempt?: number
  error_code?: string
  error_message?: string
}

export type StoryboardShot = {
  id: string
  start_ms: number
  end_ms: number
  duration_ms: number
  visual_content: string
  subjects_products_actions: string
  shot_size: string
  camera_movement: string
  reference_asset_ids: string[]
  voiceover: string
  subtitle: string
  sound_effect: string
  bgm_direction: string
  transition: string
  product_identity_required: boolean
}

export type StoryboardDraft = {
  contract_version: 'creative.ai-native.storyboard/v1'
  revision: number
  status: 'draft' | 'confirmed' | 'superseded'
  duration_seconds: number
  assets: StoryboardAsset[]
  shots: StoryboardShot[]
  channel_profile_id: string
  channel_profile_hash: string
  based_on_requirement_revision: number
  based_on_requirement_hash: string
  based_on_script_revision: number
  based_on_script_hash: string
  generation: {
    model_alias: string
    model_version: string
    route_revision_id?: string
    prompt_version: 'ai-ad-storyboard/douyin/v1'
    profile_hash: string
  }
}

export type VideoRenderState = {
  status: 'running' | 'assets_ready' | 'rendering' | 'completed' | 'render_failed' | 'failed' | 'cancelled'
  progress: number
  current_step: string
  completed_shots: number
  total_shots: number
  completed_speech_units: number
  total_speech_units: number
  failed_unit_id?: string
  failed_shot_id?: string
  failure_code?: string
  failure_reason?: string
  eta_seconds: number
  output_url?: string
}

export type VoiceoverFitSuggestion = {
  shot_id: string
  original_voiceover: string
  suggested_voiceover: string
  duration_ms: number
  max_characters: number
  prompt_version: string
  model_alias: string
  model_version: string
  route_revision_id?: string
}

export type GenerationAttempt = {
  id: string
  ordinal: number
  retry_of?: string
  status: 'planned' | 'submitted' | 'running' | 'ingesting' | 'succeeded' | 'failed' | 'cancelled'
  provider_job_id?: string
  output_asset_ref?: { asset_id: string; version: number }
  error_code?: string
  error_message?: string
}

export type ProductionVideoUnit = {
  id: string
  order: number
  shot_ids: string[]
  start_ms: number
  end_ms: number
  duration_seconds: number
  reference_asset?: { asset_id: string; version: number }
  reference_role?: StoryboardAsset['role']
  product_identity_required?: boolean
  attempts: GenerationAttempt[]
  selected_attempt_id?: string
}

export type ProductionReferenceFailure = {
  unit_id: string
  asset_id: string
  asset_name: string
  asset_source: StoryboardAsset['source']
  reason: string
}

export type ProductionPlan = {
  contract_version: 'creative.ai-native.production-plan/v1'
  revision: number
  status: 'prepared' | 'running' | 'assets_ready' | 'rendering' | 'completed' | 'render_failed' | 'failed' | 'cancelled'
  total_duration_ms: number
  aspect_ratio: '9:16'
  units: ProductionVideoUnit[]
  speech_units: Array<{ id: string; shot_id: string; start_ms?: number; end_ms?: number; text?: string; speaking_rate?: number; attempts: GenerationAttempt[]; selected_attempt_id?: string }>
  render?: {
    id: string
    status: 'rendering' | 'completed' | 'render_failed'
    progress_percent: number
    eta_seconds: number
    renderer_version: 'ffmpeg-ai-ad-timeline/v1'
    output_asset_ref?: { asset_id: string; version: number }
    error_code?: string
    error_message?: string
  }
}

export type ProductionProgress = {
  status: 'running' | 'assets_ready' | 'rendering' | 'completed' | 'render_failed' | 'failed' | 'cancelled'
  progress_percent: number
  current_step: string
  completed_video_units: number
  total_video_units: number
  completed_video_duration_ms: number
  completed_speech_units: number
  total_speech_units: number
  eta_seconds: number
  available_actions: string[]
}

export type AINativeFrontendState = {
  active_stage: AINativeStageId
  stage_status: Record<AINativeStageId, AINativeStageStatus>
  workspace: AINativeRequirementWorkspace | null
  script: AdScriptDraft | null
  storyboard: StoryboardDraft | null
  video: VideoRenderState | null
  pending_reopen: AINativeStageId | null
  error: string
}
