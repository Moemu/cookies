import assert from 'node:assert/strict'
import test from 'node:test'
import { api, type ApiBrandBriefAnalysis } from '../src/data/api.ts'

test('brand film fixture creation is project-scoped and idempotent', async () => {
  const originalFetch = globalThis.fetch
  const calls: Array<{ url: string; init: RequestInit }> = []
  globalThis.fetch = async (input, init = {}) => {
    calls.push({ url: String(input), init })
    return jsonResponse({})
  }
  try {
    await api.ensureBrandFilmFixtureWorkspace('project_demo')
  } finally {
    globalThis.fetch = originalFetch
  }
  assert.equal(calls[0].url, '/api/creative/v1/projects/project_demo/creative-workspaces/brand-film:ensure-fixture')
  assert.equal(calls[0].init.method, 'POST')
  assert.equal(new Headers(calls[0].init.headers).get('Idempotency-Key'), 'brand-film-fixture-project_demo-guerlain-v1')
  assert.equal(calls[0].init.body, undefined)
})

test('brand Brief edits preserve confirmed uploaded asset lineage', async () => {
  const originalFetch = globalThis.fetch
  const calls: Array<{ url: string; init: RequestInit }> = []
  globalThis.fetch = async (input, init = {}) => {
    calls.push({ url: String(input), init })
    return jsonResponse({})
  }
  const analysis = {
    asset_candidates: [{
      id: 'product', role: 'product_front', label: '商品正面图', source_locator: 'manual-upload',
      asset_ref: { asset_id: 'asset_1', version: 2 }, rights_status: 'user_confirmed', user_confirmed: true,
    }],
  } as ApiBrandBriefAnalysis
  try {
    await api.updateBrandFilmBrief('project_demo', 'task_1', 4, analysis)
  } finally {
    globalThis.fetch = originalFetch
  }
  assert.equal(calls[0].url, '/api/creative/v1/projects/project_demo/creative-tasks/task_1/brand-film/brief')
  assert.equal(calls[0].init.method, 'PATCH')
  assert.deepEqual(JSON.parse(String(calls[0].init.body)), { expected_revision: 4, analysis })
})

test('brand-film commands bind selection and plan generation to the current revision', async () => {
  const originalFetch = globalThis.fetch
  const calls: Array<{ url: string; init: RequestInit }> = []
  globalThis.fetch = async (input, init = {}) => {
    calls.push({ url: String(input), init })
    return jsonResponse({})
  }
  try {
    await api.selectBrandFilmConcept('project_demo', 'task_1', 6, 'concept_01')
    await api.generateBrandFilmPlan('project_demo', 'task_1', 7)
  } finally {
    globalThis.fetch = originalFetch
  }
  assert.deepEqual(JSON.parse(String(calls[0].init.body)), { expected_revision: 6, concept_id: 'concept_01' })
  assert.equal(new Headers(calls[0].init.headers).get('Idempotency-Key'), 'brand-film-select-task_1-6-concept_01')
  assert.deepEqual(JSON.parse(String(calls[1].init.body)), { expected_revision: 7 })
  assert.equal(new Headers(calls[1].init.headers).get('Idempotency-Key'), 'brand-film-plan-task_1-7')
})

function jsonResponse(value: unknown, status = 200): Response {
  return new Response(JSON.stringify(value), { status, headers: { 'Content-Type': 'application/json' } })
}
