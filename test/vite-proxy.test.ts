import assert from 'node:assert/strict'
import test from 'node:test'
import viteConfig from '../vite.config.ts'

test('Creative API requests are proxied to the Go product API before the compatibility API', () => {
  assert.equal(typeof viteConfig, 'object')
  if (typeof viteConfig !== 'object' || viteConfig === null) {
    throw new Error('vite config must be an object')
  }

  const proxy = viteConfig.server?.proxy
  assert.ok(proxy && !Array.isArray(proxy), 'vite proxy configuration is required')
  const entries = Object.entries(proxy)

  assert.equal(entries[0]?.[0], '/api/creative/v1')
  assert.equal(entries[0]?.[1], process.env.VITE_PLATFORM_PROXY_TARGET ?? 'http://127.0.0.1:8080')
  assert.equal(entries.find(([path]) => path === '/api')?.[1], process.env.VITE_COMPAT_API_PROXY_TARGET ?? 'http://127.0.0.1:8787')
})
