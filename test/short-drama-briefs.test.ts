import assert from 'node:assert/strict'
import test from 'node:test'
import {
  findLocalShortDramaBrief,
  localShortDramaBriefs,
  shortDramaVideoLabel,
} from '../src/data/shortDramaBriefs.ts'

test('short drama local briefs provide real selectable configurations', () => {
  assert.ok(localShortDramaBriefs.length >= 2)
  const brief = findLocalShortDramaBrief('brief_local_suspense_truth_v1')
  assert.equal(brief.recommendedHookStrategy, 'suspense_reveal')
  assert.ok(brief.reviewedSellingPoints.length > 0)
  assert.ok(brief.callToAction.length > 0)
})

test('short drama video labels never expose asset ids or question-mark durations', () => {
  assert.equal(shortDramaVideoLabel({}, 0), '项目正片 01 · 时长待识别')
  assert.equal(
    shortDramaVideoLabel({ sourceType: 'provider_generated', durationSeconds: 12.4, width: 720, height: 1280 }, 1),
    'AI 生成正片 02 · 12 秒 · 720×1280',
  )
})
