import assert from 'node:assert/strict'
import test from 'node:test'
import {
  buildConversationLens,
  buildConversationMessageCreate,
  compactDocumentTitle,
  conversationSearchRunsByMessage,
  conversationSourceDocuments,
  intakeMissingLabel,
} from '../src/features/strategy/strategyConversationModel.js'
import type { BriefDraft, KnowledgeDocument, MediaUnderstandingArtifact, ResearchArtifact, ResearchRun } from '../src/features/strategy/types.js'

function brief(values: Partial<{ product: string; objective: string; audience: string; proposition: string }>): BriefDraft {
  return {
    id: 'draft_1',
    brief_id: 'brief_1',
    status: 'open',
    version: 2,
    document: {
      contract_version: 'strategy-brief-version/v2',
      product: { name: values.product },
      campaign: { objective: values.objective ?? '' },
      audience: { primary: values.audience ?? '' },
      proposition: values.proposition ?? '',
      channels: [],
      budget: { total: '' },
      schedule: { window: '' },
      constraints: [],
      measurement: { primary_kpi: '' },
    },
    field_states: {},
    completeness: { ready: false, blockers: [], warnings: [] },
  }
}

test('conversation lens counts only business-critical facts as blockers', () => {
  const draft = brief({
    product: 'FlowKit',
    objective: '提高试用转化',
    audience: '效率工具用户',
  })
  draft.field_states['product.name'] = {
    field_path: 'product.name',
    confidence: 'high',
    confirmation: 'unconfirmed',
    source: { type: 'knowledge_chunk', id: 'chunk_1', locator: '正文:2-4' },
  }
  const lens = buildConversationLens(draft, [])
  assert.equal(lens.completedCore, 3)
  assert.equal(lens.totalCore, 3)
  assert.equal(lens.coreReady, true)
  assert.equal(lens.items.find(item => item.key === 'proposition')?.required, false)
  assert.equal(lens.items.find(item => item.key === 'product')?.sourceLabel, '资料片段 · 正文:2-4')
})

test('conversation attachments become immutable Message v2 document references', () => {
  const document = {
    id: 'document_1',
    content_sha256: 'a'.repeat(64),
  } as KnowledgeDocument
  assert.deepEqual(buildConversationMessageCreate('  请提炼资料  ', [document]), {
    contract_version: 'strategy-conversation-message-create/v2',
    content: [
      { type: 'text', text: '请提炼资料' },
      {
        type: 'document_ref',
        document_id: 'document_1',
        expected_content_sha256: 'a'.repeat(64),
      },
    ],
  })
})

test('media understanding artifacts become exact immutable asset references', () => {
  const artifact = {
    asset_kind: 'video',
    asset_ref: { project_id: 'project_1', asset_version: { asset_id: 'asset_9', version: 3 } },
  } as MediaUnderstandingArtifact
  const message = buildConversationMessageCreate('', [], [artifact])
  assert.deepEqual(message, {
    contract_version: 'strategy-conversation-message-create/v2',
    content: [{ type: 'asset_ref', asset_kind: 'video', asset_id: 'asset_9', asset_version: 3 }],
  })
})

test('web research becomes immutable evidence and a visible requested policy', () => {
  const artifact = { id: 'research_1', content_hash: 'c'.repeat(64) } as ResearchArtifact
  assert.deepEqual(buildConversationMessageCreate(
    '请查证市场规模',
    [],
    [],
    [artifact],
    { reasoning_mode: 'deep', web_search: 'allowed' },
  ), {
    contract_version: 'strategy-conversation-message-create/v2',
    content: [
      { type: 'text', text: '请查证市场规模' },
      { type: 'research_ref', research_artifact_id: 'research_1', expected_content_hash: 'c'.repeat(64) },
    ],
    requested_policy: { reasoning_mode: 'deep', web_search: 'allowed' },
  })
})

test('intake blockers are translated into actionable Chinese', () => {
  assert.equal(intakeMissingLabel('requirement.reference_video'), '上传一条参考视频')
  assert.equal(intakeMissingLabel('requirement.reference_video.ready'), '等待参考视频处理完成')
  assert.equal(intakeMissingLabel('requirement.unknown:确认是否可使用原音乐'), '确认是否可使用原音乐')
})

test('document title prefers parsed title and falls back to filename', () => {
  const document = {
    title: '',
    filename: 'campaign-brief.pdf',
  } as KnowledgeDocument
  assert.equal(compactDocumentTitle(document), 'campaign-brief.pdf')
})

test('conversation source list only contains documents referenced by the current brief', () => {
  const currentBrief = brief({})
  currentBrief.document.reference_ids = ['document_current', 'document_duplicate', 'research_artifact_1']
  const current = { id: 'document_current', content_sha256: 'same', status: 'ready' } as KnowledgeDocument
  const duplicate = { id: 'document_duplicate', content_sha256: 'same', status: 'parsing' } as KnowledgeDocument
  const previous = { id: 'document_previous', filename: 'strategy-conversation-brief.md' } as KnowledgeDocument

  assert.deepEqual(conversationSourceDocuments(currentBrief, [duplicate, previous, current]), [current])
  assert.deepEqual(conversationSourceDocuments(brief({}), [previous, current]), [])
})

test('conversation web searches stay linked to their originating message and exclude deep research', () => {
  const running = {
    id: 'run_chat',
    purpose: 'conversation_web_search',
    source_ref: { type: 'strategy_message', id: 'message_1' },
  } as ResearchRun
  const deep = { id: 'run_deep', purpose: 'deep_research' } as ResearchRun

  assert.deepEqual([...conversationSearchRunsByMessage([running, deep]).entries()], [['message_1', running]])
})
