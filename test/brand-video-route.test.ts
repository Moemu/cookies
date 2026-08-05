import assert from 'node:assert/strict'
import test from 'node:test'
import { resolveBrandVideoRouteTarget } from '../src/features/creative/brandVideoRoute'

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
