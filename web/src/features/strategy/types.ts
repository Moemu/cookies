export type Workspace = {
  id: string
  project_id: string
  name: string
  is_primary: boolean
  status: 'active' | 'archived'
  version: number
}
export type Conversation = {
  id: string
  project_id: string
  workspace_id: string
  status: 'open' | 'waiting_user' | 'closed'
  version: number
}

export type StrategyTask = {
  id: string
  project_id: string
  workspace_id: string
  conversation_id: string
  brief_id: string
  current_agent_task_id?: string
  current_strategy_id?: string
  status: 'active' | 'waiting_user' | 'ready_to_confirm' | 'completed' | 'cancelled'
  version: number
}

export type Message = {
  id: string
  conversation_id: string
  role: 'user' | 'assistant' | 'system_event'
  content_type: 'text' | 'business_card' | 'error_notice'
  content: string
  ai_generated: boolean
  created_at: string
}

export type FieldState = {
  field_path: string
  confidence: 'low' | 'medium' | 'high'
  confirmation: 'unconfirmed' | 'confirmed'
  source: { type: string; id: string; locator?: string }
}

export type BriefDocument = {
  contract_version: 'strategy-brief-version/v1'
  campaign: { objective: string }
  audience: { primary: string }
  proposition: string
  channels: string[]
  budget: { total: string }
  schedule: { window: string }
  constraints: string[]
  measurement: { primary_kpi: string }
}

export type BriefDraft = {
  id: string
  brief_id: string
  status: 'open' | 'confirmed' | 'superseded'
  version: number
  document: BriefDocument
  field_states: Record<string, FieldState>
  completeness: {
    ready: boolean
    blockers: Array<{ field: string; reason: string }>
    warnings: Array<{ field: string; reason: string }>
  }
}

export type BriefVersion = {
  brief_id: string
  version: number
  content_hash: string
  snapshot: BriefDocument
}

export type StrategyDocument = {
  contract_version: 'strategy-draft/v1'
  objective: string
  audience: { primary: string; insights: string[] }
  proposition: string
  channel_strategy: Array<{ platform: string; role: string; formats: string[] }>
  creative_recommendations: string[]
  constraints: string[]
  budget_and_cadence: { budget: string; cadence: string }
  experiment_matrix: Array<{ hypothesis: string; variable: string; metric: string }>
  measurement: string[]
  assumptions_and_gaps: string[]
}

export type DraftRevision = {
  strategy_id: string
  revision: number
  content_hash: string
  changed_sections: string[]
  document: StrategyDocument
}

export type StrategyDraft = {
  id: string
  task_id: string
  brief_id: string
  brief_version: number
  status: 'generating' | 'draft' | 'ready_for_review' | 'returned' | 'approved' | 'failed' | 'cancelled'
  current_revision: number
  current_review_id?: string
  version: number
  revision?: DraftRevision
}

export type GenerationReadiness = {
  ready: boolean
  generation_mode: 'deterministic' | 'provider'
  model_alias?: string
  upstream_model?: string
  route_revision_id?: string
  response_mode?: 'json_schema' | 'json_object' | 'prompt_json'
  prompt_version: string
  reason_code?: string
}

export type GenerationMetadata = {
  generation_mode: 'deterministic' | 'fake_template' | 'provider'
  provider_code?: string
  model_alias?: string
  model_version?: string
  route_revision_id?: string
  response_mode?: 'json_schema' | 'json_object' | 'prompt_json'
  prompt_version?: string
  skill_versions: Record<string, string>
  skill_snapshot_hashes: Record<string, string>
  generation_context_hash?: string
  output_hash?: string
  usage?: { input_tokens: number; output_tokens: number; total_tokens: number }
  latency_ms?: number
  validation_attempts: number
  quality_report?: { passed: boolean; score: number; errors: string[]; warnings: string[] }
}

export type Review = {
  id: string
  strategy_id: string
  candidate_revision: number
  candidate_content_hash: string
  status: 'open' | 'returned' | 'approved' | 'invalidated'
}

export type PackageVersion = {
  package_id: string
  version: number
  content_hash: string
  status: 'published' | 'superseded' | 'archived'
  snapshot: {
    strategy_id: string
    strategy_revision: number
    readiness: {
      creative_ready: boolean
      delivery_ready: boolean
      insights_ready: boolean
    }
  }
}

export type WorkspaceDetail = {
  workspace: Workspace
  current_conversation?: Conversation
  current_task?: StrategyTask
}

export type ConversationBundle = {
  conversation: Conversation
  task: StrategyTask
  brief_draft: BriefDraft
}
