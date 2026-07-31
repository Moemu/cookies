import assert from 'node:assert/strict'
import test from 'node:test'
import { api } from '../src/data/api.ts'

test('authentication uses the Go platform routes and supports an empty logout response', async () => {
  const originalFetch = globalThis.fetch
  const calls: Array<{ url: string; init?: RequestInit }> = []
  globalThis.fetch = async (input, init) => {
    const url = String(input)
    calls.push({ url, init })
    if (url.endsWith('/auth/login')) {
      return new Response(JSON.stringify({ actor: { principal: { id: 'user_admin' } } }), { status: 200 })
    }
    if (url.endsWith('/context')) {
      return new Response(JSON.stringify({ actor: { principal: { id: 'user_admin' } } }), { status: 200 })
    }
    if (url.endsWith('/auth/logout')) return new Response(null, { status: 204 })
    throw new Error(`unexpected request ${url}`)
  }

  try {
    assert.deepEqual(await api.login({ username: 'Admin', password: '123456' }), {
      authenticated: true,
      user: { id: 'user_admin', email: '', displayName: 'Admin' },
    })
    assert.equal(await api.getSession().then((session) => session.authenticated), true)
    assert.deepEqual(await api.logout(), { authenticated: false })
  } finally {
    globalThis.fetch = originalFetch
  }

  assert.equal(calls[0].url, '/platform/v1/auth/login')
  assert.equal(calls[0].init?.method, 'POST')
  assert.deepEqual(JSON.parse(String(calls[0].init?.body)), { username: 'Admin', password: '123456' })
  assert.equal(calls[1].url, '/platform/v1/context')
  assert.equal(calls[2].url, '/platform/v1/auth/logout')
})
