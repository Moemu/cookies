import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import test from 'node:test'
import { strategyApi } from '../src/features/strategy/api'
import type { StrategyDraft } from '../src/features/strategy/types'

test('AI second perspective binds directly to one immutable Strategy revision', async () => {
  const originalFetch = globalThis.fetch
  const calls: Array<{ url: string; init: RequestInit }> = []
  globalThis.fetch = async (input, init = {}) => {
    calls.push({ url: String(input), init })
    return new Response(JSON.stringify({
      analysis: { id: 'analysis_1', target_kind: 'strategy_revision', strategy_id: 'strategy_1', candidate_revision: 4, candidate_content_hash: `sha256:${'a'.repeat(64)}`, agent_task_id: 'agent_1', status: 'pending', findings: [] },
      agent_task: { id: 'agent_1' },
    }), { status: 202, headers: { 'Content-Type': 'application/json' } })
  }
  const draft = {
    id: 'strategy_1',
    current_revision: 4,
    revision: { content_hash: `sha256:${'a'.repeat(64)}` },
  } as StrategyDraft
  try {
    await strategyApi.startStrategyPerspective(draft, 'perspective-key')
  } finally {
    globalThis.fetch = originalFetch
  }

  assert.equal(calls[0].url, '/api/strategy/v1/strategy-drafts/strategy_1/perspective-analysis')
  assert.deepEqual(JSON.parse(String(calls[0].init.body)), {
    expected_revision: 4,
    expected_content_hash: `sha256:${'a'.repeat(64)}`,
  })
  assert.equal(new Headers(calls[0].init.headers).get('Idempotency-Key'), 'perspective-key')
})

test('second perspective remains optional and does not gate human review', () => {
  const workspace = readFileSync(join(process.cwd(), 'src/features/strategy/KanonStrategyWorkspace.tsx'), 'utf8')
  const hook = readFileSync(join(process.cwd(), 'src/features/strategy/useStrategyWorkspace.ts'), 'utf8')
  assert.match(workspace, /不改变人工评审状态/)
  assert.match(workspace, /不会阻止继续编辑、提交或确认/)
  assert.match(hook, /startStrategyPerspective\(state\.draft/)
  assert.doesNotMatch(hook, /只有进行中的评审可以启动深度分析/)
})
