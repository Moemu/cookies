import assert from 'node:assert/strict'
import test from 'node:test'
import { canOpenShortDramaStep, initialShortDramaPrerollState, shortDramaPrerollReducer } from '../src/features/short-drama-preroll-v2/reducer'
import type { FirstFrameCandidate, HookDirection, StoryAnalysis } from '../src/features/short-drama-preroll-v2/types'

const analysis: StoryAnalysis = {
  title: '武则天权力之路', episode: '第 1 集', synopsis: '武则天进入宫廷并面对权力抉择。',
  openingBeat: '宫廷局势突变', characters: ['武则天｜权力中心人物'], visualKeywords: ['宫廷', '权力'],
}
const hooks: HookDirection[] = [{
  id: 'direction-1', category: 'curiosity', eyebrow: '猎奇吸睛 01', title: '她为何突然回头',
  description: '用信息缺口建立悬念', hookCopy: '所有人都低估了她接下来的选择。', rationale: '来自剧情证据',
}]
const images: FirstFrameCandidate[] = [{ id: 'frame-1', label: '参考图 1', imageUrl: '/frame-1', composition: '人物近景' }]

const source = { id: 'video-1', projectId: 'project-1', version: 1, kind: 'video' as const, sourceType: 'upload' as const, mimeType: 'video/mp4', sizeBytes: 1024, durationSeconds: 120, createdAt: '2026-08-05T00:00:00Z', contentUrl: '/assets/video-1' }

function readyState() {
  let state = shortDramaPrerollReducer(initialShortDramaPrerollState, { type: 'source-selected', source })
  state = shortDramaPrerollReducer(state, { type: 'analysis-ready', analysis })
  state = shortDramaPrerollReducer(state, { type: 'hooks-ready', hooks })
  state = shortDramaPrerollReducer(state, { type: 'hook-selected', id: hooks[0].id, imagePrompt: '宫廷人物首帧', videoDescription: '宫廷权力变化', videoPrompt: '6 秒宫廷钩子', duration: 6 })
  state = shortDramaPrerollReducer(state, { type: 'images-ready', images })
  return shortDramaPrerollReducer(state, { type: 'image-selected', id: images[0].id })
}

test('short drama preroll unlocks workflow steps only after their required decision', () => {
  const empty = { ...initialShortDramaPrerollState, source }
  assert.equal(canOpenShortDramaStep(empty, 'understanding'), true)
  assert.equal(canOpenShortDramaStep(empty, 'direction'), false)
  const analyzed = shortDramaPrerollReducer(empty, { type: 'analysis-ready', analysis })
  assert.equal(canOpenShortDramaStep(analyzed, 'direction'), true)
  assert.equal(canOpenShortDramaStep(analyzed, 'first-frame'), false)
})

test('editing story summary invalidates hooks, prompts, frames and output', () => {
  const changed = shortDramaPrerollReducer(readyState(), { type: 'summary-changed', value: '新的梗概' })
  assert.equal(changed.summaryDraft, '新的梗概')
  assert.deepEqual(changed.hooks, [])
  assert.equal(changed.selectedHookId, '')
  assert.equal(changed.imagePrompt, '')
  assert.deepEqual(changed.images, [])
  assert.equal(changed.output, null)
})

test('changing duration keeps the selected first frame but invalidates video output', () => {
  const generated = shortDramaPrerollReducer(readyState(), { type: 'video-ready', output: { id: 'output-1', videoUrl: '/output.mp4', duration: 6, createdAt: '2026-08-05T00:00:00Z' } })
  const changed = shortDramaPrerollReducer(generated, { type: 'duration-changed', duration: 10 })
  assert.equal(changed.duration, 10)
  assert.equal(changed.selectedImageId, images[0].id)
  assert.equal(changed.output, null)
  assert.equal(changed.videoPrompt, '6 秒宫廷钩子')
})

test('selecting a hook stores editable prompts returned by the server', () => {
  let state = shortDramaPrerollReducer(initialShortDramaPrerollState, { type: 'analysis-ready', analysis })
  state = shortDramaPrerollReducer(state, { type: 'hooks-ready', hooks })
  state = shortDramaPrerollReducer(state, { type: 'hook-selected', id: hooks[0].id, imagePrompt: '真实首帧提示词', videoDescription: '真实视频描述', videoPrompt: '真实视频提示词', duration: 6 })
  assert.equal(state.imagePrompt, '真实首帧提示词')
  assert.equal(state.videoPrompt, '真实视频提示词')
  assert.equal(state.activeStep, 'first-frame')
})

test('trusted material edits remain unbound until the server confirms both assets', () => {
  let state = readyState()
  state = shortDramaPrerollReducer(state, { type: 'trusted-material-changed', role: 'first', value: 'asset-first' })
  state = shortDramaPrerollReducer(state, { type: 'trusted-material-changed', role: 'last', value: 'asset-last' })
  assert.equal(state.trustedMaterialsBound, false)
  state = shortDramaPrerollReducer(state, { type: 'trusted-materials-bound', firstFrameAssetId: 'asset-first', lastFrameAssetId: 'asset-last' })
  assert.equal(state.trustedMaterialsBound, true)
  assert.equal(state.trustedFirstFrameAssetId, 'asset-first')
  assert.equal(state.trustedLastFrameAssetId, 'asset-last')
})
