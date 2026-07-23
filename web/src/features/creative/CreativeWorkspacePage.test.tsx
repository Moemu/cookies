import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { CreativeWorkspacePage } from './CreativeWorkspacePage'

function jsonResponse(body: unknown) {
  return new Response(JSON.stringify(body), { status: 200, headers: { 'Content-Type': 'application/json' } })
}

describe('CreativeWorkspacePage', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('creates an image Provider Job only from an approved creative plan', async () => {
    const plan = {
      id: 'plan_1', project_id: 'project_1', strategy_output_id: 'strategy_1', status: 'ready',
      model_alias: 'cookies.image.standard', image_prompt: '冷冻鳕鱼柳产品海报', video_prompt: '冷冻鳕鱼柳开箱视频',
      created_at: '2026-07-23T00:00:00Z', updated_at: '2026-07-23T00:00:00Z',
    }
    const job = {
      id: 'job_1', kind: 'image.generate', organization_id: 'org_1', project_id: 'project_1',
      execution_status: 'queued', provider_status: 'submitted', progress: 0, project_asset_refs: [],
      error: null, attempt_count: 0, max_attempts: 3, version: 1, created_at: '2026-07-23T00:00:00Z', updated_at: '2026-07-23T00:00:00Z',
    }
    const fetchMock = vi.fn(async (url: string, init?: RequestInit) => {
      if (url.endsWith('/context')) return jsonResponse({ organization_id: 'org_1', project_id: 'project_1', brand_id: 'brand_1', product_ids: [], project_context_version: 7 })
      if (url.endsWith('/plans') && init?.method === 'POST') return jsonResponse(plan)
      if (url.endsWith('/image-jobs')) return jsonResponse(job)
      throw new Error(`Unexpected request: ${url}`)
    })
    vi.stubGlobal('fetch', fetchMock)

    render(<MemoryRouter initialEntries={['/projects/project_1/creative?strategy=strategy_1']}><Routes><Route path="/projects/:projectId/creative" element={<CreativeWorkspacePage />} /></Routes></MemoryRouter>)
    expect(screen.getByRole('button', { name: '创建图像任务' })).toBeDisabled()
    fireEvent.click(screen.getByRole('button', { name: '创建创意计划' }))
    await screen.findByText('冷冻鳕鱼柳产品海报')
    fireEvent.click(screen.getByRole('button', { name: '创建图像任务' }))

    await waitFor(() => expect(screen.getByText('job_1')).toBeInTheDocument())
    expect(fetchMock).toHaveBeenCalledWith('/api/creative/v1/projects/project_1/plans/plan_1/image-jobs', expect.objectContaining({
      method: 'POST',
      body: JSON.stringify({ project_context_version: 7, width: 1024, height: 1024 }),
    }))
  })
})
