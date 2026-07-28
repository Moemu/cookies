import { afterEach, describe, expect, it, vi } from 'vitest'
import { approveStrategy, getGenerationMetadata, listStrategyPackages, patchBriefField, sendMessage } from './api'
import type { BriefDraft, Review, StrategyDraft } from './types'

const draft: BriefDraft = {
  id: 'draft_1',
  brief_id: 'brief_1',
  status: 'open',
  version: 7,
  document: {
    contract_version: 'strategy-brief-version/v1',
    campaign: { objective: '新品认知' },
    audience: { primary: '' },
    proposition: '',
    channels: ['xiaohongshu'],
    budget: { total: '' },
    schedule: { window: '' },
    constraints: [],
    measurement: { primary_kpi: '' },
  },
  field_states: {},
  completeness: { ready: false, blockers: [], warnings: [] },
}

describe('Strategy API client', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('sends messages with a generated idempotency key', async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      void input
      void init
      return new Response('{}', { status: 202, headers: { 'Content-Type': 'application/json' } })
    })
    vi.stubGlobal('fetch', fetchMock)
    await sendMessage('conversation_1', '受众：研发负责人')
    const [url, init] = fetchMock.mock.calls[0]
    expect(url).toBe('/api/strategy/v1/conversations/conversation_1/messages')
    expect(init?.method).toBe('POST')
    expect(new Headers(init?.headers).get('Idempotency-Key')).toMatch(/^strategy-web-/)
  })

  it('carries the current Brief version in header and body', async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      void input
      void init
      return new Response(JSON.stringify(draft), { status: 200, headers: { 'Content-Type': 'application/json' } })
    })
    vi.stubGlobal('fetch', fetchMock)
    await patchBriefField('task_1', draft, 'audience.primary', '研发负责人')
    const [url, init] = fetchMock.mock.calls[0]
    expect(url).toBe('/api/strategy/v1/tasks/task_1/brief-draft')
    expect(init?.method).toBe('PATCH')
    expect(new Headers(init?.headers).get('If-Match')).toBe('"v7"')
    expect(init?.body).toBe(JSON.stringify({
      expected_version: 7,
      operations: [{ op: 'set', field_path: 'audience.primary', value: '研发负责人' }],
    }))
  })

  it('reuses a caller-provided approval key for timeout recovery', async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      void input
      void init
      return new Response('{}', { status: 201, headers: { 'Content-Type': 'application/json' } })
    })
    vi.stubGlobal('fetch', fetchMock)
    await approveStrategy(
      { id: 'strategy_1', version: 4 } as StrategyDraft,
      { id: 'review_1', candidate_content_hash: `sha256:${'a'.repeat(64)}` } as Review,
      'strategy-web-stable-retry',
    )
    const [, init] = fetchMock.mock.calls[0]
    expect(new Headers(init?.headers).get('Idempotency-Key')).toBe('strategy-web-stable-retry')
  })

  it('reads published packages by project so a revisited workspace can recover its next step', async () => {
    const fetchMock = vi.fn(async () => new Response(JSON.stringify({ items: [] }), { status: 200, headers: { 'Content-Type': 'application/json' } }))
    vi.stubGlobal('fetch', fetchMock)
    await listStrategyPackages('project_1')
    expect(fetchMock).toHaveBeenCalledWith('/api/strategy/v1/projects/project_1/strategy-packages', expect.anything())
  })

  it('keeps legacy revisions usable when generation metadata does not exist', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response(JSON.stringify({
      error: {
        code: 'NOT_FOUND',
        message: 'resource not found',
        request_id: 'request_1',
        retryable: false,
        details: [],
      },
    }), { status: 404, headers: { 'Content-Type': 'application/json' } })))

    await expect(getGenerationMetadata('legacy_strategy')).resolves.toBeNull()
  })
})
