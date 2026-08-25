import assert from 'node:assert/strict'
import test from 'node:test'
import { controlledExecutionApi } from '../src/features/browser-rpa-execution/api.ts'

test('controlled execution API supports the complete Runner v3 browser flow', async () => {
  const calls: Array<{ url: string; method: string; body?: string }> = []
  const originalFetch = globalThis.fetch
  const run = {
    id: 'run_1', project_id: 'project_1', environment_id: 'env_1', profile_id: 'profile_1', policy_id: 'policy_1', lease_id: 'lease_1',
    version: 4, account_id: '1855554434276391', state: 'awaiting_confirmation',
    authority: { approval_action_hash: 'a'.repeat(64) },
  }
  globalThis.fetch = async (input, init) => {
    const url = String(input)
    const method = init?.method ?? 'GET'
    calls.push({ url, method, body: typeof init?.body === 'string' ? init.body : undefined })
    let value: unknown = run
    if (url.endsWith('/events') || url.endsWith('/evidence')) value = { items: [] }
    if (url.includes('/environments/')) value = { id: 'env_1', account_id: run.account_id, mode: 'local_visible', browser_version: 'Edge', region: 'local', healthy: true, version: 1 }
    if (url.includes('/browser-profiles/')) value = { id: 'profile_1', environment_id: 'env_1', account_id: run.account_id, state: 'ready', version: 1 }
    if (url.includes('/site-policies/')) value = { id: 'policy_1', account_id: run.account_id, allowed_page_kinds: ['promotion_create'], allowed_platform_project_ids: ['project-platform-1'], version: 1 }
    if (url.includes('/leases/')) value = { id: 'lease_1', run_id: 'run_1', holder: 'user', fencing_token: 3, version: 2, expires_at: '2026-08-25T12:00:00Z', heartbeat_deadline: '2026-08-25T12:00:00Z' }
    if (url.endsWith(':plan')) value = { schema_version: 'oceanengine-playwright-rpa-plan/v3', plan_kind: 'promotion_create', mode: 'prepare', status: 'ready', steps: [], blocked_reasons: [], allow_remote_write: false, maximum_final_clicks: 0 }
    if (url.endsWith('/confirmations')) value = { confirmation: { id: 'confirmation_1' }, token: 'memory-only-token' }
    return new Response(JSON.stringify(value), { status: 200, headers: { 'Content-Type': 'application/json' } })
  }

  try {
    const workspace = await controlledExecutionApi.getWorkspace('project_1', 'run_1')
    assert.equal(workspace.environment.healthy, true)
    assert.equal(workspace.profile.state, 'ready')
    assert.equal(workspace.lease?.fencing_token, 3)

    await controlledExecutionApi.generatePlan('project_1', 'run_1')
    await controlledExecutionApi.heartbeatLease('project_1', 'run_1', workspace.lease!)
    await controlledExecutionApi.prepare('project_1', 'run_1')
    const confirmation = await controlledExecutionApi.confirm('project_1', run as never)
    await controlledExecutionApi.submit('project_1', run as never, workspace.lease!, confirmation)

    assert.equal(calls.find(call => call.url.endsWith(':plan'))?.method, 'POST')
    assert.match(calls.find(call => call.url.endsWith('/confirmations'))?.body ?? '', /"binding_hash":"a{64}"/)
    const submit = calls.find(call => call.url.endsWith(':submit'))
    assert.match(submit?.body ?? '', /"fencing_token":3/)
    assert.match(submit?.body ?? '', /"token":"memory-only-token"/)
    assert.match(submit?.body ?? '', /"step_id":"run_1-submit-v4"/)
    assert.match(submit?.body ?? '', /"idempotency_key":"run_1-submit-v4"/)
  } finally {
    globalThis.fetch = originalFetch
  }
})
