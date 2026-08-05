import { expect, test, type APIRequestContext } from '@playwright/test'

const projectId = 'project_investor_precision_evidence'
const otherProjectId = 'project_local'

test('three-tier mock configuration, recommendation decision, approval, and manual package remain immutable and project-scoped', async ({ page, request }) => {
  const suffix = Date.now().toString(36)
  const created = await createPlan(request, `Three-tier delivery ${suffix}`)
  expect(created.current_version.three_tier_configuration).toBeUndefined()

  const compiled = await compile(request, created.id, created.version, 'golden_path')
  const snapshot = compiled.current_version.three_tier_configuration
  expect(snapshot).toBeDefined()
  if (!snapshot) throw new Error('golden compile did not return a three-tier snapshot')
  expect(snapshot).toMatchObject({
    schema: 'delivery-three-tier/v1',
    source: 'mock',
    scenario: 'golden_path',
    fixture_scenario: 'golden_path',
    generated_at: expect.any(String),
    evidence: expect.arrayContaining([expect.any(String)]),
  })
  expect(snapshot.groups).toHaveLength(1)
  expect(snapshot.groups[0].plans).toHaveLength(2)
  expect(snapshot.groups[0].plans.flatMap((plan: ThreeTierPlan) => plan.creatives)).toHaveLength(3)
  const editableTitle = snapshot.groups[0].plans[0].creatives[0].fields.find(field => field.key === 'title')
  expect(editableTitle).toMatchObject({
    key: 'title',
    recommended: { type: 'string' },
    effective: { type: 'string' },
    source: 'mock_fixture',
    evidence_refs: expect.arrayContaining([expect.any(String)]),
    mock_required: true,
    platform_status: 'not_requested',
    confirmation: true,
  })

  const opened = await request.get(planURL(created.id))
  expect(opened.status()).toBe(200)
  expect(await opened.json()).toMatchObject({
    id: created.id,
    current_version: { canonical_hash: compiled.current_version.canonical_hash, three_tier_configuration: snapshot },
  })

  const overridden = await request.post(`${planURL(created.id)}/configuration:override`, {
    data: {
      expected_version: compiled.version,
      group_id: 'group_1',
      plan_id: 'plan_1',
      creative_id: 'creative_1',
      field_key: 'title',
      value: { type: 'string', value: '人工确认后的项目标题' },
      confirmed: true,
    },
  })
  expect(overridden.status()).toBe(200)
  const overriddenPlan = await overridden.json() as DeliveryPlan
  expect(overriddenPlan.version).toBe(compiled.version + 1)
  expect(overriddenPlan.current_version.canonical_hash).not.toBe(compiled.current_version.canonical_hash)
  expect(overriddenPlan.current_version.three_tier_configuration.groups[0].plans[0].creatives[0].fields.find(field => field.key === 'title')).toMatchObject({
    key: 'title',
    manual: { type: 'string', value: '人工确认后的项目标题' },
    effective: { type: 'string', value: '人工确认后的项目标题' },
    confirmation: true,
  })

  const preflight = await request.post(`${planURL(created.id)}/preflight`)
  expect(preflight.status()).toBe(200)
  expect(await preflight.json()).toMatchObject({
    source: 'mock',
    scenario: 'golden_path',
    passed: true,
    blocked: false,
    checks: expect.arrayContaining([
      expect.objectContaining({ code: 'three_tier_structure', passed: true }),
      expect.objectContaining({ code: 'three_tier_required_fields', passed: true }),
      expect.objectContaining({ code: 'three_tier_dependencies', passed: true }),
      expect.objectContaining({ code: 'three_tier_confirmation', passed: true }),
    ]),
  })

  const executionsBefore = await request.get(`/api/delivery/v1/projects/${projectId}/executions`)
  expect(executionsBefore.status()).toBe(200)
  const executionIDsBefore = (await executionsBefore.json() as { items: Array<{ id: string }> }).items.map(item => item.id)

  const generated = await request.post(`${planURL(created.id)}/recommendations:generate`, {
    data: { expected_version: overriddenPlan.version },
  })
  expect(generated.status()).toBe(201)
  const recommendation = await generated.json() as Recommendation
  expect(recommendation).toMatchObject({
    project_id: projectId,
    plan_id: created.id,
    plan_version: overriddenPlan.version,
    status: 'proposed',
    fingerprint: expect.any(String),
    base_snapshot_hash: expect.any(String),
    base_snapshot: { schema: 'delivery-three-tier/v1', source: 'mock', scenario: 'golden_path' },
    target_snapshot_hash: expect.any(String),
    target_snapshot: { schema: 'delivery-three-tier/v1', source: 'mock', scenario: 'golden_path' },
    evidence: expect.arrayContaining([expect.any(String)]),
    provenance: 'plan_version',
  })

  const listed = await request.get(`/api/delivery/v1/projects/${projectId}/recommendations?limit=10`)
  expect(listed.status()).toBe(200)
  expect(await listed.json()).toMatchObject({ source: 'mock', items: expect.arrayContaining([expect.objectContaining({ id: recommendation.id })]) })
  const fetchedRecommendation = await request.get(`/api/delivery/v1/projects/${projectId}/recommendations/${recommendation.id}`)
  expect(fetchedRecommendation.status()).toBe(200)
  expect(await fetchedRecommendation.json()).toMatchObject({ id: recommendation.id, status: 'proposed' })

  const acceptKey = `three-tier-accept-${suffix}`
  const accepted = await request.post(`/api/delivery/v1/projects/${projectId}/recommendations/${recommendation.id}:accept`, {
    headers: { 'Idempotency-Key': acceptKey },
    data: { expected_version: recommendation.version },
  })
  expect(accepted.status()).toBe(201)
  const decision = await accepted.json() as RecommendationDecision
  expect(decision).toMatchObject({
    recommendation: { id: recommendation.id, status: 'accepted', version: recommendation.version + 1 },
    change_set: {
      project_id: projectId,
      plan_id: created.id,
      status: 'draft',
      recommendation_id: recommendation.id,
      target_snapshot_hash: recommendation.target_snapshot_hash,
      target_snapshot: recommendation.target_snapshot,
      source: 'mock',
      scenario: 'golden_path',
    },
  })

  const replay = await request.post(`/api/delivery/v1/projects/${projectId}/recommendations/${recommendation.id}:accept`, {
    headers: { 'Idempotency-Key': acceptKey },
    data: { expected_version: recommendation.version },
  })
  expect(replay.status()).toBe(200)
  expect(await replay.json()).toMatchObject({ change_set: { id: decision.change_set.id }, recommendation: { id: recommendation.id, status: 'accepted' } })

  const unchangedPlan = await request.get(planURL(created.id))
  expect(unchangedPlan.status()).toBe(200)
  expect(await unchangedPlan.json()).toMatchObject({ version: overriddenPlan.version, current_version: { canonical_hash: overriddenPlan.current_version.canonical_hash } })
  const executionsAfter = await request.get(`/api/delivery/v1/projects/${projectId}/executions`)
  expect(executionsAfter.status()).toBe(200)
  expect((await executionsAfter.json() as { items: Array<{ id: string }> }).items.map(item => item.id)).toEqual(executionIDsBefore)

  const changeSetPreflight = await request.post(`/api/delivery/v1/projects/${projectId}/change-sets/${decision.change_set.id}:preflight`, {
    data: { expected_version: decision.change_set.version },
  })
  expect(changeSetPreflight.status()).toBe(200)
  const preflightChangeSet = await changeSetPreflight.json() as ChangeSet
  expect(preflightChangeSet.status).toBe('preflight_passed')
  const approved = await request.post(`/api/delivery/v1/projects/${projectId}/change-sets/${decision.change_set.id}:approve`, {
    data: { expected_version: preflightChangeSet.version },
  })
  expect(approved.status()).toBe(200)
  const approvedChangeSet = await approved.json() as ChangeSet
  expect(approvedChangeSet).toMatchObject({
    status: 'approved',
    approval: { target_snapshot_hash: recommendation.target_snapshot_hash, valid: true, source: 'mock' },
  })

  const packageResponse = await request.post(`/api/delivery/v1/projects/${projectId}/change-sets/${decision.change_set.id}/manual-action-package`, {
    data: { expected_version: approvedChangeSet.version },
  })
  expect(packageResponse.status()).toBe(201)
  const actionPackage = await packageResponse.json() as ManualActionPackage
  expect(actionPackage).toMatchObject({
    project_id: projectId,
    change_set_id: decision.change_set.id,
    target_snapshot_hash: recommendation.target_snapshot_hash,
    forbidden_actions: expect.arrayContaining(['platform_api_call', 'automatic_execution']),
    evidence: expect.arrayContaining([expect.any(String)]),
    provenance: 'approved_change_set',
  })
  expect(actionPackage.instructions).toEqual(expect.arrayContaining([
    expect.objectContaining({ group_id: 'group_1', plan_id: expect.any(String), creative_id: expect.any(String), field_key: expect.any(String), source: 'mock_fixture', expected_result: expect.any(String), evidence_refs: expect.arrayContaining([expect.any(String)]) }),
  ]))
  const packageReplay = await request.post(`/api/delivery/v1/projects/${projectId}/change-sets/${decision.change_set.id}/manual-action-package`, {
    data: { expected_version: approvedChangeSet.version },
  })
  expect(packageReplay.status()).toBe(200)
  expect(await packageReplay.json()).toMatchObject({ id: actionPackage.id, content_hash: actionPackage.content_hash })
  const refreshedPackage = await request.get(`/api/delivery/v1/projects/${projectId}/change-sets/${decision.change_set.id}/manual-action-package`)
  expect(refreshedPackage.status()).toBe(200)
  expect(await refreshedPackage.json()).toMatchObject({ id: actionPackage.id, source: 'mock' })

  await page.goto(`/projects/${projectId}/delivery/three-tier?view=${encodeURIComponent('建议与人工操作包')}&plan_id=${created.id}`)
  const packagePanel = page.locator('.delivery-config-package')
  await expect(packagePanel.getByText('操作包已就绪', { exact: true })).toBeVisible()
  await expect(page.getByRole('region', { name: '人工执行安全边界' })).toContainText('以下操作不在授权范围内，请勿执行')
  await expect(page.getByRole('region', { name: '待人工填写清单' })).toContainText('按广告组、计划、创意顺序')
  await expect(packagePanel.getByText('source=mock', { exact: false })).toHaveCount(0)
  await expect(packagePanel.getByText('manual_action_package', { exact: false })).toHaveCount(0)
  await expect(page.locator('.delivery-config-config-card')).toHaveCount(0)
  await expect(page.getByRole('heading', { name: '预检与审批' })).toHaveCount(0)
  await expect(page.locator('.delivery-config-flow-grid--decision > article > header > span')).toHaveCount(0)

  await page.getByRole('tab', { name: '预检与审批' }).click()
  await expect(page.getByRole('heading', { name: '预检与审批' })).toBeVisible()
  await expect(page.getByRole('heading', { name: '建议只待决策' })).toHaveCount(0)
  await expect(page.locator('.delivery-config-config-card')).toHaveCount(0)
  await expect(page.locator('.delivery-config-flow-grid--preflight > article > header').getByText('1', { exact: true })).toHaveCount(0)
  await expect(page.locator('.delivery-config-preflight-actions > button')).toHaveCount(3)
  await expect.poll(() => new URL(page.url()).searchParams.get('plan_id')).toBe(created.id)

  await page.getByRole('tab', { name: '模拟配置' }).click()
  await expect(page.locator('.delivery-config-config-card')).toBeVisible()
  await expect(page.locator('.delivery-config-flow-grid')).toHaveCount(0)
  await expect.poll(() => new URL(page.url()).searchParams.get('plan_id')).toBe(created.id)

  const crossProjectPlan = await request.get(`/api/delivery/v1/projects/${otherProjectId}/plans/${created.id}`)
  expect(crossProjectPlan.status()).toBe(404)
  const crossProjectRecommendation = await request.get(`/api/delivery/v1/projects/${otherProjectId}/recommendations/${recommendation.id}`)
  expect(crossProjectRecommendation.status()).toBe(404)
  const crossProjectPackage = await request.get(`/api/delivery/v1/projects/${otherProjectId}/change-sets/${decision.change_set.id}/manual-action-package`)
  expect(crossProjectPackage.status()).toBe(404)
})

test('server fixtures expose required, dependency, confirmation, and platform-pending preflight checks', async ({ request }) => {
  for (const fixture of ['missing_required_field', 'orphan_dependency', 'missing_confirmation', 'platform_fields_pending'] as const) {
    const plan = await createPlan(request, `Three-tier ${fixture} ${Date.now().toString(36)}`)
    const compiled = await compile(request, plan.id, plan.version, fixture)
    const preflight = await request.post(`${planURL(plan.id)}/preflight`)
    expect(preflight.status(), fixture).toBe(200)
    const body = await preflight.json() as { source: string; checks: Array<{ code: string; passed: boolean }> }
    expect(body.source).toBe('mock')
    const failedCodes = body.checks.filter(check => !check.passed).map(check => check.code)
    if (fixture === 'missing_required_field') expect(failedCodes).toContain('three_tier_required_fields')
    if (fixture === 'orphan_dependency') expect(failedCodes).toContain('three_tier_dependencies')
    if (fixture === 'missing_confirmation') expect(failedCodes).toContain('three_tier_confirmation')
    if (fixture === 'platform_fields_pending') expect(failedCodes).toContain('three_tier_platform_pending')
    expect(compiled.current_version.three_tier_configuration?.source).toBe('mock')
  }
})

test('manual override opens as a centered modal above the fixed status bar', async ({ page, request }) => {
  const viewport = { width: 1311, height: 912 }
  const created = await createPlan(request, `Three-tier override dialog ${Date.now().toString(36)}`)
  await compile(request, created.id, created.version, 'golden_path')

  await page.setViewportSize(viewport)
  await page.goto(`/projects/${projectId}/delivery/three-tier?plan_id=${created.id}`)
  const landingPageField = page.locator('.delivery-config-field').filter({ hasText: '落地页' }).first()
  await landingPageField.getByRole('button', { name: '人工覆盖' }).click()

  const dialog = page.getByRole('dialog', { name: '人工覆盖：落地页' })
  await expect(dialog).toBeVisible()
  await expect(page.locator('.delivery-config-override-backdrop')).toHaveCSS('position', 'fixed')
  await expect(page.locator('.delivery-config-override-backdrop')).toHaveCSS('z-index', '40')
  await expect(page.locator('body')).toHaveCSS('overflow', 'hidden')
  const box = await dialog.boundingBox()
  expect(box).not.toBeNull()
  expect(Math.abs(box!.x + box!.width / 2 - viewport.width / 2)).toBeLessThan(12)
  expect(Math.abs(box!.y + box!.height / 2 - viewport.height / 2)).toBeLessThan(3)
  expect(box!.y).toBeGreaterThanOrEqual(24)
  expect(box!.y + box!.height).toBeLessThanOrEqual(viewport.height - 24)

  await page.keyboard.press('Escape')
  await expect(dialog).toHaveCount(0)
  expect(await page.locator('body').evaluate(element => element.style.overflow)).toBe('')
})

async function createPlan(request: APIRequestContext, name: string): Promise<DeliveryPlan> {
  const response = await request.post(`/api/delivery/v1/projects/${projectId}/plans`, {
    data: {
      name,
      objective: 'Deterministic three-tier mock contract coverage',
      advertiser: { id: 'mock-advertiser-001', name: 'Mock advertiser', platform: 'ocean_engine' },
      budget: { total_minor: 300000, currency: 'CNY' },
      schedule: { start_at: '2026-08-04T00:00:00Z', end_at: '2026-08-11T00:00:00Z', timezone: 'Asia/Shanghai' },
      tracking: { landing_page: 'https://example.test/three-tier', pixel_id: 'PX-THREE-TIER', conversion_event: 'submit' },
      creative_references: [{ asset_id: 'asset-three-tier', version: 1, confirmed: true }],
      source_strategy_version: 'strategy-three-tier/v1',
    },
  })
  expect(response.status()).toBe(201)
  return response.json() as Promise<DeliveryPlan>
}

async function compile(request: APIRequestContext, planId: string, expectedVersion: number, fixture: Fixture): Promise<DeliveryPlan> {
  const response = await request.post(`${planURL(planId)}/configuration:compile`, { data: { expected_version: expectedVersion, fixture } })
  expect(response.status(), fixture).toBe(201)
  return response.json() as Promise<DeliveryPlan>
}

function planURL(planId: string) {
  return `/api/delivery/v1/projects/${projectId}/plans/${planId}`
}

type Fixture = 'golden_path' | 'missing_required_field' | 'orphan_dependency' | 'missing_confirmation' | 'platform_fields_pending'
type ThreeTierField = { key: string; recommended: { type: string; value: unknown }; effective: { type: string; value: unknown }; manual?: { type: string; value: unknown }; source: string; evidence_refs: string[]; mock_required: boolean; platform_status: string; confirmation: boolean }
type ThreeTierPlan = { id: string; creatives: Array<{ id: string; fields: ThreeTierField[] }> }
type ThreeTierConfiguration = { schema: string; source: string; scenario: string; fixture_scenario: string; generated_at: string; evidence: string[]; groups: Array<{ id: string; plans: ThreeTierPlan[] }> }
type DeliveryPlan = { id: string; version: number; current_version: { canonical_hash: string; three_tier_configuration?: ThreeTierConfiguration } }
type Recommendation = { id: string; version: number; status: string; target_snapshot_hash: string; target_snapshot: ThreeTierConfiguration }
type ChangeSet = { id: string; version: number; status: string; approval?: { target_snapshot_hash?: string; valid: boolean; source: string } }
type RecommendationDecision = { recommendation: Recommendation; change_set: ChangeSet }
type ManualActionPackage = { id: string; content_hash: string; instructions: Array<Record<string, unknown>>; forbidden_actions: string[]; evidence: string[]; provenance: string }
