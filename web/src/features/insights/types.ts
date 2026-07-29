export type DeliveryExecutionSnapshot = {
  id: string
  change_set_id: string
  plan_id: string
  creative_package_id: string
  mode: 'local_simulation'
  evidence_id: string
  evidence_summary: string
  metric_snapshot?: {
    id: string
    source: 'demo_fixture'
    is_simulated: true
    dataset_version: 'preroll-demo/v1'
    currency: 'CNY'
    raw_metrics: { impressions: number, clicks: number, conversions: number, spend_cents: number }
  }
}

export type InsightReport = {
  id: string
  organization_id: string
  project_id: string
  execution_id: string
  delivery_mode: 'local_simulation'
  evidence_id: string
  evidence_summary: string
  metric_snapshot_id: string
  creative_package_id: string
  is_simulated: boolean
  dataset_version: string
  status: 'draft' | 'confirmed'
  summary: string
  findings: string[]
  version: number
  created_by: string
  confirmed_by?: string
  confirmed_at?: string
  created_at: string
  updated_at: string
}

// 待确认 → 已确认 → 待复审 → 已失效。只有已确认的经验可以被下游引用。
export type ExperienceStatus = 'pending' | 'confirmed' | 'needs_review' | 'retired'

export type ExperienceReferenceOutcome = 'referenced' | 'adopted' | 'modified' | 'rejected'

export type Experience = {
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
  status: ExperienceStatus
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

export type ExperienceAudit = {
  id: string
  organization_id: string
  project_id: string
  experience_id: string
  from_status: ExperienceStatus | ''
  to_status: ExperienceStatus
  reason: string
  actor_id: string
  created_at: string
}

export type ExperienceReference = {
  id: string
  organization_id: string
  project_id: string
  experience_id: string
  consumer_kind: string
  consumer_id: string
  outcome: ExperienceReferenceOutcome
  note: string
  version: number
  created_by: string
  created_at: string
  updated_at: string
}

export type RecordExperienceReferenceInput = {
  consumer_kind: string
  consumer_id: string
  outcome: ExperienceReferenceOutcome
  note?: string
}

export type PreLaunchInsight = {
  project_id: string
  experience_references: Experience[]
  disclosure: string
}

export type PerformanceOverview = {
  project_id: string
  executions: DeliveryExecutionSnapshot[]
  disclosure: string
}
