export type DeliveryExecutionSnapshot = {
  id: string
  change_set_id: string
  plan_id: string
  mode: 'local_simulation'
  evidence_id: string
  evidence_summary: string
}

export type InsightReport = {
  id: string
  organization_id: string
  project_id: string
  execution_id: string
  delivery_mode: 'local_simulation'
  evidence_id: string
  evidence_summary: string
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

export type Experience = {
  id: string
  organization_id: string
  project_id: string
  report_id: string
  source_execution_id: string
  source_evidence_id: string
  conclusion: string
  conditions: string[]
  counterexamples: string[]
  status: 'confirmed'
  version: number
  created_by: string
  created_at: string
  updated_at: string
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
