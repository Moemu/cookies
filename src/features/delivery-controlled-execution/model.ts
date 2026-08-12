export type ComputerUseRunState =
  | 'queued'
  | 'environment_check'
  | 'awaiting_takeover'
  | 'preparing'
  | 'awaiting_confirmation'
  | 'submitting'
  | 'verifying'
  | 'succeeded'
  | 'failed'
  | 'partial'
  | 'result_unknown'
  | 'cancelled'

export type ComputerUseBlockingReason =
  | 'FINAL_CONFIRMATION_REQUIRED'
  | 'FINAL_CONFIRMATION_INVALID'
  | 'APPROVAL_INVALID'
  | 'LEASE_INVALID'
  | 'KILL_SWITCH_ACTIVE'
  | 'ACCOUNT_MISMATCH'
  | 'PROJECT_NOT_ALLOWED'
  | 'SITE_NOT_ALLOWED'
  | 'PAGE_DRIFT'
  | 'WORKFLOW_DRIFT'
  | 'SKILL_DRIFT'
  | 'RESULT_RECONCILIATION_REQUIRED'

export type ComputerUseAuthorityBinding = {
  schema_version: 'computer-use-authority/v1'
  business_execution_id: string
  change_set_id: string
  approval_id: string
  approval_action_hash: string
  account_reference_id: string
  object_fingerprint: string
  action: string
  budget_limit_minor: number
  currency: 'CNY'
  plan_canonical_hash: string
  intent_canonical_hash: string
  feedback_canonical_hash: string
  decision_canonical_hash: string
  configuration_canonical_hash: string
  workflow_id: string
  workflow_canonical_hash: string
  workflow_step_id: string
  skill_id: string
  skill_version: string
}

export type ComputerUseRun = {
  schema_version: 'computer-use-run/v1'
  id: string
  organization_id: string
  project_id: string
  platform: 'ocean_engine'
  account_id: string
  authority: ComputerUseAuthorityBinding
  environment_id: string
  profile_id: string
  lease_id: string
  policy_id: string
  state: ComputerUseRunState
  blocking_reason?: ComputerUseBlockingReason
  paused: boolean
  takeover_active: boolean
  version: number
  idempotency_key: string
  request_hash: string
  created_by: string
  created_at: string
  updated_at: string
}

export type ComputerUseRunEvent = {
  id: string
  run_id: string
  sequence: number
  kind: string
  summary: string
  actor: string
  created_at: string
}

/** Evidence is already redacted by the platform service. The UI intentionally renders keys and references, not raw page facts. */
export type ComputerUseEvidence = {
  id: string
  run_id: string
  step_id: string
  diff_keys: string[]
  page_reference: string
  screenshot_reference?: string
  object_fingerprint: string
  skill_version: string
  selector_version: string
  action_version: string
  redaction_version: string
  created_at: string
}

export type ControlledExecutionWorkspace = {
  run: ComputerUseRun
  events: ComputerUseRunEvent[]
  evidence: ComputerUseEvidence[]
}

export type ControlledExecutionTransportState =
  | { kind: 'loading' }
  | { kind: 'empty' }
  | { kind: 'forbidden'; message: string }
  | { kind: 'error'; message: string }
  | { kind: 'ready'; workspace: ControlledExecutionWorkspace }

export type ControlledExecutionPresentation = {
  kind:
    | 'queued'
    | 'environment_check'
    | 'awaiting_takeover'
    | 'preparing'
    | 'awaiting_confirmation'
    | 'submitting'
    | 'verifying'
    | 'succeeded'
    | 'failed'
    | 'approval_expired'
    | 'confirmation_expired'
    | 'partial'
    | 'result_unknown'
    | 'cancelled'
    | 'kill_switch_active'
    | 'blocked'
  tone: 'neutral' | 'warning' | 'danger' | 'success'
  title: string
  detail: string
  allowsNormalRetry: boolean
}
