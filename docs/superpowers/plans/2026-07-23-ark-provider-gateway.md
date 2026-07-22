# Ark Provider Gateway Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a server-only Ark gateway that safely exposes text generation and asynchronous image/video job APIs.

**Architecture:** A dependency-free Node HTTP server owns all environment access and maps logical capabilities to fixed Ark model IDs. The provider adapter receives the credential only in its request headers, while public endpoints return sanitized capability data and normalized errors. Generation jobs are persisted to a local JSON file so polling and cancellation remain stable across process restarts.

**Tech Stack:** Node.js 20 ESM, native `http`, `fs/promises`, and `node:test`.

---

### Task 1: Define safe provider configuration

**Files:**
- Create: `.env.example`
- Create: `server/config.mjs`
- Test: `server/test/gateway.contract.test.mjs`

- [ ] **Step 1: Write failing configuration tests**

```js
assert.equal(loadArkConfig({}).configured, false)
assert.equal(loadArkConfig({ ARK_API_KEY: 'test-token' }).models.text, 'doubao-seed-2-1-pro-260628')
```

- [ ] **Step 2: Implement the model catalog and public capability projection**

```js
export const MODEL_CATALOG = Object.freeze({
  text: 'doubao-seed-2-1-pro-260628',
  image: 'doubao-seedream-5-0-pro-260628',
  video: 'doubao-seedance-2-0-fast-260128',
  embedding: 'doubao-embedding-vision-251215',
})
```

- [ ] **Step 3: Run the contract test**

Run: `npm run test:server`

Expected: PASS without a real credential.

### Task 2: Add provider and durable jobs

**Files:**
- Create: `server/errors.mjs`
- Create: `server/ark-provider.mjs`
- Create: `server/job-repository.mjs`
- Create: `server/generation-service.mjs`
- Test: `server/test/gateway.contract.test.mjs`

- [ ] **Step 1: Write failing provider/error and lifecycle tests**

```js
assert.equal(response.status, 503)
assert.equal(body.error.code, 'PROVIDER_NOT_CONFIGURED')
assert.equal(job.status, 'cancelled')
```

- [ ] **Step 2: Implement an injected-fetch Ark adapter and normalized errors**

```js
throw new AppError('PROVIDER_REQUEST_FAILED', '模型服务暂时不可用，请稍后重试。', 502)
```

- [ ] **Step 3: Implement JSON-backed create, read, and update job operations**

```js
await repository.create({ id, kind, model, status: 'queued' })
await repository.update(id, { status: 'cancelled', cancelledAt: now })
```

- [ ] **Step 4: Run the contract test**

Run: `npm run test:server`

Expected: PASS with no outbound network calls.

### Task 3: Expose generation HTTP APIs

**Files:**
- Create: `server/http-server.mjs`
- Create: `server/index.mjs`
- Modify: `package.json`
- Test: `server/test/gateway.contract.test.mjs`

- [ ] **Step 1: Write failing HTTP contract tests**

```js
await request('POST', '/api/generation/text', { prompt: '生成策略摘要' })
await request('POST', '/api/generation/media', { kind: 'image', prompt: '商品主视觉' })
await request('POST', `/api/generation/jobs/${job.id}/cancel`)
```

- [ ] **Step 2: Implement capability, text, media, query, and cancel routes**

```js
GET  /api/provider/capabilities
POST /api/generation/text
POST /api/generation/media
GET  /api/generation/jobs/:id
POST /api/generation/jobs/:id/cancel
```

- [ ] **Step 3: Add repeatable server commands**

```json
{
  "test:server": "node --test server/test/**/*.test.mjs",
  "dev:server": "node --watch server/index.mjs"
}
```

- [ ] **Step 4: Verify tests, build, and credential hygiene**

Run: `npm run test:server && npm run build`

Expected: all tests and the existing frontend build pass; `.env.example` contains no secret value.
