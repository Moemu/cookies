import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { StrategyReviewPage } from './StrategyReviewPage'

const review = {
  id: 'review_1',
  organization_id: 'org_1',
  project_id: 'project_1',
  strategy_id: 'strategy_1',
  candidate_revision: 2,
  candidate_content_hash: 'a'.repeat(64),
  brief_id: 'brief_1',
  brief_version: 1,
  project_context_version: 1,
  status: 'open',
  created_by: 'user_1',
  created_at: '2026-07-24T00:00:00Z',
  updated_at: '2026-07-24T00:00:00Z',
}

const draft = {
  id: 'strategy_1',
  project_id: 'project_1',
  status: 'in_review',
  current_revision: 2,
  version: 3,
}

const revisions = {
  items: [
    {
      strategy_id: 'strategy_1',
      revision: 1,
      changed_sections: ['all'],
      content_hash: 'b'.repeat(64),
      created_by: 'user_1',
      created_at: '2026-07-24T00:00:00Z',
      document: { objective: '新品认知' },
    },
    {
      strategy_id: 'strategy_1',
      revision: 2,
      base_revision: 1,
      changed_sections: ['objective'],
      content_hash: 'a'.repeat(64),
      created_by: 'user_1',
      created_at: '2026-07-24T01:00:00Z',
      document: { objective: '新品成交' },
    },
  ],
}

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })
}

describe('StrategyReviewPage', () => {
  afterEach(() => {
    cleanup()
    vi.unstubAllGlobals()
  })

  it('shows candidate differences and requires a reason before returning review', async () => {
    vi.stubGlobal('fetch', vi.fn(async (input: string | URL | Request, init?: RequestInit) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url
      const method = init?.method || 'GET'
      if (url === '/api/strategy/v1/strategy-reviews/review_1' && method === 'GET') return jsonResponse(review)
      if (url === '/api/strategy/v1/strategy-drafts/strategy_1' && method === 'GET') return jsonResponse(draft)
      if (url === '/api/strategy/v1/strategy-drafts/strategy_1/revisions') return jsonResponse(revisions)
      if (url === '/api/strategy/v1/strategy-reviews/review_1/comments') return jsonResponse({ items: [] })
      if (url === '/api/strategy/v1/strategy-reviews/review_1:return') {
        return jsonResponse({ ...review, status: 'returned', decision_reason: '补充证据' })
      }
      return jsonResponse({}, 404)
    }))

    render(<MemoryRouter initialEntries={['/projects/project_1/strategy/reviews/review_1']}>
      <Routes>
        <Route element={<StrategyReviewPage />} path="/projects/:projectId/strategy/reviews/:reviewId" />
      </Routes>
    </MemoryRouter>)

    expect(await screen.findByRole('heading', { name: 'Revision 2 评审' })).toBeInTheDocument()
    expect(screen.getByText('新品认知')).toBeInTheDocument()
    expect(screen.getByText('新品成交')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '退回修改' })).toBeDisabled()

    fireEvent.change(screen.getByPlaceholderText('退回原因（必填）'), { target: { value: '补充证据' } })
    fireEvent.click(screen.getByRole('button', { name: '退回修改' }))

    await waitFor(() => expect(fetch).toHaveBeenCalledWith(
      '/api/strategy/v1/strategy-reviews/review_1:return',
      expect.objectContaining({ method: 'POST', body: JSON.stringify({ reason: '补充证据' }) }),
    ))
    expect(await screen.findByText('该评审已结束。')).toBeInTheDocument()
  })
})
