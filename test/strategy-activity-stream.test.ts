import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import test from 'node:test'
import { activityCursorKey, activityReconnectDelay, parseActivitySnapshot } from '../src/features/strategy/useActivityStream.js'
import { activityReconciliationPlan } from '../src/features/strategy/useStrategyWorkspace.js'
import type { TaskActivity, TaskActivitySnapshot } from '../src/features/strategy/types.js'

test('Activity reconnect delay is capped at five seconds plus bounded jitter', () => {
  assert.equal(activityReconnectDelay(0, 0), 500)
  assert.equal(activityReconnectDelay(2, 1), 2_150)
  assert.equal(activityReconnectDelay(99, 1), 5_000)
})

test('Activity reconnect cursors are isolated by authenticated owner and workspace', () => {
  const first = activityCursorKey('org_1:user_1:project_1', 'project_1', 'workspace_1')
  const otherUser = activityCursorKey('org_1:user_2:project_1', 'project_1', 'workspace_1')
  const otherWorkspace = activityCursorKey('org_1:user_1:project_1', 'project_1', 'workspace_2')
  assert.notEqual(first, otherUser)
  assert.notEqual(first, otherWorkspace)
  assert.match(first, /^strategy:activity-snapshot:/)
})

test('Activity stream accepts only complete v1 snapshots', () => {
  const snapshot = parseActivitySnapshot(JSON.stringify({
    contract_version: 'strategy-task-activity-snapshot/v1',
    snapshot_id: `sha256:${'a'.repeat(64)}`,
    captured_at: '2026-08-10T00:00:00Z',
    items: [],
  }))
  assert.equal(snapshot.items.length, 0)
  assert.throws(() => parseActivitySnapshot(JSON.stringify({
    contract_version: 'strategy-task-activity/v1',
    snapshot_id: 'delta:1',
    captured_at: '2026-08-10T00:00:00Z',
    items: [],
  })))
})

test('Activity reconciliation refreshes terminal resources and live research progress', () => {
  const running = activity({ id: 'research_1', status: 'running', resourceType: 'knowledge_research_run' })
  const previous = snapshot([running, activity({ id: 'document_1', status: 'running', resourceType: 'knowledge_document' })])
  const next = snapshot([
    { ...running, status: 'succeeded' },
    activity({ id: 'document_1', status: 'running', resourceType: 'knowledge_document' }),
    activity({
      id: 'agent_1', status: 'failed', resourceType: 'strategy_draft',
      executionType: 'strategy_agent_task', kind: 'strategy_generation',
    }),
  ])
  const plan = activityReconciliationPlan(previous, next)
  assert.deepEqual(plan.researchRunIds, ['research_1'])
  assert.equal(plan.refreshDocuments, false)
  assert.equal(plan.agentActivities.length, 1)
})

test('Activity reconciliation makes persisted research rounds and conclusions visible without a second poller', () => {
  const running = activity({ id: 'research_1', status: 'running', resourceType: 'knowledge_research_run' })
  const roundOne = {
    ...running,
    phase: 'searching',
    round: { current: 1, max: 6 },
    updated_at: '2026-08-10T00:01:00Z',
  }
  const roundTwo = {
    ...roundOne,
    phase: 'cross_checking',
    round: { current: 2, max: 6 },
    confirmed_conclusions: [{ id: 'finding_1', text: '已核验结论', status: 'verified', source_count: 2 }],
    updated_at: '2026-08-10T00:02:00Z',
  } satisfies TaskActivity

  assert.deepEqual(activityReconciliationPlan(snapshot([roundOne]), snapshot([roundTwo])).researchRunIds, ['research_1'])
  assert.deepEqual(activityReconciliationPlan(snapshot([roundTwo]), snapshot([roundTwo])).researchRunIds, [])
  assert.deepEqual(activityReconciliationPlan(snapshot([]), snapshot([roundOne])).researchRunIds, ['research_1'])
})

test('Strategy workspace no longer owns separate Agent, Research, or Document pollers', () => {
  const source = readFileSync(join(process.cwd(), 'src/features/strategy/useStrategyWorkspace.ts'), 'utf8')
  assert.doesNotMatch(source, /conversationResearchPollingKey/)
  assert.doesNotMatch(source, /const agentTaskId = state\.pendingAgentTaskId/)
  assert.doesNotMatch(source, /const researchRun = state\.researchRun/)
  assert.doesNotMatch(source, /const run = state\.researchRun/)
  assert.doesNotMatch(source, /state\.documents\.some\(document => document\.status === 'parse_queued'/)
})

test('overlapping workspace loads cannot restore stale bootstrap state after a mutation', () => {
  const source = readFileSync(join(process.cwd(), 'src/features/strategy/useStrategyWorkspace.ts'), 'utf8')
  assert.match(source, /const loadSequence = useRef\(0\)/)
  assert.match(source, /const sequence = \+\+loadSequence\.current/)
  assert.match(source, /if \(sequence !== loadSequence\.current\) return/)
})

function snapshot(items: TaskActivity[]): TaskActivitySnapshot {
  return {
    contract_version: 'strategy-task-activity-snapshot/v1',
    snapshot_id: `sha256:${'a'.repeat(64)}`,
    captured_at: '2026-08-10T00:00:00Z',
    items,
  }
}

function activity(input: {
  id: string
  status: TaskActivity['status']
  resourceType: string
  executionType?: string
  kind?: TaskActivity['kind']
}): TaskActivity {
  return {
    contract_version: 'strategy-task-activity/v1',
    id: input.id,
    kind: input.kind ?? 'deep_research',
    status: input.status,
    phase: input.status,
    round: null,
    progress: { kind: 'milestone', value: 50, message: '处理中' },
    summary: '处理中',
    confirmed_conclusions: [],
    source_scope: { project_id: 'project_1' },
    resource_ref: { type: input.resourceType, id: input.id },
    execution_ref: input.executionType ? { type: input.executionType, id: `execution_${input.id}`, version: 2 } : undefined,
    actions: ['open'],
    cancel_requested: false,
    heartbeat_at: '2026-08-10T00:00:00Z',
    updated_at: '2026-08-10T00:00:00Z',
  }
}
