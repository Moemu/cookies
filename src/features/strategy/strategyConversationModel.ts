import type {
  BriefDraft,
  FieldState,
  KnowledgeDocument,
  MediaUnderstandingArtifact,
  MessageCreateV2,
  MessageRequestedPolicy,
  ResearchArtifact,
  ResearchRun,
} from './types'

export type RequirementConfirmationOperation = { fieldPath: string; value: string }

export function requirementConfirmationOperations(brief: BriefDraft): RequirementConfirmationOperation[] {
  const fields = [
    ['product.name', brief.document.product?.name],
    ['campaign.objective', brief.document.campaign.objective],
    ['audience.primary', brief.document.audience.primary],
    ['proposition', brief.document.proposition],
  ] as const
  return fields.flatMap(([fieldPath, value]) => {
    if (!value?.trim() || brief.field_states[fieldPath]?.confirmation === 'confirmed') return []
    return [{ fieldPath, value }]
  })
}

export type ConversationLensItem = {
  key: 'product' | 'objective' | 'audience' | 'proposition'
  label: string
  value: string
  required: boolean
  confirmed: boolean
  sourceLabel?: string
}

export type ConversationLens = {
  items: ConversationLensItem[]
  completedCore: number
  totalCore: number
  coreReady: boolean
  sourceCount: number
  readySourceCount: number
}

export function buildConversationLens(
  brief: BriefDraft | null,
  documents: KnowledgeDocument[],
): ConversationLens {
  const item = (
    key: ConversationLensItem['key'],
    label: string,
    value: string | undefined,
    fieldPath: string,
    required: boolean,
  ): ConversationLensItem => ({
    key,
    label,
    value: value?.trim() ?? '',
    required,
    confirmed: brief?.field_states[fieldPath]?.confirmation === 'confirmed',
    sourceLabel: conversationSourceLabel(brief?.field_states[fieldPath]),
  })
  const items = [
    item('product', '产品 / 主题', brief?.document.product?.name, 'product.name', true),
    item('objective', '这次要解决什么', brief?.document.campaign.objective, 'campaign.objective', true),
    item('audience', '最想影响谁', brief?.document.audience.primary, 'audience.primary', true),
    item('proposition', '希望记住什么', brief?.document.proposition, 'proposition', false),
  ]
  const core = items.filter(value => value.required)
  const completedCore = core.filter(value => value.value).length
  return {
    items,
    completedCore,
    totalCore: core.length,
    coreReady: completedCore === core.length,
    sourceCount: documents.length,
    readySourceCount: documents.filter(value => value.status === 'ready').length,
  }
}

export function conversationSourceDocuments(
  brief: BriefDraft | null,
  documents: KnowledgeDocument[],
): KnowledgeDocument[] {
  const referencedDocumentIds = new Set(brief?.document.reference_ids ?? [])
  if (!referencedDocumentIds.size) return []
  const uniqueDocuments = new Map<string, KnowledgeDocument>()
  for (const document of documents) {
    if (!referencedDocumentIds.has(document.id)) continue
    const identity = document.content_sha256 || document.id
    const existing = uniqueDocuments.get(identity)
    if (!existing || (existing.status !== 'ready' && document.status === 'ready')) {
      uniqueDocuments.set(identity, document)
    }
  }
  return [...uniqueDocuments.values()]
}

export function conversationSearchRunsByMessage(runs: ResearchRun[]): Map<string, ResearchRun> {
  const result = new Map<string, ResearchRun>()
  for (const run of runs) {
    if (run.purpose !== 'conversation_web_search' || run.source_ref?.type !== 'strategy_message') continue
    if (!result.has(run.source_ref.id)) result.set(run.source_ref.id, run)
  }
  return result
}

export function conversationSourceLabel(state: FieldState | undefined): string | undefined {
  if (!state) return undefined
  if (state.source.type === 'knowledge_chunk') {
    return state.source.locator ? `资料片段 · ${state.source.locator}` : '资料片段'
  }
  if (state.source.type === 'media_artifact') {
    return state.source.locator ? `媒体证据 · ${state.source.locator}` : '媒体证据'
  }
  if (state.source.type === 'conversation_message') return '来自对话'
  if (state.source.type === 'user_edit') return '人工补充'
  return '有来源记录'
}

export function buildConversationMessageCreate(
  content: string,
  documents: KnowledgeDocument[],
  media: MediaUnderstandingArtifact[] = [],
  researchArtifacts: ResearchArtifact[] = [],
  requestedPolicy?: MessageRequestedPolicy,
): string | MessageCreateV2 {
  const normalized = content.trim()
  if (!documents.length && !media.length && !researchArtifacts.length && !requestedPolicy) return normalized
  return {
    contract_version: 'strategy-conversation-message-create/v2',
    content: [
      ...(normalized ? [{ type: 'text' as const, text: normalized }] : []),
      ...documents.map(document => ({
        type: 'document_ref' as const,
        document_id: document.id,
        expected_content_sha256: document.content_sha256,
      })),
      ...media.map(artifact => ({
        type: 'asset_ref' as const,
        asset_kind: artifact.asset_kind,
        asset_id: artifact.asset_ref.asset_version.asset_id,
        asset_version: artifact.asset_ref.asset_version.version,
      })),
      ...researchArtifacts.map(artifact => ({
        type: 'research_ref' as const,
        research_artifact_id: artifact.id,
        expected_content_hash: artifact.content_hash,
      })),
    ],
    ...(requestedPolicy ? { requested_policy: requestedPolicy } : {}),
  }
}

export function intakeMissingLabel(value: string): string {
  if (value === 'requirement.core.objective') return '补充本次创作目标'
  if (value === 'requirement.core.product_or_subject') return '补充产品或创作主题'
  if (value === 'requirement.core.audience') return '补充核心受众'
  if (value === 'requirement.reference_video') return '上传一条参考视频'
  if (value === 'requirement.reference_video.ready') return '等待参考视频处理完成'
  if (value.startsWith('requirement.unknown:')) return value.slice('requirement.unknown:'.length)
  return value
}

export function compactDocumentTitle(document: KnowledgeDocument): string {
  const title = document.title?.trim() || document.filename.trim()
  return title || '未命名资料'
}
