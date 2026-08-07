import assert from 'node:assert/strict'
import test from 'node:test'

import { briefProductCandidateOperations } from '../src/features/strategy/briefProductCandidate'
import type { BriefDocument } from '../src/features/strategy/types'

function operationValue(document: BriefDocument, candidateIndex: number, fieldPath: string) {
  const candidate = document.product?.candidates?.[candidateIndex]
  assert.ok(candidate)
  return briefProductCandidateOperations(document, candidate).find(operation => operation.fieldPath === fieldPath)?.value
}

test('projects every selected candidate field into the fixed Brief', () => {
  const document: BriefDocument = {
    contract_version: 'strategy-brief-version/v2',
    campaign: { objective: '双十一种草' }, audience: { primary: '' }, proposition: '', channels: ['xiaohongshu'],
    budget: { total: '' }, schedule: { window: '' }, constraints: [], measurement: { primary_kpi: '' },
    creative: { mandatory_elements: ['公共包装露出'], prohibited_claims: ['公共禁用词'] },
    product: { candidates: [{
      name: '黄金复原蜜', category: '精华', selling_points: ['修护', '修护'], evidence: ['原文证据'],
      mandatory_elements: ['一蜜多用'], prohibited_claims: ['不称为油'],
    }] },
  }

  const candidate = document.product?.candidates?.[0]
  assert.ok(candidate)
  assert.deepEqual(briefProductCandidateOperations(document, candidate), [
    { fieldPath: 'product.name', value: '黄金复原蜜' },
    { fieldPath: 'product.category', value: '精华' },
    { fieldPath: 'product.selling_points', value: ['修护'] },
    { fieldPath: 'product.evidence', value: ['原文证据'] },
    { fieldPath: 'creative.mandatory_elements', value: ['公共包装露出', '一蜜多用'] },
    { fieldPath: 'creative.prohibited_claims', value: ['公共禁用词', '不称为油'] },
  ])
})

test('switching candidates clears stale product facts and replaces product-specific guardrails', () => {
  const document: BriefDocument = {
    contract_version: 'strategy-brief-version/v2',
    campaign: { objective: '双十一种草' }, audience: { primary: '' }, proposition: '', channels: ['xiaohongshu'],
    budget: { total: '' }, schedule: { window: '' }, constraints: [], measurement: { primary_kpi: '' },
    creative: {
      mandatory_elements: ['公共包装露出', '一蜜多用'],
      prohibited_claims: ['公共禁用词', '不称为油'],
    },
    product: {
      name: '黄金复原蜜', category: '精华', selling_points: ['修护'], evidence: ['旧证据'],
      candidates: [
        { name: '黄金复原蜜', mandatory_elements: ['一蜜多用'], prohibited_claims: ['不称为油'] },
        { name: '金钻粉底液', mandatory_elements: ['展示妆效'], prohibited_claims: [] },
      ],
    },
  }

  assert.equal(operationValue(document, 1, 'product.category'), '')
  assert.deepEqual(operationValue(document, 1, 'product.selling_points'), [])
  assert.deepEqual(operationValue(document, 1, 'product.evidence'), [])
  assert.deepEqual(operationValue(document, 1, 'creative.mandatory_elements'), ['公共包装露出', '展示妆效'])
  assert.deepEqual(operationValue(document, 1, 'creative.prohibited_claims'), ['公共禁用词'])
})
