export type CreativeIntakeStatus = 'draft' | 'needs_clarification' | 'ready' | 'superseded'

export type CreativeIntakeInput = {
  source: 'manual'
  channel: 'xiaohongshu'
  objective: string
  audience: string
  core_message: string
  call_to_action: string
  concept: string
  tone: string[]
  visual_keywords: string[]
  mandatory_elements: string[]
  prohibited_claims: string[]
}

export type CreativeIntakeRequest = Omit<CreativeIntakeInput, 'source'> & {
  source: CreativeIntake['source']
  strategy_package?: {
    package_id: string
    package_version: number
    expected_content_hash: string
  }
  creative_routes?: CreativeRouteSnapshot[]
}

export type CreativeRouteSnapshot = {
  route_type: 'pre_roll'
  video_purpose: 'performance'
  channels: Array<'douyin' | 'kuaishou'>
  reason: string
  target_duration_seconds: 5
  aspect_ratio: '9:16'
  source_asset_refs: AssetVersionRef[]
  evidence_refs: string[]
  requires_human_confirmation: true
}

export type AssetVersionRef = { asset_id: string, version: number }

export type CreativeIntake = {
  id: string
  organization_id: string
  project_id: string
  source: 'manual' | 'strategy_package' | 'uploaded_document' | 'conversation'
  status: CreativeIntakeStatus
  request: CreativeIntakeRequest
  missing_fields: string[]
  warnings: string[]
  confirmed_by?: string
  version: number
  created_at: string
  updated_at: string
}

export type CreativeTask = {
  id: string
  organization_id: string
  project_id: string
  intake_id: string
  format: 'image_text' | 'video'
  channel: 'xiaohongshu' | 'douyin' | 'kuaishou'
  video_purpose?: 'performance'
  performance_mode?: 'pre_roll'
  status: 'draft' | 'generating' | 'generated' | 'rendering' | 'in_progress' | 'ready_for_review' | 'approved' | 'delivered' | 'archived'
  direction: { content_type: CreativeContentType, focus: string, audience: string, core_message: string, call_to_action: string, concept: string, tone: string[], visual_keywords: string[] }
  version: number
  created_at: string
  updated_at: string
}

export type CreativeContentType = 'lifestyle' | 'ingredient_explanation' | 'usage_scenario' | 'list_guide' | 'comparison' | 'custom'

export type CreateCreativeTaskInput = {
  content_type: CreativeContentType
  focus: string
  audience?: string
  core_message?: string
  call_to_action?: string
}

export type ImageTextDraft = {
  task_id: string
  version: number
  status: 'draft' | 'ready_for_review' | 'approved' | 'superseded'
  title_candidates: string[]
  body: string
  topics: string[]
  cover_copy: string
  image_plan: Array<{ order: number, purpose: string, visual_brief: string, caption: string, asset_ref?: { asset_id: string, version: number } }>
  created_at: string
}

export type ReviseDraftInput = {
  expected_version: number
  title_candidates: string[]
  body: string
  topics: string[]
  cover_copy: string
  image_plan: ImageTextDraft['image_plan']
}

export type CreativeVersion = {
  id: string
  organization_id: string
  project_id: string
  creative_task_id: string
  version: number
  draft_version: number
  status: 'created' | 'checked' | 'approved' | 'superseded'
  format?: 'image_text' | 'video'
  snapshot: ImageTextDraft
  video_snapshot?: VideoVersionSnapshot
  content_hash: string
  created_by: string
  created_at: string
  check?: { passed: boolean, blockers: string[], warnings: string[], checked_by: string, checked_at: string }
  approval?: { approved_by: string, approved_at: string }
}

export type CreativePackage = {
  id: string
  organization_id: string
  project_id: string
  creative_version_id: string
  content_hash: string
  format?: 'image_text' | 'video'
  snapshot: ImageTextDraft
  video_snapshot?: VideoVersionSnapshot
  created_by: string
  created_at: string
}

export type CreativeTaskDetail = {
  task: CreativeTask
  intake: CreativeIntake
  draft: ImageTextDraft
  video_draft?: VideoDraft
  production_jobs: Array<{ task_id: string, kind: string, provider_job_id: string, created_at: string }>
}
export type VideoDraft = {
  contract_version: 'creative-video-draft/v1'
  task_id: string
  revision: number
  concept: string
  prompt: string
  duration_seconds: 5
  aspect_ratio: '9:16'
  resolution: '720p'
  source_video: AssetVersionRef
  mandatory_elements: string[]
  prohibited_claims: string[]
  cta: string
  created_at: string
}

export type CreativeRenderJob = {
  id: string
  project_id: string
  creative_task_id: string
  status: 'queued' | 'running' | 'succeeded' | 'failed'
  pre_roll_video: AssetVersionRef
  main_video: AssetVersionRef
  output_asset?: { project_id: string, asset_version: AssetVersionRef }
  error_code?: string
  error_message?: string
  created_at: string
  updated_at: string
}

export type VideoVersionSnapshot = {
  contract_version: 'creative-video-version/v1'
  format: 'video'
  channel: 'douyin' | 'kuaishou'
  video_purpose: 'performance'
  performance_mode: 'pre_roll'
  strategy_package_ref: { package_id: string, package_version: number, expected_content_hash: string }
  draft_revision: number
  source_video: AssetVersionRef
  generated_preroll: AssetVersionRef
  final_video: AssetVersionRef
  provider_job_id: string
  render_job_id: string
}
