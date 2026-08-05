import { useCallback, useEffect, useRef, useState } from 'react'
import { BackendApiError } from '../../backend/platform'
import { strategyApi, createMutationKey } from './api'
import { buildConversationMessageCreate, waitForConversationResearch } from './strategyConversationModel'
import { useConversationStream } from './useConversationStream'
import type {
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
  SkillRun,
  StrategyDraft,
  StrategyP0Metrics,
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
  isLoading: boolean
  busy: string
  error: string
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
  if (code === 'MODEL_RATE_LIMITED') {
    return kind === 'strategy.brief.extract'
      ? '文本模型请求暂时受限，请稍后重新发送需求消息。'
      : '文本模型请求频率受限，请稍后再点击“重新生成策略”。'
  }
  if (code === 'MODEL_REQUEST_REJECTED') return '文本模型不支持当前路由参数，请联系管理员检查模型配置后重试。'
  if (code === 'MODEL_OUTPUT_INVALID') return '模型输出未通过策略结构校验，可以重新生成。'
  if (code === 'MODEL_UNAVAILABLE') return '文本模型当前不可用，请检查模型配置后重试。'
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
    isLoading: true,
    busy: '',
    error: '',
    pendingAgentTaskId: '',
    pendingAgentPurpose: '',
  })
  const currentWorkspaceId = useRef('')
  const approvalMutationKey = useRef('')

  const load = useCallback(async (signal?: AbortSignal, requestedWorkspaceId?: string) => {
    const workspaceResult = await strategyApi.listWorkspaces(projectId, signal)
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
      const [readiness, documents, packages, researchRuns, conversationCapabilities, p0Metrics] = await Promise.all([
        strategyApi.getGenerationReadiness(projectId, signal).catch(() => null),
        strategyApi.listKnowledgeDocuments(projectId, signal).then(value => value.items).catch(() => []),
        strategyApi.listStrategyPackages(projectId, signal).then(value => value.items).catch(() => []),
        strategyApi.listResearchRuns(projectId, signal).then(value => value.items).catch(() => []),
        strategyApi.getConversationCapabilities(signal).catch(() => null),
        strategyApi.getP0Metrics(projectId, 30, signal).catch(() => null),
      ])
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
        researchRun: researchRuns[0] ?? null,
        published: packages.find(value => value.status === 'published') ?? null,
        isLoading: false,
      }))
      return
    }

    currentWorkspaceId.current = targetId
    const detail = await strategyApi.getWorkspace(targetId, signal)
    if (detail.workspace.project_id !== projectId) throw new Error('策略工作区不属于当前 Project。')
    rememberWorkspace(projectId, detail.workspace.id)
    const conversation = detail.current_conversation
    const task = detail.current_task

    const [messageResult, brief, packages, readiness, documents, memory, skillRuns, researchRuns, conversationCapabilities, p0Metrics] = await Promise.all([
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
      strategyApi.getConversationCapabilities(signal).catch(() => null),
      strategyApi.getP0Metrics(projectId, 30, signal).catch(() => null),
    ])
    const mediaArtifacts = await loadConversationMedia(projectId, messageResult, signal)

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
    if (task?.current_strategy_id) {
      draft = await strategyApi.getStrategy(task.current_strategy_id, signal)
      if (draft.status === 'failed' && task.current_agent_task_id) {
        const inspection = await strategyApi.getAgentTask(task.current_agent_task_id, signal).catch(() => null)
        const problem = inspection?.task.error ?? inspection?.job?.error
        agentFailure = agentFailureMessage(problem?.code, problem?.message, inspection?.task.kind)
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
        if (analysis?.status === 'failed' && analysis.agent_task_id) {
          const inspection = await strategyApi.getAgentTask(analysis.agent_task_id, signal).catch(() => null)
          const problem = inspection?.task.error ?? inspection?.job?.error
          deepReviewFailure = agentFailureMessage(problem?.code, problem?.message)
        }
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
    const agentPurpose = state.pendingAgentPurpose
    if (!agentTaskId) return
    const controller = new AbortController()
    let timer = 0
    const inspect = async () => {
      try {
        const inspection = await strategyApi.getAgentTask(agentTaskId, controller.signal)
        const task = inspection.task
        if (task.status === 'failed' || task.status === 'cancelled') {
          const problem = task.error ?? inspection.job?.error
          const failureMessage = agentFailureMessage(problem?.code, problem?.message, task.kind)
          await reload()
          setState(current => ({
            ...current,
            pendingAgentTaskId: '',
            pendingAgentPurpose: '',
            error: agentPurpose === 'deep_review' ? '' : failureMessage,
            deepReviewError: agentPurpose === 'deep_review' ? failureMessage : current.deepReviewError,
          }))
          return
        }
        if (task.status === 'succeeded') {
          setState(current => ({
            ...current,
            pendingAgentTaskId: '',
            pendingAgentPurpose: '',
            deepReviewError: agentPurpose === 'deep_review' ? '' : current.deepReviewError,
          }))
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
  }, [reload, state.pendingAgentPurpose, state.pendingAgentTaskId])

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
  }, [projectId, state.researchRun])

  useEffect(() => {
    if (!state.documents.some(document => document.status === 'parse_queued' || document.status === 'parsing')) return
    const controller = new AbortController()
    let timer = 0
    const inspect = async () => {
      try {
        const documents = await strategyApi.listKnowledgeDocuments(projectId, controller.signal)
        setState(current => ({ ...current, documents: documents.items }))
        if (documents.items.some(document => document.status === 'parse_queued' || document.status === 'parsing')) {
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
  }, [projectId, state.documents])

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

  const uploadAndReferenceDocument = async (file: File) => {
    const document = await strategyApi.uploadKnowledgeDocument(projectId, file)
    setState(current => ({ ...current, documents: [document, ...current.documents.filter(value => value.id !== document.id)] }))
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
    ) => perform(requestedPolicy?.web_search === 'allowed' ? 'web-search' : 'message', async () => {
      const conversationId = state.detail?.current_conversation?.id
      if (!conversationId) throw new Error('请先开始需求对话。')
      if (documents.some(document => document.status !== 'ready')) {
        throw new Error('请等待附件解析完成后再发送。')
      }
      if (media.some(artifact => artifact.status !== 'ready' && artifact.status !== 'partial')) {
        throw new Error('请等待图片或视频理解完成后再发送。')
      }
      let researchArtifacts: ResearchRun['artifacts'] = []
      if (requestedPolicy?.web_search === 'allowed') {
        const query = content.trim()
        if (!query) throw new Error('联网查证需要先输入一个明确问题。')
        const initialRun = await strategyApi.runExternalResearch(projectId, {
          category: 'general',
          query,
          document_ids: [],
          disclosed_fields: ['query'],
          confirmed: true,
        })
        setState(current => ({ ...current, researchRun: initialRun }))
        const completedRun = await waitForConversationResearch(
          initialRun,
          researchRunId => strategyApi.getResearchRun(projectId, researchRunId),
        )
        setState(current => ({ ...current, researchRun: completedRun }))
        if (completedRun.status !== 'succeeded') {
          throw new Error(completedRun.error_message || '联网查证未完成；原消息没有发送，请关闭联网查证后重试。')
        }
        if (!completedRun.artifacts.length) {
          throw new Error('联网查证没有返回可引用的证据；原消息没有发送。')
        }
        researchArtifacts = completedRun.artifacts
      }
      const result = await strategyApi.sendMessage(
        conversationId,
        buildConversationMessageCreate(content, documents, media, researchArtifacts, requestedPolicy),
        createMutationKey('strategy-message'),
      )
      setState(current => ({
        ...current,
        messages: current.messages.some(message => message.id === result.message.id)
          ? current.messages
          : [...current.messages, result.message],
        pendingAgentTaskId: result.agent_task.id,
        pendingAgentPurpose: 'general',
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
    confirmRequirement: () => perform('confirm-requirement', async () => {
      const task = state.detail?.current_task
      let brief = state.brief
      if (!task || !brief || brief.status !== 'open') throw new Error('当前需求不可确认。')
      const corePaths = ['product.name', 'campaign.objective', 'audience.primary']
      const operations = corePaths.flatMap(fieldPath => {
        const value = fieldPath === 'product.name'
          ? brief?.document.product?.name
          : fieldPath === 'campaign.objective'
            ? brief?.document.campaign.objective
            : brief?.document.audience.primary
        if (!value?.trim() || brief?.field_states[fieldPath]?.confirmation === 'confirmed') return []
        return [{ fieldPath, value }]
      })
      if (operations.length) {
        brief = await strategyApi.patchBriefFields(
          task.id,
          brief,
          operations,
          createMutationKey('strategy-requirement-confirm-fields'),
        )
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
    runResearch: (
      category: 'general' | 'audience' | 'competitor' | 'industry',
      query: string,
      documentIds: string[],
    ) =>
      perform('research', async () => {
        const researchRun = await strategyApi.runExternalResearch(projectId, {
          category,
          query,
          document_ids: documentIds,
          disclosed_fields: documentIds.length ? ['query', 'document_content'] : ['query'],
          confirmed: true,
        })
        setState(current => ({ ...current, researchRun }))
      }, false),
    setResearchArtifactAdoption: (artifactId: string, adopted: boolean) =>
      perform(`research-adoption:${artifactId}`, async () => {
        const task = state.detail?.current_task
        const brief = state.brief
        if (!task || brief?.status !== 'open') throw new Error('当前 Brief 不可更新。')
        const current = brief.document.reference_ids ?? []
        const references = adopted
          ? Array.from(new Set([...current, artifactId]))
          : current.filter(value => value !== artifactId)
        const updated = await strategyApi.patchBriefField(
          task.id,
          brief,
          'reference_ids',
          references,
          createMutationKey('strategy-research-reference'),
        )
        setState(value => ({ ...value, brief: updated }))
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
