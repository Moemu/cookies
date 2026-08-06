import assert from 'node:assert/strict'
import test from 'node:test'
import { mergeConversationMedia } from '../src/features/strategy/useStrategyWorkspace.ts'
import type { MediaUnderstandingArtifact } from '../src/features/strategy/types.ts'

function artifact(id: string, projectId: string, status: MediaUnderstandingArtifact['status'], createdAt: string) {
  return { id, project_id: projectId, status, created_at: createdAt } as MediaUnderstandingArtifact
}

test('conversation reload preserves newly uploaded media that has not been sent yet', () => {
  const pendingUpload = artifact('media_pending', 'project_1', 'running', '2026-08-04T17:00:01Z')
  const restoredReference = artifact('media_sent', 'project_1', 'partial', '2026-08-04T17:00:00Z')
  const otherProject = artifact('media_other', 'project_2', 'ready', '2026-08-04T17:00:02Z')

  assert.deepEqual(
    mergeConversationMedia([pendingUpload, otherProject], [restoredReference], 'project_1').map(value => value.id),
    ['media_pending', 'media_sent'],
  )
})
