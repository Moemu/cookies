import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import App from './App'

const identity = {
  actor: { organization_id: 'org_local', principal: { kind: 'user', id: 'user_local' }, scopes: ['project.read', 'project.write', 'assets.read', 'assets.write'] },
  organization: { id: 'org_local', name: '本地组织', status: 'active' },
  user: { id: 'user_local', display_name: '本地用户', status: 'active' },
  membership: { organization_id: 'org_local', user_id: 'user_local', role: 'owner', status: 'active' },
}

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
    window.history.pushState({}, '', '/strategy')
    vi.stubGlobal('fetch', vi.fn(async (input: string | URL | Request, init?: RequestInit) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url
      const method = init?.method || (input instanceof Request ? input.method : 'GET')
      if (url === '/platform/v1/me') return jsonResponse(identity)
      if (url === '/platform/v1/brands' && method === 'POST') return jsonResponse({ id: 'brand_studio', organization_id: 'org_local', name: 'Cookies Studio', status: 'active' }, 201)
      if (url === '/platform/v1/projects' && method === 'POST') return jsonResponse(createdProject, 201)
      if (url === '/platform/v1/projects') return jsonResponse({ items: [project] })
      if (url === '/platform/v1/projects/project_demo/assets/asset_1234567890/versions/1' && method === 'DELETE') return new Response(null, { status: 204 })
      if (url.endsWith('/context')) return jsonResponse({ organization_id: 'org_local', project_id: url.includes('project_summer') ? 'project_summer' : 'project_demo', brand_id: url.includes('project_summer') ? 'brand_studio' : 'brand_local', product_ids: [], project_context_version: 1 })
      if (url.includes('/assets?')) return jsonResponse({ items: [asset] })
      if (url.endsWith('/preview')) return jsonResponse({ url: '', method: 'GET', headers: {}, expires_at: '2026-07-22T01:00:00Z' })
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
    expect(screen.queryByRole('link', { name: '洞察' })).not.toBeInTheDocument()
    expect(screen.queryByRole('link', { name: '投放' })).not.toBeInTheDocument()
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

  it('creates a brand-bound project and opens its asset library', async () => {
    render(<App />)

    fireEvent.click(await screen.findByRole('button', { name: '示例项目' }))
    fireEvent.click(screen.getByRole('menuitem', { name: '新建项目' }))
    expect(screen.getByRole('heading', { name: '新建项目' })).toBeInTheDocument()

    fireEvent.change(screen.getByPlaceholderText('例如：夏季新品推广'), { target: { value: '夏季新品推广' } })
    fireEvent.change(screen.getByPlaceholderText('例如：Cookies Studio'), { target: { value: 'Cookies Studio' } })
    fireEvent.click(screen.getByRole('button', { name: '创建项目' }))

    expect(await screen.findByRole('heading', { name: '夏季新品推广 · 素材库' })).toBeInTheDocument()
    expect(window.location.pathname).toBe('/projects/project_summer/assets')
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
