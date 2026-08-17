import assert from 'node:assert/strict'
import test from 'node:test'
import { initialShortDramaPrerollState, shortDramaPrerollReducer } from '../src/features/short-drama-preroll-v2/reducer'
import type { FirstFrameCandidate } from '../src/features/short-drama-preroll-v2/types'

const readyOne: FirstFrameCandidate = {
  id: 'candidate-1', label: '人物情绪版', imageUrl: 'https://example.com/one.png', composition: 'A B C D', status: 'ready',
}
const readyTwo: FirstFrameCandidate = {
  id: 'candidate-2', label: '环境悬念版', imageUrl: 'https://example.com/two.png', composition: 'A B C D', status: 'ready',
}
const failed: FirstFrameCandidate = {
  id: 'candidate-3', label: '动作道具版', composition: 'A B C D', status: 'failed', recoverable: true,
  currentAttemptId: 'candidate-3-attempt-0', attemptCount: 1, errorMessage: '图片请求未通过',
}

test('reference board retry preserves successful slots and selection', () => {
  const withImages = shortDramaPrerollReducer(initialShortDramaPrerollState, { type: 'images-ready', images: [readyOne, readyTwo, failed] })
  const selected = { ...withImages, selectedImageId: readyOne.id }
  const retrying = shortDramaPrerollReducer(selected, { type: 'image-retry-started', id: failed.id })

  assert.equal(retrying.retryingImageId, failed.id)
  assert.equal(retrying.selectedImageId, readyOne.id)
  assert.equal(retrying.images[0].imageUrl, readyOne.imageUrl)
  assert.equal(retrying.images[1].imageUrl, readyTwo.imageUrl)
  assert.equal(retrying.images[2].status, 'queued')

  const recovered: FirstFrameCandidate = { ...failed, status: 'ready', imageUrl: 'https://example.com/three.png', recoverable: false, attemptCount: 2 }
  const synchronized = shortDramaPrerollReducer(retrying, { type: 'images-ready', images: [readyOne, readyTwo, recovered] })
  assert.equal(synchronized.retryingImageId, '')
  assert.equal(synchronized.selectedImageId, readyOne.id)
  assert.deepEqual(synchronized.images.map(item => item.status), ['ready', 'ready', 'ready'])
})
