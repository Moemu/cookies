import { expect, test, type APIRequestContext } from '@playwright/test'

const projectId = 'project_investor_precision_evidence'
const otherProjectId = 'project_local'
const alertTypes = ['review_rejected', 'spend_spike', 'zero_conversion', 'cost_worsening'] as const

test('Delivery monitoring evaluates fixtures, preserves provenance, paginates, and resolves open alerts optimistically', async ({ request }) => {
  await createMonitoringMetricSnapshot(request)
  const normal = await evaluate(request, 'normal_day')
  expect(normal.status()).toBe(200)
  expect(await normal.json()).toMatchObject({
    items: [],
    source: 'demo_fixture',
    is_simulated: true,
    scenario: 'normal_day',
  })

  const anomalous = await evaluate(request, 'anomaly_day')
  expect(anomalous.status()).toBe(200)
  const anomaly = await anomalous.json() as AlertEvaluationResult
  expect(anomaly).toMatchObject({
    source: 'demo_fixture',
    is_simulated: true,
    scenario: 'anomaly_day',
  })
  expect(anomaly.created_count + anomaly.reused_count).toBe(anomaly.items.length)
  expect(anomaly.items.map(item => item.type).sort()).toEqual([...alertTypes].sort())

  for (const alert of anomaly.items) {
    expect(alert).toMatchObject({
      organization_id: expect.any(String),
      project_id: projectId,
      plan_id: expect.any(String),
      execution_id: expect.any(String),
      monitored_entity: { type: 'delivery_plan', id: alert.plan_id, advertiser_id: expect.any(String) },
      rule_id: expect.any(String),
      rule_version: 'v1',
      fingerprint: expect.any(String),
      status: 'open',
      version: expect.any(Number),
      source: 'demo_fixture',
      is_simulated: true,
      scenario: 'anomaly_day',
      dataset_version: 'preroll-demo/v1',
      fixture_version: expect.any(String),
      owner: { id: expect.any(String), display_name: expect.any(String), source: expect.any(String) },
      evidence_refs: expect.arrayContaining([expect.any(String)]),
      freshness: { status: 'fresh', as_of: expect.any(String), evaluated_at: expect.any(String) },
    })
    expect(alert.window).toMatchObject({ start: expect.any(String), end: expect.any(String), timezone: expect.any(String), data_through: expect.any(String) })
    expect(alert.metric_definition).toMatchObject({ name: expect.any(String), unit: expect.any(String) })
  }
  const byType = Object.fromEntries(anomaly.items.map(item => [item.type, item]))
  expect(byType.spend_spike.metric_definition).toMatchObject({ observed_value: 60000, baseline_value: 20000, threshold: 30000 })
  expect(byType.zero_conversion.metric_definition).toMatchObject({ observed_value: 0, numerator: 0, denominator: 400, threshold: 1 })
  expect(byType.cost_worsening.metric_definition).toMatchObject({ observed_value: 60000, numerator: 60000, denominator: 1, baseline_value: 1000, threshold: 1500 })

  const firstPage = await request.get(alertsURL('?status=open&fixture=anomaly_day&limit=2'))
  expect(firstPage.status()).toBe(200)
  const pageOne = await firstPage.json() as AlertList
  expect(pageOne).toMatchObject({ source: 'demo_fixture', is_simulated: true })
  expect(pageOne.items).toHaveLength(2)
  expect(pageOne.next_cursor).toEqual(expect.any(String))
  const secondPage = await request.get(alertsURL(`?status=open&fixture=anomaly_day&limit=2&cursor=${encodeURIComponent(pageOne.next_cursor!)}`))
  expect(secondPage.status()).toBe(200)
  const pageTwo = await secondPage.json() as AlertList
  expect(pageTwo.items).toHaveLength(2)
  expect(new Set([...pageOne.items, ...pageTwo.items].map(item => item.id)).size).toBe(4)

  const typed = await request.get(alertsURL('?type=spend_spike&severity=medium&limit=10'))
  expect(typed.status()).toBe(200)
  const typedBody = await typed.json() as AlertList
  expect(typedBody.items).toEqual(expect.arrayContaining([
    expect.objectContaining({ type: 'spend_spike', severity: 'medium' }),
  ]))

  const toAcknowledge = anomaly.items[0]
  const isolatedRead = await request.get(`/api/delivery/v1/projects/${otherProjectId}/alerts?limit=10`)
  expect(isolatedRead.status()).toBe(200)
  expect((await isolatedRead.json() as AlertList).items.map(item => item.id)).not.toContain(toAcknowledge.id)
  const isolatedEvaluation = await request.post(`/api/delivery/v1/projects/${otherProjectId}/alerts:evaluate`, { data: { fixture: 'anomaly_day' } })
  expect(isolatedEvaluation.status()).toBe(200)
  expect((await isolatedEvaluation.json() as AlertEvaluationResult).items).toEqual([])
  const crossProjectPatch = await request.patch(`/api/delivery/v1/projects/${otherProjectId}/alerts/${toAcknowledge.id}`, { data: { action: 'dismiss', expected_version: toAcknowledge.version } })
  expect(crossProjectPatch.status()).toBe(404)
  expect(await crossProjectPatch.json()).toMatchObject({ error: { code: 'RESOURCE_NOT_FOUND' } })
  const acknowledged = await update(request, toAcknowledge.id, 'acknowledge', toAcknowledge.version)
  expect(acknowledged.status()).toBe(200)
  const acknowledgedBody = await acknowledged.json() as DeliveryMonitoringAlert
  expect(acknowledgedBody).toMatchObject({ id: toAcknowledge.id, status: 'acknowledged', version: toAcknowledge.version + 1 })

  const staleUpdate = await update(request, toAcknowledge.id, 'dismiss', toAcknowledge.version)
  expect(staleUpdate.status()).toBe(409)
  expect(await staleUpdate.json()).toMatchObject({ error: { code: 'VERSION_CONFLICT' } })

  const toDismiss = anomaly.items[1]
  const dismissed = await update(request, toDismiss.id, 'dismiss', toDismiss.version)
  expect(dismissed.status()).toBe(200)
  expect(await dismissed.json()).toMatchObject({ id: toDismiss.id, status: 'dismissed' })

  for (const fixture of ['stale_data', 'insufficient_data'] as const) {
    const response = await evaluate(request, fixture)
    expect(response.status()).toBe(200)
    const result = await response.json() as AlertEvaluationResult
    expect(result).toMatchObject({ source: 'demo_fixture', is_simulated: true, scenario: fixture })
    // Freshness-only fixtures never manufacture a fifth alert class. They leave
    // the four alert classifications reserved for actionable anomaly evidence.
    expect(result.items).toEqual([])
  }
})

function evaluate(request: APIRequestContext, fixture: MonitoringFixture) {
  return request.post(`/api/delivery/v1/projects/${projectId}/alerts:evaluate`, { data: { fixture } })
}

function update(request: APIRequestContext, alertId: string, action: 'acknowledge' | 'dismiss', expectedVersion: number) {
  return request.patch(`/api/delivery/v1/projects/${projectId}/alerts/${alertId}`, { data: { action, expected_version: expectedVersion } })
}

function alertsURL(query = '') {
  return `/api/delivery/v1/projects/${projectId}/alerts${query}`
}

async function createMonitoringMetricSnapshot(request: APIRequestContext) {
  const suffix = `monitoring-${Date.now().toString(36)}`
  const planResponse = await request.post(`/api/delivery/v1/projects/${projectId}/plans`, {
    data: {
      name: `Monitoring E2E ${suffix}`,
      objective: 'Provide deterministic evidence for monitoring alert evaluation',
      advertiser: { id: 'mock-advertiser-001', name: 'Cookies Mock Advertiser', platform: 'ocean_engine' },
      budget: { total_minor: 300000, currency: 'CNY' },
      schedule: { start_at: '2026-08-01T00:00:00Z', end_at: '2026-08-31T00:00:00Z', timezone: 'Asia/Shanghai' },
      tracking: { landing_page: 'https://demo.cookies.local', pixel_id: `PX-${suffix}`, conversion_event: 'lead_submit' },
      creative_references: [{ asset_id: `asset-${suffix}`, version: 1, confirmed: true }],
      source_strategy_version: 'strategy-v1',
    },
  })
  expect(planResponse.status()).toBe(201)
  const plan = await planResponse.json() as { id: string; version: number }

  const changeSetResponse = await request.post(`/api/delivery/v1/projects/${projectId}/plans/${plan.id}:create-change-set`, { data: { expected_version: plan.version } })
  expect(changeSetResponse.status()).toBe(201)
  let changeSet = await changeSetResponse.json() as { id: string; version: number }
  const preflight = await request.post(`/api/delivery/v1/projects/${projectId}/change-sets/${changeSet.id}:preflight`, { data: { expected_version: changeSet.version } })
  expect(preflight.status()).toBe(200)
  changeSet = await preflight.json() as { id: string; version: number }
  const approval = await request.post(`/api/delivery/v1/projects/${projectId}/change-sets/${changeSet.id}:approve`, { data: { expected_version: changeSet.version } })
  expect(approval.status()).toBe(200)
  changeSet = await approval.json() as { id: string; version: number }
  const execution = await request.post(`/api/delivery/v1/projects/${projectId}/change-sets/${changeSet.id}:execute`, {
    headers: { 'Idempotency-Key': `monitoring-${suffix}` },
    data: { expected_version: changeSet.version, scenario: 'success' },
  })
  expect(execution.status()).toBe(201)
  const executionBody = await execution.json() as { execution: { id: string } }
  const metric = await request.post(`/api/delivery/v1/projects/${projectId}/executions/${executionBody.execution.id}/metric-snapshots`, {
    data: { dataset_version: 'preroll-demo/v1' },
  })
  expect(metric.status()).toBe(201)
}

type MonitoringFixture = 'normal_day' | 'anomaly_day' | 'stale_data' | 'insufficient_data'

type DeliveryMonitoringAlert = {
  id: string
  type: typeof alertTypes[number]
  plan_id: string
  version: number
  status: 'open' | 'acknowledged' | 'dismissed'
  severity: 'critical' | 'high' | 'medium' | 'low'
  window: { start: string; end: string; timezone: string; data_through: string }
  metric_definition: { name: string; unit: string; observed_value?: number; baseline_value?: number; threshold?: number; numerator?: number; denominator?: number }
  freshness: { status: 'fresh' | 'stale' | 'unknown' | 'insufficient_data'; as_of: string; evaluated_at: string }
}

type AlertEvaluationResult = {
  items: DeliveryMonitoringAlert[]
  created_count: number
  reused_count: number
}

type AlertList = { items: DeliveryMonitoringAlert[]; next_cursor: string | null }
