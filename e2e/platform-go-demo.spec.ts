import { expect, test } from '@playwright/test'

const demoProjectId = 'project_investor_precision_evidence'
const demoProjectName = '投资人路演：精度证据增长'

test.beforeEach(async ({ page }) => {
  await page.route('**/platform/v1/context', async route => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        actor: {
          organization_id: 'org_local',
          principal: { kind: 'user', id: 'user_local' },
          scopes: ['project.read', 'delivery.read'],
        },
      }),
    })
  })
  await page.route('**/api/provider/capabilities', async route => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        provider: 'ark',
        status: 'not_configured',
        capabilities: [
          { capability: 'image.generate', model: 'cookies.image.standard' },
          { capability: 'text.generate', model: 'cookies.text.standard' },
        ],
        checkedAt: '2026-07-28T00:00:00.000Z',
      }),
    })
  })
})

test('默认 Go demo Project 可见且主链路来自 /platform/v1', async ({ page }) => {
  const platformRequests: string[] = []
  const legacyProjectRequests: string[] = []

  page.on('request', request => {
    const url = new URL(request.url())
    if (url.pathname.startsWith('/platform/v1/')) platformRequests.push(url.pathname)
    if (url.pathname.startsWith('/api/projects')) legacyProjectRequests.push(url.pathname)
  })

  await page.goto('/')

  await expect(page.getByRole('heading', { name: '代理商客户组合工作台' })).toBeVisible()
  await expect.poll(() => platformRequests).toEqual(expect.arrayContaining([
    '/platform/v1/projects',
  ]))
  expect(legacyProjectRequests).toEqual([])

  await page.goto(`/projects/${demoProjectId}/manage`)

  await expect(page.getByRole('heading', { name: demoProjectName, level: 1 })).toBeVisible()
  await expect(page.getByText('拆解公开样本的钩子、证明与 CTA 结构，生成白域精工可用的原创复刻草案。')).toBeVisible()
  await expect(page.getByRole('heading', { name: '最近业务任务' })).toBeVisible()
  await expect(page.getByText('精密制造主创意组合')).toBeVisible()
  await expect(page.getByText('突出 ±0.01mm 精度、交付稳定性和真实制造画面')).toBeVisible()
  await expect(page.getByText('5 个任务全部限定在当前 Project')).toBeVisible()
  await expect(page.getByText('1 个 ChangeSet')).toBeVisible()

  await expect.poll(() => platformRequests).toContain(`/platform/v1/projects/${demoProjectId}`)
  expect(legacyProjectRequests).toEqual([])
})
