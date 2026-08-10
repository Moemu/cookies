import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import test from 'node:test'

test('creative video shelf exposes every current-project video and a truthful empty state', async () => {
  const pageSource = await readFile(
    new URL('../src/components/SpecializedPages.tsx', import.meta.url),
    'utf8',
  )
  const clientSource = await readFile(
    new URL('../src/data/platformClient.ts', import.meta.url),
    'utf8',
  )

  assert.match(pageSource, /CURRENT PROJECT VIDEO ASSETS/)
  assert.match(pageSource, /当前项目生成视频/)
  assert.match(pageSource, /visibleVideos\.map/)
  assert.doesNotMatch(pageSource, /videos\.slice\(0,\s*6\)/)
  assert.match(pageSource, /完成任一视频生成任务后会自动出现在这里/)
  assert.match(clientSource, /sourceType:\s*normalizedSourceType/)
})
