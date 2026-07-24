import { cleanup, render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { StrategyLanding, StrategyProjectHome } from './StrategyProjectHome'

const project = {
  id: 'project_1',
  organization_id: 'org_1',
  name: '新品项目',
  status: 'active' as const,
  primary_brand_id: 'brand_1',
  project_context_version: 1,
  created_at: '2026-07-23T00:00:00Z',
  updated_at: '2026-07-23T00:00:00Z',
}

describe('Strategy project home', () => {
  afterEach(() => {
    cleanup()
    vi.unstubAllGlobals()
  })

  it('lists resumable workspaces with canonical project routes', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response(JSON.stringify({
      items: [{ id: 'workspace_1', project_id: 'project_1', name: '主策略工作区', is_primary: true, status: 'active', version: 1 }],
    }), { status: 200, headers: { 'Content-Type': 'application/json' } })))
    render(<MemoryRouter><StrategyProjectHome project={project} /></MemoryRouter>)
    expect(await screen.findByRole('heading', { name: '主策略工作区' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /主策略工作区/ })).toHaveAttribute(
      'href',
      '/projects/project_1/strategy/workspaces/workspace_1/conversation',
    )
  })

  it('redirects the compatibility Strategy root to the canonical project route', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response(JSON.stringify({ items: [] }), {
      status: 200, headers: { 'Content-Type': 'application/json' },
    })))
    render(<MemoryRouter initialEntries={['/strategy']}><Routes>
      <Route element={<StrategyLanding project={project} />} path="/strategy" />
      <Route element={<StrategyProjectHome project={project} />} path="/projects/:projectId/strategy/workspaces" />
    </Routes></MemoryRouter>)
    expect(await screen.findByRole('heading', { name: '新品项目 · 策略' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /创建并开始/ })).toBeInTheDocument()
  })
})
