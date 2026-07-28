import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { CreativePreRollPage } from './CreativePreRollPage'

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

describe('CreativePreRollPage', () => {
  beforeEach(() => {
    vi.stubGlobal('fetch', vi.fn(async (input: string | URL | Request) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url
      if (url.endsWith('/creative-intakes?limit=100')) return jsonResponse({ items: [] })
      if (url.endsWith('/assets?limit=100')) return jsonResponse({ items: [] })
      if (url.endsWith('/creative-tasks?limit=100')) return jsonResponse({ items: [] })
      if (url.endsWith('/creative-packages?limit=100')) return jsonResponse({ items: [] })
      return jsonResponse({
        error: {
          code: 'NOT_FOUND',
          message: 'not found',
          request_id: 'req_test',
          retryable: false,
          details: [],
        },
      }, 404)
    }))
  })

  afterEach(() => {
    cleanup()
    vi.unstubAllGlobals()
  })

  it('renders the project-scoped pre-roll workbench and explains missing prerequisites', async () => {
    render(
      <MemoryRouter initialEntries={['/projects/project_1/creative/video/performance/pre-roll']}>
        <Routes>
          <Route
            path="/projects/:projectId/creative/video/performance/pre-roll"
            element={<CreativePreRollPage />}
          />
        </Routes>
      </MemoryRouter>,
    )

    expect(screen.getByRole('heading', { name: '短剧前贴创作' })).toBeInTheDocument()
    expect(await screen.findByText('还没有包含前贴路线的已批准 StrategyPackage。')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '确认路线并创建任务' })).toBeDisabled()

    fireEvent.click(screen.getByRole('button', { name: '上传新的 MP4 主视频' }))
    expect(screen.getByText('拖拽 MP4 主视频到此处')).toBeInTheDocument()
    await waitFor(() => expect(fetch).toHaveBeenCalledTimes(4))
  })
})
