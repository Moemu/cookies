import assert from 'node:assert/strict'
import test from 'node:test'
import { deriveBrandFilmStages, resolveBrandFilmStage } from '../src/features/brand-film/stage.ts'

test('brand film stages progressively unlock from persisted workflow facts', () => {
  const initial = deriveBrandFilmStages({ briefConfirmed: false, conceptSelected: false, planConfirmed: false, visualPreviewReady: false, audioPreviewReady: false })
  assert.deepEqual(initial.map(stage => stage.accessible), [true, false, false, false, false])
  assert.equal(resolveBrandFilmStage(null, initial), 'brief')

  const planned = deriveBrandFilmStages({ briefConfirmed: true, conceptSelected: true, planConfirmed: true, visualPreviewReady: false, audioPreviewReady: false })
  assert.deepEqual(planned.map(stage => stage.accessible), [true, true, true, true, false])
  assert.deepEqual(planned.map(stage => stage.complete), [true, true, true, false, false])
  assert.equal(resolveBrandFilmStage(null, planned), 'generation')
})

test('brand film stage routing preserves accessible requests and repairs locked requests', () => {
  const stages = deriveBrandFilmStages({ briefConfirmed: true, conceptSelected: true, planConfirmed: false, visualPreviewReady: false, audioPreviewReady: false })
  assert.equal(resolveBrandFilmStage('brief', stages), 'brief')
  assert.equal(resolveBrandFilmStage('storyboard', stages), 'storyboard')
  assert.equal(resolveBrandFilmStage('audio', stages), 'storyboard')
  assert.equal(resolveBrandFilmStage('unknown', stages), 'storyboard')
})

test('audio is the latest stage once a visual preview exists', () => {
  const stages = deriveBrandFilmStages({ briefConfirmed: true, conceptSelected: true, planConfirmed: true, visualPreviewReady: true, audioPreviewReady: true })
  assert.equal(resolveBrandFilmStage(null, stages), 'audio')
  assert.equal(stages.at(-1)?.complete, true)
})
