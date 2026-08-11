import assert from 'node:assert/strict'
import test from 'node:test'
import { parseRoute } from '../src/lib/router.js'

test('the host router exposes URL-authoritative Strategy stage state', () => {
  const route = parseRoute('/projects/project_1/strategy/workspaces/workspace_1/review?panel=activity&resource=task_7')
  assert.equal(route.projectId, 'project_1')
  assert.equal(route.systemKey, 'strategy')
  assert.equal(route.navId, 'workspaces')
  assert.equal(route.objectId, 'workspace_1')
  assert.equal(route.strategyStage, 'review')
  assert.equal(route.strategyPanel, 'activity')
  assert.equal(route.strategyResource, 'task_7')
  assert.equal(route.strategyNeedsCanonicalRedirect, false)
})

test('legacy Strategy view routes expose their one-time canonical redirect target', () => {
  const route = parseRoute('/projects/project_1/strategy/workspaces/workspace_1?view=%E5%8F%98%E6%9B%B4%E8%AE%B0%E5%BD%95')
  assert.equal(route.strategyStage, 'strategy')
  assert.equal(route.strategyPanel, 'history')
  assert.equal(route.strategyNeedsCanonicalRedirect, true)
})
