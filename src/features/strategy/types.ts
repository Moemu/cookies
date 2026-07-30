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
  discarded_at?: string
  discarded_by?: string
  discard_reason?: string
  version: number
}

export type StrategyTaskListItem = {
  task: StrategyTask & { created_at: string; updated_at: string }
  name: string
  objective: string
  brief_status: BriefDraft['status']
  brief_ready: boolean
  strategy_status?: StrategyDraft['status']
  review_status?: Review['status']
  strategy_revision: number
  strategy_version: number
  strategy_archived_at?: string
  strategy_archived_by?: string
  strategy_archive_reason?: string
}

export type StrategyTaskBundle = {
  workspace: Workspace
  conversation: Conversation
  task: StrategyTask
  brief_draft: BriefDraft
}

export type Message = {
  id: string
  conversation_id: string
  role: 'user' | 'assistant' | 'system_event'
  content_type: 'text' | 'business_card' | 'error_notice'
  content: string
  ai_generated: boolean
  agent_task_id?: string
  skill_run_ids?: string[]
  created_at: string
}

export type FieldState = {
  field_path: string
  confidence: 'low' | 'medium' | 'high'
  confirmation: 'unconfirmed' | 'confirmed'
  source: { type: string; id: string; locator?: string }
}

export type BriefDocument = {
  contract_version: 'strategy-brief-version/v1' | 'strategy-brief-version/v2'
  brand?: { name?: string }
  product?: { name?: string; evidence?: string[] }
  industry?: string
  region?: string
  language?: string
  campaign: { objective: string }
  audience: { primary: string }
  proposition: string
  channels: string[]
  budget: { total: string }
  schedule: { window: string }
  constraints: string[]
  measurement: { primary_kpi: string }
  platform_briefs?: Array<{
    platform: string
    role?: string
    content_formats?: string[]
    conversion_path?: string
    budget?: string
    primary_kpi?: string
  }>
  creative?: {
    tone?: string[]
    mandatory_elements?: string[]
    prohibited_claims?: string[]
  }
  reference_ids?: string[]
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

export type BriefCenterSummary = {
  brief_id: string
  task_id: string
  workspace_id: string
  name: string
  objective: string
  status: BriefDraft['status']
  version: number
  ready: boolean
  blocker_count: number
  warning_count: number
  conflict_count: number
  latest_confirmed_version: number
  discarded_at?: string
  updated_at: string
}

export type BriefCenterDetail = {
  summary: BriefCenterSummary
  draft: BriefDraft
  versions: BriefVersion[]
}

export type PlatformPlan = {
  platform: string
  role: string
  audience_angle: string
  content_pillars: string[]
  formats: string[]
  conversion_path: string
  cadence: string
  primary_kpi: string
  creative_ideas: string[]
  constraints: string[]
}

export type StrategyDocument = {
  contract_version: 'strategy-draft/v1' | 'strategy-draft/v2'
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
  executive_summary?: string
  cross_platform_role?: string
  platform_plans?: PlatformPlan[]
  evidence_refs?: string[]
  compliance?: {
    contract_version: string
    passed: boolean
    issues: Array<{ rule_id: string; severity: 'warning' | 'blocker'; message: string; evidence?: string }>
    checked_at: string
  }
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
  archived_at?: string
  archived_by?: string
  archive_reason?: string
  current_revision: number
  current_review_id?: string
  version: number
  skill_versions: Record<string, string>
  revision?: DraftRevision
}

export type StrategyCenterSummary = {
  strategy_id: string
  task_id: string
  workspace_id: string
  name: string
  objective: string
  brief_id: string
  brief_version: number
  status: StrategyDraft['status']
  current_revision: number
  version: number
  review_id?: string
  review_status?: Review['status']
  package_id?: string
  package_version: number
  package_status?: PackageVersion['status']
  archived_at?: string
  archived_by?: string
  archive_reason?: string
  created_at: string
  updated_at: string
}

export type GenerationReadiness = {
  ready: boolean
  generation_mode: 'deterministic' | 'provider'
  model_alias?: string
  upstream_model?: string
  route_revision_id?: string
  response_mode?: 'json_schema' | 'json_object' | 'prompt_json'
  api_mode?: 'chat_completions' | 'responses'
  background?: boolean
  prompt_version: string
  reason_code?: string
}

export type GenerationProbe = {
  ready: boolean
  provider_code: string
  model_alias: string
  model_version: string
  route_revision_id?: string
  response_mode?: 'json_schema' | 'json_object' | 'prompt_json'
  api_mode?: 'chat_completions' | 'responses'
  background?: boolean
  usage?: { input_tokens: number; output_tokens: number; total_tokens: number }
  latency_ms: number
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
  attempts: SkillRunAttempt[]
}

export type Review = {
  id: string
  project_id: string
  strategy_id: string
  candidate_revision: number
  candidate_content_hash: string
  status: 'open' | 'returned' | 'approved' | 'invalidated'
  decision_reason?: string
  decided_by?: string
  decided_at?: string
  created_by: string
  created_at: string
  updated_at: string
  review_mode?: 'self_confirmation' | 'leader_approval' | 'designated_approvers'
  required_approvals?: number
  approval_count?: number
  assignments?: ReviewAssignment[]
}

export type ReviewAssignment = {
  id: string
  review_id: string
  reviewer_user_id: string
  review_mode: 'self_confirmation' | 'leader_approval' | 'designated_approvers'
  status: 'pending' | 'approved' | 'returned' | 'cancelled'
  decision_reason?: string
  decided_at?: string
}

export type ReviewPolicy = {
  organization_id: string
  project_id: string
  mode: 'self_confirmation' | 'leader_approval' | 'designated_approvers'
  approver_user_ids: string[]
  allow_self_approval: boolean
  version: number
  updated_by?: string
  updated_at?: string
}

export type ReviewComment = {
  id: string
  review_id: string
  author_id: string
  body: string
  created_at: string
}

export type DeepReviewFinding = {
  severity: 'blocker' | 'warning' | 'opportunity'
  section: string
  title: string
  detail: string
  recommendation: string
}

export type DeepReviewAnalysis = {
  id: string
  review_id: string
  strategy_id: string
  candidate_revision: number
  candidate_content_hash: string
  agent_task_id: string
  status: 'pending' | 'succeeded' | 'failed'
  summary?: string
  findings: DeepReviewFinding[]
  model_alias?: string
  model_version?: string
  route_revision_id?: string
  response_mode?: 'json_schema' | 'json_object' | 'prompt_json'
  api_mode?: 'chat_completions' | 'responses'
  background?: boolean
  usage?: { input_tokens: number; output_tokens: number; total_tokens: number }
  latency_ms?: number
}

export type PackageVersion = {
  package_id: string
  version: number
  content_hash: string
  status: 'published' | 'superseded' | 'archived'
  published_by?: string
  published_at?: string
  snapshot: {
    contract_version?: 'strategy-package/v1' | 'strategy-package/v2'
    strategy_id: string
    strategy_revision: number
    strategy?: StrategyDocument
    brief?: BriefVersion
    readiness: {
      creative_ready: boolean
      delivery_ready: boolean
      insights_ready: boolean
    }
  }
}

export type SkillRun = {
  id: string
  agent_task_id: string
  skill_name: string
  skill_version: string
  status: string
  input_hash: string
  output_hash?: string
  generation_mode?: string
  model_version?: string
  prompt_version?: string
  latency_ms: number
  validation_attempts: number
  quality_report?: { passed: boolean; score: number; errors: string[]; warnings: string[] }
  attempts: SkillRunAttempt[]
}

export type SkillRunAttempt = {
  attempt_no: number
  purpose: 'conversation' | 'generate' | 'repair' | 'revise' | 'deep_review' | string
  provider_code: string
  model_alias?: string
  model_version?: string
  route_revision_id?: string
  response_mode?: 'json_schema' | 'json_object' | 'prompt_json'
  api_mode?: string
  background?: boolean
  prompt_version?: string
  usage?: { input_tokens: number; output_tokens: number; total_tokens: number }
  latency_ms: number
  validation_passed: boolean
  validation_errors: string[]
  output_hash?: string
  created_at: string
}

export type SkillDescriptor = {
  name: string
  version: string
  kind: 'platform' | 'objective'
  match: string[]
  quality_checks: string[]
  content_hash: string
}

export type KnowledgeDocument = {
  id: string
  project_id: string
  title?: string
  source_uri?: string
  source_type?: string
  filename: string
  mime_type: string
  size_bytes: number
  content_sha256: string
  text_sha256: string
  status: 'ready'
  created_at: string
}

export type AgentTask = {
  id: string
  status: 'dispatch_pending' | 'queued' | 'running' | 'succeeded' | 'failed' | 'cancelled'
  version: number
  error?: { code: string; message: string; retryable: boolean }
}

export type AgentTaskInspection = {
  task: AgentTask
  job?: {
    status: 'queued' | 'running' | 'succeeded' | 'failed' | 'cancelled'
    attempt_count: number
    max_attempts: number
    error?: { code: string; message: string; retryable: boolean }
  }
}

export type ResearchArtifact = {
  id: string
  research_run_id: string
  source_type: string
  category: 'general' | 'audience' | 'competitor' | 'industry'
  title: string
  source_url?: string
  content: string
  citations: string[]
  content_hash: string
}

export type ResearchRun = {
  id: string
  mode: 'web' | 'mcp'
  category: ResearchArtifact['category']
  query: string
  document_ids: string[]
  disclosed_fields: string[]
  status: 'running' | 'succeeded' | 'failed' | 'unavailable'
  confirmed_at: string
  created_at: string
  updated_at: string
  error_code?: string
  error_message?: string
  artifacts: ResearchArtifact[]
}

export type EvidenceReference = {
  evidence_type: 'research_artifact' | 'knowledge_document' | 'external_reference'
  evidence_id: string
  target_type: 'brief_draft' | 'brief_version' | 'strategy_revision'
  target_id: string
  target_version: number
  field_path: string
  content_hash: string
  created_by: string
  created_at: string
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

export type ConversationMemory = {
  summary: string
  open_questions: string[]
  version: number
}
