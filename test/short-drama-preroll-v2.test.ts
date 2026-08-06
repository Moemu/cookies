import assert from 'node:assert/strict'
import test from 'node:test'
import { fixtureAnalysis, fixtureHooks, fixtureImages } from '../src/features/short-drama-preroll-v2/fixtures'
import { canOpenShortDramaStep, initialShortDramaPrerollState, shortDramaPrerollReducer } from '../src/features/short-drama-preroll-v2/reducer'

const source = { id: 'video-1', projectId: 'project-1', version: 1, kind: 'video' as const, sourceType: 'upload' as const, mimeType: 'video/mp4', sizeBytes: 1024, durationSeconds: 120, createdAt: '2026-08-05T00:00:00Z', contentUrl: '/assets/video-1' }

function readyState() {
  let state = shortDramaPrerollReducer(initialShortDramaPrerollState, { type: 'source-selected', source })
  state = shortDramaPrerollReducer(state, { type: 'analysis-ready', analysis: fixtureAnalysis })
  state = shortDramaPrerollReducer(state, { type: 'hooks-ready', hooks: fixtureHooks })
  state = shortDramaPrerollReducer(state, { type: 'hook-selected', id: fixtureHooks[0].id })
  state = shortDramaPrerollReducer(state, { type: 'images-ready', images: fixtureImages })
  return shortDramaPrerollReducer(state, { type: 'image-selected', id: fixtureImages[0].id })
}

test('short drama preroll unlocks workflow steps only after their required decision', () => {
  const empty = { ...initialShortDramaPrerollState, source }
  assert.equal(canOpenShortDramaStep(empty, 'understanding'), true)
  assert.equal(canOpenShortDramaStep(empty, 'direction'), false)
  const analyzed = shortDramaPrerollReducer(empty, { type: 'analysis-ready', analysis: fixtureAnalysis })
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
  assert.equal(changed.selectedImageId, fixtureImages[0].id)
  assert.equal(changed.output, null)
  assert.match(changed.videoPrompt, /10 秒/)
})

test('selecting a hook derives editable image and video prompts from the same story facts', () => {
  let state = shortDramaPrerollReducer(initialShortDramaPrerollState, { type: 'analysis-ready', analysis: fixtureAnalysis })
  state = shortDramaPrerollReducer(state, { type: 'hooks-ready', hooks: fixtureHooks })
  state = shortDramaPrerollReducer(state, { type: 'hook-selected', id: fixtureHooks[0].id })
  assert.match(state.imagePrompt, /旧录音机/)
  assert.match(state.videoPrompt, /第七个人的声音/)
  assert.equal(state.activeStep, 'first-frame')
})
