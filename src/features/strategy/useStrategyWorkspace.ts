import { useCallback, useEffect, useRef, useState } from 'react'
import { BackendApiError } from '../../backend/platform'
import { strategyApi, createMutationKey } from './api'
import { useConversationStream } from './useConversationStream'
import type {
  BriefDraft,
  BriefVersion,
  ConversationMemory,
  DeepReviewAnalysis,
  DraftRevision,
  GenerationMetadata,
  GenerationProbe,
  GenerationReadiness,
  KnowledgeDocument,
  Message,
  PackageVersion,
  ResearchRun,
  Review,
  ReviewComment,
  SkillRun,
  StrategyDraft,
  Workspace,
  WorkspaceDetail,
} from './types'

export type StrategyWorkspaceState = {
  workspaces: Workspace[]
  detail: WorkspaceDetail | null
  messages: Message[]
  memory: ConversationMemory | null
  brief: BriefDraft | null
  briefVersion: BriefVersion | null
  draft: StrategyDraft | null
  revisions: DraftRevision[]
  review: Review | null
  comments: ReviewComment[]
  deepReview: DeepReviewAnalysis | null
  published: PackageVersion | null
  packages: PackageVersion[]
  readiness: GenerationReadiness | null
  probe: GenerationProbe | null
  metadata: GenerationMetadata | null
  skillRuns: SkillRun[]
  documents: KnowledgeDocument[]
  researchRun: ResearchRun | null
  isLoading: boolean
  busy: string
  error: string
  pendingAgentTaskId: string
}

function messageOf(error: unknown) {
  if (error instanceof BackendApiError && error.code === 'VERSION_CONFLICT') {
    return '服务端版本已经变化，已重新加载最新内容，请确认后再次操作。'
  }
  return error instanceof Error ? error.message : '策略工作区操作失败。'
}

function agentFailureMessage(code?: string, message?: string) {
  if (code === 'MODEL_RATE_LIMITED') return '文本模型请求频率受限，请稍后再点击“重新生成策略”。'
  if (code === 'MODEL_OUTPUT_INVALID') return '模型输出未通过策略结构校验，可以重新生成。'
  if (code === 'MODEL_UNAVAILABLE') return '文本模型当前不可用，请检查模型配置后重试。'
  return message || '本轮 Strategy Agent 任务未完成。'
}

export function useStrategyWorkspace(projectId: string, preferredWorkspaceId = '') {
  const [state, setState] = useState<StrategyWorkspaceState>({
    workspaces: [],
    detail: null,
    messages: [],
    memory: null,
    brief: null,
    briefVersion: null,
    draft: null,
    revisions: [],
    review: null,
    comments: [],
    deepReview: null,
    published: null,
    packages: [],
    readiness: null,
    probe: null,
    metadata: null,
    skillRuns: [],
    documents: [],
    researchRun: null,
    isLoading: true,
    busy: '',
    error: '',
    pendingAgentTaskId: '',
  })
  const currentWorkspaceId = useRef('')
  const approvalMutationKey = useRef('')

  const load = useCallback(async (signal?: AbortSignal, requestedWorkspaceId?: string) => {
    const workspaceResult = await strategyApi.listWorkspaces(projectId, signal)
    const workspaces = [...workspaceResult.items].sort((left, right) =>
      Number(right.is_primary) - Number(left.is_primary) || right.version - left.version)
    const targetId = requestedWorkspaceId
      || preferredWorkspaceId
      || currentWorkspaceId.current
      || workspaces.find(workspace => workspace.is_primary)?.id
      || workspaces[0]?.id
      || ''

    if (!targetId) {
      const [readiness, documents, packages, researchRuns] = await Promise.all([
        strategyApi.getGenerationReadiness(projectId, signal).catch(() => null),
        strategyApi.listKnowledgeDocuments(projectId, signal).then(value => value.items).catch(() => []),
        strategyApi.listStrategyPackages(projectId, signal).then(value => value.items).catch(() => []),
        strategyApi.listResearchRuns(projectId, signal).then(value => value.items).catch(() => []),
      ])
      setState(current => ({
        ...current,
        workspaces,
        detail: null,
        readiness,
        documents,
        packages,
        researchRun: researchRuns[0] ?? null,
        published: packages.find(value => value.status === 'published') ?? null,
        isLoading: false,
      }))
      return
    }

    currentWorkspaceId.current = targetId
    const detail = await strategyApi.getWorkspace(targetId, signal)
    if (detail.workspace.project_id !== projectId) throw new Error('策略工作区不属于当前 Project。')
    const conversation = detail.current_conversation
    const task = detail.current_task

    const [messageResult, brief, packages, readiness, documents, memory, skillRuns, researchRuns] = await Promise.all([
      conversation ? strategyApi.listMessages(conversation.id, signal).then(value => value.items) : Promise.resolve([]),
      task ? strategyApi.getBriefDraft(task.id, signal) : Promise.resolve(null),
      strategyApi.listStrategyPackages(projectId, signal).then(value => value.items).catch(() => []),
      strategyApi.getGenerationReadiness(projectId, signal).catch(() => null),
      strategyApi.listKnowledgeDocuments(projectId, signal).then(value => value.items).catch(() => []),
      conversation ? strategyApi.getConversationMemory(conversation.id, signal).catch(() => null) : Promise.resolve(null),
      task?.current_agent_task_id
        ? strategyApi.listSkillRuns(task.current_agent_task_id, signal).then(value => value.items).catch(() => [])
        : Promise.resolve([]),
      strategyApi.listResearchRuns(projectId, signal).then(value => value.items).catch(() => []),
    ])

    let briefVersion: BriefVersion | null = null
    if (task && brief?.status === 'confirmed') {
      const versions = await strategyApi.listBriefVersions(task.brief_id, signal)
      briefVersion = [...versions.items].sort((left, right) => right.version - left.version)[0] ?? null
    }

    let draft: StrategyDraft | null = null
    let revisions: DraftRevision[] = []
    let metadata: GenerationMetadata | null = null
    let review: Review | null = null
    let comments: ReviewComment[] = []
    let deepReview: DeepReviewAnalysis | null = null
    let published: PackageVersion | null = null
    let agentFailure = ''
    if (task?.current_strategy_id) {
      draft = await strategyApi.getStrategy(task.current_strategy_id, signal)
      if (draft.status === 'failed' && task.current_agent_task_id) {
        const inspection = await strategyApi.getAgentTask(task.current_agent_task_id, signal).catch(() => null)
        const problem = inspection?.task.error ?? inspection?.job?.error
        agentFailure = agentFailureMessage(problem?.code, problem?.message)
      }
      if (draft.current_revision > 0) {
        const [revisionResult, generationMetadata] = await Promise.all([
          strategyApi.listStrategyRevisions(draft.id, signal),
          strategyApi.getGenerationMetadata(draft.id, signal),
        ])
        revisions = revisionResult.items
        metadata = generationMetadata
      }
      published = packages
        .filter(value => value.status === 'published' && value.snapshot.strategy_id === draft?.id)
        .sort((left, right) => right.version - left.version)[0] ?? null
      if (draft.current_review_id) {
        review = await strategyApi.getReview(draft.current_review_id, signal)
        const [commentResult, analysis] = await Promise.all([
          strategyApi.listReviewComments(review.id, signal),
          strategyApi.getDeepReview(review.id, signal).catch(() => null),
        ])
        comments = commentResult.items
        deepReview = analysis
      }
    }

    setState(current => ({
      ...current,
      workspaces,
      detail,
      messages: messageResult,
      memory,
      brief,
      briefVersion,
      draft,
      revisions,
      review,
      comments,
      deepReview,
      packages,
      published,
      readiness,
      metadata,
      skillRuns,
      documents,
      researchRun: researchRuns.find(run =>
        run.artifacts.some(artifact => brief?.document.reference_ids?.includes(artifact.id)),
      ) ?? null,
      isLoading: false,
      error: agentFailure,
    }))
  }, [preferredWorkspaceId, projectId])

  const reload = useCallback(async () => {
    try {
      await load()
    } catch (error) {
      setState(current => ({ ...current, isLoading: false, error: messageOf(error) }))
    }
  }, [load])

  useEffect(() => {
    currentWorkspaceId.current = preferredWorkspaceId
    setState(current => ({ ...current, isLoading: true, error: '' }))
    const controller = new AbortController()
    void load(controller.signal).catch(error => {
      if (!(error instanceof DOMException && error.name === 'AbortError')) {
        setState(current => ({ ...current, isLoading: false, error: messageOf(error) }))
      }
    })
    return () => controller.abort()
  }, [load, preferredWorkspaceId])

  useConversationStream(state.detail?.current_conversation?.id, reload)

  useEffect(() => {
    const agentTaskId = state.pendingAgentTaskId
    if (!agentTaskId) return
    const controller = new AbortController()
    let timer = 0
    const inspect = async () => {
      try {
        const inspection = await strategyApi.getAgentTask(agentTaskId, controller.signal)
        const task = inspection.task
        if (task.status === 'failed' || task.status === 'cancelled') {
          const problem = task.error ?? inspection.job?.error
          const failureMessage = agentFailureMessage(problem?.code, problem?.message)
          await reload()
          setState(current => ({
            ...current,
            pendingAgentTaskId: '',
            error: failureMessage,
          }))
          return
        }
        if (task.status === 'succeeded') {
          setState(current => ({ ...current, pendingAgentTaskId: '' }))
          await reload()
          return
        }
        timer = window.setTimeout(inspect, 1500)
      } catch {
        if (!controller.signal.aborted) timer = window.setTimeout(inspect, 2500)
      }
    }
    timer = window.setTimeout(inspect, 800)
    return () => {
      controller.abort()
      window.clearTimeout(timer)
    }
  }, [reload, state.pendingAgentTaskId])

  useEffect(() => {
    const researchRun = state.researchRun
    if (!researchRun || researchRun.status !== 'running') return
    const controller = new AbortController()
    let timer = 0
    const inspect = async () => {
      try {
        const next = await strategyApi.getResearchRun(projectId, researchRun.id, controller.signal)
        setState(current => ({ ...current, researchRun: next }))
        if (next.status === 'running') {
          timer = window.setTimeout(inspect, 1800)
          return
        }
        const task = state.detail?.current_task
        const brief = state.brief
        if (next.status === 'succeeded' && next.artifacts.length && task && brief?.status === 'open') {
          const references = Array.from(new Set([
            ...(brief.document.reference_ids ?? []),
            ...next.artifacts.map(artifact => artifact.id),
          ]))
          const updated = await strategyApi.patchBriefField(
            task.id,
            brief,
            'reference_ids',
            references,
            createMutationKey('strategy-research-reference'),
          )
          setState(current => ({ ...current, brief: updated }))
        }
      } catch (error) {
        if (!controller.signal.aborted) {
          setState(current => ({ ...current, error: messageOf(error) }))
          timer = window.setTimeout(inspect, 3000)
        }
      }
    }
    timer = window.setTimeout(inspect, 800)
    return () => {
      controller.abort()
      window.clearTimeout(timer)
    }
  }, [projectId, state.brief, state.detail?.current_task, state.researchRun])

  const perform = useCallback(async (name: string, action: () => Promise<void>, reloadAfter = true) => {
    setState(current => ({ ...current, busy: name, error: '' }))
    try {
      await action()
      if (reloadAfter) await load()
      return true
    } catch (error) {
      if (error instanceof BackendApiError && error.code === 'VERSION_CONFLICT') {
        await load().catch(() => undefined)
      }
      setState(current => ({ ...current, error: messageOf(error) }))
      return false
    } finally {
      setState(current => ({ ...current, busy: '' }))
    }
  }, [load])

  const actions = {
    reload,
    probeGeneration: () => perform('generation-probe', async () => {
      const probe = await strategyApi.probeGeneration(projectId)
      setState(current => ({ ...current, probe }))
    }, false),
    createWorkspace: () => perform('workspace', async () => {
      const workspace = await strategyApi.createWorkspace(
        projectId,
        '主策略工作区',
        createMutationKey('strategy-workspace'),
      )
      currentWorkspaceId.current = workspace.id
    }),
    selectWorkspace: (workspaceId: string) => perform('workspace', async () => {
      currentWorkspaceId.current = workspaceId
      await load(undefined, workspaceId)
    }, false),
    startConversation: () => perform('conversation', async () => {
      const workspaceId = state.detail?.workspace.id
      if (!workspaceId) throw new Error('请先创建策略工作区。')
      const bundle = await strategyApi.createConversation(
        projectId,
        workspaceId,
        createMutationKey('strategy-conversation'),
      )
      setState(current => ({
        ...current,
        detail: current.detail
          ? { ...current.detail, current_conversation: bundle.conversation, current_task: bundle.task }
          : current.detail,
        brief: bundle.brief_draft,
      }))
    }),
    sendMessage: (content: string) => perform('message', async () => {
      const conversationId = state.detail?.current_conversation?.id
      if (!conversationId) throw new Error('请先开始需求对话。')
      const result = await strategyApi.sendMessage(
        conversationId,
        content,
        createMutationKey('strategy-message'),
      )
      setState(current => ({
        ...current,
        messages: current.messages.some(message => message.id === result.message.id)
          ? current.messages
          : [...current.messages, result.message],
        pendingAgentTaskId: result.agent_task.id,
      }))
    }, false),
    patchBriefField: (fieldPath: string, value: unknown) => perform(`brief:${fieldPath}`, async () => {
      const task = state.detail?.current_task
      if (!task || !state.brief) throw new Error('Brief 草稿尚未创建。')
      const brief = await strategyApi.patchBriefField(
        task.id,
        state.brief,
        fieldPath,
        value,
        createMutationKey('strategy-brief-patch'),
      )
      setState(current => ({ ...current, brief }))
    }, false),
    confirmBriefFields: (operations: Array<{ fieldPath: string; value: unknown }>) => perform('confirm-brief-fields', async () => {
      const task = state.detail?.current_task
      if (!task || !state.brief) throw new Error('Brief 草稿尚未创建。')
      if (!operations.length) return
      const brief = await strategyApi.patchBriefFields(
        task.id,
        state.brief,
        operations,
        createMutationKey('strategy-brief-confirm-fields'),
      )
      setState(current => ({ ...current, brief }))
    }, false),
    confirmBrief: () => perform('confirm-brief', async () => {
      const task = state.detail?.current_task
      if (!task || !state.brief) throw new Error('Brief 草稿尚未创建。')
      await strategyApi.confirmBrief(
        task.id,
        state.brief.version,
        createMutationKey('strategy-brief-confirm'),
      )
    }),
    generateStrategy: () => perform('generate-strategy', async () => {
      const task = state.detail?.current_task
      if (!task || !state.briefVersion) throw new Error('请先确认 Brief。')
      const result = await strategyApi.createStrategy(
        task.id,
        state.briefVersion,
        createMutationKey('strategy-create'),
      )
      setState(current => ({
        ...current,
        draft: result.strategy_draft,
        pendingAgentTaskId: result.agent_task.id,
      }))
    }, false),
    retryStrategy: () => perform('retry-strategy', async () => {
      if (!state.draft) throw new Error('失败的策略草稿不存在。')
      const result = await strategyApi.retryStrategy(
        state.draft,
        createMutationKey('strategy-retry'),
      )
      setState(current => ({
        ...current,
        draft: result.strategy_draft,
        pendingAgentTaskId: result.agent_task.id,
      }))
    }, false),
    patchStrategySection: (section: string, value: unknown) => perform(`strategy:${section}`, async () => {
      if (!state.draft) throw new Error('策略草稿尚未创建。')
      const draft = await strategyApi.patchStrategySection(
        state.draft,
        section,
        value,
        createMutationKey('strategy-section'),
      )
      setState(current => ({ ...current, draft, review: null }))
    }, false),
    reviseStrategy: (instruction: string) => perform('revise-strategy', async () => {
      if (!state.draft) throw new Error('策略草稿尚未创建。')
      const agentTask = await strategyApi.reviseStrategy(
        state.draft,
        instruction,
        createMutationKey('strategy-revise'),
      )
      setState(current => ({
        ...current,
        draft: current.draft ? { ...current.draft, status: 'generating' } : current.draft,
        pendingAgentTaskId: agentTask.id,
      }))
    }, false),
    submitStrategy: () => perform('submit-review', async () => {
      if (!state.draft) throw new Error('策略草稿尚未创建。')
      const review = await strategyApi.submitStrategy(
        state.draft,
        createMutationKey('strategy-submit'),
      )
      approvalMutationKey.current = ''
      setState(current => ({ ...current, review }))
    }),
    addComment: (body: string) => perform('review-comment', async () => {
      if (!state.review) throw new Error('当前没有可评论的评审。')
      await strategyApi.addReviewComment(state.review.id, body)
    }),
    startDeepReview: () => perform('deep-review', async () => {
      if (!state.review || state.review.status !== 'open') throw new Error('只有进行中的评审可以启动深度分析。')
      const result = await strategyApi.startDeepReview(
        state.review.id, state.review.status, createMutationKey('strategy-deep-review'),
      )
      setState(current => ({
        ...current,
        deepReview: result.analysis,
        pendingAgentTaskId: result.agent_task.id,
      }))
    }, false),
    returnReview: (reason: string) => perform('return-review', async () => {
      if (!state.review) throw new Error('当前没有可退回的评审。')
      await strategyApi.returnReview(state.review.id, reason)
    }),
    approveReview: () => perform('approve-review', async () => {
      if (!state.review || !state.draft) throw new Error('当前没有可批准的评审。')
      if (!approvalMutationKey.current) {
        approvalMutationKey.current = createMutationKey('strategy-approve')
      }
      await strategyApi.approveStrategy(state.draft, state.review, approvalMutationKey.current)
      approvalMutationKey.current = ''
    }),
    uploadDocument: (file: File) => perform('upload-document', async () => {
      const document = await strategyApi.uploadKnowledgeDocument(projectId, file)
      setState(current => ({ ...current, documents: [document, ...current.documents] }))
      const task = state.detail?.current_task
      if (task && state.brief?.status === 'open') {
        const references = Array.from(new Set([...(state.brief.document.reference_ids ?? []), document.id]))
        const brief = await strategyApi.patchBriefField(
          task.id,
          state.brief,
          'reference_ids',
          references,
          createMutationKey('strategy-brief-reference'),
        )
        setState(current => ({ ...current, brief }))
      }
    }, false),
    runResearch: (
      mode: 'web' | 'mcp',
      category: 'general' | 'audience' | 'competitor' | 'industry',
      query: string,
      includeDocuments: boolean,
    ) =>
      perform('research', async () => {
        const documentIds = includeDocuments ? state.documents.map(document => document.id).slice(0, 20) : []
        const researchRun = await strategyApi.runExternalResearch(projectId, {
          mode,
          category,
          query,
          document_ids: documentIds,
          disclosed_fields: documentIds.length ? ['query', 'document_content'] : ['query'],
          confirmed: true,
        })
        setState(current => ({ ...current, researchRun }))
        if (researchRun.artifacts.length && state.brief?.status === 'open' && state.detail?.current_task) {
          const references = Array.from(new Set([
            ...(state.brief.document.reference_ids ?? []),
            ...researchRun.artifacts.map(artifact => artifact.id),
          ]))
          const brief = await strategyApi.patchBriefField(
            state.detail.current_task.id,
            state.brief,
            'reference_ids',
            references,
            createMutationKey('strategy-research-reference'),
          )
          setState(current => ({ ...current, brief }))
        }
      }, false),
  }

  return { state, actions }
}
