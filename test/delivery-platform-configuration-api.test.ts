import assert from 'node:assert/strict'
import test from 'node:test'
import { deliveryOptimizationApi } from '../src/api/delivery.ts'

const now = '2026-08-11T08:00:00.000Z'

test('platform recommendation endpoints preserve v2 snapshots and idempotency', async t => {
  const originalFetch = globalThis.fetch
  const calls: Array<{ url: string; init?: RequestInit }> = []
  globalThis.fetch = async (url, init) => {
    calls.push({ url: String(url), init })
    const path = String(url)
    if (path.endsWith('/recommendations')) return jsonResponse({ items: [recommendationPayload()], source: 'mock' })
    if (path.endsWith(':accept')) return jsonResponse({ recommendation: { ...recommendationPayload(), status: 'accepted' }, change_set: changeSetPayload() })
    if (path.endsWith(':reject')) return jsonResponse({ ...recommendationPayload(), status: 'rejected' })
    return jsonResponse(recommendationPayload())
  }
  t.after(() => { globalThis.fetch = originalFetch })

  const generated = await deliveryOptimizationApi.generateRecommendations('project_1', 'plan_1', 2)
  const listed = await deliveryOptimizationApi.listRecommendations('project_1')
  const accepted = await deliveryOptimizationApi.acceptRecommendation('project_1', generated.id, generated.version, 'idem-platform-v2')
  const rejected = await deliveryOptimizationApi.rejectRecommendation('project_1', generated.id, generated.version)

  assert.equal(generated.runtimeStatus, 'active')
  assert.equal(generated.targetConfiguration?.schema_version, 'delivery-platform-configuration/v2')
  assert.equal(listed.length, 1)
  assert.equal(new Headers(calls[2].init?.headers).get('Idempotency-Key'), 'idem-platform-v2')
  assert.deepEqual(JSON.parse(calls[2].init?.body as string), { expected_version: 1 })
  assert.equal(accepted.changeSet.targetSnapshot?.schema_version, 'delivery-platform-configuration/v2')
  assert.equal(rejected.status, 'rejected')
  assert.equal(calls.some(call => call.url.includes('manual-action-package')), false)
})

function jsonResponse(body: unknown) {
  return new Response(JSON.stringify(body), { headers: { 'Content-Type': 'application/json' } })
}

function configuration() {
  return {
    schema_version: 'delivery-platform-configuration/v2', configuration_id: 'configuration_1', version_number: 2,
    platform: 'ocean_engine', profile_version: 'oceanengine-configuration/v1', canonical_hash: 'a'.repeat(64),
    hash_algorithm: 'RFC8785-JCS-SHA256(canonical_payload)', intent: {}, payload: { profile: 'ocean_engine', ocean_engine: { profile: 'ocean_engine', project: {}, promotions: [] } },
    configuration_provenance: {}, fact_provenance: {}, compilation_metadata: {},
  }
}

function recommendationPayload() {
  return {
    id: 'recommendation_1', organization_id: 'org_1', project_id: 'project_1', plan_id: 'plan_1', plan_version: 2,
    simulation_run_id: 'simulation_1', fingerprint: 'fingerprint', base_snapshot_hash: 'a'.repeat(64), target_snapshot_hash: 'a'.repeat(64),
    base_configuration: configuration(), target_configuration: configuration(), runtime_status: 'active', read_only: false,
    evidence: ['simulation://run/1'], action: 'reduce_budget_10_percent', impact: 'reviewed budget reduction', risks: [], observation: 'measured evidence',
    provenance: 'post-launch-simulator/v1', status: 'proposed', version: 1, created_by: 'user_1', created_at: now, updated_at: now,
  }
}

function changeSetPayload() {
  return {
    id: 'changeset_1', organization_id: 'org_1', project_id: 'project_1', plan_id: 'plan_1', plan_name: 'Platform plan', plan_version: 2,
    plan_canonical_hash: 'a'.repeat(64), target_snapshot: configuration(), target_snapshot_hash: 'a'.repeat(64), runtime_status: 'active', read_only: false,
    budget_limit: { total_minor: 300000, currency: 'CNY' }, status: 'draft', risk_level: 'low', preflight_notes: [], source: 'mock', scenario: 'platform_configuration',
    version: 1, created_by: 'user_1', created_at: now, updated_at: now,
  }
}
