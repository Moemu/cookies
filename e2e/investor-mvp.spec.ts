import { expect, test, type Page } from '@playwright/test'

async function createProjectWithBrief(page: Page, name: string) {
  const projectResponse = await page.request.post('http://127.0.0.1:8787/api/projects', {
    data: { name, brand: 'E2E 隔离品牌', objective: '独立验证当前项目服务端事实来源' },
  })
  expect(projectResponse.ok()).toBeTruthy()
  const project = await projectResponse.json() as { id: string }
  const briefResponse = await page.request.post('http://127.0.0.1:8787/api/artifacts', {
    data: { projectId: project.id, kind: 'brief', status: 'ready', content: `${name} 已确认 Brief` },
  })
  expect(briefResponse.ok()).toBeTruthy()
  return project.id
}

async function openProject(page: Page, name: string) {
  const projectId = await createProjectWithBrief(page, name)
  await page.goto(`/projects/${projectId}/home`)
  await expect(page.getByRole('heading', { name })).toBeVisible()
  return projectId
}

async function setProviderMode(page: Page, mode: 'success' | 'create_failure' | 'task_failure') {
  const response = await page.request.post('http://127.0.0.1:8791/test/mode', { data: { mode } })
  expect(response.ok()).toBeTruthy()
}

async function syncPrerollJob(
  page: Page,
  projectId: string,
  prerollType: 'short_drama' | 'game',
  expectedStatus: 'succeeded' | 'failed',
) {
  const query = `projectId=${projectId}&purpose=preroll&prerollType=${prerollType}`
  await expect.poll(async () => {
    const jobsResponse = await page.request.get(`http://127.0.0.1:8787/api/generation-jobs?${query}`)
    const jobs = await jobsResponse.json() as Array<{ id: string }>
    const job = jobs.at(-1)
    if (!job) return false
    const response = await page.request.get(`http://127.0.0.1:8787/api/generation-jobs/${job.id}?${query}`)
    const synced = await response.json() as { status: string }
    return synced.status === expectedStatus
  }, { timeout: 10_000 }).toBe(true)
}

async function expectInitialViewportControl(page: Page, locator: ReturnType<Page['locator']>, width: number) {
  await expect(locator).toBeVisible()
  await expect(locator).toBeEnabled()
  const box = await locator.boundingBox()
  expect(box).not.toBeNull()
  expect(box!.x).toBeGreaterThanOrEqual(0)
  expect(box!.x + box!.width).toBeLessThanOrEqual(width)
  expect(box!.y).toBeGreaterThanOrEqual(0)
  expect(box!.y + box!.height).toBeLessThanOrEqual(await page.evaluate(() => window.innerHeight))
  await locator.click({ trial: true })
}

async function expectNoHorizontalOverflow(page: Page) {
  expect(await page.evaluate(() => {
    const pageContainer = document.querySelector<HTMLElement>('.page-frame')
    return document.documentElement.scrollWidth <= window.innerWidth
      && document.body.scrollWidth <= window.innerWidth
      && (pageContainer?.scrollWidth ?? 0) <= (pageContainer?.clientWidth ?? 0)
  })).toBeTruthy()
}

test('项目主路径仅使用本用例创建的 Project 和 Brief', async ({ page }) => {
  const projectId = await openProject(page, 'E2E 独立主路径')

  await expect(page).toHaveURL(`/projects/${projectId}/home`)
  await expect(page.getByRole('region', { name: '项目八阶段业务流程' })).toBeVisible()
  await page.getByRole('button', { name: '投后承接阶段 06 至 08 投放数据形成复盘，再沉淀为下一轮经验' }).click()
  await expect(page).toHaveURL(`/projects/${projectId}/insight/performance`)
  await expect(page.getByText('没有可用的投后运营数据')).toBeVisible()
})

test('创意镜头支持键盘切换并同步当前预览', async ({ page }) => {
  const projectId = await openProject(page, 'E2E 键盘镜头')
  await page.goto(`/projects/${projectId}/creative/video?view=${encodeURIComponent('效果广告')}`)

  const firstShot = page.getByRole('button', { name: /消息弹窗与人物停顿/ })
  await firstShot.focus()
  await page.keyboard.press('ArrowDown')

  await expect(page.getByRole('button', { name: /切入高速 CNC 现场/ })).toBeFocused()
  await expect(page.getByRole('status')).toContainText('当前镜头：02 · 切入高速 CNC 现场。')
  await expect(page.getByLabel('当前镜头预览')).toContainText('02 / 03')
})

test('前贴分镜成功后出现持久化资产，并在刷新后按当前前贴类型恢复', async ({ page }) => {
  const projectId = await openProject(page, 'E2E 前贴成功恢复')
  await page.goto(`/projects/${projectId}/creative/video?view=${encodeURIComponent('效果广告')}`)
  await setProviderMode(page, 'success')

  const generate = page.getByRole('button', { name: '生成前贴分镜' })
  const addToLibrary = page.getByRole('button', { name: '加入混剪素材箱' })
  await expect(generate).toBeEnabled()
  await expect(addToLibrary).toBeDisabled()
  await generate.click()
  await syncPrerollJob(page, projectId, 'short_drama', 'succeeded')
  await page.reload()
  await expect(addToLibrary).toBeEnabled()

  await expect.poll(async () => {
    const response = await page.request.get(`http://127.0.0.1:8787/api/artifacts?projectId=${projectId}&purpose=preroll&prerollType=short_drama`)
    const items = await response.json() as Array<{ status: string, sourceJobId?: string }>
    return items.some(item => item.status === 'ready' && Boolean(item.sourceJobId))
  }).toBe(true)
  await page.reload()

  await expect(addToLibrary).toBeEnabled()
  await page.goto(`/projects/${projectId}/creative/video?view=${encodeURIComponent('素材剪辑')}`)
  await expect(page.getByText('短剧前贴视频').first()).toBeVisible()
  await expect(page.getByText('当前 Project 暂无可用于混剪的已持久化视频资产。')).toHaveCount(0)
})

test('前贴任务按项目和类型隔离，重试或失败不会保留旧成功可用态', async ({ page }) => {
  const firstProjectId = await openProject(page, 'E2E 前贴重试失败')
  await page.goto(`/projects/${firstProjectId}/creative/video?view=${encodeURIComponent('效果广告')}`)
  await setProviderMode(page, 'success')
  await page.getByRole('button', { name: '生成前贴分镜' }).click()
  await syncPrerollJob(page, firstProjectId, 'short_drama', 'succeeded')
  await page.reload()
  await expect(page.getByRole('button', { name: '加入混剪素材箱' })).toBeEnabled()

  await setProviderMode(page, 'task_failure')
  await page.getByRole('button', { name: '重新生成前贴' }).click()
  await expect(page.getByRole('button', { name: '加入混剪素材箱' })).toBeDisabled()
  await syncPrerollJob(page, firstProjectId, 'short_drama', 'failed')
  await page.reload()
  await expect(page.getByRole('button', { name: '加入混剪素材箱' })).toBeDisabled()

  const secondProjectId = await createProjectWithBrief(page, 'E2E 前贴类型隔离')

  await setProviderMode(page, 'success')
  await page.goto(`/projects/${secondProjectId}/creative/video?view=${encodeURIComponent('效果广告')}`)
  await page.getByRole('tab', { name: /游戏前贴/ }).click()
  await page.getByRole('button', { name: '生成前贴分镜' }).click()
  await syncPrerollJob(page, secondProjectId, 'game', 'succeeded')
  await page.reload()
  await page.getByRole('tab', { name: /游戏前贴/ }).click()
  await expect(page.getByRole('button', { name: '加入混剪素材箱' })).toBeEnabled()

  const [firstGameJobs, secondShortJobs, secondGameArtifacts] = await Promise.all([
    page.request.get(`http://127.0.0.1:8787/api/generation-jobs?projectId=${firstProjectId}&purpose=preroll&prerollType=game`),
    page.request.get(`http://127.0.0.1:8787/api/generation-jobs?projectId=${secondProjectId}&purpose=preroll&prerollType=short_drama`),
    page.request.get(`http://127.0.0.1:8787/api/artifacts?projectId=${secondProjectId}&purpose=preroll&prerollType=game`),
  ])
  expect(await firstGameJobs.json()).toEqual([])
  expect(await secondShortJobs.json()).toEqual([])
  expect(await secondGameArtifacts.json()).toEqual(expect.arrayContaining([
    expect.objectContaining({ status: 'ready', prerollType: 'game' }),
  ]))
})

test('前贴创建被 Provider 拒绝时展示可重试错误，且不伪造资产', async ({ page }) => {
  const projectId = await openProject(page, 'E2E 前贴创建拒绝')
  await page.goto(`/projects/${projectId}/creative/video?view=${encodeURIComponent('效果广告')}`)
  await page.getByRole('tab', { name: /游戏前贴/ }).click()
  await setProviderMode(page, 'create_failure')

  await page.getByRole('button', { name: '生成前贴分镜' }).click()
  await expect(page.getByRole('status')).toContainText('Model provider could not complete the request')
  await expect(page.getByRole('button', { name: '加入混剪素材箱' })).toBeDisabled()
  await setProviderMode(page, 'success')
})

test('主创意聚合和 ChangeSet 不接受前贴资产', async ({ page }) => {
  const projectId = await openProject(page, 'E2E 主创意边界')
  const artifactsResponse = await page.request.get(`http://127.0.0.1:8787/api/artifacts?projectId=${projectId}`)
  const artifacts = await artifactsResponse.json() as Array<{ id: string, kind: string }>
  const briefId = artifacts.find(artifact => artifact.kind === 'brief')?.id
  expect(briefId).toBeTruthy()
  const mainCreative = await page.request.post('http://127.0.0.1:8787/api/artifacts', {
    data: { projectId, kind: 'image', status: 'ready', content: '当前项目主创意' },
  })
  const preroll = await page.request.post('http://127.0.0.1:8787/api/artifacts', {
    data: {
      projectId,
      kind: 'video',
      purpose: 'preroll',
      prerollType: 'short_drama',
      status: 'ready',
      content: '不得作为主创意的前贴',
    },
  })
  expect(mainCreative.ok()).toBeTruthy()
  expect(preroll.ok()).toBeTruthy()
  const mainArtifact = await mainCreative.json() as { id: string }
  const prerollArtifact = await preroll.json() as { id: string }

  await page.goto(`/projects/${projectId}/delivery/plans`)
  await expect(page.locator('.upstream-source')).toContainText('当前项目主创意')
  await expect(page.locator('.upstream-source')).not.toContainText('不得作为主创意的前贴')

  const rejected = await page.request.post('http://127.0.0.1:8787/api/change-sets', {
    data: { projectId, name: '拒绝前贴', artifactIds: [briefId, prerollArtifact.id], budgetLimit: 100 },
  })
  expect(rejected.status()).toBe(400)
  const accepted = await page.request.post('http://127.0.0.1:8787/api/change-sets', {
    data: { projectId, name: '接受主创意', artifactIds: [briefId, mainArtifact.id], budgetLimit: 100 },
  })
  expect(accepted.ok()).toBeTruthy()
})

test('素材体验页只展示本用例当前 Project 的持久化 Artifact', async ({ page }) => {
  const projectId = await openProject(page, 'E2E 资产体验')
  const currentAsset = await page.request.post('http://127.0.0.1:8787/api/artifacts', {
    data: { projectId, kind: 'image', status: 'ready', content: '本项目持久化主创意' },
  })
  expect(currentAsset.ok()).toBeTruthy()
  const otherProjectId = await createProjectWithBrief(page, 'E2E 其他资产项目')
  const otherAsset = await page.request.post('http://127.0.0.1:8787/api/artifacts', {
    data: { projectId: otherProjectId, kind: 'video', status: 'ready', content: '其他项目不可见资产' },
  })
  expect(otherAsset.ok()).toBeTruthy()

  await page.goto(`/projects/${projectId}/insight/assets`)
  await expect(page.getByRole('heading', { name: '本项目持久化主创意' })).toBeVisible()
  await expect(page.getByText('其他项目不可见资产')).toHaveCount(0)
})

test('前贴任务取消后刷新会恢复服务端取消态，且不暴露旧预览或素材箱入口', async ({ page }) => {
  const projectId = await openProject(page, 'E2E 前贴取消恢复')
  await page.goto(`/projects/${projectId}/creative/video?view=${encodeURIComponent('效果广告')}`)
  await setProviderMode(page, 'success')

  await page.getByRole('button', { name: '生成前贴分镜' }).click()
  const cancel = page.getByRole('button', { name: '取消生成' })
  await expect(cancel).toBeVisible()
  await cancel.click()
  await expect(page.getByRole('status')).toContainText('前贴分镜任务已取消')
  await page.reload()

  const query = `projectId=${projectId}&purpose=preroll&prerollType=short_drama`
  await expect.poll(async () => {
    const response = await page.request.get(`http://127.0.0.1:8787/api/generation-jobs?${query}`)
    const jobs = await response.json() as Array<{ status: string }>
    return jobs.at(-1)?.status
  }).toBe('cancelled')
  await expect(page.getByText('已取消').first()).toBeVisible()
  await expect(page.getByRole('button', { name: '播放短剧前贴预览' })).toBeDisabled()
  await expect(page.getByRole('button', { name: '加入混剪素材箱' })).toBeDisabled()
  await page.goto(`/projects/${projectId}/creative/video?view=${encodeURIComponent('素材剪辑')}`)
  await expect(page.getByText('当前 Project 暂无可用于混剪的已持久化视频资产。')).toBeVisible()
})

for (const width of [1280, 1440, 1680]) {
  test(`桌面 ${width}px 初始视口中前贴工作区和素材剪辑控件可操作且无横向溢出`, async ({ page }) => {
    await page.setViewportSize({ width, height: 960 })
    const projectId = await openProject(page, `E2E 初始桌面视口 ${width}`)
    await page.goto(`/projects/${projectId}/creative/video?view=${encodeURIComponent('效果广告')}`)

    const workspace = page.locator('.preroll-workspace')
    const generate = page.getByRole('button', { name: '生成前贴分镜' })
    const addToLibrary = page.getByRole('button', { name: '加入混剪素材箱' })
    await expect(workspace).toBeVisible()
    await expectInitialViewportControl(page, generate, width)
    await expect(addToLibrary).toBeVisible()
    await expect(page.getByRole('button', { name: '播放短剧前贴预览' })).toBeVisible()
    await expectNoHorizontalOverflow(page)

    await setProviderMode(page, 'success')
    await generate.click()
    const cancel = page.getByRole('button', { name: '取消生成' })
    await expectInitialViewportControl(page, cancel, width)
    await cancel.click()

    await page.getByRole('button', { name: '生成前贴分镜' }).click()
    await syncPrerollJob(page, projectId, 'short_drama', 'succeeded')
    await page.reload()
    await expectInitialViewportControl(page, addToLibrary, width)
    await expectNoHorizontalOverflow(page)

    const assetResponse = await page.request.post('http://127.0.0.1:8787/api/artifacts', {
      data: {
        projectId,
        kind: 'video',
        purpose: 'preroll',
        prerollType: 'game',
        status: 'ready',
        content: '素材剪辑初始视口资产',
      },
    })
    expect(assetResponse.ok()).toBeTruthy()
    await page.goto(`/projects/${projectId}/creative/video?view=${encodeURIComponent('素材剪辑')}`)

    const assetBin = page.locator('.editing-assets')
    const sourceAsset = assetBin.getByRole('button').first()
    const addToTimeline = page.getByRole('button', { name: '加入混剪时间线' })
    await expect(assetBin).toBeVisible()
    await expectInitialViewportControl(page, sourceAsset, width)
    await expectNoHorizontalOverflow(page)

    await sourceAsset.click()
    await expectInitialViewportControl(page, addToTimeline, width)
    await expectInitialViewportControl(page, page.getByRole('button', { name: '生成混剪版本' }), width)
    await expectInitialViewportControl(page, page.getByRole('button', { name: '保存为 EditTask' }), width)
    await expectNoHorizontalOverflow(page)
  })
}
