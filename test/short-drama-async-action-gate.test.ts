import assert from 'node:assert/strict'
import test from 'node:test'
import { createAsyncActionGate } from '../src/features/short-drama-preroll-v2/asyncActionGate'

test('short drama direction selection gate rejects a concurrent duplicate request', async () => {
  const gate = createAsyncActionGate()
  let release!: () => void
  const blocker = new Promise<void>(resolve => { release = resolve })
  let requestCount = 0

  const first = gate.run(async () => {
    requestCount += 1
    await blocker
    return 'selected'
  })
  const duplicate = await gate.run(async () => {
    requestCount += 1
    return 'duplicate'
  })

  assert.deepEqual(duplicate, { started: false })
  assert.equal(requestCount, 1)
  release()
  assert.deepEqual(await first, { started: true, value: 'selected' })
  assert.equal(gate.isActive(), false)
})
