import assert from 'node:assert/strict'
import test from 'node:test'
import {
  briefCompleteness,
  createInitialGamePrerollState,
  generationBlockers,
  stepAccessible,
} from '../src/features/game-preroll/model.ts'

test('game preroll v2 starts as a five-step upload-first flow', () => {
  const state = createInitialGamePrerollState('project_demo')
  assert.equal(state.step, 'upload')
  assert.equal(state.config.durationSeconds, 8)
  assert.equal(state.config.channel, 'douyin')
  assert.equal(state.config.aspectRatio, '9:16')
  assert.equal(stepAccessible(state, 'analysis'), false)
})

test('generation remains blocked until brief, candidate and evidence are ready', () => {
  const state = createInitialGamePrerollState('project_demo')
  assert.deepEqual(generationBlockers(state), [
    '未上传原视频', '素材尚未完成拆解', '广告简报尚未确认',
    '尚未人工选择钩子方案', '证据帧不足 3 张',
  ])
})

test('brief completeness only counts required editable fields', () => {
  assert.equal(briefCompleteness([
    { id: 'a', label: '广告目标', value: '下载', provenance: 'manual', required: true, evidenceRefs: [] },
    { id: 'b', label: '目标受众', value: '', provenance: 'ai_inference', required: true, evidenceRefs: [] },
    { id: 'c', label: '补充信息', value: '', provenance: 'manual', required: false, evidenceRefs: [] },
  ]), 50)
})
