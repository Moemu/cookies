import { ApiProblem, apiRequest } from '../../shared/api/client'
import type {
  BriefDraft,
  BriefVersion,
  ConversationBundle,
  GenerationMetadata,
  GenerationReadiness,
  Message,
  KnowledgeDocument,
  PackageVersion,
  ResearchRun,
  Review,
  ReviewComment,
  SkillRun,
  DraftRevision,
  StrategyDraft,
  Workspace,
  WorkspaceDetail,
} from './types'

const root = '/api/strategy/v1'

export function createMutationKey() {
  return `strategy-web-${crypto.randomUUID()}`
}

function mutationHeaders(key = createMutationKey()) {
  return { 'Idempotency-Key': key }
}

export function listWorkspaces(projectId: string, signal?: AbortSignal) {
  return apiRequest<{ items: Workspace[] }>(`${root}/projects/${encodeURIComponent(projectId)}/workspaces`, { signal })
}

export function createWorkspace(projectId: string, name: string) {
  return apiRequest<Workspace>(`${root}/workspaces`, {
    method: 'POST',
    headers: mutationHeaders(),
    body: JSON.stringify({ project_id: projectId, name }),
  })
}

export function getWorkspace(workspaceId: string, signal?: AbortSignal) {
  return apiRequest<WorkspaceDetail>(`${root}/workspaces/${encodeURIComponent(workspaceId)}`, { signal })
}

export function createConversation(projectId: string, workspaceId: string) {
  return apiRequest<ConversationBundle>(`${root}/conversations`, {
    method: 'POST',
    headers: mutationHeaders(),
    body: JSON.stringify({ project_id: projectId, workspace_id: workspaceId }),
  })
}

export function listMessages(conversationId: string, signal?: AbortSignal) {
  return apiRequest<{ items: Message[] }>(`${root}/conversations/${encodeURIComponent(conversationId)}/messages?limit=100`, { signal })
}

export function getConversationMemory(conversationId: string, signal?: AbortSignal) {
  return apiRequest<{ summary: string; open_questions: string[]; version: number }>(
    `${root}/conversations/${encodeURIComponent(conversationId)}/memory`,
    { signal },
  )
}

export function sendMessage(conversationId: string, content: string) {
  return apiRequest(`${root}/conversations/${encodeURIComponent(conversationId)}/messages`, {
    method: 'POST',
    headers: mutationHeaders(),
    body: JSON.stringify({ content }),
  })
}

export function getBriefDraft(taskId: string, signal?: AbortSignal) {
  return apiRequest<BriefDraft>(`${root}/tasks/${encodeURIComponent(taskId)}/brief-draft`, { signal })
}

export function patchBriefField(taskId: string, draft: BriefDraft, fieldPath: string, value: unknown) {
  return apiRequest<BriefDraft>(`${root}/tasks/${encodeURIComponent(taskId)}/brief-draft`, {
    method: 'PATCH',
    headers: { ...mutationHeaders(), 'If-Match': `"v${draft.version}"` },
    body: JSON.stringify({
      expected_version: draft.version,
      operations: [{ op: 'set', field_path: fieldPath, value }],
    }),
  })
}

export function confirmBrief(taskId: string, expectedVersion: number) {
  return apiRequest<BriefVersion>(`${root}/tasks/${encodeURIComponent(taskId)}/brief:confirm`, {
    method: 'POST',
    headers: mutationHeaders(),
    body: JSON.stringify({ expected_version: expectedVersion }),
  })
}

export function listBriefVersions(briefId: string, signal?: AbortSignal) {
  return apiRequest<{ items: BriefVersion[] }>(`${root}/briefs/${encodeURIComponent(briefId)}/versions`, { signal })
}

export function createStrategy(taskId: string, brief: BriefVersion) {
  return apiRequest<{ strategy_draft: StrategyDraft }>(`${root}/tasks/${encodeURIComponent(taskId)}/strategies`, {
    method: 'POST',
    headers: mutationHeaders(),
    body: JSON.stringify({ brief_id: brief.brief_id, brief_version: brief.version }),
  })
}

export function getGenerationReadiness(projectId: string, signal?: AbortSignal) {
  return apiRequest<GenerationReadiness>(`${root}/projects/${encodeURIComponent(projectId)}/generation-readiness`, { signal })
}

export function getStrategy(strategyId: string, signal?: AbortSignal) {
  return apiRequest<StrategyDraft>(`${root}/strategy-drafts/${encodeURIComponent(strategyId)}`, { signal })
}

export function listStrategyRevisions(strategyId: string, signal?: AbortSignal) {
  return apiRequest<{ items: DraftRevision[] }>(
    `${root}/strategy-drafts/${encodeURIComponent(strategyId)}/revisions`,
    { signal },
  )
}

export function listSkillRuns(agentTaskId: string, signal?: AbortSignal) {
  return apiRequest<{ items: SkillRun[] }>(
    `${root}/agent-tasks/${encodeURIComponent(agentTaskId)}/skill-runs`,
    { signal },
  )
}

export async function getGenerationMetadata(strategyId: string, signal?: AbortSignal) {
  try {
    return await apiRequest<GenerationMetadata>(`${root}/strategy-drafts/${encodeURIComponent(strategyId)}/generation-metadata`, { signal })
  } catch (error) {
    if (error instanceof ApiProblem && error.problem.error.code === 'NOT_FOUND') return null
    throw error
  }
}

export function reviseStrategy(draft: StrategyDraft, instruction: string) {
  return apiRequest(`${root}/strategy-drafts/${encodeURIComponent(draft.id)}:revise`, {
    method: 'POST',
    headers: mutationHeaders(),
    body: JSON.stringify({
      expected_version: draft.version,
      base_revision: draft.current_revision,
      instruction,
    }),
  })
}

export function patchStrategySection(draft: StrategyDraft, section: string, value: unknown) {
  return apiRequest<StrategyDraft>(`${root}/strategy-drafts/${encodeURIComponent(draft.id)}`, {
    method: 'PATCH',
    headers: mutationHeaders(),
    body: JSON.stringify({
      expected_version: draft.version,
      base_revision: draft.current_revision,
      section,
      value,
    }),
  })
}

export function submitStrategy(draft: StrategyDraft) {
  return apiRequest<Review>(`${root}/strategy-drafts/${encodeURIComponent(draft.id)}:submit`, {
    method: 'POST',
    headers: mutationHeaders(),
    body: JSON.stringify({ expected_version: draft.version, candidate_revision: draft.current_revision }),
  })
}

export function getReview(reviewId: string, signal?: AbortSignal) {
  return apiRequest<Review>(`${root}/strategy-reviews/${encodeURIComponent(reviewId)}`, { signal })
}

export function listReviewComments(reviewId: string, signal?: AbortSignal) {
  return apiRequest<{ items: ReviewComment[] }>(
    `${root}/strategy-reviews/${encodeURIComponent(reviewId)}/comments`,
    { signal },
  )
}

export function addReviewComment(reviewId: string, body: string) {
  return apiRequest<ReviewComment>(`${root}/strategy-reviews/${encodeURIComponent(reviewId)}/comments`, {
    method: 'POST',
    body: JSON.stringify({ body }),
  })
}

export function returnReview(reviewId: string, reason: string) {
  return apiRequest<Review>(`${root}/strategy-reviews/${encodeURIComponent(reviewId)}:return`, {
    method: 'POST',
    body: JSON.stringify({ reason }),
  })
}

export function listStrategyPackages(projectId: string, signal?: AbortSignal) {
  return apiRequest<{ items: PackageVersion[] }>(`${root}/projects/${encodeURIComponent(projectId)}/strategy-packages`, { signal })
}

export function approveStrategy(draft: StrategyDraft, review: Review, mutationKey?: string) {
  return apiRequest<PackageVersion>(`${root}/strategy-drafts/${encodeURIComponent(draft.id)}:approve`, {
    method: 'POST',
    headers: mutationHeaders(mutationKey),
    body: JSON.stringify({
      review_id: review.id,
      candidate_content_hash: review.candidate_content_hash,
      expected_version: draft.version,
    }),
  })
}

export function listKnowledgeDocuments(projectId: string, signal?: AbortSignal) {
  return apiRequest<{ items: KnowledgeDocument[] }>(
    `/platform/v1/projects/${encodeURIComponent(projectId)}/knowledge/documents`,
    { signal },
  )
}

export function uploadKnowledgeDocument(projectId: string, file: File) {
  const body = new FormData()
  body.append('file', file)
  return apiRequest<KnowledgeDocument>(
    `/platform/v1/projects/${encodeURIComponent(projectId)}/knowledge/documents`,
    { method: 'POST', body },
  )
}

export function runExternalResearch(
  projectId: string,
  request: {
    mode: 'web' | 'mcp'
    query: string
    document_ids: string[]
    disclosed_fields: string[]
    confirmed: boolean
  },
) {
  return apiRequest<ResearchRun>(
    `/platform/v1/projects/${encodeURIComponent(projectId)}/knowledge/research-runs`,
    { method: 'POST', body: JSON.stringify(request) },
  )
}

export function createStrategyFeedback(
  projectId: string,
  request: {
    target_type: 'strategy_revision' | 'strategy_package'
    target_id: string
    target_version: number
    rating: 'useful' | 'partly_useful' | 'not_useful'
    comment: string
  },
) {
  return apiRequest(`${root}/projects/${encodeURIComponent(projectId)}/feedback`, {
    method: 'POST',
    headers: mutationHeaders(),
    body: JSON.stringify(request),
  })
}
