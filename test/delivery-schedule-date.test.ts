import assert from 'node:assert/strict'
import test from 'node:test'

import { fromShanghaiEndDate, fromShanghaiStartDate, toShanghaiDateInput } from '../src/lib/deliverySchedule'

test('delivery schedule uses whole Shanghai calendar days', () => {
  assert.equal(toShanghaiDateInput('2026-08-24T16:00:00Z'), '2026-08-25')
  assert.equal(fromShanghaiStartDate('2026-08-26'), '2026-08-26T00:00:00+08:00')
  assert.equal(fromShanghaiEndDate('2026-08-31'), '2026-08-31T23:59:59+08:00')
})
