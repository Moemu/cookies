import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import test from 'node:test'

test('self confirmation uses one atomic publish command while formal review keeps submit and approve', async () => {
  const [apiSource, hookSource, workspaceSource, serverSource, openapi] = await Promise.all([
    readFile(new URL('../src/features/strategy/api.ts', import.meta.url), 'utf8'),
    readFile(new URL('../src/features/strategy/useStrategyWorkspace.ts', import.meta.url), 'utf8'),
    readFile(new URL('../src/features/strategy/KanonStrategyWorkspace.tsx', import.meta.url), 'utf8'),
    readFile(new URL('../internal/systems/strategy/review_confirm.go', import.meta.url), 'utf8'),
    readFile(new URL('../api/openapi/strategy-v1.yaml', import.meta.url), 'utf8'),
  ])

  assert.match(apiSource, /strategy-drafts\/\$\{encodeURIComponent\(draft\.id\)\}:confirm/)
  assert.match(apiSource, /strategy-drafts\/\$\{encodeURIComponent\(draft\.id\)\}:submit/)
  assert.match(apiSource, /strategy-drafts\/\$\{encodeURIComponent\(draft\.id\)\}:approve/)
  assert.match(hookSource, /reviewPolicy\?\.mode === 'self_confirmation'/)
  assert.match(hookSource, /strategyApi\.confirmStrategy/)
  assert.doesNotMatch(hookSource, /confirmStrategy[\s\S]{0,500}submitStrategy[\s\S]{0,500}approveStrategy/)
  assert.match(workspaceSource, /个人模式无需提交给自己评审/)
  assert.match(workspaceSource, /确认并发布策略包/)
  assert.match(workspaceSource, /提交正式评审/)
  assert.match(workspaceSource, /尚无可确认的策略版本/)
  assert.match(workspaceSource, /不需要提交给自己评审/)
  assert.match(serverSource, /publishReviewedStrategyInTx/)
  assert.match(serverSource, /strategy\.confirm/)
  assert.match(openapi, /strategy-drafts\/\{strategy_id\}:confirm/)
})

test('handoff is read-only against Strategy and exposes only task overlay mutations', async () => {
  const [plannerSource, workspaceSource, planningSource, packageSummarySource] = await Promise.all([
    readFile(new URL('../src/features/strategy/CreativeTaskPlanner.tsx', import.meta.url), 'utf8'),
    readFile(new URL('../src/features/strategy/KanonStrategyWorkspace.tsx', import.meta.url), 'utf8'),
    readFile(new URL('../src/features/strategy/creativeTaskPlanning.ts', import.meta.url), 'utf8'),
    readFile(new URL('../src/features/strategy/CreativeHandoffPackageSummary.tsx', import.meta.url), 'utf8'),
  ])

  assert.doesNotMatch(plannerSource, /patchStrategySection|onCreateRouteRevision|createRouteRevisionChannelStrategy/)
  assert.doesNotMatch(workspaceSource, /patchStrategySection\(['"]channel_strategy['"]/)
  assert.doesNotMatch(planningSource, /createRouteRevisionChannelStrategy/)
  assert.match(plannerSource, /patchCreativeTaskPlanAnswers/)
  assert.match(plannerSource, /handoffCreativeTaskStrategy/)
  assert.match(plannerSource, /返回策略查看修复建议/)
  assert.match(packageSummarySource, /strategyPackage\.content_hash/)
  assert.match(packageSummarySource, /handoff\.handoff_content_hash/)
  assert.match(packageSummarySource, /任务答案和 Overlay/)
  assert.match(packageSummarySource, /不会改变以上 Package 内容或哈希/)
})

test('a delayed review-policy read cannot overwrite an actively edited form', async () => {
  const source = await readFile(new URL('../src/features/strategy/KanonReviewCenter.tsx', import.meta.url), 'utf8')

  assert.match(source, /const policyFormDirty = useRef\(false\)/)
  assert.match(source, /if \(policyFormDirty\.current\) return/)
  assert.match(source, /policyFormDirty\.current = true[\s\S]{0,120}setMode/)
  assert.match(source, /disabled=\{!policy \|\| Boolean\(busy\)/)
})
