import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import test from 'node:test'
import { findPublishedPackageForDraft } from '../src/features/strategy/creativeTaskPlanning'
import type { PackageVersion } from '../src/features/strategy/types'

function strategyPackage(
  packageId: string,
  strategyId: string,
  strategyRevision: number,
  briefId = 'brief-current',
): PackageVersion {
  return {
    package_id: packageId,
    version: 1,
    content_hash: `sha256:${packageId}`,
    status: 'published',
    snapshot: {
      strategy_id: strategyId,
      strategy_revision: strategyRevision,
      brief: { brief_id: briefId, version: 1 } as PackageVersion['snapshot']['brief'],
      readiness: { creative_ready: true, delivery_ready: true, insights_ready: true },
    },
  }
}

test('creative task planning only binds the package for the current strategy revision', () => {
  const stalePackage = strategyPackage('stale', 'strategy-old', 3)
  const previousRevision = strategyPackage('previous', 'strategy-current', 1)
  const currentPackage = strategyPackage('current', 'strategy-current', 2)

  assert.equal(findPublishedPackageForDraft(
    [stalePackage, previousRevision, currentPackage],
    { brief_id: 'brief-current', version: 1 },
    { id: 'strategy-current', current_revision: 2 },
  )?.package_id, 'current')
})

test('creative task planning does not fall back to another strategy with the same Brief', () => {
  assert.equal(findPublishedPackageForDraft(
    [strategyPackage('stale', 'strategy-old', 1)],
    { brief_id: 'brief-current', version: 1 },
    { id: 'strategy-current', current_revision: 1 },
  ), null)
})

test('creative handoff cannot write back to Strategy and keeps task overlay as its only editable seam', async () => {
  const [plannerSource, workspaceSource] = await Promise.all([
    readFile(new URL('../src/features/strategy/CreativeTaskPlanner.tsx', import.meta.url), 'utf8'),
    readFile(new URL('../src/features/strategy/KanonStrategyWorkspace.tsx', import.meta.url), 'utf8'),
  ])

  assert.doesNotMatch(plannerSource, /onCreateRouteRevision|createRouteRevisionChannelStrategy|patchStrategySection/)
  assert.doesNotMatch(workspaceSource, /onCreateRouteRevision|patchStrategySection\(['"]channel_strategy['"]/)
  assert.match(plannerSource, /patchCreativeTaskPlanAnswers/)
  assert.match(plannerSource, /handoffCreativeTaskStrategy/)
  assert.match(plannerSource, /返回策略查看修复建议/)
})
