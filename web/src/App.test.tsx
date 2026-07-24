import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import App from './App'

const identity = {
  actor: {
    organization_id: 'org_local',
    principal: { kind: 'user', id: 'user_local' },
    scopes: ['project.read', 'project.write', 'assets.read', 'assets.write', 'delivery.read', 'delivery.write', 'delivery.approve', 'delivery.execute', 'insights.read', 'insights.write', 'insights.confirm'],
  },
  organization: { id: 'org_local', name: '本地组织', status: 'active' },
  user: { id: 'user_local', display_name: '本地用户', status: 'active' },
  membership: { organization_id: 'org_local', user_id: 'user_local', role: 'owner', status: 'active' },
}
let activeIdentity = identity

const project = {
  id: 'project_demo',
  organization_id: 'org_local',
  name: '示例项目',
  status: 'active',
  primary_brand_id: 'brand_local',
  project_context_version: 1,
  created_at: '2026-07-22T00:00:00Z',
  updated_at: '2026-07-22T00:00:00Z',
}

const createdProject = {
  ...project,
  id: 'project_summer',
  name: '夏季新品推广',
  primary_brand_id: 'brand_studio',
}

const asset = {
  ref: { project_id: 'project_demo', asset_version: { asset_id: 'asset_1234567890', version: 1 } },
  asset: { id: 'asset_1234567890', organization_id: 'org_local', asset_kind: 'image', status: 'ready', owner_system: 'assets', latest_version: 1, created_at: '2026-07-22T00:00:00Z', updated_at: '2026-07-22T00:00:00Z' },
  version: { organization_id: 'org_local', asset_id: 'asset_1234567890', version: 1, status: 'ready', source_type: 'upload', mime_type: 'image/png', size_bytes: 1024, sha256: 'a'.repeat(64), width_pixels: 1200, height_pixels: 800, project_context_version: 1, created_at: '2026-07-22T00:00:00Z' },
  created_at: '2026-07-22T00:00:00Z',
}

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })
}

describe('App', () => {
  beforeEach(() => {
    activeIdentity = identity
    window.history.pushState({}, '', '/strategy')
    vi.stubGlobal('fetch', vi.fn(async (input: string | URL | Request, init?: RequestInit) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url
      const method = init?.method || (input instanceof Request ? input.method : 'GET')
      if (url === '/platform/v1/me') return jsonResponse(activeIdentity)
      if (url === '/platform/v1/brands' && method === 'POST') return jsonResponse({ id: 'brand_studio', organization_id: 'org_local', name: 'Cookies Studio', status: 'active' }, 201)
      if (url === '/platform/v1/projects' && method === 'POST') return jsonResponse(createdProject, 201)
      if (url === '/platform/v1/projects') return jsonResponse({ items: [project] })
      if (url === '/platform/v1/projects/project_demo/assets/asset_1234567890/versions/1' && method === 'DELETE') return new Response(null, { status: 204 })
      if (url.endsWith('/context')) return jsonResponse({ organization_id: 'org_local', project_id: url.includes('project_summer') ? 'project_summer' : 'project_demo', brand_id: url.includes('project_summer') ? 'brand_studio' : 'brand_local', product_ids: [], project_context_version: 1 })
      if (url.includes('/assets?')) return jsonResponse({ items: [asset] })
      if (url.endsWith('/preview')) return jsonResponse({ url: '', method: 'GET', headers: {}, expires_at: '2026-07-22T01:00:00Z' })
      if (url.includes('/creative-packages?')) return jsonResponse({ items: [] })
      if (url.includes('/delivery/v1/') && url.includes('/plans?')) return jsonResponse({ items: [] })
      if (url.includes('/delivery/v1/') && url.includes('/executions?')) return jsonResponse({ items: [] })
      if (url.includes('/insights/v1/') && url.includes('/reports?')) return jsonResponse({ items: [] })
      if (url.includes('/insights/v1/') && url.includes('/experiences?')) return jsonResponse({ items: [] })
      if (url.endsWith('/prelaunch')) return jsonResponse({ project_id: 'project_demo', experience_references: [], disclosure: '仅引用已确认经验。' })
      if (url.endsWith('/performance')) return jsonResponse({ project_id: 'project_demo', executions: [], disclosure: '当前为本地模拟执行证据。' })
      return jsonResponse({ error: { code: 'RESOURCE_NOT_FOUND', message: 'not found', request_id: 'req_test', retryable: false, details: [] } }, 404)
    }))
    Object.defineProperty(URL, 'createObjectURL', { configurable: true, value: vi.fn(() => 'blob:test-preview') })
    Object.defineProperty(URL, 'revokeObjectURL', { configurable: true, value: vi.fn() })
  })

  afterEach(() => {
    cleanup()
    vi.unstubAllGlobals()
  })

  it('mounts independent module workspaces from the shared shell', async () => {
    render(<App />)

    expect(await screen.findByRole('heading', { name: '示例项目 · 策略' })).toBeInTheDocument()
    fireEvent.click(screen.getByRole('link', { name: '创意' }))
    expect(await screen.findByRole('heading', { name: '小红书图文创作' })).toBeInTheDocument()

    fireEvent.click(screen.getByRole('link', { name: '管理' }))
    expect(await screen.findByRole('heading', { name: '组织与访问' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: '洞察' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: '投放' })).toBeInTheDocument()

    fireEvent.click(screen.getByRole('link', { name: '投放' }))
    expect(await screen.findByRole('heading', { name: '投放计划与变更控制' })).toBeInTheDocument()
    fireEvent.click(screen.getByRole('link', { name: '洞察' }))
    expect(await screen.findByRole('heading', { name: '投前洞察' })).toBeInTheDocument()
  })

  it('exposes the current identity through a user menu and read-only profile', async () => {
    render(<App />)

    fireEvent.click(await screen.findByRole('button', { name: '打开 本地用户 的用户菜单' }))
    expect(screen.getByRole('menuitem', { name: '个人资料' })).toBeInTheDocument()
    expect(screen.getByRole('menuitem', { name: '安全摘要' })).toBeInTheDocument()
    expect(screen.getByRole('menuitem', { name: '偏好设置' })).toBeInTheDocument()

    fireEvent.click(screen.getByRole('menuitem', { name: '个人资料' }))
    expect(await screen.findByRole('heading', { name: '个人资料' })).toBeInTheDocument()
    expect(screen.getAllByText('本地用户')).toHaveLength(2)
    expect(screen.getByText('本地组织')).toBeInTheDocument()
  })

  it('hides unauthorized modules and gives a recoverable 403 on a direct URL', async () => {
    activeIdentity = {
      ...identity,
      actor: { ...identity.actor, scopes: ['project.read', 'assets.read'] },
    }
    window.history.pushState({}, '', '/projects/project_demo/delivery/plans')
    render(<App />)

    expect(await screen.findByRole('heading', { name: '当前账号没有访问权限' })).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: '投放' })).not.toBeInTheDocument()
    expect(screen.queryByRole('link', { name: '洞察' })).not.toBeInTheDocument()
    expect(screen.getByRole('link', { name: '返回项目' })).toHaveAttribute('href', '/projects/project_demo/home')
  })

  it('loads the real project asset library and opens a validated upload flow', async () => {
    window.history.pushState({}, '', '/projects/project_demo/assets')
    render(<App />)

    expect(await screen.findByRole('heading', { name: '示例项目 · 素材库' })).toBeInTheDocument()
    expect(await screen.findByText('asset_1234567890 · v1')).toBeInTheDocument()
    expect(screen.getAllByText('用户上传')).toHaveLength(2)
    expect(screen.getByText('1200 × 800')).toBeInTheDocument()

    fireEvent.change(screen.getByPlaceholderText('搜索资产 ID 或类型'), { target: { value: 'missing' } })
    expect(await screen.findByRole('heading', { name: '没有匹配的素材' })).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: '添加素材' }))
    expect(screen.getByRole('heading', { name: '上传素材' })).toBeInTheDocument()

    const file = new File(['image'], 'campaign.png', { type: 'image/png' })
    fireEvent.change(screen.getByLabelText('选择图片文件'), { target: { files: [file] } })
    expect(screen.getByText('campaign.png')).toBeInTheDocument()
    await waitFor(() => expect(screen.getByRole('button', { name: '开始上传' })).toBeEnabled())
  })

  it('creates a brand-bound project and opens its project home', async () => {
    render(<App />)

    fireEvent.click(await screen.findByRole('button', { name: '示例项目' }))
    fireEvent.click(screen.getByRole('menuitem', { name: '新建项目' }))
    expect(screen.getByRole('heading', { name: '新建项目' })).toBeInTheDocument()

    fireEvent.change(screen.getByPlaceholderText('例如：夏季新品推广'), { target: { value: '夏季新品推广' } })
    fireEvent.change(screen.getByPlaceholderText('例如：Cookies Studio'), { target: { value: 'Cookies Studio' } })
    fireEvent.click(screen.getByRole('button', { name: '创建项目' }))

    expect(await screen.findByRole('heading', { name: '夏季新品推广' })).toBeInTheDocument()
    expect(window.location.pathname).toBe('/projects/project_summer/home')
    expect(fetch).toHaveBeenCalledWith('/platform/v1/brands', expect.objectContaining({
      method: 'POST',
      body: JSON.stringify({ name: 'Cookies Studio' }),
    }))
    expect(fetch).toHaveBeenCalledWith('/platform/v1/projects', expect.objectContaining({
      method: 'POST',
      body: JSON.stringify({ name: '夏季新品推广', primary_brand_id: 'brand_studio', product_ids: [], activate: true }),
    }))
  })

  it('deletes a project asset relationship after explicit confirmation', async () => {
    window.history.pushState({}, '', '/projects/project_demo/assets')
    render(<App />)

    expect(await screen.findByText('asset_1234567890 · v1')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: '删除 asset_1234567890 · v1' }))
    expect(screen.getByRole('heading', { name: '从项目中删除素材？' })).toBeInTheDocument()
    expect(screen.getByText('底层不可变版本、文件和引用记录仍会保留，用于审计和 Provider 任务溯源。')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: '确认删除' }))

    await waitFor(() => expect(screen.queryByText('asset_1234567890 · v1')).not.toBeInTheDocument())
    expect(fetch).toHaveBeenCalledWith('/platform/v1/projects/project_demo/assets/asset_1234567890/versions/1', expect.objectContaining({ method: 'DELETE' }))
  })
})
