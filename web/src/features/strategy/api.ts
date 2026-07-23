import { apiRequest } from '../../shared/api/client'
import type {
  BriefDraft,
  BriefVersion,
  ConversationBundle,
  Message,
  PackageVersion,
  Review,
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

export function getStrategy(strategyId: string, signal?: AbortSignal) {
  return apiRequest<StrategyDraft>(`${root}/strategy-drafts/${encodeURIComponent(strategyId)}`, { signal })
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
