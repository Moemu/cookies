import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import {
  clearWorkspaceSessionValue,
  readWorkspaceSessionValue,
  stageScrollSessionKey,
  workspaceSessionKey,
  writeWorkspaceSessionValue,
} from '../src/features/strategy/workspace/workspaceSessionState.js'

class MemoryStorage {
  readonly values = new Map<string, string>()

  getItem(key: string) {
    return this.values.get(key) ?? null
  }

  setItem(key: string, value: string) {
    this.values.set(key, value)
  }

  removeItem(key: string) {
    this.values.delete(key)
  }
}

test('workspace session keys isolate project, workspace, resource and stage', () => {
  const briefKey = workspaceSessionKey('project / 华东', 'workspace/1', 'brief-field', 'brief:1', 'campaign.objective')
  const otherWorkspaceKey = workspaceSessionKey('project / 华东', 'workspace/2', 'brief-field', 'brief:1', 'campaign.objective')
  const otherUserKey = workspaceSessionKey('org_1:user_2:project / 华东', 'workspace/1', 'brief-field', 'brief:1', 'campaign.objective')
  assert.notEqual(briefKey, otherWorkspaceKey)
  assert.notEqual(briefKey, otherUserKey)
  assert.match(briefKey, /^cookies\.strategy\.workspace-session\.v1:/)
  assert.equal(briefKey.includes('project / 华东'), false)
  assert.notEqual(stageScrollSessionKey('project_1', 'workspace_1', 'brief'), stageScrollSessionKey('project_1', 'workspace_1', 'strategy'))
})

test('workspace session values are schema-versioned, clearable and fail closed', () => {
  const storage = new MemoryStorage()
  const key = workspaceSessionKey('project_1', 'workspace_1', 'intake-composer')
  const value = { content: '尚未发送的需求', attachedDocumentIds: ['document_1'] }

  writeWorkspaceSessionValue(key, value, storage)
  assert.deepEqual(readWorkspaceSessionValue(key, storage), value)
  assert.deepEqual(JSON.parse(storage.getItem(key) ?? '{}'), { schema_version: 1, value })

  storage.setItem(key, JSON.stringify({ schema_version: 2, value: { content: '不兼容数据' } }))
  assert.equal(readWorkspaceSessionValue(key, storage), null)
  storage.setItem(key, '{broken json')
  assert.equal(readWorkspaceSessionValue(key, storage), null)

  clearWorkspaceSessionValue(key, storage)
  assert.equal(storage.getItem(key), null)
  assert.equal(readWorkspaceSessionValue(key, null), null)
})

test('workspace drafts retain an in-memory recovery copy when browser session storage is unavailable', () => {
  const key = workspaceSessionKey('org_1:user_1:project_1', 'workspace_1', 'research-composer')
  writeWorkspaceSessionValue(key, { query: '待研究的问题' })
  assert.deepEqual(readWorkspaceSessionValue(key), { query: '待研究的问题' })
  clearWorkspaceSessionValue(key)
  assert.equal(readWorkspaceSessionValue(key), null)
})

test('Strategy stages restore scroll and keep unsaved inputs across all five stages', () => {
  const workspace = readFileSync(new URL('../src/features/strategy/KanonStrategyWorkspace.tsx', import.meta.url), 'utf8')
  const conversation = readFileSync(new URL('../src/features/strategy/StrategyConversationPane.tsx', import.meta.url), 'utf8')
  const handoff = readFileSync(new URL('../src/features/strategy/CreativeTaskPlanner.tsx', import.meta.url), 'utf8')

  assert.match(workspace, /workspaceSessionOwner = `\$\{session\.organization\?\.id[\s\S]{0,180}\$\{currentProject\.id\}`/)
  assert.match(workspace, /<KanonStrategyWorkspaceSession[\s\S]{0,100}key=\{workspaceSessionOwner\}/)
  assert.match(workspace, /assistant-open\.v1:\$\{workspaceSessionOwner\}/)
  assert.match(workspace, /assistant-expanded\.v1:\$\{workspaceSessionOwner\}/)
  assert.match(workspace, /assistant-width\.v1:\$\{workspaceSessionOwner\}/)
  assert.match(workspace, /assistant-seen\.v1:\$\{workspaceSessionOwner\}:\$\{assistantWorkspaceId \|\| 'workspace-pending'\}/)
  assert.doesNotMatch(workspace, /assistant-(?:open|expanded|width)\.v1:\$\{currentProject\.id\}/)
  assert.match(workspace, /stageScrollSessionKey\(workspaceSessionOwner, workspaceSessionId, activeStage\)/)
  assert.doesNotMatch(workspace, /scrollTo\(\{ top: 0 \}\)/)
  assert.match(workspace, /'brief-field', brief\.id, field\.path/)
  assert.match(workspace, /'strategy-section-draft', revisionIdentity, section/)
  assert.match(workspace, /'research-composer'/)
  assert.match(workspace, /readResearchComposerDraft\(composerStorageKey\)/)
  assert.match(workspace, /'review-draft'/)
  assert.match(workspace, /readReviewSessionDraft\(draftStorageKey\)/)
  assert.match(workspace, /Boolean\(comment\.trim\(\)\)[\s\S]{0,100}Boolean\(reason\.trim\(\)\)/)
  assert.match(workspace, /dirtySections\.length[\s\S]{0,500}disabled=\{!canDecide \|\| Boolean\(busy\) \|\| Boolean\(dirtySections\.length\)\}/)
  assert.match(conversation, /readConversationComposerDraft\(draftStorageKey\)/)
  assert.match(conversation, /writeWorkspaceSessionValue\(draftStorageKey, value\)/)
  assert.match(handoff, /answersWithSessionDraft\(nextPlan, draftStorageKey\)/)
  assert.match(handoff, /writeWorkspaceSessionValue<HandoffAnswersSessionDraft>/)
  assert.match(handoff, /disabled=\{!activePlan\.completeness\.ready \|\| hasUnsavedAnswers/)
})
