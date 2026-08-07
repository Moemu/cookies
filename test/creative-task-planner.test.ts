import assert from 'node:assert/strict'
import test from 'node:test'
import { createRouteRevisionChannelStrategy, findPublishedPackageForDraft } from '../src/features/strategy/creativeTaskPlanning'
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

test('route repair makes a concrete Xiaohongshu image strategy explicit without changing other channels', () => {
  const result = createRouteRevisionChannelStrategy([{
    platform: 'xiaohongshu', role: '搜索承接与种草获客', formats: ['精度检测实拍三联图'],
  }, {
    platform: 'douyin', role: '短视频触达', formats: ['short_video'],
  }])

  assert.equal(result.changed, true)
  assert.deepEqual(result.value[0].formats, ['精度检测实拍三联图', '小红书图文笔记'])
  assert.deepEqual(result.value[1].formats, ['short_video'])
})

test('route repair follows the selected brand video business instead of always adding image text', () => {
  const result = createRouteRevisionChannelStrategy([{
    platform: 'xiaohongshu', role: '搜索承接与销售转化', formats: ['精华成分图文'],
  }, {
    platform: 'douyin', role: '产品卖点转化', formats: ['海报'],
  }], 'brand_video')

  assert.equal(result.changed, true)
  assert.match(result.value[0].role, /品牌认知/)
  assert.match(result.value[1].role, /品牌认知/)
  assert.deepEqual(result.value[0].formats, ['精华成分图文', '品牌短视频'])
  assert.deepEqual(result.value[1].formats, ['海报', '品牌短视频'])
})
