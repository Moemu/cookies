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

export type CreativeIntake = {
  id: string
  organization_id: string
  project_id: string
  source: 'manual' | 'strategy_package' | 'uploaded_document' | 'conversation'
  status: CreativeIntakeStatus
  request: CreativeIntakeInput
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
  format: 'image_text'
  channel: 'xiaohongshu'
  status: 'draft' | 'in_progress' | 'ready_for_review'
  direction: { concept: string, tone: string[], visual_keywords: string[] }
  version: number
  created_at: string
  updated_at: string
}

export type ImageTextDraft = {
  task_id: string
  version: number
  status: 'draft' | 'ready_for_review' | 'approved' | 'superseded'
  title_candidates: string[]
  body: string
  topics: string[]
  cover_copy: string
  image_plan: Array<{ order: number, purpose: string, visual_brief: string, caption: string }>
  created_at: string
}

export type CreativeTaskDetail = {
  task: CreativeTask
  intake: CreativeIntake
  draft: ImageTextDraft
  production_jobs: Array<{ task_id: string, kind: 'cover_image', provider_job_id: string, created_at: string }>
}
