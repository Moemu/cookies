import assert from 'node:assert/strict'
import test from 'node:test'
import { deliveryAlertApi } from '../src/api/delivery.ts'

test('delivery alert client uses only project-scoped evaluate, list, and PATCH actions', async t => {
  const originalFetch = globalThis.fetch
  const calls: Array<{ url: string; init?: RequestInit }> = []
  globalThis.fetch = async (url, init) => {
    calls.push({ url: String(url), init })
    if (String(url).endsWith('alerts:evaluate')) return jsonResponse({ items: [sampleAlert()], created_count: 1, reused_count: 0, source: 'demo_fixture', is_simulated: true, scenario: 'anomaly_day', evaluated_at: now })
    if (String(url).endsWith('/alerts')) return jsonResponse({ items: [sampleAlert()], next_cursor: 'page-2', source: 'demo_fixture', is_simulated: true })
    if (String(url).includes('cursor=page-2')) return jsonResponse({ items: [{ ...sampleAlert(), id: 'alert_2' }], next_cursor: null, source: 'demo_fixture', is_simulated: true })
    return jsonResponse({ ...sampleAlert(), status: 'acknowledged', version: 2 })
  }
  t.after(() => { globalThis.fetch = originalFetch })

  const evaluated = await deliveryAlertApi.evaluate('project one', 'anomaly_day')
  const listed = await deliveryAlertApi.list('project one')
  const updated = await deliveryAlertApi.action('project one', 'alert/1', 'acknowledge', 1)

  assert.equal(calls[0].url, '/api/delivery/v1/projects/project%20one/alerts:evaluate')
  assert.deepEqual(JSON.parse(calls[0].init?.body as string), { fixture: 'anomaly_day' })
  assert.equal(calls[1].url, '/api/delivery/v1/projects/project%20one/alerts')
  assert.equal(calls[2].url, '/api/delivery/v1/projects/project%20one/alerts?cursor=page-2')
  assert.equal(calls[3].url, '/api/delivery/v1/projects/project%20one/alerts/alert%2F1')
  assert.equal(calls[3].init?.method, 'PATCH')
  assert.deepEqual(JSON.parse(calls[3].init?.body as string), { action: 'acknowledge', expected_version: 1 })
  assert.equal(evaluated.items[0].window.dataThrough, now)
  assert.equal(listed.length, 2)
  assert.equal(listed[0].owner.displayName, '投手 A')
  assert.equal(updated.status, 'acknowledged')
})

const now = '2026-08-04T08:00:00.000Z'
function jsonResponse(body: unknown) { return new Response(JSON.stringify(body), { headers: { 'Content-Type': 'application/json' } }) }
function sampleAlert() {
  return {
    id: 'alert_1', organization_id: 'org_1', project_id: 'project one', plan_id: 'plan_1', execution_id: 'exec_1',
    monitored_entity: { type: 'delivery_plan', id: 'plan_1', advertiser_id: 'adv_1' }, type: 'spend_spike', rule_id: 'spend_spike', rule_version: 'v1', fingerprint: 'fingerprint', severity: 'high', status: 'open', version: 1,
    window: { start: '2026-08-03T00:00:00.000Z', end: now, timezone: 'Asia/Shanghai', data_through: now },
    metric_definition: { name: 'Spend', unit: 'CNY', observed_value: 1300, baseline_value: 500, threshold: 1000 },
    owner: { id: 'user_1', display_name: '投手 A', source: 'project_owner' }, evidence_refs: ['fixture://alert/1'], source: 'demo_fixture', is_simulated: true, scenario: 'anomaly_day', dataset_version: 'delivery-demo/v1', fixture_version: 'v1', freshness: { status: 'fresh', as_of: now, evaluated_at: now, age_seconds: 0, max_age_seconds: 3600 }, created_by: 'system', created_at: now, updated_at: now,
  }
}
