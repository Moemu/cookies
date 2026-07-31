import { expect, test, type Page } from '@playwright/test'

const primaryProjectId = 'project_investor_precision_evidence'
const otherProjectId = 'project_local'

test('DeliveryPlan 草稿生命周期、Project 隔离与服务端权威预检', async ({ page, request }) => {
  const suffix = Date.now().toString(36)
  const goldenName = `E2E 黄金计划 ${suffix}`

  await page.goto(`/projects/${primaryProjectId}/delivery/plans`)
  await expect(page.getByRole('heading', { name: '计划草稿' })).toBeVisible()
  const workspace = page.locator('.delivery-lifecycle-workspace')
  const workspaceBox = await workspace.boundingBox()
  expect(workspaceBox?.height).toBeLessThanOrEqual(720)
  await expect(page.locator('.delivery-plan-scroll')).toHaveCSS('overflow-y', 'auto')
  await expect(page.locator('.delivery-plan-form')).toHaveCSS('overflow-y', 'auto')
  const blankDraftButton = page.getByRole('button', { name: '新建空白草稿', exact: true })
  await expect(blankDraftButton.locator('svg.lucide-file-plus')).toBeVisible()
  await page.getByLabel('计划名称').fill(`不应继承 ${suffix}`)
  await blankDraftButton.click()
  await expect(page.getByLabel('计划名称')).not.toHaveValue(`不应继承 ${suffix}`)
  await startNewPlan(page, goldenName)
  const targetFieldControlOffsets = await page.locator('.delivery-field-grid > label').evaluateAll(labels =>
    labels.map(label => {
      const control = label.querySelector('input, textarea, select')
      if (!control) throw new Error('delivery field label is missing its control')
      return Math.round(control.getBoundingClientRect().top - label.getBoundingClientRect().top)
    }),
  )
  expect(Math.max(...targetFieldControlOffsets) - Math.min(...targetFieldControlOffsets)).toBeLessThanOrEqual(1)
  await expect(page.getByRole('button', { name: '保存草稿', exact: true })).toBeInViewport()
  await expect(page.getByText('source=mock').first()).toBeVisible()

  await page.getByRole('button', { name: '预算与排期', exact: true }).click()
  await page.getByLabel('总预算').fill('3000')
  await page.getByRole('button', { name: '追踪', exact: true }).click()
  await page.getByLabel('追踪像素 ID').fill(`PX-E2E-${suffix}`)
  await page.getByRole('button', { name: '素材引用', exact: true }).click()
  await page.getByLabel('素材 Asset ID').fill(`asset-e2e-${suffix}`)

  const createResponsePromise = page.waitForResponse(response =>
    response.request().method() === 'POST' && new URL(response.url()).pathname === `/api/delivery/v1/projects/${primaryProjectId}/plans`,
  )
  await page.getByRole('button', { name: '保存草稿', exact: true }).click()
  const createResponse = await createResponsePromise
  expect(createResponse.status()).toBe(201)
  const created = await createResponse.json() as { id: string; source: string; scenario: string; current_version_number: number }
  expect(created).toMatchObject({ source: 'mock', scenario: 'golden_path', current_version_number: 1 })
  const planId = created.id
  await expect(page.getByText(/已保存为 V1/)).toBeVisible()

  await page.reload()
  await expect(page.getByText(goldenName).first()).toBeVisible()
  await page.getByRole('button').filter({ hasText: goldenName }).first().click()
  await page.getByRole('button', { name: '预算与排期', exact: true }).click()
  await page.getByLabel('总预算').fill('4200')
  const updateResponsePromise = page.waitForResponse(response =>
    response.request().method() === 'PATCH' && new URL(response.url()).pathname === `/api/delivery/v1/projects/${primaryProjectId}/plans/${planId}`,
  )
  await page.getByRole('button', { name: '保存新版本', exact: true }).click()
  const updateResponse = await updateResponsePromise
  expect(updateResponse.status()).toBe(200)
  const updated = await updateResponse.json() as { current_version_number: number; versions: Array<{ version_number: number; budget: { total_minor: number } }> }
  expect(updated.current_version_number).toBe(2)
  expect(updated.versions).toEqual(expect.arrayContaining([
    expect.objectContaining({ version_number: 1, budget: { total_minor: 300000, currency: 'CNY' } }),
    expect.objectContaining({ version_number: 2, budget: { total_minor: 420000, currency: 'CNY' } }),
  ]))
  await expect(page.getByRole('button', { name: '查看版本 V1' })).toBeVisible()
  await expect(page.getByRole('button', { name: '查看版本 V2' })).toBeVisible()
  await page.getByRole('button', { name: '查看版本 V1' }).click()
  await expect(page.getByText('历史快照 · V1')).toBeVisible()
  await expect(page.locator('.version-snapshot').getByText('¥3,000.00')).toBeVisible()

  const crossProject = await request.get(`/api/delivery/v1/projects/${otherProjectId}/plans/${planId}`)
  expect(crossProject.status()).toBe(404)
  expect(await crossProject.json()).toMatchObject({ error: { code: 'RESOURCE_NOT_FOUND' } })
  await page.goto(`/projects/${otherProjectId}/delivery/plans`)
  await expect(page.getByText(goldenName)).toHaveCount(0)

  await page.goto(`/projects/${primaryProjectId}/delivery/plans`)
  await expect(page.getByText(goldenName).first()).toBeVisible()
  await page.getByRole('button').filter({ hasText: goldenName }).first().click()
  const goldenPreflightPromise = waitForPreflight(page, planId)
  await page.getByRole('button', { name: '运行服务端预检', exact: true }).click()
  const goldenPreflight = await goldenPreflightPromise
  expect(goldenPreflight).toMatchObject({ source: 'mock', scenario: 'golden_path', passed: true, blocked: false })
  await expect(page.getByText('黄金场景全部通过，可继续后续受控流程。')).toBeVisible()

  const budgetPlanId = await createScenarioPlan(page, `E2E 预算零计划 ${suffix}`, async () => {
    await page.getByRole('button', { name: '预算与排期', exact: true }).click()
    await page.getByLabel('总预算').fill('0')
  })
  const budgetPreflightPromise = waitForPreflight(page, budgetPlanId)
  await page.getByRole('button', { name: '运行服务端预检', exact: true }).click()
  const budgetPreflight = await budgetPreflightPromise
  expect(budgetPreflight).toMatchObject({ source: 'mock', scenario: 'budget_zero', blocked: true })
  await expect(page.getByText('服务端预检阻断', { exact: true })).toBeVisible()
  await expect(page.getByText('error', { exact: true })).toBeVisible()
  await page.getByRole('button', { name: '修复 budget_positive' }).click()
  await expect(page.getByLabel('总预算')).toBeFocused()

  const creativePlanId = await createScenarioPlan(page, `E2E 素材警告计划 ${suffix}`, async () => {
    await page.getByRole('button', { name: '素材引用', exact: true }).click()
    await page.getByLabel('素材已人工确认').uncheck()
  })
  const creativePreflightPromise = waitForPreflight(page, creativePlanId)
  await page.getByRole('button', { name: '运行服务端预检', exact: true }).click()
  const creativePreflight = await creativePreflightPromise
  expect(creativePreflight).toMatchObject({ source: 'mock', scenario: 'creative_unconfirmed', passed: true, blocked: false })
  await expect(page.getByText('warning', { exact: true })).toBeVisible()
  await page.getByRole('button', { name: '修复 creative_confirmed' }).click()
  await expect(page.getByLabel('素材已人工确认')).toBeFocused()

  const trackingPlanId = await createScenarioPlan(page, `E2E 追踪缺失计划 ${suffix}`, async () => {
    await page.getByRole('button', { name: '追踪', exact: true }).click()
    await page.getByLabel('追踪像素 ID').fill('')
  })
  const trackingPreflightPromise = waitForPreflight(page, trackingPlanId)
  await page.getByRole('button', { name: '运行服务端预检', exact: true }).click()
  const trackingPreflight = await trackingPreflightPromise
  expect(trackingPreflight).toMatchObject({ source: 'mock', scenario: 'tracking_missing', blocked: true })
  await page.getByRole('button', { name: '修复 tracking_complete' }).click()
  await expect(page.getByLabel('追踪像素 ID')).toBeFocused()
})

async function startNewPlan(page: Page, name: string) {
  await page.getByRole('button', { name: '新建 mock 投放计划' }).click()
  await page.getByLabel('计划名称').fill(name)
  await page.getByLabel('业务目标').fill('获取高质量销售线索并验证投前门禁')
  await page.getByLabel('Mock 广告主').selectOption('mock-advertiser-001')
}

async function createScenarioPlan(page: Page, name: string, configure: () => Promise<void>) {
  await startNewPlan(page, name)
  await configure()
  const createResponsePromise = page.waitForResponse(response =>
    response.request().method() === 'POST' && new URL(response.url()).pathname === `/api/delivery/v1/projects/${primaryProjectId}/plans`,
  )
  await page.getByRole('button', { name: '保存草稿', exact: true }).click()
  const response = await createResponsePromise
  expect(response.status()).toBe(201)
  const plan = await response.json() as { id: string; source: string; scenario: string }
  expect(plan.source).toBe('mock')
  return plan.id
}

async function waitForPreflight(page: Page, planId: string) {
  const response = await page.waitForResponse(candidate =>
    candidate.request().method() === 'POST' && new URL(candidate.url()).pathname === `/api/delivery/v1/projects/${primaryProjectId}/plans/${planId}/preflight`,
  )
  expect(response.status()).toBe(200)
  return response.json() as Promise<{
    source: string
    scenario: string
    passed: boolean
    blocked: boolean
  }>
}
