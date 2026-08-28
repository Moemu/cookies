import assert from 'node:assert/strict'
import test from 'node:test'
import { presentObjectAvailability, presentPlanBlockedReason } from '../src/features/browser-rpa-execution/objectAvailabilityPresentation'

test('shows a friendly action for an unbound brand', () => {
  const result = presentObjectAvailability({
    field_key: 'promotions.0.settings.brand_reference',
    object_kind: 'brand',
    internal_object_id: 'brand-other',
    display_name: '其他',
    available: false,
    reason: '未绑定巨量平台 ID',
  })

  assert.equal(result.scopeLabel, '单元 1')
  assert.equal(result.kindLabel, '品牌')
  assert.equal(result.statusLabel, '需处理')
  assert.equal(result.statusDetail, '请先为“其他”绑定巨量品牌 ID。')
})

test('does not show a full landing-page URL as its name or platform ID', () => {
  const landingPageUrl = 'https://market.m.taobao.com/app/starlink/wakeup-transit/pages/download?projectid=__PROJECT_ID__'
  const result = presentObjectAvailability({
    field_key: 'promotions.0.landing_page_reference',
    object_kind: 'owned_landing_page',
    internal_object_id: landingPageUrl,
    display_name: landingPageUrl,
    platform_object_id: landingPageUrl,
    available: true,
  })

  assert.equal(result.name, 'market.m.taobao.com 落地页')
  assert.equal(result.statusLabel, '已绑定')
  assert.equal(result.statusDetail, '落地页可用。')
  assert.equal(result.platformId, undefined)
})

test('keeps a normal platform ID and uses the business object name', () => {
  const result = presentObjectAvailability({
    field_key: 'project.marketing_product_reference',
    object_kind: 'product',
    internal_object_id: 'product-1',
    display_name: '限时福利快来薅羊毛啦',
    platform_object_id: '1786513565497554221',
    available: true,
  })

  assert.equal(result.scopeLabel, '项目')
  assert.equal(result.kindLabel, '营销商品')
  assert.equal(result.name, '限时福利快来薅羊毛啦')
  assert.equal(result.platformId, '1786513565497554221')
})

test('replaces the internal block code with a clear message', () => {
  assert.equal(
    presentPlanBlockedReason('PLATFORM_OBJECTS_UNAVAILABLE'),
    '有巨量对象尚未绑定。请处理下方标记为“需处理”的对象。',
  )
})
