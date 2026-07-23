import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { StrategyWorkspacePage } from './StrategyWorkspacePage'

function jsonResponse(body: unknown) {
  return new Response(JSON.stringify(body), { status: 200, headers: { 'Content-Type': 'application/json' } })
}

describe('StrategyWorkspacePage', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('renders proposal status and sends approval from the strategy workspace', async () => {
    const proposal = {
      id: 'proposal_1', project_id: 'project_1', status: 'created', template_version: 'volcad-v1',
      input: { brand: '极地鲜生', product: '深海鳕鱼柳', target_audience: '城市家庭', platform: '抖音', budget: '618', compliance: ['禁用绝对化用语'] },
      created_at: '2026-07-23T00:00:00Z', updated_at: '2026-07-23T00:00:00Z',
    }
    const generated = { ...proposal, status: 'generated', strategy: { id: 'strategy_1', proposal_id: 'proposal_1', status: 'draft', content: { headline: '今晚吃点海的鲜' } } }
    const approved = { ...generated.strategy, status: 'approved', approved_at: '2026-07-23T00:01:00Z' }
    const fetchMock = vi.fn(async (url: string, init?: RequestInit) => {
      if (url.endsWith('/context')) return jsonResponse({ organization_id: 'org_1', project_id: 'project_1', brand_id: 'brand_1', product_ids: [], project_context_version: 3 })
      if (url.endsWith('/proposals') && init?.method === 'POST') return jsonResponse(proposal)
      if (url.endsWith('/generate')) return jsonResponse(generated)
      if (url.endsWith('/approve')) return jsonResponse(approved)
      throw new Error(`Unexpected request: ${url}`)
    })
    vi.stubGlobal('fetch', fetchMock)

    render(<MemoryRouter initialEntries={['/projects/project_1/strategy']}><Routes><Route path="/projects/:projectId/strategy" element={<StrategyWorkspacePage />} /></Routes></MemoryRouter>)
    fireEvent.click(await screen.findByRole('button', { name: '创建项目提案' }))
    await screen.findByText('proposal_1')
    fireEvent.click(screen.getByRole('button', { name: '生成策略' }))
    await screen.findByText((_, element) => element?.className === 'strategy-json'
      && element.textContent?.includes('今晚吃点海的鲜') === true)
    fireEvent.click(screen.getByRole('button', { name: '审批策略' }))

    await waitFor(() => expect(screen.getByRole('button', { name: '策略已审批' })).toBeDisabled())
    expect(fetchMock).toHaveBeenCalledWith('/api/strategy/v1/projects/project_1/strategies/strategy_1/approve', expect.objectContaining({ method: 'POST' }))
    expect(screen.getByRole('link', { name: '进入创意工作区' })).toHaveAttribute('href', '/projects/project_1/creative?strategy=strategy_1')
  })
})
