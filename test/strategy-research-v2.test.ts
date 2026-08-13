import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import test from 'node:test'

const root = resolve(import.meta.dirname, '..')
const workspace = readFileSync(resolve(root, 'src/features/strategy/KanonStrategyWorkspace.tsx'), 'utf8')
const api = readFileSync(resolve(root, 'src/features/strategy/api.ts'), 'utf8')
const hook = readFileSync(resolve(root, 'src/features/strategy/research/useResearchAdoptionProposals.ts'), 'utf8')
const orchestrator = readFileSync(resolve(root, 'internal/platform/knowledge/research_orchestrator.go'), 'utf8')

test('deep research UI exposes persisted rounds, honest verification, and non-blocking recovery', () => {
  assert.match(workspace, /后台运行 · 可继续编辑/)
  assert.match(workspace, /循环轮次/)
  assert.match(workspace, /每轮均已持久化，可从断点恢复/)
  assert.match(workspace, /模型引用 · 未核验正文/)
  assert.match(workspace, /正文与摘录已核验/)
  assert.match(workspace, /冲突来源/)
  assert.match(workspace, /从断点重试/)
  assert.doesNotMatch(workspace, /采纳到 Brief/)
	assert.match(workspace, /正在准备项目上下文/)
	assert.match(workspace, /!contextReady/)
})

test('research adoption requires explicit diff actions, expected versions, and idempotency', () => {
  assert.match(workspace, /当前值/)
  assert.match(workspace, /研究建议/)
  assert.match(workspace, /编辑后采纳/)
  assert.match(workspace, /重新映射/)
  assert.match(api, /research-adoption-proposals\/\$\{encodeURIComponent\(proposalId\)\}:apply/)
  assert.match(api, /Idempotency-Key.*research-proposal-apply/s)
  assert.match(api, /expected_version: expectedVersion/)
  assert.match(hook, /await onArtifactChanged\(\)/)
  assert.match(hook, /await refresh\(\)/)
})

test('server owns deep research stopping and never equates model citation with verification', () => {
  assert.match(orchestrator, /coverage_complete/)
  assert.match(orchestrator, /diminishing_returns/)
  assert.match(orchestrator, /token_budget/)
  assert.match(orchestrator, /time_budget/)
  assert.match(orchestrator, /len\(verifiedSupportingDomains\) >= 2/)
  assert.match(orchestrator, /record\(evidence, "conflicted"\)/)
})
