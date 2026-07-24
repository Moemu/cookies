export type DeliveryPlan = {
  id: string
  organization_id: string
  project_id: string
  creative_package_id: string
  creative_package_hash: string
  creative_version_id: string
  name: string
  objective: string
  budget_cents: number
  start_at: string
  end_at: string
  status: 'draft'
  version: number
  created_by: string
  created_at: string
  updated_at: string
}

export type ChangeSetStatus =
  | 'draft'
  | 'preflight_passed'
  | 'preflight_failed'
  | 'approved'
  | 'rejected'
  | 'executed'
  | 'rolled_back'

export type DeliveryChangeSet = {
  id: string
  organization_id: string
  project_id: string
  plan_id: string
  plan_version: number
  status: ChangeSetStatus
  risk_level: string
  preflight_notes: string[]
  approved_by?: string
  approved_at?: string
  version: number
  created_by: string
  created_at: string
  updated_at: string
}

export type DeliveryExecutionResult = {
  change_set: DeliveryChangeSet
  execution: {
    id: string
    organization_id: string
    project_id: string
    change_set_id: string
    status: 'succeeded'
    mode: 'local_simulation'
    executed_by: string
    started_at: string
    completed_at: string
  }
  evidence: {
    id: string
    organization_id: string
    project_id: string
    execution_id: string
    summary: string
    mode: 'local_simulation'
    reversible: boolean
    created_at: string
  }
}

export type DeliveryPlanDetail = {
  plan: DeliveryPlan
  change_sets: DeliveryChangeSet[]
  executions: DeliveryExecutionResult[]
}
