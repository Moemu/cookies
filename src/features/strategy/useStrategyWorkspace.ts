import { useCallback, useEffect, useRef, useState } from 'react'
import { BackendApiError } from '../../backend/platform'
import { strategyApi, createMutationKey } from './api'
import { buildConversationMessageCreate, requirementConfirmationOperations } from './strategyConversationModel'
import { useActivityStream } from './useActivityStream'
import { useConversationStream } from './useConversationStream'
import type {
  ActivityConnection,
  BriefDraft,
  BriefVersion,
  ConversationCapabilities,
  ConversationMemory,
  CreativeIntakeV4,
  DeepReviewAnalysis,
  DraftRevision,
  GenerationMetadata,
  GenerationProbe,
  GenerationReadiness,
  KnowledgeDocument,
  MediaUnderstandingArtifact,
  Message,
  MessageRequestedPolicy,
  PackageVersion,
  ResearchRun,
  Review,
  ReviewComment,
  ReviewPolicy,
  SkillRun,
  StrategyDraft,
  StrategyP0Metrics,
  TaskActivity,
  TaskActivitySnapshot,
  ProjectContextManifest,
  Workspace,
  WorkspaceDetail,
} from './types'
import type { BriefProductCandidate } from './types'
import { briefProductCandidateOperations } from './briefProductCandidate'

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
  reviewPolicy: ReviewPolicy | null
  comments: ReviewComment[]
  deepReview: DeepReviewAnalysis | null
  deepReviewError: string
  published: PackageVersion | null
  packages: PackageVersion[]
  readiness: GenerationReadiness | null
  probe: GenerationProbe | null
  metadata: GenerationMetadata | null
  skillRuns: SkillRun[]
  documents: KnowledgeDocument[]
  mediaArtifacts: MediaUnderstandingArtifact[]
  conversationCapabilities: ConversationCapabilities | null
  p0Metrics: StrategyP0Metrics | null
  researchRun: ResearchRun | null
  researchRuns: ResearchRun[]
  activities: TaskActivity[]
  activityConnection: ActivityConnection
  activityError: string
  isLoading: boolean
  busy: string
  error: string
  conversationNotice: string
  pendingAgentTaskId: string
  pendingAgentPurpose: 'deep_review' | 'general' | ''
}

function messageOf(error: unknown) {
  if (error instanceof BackendApiError && error.code === 'VERSION_CONFLICT') {
    return '服务端版本已经变化，已重新加载最新内容，请确认后再次操作。'
  }
  return error instanceof Error ? error.message : '策略工作区操作失败。'
}

export function agentFailureMessage(code?: string, message?: string, kind?: string) {
  if (code === 'CONVERSATION_WEB_SEARCH_FAILED') {
    return '联网搜索未完成，因此本轮没有生成无来源回答。可以重新发送，或关闭联网搜索后改用普通回答。'
  }
  if (code === 'MODEL_RATE_LIMITED') {
    return kind === 'strategy.brief.extract'
      ? '文本模型请求暂时受限，请稍后重新发送需求消息。'
      : '文本模型请求频率受限，请稍后再点击“重新生成策略”。'
  }
  if (code === 'MODEL_REQUEST_REJECTED') return '文本模型不支持当前路由参数，请联系管理员检查模型配置后重试。'
  if (code === 'MODEL_OUTPUT_INVALID') return '模型输出未通过策略结构校验，可以重新生成。'
  if (code === 'MODEL_UNAVAILABLE') return '文本模型当前不可用，请检查模型配置后重试。'
  if (code === 'AGENT_EXECUTION_FAILED' || code === 'JOB_EXECUTION_FAILED') {
    return 'AI 助手本轮执行未完成，请重新发送。已填写的内容和历史对话不会丢失。'
  }
  return message || '本轮 Strategy Agent 任务未完成。'
}

const recentWorkspaceKey = 'cookies.strategy-workspace-selection.v1'

function readRememberedWorkspace(projectId: string) {
  try {
    const raw = window.localStorage.getItem(recentWorkspaceKey)
    if (!raw) return ''
    const parsed = JSON.parse(raw) as unknown
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return ''
    const workspaceId = (parsed as Record<string, unknown>)[projectId]
    return typeof workspaceId === 'string' ? workspaceId : ''
  } catch {
    return ''
  }
}

function rememberWorkspace(projectId: string, workspaceId: string) {
  try {
    const raw = window.localStorage.getItem(recentWorkspaceKey)
    const parsed = raw ? JSON.parse(raw) as unknown : {}
    const current = parsed && typeof parsed === 'object' && !Array.isArray(parsed)
      ? parsed as Record<string, unknown>
      : {}
    window.localStorage.setItem(recentWorkspaceKey, JSON.stringify({
      ...current,
      [projectId]: workspaceId,
    }))
  } catch {
    // Workspace selection is a navigation preference and must not block loading.
  }
}

export function useStrategyWorkspace(projectId: string, preferredWorkspaceId = '', activityCursorScope = '') {
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
    reviewPolicy: null,
    comments: [],
    deepReview: null,
    deepReviewError: '',
    published: null,
    packages: [],
    readiness: null,
    probe: null,
    metadata: null,
    skillRuns: [],
    documents: [],
    mediaArtifacts: [],
    conversationCapabilities: null,
    p0Metrics: null,
    researchRun: null,
    researchRuns: [],
    activities: [],
    activityConnection: 'connecting',
    activityError: '',
    isLoading: true,
    busy: '',
    error: '',
    conversationNotice: '',
    pendingAgentTaskId: '',
    pendingAgentPurpose: '',
  })
  const currentWorkspaceId = useRef('')
  const approvalMutationKey = useRef('')
  const stateRef = useRef(state)
  const loadSequence = useRef(0)
  const briefMutationQueue = useRef<Promise<void>>(Promise.resolve())
  const activitySnapshotRef = useRef<TaskActivitySnapshot | null>(null)
  const activityScopeRef = useRef('')
  const activityWorkspaceId = state.detail?.workspace.id ?? ''
  stateRef.current = state

  const load = useCallback(async (signal?: AbortSignal, requestedWorkspaceId?: string) => {
    const sequence = ++loadSequence.current
    const workspaceResult = await strategyApi.listWorkspaces(projectId, signal)
    if (sequence !== loadSequence.current) return
    const workspaces = [...workspaceResult.items].sort((left, right) =>
      Number(right.is_primary) - Number(left.is_primary) || right.version - left.version)
    const rememberedWorkspaceId = readRememberedWorkspace(projectId)
    const validRememberedWorkspaceId = workspaces.some(workspace => workspace.id === rememberedWorkspaceId)
      ? rememberedWorkspaceId
      : ''
    const targetId = requestedWorkspaceId
      || preferredWorkspaceId
      || currentWorkspaceId.current
      || validRememberedWorkspaceId
      || workspaces.find(workspace => workspace.is_primary)?.id
      || workspaces[0]?.id
      || ''

    if (!targetId) {
      const [readiness, documents, packages, researchRuns, conversationCapabilities, p0Metrics, reviewPolicy] = await Promise.all([
        strategyApi.getGenerationReadiness(projectId, signal).catch(() => null),
        strategyApi.listKnowledgeDocuments(projectId, signal).then(value => value.items).catch(() => []),
        strategyApi.listStrategyPackages(projectId, signal).then(value => value.items).catch(() => []),
        strategyApi.listResearchRuns(projectId, signal).then(value => value.items).catch(() => []),
        strategyApi.getConversationCapabilities(signal).catch(() => null),
        strategyApi.getP0Metrics(projectId, 30, signal).catch(() => null),
        strategyApi.getReviewPolicy(projectId, signal),
      ])
      if (sequence !== loadSequence.current) return
      setState(current => ({
        ...current,
        workspaces,
        detail: null,
        readiness,
        documents,
        mediaArtifacts: [],
        conversationCapabilities,
        p0Metrics,
        packages,
        reviewPolicy,
        researchRun: researchRuns[0] ?? null,
        researchRuns,
        published: packages.find(value => value.status === 'published') ?? null,
        isLoading: false,
      }))
      return
    }

    currentWorkspaceId.current = targetId
    const detail = await strategyApi.getWorkspace(targetId, signal)
    if (sequence !== loadSequence.current) return
    if (detail.workspace.project_id !== projectId) throw new Error('策略工作区不属于当前 Project。')
    rememberWorkspace(projectId, detail.workspace.id)
    const conversation = detail.current_conversation
    const task = detail.current_task

    const messagePromise: Promise<Message[]> = conversation
      ? strategyApi.listMessages(conversation.id, signal).then(value => value.items)
      : Promise.resolve([])
    const memoryPromise: Promise<ConversationMemory | null> = conversation
      ? messagePromise.then(messages => messages.length > 0
        ? strategyApi.getConversationMemory(conversation.id, signal).catch(() => null)
        : null)
      : Promise.resolve(null)

    const [messageResult, brief, packages, readiness, documents, memory, skillRuns, researchRuns, conversationCapabilities, p0Metrics, reviewPolicy] = await Promise.all([
      messagePromise,
      task ? strategyApi.getBriefDraft(task.id, signal) : Promise.resolve(null),
      strategyApi.listStrategyPackages(projectId, signal).then(value => value.items).catch(() => []),
      strategyApi.getGenerationReadiness(projectId, signal).catch(() => null),
      strategyApi.listKnowledgeDocuments(projectId, signal).then(value => value.items).catch(() => []),
      memoryPromise,
      task?.current_agent_task_id
        ? strategyApi.listSkillRuns(task.current_agent_task_id, signal).then(value => value.items).catch(() => [])
        : Promise.resolve([]),
      strategyApi.listResearchRuns(projectId, signal).then(value => value.items).catch(() => []),
      strategyApi.getConversationCapabilities(signal).catch(() => null),
      strategyApi.getP0Metrics(projectId, 30, signal).catch(() => null),
      strategyApi.getReviewPolicy(projectId, signal),
    ])
    const mediaArtifacts = await loadConversationMedia(projectId, messageResult, signal)
    if (sequence !== loadSequence.current) return

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
    let deepReviewFailure = ''
    let published: PackageVersion | null = null
    let agentFailure = ''
    let recoveredAgentTaskId = ''
    if (task?.current_agent_task_id) {
      const inspection = await strategyApi.getAgentTask(task.current_agent_task_id, signal).catch(() => null)
      if (inspection?.task.kind === 'strategy.brief.extract' &&
        ['dispatch_pending', 'queued', 'running'].includes(inspection.task.status)) {
        recoveredAgentTaskId = inspection.task.id
      }
    }
    if (task?.current_strategy_id) {
      draft = await strategyApi.getStrategy(task.current_strategy_id, signal)
      if (draft.status === 'failed' && task.current_agent_task_id) {
        const inspection = await strategyApi.getAgentTask(task.current_agent_task_id, signal).catch(() => null)
        const problem = inspection?.task.error ?? inspection?.job?.error
        agentFailure = agentFailureMessage(problem?.code, problem?.message, inspection?.task.kind)
      }
      if (draft.current_revision > 0) {
        const [revisionResult, generationMetadata, perspective] = await Promise.all([
          strategyApi.listStrategyRevisions(draft.id, signal),
          strategyApi.getGenerationMetadata(draft.id, signal),
          strategyApi.getStrategyPerspective(draft.id, signal).catch(() => null),
        ])
        revisions = revisionResult.items
        metadata = generationMetadata
        deepReview = perspective
        if (perspective?.status === 'failed' && perspective.agent_task_id) {
          const inspection = await strategyApi.getAgentTask(perspective.agent_task_id, signal).catch(() => null)
          const problem = inspection?.task.error ?? inspection?.job?.error
          deepReviewFailure = agentFailureMessage(problem?.code, problem?.message)
        }
      }
      published = packages
        .filter(value => value.status === 'published' && value.snapshot.strategy_id === draft?.id)
        .sort((left, right) => right.version - left.version)[0] ?? null
      if (draft.current_review_id) {
        review = await strategyApi.getReview(draft.current_review_id, signal)
        const [commentResult, legacyAnalysis] = await Promise.all([
          strategyApi.listReviewComments(review.id, signal),
          deepReview ? Promise.resolve(null) : strategyApi.getDeepReview(review.id, signal).catch(() => null),
        ])
        comments = commentResult.items
        deepReview = deepReview ?? legacyAnalysis
        if (!deepReviewFailure && legacyAnalysis?.status === 'failed' && legacyAnalysis.agent_task_id) {
          const inspection = await strategyApi.getAgentTask(legacyAnalysis.agent_task_id, signal).catch(() => null)
          const problem = inspection?.task.error ?? inspection?.job?.error
          deepReviewFailure = agentFailureMessage(problem?.code, problem?.message)
        }
      }
    }

    if (sequence !== loadSequence.current) return
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
      reviewPolicy,
      comments,
      deepReview,
      deepReviewError: deepReviewFailure,
      packages,
      published,
      readiness,
      metadata,
      skillRuns,
      documents,
      mediaArtifacts: mergeConversationMedia(current.mediaArtifacts, mediaArtifacts, projectId),
      conversationCapabilities,
      p0Metrics,
	  researchRun: researchRuns.find(run => run.run_mode === 'deep') ?? null,
      researchRuns,
      isLoading: false,
      error: agentFailure,
      pendingAgentTaskId: recoveredAgentTaskId || current.pendingAgentTaskId,
      pendingAgentPurpose: recoveredAgentTaskId ? 'general' : current.pendingAgentPurpose,
    }))
  }, [preferredWorkspaceId, projectId])

  const reload = useCallback(async () => {
    try {
      await load()
    } catch (error) {
      setState(current => ({ ...current, isLoading: false, error: messageOf(error) }))
    }
  }, [load])

  const reconcileConversation = useCallback(async () => {
    const current = stateRef.current
    const workspaceId = current.detail?.workspace.id
    const conversationId = current.detail?.current_conversation?.id
    if (!workspaceId || !conversationId) return
    const messagesPromise = strategyApi.listMessages(conversationId).then(value => value.items)
    const memoryPromise: Promise<ConversationMemory | null> = messagesPromise.then(messages => messages.length > 0
      ? strategyApi.getConversationMemory(conversationId).catch(() => null)
      : null)
    const [detail, messages, memory, researchRuns] = await Promise.all([
      strategyApi.getWorkspace(workspaceId),
      messagesPromise,
      memoryPromise,
      strategyApi.listResearchRuns(projectId).then(value => value.items).catch(() => current.researchRuns),
    ])
    const task = detail.current_task
    const brief = task ? await strategyApi.getBriefDraft(task.id).catch(() => current.brief) : null
    let briefVersion = current.briefVersion
    if (task && brief?.status === 'confirmed') {
      const versions = await strategyApi.listBriefVersions(task.brief_id)
      briefVersion = [...versions.items].sort((left, right) => right.version - left.version)[0] ?? null
    }
    setState(value => ({
      ...value,
      detail,
      messages,
      memory,
      brief,
      briefVersion,
      researchRuns,
	  researchRun: researchRuns.find(run => run.run_mode === 'deep') ?? value.researchRun,
    }))
  }, [projectId])

  const reconcileStrategy = useCallback(async () => {
    const current = stateRef.current
    const strategyId = current.draft?.id ?? current.detail?.current_task?.current_strategy_id
    if (!strategyId) return
    const draft = await strategyApi.getStrategy(strategyId)
    const [revisionResult, metadata] = draft.current_revision > 0
      ? await Promise.all([
          strategyApi.listStrategyRevisions(draft.id),
          strategyApi.getGenerationMetadata(draft.id),
        ])
      : [{ items: [] as DraftRevision[] }, null]
    setState(value => ({
      ...value,
      draft,
      revisions: revisionResult.items,
      metadata,
    }))
  }, [])

  const reconcileDeepReview = useCallback(async () => {
    const draft = stateRef.current.draft
    if (!draft) return
    const analysis = await strategyApi.getStrategyPerspective(draft.id).catch(() => null)
    setState(value => ({ ...value, deepReview: analysis }))
  }, [])

  const reconcileAgentActivity = useCallback(async (activity: TaskActivity) => {
    const execution = activity.execution_ref
    if (!execution || execution.type !== 'strategy_agent_task') return
    const inspection = await strategyApi.getAgentTask(execution.id)
    if (activity.kind === 'assistant') await reconcileConversation()
    if (activity.kind === 'strategy_generation') await reconcileStrategy()
    if (activity.kind === 'deep_review') await reconcileDeepReview()
    const problem = inspection.task.error ?? inspection.job?.error
    const failureMessage = activity.status === 'failed'
      ? agentFailureMessage(problem?.code, problem?.message, inspection.task.kind)
      : ''
    setState(value => ({
      ...value,
      pendingAgentTaskId: value.pendingAgentTaskId === execution.id ? '' : value.pendingAgentTaskId,
      pendingAgentPurpose: value.pendingAgentTaskId === execution.id ? '' : value.pendingAgentPurpose,
      error: activity.kind === 'deep_review' ? value.error : failureMessage,
      deepReviewError: activity.kind === 'deep_review' ? failureMessage : value.deepReviewError,
      conversationNotice: activity.kind === 'assistant' ? '' : value.conversationNotice,
    }))
  }, [reconcileConversation, reconcileDeepReview, reconcileStrategy])

  const handleActivitySnapshot = useCallback((snapshot: TaskActivitySnapshot) => {
    const scope = `${projectId}:${activityWorkspaceId}`
    const previous = activityScopeRef.current === scope ? activitySnapshotRef.current : null
    activityScopeRef.current = scope
    activitySnapshotRef.current = snapshot
    const activeAgent = snapshot.items.find(item =>
      item.execution_ref?.type === 'strategy_agent_task' &&
      ['queued', 'running', 'waiting_user', 'stalled'].includes(item.status),
    )
    setState(current => ({
      ...current,
      activities: snapshot.items,
      activityError: '',
      pendingAgentTaskId: activeAgent?.execution_ref?.id ?? current.pendingAgentTaskId,
      pendingAgentPurpose: activeAgent
        ? activeAgent.kind === 'deep_review' ? 'deep_review' : 'general'
        : current.pendingAgentPurpose,
    }))
    const plan = activityReconciliationPlan(previous, snapshot)
    const tasks: Array<Promise<unknown>> = plan.agentActivities.map(reconcileAgentActivity)
    if (plan.refreshDocuments) {
      tasks.push(strategyApi.listKnowledgeDocuments(projectId).then(result => {
        setState(current => ({ ...current, documents: result.items }))
      }))
    }
    for (const researchRunId of plan.researchRunIds) {
      tasks.push(strategyApi.getResearchRun(projectId, researchRunId).then(run => {
        setState(current => ({
          ...current,
          researchRun: !current.researchRun || current.researchRun.id === run.id ? run : current.researchRun,
          researchRuns: [run, ...current.researchRuns.filter(value => value.id !== run.id)],
        }))
      }))
    }
    if (tasks.length) {
      void Promise.all(tasks).catch(error => {
        setState(current => ({ ...current, activityError: messageOf(error) }))
      })
    }
  }, [activityWorkspaceId, projectId, reconcileAgentActivity])

  const handleActivityConnection = useCallback((connection: ActivityConnection, message = '') => {
    setState(current => ({
      ...current,
      activityConnection: connection,
      activityError: connection === 'live' ? '' : message || current.activityError,
    }))
  }, [])

  useEffect(() => {
    currentWorkspaceId.current = preferredWorkspaceId
    activitySnapshotRef.current = null
    activityScopeRef.current = ''
    setState(current => ({
      ...current,
      activities: [],
      activityConnection: 'connecting',
      activityError: '',
      isLoading: true,
      error: '',
    }))
    const controller = new AbortController()
    void load(controller.signal).catch(error => {
      if (!(error instanceof DOMException && error.name === 'AbortError')) {
        setState(current => ({ ...current, isLoading: false, error: messageOf(error) }))
      }
    })
    return () => controller.abort()
  }, [load, preferredWorkspaceId])

  useActivityStream({
    projectId,
    workspaceId: activityWorkspaceId,
    cursorScope: activityCursorScope,
    onSnapshot: handleActivitySnapshot,
    onConnection: handleActivityConnection,
  })

  useConversationStream(state.detail?.current_conversation?.id, activityCursorScope, reconcileConversation)

  useEffect(() => {
    if (!state.mediaArtifacts.some(artifact => artifact.status === 'running')) return
    const controller = new AbortController()
    let timer = 0
    const inspect = async () => {
      try {
        const next = await Promise.all(state.mediaArtifacts.map(artifact =>
          artifact.status === 'running'
            ? strategyApi.getMediaUnderstanding(projectId, artifact.id, controller.signal)
            : Promise.resolve(artifact),
        ))
        setState(current => ({
          ...current,
          mediaArtifacts: mergeConversationMedia(current.mediaArtifacts, next, projectId),
        }))
        if (next.some(artifact => artifact.status === 'running')) {
          timer = window.setTimeout(inspect, 1800)
        }
      } catch {
        if (!controller.signal.aborted) timer = window.setTimeout(inspect, 3000)
      }
    }
    timer = window.setTimeout(inspect, 800)
    return () => {
      controller.abort()
      window.clearTimeout(timer)
    }
  }, [projectId, state.mediaArtifacts])

  const perform = useCallback(async (
    name: string,
    action: () => Promise<void>,
    reloadAfter = true,
    errorTarget: 'global' | 'deep_review' = 'global',
  ) => {
    setState(current => ({
      ...current,
      busy: name,
      error: '',
      conversationNotice: name === 'message' || name === 'web-search' ? '' : current.conversationNotice,
      deepReviewError: errorTarget === 'deep_review' ? '' : current.deepReviewError,
    }))
    try {
      await action()
      if (reloadAfter) await load()
      return true
    } catch (error) {
      if (error instanceof BackendApiError && error.code === 'VERSION_CONFLICT') {
        await load().catch(() => undefined)
      }
      setState(current => ({
        ...current,
        error: errorTarget === 'global' ? messageOf(error) : '',
        deepReviewError: errorTarget === 'deep_review' ? messageOf(error) : current.deepReviewError,
      }))
      return false
    } finally {
      setState(current => ({ ...current, busy: '' }))
    }
  }, [load])

  const enqueueBriefPatch = useCallback((
    operations: Array<{ fieldPath: string; value: unknown }>,
    confirmationMode: 'draft' | 'confirm',
    mutationPrefix: string,
  ) => {
    const queuedState = stateRef.current
    const queuedTaskId = queuedState.detail?.current_task?.id ?? ''
    const queuedBriefId = queuedState.brief?.id ?? ''
    const execute = async () => {
      const current = stateRef.current
      const task = current.detail?.current_task
      const draft = current.brief
      if (!task || !draft || task.id !== queuedTaskId || draft.id !== queuedBriefId) {
        throw new Error('Brief 已切换，本次保存未提交。')
      }
      try {
        const brief = await strategyApi.patchBriefFields(
          task.id,
          draft,
          operations,
          createMutationKey(mutationPrefix),
          confirmationMode,
        )
        if (stateRef.current.brief?.id === brief.id) {
          stateRef.current = { ...stateRef.current, brief }
          setState(currentState => currentState.brief?.id === brief.id
            ? { ...currentState, brief }
            : currentState)
        }
        return brief
      } catch (error) {
        if (error instanceof BackendApiError && error.code === 'VERSION_CONFLICT') {
          await load().catch(() => undefined)
        }
        throw error
      }
    }
    const result = briefMutationQueue.current.then(execute, execute)
    briefMutationQueue.current = result.then(() => undefined, () => undefined)
    return result
  }, [load])

  const uploadAndReferenceDocument = async (file: File) => {
    const document = await strategyApi.uploadKnowledgeDocument(projectId, file)
    setState(current => ({ ...current, documents: [document, ...current.documents.filter(value => value.id !== document.id)] }))
    const current = stateRef.current
    if (current.detail?.current_task && current.brief?.status === 'open') {
      const references = Array.from(new Set([...(current.brief.document.reference_ids ?? []), document.id]))
      await enqueueBriefPatch([{ fieldPath: 'reference_ids', value: references }], 'confirm', 'strategy-brief-reference')
    }
    return document
  }

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
    sendMessage: (
      content: string,
      documents: KnowledgeDocument[] = [],
      media: MediaUnderstandingArtifact[] = [],
      requestedPolicy?: MessageRequestedPolicy,
      contextStage: ProjectContextManifest['stage'] = 'intake',
      contextSurface: 'workspace' | 'assistant' = 'workspace',
      excludedSourceIds: string[] = [],
    ) => perform('message', async () => {
      const conversationId = state.detail?.current_conversation?.id
      if (!conversationId) throw new Error('请先开始需求对话。')
      if (documents.some(document => document.status !== 'ready')) {
        throw new Error('请等待附件解析完成后再发送。')
      }
      if (media.some(artifact => artifact.status !== 'ready' && artifact.status !== 'partial')) {
        throw new Error('请等待图片或视频理解完成后再发送。')
      }
      const query = content.trim()
      const effectivePolicy = requestedPolicy?.web_search === 'allowed' && !query
        ? requestedPolicy.reasoning_mode === 'deep' ? { reasoning_mode: 'deep' as const } : undefined
        : requestedPolicy
      const result = await strategyApi.sendMessage(
        conversationId,
        buildConversationMessageCreate(content, documents, media, [], effectivePolicy),
        createMutationKey('strategy-message'),
        contextStage,
        contextSurface,
        excludedSourceIds,
      )
      setState(current => ({
        ...current,
        messages: current.messages.some(message => message.id === result.message.id)
          ? current.messages
          : [...current.messages, result.message],
        pendingAgentTaskId: result.agent_task.id,
        pendingAgentPurpose: 'general',
        conversationNotice: requestedPolicy?.web_search === 'allowed'
          ? '正在联网搜索；搜索完成后会基于返回证据生成本轮回答。'
          : '',
      }))
    }, false),
    autosaveBriefField: async (fieldPath: string, value: unknown) => {
      try {
        await enqueueBriefPatch([{ fieldPath, value }], 'draft', 'strategy-brief-autosave')
        return true
      } catch {
        return false
      }
    },
    patchBriefField: (fieldPath: string, value: unknown) => perform(`brief:${fieldPath}`, async () => {
      await enqueueBriefPatch([{ fieldPath, value }], 'confirm', 'strategy-brief-patch')
    }, false),
    selectBriefProductCandidate: (candidate: BriefProductCandidate) => perform('brief:select-product', async () => {
      const brief = stateRef.current.brief
      if (!brief) throw new Error('Brief 草稿尚未创建。')
      await enqueueBriefPatch(briefProductCandidateOperations(brief.document, candidate), 'confirm', 'strategy-brief-select-product')
    }, false),
    confirmBriefFields: (operations: Array<{ fieldPath: string; value: unknown }>) => perform('confirm-brief-fields', async () => {
      if (!operations.length) return
      await enqueueBriefPatch(operations, 'confirm', 'strategy-brief-confirm-fields')
    }, false),
    confirmBrief: () => perform('confirm-brief', async () => {
      await briefMutationQueue.current
      const task = stateRef.current.detail?.current_task
      const brief = stateRef.current.brief
      if (!task || !brief) throw new Error('Brief 草稿尚未创建。')
      await strategyApi.confirmBrief(
        task.id,
        brief.version,
        createMutationKey('strategy-brief-confirm'),
      )
    }),
    createBriefRevision: () => perform('create-brief-revision', async () => {
      const task = state.detail?.current_task
      const version = state.briefVersion
      if (!task || !version) throw new Error('没有可用于补充修订的冻结 Brief。')
      await strategyApi.createBriefRevisionDraft(
        task.id,
        version.version,
        createMutationKey('strategy-brief-revision'),
      )
    }),
    confirmRequirement: () => perform('confirm-requirement', async () => {
      await briefMutationQueue.current
      const task = stateRef.current.detail?.current_task
      let brief = stateRef.current.brief
      if (!task || !brief || brief.status !== 'open') throw new Error('当前需求不可确认。')
      const operations = requirementConfirmationOperations(brief)
      if (operations.length) {
        brief = await enqueueBriefPatch(operations, 'confirm', 'strategy-requirement-confirm-fields')
      }
      if (!brief.completeness.ready) {
        throw new Error('还缺少产品、目标或核心受众，请继续用一句话补充。')
      }
      await strategyApi.confirmBrief(
        task.id,
        brief.version,
        createMutationKey('strategy-requirement-confirm'),
      )
    }),
    createRequirementViralRemake: async (): Promise<{ intake: CreativeIntakeV4; taskId?: string } | null> => {
      setState(current => ({ ...current, busy: 'viral-remake', error: '' }))
      try {
        const brief = state.briefVersion
        if (!brief) throw new Error('请先确认当前需求。')
        const catalog = await strategyApi.listCreativeBusinesses(projectId)
        const capability = catalog.items.find(item =>
          item.business_code === 'viral_remake' && item.selectable && item.lifecycle === 'active',
        )
        if (!capability) throw new Error('爆款裂变能力当前不可用。')
        const intake = await strategyApi.createRequirementViralIntake(
          projectId,
          brief,
          capability,
          createMutationKey('requirement-viral-intake'),
        )
        if (intake.status !== 'ready') return { intake }
        const task = await strategyApi.createRequirementViralTask(projectId, intake)
        return { intake, taskId: task.id }
      } catch (error) {
        setState(current => ({ ...current, error: messageOf(error) }))
        return null
      } finally {
        setState(current => ({ ...current, busy: '' }))
      }
    },
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
        pendingAgentPurpose: 'general',
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
        pendingAgentPurpose: 'general',
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
        pendingAgentPurpose: 'general',
      }))
    }, false),
    submitStrategy: () => perform(state.reviewPolicy?.mode === 'self_confirmation' ? 'confirm-publish' : 'submit-review', async () => {
      if (!state.draft) throw new Error('策略草稿尚未创建。')
      if (state.reviewPolicy?.mode === 'self_confirmation') {
        await strategyApi.confirmStrategy(
          state.draft,
          createMutationKey('strategy-confirm'),
        )
        approvalMutationKey.current = ''
        return
      }
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
      if (!state.draft?.revision) throw new Error('请先生成一个可分析的策略 Revision。')
      const result = await strategyApi.startStrategyPerspective(state.draft, createMutationKey('strategy-perspective'))
      setState(current => ({
        ...current,
        deepReview: result.analysis,
        deepReviewError: '',
        pendingAgentTaskId: result.agent_task.id,
        pendingAgentPurpose: 'deep_review',
      }))
    }, false, 'deep_review'),
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
      await uploadAndReferenceDocument(file)
    }, false),
	cancelDocumentParse: (document: KnowledgeDocument) => perform(`document:cancel:${document.id}`, async () => {
		await strategyApi.cancelDocumentParse(projectId, document.id)
		const result = await strategyApi.listKnowledgeDocuments(projectId)
		setState(current => ({ ...current, documents: result.items }))
	}, false),
	retryDocumentParse: (document: KnowledgeDocument) => perform(`document:retry:${document.id}`, async () => {
		const updated = await strategyApi.retryDocumentParse(projectId, document.id)
		setState(current => ({
			...current,
			documents: [updated, ...current.documents.filter(value => value.id !== updated.id)],
		}))
	}, false),
	runDocumentVisionFallback: (document: KnowledgeDocument, pageNumbers: number[] = []) => perform(`document:vision:${document.id}`, async () => {
		const updated = await strategyApi.runDocumentVisionFallback(projectId, document.id, pageNumbers)
		setState(current => ({
			...current,
			documents: [updated, ...current.documents.filter(value => value.id !== updated.id)],
		}))
	}, false),
    uploadConversationDocument: async (file: File): Promise<KnowledgeDocument | null> => {
      setState(current => ({ ...current, busy: 'upload-document', error: '' }))
      try {
        return await uploadAndReferenceDocument(file)
      } catch (error) {
        setState(current => ({ ...current, error: messageOf(error) }))
        return null
      } finally {
        setState(current => ({ ...current, busy: '' }))
      }
    },
    uploadConversationMedia: async (file: File): Promise<MediaUnderstandingArtifact | null> => {
      setState(current => ({ ...current, busy: 'upload-media', error: '' }))
      try {
        const artifact = await strategyApi.uploadMediaForUnderstanding(projectId, file)
        setState(current => ({
          ...current,
          mediaArtifacts: [artifact, ...current.mediaArtifacts.filter(value => value.id !== artifact.id)],
        }))
        return artifact
      } catch (error) {
        setState(current => ({ ...current, error: messageOf(error) }))
        return null
      } finally {
        setState(current => ({ ...current, busy: '' }))
      }
    },
    cancelActivity: (activity: TaskActivity) => perform(`activity:cancel:${activity.id}`, async () => {
      if (!activity.actions.includes('cancel')) throw new Error('当前任务不可取消。')
      if (activity.resource_ref.type === 'knowledge_document') {
        await strategyApi.cancelDocumentParse(projectId, activity.resource_ref.id)
        return
      }
      if (activity.resource_ref.type === 'knowledge_research_run') {
        await strategyApi.cancelResearchRun(projectId, activity.resource_ref.id)
        return
      }
      const execution = activity.execution_ref
      if (!execution || execution.type !== 'strategy_agent_task') {
        throw new Error('当前任务没有可控制的执行引用。')
      }
      const expectedVersion = execution.version ?? (await strategyApi.getAgentTask(execution.id)).task.version
      await strategyApi.cancelAgentTask(execution.id, expectedVersion)
    }, false),
    retryActivity: (activity: TaskActivity) => perform(`activity:retry:${activity.id}`, async () => {
      if (!activity.actions.includes('retry')) throw new Error('当前任务不可重试。')
      if (activity.resource_ref.type === 'knowledge_document') {
        const document = await strategyApi.retryDocumentParse(projectId, activity.resource_ref.id)
        setState(current => ({
          ...current,
          documents: [document, ...current.documents.filter(value => value.id !== document.id)],
        }))
        return
      }
      if (activity.resource_ref.type === 'knowledge_research_run') {
        const run = await strategyApi.retryResearchRun(projectId, activity.resource_ref.id)
        setState(current => ({
          ...current,
          researchRun: current.researchRun?.id === run.id ? run : current.researchRun,
          researchRuns: [run, ...current.researchRuns.filter(value => value.id !== run.id)],
        }))
        return
      }
      const draft = state.draft
      if (!draft || activity.kind !== 'strategy_generation') {
        throw new Error('失败的策略草稿不存在。')
      }
      const result = await strategyApi.retryStrategy(draft, createMutationKey('strategy-activity-retry'))
      setState(current => ({
        ...current,
        draft: result.strategy_draft,
        pendingAgentTaskId: result.agent_task.id,
        pendingAgentPurpose: 'general',
      }))
    }, false),
    runResearch: (
      category: 'general' | 'audience' | 'competitor' | 'industry',
      query: string,
      documentIds: string[],
	  contextManifest: ProjectContextManifest | null,
    ) =>
      perform('research', async () => {
		if (!contextManifest || !state.detail?.workspace.id) throw new Error('项目上下文仍在准备，请稍后再开始研究。')
        const researchRun = await strategyApi.runExternalResearch(projectId, {
          category,
          purpose: 'deep_research',
		  run_mode: 'deep',
		  source_ref: { type: 'strategy_workspace', id: state.detail.workspace.id },
		  input_snapshot_ref: `strategy_workspace:${state.detail.workspace.id}:v${contextManifest.workspace_ref.version}`,
		  input_snapshot: contextManifest,
          query,
          document_ids: documentIds,
          disclosed_fields: documentIds.length ? ['query', 'document_content'] : ['query'],
          confirmed: true,
        })
        setState(current => ({
          ...current,
          researchRun,
          researchRuns: [researchRun, ...(current.researchRuns ?? []).filter(value => value.id !== researchRun.id)],
        }))
      }, false),
	cancelResearchRun: (researchRunId: string) =>
		perform(`research-cancel:${researchRunId}`, async () => {
			await strategyApi.cancelResearchRun(projectId, researchRunId)
			const run = await strategyApi.getResearchRun(projectId, researchRunId)
			setState(current => ({
				...current,
				researchRun: run,
				researchRuns: [run, ...current.researchRuns.filter(value => value.id !== run.id)],
			}))
		}, false),
	retryResearchRun: (researchRunId: string) =>
		perform(`research-retry:${researchRunId}`, async () => {
			const run = await strategyApi.retryResearchRun(projectId, researchRunId)
			setState(current => ({
				...current,
				researchRun: run,
				researchRuns: [run, ...current.researchRuns.filter(value => value.id !== run.id)],
			}))
		}, false),
    setResearchArtifactAdoption: (artifactId: string, adopted: boolean) =>
      perform(`research-adoption:${artifactId}`, async () => {
        const task = stateRef.current.detail?.current_task
        const brief = stateRef.current.brief
        if (!task || brief?.status !== 'open') throw new Error('当前 Brief 不可更新。')
        const current = brief.document.reference_ids ?? []
        const references = adopted
          ? Array.from(new Set([...current, artifactId]))
          : current.filter(value => value !== artifactId)
        await enqueueBriefPatch([{ fieldPath: 'reference_ids', value: references }], 'confirm', 'strategy-research-reference')
      }, false),
  }

  return { state, actions }
}

async function loadConversationMedia(projectId: string, messages: Message[], signal?: AbortSignal) {
  const refs = new Map<string, { assetId: string; version: number }>()
  for (const message of messages) {
    for (const block of message.content_blocks ?? []) {
      if (block.type !== 'asset_ref') continue
      refs.set(`${block.asset_id}:${block.asset_version}`, { assetId: block.asset_id, version: block.asset_version })
    }
  }
  const values = await Promise.all([...refs.values()].map(ref =>
    strategyApi.getLatestMediaUnderstanding(projectId, ref.assetId, ref.version, signal).catch(() => null),
  ))
  return values.filter((value): value is MediaUnderstandingArtifact => Boolean(value))
}

export function mergeConversationMedia(
  current: MediaUnderstandingArtifact[],
  restored: MediaUnderstandingArtifact[],
  projectId: string,
) {
  const result = new Map(
    current
      .filter(value => value.project_id === projectId)
      .map(value => [value.id, value]),
  )
  for (const artifact of restored) result.set(artifact.id, artifact)
  return [...result.values()].sort((left, right) => right.created_at.localeCompare(left.created_at))
}

export function activityReconciliationPlan(
  previous: TaskActivitySnapshot | null,
  next: TaskActivitySnapshot,
) {
  const result: {
    agentActivities: TaskActivity[]
    researchRunIds: string[]
    refreshDocuments: boolean
  } = { agentActivities: [], researchRunIds: [], refreshDocuments: false }
  if (!previous) return result
  const previousItems = new Map(previous.items.map(item => [item.id, item]))
  const terminal = new Set(['partially_completed', 'succeeded', 'failed', 'cancelled'])
  for (const item of next.items) {
    const before = previousItems.get(item.id)
    if (item.resource_ref.type === 'knowledge_research_run' && researchActivityChanged(before, item)) {
      result.researchRunIds.push(item.resource_ref.id)
      continue
    }
    if (!terminal.has(item.status) || before?.status === item.status) continue
    if (item.execution_ref?.type === 'strategy_agent_task') {
      result.agentActivities.push(item)
      continue
    }
    if (item.resource_ref.type === 'knowledge_document') {
      result.refreshDocuments = true
      continue
    }
  }
  result.researchRunIds = [...new Set(result.researchRunIds)]
  return result
}

function researchActivityChanged(before: TaskActivity | undefined, next: TaskActivity) {
  if (!before) return true
  return before.status !== next.status ||
    before.phase !== next.phase ||
    before.updated_at !== next.updated_at ||
    before.heartbeat_at !== next.heartbeat_at ||
    before.round?.current !== next.round?.current ||
    before.round?.max !== next.round?.max ||
    before.execution_ref?.version !== next.execution_ref?.version
}
