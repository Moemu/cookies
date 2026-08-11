import assert from 'node:assert/strict'
import test from 'node:test'
import { systems } from '../src/data/navigation.js'

test('Strategy cross-workspace centers form one prominent shared-context hub', () => {
  const strategy = systems.find(system => system.key === 'strategy')
  assert.ok(strategy)

  const hubs = strategy.nav.filter(item => item.prominence === 'hub')
  assert.deepEqual(hubs.map(item => item.id), ['briefs', 'research', 'strategies', 'reviews'])
  assert.ok(hubs.every(item => item.group === '策略中枢'))
  assert.ok(hubs.every(item => item.navHint && item.navHint.length >= 6))
})
