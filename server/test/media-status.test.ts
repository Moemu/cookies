import assert from 'node:assert/strict'
import test from 'node:test'
import { presentCreativeStatus, type MediaArtifact, type MediaGenerationJob } from '../../src/lib/media-status.js'

const generatedAt = '2026-07-23T08:30:00.000Z'
const formatDate = (value: string) => `时间 ${value}`

function mediaJob(kind: 'image' | 'video', status: MediaGenerationJob['status']): MediaGenerationJob {
  return {
    artifactKind: kind,
    status,
    model: `${kind}-model`,
    updatedAt: generatedAt,
  }
}

function readyAsset(kind: 'image' | 'video'): MediaArtifact {
  return {
    kind,
    status: 'ready',
    content: 'https://assets.test/result',
    sourceJobId: `${kind}-succeeded`,
    updatedAt: generatedAt,
  }
}

test('图片和视频成功任务在刷新后展示为已完成，并保留 AI、模型与生成时间', () => {
  for (const kind of ['image', 'video'] as const) {
    const presentation = presentCreativeStatus(readyAsset(kind), mediaJob(kind, 'succeeded'), formatDate)

    assert.equal(presentation.status, '已完成')
    assert.match(presentation.owner, /AI 生成/)
    assert.match(presentation.sourceVersion ?? '', new RegExp(`${kind}-model`))
    assert.match(presentation.sourceVersion ?? '', /时间 2026-07-23T08:30:00.000Z/)
  }
})

test('媒体任务状态保持排队、运行、失败和取消的可辨识语义', () => {
  const expected = {
    queued: '排队中',
    running: '制作中',
    failed: '生成失败',
    cancelled: '已取消',
  } as const

  for (const [status, displayStatus] of Object.entries(expected) as Array<[keyof typeof expected, typeof expected[keyof typeof expected]]>) {
    assert.equal(presentCreativeStatus(undefined, mediaJob('image', status), formatDate).status, displayStatus)
  }
})

test('没有任务时，已就绪媒体资产仍展示为已完成', () => {
  assert.equal(presentCreativeStatus(readyAsset('image'), undefined, formatDate).status, '已完成')
})
