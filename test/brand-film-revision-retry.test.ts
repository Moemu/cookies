import assert from 'node:assert/strict'
import test from 'node:test'
import { CreativeApiError } from '../src/data/api.ts'
import { runWithLatestCreativeRevision } from '../src/features/brand-film/revisionRetry.ts'

test('brand film mutation refreshes and retries once after a draft revision conflict', async () => {
  const revisions: number[] = []
  let refreshes = 0

  const result = await runWithLatestCreativeRevision(
    39,
    async revision => {
      revisions.push(revision)
      if (revision === 39) throw new CreativeApiError('draft changed', 412)
      return `saved-at-${revision}`
    },
    async () => {
      refreshes += 1
      return 44
    },
  )

  assert.equal(result, 'saved-at-44')
  assert.deepEqual(revisions, [39, 44])
  assert.equal(refreshes, 1)
})

test('brand film mutation does not retry non-conflict failures', async () => {
  let refreshes = 0

  await assert.rejects(
    runWithLatestCreativeRevision(
      44,
      async () => { throw new CreativeApiError('provider failed', 500) },
      async () => { refreshes += 1; return 45 },
    ),
    /provider failed/,
  )

  assert.equal(refreshes, 0)
})
