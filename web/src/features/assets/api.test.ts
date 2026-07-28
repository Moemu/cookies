import { afterEach, describe, expect, it, vi } from 'vitest'
import { createRemixPlan, createRemixRenderJob, getRemixPlan, getRemixRenderJob, listRemixPlans, putAssetContent, removeProjectAsset } from './api'
import type { BulkRemixPlan } from './aiRemixPlanner'

describe('asset upload API', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('keeps signed TOS headers while leaving forbidden transport headers to the browser', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(null, { status: 204 }))
    vi.stubGlobal('fetch', fetchMock)
    const file = new File(['image'], 'asset.png', { type: 'image/png' })

    await putAssetContent({
      url: 'https://tos.example.com/signed-upload',
      method: 'PUT',
      headers: {
        Host: 'tos.example.com',
        'Content-Length': String(file.size),
        'Content-Type': 'image/png',
        'x-tos-forbid-overwrite': 'true',
      },
      expires_at: '2026-07-22T01:00:00Z',
    }, file)

    const [, request] = fetchMock.mock.calls[0] as [string, RequestInit]
    const headers = new Headers(request.headers)
    expect(headers.get('host')).toBeNull()
    expect(headers.get('content-length')).toBeNull()
    expect(headers.get('content-type')).toBe('image/png')
    expect(headers.get('x-tos-forbid-overwrite')).toBe('true')
    expect(request.body).toBe(file)
  })

  it('removes only the selected project asset relationship', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(null, { status: 204 }))
    vi.stubGlobal('fetch', fetchMock)

    await removeProjectAsset('project/one', 'asset/two', 3)

    expect(fetchMock).toHaveBeenCalledWith('/platform/v1/projects/project%2Fone/assets/asset%2Ftwo/versions/3', expect.objectContaining({ method: 'DELETE' }))
  })

  it('saves remix plans through the platform API contract', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ id: 'remixplan_1' }), { status: 201, headers: { 'Content-Type': 'application/json' } }))
    vi.stubGlobal('fetch', fetchMock)

    await createRemixPlan('project/one', sampleRemixPlan())

    const [url, request] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('/platform/v1/projects/project%2Fone/remix-plans')
    expect(request.method).toBe('POST')
    const body = JSON.parse(request.body as string) as Record<string, unknown>
    expect(body.client_plan_id).toBe('remix_client_1')
    const segments = body.segments as Array<{ clips: Array<{ asset_version: { asset_id: string; version: number } }> }>
    const summary = body.summary as { coverage_percent: number }
    expect(segments[0].clips[0].asset_version).toEqual({ asset_id: 'asset_1', version: 2 })
    expect(summary.coverage_percent).toBe(70)
  })

  it('reads saved remix plans with encoded identifiers', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ id: 'remixplan_1' }), { status: 200, headers: { 'Content-Type': 'application/json' } }))
    vi.stubGlobal('fetch', fetchMock)

    await getRemixPlan('project/one', 'plan/two')

    expect(fetchMock).toHaveBeenCalledWith('/platform/v1/projects/project%2Fone/remix-plans/plan%2Ftwo', expect.objectContaining({ headers: expect.any(Headers) }))
  })

  it('lists recent remix plans through the platform API', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ items: [] }), { status: 200, headers: { 'Content-Type': 'application/json' } }))
    vi.stubGlobal('fetch', fetchMock)

    await listRemixPlans('project/one', 8)

    expect(fetchMock).toHaveBeenCalledWith('/platform/v1/projects/project%2Fone/remix-plans?limit=8', expect.objectContaining({ headers: expect.any(Headers) }))
  })

  it('creates remix render jobs through the platform API contract', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ id: 'remixrender_1' }), { status: 202, headers: { 'Content-Type': 'application/json' } }))
    vi.stubGlobal('fetch', fetchMock)

    await createRemixRenderJob('project/one', 'plan/two', 'draft')

    const [url, request] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('/platform/v1/projects/project%2Fone/remix-render-jobs')
    expect(request.method).toBe('POST')
    expect(JSON.parse(request.body as string)).toEqual({
      plan_id: 'plan/two',
      target_format: 'mp4',
      target_quality: 'draft',
    })
  })

  it('reads remix render jobs with encoded identifiers', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ id: 'remixrender_1' }), { status: 200, headers: { 'Content-Type': 'application/json' } }))
    vi.stubGlobal('fetch', fetchMock)

    await getRemixRenderJob('project/one', 'job/two')

    expect(fetchMock).toHaveBeenCalledWith('/platform/v1/projects/project%2Fone/remix-render-jobs/job%2Ftwo', expect.objectContaining({ headers: expect.any(Headers) }))
  })
})

function sampleRemixPlan(): BulkRemixPlan {
  return {
    id: 'remix_client_1',
    targetSeconds: 30,
    actualSeconds: 21,
    pace: 'balanced',
    segments: [{
      segment: 'opening',
      label: '前段',
      targetSeconds: 8,
      actualSeconds: 4,
      clips: [{
        id: 'opening_asset_1_2',
        segment: 'opening',
        assetId: 'asset_1',
        version: 2,
        label: 'asset_1 · v2',
        sourceType: 'upload',
        mimeType: 'video/mp4',
        aspect: 'vertical',
        startSeconds: 0,
        durationSeconds: 4,
        inPointSeconds: 0,
        outPointSeconds: 4,
        score: 0.82,
        reason: 'test',
      }],
    }, {
      segment: 'middle',
      label: '中段',
      targetSeconds: 15,
      actualSeconds: 12,
      clips: [],
    }, {
      segment: 'ending',
      label: '后段',
      targetSeconds: 7,
      actualSeconds: 5,
      clips: [],
    }],
    warnings: [],
    summary: {
      selectedAssets: 1,
      usedAssets: 1,
      coveragePercent: 70,
      strategy: 'balanced',
    },
  }
}
