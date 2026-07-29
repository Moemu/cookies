import { BackendApiError, apiRequest } from '../../backend/platform'
import type {
  AgentTask,
  BriefDraft,
  BriefVersion,
  ConversationBundle,
  ConversationMemory,
  DraftRevision,
  GenerationMetadata,
  GenerationReadiness,
  KnowledgeDocument,
  Message,
  PackageVersion,
  ResearchRun,
  Review,
  ReviewComment,
  ReviewPolicy,
  SkillRun,
  SkillDescriptor,
  StrategyDraft,
  Workspace,
  WorkspaceDetail,
} from './types'

const root = '/api/strategy/v1'

export function createMutationKey(prefix = 'strategy-kanon') {
  const id = globalThis.crypto?.randomUUID?.() ?? `${Date.now()}-${Math.random().toString(16).slice(2)}`
  return `${prefix}-${id}`
}

function mutationHeaders(key?: string) {
  return { 'Idempotency-Key': key ?? createMutationKey() }
}

export const strategyApi = {
  listSkills: (signal?: AbortSignal) =>
    apiRequest<{ items: SkillDescriptor[] }>(`${root}/skills`, { signal }),

  listWorkspaces: (projectId: string, signal?: AbortSignal) =>
    apiRequest<{ items: Workspace[] }>(`${root}/projects/${encodeURIComponent(projectId)}/workspaces`, { signal }),

  createWorkspace: (projectId: string, name: string, mutationKey?: string) =>
    apiRequest<Workspace>(`${root}/workspaces`, {
      method: 'POST',
      headers: mutationHeaders(mutationKey),
      body: JSON.stringify({ project_id: projectId, name }),
    }),

  getWorkspace: (workspaceId: string, signal?: AbortSignal) =>
    apiRequest<WorkspaceDetail>(`${root}/workspaces/${encodeURIComponent(workspaceId)}`, { signal }),

  createConversation: (projectId: string, workspaceId: string, mutationKey?: string) =>
    apiRequest<ConversationBundle>(`${root}/conversations`, {
      method: 'POST',
      headers: mutationHeaders(mutationKey),
      body: JSON.stringify({ project_id: projectId, workspace_id: workspaceId }),
    }),

  listMessages: (conversationId: string, signal?: AbortSignal) =>
    apiRequest<{ items: Message[] }>(`${root}/conversations/${encodeURIComponent(conversationId)}/messages?limit=100`, { signal }),

  getConversationMemory: (conversationId: string, signal?: AbortSignal) =>
    apiRequest<ConversationMemory>(`${root}/conversations/${encodeURIComponent(conversationId)}/memory`, { signal }),

  sendMessage: (conversationId: string, content: string, mutationKey?: string) =>
    apiRequest<{ message: Message; agent_task: AgentTask }>(
      `${root}/conversations/${encodeURIComponent(conversationId)}/messages`,
      {
        method: 'POST',
        headers: mutationHeaders(mutationKey),
        body: JSON.stringify({ content }),
      },
    ),

  getAgentTask: (agentTaskId: string, signal?: AbortSignal) =>
    apiRequest<AgentTask>(`${root}/agent-tasks/${encodeURIComponent(agentTaskId)}`, { signal }),

  listSkillRuns: (agentTaskId: string, signal?: AbortSignal) =>
    apiRequest<{ items: SkillRun[] }>(`${root}/agent-tasks/${encodeURIComponent(agentTaskId)}/skill-runs`, { signal }),

  getBriefDraft: (taskId: string, signal?: AbortSignal) =>
    apiRequest<BriefDraft>(`${root}/tasks/${encodeURIComponent(taskId)}/brief-draft`, { signal }),

  patchBriefField: (taskId: string, draft: BriefDraft, fieldPath: string, value: unknown, mutationKey?: string) =>
    apiRequest<BriefDraft>(`${root}/tasks/${encodeURIComponent(taskId)}/brief-draft`, {
      method: 'PATCH',
      headers: { ...mutationHeaders(mutationKey), 'If-Match': `"v${draft.version}"` },
      body: JSON.stringify({
        expected_version: draft.version,
        operations: [{ op: 'set', field_path: fieldPath, value }],
      }),
    }),

  confirmBrief: (taskId: string, expectedVersion: number, mutationKey?: string) =>
    apiRequest<BriefVersion>(`${root}/tasks/${encodeURIComponent(taskId)}/brief:confirm`, {
      method: 'POST',
      headers: mutationHeaders(mutationKey),
      body: JSON.stringify({ expected_version: expectedVersion }),
    }),

  listBriefVersions: (briefId: string, signal?: AbortSignal) =>
    apiRequest<{ items: BriefVersion[] }>(`${root}/briefs/${encodeURIComponent(briefId)}/versions`, { signal }),

  createStrategy: (taskId: string, brief: BriefVersion, mutationKey?: string) =>
    apiRequest<{ strategy_draft: StrategyDraft; agent_task: AgentTask }>(
      `${root}/tasks/${encodeURIComponent(taskId)}/strategies`,
      {
        method: 'POST',
        headers: mutationHeaders(mutationKey),
        body: JSON.stringify({ brief_id: brief.brief_id, brief_version: brief.version }),
      },
    ),

  getGenerationReadiness: (projectId: string, signal?: AbortSignal) =>
    apiRequest<GenerationReadiness>(`${root}/projects/${encodeURIComponent(projectId)}/generation-readiness`, { signal }),

  getStrategy: (strategyId: string, signal?: AbortSignal) =>
    apiRequest<StrategyDraft>(`${root}/strategy-drafts/${encodeURIComponent(strategyId)}`, { signal }),

  listStrategyRevisions: (strategyId: string, signal?: AbortSignal) =>
    apiRequest<{ items: DraftRevision[] }>(`${root}/strategy-drafts/${encodeURIComponent(strategyId)}/revisions`, { signal }),

  getGenerationMetadata: async (strategyId: string, signal?: AbortSignal) => {
    try {
      return await apiRequest<GenerationMetadata>(
        `${root}/strategy-drafts/${encodeURIComponent(strategyId)}/generation-metadata`,
        { signal },
      )
    } catch (error) {
      if (error instanceof BackendApiError && error.status === 404) return null
      throw error
    }
  },

  patchStrategySection: (draft: StrategyDraft, section: string, value: unknown, mutationKey?: string) =>
    apiRequest<StrategyDraft>(`${root}/strategy-drafts/${encodeURIComponent(draft.id)}`, {
      method: 'PATCH',
      headers: mutationHeaders(mutationKey),
      body: JSON.stringify({
        expected_version: draft.version,
        base_revision: draft.current_revision,
        section,
        value,
      }),
    }),

  reviseStrategy: (draft: StrategyDraft, instruction: string, mutationKey?: string) =>
    apiRequest<AgentTask>(`${root}/strategy-drafts/${encodeURIComponent(draft.id)}:revise`, {
      method: 'POST',
      headers: mutationHeaders(mutationKey),
      body: JSON.stringify({
        expected_version: draft.version,
        base_revision: draft.current_revision,
        instruction,
      }),
    }),

  submitStrategy: (draft: StrategyDraft, mutationKey?: string) =>
    apiRequest<Review>(`${root}/strategy-drafts/${encodeURIComponent(draft.id)}:submit`, {
      method: 'POST',
      headers: mutationHeaders(mutationKey),
      body: JSON.stringify({
        expected_version: draft.version,
        candidate_revision: draft.current_revision,
      }),
    }),

  getReview: (reviewId: string, signal?: AbortSignal) =>
    apiRequest<Review>(`${root}/strategy-reviews/${encodeURIComponent(reviewId)}`, { signal }),

  listReviews: (
    projectId: string,
    filter: 'all' | 'assigned_to_me' | 'requested_by_me' = 'all',
    status = '',
    signal?: AbortSignal,
  ) => {
    const query = new URLSearchParams({ filter })
    if (status) query.set('status', status)
    return apiRequest<{ items: Review[] }>(
      `${root}/projects/${encodeURIComponent(projectId)}/reviews?${query.toString()}`,
      { signal },
    )
  },

  getReviewPolicy: (projectId: string, signal?: AbortSignal) =>
    apiRequest<ReviewPolicy>(
      `${root}/projects/${encodeURIComponent(projectId)}/review-policy`,
      { signal },
    ),

  updateReviewPolicy: (
    projectId: string,
    policy: Pick<ReviewPolicy, 'mode' | 'approver_user_ids' | 'allow_self_approval' | 'version'>,
  ) =>
    apiRequest<ReviewPolicy>(`${root}/projects/${encodeURIComponent(projectId)}/review-policy`, {
      method: 'PUT',
      body: JSON.stringify({
        mode: policy.mode,
        approver_user_ids: policy.approver_user_ids,
        allow_self_approval: policy.allow_self_approval,
        expected_version: policy.version,
      }),
    }),

  listReviewComments: (reviewId: string, signal?: AbortSignal) =>
    apiRequest<{ items: ReviewComment[] }>(
      `${root}/strategy-reviews/${encodeURIComponent(reviewId)}/comments`,
      { signal },
    ),

  addReviewComment: (reviewId: string, body: string) =>
    apiRequest<ReviewComment>(`${root}/strategy-reviews/${encodeURIComponent(reviewId)}/comments`, {
      method: 'POST',
      body: JSON.stringify({ body }),
    }),

  returnReview: (reviewId: string, reason: string) =>
    apiRequest<Review>(`${root}/strategy-reviews/${encodeURIComponent(reviewId)}:return`, {
      method: 'POST',
      body: JSON.stringify({ reason }),
    }),

  approveStrategy: (draft: StrategyDraft, review: Review, mutationKey?: string) =>
    apiRequest<PackageVersion>(`${root}/strategy-drafts/${encodeURIComponent(draft.id)}:approve`, {
      method: 'POST',
      headers: mutationHeaders(mutationKey),
      body: JSON.stringify({
        review_id: review.id,
        candidate_content_hash: review.candidate_content_hash,
        expected_version: draft.version,
      }),
    }),

  listStrategyPackages: (projectId: string, signal?: AbortSignal) =>
    apiRequest<{ items: PackageVersion[] }>(
      `${root}/projects/${encodeURIComponent(projectId)}/strategy-packages`,
      { signal },
    ),

  listKnowledgeDocuments: (projectId: string, signal?: AbortSignal) =>
    apiRequest<{ items: KnowledgeDocument[] }>(
      `/platform/v1/projects/${encodeURIComponent(projectId)}/knowledge/documents`,
      { signal },
    ),

  uploadKnowledgeDocument: (projectId: string, file: File) => {
    const body = new FormData()
    body.append('file', file)
    return apiRequest<KnowledgeDocument>(
      `/platform/v1/projects/${encodeURIComponent(projectId)}/knowledge/documents`,
      { method: 'POST', body },
    )
  },

  runExternalResearch: (
    projectId: string,
    request: {
      mode: 'web' | 'mcp'
      query: string
      document_ids: string[]
      disclosed_fields: string[]
      confirmed: boolean
    },
  ) =>
    apiRequest<ResearchRun>(
      `/platform/v1/projects/${encodeURIComponent(projectId)}/knowledge/research-runs`,
      { method: 'POST', body: JSON.stringify(request) },
    ),

  listResearchRuns: (projectId: string, signal?: AbortSignal) =>
    apiRequest<{ items: ResearchRun[] }>(
      `/platform/v1/projects/${encodeURIComponent(projectId)}/knowledge/research-runs?limit=20`,
      { signal },
    ),

  getResearchRun: (projectId: string, researchRunId: string, signal?: AbortSignal) =>
    apiRequest<ResearchRun>(
      `/platform/v1/projects/${encodeURIComponent(projectId)}/knowledge/research-runs/${encodeURIComponent(researchRunId)}`,
      { signal },
    ),
}
