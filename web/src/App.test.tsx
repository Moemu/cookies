import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import App from './App'

const identity = {
  actor: { organization_id: 'org_local', principal: { kind: 'user', id: 'user_local' }, scopes: ['project.read', 'assets.read', 'assets.write'] },
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
    vi.stubGlobal('fetch', vi.fn(async (input: string | URL | Request) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url
      if (url === '/platform/v1/me') return jsonResponse(identity)
      if (url === '/platform/v1/projects') return jsonResponse({ items: [project] })
      if (url.endsWith('/context')) return jsonResponse({ organization_id: 'org_local', project_id: 'project_demo', brand_id: 'brand_local', product_ids: [], project_context_version: 1 })
      if (url.includes('/assets?')) return jsonResponse({ items: [asset] })
      if (url.endsWith('/preview')) return jsonResponse({ url: '', method: 'GET', headers: {}, expires_at: '2026-07-22T01:00:00Z' })
      return jsonResponse({ error: { code: 'RESOURCE_NOT_FOUND', message: 'not found', request_id: 'req_test', retryable: false, details: [] } }, 404)
    }))
    Object.defineProperty(URL, 'createObjectURL', { configurable: true, value: vi.fn(() => 'blob:test-preview') })
    Object.defineProperty(URL, 'revokeObjectURL', { configurable: true, value: vi.fn() })
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('mounts independent module workspaces from the shared shell', () => {
    render(<App />)

    expect(screen.getByRole('heading', { name: '策略工作区' })).toBeInTheDocument()
    fireEvent.click(screen.getByRole('link', { name: '创意' }))
    expect(screen.getByRole('heading', { name: '创意工作区' })).toBeInTheDocument()
  })

  it('loads the real project asset library and opens a validated upload flow', async () => {
    window.history.pushState({}, '', '/projects/project_demo/assets')
    render(<App />)

    expect(screen.getByRole('heading', { name: '项目素材库' })).toBeInTheDocument()
    expect(await screen.findByText('asset_1234567890 · v1')).toBeInTheDocument()
    expect(screen.getAllByText('用户上传')).toHaveLength(2)
    expect(screen.getByText('1200 × 800')).toBeInTheDocument()

    fireEvent.change(screen.getByPlaceholderText('搜索资产 ID 或类型'), { target: { value: 'missing' } })
    expect(await screen.findByRole('heading', { name: '没有匹配的素材' })).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: '上传素材' }))
    expect(screen.getByRole('heading', { name: '上传素材' })).toBeInTheDocument()

    const file = new File(['image'], 'campaign.png', { type: 'image/png' })
    fireEvent.change(screen.getByLabelText('选择图片文件'), { target: { files: [file] } })
    expect(screen.getByText('campaign.png')).toBeInTheDocument()
    await waitFor(() => expect(screen.getByRole('button', { name: '开始上传' })).toBeEnabled())
  })
})
