import assert from 'node:assert/strict'
import test from 'node:test'
import {
  resolveBrandVideoRouteOptions,
  resolveBrandVideoRouteTarget,
  resolveBrandVideoRouteTargets,
  toggleBrandVideoChannel,
} from '../src/features/creative/brandVideoRoute'

test('brand video task inherits the selected route and Xiaohongshu channel', () => {
  assert.deepEqual(resolveBrandVideoRouteTarget({
    selected_route_id: 'route_brand_video',
    base_handoff: {
      routes: [{
        route_id: 'route_brand_video',
        deliverable_type: 'video',
        purpose: 'brand',
        channels: ['xiaohongshu'],
      }],
    },
  }), {
    selectedRouteId: 'route_brand_video',
    channel: 'xiaohongshu',
  })
})

test('brand video task rejects a route that cannot be produced', () => {
  assert.throws(() => resolveBrandVideoRouteTarget({
    request: {
      selected_route_id: 'route_brand_video',
      creative_routes: [{
        route_id: 'route_brand_video',
        route_type: 'brand_video',
        channels: ['wechat_official_account'],
      }],
    },
  }), /没有 Creative 支持的生产渠道/)
})

test('brand video task requires an explicit choice for a multi-channel route', () => {
  const intake = {
    selected_route_id: 'route_brand_video',
    base_handoff: {
      routes: [{
        route_id: 'route_brand_video',
        deliverable_type: 'video',
        purpose: 'brand',
        channels: ['douyin', 'xiaohongshu'],
      }],
    },
  }

  assert.deepEqual(resolveBrandVideoRouteOptions(intake), {
    selectedRouteId: 'route_brand_video',
    channels: ['douyin', 'xiaohongshu'],
  })
  assert.throws(() => resolveBrandVideoRouteTarget(intake), /请先明确选择本次生产渠道/)
  assert.deepEqual(resolveBrandVideoRouteTarget(intake, 'xiaohongshu'), {
    selectedRouteId: 'route_brand_video',
    channel: 'xiaohongshu',
  })
})

test('one confirmed master direction can create distinct channel adaptations', () => {
  const intake = {
    selected_route_id: 'route_brand_video',
    base_handoff: {
      routes: [{
        route_id: 'route_brand_video',
        deliverable_type: 'video',
        purpose: 'brand',
        channels: ['douyin', 'xiaohongshu'],
      }],
    },
  }

  assert.deepEqual(resolveBrandVideoRouteTargets(intake, ['xiaohongshu', 'douyin', 'xiaohongshu']), [
    { selectedRouteId: 'route_brand_video', channel: 'xiaohongshu' },
    { selectedRouteId: 'route_brand_video', channel: 'douyin' },
  ])
  assert.throws(() => resolveBrandVideoRouteTargets(intake, []), /至少选择一个渠道适配/)
  assert.throws(
    () => resolveBrandVideoRouteTargets(intake, ['kuaishou']),
    /渠道适配不在已冻结的品牌视频路线中/,
  )
})

test('channel adaptation selection supports independent toggles', () => {
  assert.deepEqual(toggleBrandVideoChannel([], 'xiaohongshu'), ['xiaohongshu'])
  assert.deepEqual(toggleBrandVideoChannel(['xiaohongshu'], 'douyin'), ['xiaohongshu', 'douyin'])
  assert.deepEqual(toggleBrandVideoChannel(['xiaohongshu', 'douyin'], 'xiaohongshu'), ['douyin'])
})
