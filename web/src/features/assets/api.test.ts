// @vitest-environment node

import { afterEach, describe, expect, it, vi } from 'vitest'
import { applyRemixPreroll, createAgentRun, createEvalRun, createFeedbackEvent, createQualityReport, createRemixPlan, createRemixPreroll, createRemixRenderJob, getAssetFeature, getRenderJobQualityReport, getRemixPlan, getRemixRenderJob, listAssetFeatures, listRemixPlans, putAssetContent, removeProjectAsset, searchKnowledge, upsertAssetFeature } from './api'
import type { BulkRemixPlan } from './aiRemixPlanner'
import type { AssetFeature } from './types'

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

  it('reads and writes asset features through scoped platform endpoints', async () => {
    const fetchMock = vi.fn().mockImplementation(() => Promise.resolve(new Response(JSON.stringify({ items: [] }), { status: 200, headers: { 'Content-Type': 'application/json' } })))
    vi.stubGlobal('fetch', fetchMock)

    await listAssetFeatures('project/one')
    await getAssetFeature('project/one', 'asset/two', 3, 'vlm/version')
    await upsertAssetFeature('project/one', sampleAssetFeature())

    expect(fetchMock).toHaveBeenNthCalledWith(1, '/platform/v1/projects/project%2Fone/assets/features?limit=200', expect.objectContaining({ headers: expect.any(Headers) }))
    expect(fetchMock).toHaveBeenNthCalledWith(2, '/platform/v1/projects/project%2Fone/assets/asset%2Ftwo/versions/3/features/vlm%2Fversion', expect.objectContaining({ headers: expect.any(Headers) }))
    const [url, request] = fetchMock.mock.calls[2] as [string, RequestInit]
    expect(url).toBe('/platform/v1/projects/project%2Fone/assets/asset_1/versions/2/features/vlm-2026-07-26')
    expect(request.method).toBe('PUT')
    expect(JSON.parse(request.body as string)).toMatchObject({ schema_version: 'asset_feature_v1', hook_strength: 0.86 })
  })

  it('saves remix plans through the platform API contract', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ id: 'remixplan_1' }), { status: 201, headers: { 'Content-Type': 'application/json' } }))
    vi.stubGlobal('fetch', fetchMock)

    await createRemixPlan('project/one', sampleRemixPlan())

    const [url, request] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('/platform/v1/projects/project%2Fone/remix-plans')
    expect(request.method).toBe('POST')
    const body = JSON.parse(request.body as string) as Record<string, unknown>
    expect(body.schema_version).toBe('remix_plan_v2')
    expect(body.client_plan_id).toBe('remix_client_1')
    const segments = body.segments as Array<{
      shots: Array<{ asset_version: { asset_id: string; version: number }; timeline: { duration_seconds: number } }>
      clips: Array<{ asset_version: { asset_id: string; version: number } }>
    }>
    const summary = body.summary as { coverage_percent: number }
    expect(segments[0].shots[0].asset_version).toEqual({ asset_id: 'asset_1', version: 2 })
    expect(segments[0].shots[0].timeline.duration_seconds).toBe(4)
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

  it('creates and reads quality reports for remix render jobs', async () => {
    const fetchMock = vi.fn().mockImplementation(() => Promise.resolve(new Response(JSON.stringify({ id: 'qualityreport_1', quality_report: { id: 'qualityreport_1' } }), { status: 200, headers: { 'Content-Type': 'application/json' } })))
    vi.stubGlobal('fetch', fetchMock)

    await createQualityReport('project/one', 'job/two')
    await getRenderJobQualityReport('project/one', 'job/two')

    const [createUrl, createRequest] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(createUrl).toBe('/platform/v1/projects/project%2Fone/remix-quality-reports')
    expect(createRequest.method).toBe('POST')
    expect(JSON.parse(createRequest.body as string)).toEqual({
      render_job_id: 'job/two',
      policy: 'fail_critical',
    })
    expect(fetchMock).toHaveBeenNthCalledWith(2, '/platform/v1/projects/project%2Fone/remix-render-jobs/job%2Ftwo/quality-report', expect.objectContaining({ headers: expect.any(Headers) }))
  })

  it('calls task14 remix AI workflow endpoints', async () => {
    const fetchMock = vi.fn().mockImplementation(() => Promise.resolve(new Response(JSON.stringify({ id: 'ok', items: [] }), { status: 200, headers: { 'Content-Type': 'application/json' } })))
    vi.stubGlobal('fetch', fetchMock)

    await createRemixPreroll('project/one', 'plan/two', { asset_id: 'asset/three', version: 4 }, ['quality:critical'])
    await applyRemixPreroll('project/one', 'preroll/two')
    await createAgentRun('project/one', 'job/two')
    await searchKnowledge('project/one', 'hook citation')
    await createEvalRun('project/one')
    await createFeedbackEvent('project/one', 'asset_out', 4, '保留这条评论', { asset_id: 'asset_out', version: 1 })

    expect(fetchMock).toHaveBeenNthCalledWith(1, '/platform/v1/projects/project%2Fone/remix-prerolls', expect.objectContaining({ method: 'POST', headers: expect.any(Headers) }))
    expect(JSON.parse((fetchMock.mock.calls[0] as [string, RequestInit])[1].body as string)).toMatchObject({ plan_id: 'plan/two', reference_asset: { asset_id: 'asset/three', version: 4 }, style_constraints: ['quality:critical'] })
    expect(fetchMock).toHaveBeenNthCalledWith(2, '/platform/v1/projects/project%2Fone/remix-prerolls/preroll%2Ftwo/apply', expect.objectContaining({ method: 'POST', headers: expect.any(Headers) }))
    expect(fetchMock).toHaveBeenNthCalledWith(3, '/platform/v1/projects/project%2Fone/agent-runs', expect.objectContaining({ method: 'POST', headers: expect.any(Headers) }))
    expect(fetchMock).toHaveBeenNthCalledWith(4, '/platform/v1/projects/project%2Fone/knowledge/search?q=hook%20citation&limit=3', expect.objectContaining({ headers: expect.any(Headers) }))
    expect(fetchMock).toHaveBeenNthCalledWith(5, '/platform/v1/projects/project%2Fone/remix-eval-runs', expect.objectContaining({ method: 'POST', headers: expect.any(Headers) }))
    expect(fetchMock).toHaveBeenNthCalledWith(6, '/platform/v1/projects/project%2Fone/remix-feedback-events', expect.objectContaining({ method: 'POST', headers: expect.any(Headers) }))
    expect(JSON.parse((fetchMock.mock.calls[5] as [string, RequestInit])[1].body as string)).toMatchObject({ event_type: 'rating', target_type: 'asset', rating: 4, comment: '保留这条评论' })
  })
})

function sampleRemixPlan(): BulkRemixPlan {
  return {
    id: 'remix_client_1',
      schemaVersion: 'remix_plan_v2',
    targetSeconds: 30,
    actualSeconds: 21,
    pace: 'balanced',
    segments: [{
      segment: 'opening',
      label: '前段',
      targetSeconds: 8,
      actualSeconds: 4,
      shots: [{
        id: 'opening_asset_1_2',
        segment: 'opening',
        source: 'existing_asset',
        assetId: 'asset_1',
        version: 2,
        assetVersion: { asset_id: 'asset_1', version: 2 },
        timeline: {
          startSeconds: 0,
          durationSeconds: 4,
          inPointSeconds: 0,
          outPointSeconds: 4,
        },
        creative: {
          scene: '',
          shotType: 'close_up',
          cameraAngle: '',
          dialogueOrNarration: '',
          subtitle: '',
          transition: 'cut',
          ctaElement: '',
        },
        planning: {
          score: 0.82,
          reasonCodes: ['test'],
          reason: 'test',
          evidence: ['fixture'],
        },
        risks: [],
      }],
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
      shots: [],
      clips: [],
    }, {
      segment: 'ending',
      label: '后段',
      targetSeconds: 7,
      actualSeconds: 5,
      shots: [],
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

function sampleAssetFeature(): AssetFeature {
  return {
    organization_id: 'org_1',
    project_id: 'project_1',
    asset_id: 'asset_1',
    asset_version: 2,
    schema_version: 'asset_feature_v1',
    feature_version: 'vlm-2026-07-26',
    hook_strength: 0.86,
    product_visibility: 0.74,
    scene_tags: ['factory'],
    product_tags: ['cnc'],
    person_tags: ['engineer'],
    action_tags: ['cutting'],
    emotion_tags: ['trust'],
    selling_points: ['0.01mm precision'],
    cta_presence: true,
    similarity_group: 'precision-demo-a',
    similarity_risk: 'medium',
    evidence: ['00:00-00:03 strong hook'],
    created_at: '2026-07-26T10:00:00Z',
    updated_at: '2026-07-26T10:00:00Z',
  }
}
