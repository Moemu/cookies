import assert from 'node:assert/strict'
import test from 'node:test'
import { api, type ApiBrandBriefReview } from '../src/data/api.ts'

test('brand Brief review client prepares, updates, and confirms one intake revision', async () => {
  const originalFetch = globalThis.fetch
  const calls: Array<{ url: string; init: RequestInit }> = []
  globalThis.fetch = async (input, init = {}) => {
    calls.push({ url: String(input), init })
    return new Response(JSON.stringify({
      contract_version: 'creative-brand-brief-review/v1',
      intake_id: 'intake_1', status: 'draft', revision: 4,
      document: { summary: '已确认的创作输入' }, blockers: [], warnings: [],
    }), { status: 200, headers: { 'Content-Type': 'application/json' } })
  }
  const review = {
    revision: 4,
    document: { summary: '已确认的创作输入' },
  } as ApiBrandBriefReview
  try {
    await api.prepareBrandBriefReview('project 1', 'intake/1')
    await api.updateBrandBriefReview('project 1', 'intake/1', review)
    await api.confirmBrandBriefReview('project 1', 'intake/1', 5)
  } finally {
    globalThis.fetch = originalFetch
  }

  assert.deepEqual(calls.map(call => [call.url, call.init.method]), [
    ['/api/creative/v1/projects/project%201/creative-intakes/intake%2F1/brand-brief:prepare', 'POST'],
    ['/api/creative/v1/projects/project%201/creative-intakes/intake%2F1/brand-brief', 'PATCH'],
    ['/api/creative/v1/projects/project%201/creative-intakes/intake%2F1/brand-brief:confirm', 'POST'],
  ])
  assert.deepEqual(JSON.parse(String(calls[1].init.body)), {
    expected_revision: 4,
    document: { summary: '已确认的创作输入' },
  })
  assert.deepEqual(JSON.parse(String(calls[2].init.body)), { expected_revision: 5 })
})
