import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { CreativeImageTextPage } from './CreativeImageTextPage'

const task = {
  id: 'creativetask_1', organization_id: 'org_1', project_id: 'project_1', intake_id: 'creativeintake_1', format: 'image_text', channel: 'xiaohongshu', status: 'draft',
  direction: { concept: '晨光咖啡桌', tone: ['自然', '克制'], visual_keywords: ['晨光'] }, version: 1, created_at: '2026-07-23T01:00:00Z', updated_at: '2026-07-23T01:00:00Z',
}
const intake = {
  id: 'creativeintake_1', organization_id: 'org_1', project_id: 'project_1', source: 'manual', status: 'ready',
  request: { source: 'manual', channel: 'xiaohongshu', objective: '建立新品认知', audience: '年轻上班族', core_message: '给忙碌早晨的一点从容', call_to_action: '收藏灵感', concept: '晨光咖啡桌', tone: ['自然'], visual_keywords: ['晨光'], mandatory_elements: [], prohibited_claims: [] },
  missing_fields: [], warnings: [], version: 1, created_at: '2026-07-23T01:00:00Z', updated_at: '2026-07-23T01:00:00Z',
}
const strategyIntake = {
  ...intake,
  id: 'creativeintake_strategy_1',
  source: 'strategy_package',
  request: { ...intake.request, source: 'strategy_package', strategy_package: { package_id: 'strategypackage_1', package_version: 1, expected_content_hash: 'sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa' } },
}
const duplicateStrategyIntake = { ...strategyIntake, id: 'creativeintake_strategy_duplicate_1' }
const detail = {
  task, intake,
  draft: { task_id: task.id, version: 1, status: 'draft', title_candidates: ['给早晨一点从容', '一杯咖啡的仪式感', '上班前的留白'], body: '一段可审阅的正文。', topics: ['#创意灵感'], cover_copy: '从容开始', image_plan: [{ order: 1, purpose: '封面', visual_brief: '晨光中的咖啡桌', caption: '从容开始' }], created_at: '2026-07-23T01:00:00Z' },
  production_jobs: [],
}

function jsonResponse(body: unknown, status = 200) { return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } }) }

describe('CreativeImageTextPage', () => {
  beforeEach(() => {
    vi.stubGlobal('fetch', vi.fn(async (input: string | URL | Request, init?: RequestInit) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url
      const method = init?.method || (input instanceof Request ? input.method : 'GET')
      if (url.endsWith('/creative-tasks?limit=100')) return jsonResponse({ items: [] })
      if (url.endsWith('/creative-intakes?limit=100')) return jsonResponse({ items: [] })
      if (url.endsWith('/creative-intakes') && method === 'POST') return jsonResponse(intake, 201)
      if (url.includes('creative-intakes/creativeintake_1:create-task') && method === 'POST') return jsonResponse(task, 201)
      if (url.endsWith('/creative-tasks/creativetask_1')) return jsonResponse(detail)
      return jsonResponse({ error: { code: 'NOT_FOUND', message: 'not found', request_id: 'req_test', retryable: false, details: [] } }, 404)
    }))
  })

  afterEach(() => { cleanup(); vi.unstubAllGlobals() })

  it('creates a ready manual Intake and shows its image-text draft', async () => {
    render(<MemoryRouter initialEntries={['/projects/project_1/creative']}><Routes><Route path="/projects/:projectId/creative" element={<CreativeImageTextPage />} /></Routes></MemoryRouter>)
    expect(screen.getByRole('heading', { name: '小红书图文创作' })).toBeInTheDocument()
    await waitFor(() => expect(fetch).toHaveBeenCalled())
    fireEvent.change(screen.getByLabelText('传播目标'), { target: { value: '建立新品认知' } })
    fireEvent.change(screen.getByLabelText('目标人群'), { target: { value: '年轻上班族' } })
    fireEvent.change(screen.getByLabelText('核心信息'), { target: { value: '给忙碌早晨的一点从容' } })
    fireEvent.click(screen.getByRole('button', { name: '保存输入并创建图文任务' }))
    expect(await screen.findByRole('heading', { name: '这篇图文要怎么讲？' })).toBeInTheDocument()
    fireEvent.change(screen.getByLabelText('本次创作角度'), { target: { value: '通勤早晨的生活方式种草' } })
    fireEvent.click(screen.getByRole('button', { name: '创建这篇图文任务' }))
    expect(await screen.findByRole('heading', { name: '晨光咖啡桌' })).toBeInTheDocument()
    expect(screen.getByText('给早晨一点从容')).toBeInTheDocument()
    expect(fetch).toHaveBeenCalledWith('/api/creative/v1/projects/project_1/creative-intakes', expect.objectContaining({ method: 'POST' }))
  })

  it('shows a ready Strategy Intake and explicitly creates its Creative task', async () => {
    vi.stubGlobal('fetch', vi.fn(async (input: string | URL | Request, init?: RequestInit) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url
      const method = init?.method || (input instanceof Request ? input.method : 'GET')
      if (url.endsWith('/creative-tasks?limit=100')) return jsonResponse({ items: [] })
      if (url.endsWith('/creative-intakes?limit=100')) return jsonResponse({ items: [strategyIntake] })
      if (url.includes('creative-intakes/creativeintake_strategy_1:create-task') && method === 'POST') return jsonResponse(task, 201)
      if (url.endsWith('/creative-tasks/creativetask_1')) return jsonResponse(detail)
      return jsonResponse({ error: { code: 'NOT_FOUND', message: 'not found', request_id: 'req_test', retryable: false, details: [] } }, 404)
    }))
    render(<MemoryRouter initialEntries={['/projects/project_1/creative']}><Routes><Route path="/projects/:projectId/creative" element={<CreativeImageTextPage />} /></Routes></MemoryRouter>)
    expect(await screen.findByText('策略包版本 v1')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: '基于此策略创建图文任务' }))
    fireEvent.change(await screen.findByLabelText('本次创作角度'), { target: { value: '成分解释角度' } })
    fireEvent.click(screen.getByRole('button', { name: '创建这篇图文任务' }))
    expect(await screen.findByRole('heading', { name: '晨光咖啡桌' })).toBeInTheDocument()
    expect(fetch).toHaveBeenCalledWith('/api/creative/v1/projects/project_1/creative-intakes/creativeintake_strategy_1:create-task', expect.objectContaining({ method: 'POST' }))
  })

  it('groups duplicate handoffs of the same Strategy package and labels Creative tasks separately', async () => {
    const strategyTask = { ...task, intake_id: strategyIntake.id }
    const strategyDetail = { ...detail, task: strategyTask, intake: strategyIntake, draft: { ...detail.draft, task_id: strategyTask.id } }
    vi.stubGlobal('fetch', vi.fn(async (input: string | URL | Request) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url
      if (url.endsWith('/creative-tasks?limit=100')) return jsonResponse({ items: [strategyTask] })
      if (url.endsWith('/creative-intakes?limit=100')) return jsonResponse({ items: [strategyIntake, duplicateStrategyIntake] })
      if (url.endsWith('/creative-tasks/creativetask_1')) return jsonResponse(strategyDetail)
      return jsonResponse({ error: { code: 'NOT_FOUND', message: 'not found', request_id: 'req_test', retryable: false, details: [] } }, 404)
    }))
    render(<MemoryRouter initialEntries={['/projects/project_1/creative']}><Routes><Route path="/projects/:projectId/creative" element={<CreativeImageTextPage />} /></Routes></MemoryRouter>)
    expect(await screen.findByText('检测到 2 次重复交接，已合并显示', { exact: false })).toBeInTheDocument()
    expect(screen.getAllByText('策略包版本 v1')).toHaveLength(1)
    expect(screen.getByRole('region', { name: '图文任务' })).toBeInTheDocument()
    expect(screen.getByText('来自策略包 v1')).toBeInTheDocument()
  })

  it('saves the next draft revision and freezes the selected revision', async () => {
    vi.stubGlobal('fetch', vi.fn(async (input: string | URL | Request, init?: RequestInit) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url
      const method = init?.method || (input instanceof Request ? input.method : 'GET')
      if (url.endsWith('/creative-tasks?limit=100')) return jsonResponse({ items: [task] })
      if (url.endsWith('/creative-intakes?limit=100')) return jsonResponse({ items: [intake] })
      if (url.endsWith('/creative-tasks/creativetask_1')) return jsonResponse(detail)
      if (url.endsWith('/creative-tasks/creativetask_1:draft') && method === 'PATCH') return jsonResponse({ ...detail.draft, version: 2 })
      if (url.endsWith('/creative-tasks/creativetask_1:freeze-version') && method === 'POST') return jsonResponse({ id: 'creativeversion_1', organization_id: 'org_1', project_id: 'project_1', creative_task_id: task.id, version: 1, draft_version: 1, status: 'created', snapshot: detail.draft, content_hash: 'sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', created_by: 'usr_1', created_at: '2026-07-23T02:00:00Z' }, 201)
      return jsonResponse({ error: { code: 'NOT_FOUND', message: 'not found', request_id: 'req_test', retryable: false, details: [] } }, 404)
    }))
    render(<MemoryRouter initialEntries={['/projects/project_1/creative']}><Routes><Route path="/projects/:projectId/creative" element={<CreativeImageTextPage />} /></Routes></MemoryRouter>)
    expect(await screen.findByRole('button', { name: '编辑草稿' })).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: '编辑草稿' }))
    fireEvent.click(screen.getByRole('button', { name: '保存为下一版草稿' }))
    await waitFor(() => expect(fetch).toHaveBeenCalledWith('/api/creative/v1/projects/project_1/creative-tasks/creativetask_1:draft', expect.objectContaining({ method: 'PATCH' })))
    fireEvent.click(screen.getByRole('button', { name: '冻结为创意版本' }))
    expect(await screen.findByText('已冻结 CreativeVersion v1')).toBeInTheDocument()
  })
})
