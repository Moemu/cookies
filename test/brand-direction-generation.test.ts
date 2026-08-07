import assert from 'node:assert/strict'
import test from 'node:test'
import type { ApiCreativeDirectionBatch } from '../src/data/api'
import {
  activeBrandVideoTasks,
  availableBrandDirections,
  brandDirectionFailureMessage,
  brandVideoTaskStatusLabel,
  isBrandDirectionGenerating,
  isChannelNeutralBrandDirectionBatch,
} from '../src/features/creative/brandDirectionGeneration'

function batch(status: ApiCreativeDirectionBatch['status']): ApiCreativeDirectionBatch {
  return {
    contract_version: 'creative-direction-candidate-batch/v1',
    batch_id: 'directionbatch_1',
    intake_id: 'intake_1',
    status,
    candidates: [],
    created_at: '2026-08-05T00:00:00Z',
  }
}

test('brand direction generation keeps persisted generating state visible', () => {
  assert.equal(isBrandDirectionGenerating(batch('generating')), true)
  assert.equal(isBrandDirectionGenerating(batch('ready')), false)
  assert.equal(isBrandDirectionGenerating(null), false)
})

test('brand direction failure codes produce actionable messages', () => {
  assert.match(brandDirectionFailureMessage('DIRECTION_PROVIDER_FAILED'), /模型服务/)
  assert.match(brandDirectionFailureMessage('DIRECTION_QUALITY_VALIDATION_FAILED'), /质量校验/)
  assert.match(brandDirectionFailureMessage('UNKNOWN'), /重新生成/)
})

test('confirmed brand direction takes precedence when restoring a batch', () => {
  const value = batch('ready')
  value.candidates = [
    {
      contract_version: 'creative-direction/v1', direction_id: 'candidate_1', concept: '候选', creative_rationale: '理由',
      message_plan: [], execution_outline: [], guardrail_trace: [], status: 'candidate',
    },
    {
      contract_version: 'creative-direction/v1', direction_id: 'confirmed_1', concept: '已确认', creative_rationale: '理由',
      message_plan: [], execution_outline: [], guardrail_trace: [], status: 'confirmed',
    },
  ]
  assert.deepEqual(availableBrandDirections(value).map(item => item.direction_id), ['confirmed_1'])
})

test('legacy channel-led direction batches require regeneration', () => {
  const value = batch('ready')
  value.prompt_version = 'creative-direction/strategy-handoff-v3'
  assert.equal(isChannelNeutralBrandDirectionBatch(value), false)
  value.prompt_version = 'creative-direction/strategy-handoff-v4'
  assert.equal(isChannelNeutralBrandDirectionBatch(value), true)
})

test('brand route without context lists active brand-video tasks for explicit selection', () => {
  const task = (id: string, createdAt: string, performanceMode = 'brand_video', status = 'draft') => ({
    id, organization_id: 'org_1', project_id: 'project_1', intake_id: `intake_${id}`,
    format: 'video' as const, channel: 'xiaohongshu', performance_mode: performanceMode, status,
    direction: { focus: id, audience: '', core_message: '', call_to_action: '', concept: '', tone: [], visual_keywords: [] },
    version: 1, created_at: createdAt, updated_at: createdAt,
  })
  const tasks = activeBrandVideoTasks([
    task('old', '2026-08-01T00:00:00Z'),
    task('new', '2026-08-05T00:00:00Z'),
    task('archived', '2026-08-06T00:00:00Z', 'brand_video', 'archived'),
    task('performance', '2026-08-07T00:00:00Z', 'commerce_preroll'),
  ])
  assert.deepEqual(tasks.map(item => item.id), ['new', 'old'])
  assert.equal(brandVideoTaskStatusLabel('in_progress'), '制作中')
  assert.equal(brandVideoTaskStatusLabel('unknown'), '待完善')
})
