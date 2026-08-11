import assert from 'node:assert/strict'
import test from 'node:test'

import { getFullStrategyDraftReadiness, getFullStrategyReadiness } from '../src/features/strategy/strategyBriefReadiness'
import type { BriefDocument, BriefDraft, BriefVersion } from '../src/features/strategy/types'

function briefDocument(overrides: Partial<BriefDocument> = {}): BriefDocument {
  return {
    contract_version: 'strategy-brief-version/v2',
    campaign: { objective: '建立第三代黄金复原蜜的新品认知' },
    audience: { primary: '关注高端抗老修护的都市女性' },
    proposition: '一滴焕活，强韧年轻肌底',
    channels: ['xiaohongshu', 'douyin'],
    budget: { total: '' },
    schedule: { window: '' },
    constraints: [],
    measurement: { primary_kpi: '' },
    reference_ids: [],
    ...overrides,
  }
}

function briefVersion(snapshot: BriefDocument): BriefVersion {
  return { brief_id: 'brief_guerlain', version: 1, content_hash: 'sha256:test', snapshot }
}

test('blocks full strategy generation and names a missing proposition', () => {
  const readiness = getFullStrategyReadiness(briefVersion(briefDocument({ proposition: '' })))

  assert.equal(readiness.ready, false)
  assert.deepEqual(readiness.blockers, [{ field: 'proposition', reason: '完整策略需要该信息' }])
})

test('uses server readiness so unconfirmed fields are not lost in the client fallback', () => {
  const version = briefVersion(briefDocument())
  version.full_strategy_readiness = {
    ready: false,
    blockers: [{ field: 'channels', reason: '完整策略需要用户确认' }],
    warnings: [],
  }

  assert.deepEqual(getFullStrategyReadiness(version), version.full_strategy_readiness)
})

test('allows generation when all required full-strategy inputs are present', () => {
  assert.equal(getFullStrategyReadiness(briefVersion(briefDocument())).ready, true)
})

test('supplement revision cannot freeze until the new strategy input is confirmed', () => {
  const document = briefDocument({ proposition: '' })
  const draft: BriefDraft = {
    id: 'draft_2', brief_id: 'brief_guerlain', status: 'open', version: 1, base_brief_version: 1,
    document,
    field_states: {
      'campaign.objective': { field_path: 'campaign.objective', source: { type: 'user', id: 'user_local' }, confidence: 'high', confirmation: 'confirmed' },
      'audience.primary': { field_path: 'audience.primary', source: { type: 'user', id: 'user_local' }, confidence: 'high', confirmation: 'confirmed' },
      channels: { field_path: 'channels', source: { type: 'user', id: 'user_local' }, confidence: 'high', confirmation: 'confirmed' },
    },
    completeness: { ready: true, blockers: [], warnings: [] },
  }

  assert.deepEqual(getFullStrategyDraftReadiness(draft).blockers, [
    { field: 'proposition', reason: '完整策略需要该信息' },
  ])
})
