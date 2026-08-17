import assert from 'node:assert/strict'
import test from 'node:test'
import { oceanEngineCalibrationDispositions, visibleOceanEngineManifestFields } from '../src/lib/oceanengineCalibrationManifest'

const configuration = {
  project: {
    marketing_purpose: 'lead_generation',
    marketing_scenario: 'short_video_image_text',
    product_selection_mode: 'manual',
    lead_capture_mode: 'custom_lead',
    carrier: 'orange_landing_page',
    optimization_target_reference: { namespace: 'oceanengine', object_kind: 'optimization_target', scope: 'account:test', state: 'unresolved', reason: 'pending' },
    delivery_mode: 'manual',
    targeting: { smart_expansion: false },
    schedule: { start_at: '2026-08-24T00:00:00Z', end_at: '2026-09-07T00:00:00Z', timezone: 'Asia/Shanghai' },
    budget_and_bidding: { currency: 'CNY', daily_budget_minor: 200000, bidding_strategy: 'stable_cost', charging_mode: 'CPC' },
    project_name: 'Local plan',
  },
  promotions: [{
    delivery_identity: { mode: 'account_info' },
    base_material_references: [{ id: 'asset_1', state: 'resolved' }],
    copy_items: [{ text: 'Local copy' }],
    promotion_name: 'Promotion',
  }],
}

test('the configuration page shows only usable Manifest fields', () => {
  const projectFields = visibleOceanEngineManifestFields(configuration, 'project')
  const promotionFields = visibleOceanEngineManifestFields(configuration, 'promotion', configuration.promotions[0])

  assert.deepEqual(projectFields.map(field => field.key), [
    'project.marketing_purpose',
    'project.marketing_scenario',
    'project.product_selection_mode',
    'project.lead_capture_mode',
    'project.carrier',
    'project.delivery_mode',
    'project.targeting',
    'project.schedule',
    'project.daily_budget_minor',
    'project.bidding_strategy',
    'project.charging_mode',
    'project.project_name',
  ])
  assert.equal(projectFields.find(field => field.key === 'project.daily_budget_minor')?.unit, 'CNY_fen')
  assert.equal(projectFields.find(field => field.key === 'project.product_selection_mode')?.label, '营销产品选择方式')
  assert.equal(projectFields.find(field => field.key === 'project.carrier')?.label, '投放载体')
  assert.deepEqual(promotionFields.map(field => field.key), [
    'promotion.delivery_identity',
    'promotion.base_materials',
    'promotion.promotion_name',
    'promotion.material_replacement_edit',
  ])
})

test('the calibration view preserves blocked and pending field reasons', () => {
  const project = oceanEngineCalibrationDispositions(configuration, 'project')
  const promotion = oceanEngineCalibrationDispositions(configuration, 'promotion')

  assert.equal(project.find(field => field.key === 'project.marketing_scenario')?.state, 'ready')
  assert.equal(project.find(field => field.key === 'project.carrier')?.state, 'ready')
  assert.equal(project.find(field => field.key === 'project.optimization_target_reference')?.state, 'missing_value')
  assert.equal(project.find(field => field.key === 'project.bid_minor')?.state, 'condition_unmet')
  assert.equal(promotion.find(field => field.key === 'promotion.bid')?.state, 'missing_value')
  assert.equal(promotion.find(field => field.key === 'promotion.category')?.state, 'missing_value')
})
