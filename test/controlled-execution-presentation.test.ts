import assert from 'node:assert/strict'
import test from 'node:test'
import type { ComputerUseRun } from '../src/features/delivery-controlled-execution/model.ts'
import { presentControlledExecution } from '../src/features/delivery-controlled-execution/presentation.ts'

const hash = 'a'.repeat(64)

test('controlled execution presentation keeps approval, confirmation, and result recovery distinct', () => {
  const cases: Array<{ name: string; run: Partial<ComputerUseRun>; kind: string; retry: boolean }> = [
    { name: 'approval invalid', run: { state: 'awaiting_confirmation', blocking_reason: 'APPROVAL_INVALID' }, kind: 'approval_expired', retry: false },
    { name: 'confirmation invalid', run: { state: 'awaiting_confirmation', blocking_reason: 'FINAL_CONFIRMATION_INVALID' }, kind: 'confirmation_expired', retry: false },
    { name: 'partial', run: { state: 'partial' }, kind: 'partial', retry: false },
    { name: 'unknown', run: { state: 'result_unknown' }, kind: 'result_unknown', retry: false },
    { name: 'cancelled', run: { state: 'cancelled' }, kind: 'cancelled', retry: false },
    { name: 'kill switch', run: { state: 'preparing', blocking_reason: 'KILL_SWITCH_ACTIVE' }, kind: 'kill_switch_active', retry: false },
    { name: 'takeover', run: { state: 'awaiting_takeover' }, kind: 'awaiting_takeover', retry: true },
  ]

  for (const value of cases) {
    const presentation = presentControlledExecution(run(value.run))
    assert.equal(presentation.kind, value.kind, value.name)
    assert.equal(presentation.allowsNormalRetry, value.retry, value.name)
  }
})

function run(patch: Partial<ComputerUseRun>): ComputerUseRun {
  return {
    schema_version: 'computer-use-run/v1', id: 'run_1', organization_id: 'org_1', project_id: 'project_1', platform: 'ocean_engine', account_id: 'account_1',
    authority: {
      schema_version: 'computer-use-authority/v1', business_execution_id: 'execution_1', change_set_id: 'change_1', approval_id: 'approval_1', approval_action_hash: hash,
      account_reference_id: 'account_1', object_fingerprint: 'object_1', action: 'create', budget_limit_minor: 100_00, currency: 'CNY',
      plan_canonical_hash: hash, intent_canonical_hash: hash, feedback_canonical_hash: hash, decision_canonical_hash: hash, configuration_canonical_hash: hash,
      workflow_id: 'workflow_1', workflow_canonical_hash: hash, workflow_step_id: 'step_1', skill_id: 'oceanengine-ecommerce-manual', skill_version: 'v0.1-calibration',
    },
    environment_id: 'environment_1', profile_id: 'profile_1', lease_id: 'lease_1', policy_id: 'policy_1', state: 'queued', paused: false, takeover_active: false,
    version: 1, idempotency_key: 'idempotency_1', request_hash: hash, created_by: 'user_1', created_at: '2026-08-12T00:00:00Z', updated_at: '2026-08-12T00:00:00Z',
    ...patch,
  }
}
