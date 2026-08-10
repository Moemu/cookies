import assert from 'node:assert/strict'
import test from 'node:test'
import { api, ApiRequestError } from '../src/data/api.ts'

test('Miyun client uses project-scoped escaped routes, versions, and idempotency keys', async () => {
  const originalFetch = globalThis.fetch
  const calls: Array<{ url: string; init: RequestInit }> = []
  globalThis.fetch = async (input, init = {}) => {
    calls.push({ url: String(input), init })
    return jsonResponse({})
  }
  try {
    await api.createMiyunCrawlJob('project / one', { product_profile_id: 'profile_1', operation: 'product' }, 'key-1')
    await api.cancelMiyunCrawlJob('project / one', 'job / one', 7)
    await api.retryMiyunCrawlJob('project / one', 'job / one', 'key-2')
    await api.confirmMiyunMaterial('project / one', 'material / one', 8, 'approved')
    await api.rejectMiyunMaterial('project / one', 'material / one', 9)
    await api.retryMiyunMaterialImport('project / one', 'material / one', 10)
  } finally { globalThis.fetch = originalFetch }
  assert.equal(calls[0].url, '/api/insights/v1/projects/project%20%2F%20one/miyun/crawl-jobs')
  assert.equal(new Headers(calls[0].init.headers).get('Idempotency-Key'), 'key-1')
  assert.deepEqual(JSON.parse(String(calls[0].init.body)), { product_profile_id: 'profile_1', operation: 'product' })
  assert.deepEqual(JSON.parse(String(calls[1].init.body)), { expected_version: 7 })
  assert.equal(new Headers(calls[2].init.headers).get('Idempotency-Key'), 'key-2')
  assert.equal(calls[2].init.body, undefined)
  assert.deepEqual(JSON.parse(String(calls[3].init.body)), { expected_version: 8, note: 'approved' })
  assert.deepEqual(JSON.parse(String(calls[4].init.body)), { expected_version: 9 })
  assert.deepEqual(JSON.parse(String(calls[5].init.body)), { expected_version: 10 })
})

test('Miyun material preview stays same-origin and never returns the upstream source URL', () => {
  assert.equal(
    api.getMiyunMaterialPreviewUrl('project / one', 'material / one'),
    '/api/insights/v1/projects/project%20%2F%20one/miyun/materials/material%20%2F%20one/preview',
  )
})

test('Miyun product source and analysis permit a manual product name without product_id', async () => {
  const originalFetch = globalThis.fetch
  const calls: Array<{ url: string; init: RequestInit }> = []
  globalThis.fetch = async (input, init = {}) => {
    calls.push({ url: String(input), init })
    return jsonResponse({})
  }
  try {
    await api.getMiyunProductSource('project / one')
    await api.analyzeMiyunProductProfile('project / one', {
      connection_id: 'connection_1', product_name: 'Manual product', category_name: 'Skin care',
      product_asset_refs: [{ asset_id: 'asset_1', version: 2 }], knowledge_document_ids: ['doc_1'],
    })
  } finally { globalThis.fetch = originalFetch }
  assert.equal(calls[0].url, '/api/insights/v1/projects/project%20%2F%20one/miyun/product-source')
  assert.equal(calls[0].init.method, 'GET')
  assert.equal(calls[1].url, '/api/insights/v1/projects/project%20%2F%20one/miyun/product-profiles:analyze')
  assert.deepEqual(JSON.parse(String(calls[1].init.body)), {
    connection_id: 'connection_1', product_name: 'Manual product', category_name: 'Skin care',
    product_asset_refs: [{ asset_id: 'asset_1', version: 2 }], knowledge_document_ids: ['doc_1'],
  })
})

test('Miyun conflict preserves HTTP problem details for UI decisions', async () => {
  const originalFetch = globalThis.fetch
  globalThis.fetch = async () => jsonResponse({ error: { code: 'VERSION_CONFLICT', message: 'refresh' } }, 409)
  try {
    await assert.rejects(
      api.confirmMiyunMaterial('project_1', 'material_1', 3),
      (error: unknown) => error instanceof ApiRequestError && error.status === 409 && error.code === 'VERSION_CONFLICT',
    )
  } finally { globalThis.fetch = originalFetch }
})

function jsonResponse(value: unknown, status = 200): Response {
  return new Response(JSON.stringify(value), { status, headers: { 'Content-Type': 'application/json' } })
}
