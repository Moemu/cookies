import assert from 'node:assert/strict'
import test from 'node:test'
import {
  isStrategyPanel,
  isStrategyStage,
  parseStrategyWorkspaceRoute,
  resolveLegacyStrategyWorkspaceView,
  strategyWorkspacePath,
} from '../src/features/strategy/workspace/workspaceRoute.js'

test('legacy Strategy workspace views have one deterministic stage and panel destination', () => {
  const cases = [
    ['概览', { stage: 'intake' }],
    ['对话', { stage: 'intake' }],
    ['Brief', { stage: 'brief' }],
    ['研究', { stage: 'brief', panel: 'research' }],
    ['策略', { stage: 'strategy' }],
    ['创意任务策略', { stage: 'handoff' }],
    ['实验', { stage: 'strategy' }],
    ['评审', { stage: 'review' }],
    ['变更记录', { stage: 'strategy', panel: 'history' }],
  ] as const

  for (const [view, expected] of cases) {
    assert.deepEqual(resolveLegacyStrategyWorkspaceView(view), expected, view)
  }
  assert.deepEqual(resolveLegacyStrategyWorkspaceView('unknown-view'), { stage: 'intake' })
})

test('Strategy workspace paths carry stable stage IDs and optional contextual panels', () => {
  assert.equal(
    strategyWorkspacePath('project / 华东', 'workspace/1', { stage: 'brief', panel: 'research' }),
    '/projects/project%20%2F%20%E5%8D%8E%E4%B8%9C/strategy/workspaces/workspace%2F1/brief?panel=research',
  )
  assert.equal(strategyWorkspacePath('project_1', 'workspace_1', { stage: 'handoff' }), '/projects/project_1/strategy/workspaces/workspace_1/handoff')
  assert.equal(isStrategyStage('research'), false)
  assert.equal(isStrategyStage('review'), true)
  assert.equal(isStrategyPanel('activity'), true)
  assert.equal(isStrategyPanel('experiments'), false)
})

test('canonical Strategy workspace routes restore stage, panel, and selected resource', () => {
  const parsed = parseStrategyWorkspaceRoute('/projects/project%20one/strategy/workspaces/workspace%2F1/strategy?panel=materials&resource=document_17')
  assert.deepEqual(parsed, {
    projectId: 'project one',
    workspaceId: 'workspace/1',
    location: { stage: 'strategy', panel: 'materials', resource: 'document_17' },
    needsCanonicalRedirect: false,
  })
})

test('legacy and malformed Strategy workspace routes resolve once to a safe canonical destination', () => {
  assert.deepEqual(
    parseStrategyWorkspaceRoute('/projects/project_1/strategy/workspaces/workspace_1?view=%E7%A0%94%E7%A9%B6'),
    {
      projectId: 'project_1',
      workspaceId: 'workspace_1',
      location: { stage: 'brief', panel: 'research', resource: undefined },
      needsCanonicalRedirect: true,
    },
  )
  assert.deepEqual(
    parseStrategyWorkspaceRoute('/projects/project_1/strategy/workspaces/workspace_1/unknown?panel=experiments'),
    {
      projectId: 'project_1',
      workspaceId: 'workspace_1',
      location: { stage: 'intake', panel: undefined, resource: undefined },
      needsCanonicalRedirect: true,
    },
  )
})
