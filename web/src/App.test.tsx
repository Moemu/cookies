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
  version: { organization_id: 'org_local', asset_id: 'asset_1234567890', version: 1, status: 'ready', source_type: 'upload', mime_type: 'image/png', size_bytes: 1024, sha256: 'a'.repeat(64), width_pixels: 1200, height_pixels: 800, media: { probe_status: 'not_required' }, project_context_version: 1, created_at: '2026-07-22T00:00:00Z' },
  created_at: '2026-07-22T00:00:00Z',
}

const videoAsset = {
  ref: { project_id: 'project_demo', asset_version: { asset_id: 'video_1234567890', version: 1 } },
  asset: { id: 'video_1234567890', organization_id: 'org_local', asset_kind: 'video', status: 'ready', owner_system: 'assets', latest_version: 1, created_at: '2026-07-22T00:00:00Z', updated_at: '2026-07-22T00:00:00Z' },
  version: { organization_id: 'org_local', asset_id: 'video_1234567890', version: 1, status: 'ready', source_type: 'upload', mime_type: 'video/mp4', size_bytes: 4096, sha256: 'b'.repeat(64), width_pixels: 1080, height_pixels: 1920, media: { duration_seconds: 8, fps: 30, codec: 'h264', probe_status: 'succeeded' }, project_context_version: 1, created_at: '2026-07-22T00:00:00Z' },
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
      if (url.includes('/assets/features?')) return jsonResponse({ items: [] })
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

  it('mounts independent module workspaces from the shared shell', () => {
    render(<App />)

    expect(screen.getByRole('heading', { name: '策略工作区' })).toBeInTheDocument()
    fireEvent.click(screen.getByRole('link', { name: '创意' }))
    expect(screen.getByRole('heading', { name: '创意工作区' })).toBeInTheDocument()

    fireEvent.click(screen.getByRole('link', { name: '管理' }))
    expect(screen.getByRole('heading', { name: '组织与访问' })).toBeInTheDocument()
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

  it('shows task14 remix recovery, trace, citations, eval, and feedback retry UI', async () => {
    window.history.pushState({}, '', '/projects/project_demo/assets/remix')
    const savedPlan = {
      id: 'remixplan_1',
      organization_id: 'org_local',
      project_id: 'project_demo',
      schema_version: 'remix_plan_v2',
      client_plan_id: 'remix_client_1',
      target_seconds: 30,
      actual_seconds: 8,
      pace: 'balanced',
      segments: [{ segment: 'opening', label: '前段', target_seconds: 8, actual_seconds: 8, shots: [], clips: [] }, { segment: 'middle', label: '中段', target_seconds: 15, actual_seconds: 0, shots: [], clips: [] }, { segment: 'ending', label: '后段', target_seconds: 7, actual_seconds: 0, shots: [], clips: [] }],
      warnings: [],
      summary: { selected_assets: 1, used_assets: 1, coverage_percent: 27, strategy: '均衡节奏' },
      created_at: '2026-07-22T00:00:00Z',
      updated_at: '2026-07-22T00:00:00Z',
    }
    vi.stubGlobal('fetch', vi.fn(async (input: string | URL | Request, init?: RequestInit) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url
      const method = init?.method || (input instanceof Request ? input.method : 'GET')
      if (url === '/platform/v1/me') return jsonResponse(identity)
      if (url === '/platform/v1/projects') return jsonResponse({ items: [project] })
      if (url.endsWith('/context')) return jsonResponse({ organization_id: 'org_local', project_id: 'project_demo', brand_id: 'brand_local', product_ids: [], project_context_version: 1 })
      if (url.includes('/assets/features?')) return jsonResponse({ items: [] })
      if (url.includes('/assets?')) return jsonResponse({ items: [videoAsset] })
      if (url.endsWith('/preview')) return jsonResponse({ url: '', method: 'GET', headers: {}, expires_at: '2026-07-22T01:00:00Z' })
      if (url.endsWith('/remix-plans') && method === 'POST') return jsonResponse(savedPlan, 201)
      if (url.endsWith('/remix-plans?limit=8')) return jsonResponse({ items: [savedPlan] })
      if (url.endsWith('/remix-plans/remixplan_1')) return jsonResponse(savedPlan)
      if (url.includes('/knowledge/search')) return jsonResponse({ items: [{ citations: [{ document_id: 'doc_1', chunk_id: 'chunk_1', title: 'Hook 策略', source_uri: 'docs/hook.md', section: '开场', start_line: 3, end_line: 8, snippet: '3 秒内建立停留理由。' }] }] })
      if (url.endsWith('/remix-prerolls')) return jsonResponse({ id: 'preroll_1', plan_id: 'remixplan_1', hook_type: 'conflict', reference_asset: { asset_id: 'video_1234567890', version: 1 }, style_constraints: ['quality:critical'], duration_seconds: 4, mode: 'generate_video', prompt_draft: '前贴 prompt', quality_verdict: 'critical', status: 'failed', error_code: 'QUALITY_CRITICAL', error_message: '前贴质检存在 critical 问题，已阻断插入' })
      if (url.endsWith('/remix-render-jobs') && method === 'POST') return jsonResponse({ id: 'remixrender_1' }, 202)
      if (url.endsWith('/remix-render-jobs/remixrender_1')) return jsonResponse({ id: 'remixrender_1', organization_id: 'org_local', project_id: 'project_demo', plan_id: 'remixplan_1', status: 'failed', progress: 40, target_format: 'mp4', target_quality: 'draft', requires_review: false, error_code: 'ENCODER_FAILED', error_message: 'encoder failed', created_at: '2026-07-22T00:00:00Z', updated_at: '2026-07-22T00:00:00Z' })
      if (url.endsWith('/agent-runs')) return jsonResponse({ id: 'agentrun_1', workflow: 'render_diagnosis', status: 'succeeded', target: { render_job_id: 'remixrender_1' }, steps: [{ id: 'step_1', label: '读取渲染错误并生成诊断', status: 'succeeded', summary: '编码器失败，建议降低码率。' }], tool_calls: [{ id: 'tool_1', name: 'remix.render.diagnose', status: 'succeeded', references: [{ type: 'render_job', id: 'remixrender_1' }] }], trace_spans: [{ id: 'span_1', name: 'render diagnosis agent', kind: 'agent', status: 'succeeded' }, { id: 'span_2', parent_id: 'span_1', name: 'diagnosis-summary', kind: 'model', status: 'succeeded', model: 'fake.render-diagnosis.v1' }] })
      if (url.endsWith('/remix-eval-runs')) return jsonResponse({ id: 'remixevalrun_1', status: 'succeeded', planner_version: 'planner-v1', prompt_version: 'prompt-v1', score: 0.5, total_cases: 2, passed_cases: 1, failed_cases: ['case_1'], results: [{ id: 'result_1', case_id: 'case_1', case_type: 'mcq', score: 0, passed: false, expected: 'b', actual: 'a', reason: '选择了错误 Hook' }] })
      if (url.endsWith('/remix-feedback-events')) return jsonResponse({ error: { code: 'VALIDATION_FAILED', message: '评分服务暂不可用', request_id: 'req_test', retryable: true, details: [] } }, 500)
      return jsonResponse({ error: { code: 'RESOURCE_NOT_FOUND', message: 'not found', request_id: 'req_test', retryable: false, details: [] } }, 404)
    }))

    render(<App />)
    expect(await screen.findByRole('heading', { name: 'AI 海量素材混剪' })).toBeInTheDocument()
    fireEvent.click(await screen.findByRole('button', { name: '前段' }))
    fireEvent.click(screen.getByRole('button', { name: '生成混剪草案' }))
    expect(await screen.findByText('Hook 策略')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: '模拟失败重试' }))
    expect(await screen.findByText('前贴质检存在 critical 问题，已阻断插入')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: '提交渲染' }))
    fireEvent.click(await screen.findByRole('button', { name: 'Agent 诊断' }))
    expect(await screen.findByRole('heading', { name: 'Agent Trace' })).toBeInTheDocument()
    expect(screen.getByText('remix.render.diagnose · succeeded')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: '运行评测' }))
    expect(await screen.findByText('选择了错误 Hook')).toBeInTheDocument()

    fireEvent.change(screen.getByLabelText('反馈评论'), { target: { value: '保留这条失败后的评论' } })
    fireEvent.click(screen.getByRole('button', { name: '提交评分' }))
    expect(await screen.findByText('评分服务暂不可用')).toBeInTheDocument()
    expect(screen.getByLabelText('反馈评论')).toHaveValue('保留这条失败后的评论')
  })
})
