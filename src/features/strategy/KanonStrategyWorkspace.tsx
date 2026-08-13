import {
  AlertCircle,
  Archive,
  BadgeCheck,
  BookOpen,
  Check,
  ChevronRight,
  CircleCheck,
  Download,
  ExternalLink,
  FileText,
  LoaderCircle,
  MessageSquare,
  Plus,
  RefreshCw,
  RotateCcw,
  Search,
  ShieldCheck,
  Sparkles,
  Upload,
} from 'lucide-react'
import { useEffect, useLayoutEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { useProject } from '../../context/ProjectContext'
import { useAuth } from '../../context/AuthContext'
import { AssistantProposalList, ProjectAssistantDock } from './assistant/ProjectAssistantDock'
import { useAssistantProposals } from './assistant/useAssistantProposals'
import { useProjectContextManifest } from './assistant/useProjectContextManifest'
import { useResearchAdoptionProposals } from './research/useResearchAdoptionProposals'
import { CreativeHandoffWorkspace } from './CreativeTaskPlanner'
import { StrategyConversationPane } from './StrategyConversationPane'
import { MaterialsDrawer } from './documents/MaterialsDrawer'
import { fullStrategyFieldLabels, getFullStrategyDraftReadiness, getFullStrategyReadiness } from './strategyBriefReadiness'
import { useStrategyWorkspace } from './useStrategyWorkspace'
import { StageRail, type StageRailItem } from './workspace/StageRail'
import { StrategyWorkspaceShell } from './workspace/StrategyWorkspaceShell'
import { useStrategyWorkspaceRoute } from './workspace/WorkspaceProvider'
import { WorkspaceTopbar } from './workspace/WorkspaceTopbar'
import type { StrategyPanel, StrategyStage } from './workspace/workspaceRoute'
import {
  clearWorkspaceSessionValue,
  readWorkspaceSessionValue,
  stageScrollSessionKey,
  workspaceSessionKey,
  writeWorkspaceSessionValue,
} from './workspace/workspaceSessionState'
import type {
  BriefDraft,
  BriefProductCandidate,
  BriefVersion,
  CreativeStrategy,
  DraftRevision,
  KnowledgeDocument,
  Review,
  StrategyDocument,
  StrategyDraft,
  TaskActivity,
  ActivityConnection,
	ArtifactProposal,
	BriefPatchOperation,
} from './types'

type Props = {
  onOpenCreative: (navId: string, view: string, contextId: string) => void
}

export function KanonStrategyWorkspace({
  onOpenCreative,
}: Props) {
  const { currentProject } = useProject()
  const { session } = useAuth()
  const workspaceSessionOwner = `${session.organization?.id ?? 'organization-unknown'}:${session.user?.id ?? 'user-unknown'}:${currentProject.id}`
  return <KanonStrategyWorkspaceSession
    key={workspaceSessionOwner}
    onOpenCreative={onOpenCreative}
  />
}

function KanonStrategyWorkspaceSession({
  onOpenCreative,
}: Props) {
  const { currentProject } = useProject()
  const { session } = useAuth()
  const workspaceRoute = useStrategyWorkspaceRoute()
  const { location, workspaceId } = workspaceRoute
  const activeStage = location.stage
  const workspaceSessionOwner = `${session.organization?.id ?? 'organization-unknown'}:${session.user?.id ?? 'user-unknown'}:${currentProject.id}`
  const { state, actions } = useStrategyWorkspace(currentProject.id, workspaceId, workspaceSessionOwner)
  const mainRef = useRef<HTMLElement>(null)
  const stageHeadingRef = useRef<HTMLHeadingElement>(null)
  const assistantStorageKey = `cookies.strategy.assistant-open.v1:${workspaceSessionOwner}`
  const assistantExpandedStorageKey = `cookies.strategy.assistant-expanded.v1:${workspaceSessionOwner}`
  const assistantWidthStorageKey = `cookies.strategy.assistant-width.v1:${workspaceSessionOwner}`
  const [assistantOpen, setAssistantOpen] = useState(() => location.panel === 'assistant' || (!location.panel && readStoredAssistantState(assistantStorageKey)))
  const [assistantExpanded, setAssistantExpanded] = useState(() => readStoredAssistantState(assistantExpandedStorageKey))
  const [assistantWidth, setAssistantWidth] = useState(() => readStoredAssistantWidth(assistantWidthStorageKey))
  const [assistantResizing, setAssistantResizing] = useState(false)
  const assistantResizeStart = useRef({ clientX: 0, width: 356 })
  const assistantWorkspaceId = state.detail?.workspace.id ?? workspaceId ?? ''
  const [assistantExcludedSourceIds, setAssistantExcludedSourceIds] = useState<string[]>([])
  const workspaceSessionId = assistantWorkspaceId || 'workspace-pending'
  const workspaceRenderReady = Boolean(state.detail?.current_task)
  const assistantContextRevisionKey = [
    state.brief?.version ?? 0,
    state.briefVersion?.version ?? 0,
    state.draft?.current_revision ?? 0,
    state.memory?.version ?? 0,
    state.documents.length,
  ].join(':')
  const { manifest: assistantManifest, error: assistantContextError } = useProjectContextManifest(
    assistantWorkspaceId,
    activeStage,
    assistantContextRevisionKey,
  )
  const activeAssistantExcludedSourceIds = useMemo(() => {
    const available = new Set(assistantManifest?.selected_source_refs.map(ref => ref.id) ?? [])
    return assistantExcludedSourceIds.filter(id => available.has(id))
  }, [assistantExcludedSourceIds, assistantManifest])
  const assistantSeenStorageKey = `cookies.strategy.assistant-seen.v1:${workspaceSessionOwner}:${assistantWorkspaceId || 'workspace-pending'}`
  const latestAssistantMessageId = [...state.messages].reverse().find(message => message.role === 'assistant')?.id ?? ''
  const assistantProposals = useAssistantProposals(
    assistantWorkspaceId,
    `${assistantContextRevisionKey}:${latestAssistantMessageId}`,
    actions.reload,
  )
	const researchAdoption = useResearchAdoptionProposals(
		assistantWorkspaceId,
		state.researchRun?.id ?? '',
		`${assistantContextRevisionKey}:${state.researchRun?.updated_at ?? ''}`,
		actions.reload,
	)
  const [lastSeenAssistantMessageId, setLastSeenAssistantMessageId] = useState(() => readStoredString(assistantSeenStorageKey))
  const assistantUnread = Boolean(!assistantOpen && latestAssistantMessageId && latestAssistantMessageId !== lastSeenAssistantMessageId)

  useLayoutEffect(() => {
    const main = mainRef.current
    if (!main) return
    const storageKey = stageScrollSessionKey(workspaceSessionOwner, workspaceSessionId, activeStage)
    const storedScrollTop = readWorkspaceSessionValue<number>(storageKey)
    let lastScrollTop = typeof storedScrollTop === 'number' && Number.isFinite(storedScrollTop)
      ? Math.max(0, storedScrollTop)
      : 0
    main.scrollTo({ top: lastScrollTop })
    const trackScroll = () => { lastScrollTop = main.scrollTop }
    const rememberScroll = () => writeWorkspaceSessionValue(storageKey, lastScrollTop)
    main.addEventListener('scroll', trackScroll, { passive: true })
    window.addEventListener('pagehide', rememberScroll)
    const frame = window.requestAnimationFrame(() => {
      main.scrollTo({ top: lastScrollTop })
      stageHeadingRef.current?.focus({ preventScroll: true })
    })
    return () => {
      window.cancelAnimationFrame(frame)
      rememberScroll()
      main.removeEventListener('scroll', trackScroll)
      window.removeEventListener('pagehide', rememberScroll)
    }
  }, [activeStage, workspaceRenderReady, workspaceSessionId, workspaceSessionOwner])

  useEffect(() => {
    if (location.panel === 'assistant') setAssistantOpen(true)
    else if (location.panel) setAssistantOpen(false)
  }, [location.panel])

  useEffect(() => {
    setAssistantExcludedSourceIds([])
  }, [assistantWorkspaceId])

  useEffect(() => {
    storeAssistantState(assistantStorageKey, assistantOpen)
  }, [assistantOpen, assistantStorageKey])

  useEffect(() => {
    storeAssistantState(assistantExpandedStorageKey, assistantExpanded)
  }, [assistantExpanded, assistantExpandedStorageKey])

  useEffect(() => {
    if (!assistantOpen || !assistantExpanded) return
    const exitExpandedMode = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setAssistantExpanded(false)
    }
    window.addEventListener('keydown', exitExpandedMode)
    return () => window.removeEventListener('keydown', exitExpandedMode)
  }, [assistantExpanded, assistantOpen])

  useEffect(() => {
    setLastSeenAssistantMessageId(readStoredString(assistantSeenStorageKey))
  }, [assistantSeenStorageKey])

  useEffect(() => {
    if (!assistantOpen || !latestAssistantMessageId) return
    setLastSeenAssistantMessageId(latestAssistantMessageId)
    storeString(assistantSeenStorageKey, latestAssistantMessageId)
  }, [assistantOpen, assistantSeenStorageKey, latestAssistantMessageId])

  useEffect(() => {
    if (!assistantResizing) return
    const move = (event: PointerEvent) => {
      const delta = assistantResizeStart.current.clientX - event.clientX
      const maxWidth = Math.max(360, Math.min(560, window.innerWidth - 720))
      setAssistantWidth(Math.max(320, Math.min(maxWidth, assistantResizeStart.current.width + delta)))
    }
    const stop = () => setAssistantResizing(false)
    window.addEventListener('pointermove', move)
    window.addEventListener('pointerup', stop, { once: true })
    return () => {
      window.removeEventListener('pointermove', move)
      window.removeEventListener('pointerup', stop)
    }
  }, [assistantResizing])

  useEffect(() => {
    if (assistantResizing) return
    storeAssistantWidth(assistantWidthStorageKey, assistantWidth)
  }, [assistantResizing, assistantWidth, assistantWidthStorageKey])

  useEffect(() => {
    if (!workspaceId && state.detail?.workspace.id) {
      workspaceRoute.navigateWorkspace(state.detail.workspace.id, location, true)
    }
  }, [location, state.detail?.workspace.id, workspaceId, workspaceRoute])

  if (state.isLoading) {
    return <div className="kanon-strategy-state" role="status">
      <LoaderCircle className="spin" size={24}/>
      <h2>正在恢复策略工作区</h2>
      <p>读取真实对话、Brief、策略版本和评审状态。</p>
    </div>
  }

  if (!state.detail) {
    return <div className="kanon-strategy-state">
      <MessageSquare size={28}/>
      <h2>创建当前 Project 的策略工作区</h2>
      <p>工作区会持久化同一条对话、Brief、策略修订和评审结果。</p>
      {state.error ? <div className="kanon-strategy-alert" role="alert"><AlertCircle size={15}/>{state.error}</div> : null}
      <button className="primary-button" disabled={Boolean(state.busy)} onClick={() => void actions.createWorkspace()}>
        <Plus size={16}/>{state.busy === 'workspace' ? '创建中…' : '创建主策略工作区'}
      </button>
    </div>
  }

  if (!state.detail.current_conversation || !state.detail.current_task) {
    const bootstrapItems: StageRailItem[] = [
      { stage: 'intake', status: 'current', detail: '等待开始对话' },
      { stage: 'brief', status: 'blocked', detail: '等待需求输入' },
      { stage: 'strategy', status: 'blocked', detail: '等待 Brief' },
      { stage: 'review', status: 'blocked', detail: '等待策略' },
      { stage: 'handoff', status: 'blocked', detail: '等待确认' },
    ]
    return <StrategyWorkspaceShell
      assistant={null}
      assistantOpen={false}
      topbar={<WorkspaceTopbar
        activeWorkspaceId={state.detail.workspace.id}
        activityDisabled
        assistantDisabled
        assistantOpen={false}
        backgroundTaskCount={0}
        busy={Boolean(state.busy)}
        onAssistantToggle={() => undefined}
        onOpenActivity={() => undefined}
        onRefresh={() => void actions.reload()}
        onWorkspaceChange={nextWorkspaceId => workspaceRoute.navigateWorkspace(nextWorkspaceId, { stage: activeStage })}
        workspaceName={state.detail.workspace.name}
        workspaces={state.workspaces}
      />}
      rail={<StageRail
        activeStage={activeStage}
        items={bootstrapItems}
        onSelect={stage => workspaceRoute.navigate({ stage })}
      />}
    >
      <div className="kanon-strategy-root">
        <div className="kanon-strategy-state">
          <Sparkles size={28}/>
          <h2>开始需求对话</h2>
          <p>系统将创建持久对话和 Brief 草稿；刷新页面后可以继续。</p>
          {state.error ? <div className="kanon-strategy-alert" role="alert"><AlertCircle size={15}/>{state.error}</div> : null}
          <button className="primary-button" disabled={Boolean(state.busy)} onClick={() => void actions.startConversation()}>
            <MessageSquare size={16}/>{state.busy === 'conversation' ? '正在启动…' : '开始策略梳理'}
          </button>
        </div>
      </div>
    </StrategyWorkspaceShell>
  }

  const lifecycleLocked = Boolean(state.detail.current_task.discarded_at || state.draft?.archived_at)
  const hasEvidence = Boolean(
    state.documents.length ||
    state.brief?.document.reference_ids?.length ||
    state.researchRun?.artifacts.length,
  )
  const backgroundTaskCount = state.activities.filter(activity =>
    ['queued', 'running', 'waiting_user', 'stalled'].includes(activity.status),
  ).length + state.mediaArtifacts.filter(artifact => artifact.status === 'running').length
  const stageItems: StageRailItem[] = [
    {
      stage: 'intake',
      status: state.messages.some(message => message.role === 'assistant') ? 'complete' : activeStage === 'intake' ? 'current' : 'available',
      detail: state.messages.length ? `${state.messages.length} 条消息` : '等待需求输入',
    },
    {
      stage: 'brief',
      status: state.brief?.status === 'confirmed' ? 'complete' : state.brief?.completeness.ready ? 'available' : 'blocked',
      detail: state.brief?.status === 'confirmed'
        ? `已确认 v${state.briefVersion?.version ?? state.brief.version}`
        : state.brief?.completeness.ready ? '可以确认' : `${state.brief?.completeness.blockers.length ?? 0} 个待补项`,
    },
    {
      stage: 'strategy',
      status: state.draft?.current_revision ? 'complete' : state.brief?.status === 'confirmed' ? 'available' : 'blocked',
      detail: state.draft?.current_revision
        ? `修订 ${state.draft.current_revision}`
        : state.brief?.status === 'confirmed' ? '可以生成' : '等待 Brief',
    },
    {
      stage: 'review',
      status: state.review?.status === 'approved' ? 'complete' : state.draft?.current_revision ? 'available' : 'blocked',
      detail: state.review?.status === 'approved' ? '已确认' : state.review?.status === 'open' ? '等待决定' : '尚未提交',
    },
    {
      stage: 'handoff',
      status: state.published ? 'complete' : state.review?.status === 'approved' ? 'available' : 'blocked',
      detail: state.published ? `Package v${state.published.version}` : '等待确认',
    },
  ]
  const contextLabels = [
    state.brief?.status === 'confirmed' ? `Brief v${state.briefVersion?.version ?? state.brief.version}` : 'Brief 草稿',
    state.draft?.current_revision ? `策略修订 ${state.draft.current_revision}` : '策略未生成',
    hasEvidence ? `${state.documents.length + (state.researchRun?.artifacts.length ?? 0)} 份资料/研究` : '暂无引用资料',
  ]
  const closeSupportingPanel = () => workspaceRoute.navigate({ stage: activeStage })
  const supportingPanel = location.panel === 'research' ? <WorkspaceSupportingPanel title="研究" detail="研究与 Brief 保持同一工作区上下文" disabled={lifecycleLocked} onClose={closeSupportingPanel}>
    <ResearchPane
      brief={state.brief}
      busy={state.busy}
	  contextError={assistantContextError}
	  contextReady={Boolean(assistantManifest)}
      documents={state.documents}
	  draft={state.draft}
      researchRun={state.researchRun}
      key={workspaceSessionId}
	  proposals={researchAdoption.items}
	  proposalBusyId={researchAdoption.busyId}
	  proposalError={researchAdoption.error}
	  onApplyProposal={researchAdoption.apply}
	  onIgnoreProposal={researchAdoption.ignore}
	  onRemapProposal={researchAdoption.remap}
	  onCancelResearch={actions.cancelResearchRun}
	  onRetryResearch={actions.retryResearchRun}
	  onResearch={(category, query, documentIds) => actions.runResearch(category, query, documentIds, assistantManifest)}
      onUpload={actions.uploadDocument}
      sessionProjectId={workspaceSessionOwner}
      sessionWorkspaceId={workspaceSessionId}
    />
  </WorkspaceSupportingPanel> : location.panel === 'history' ? <WorkspaceSupportingPanel title="版本与变更" detail="只读查看当前工作区的修订事实" disabled={lifecycleLocked} onClose={closeSupportingPanel}>
    <ChangeLogPane comments={state.comments} revisions={state.revisions} review={state.review}/>
  </WorkspaceSupportingPanel> : location.panel === 'materials' ? <WorkspaceSupportingPanel title="项目资料" detail="当前 Brief 和策略可引用的来源" disabled={lifecycleLocked} onClose={closeSupportingPanel}>
	<MaterialsDrawer
		busy={state.busy}
		documents={state.documents}
		initialDocumentId={location.resource}
		onCancel={actions.cancelDocumentParse}
		onRetry={actions.retryDocumentParse}
		onVisualFallback={actions.runDocumentVisionFallback}
		onUpload={actions.uploadDocument}
		projectId={currentProject.id}
		referenceIds={state.brief?.document.reference_ids ?? []}
		researchArtifacts={state.researchRun?.artifacts ?? []}
	/>
  </WorkspaceSupportingPanel> : location.panel === 'activity' ? <WorkspaceSupportingPanel title="后台任务" detail="切换阶段不会取消这些任务" onClose={closeSupportingPanel}>
    <WorkspaceActivityPanel
      activities={state.activities}
      busy={state.busy}
      connection={state.activityConnection}
      error={state.activityError}
      onCancel={actions.cancelActivity}
      onOpen={activity => {
        setAssistantOpen(false)
        if (activity.resource_ref.type === 'knowledge_document') {
          workspaceRoute.navigate({ stage: activeStage, panel: 'materials', resource: activity.resource_ref.id })
          return
        }
        if (activity.resource_ref.type === 'knowledge_research_run') {
          workspaceRoute.navigate({ stage: activeStage === 'intake' ? 'brief' : activeStage, panel: 'research', resource: activity.resource_ref.id })
          return
        }
        workspaceRoute.navigate({
          stage: activity.kind === 'assistant' ? 'intake' : activity.kind === 'deep_review' && activity.resource_ref.type === 'strategy_review' ? 'review' : 'strategy',
        })
      }}
      onRetry={actions.retryActivity}
    />
  </WorkspaceSupportingPanel> : activeStage === 'review' ? <SummaryRail
    brief={state.brief}
    briefVersion={state.briefVersion?.version}
    draft={state.draft}
    publishedVersion={state.published?.version}
    review={state.review}
    workspaceName={state.detail.workspace.name}
  /> : null

  const assistant = <ProjectAssistantDock
    brief={state.brief}
    contextLabels={contextLabels}
    contextError={assistantContextError}
    disabled={lifecycleLocked}
    excludedSourceIds={activeAssistantExcludedSourceIds}
    expanded={assistantExpanded}
    manifest={assistantManifest}
    messages={state.messages}
    onClose={() => {
      setAssistantOpen(false)
      if (location.panel === 'assistant') workspaceRoute.navigate({ stage: activeStage }, true)
    }}
    onExpandedChange={setAssistantExpanded}
    onSend={async content => {
      const accepted = await actions.sendMessage(
        content, [], [], undefined, activeStage, 'assistant', activeAssistantExcludedSourceIds,
      )
      if (accepted) setAssistantExcludedSourceIds([])
      return accepted
    }}
    onToggleSource={sourceId => setAssistantExcludedSourceIds(current => current.includes(sourceId)
      ? current.filter(id => id !== sourceId)
      : [...current, sourceId])}
    onResizeStart={clientX => {
      assistantResizeStart.current = { clientX, width: assistantWidth }
      setAssistantResizing(true)
    }}
    onApplyProposal={assistantProposals.apply}
    onIgnoreProposal={assistantProposals.ignore}
    pending={Boolean(state.pendingAgentTaskId)}
    proposalBusyId={assistantProposals.busyId}
    proposalError={assistantProposals.error}
    proposals={assistantProposals.proposals}
    stage={activeStage}
  />

  return <StrategyWorkspaceShell
    assistant={assistant}
    assistantExpanded={assistantExpanded}
    assistantOpen={assistantOpen}
    assistantWidth={assistantWidth}
    topbar={<WorkspaceTopbar
      activeWorkspaceId={state.detail.workspace.id}
      assistantOpen={assistantOpen}
      assistantUnread={assistantUnread}
      backgroundTaskCount={backgroundTaskCount}
      busy={Boolean(state.busy)}
      onAssistantToggle={() => {
        const nextOpen = !assistantOpen
        setAssistantOpen(nextOpen)
        if (nextOpen && location.panel && location.panel !== 'assistant') {
          workspaceRoute.navigate({ stage: activeStage }, true)
        }
        if (!nextOpen && location.panel === 'assistant') workspaceRoute.navigate({ stage: activeStage }, true)
      }}
      onOpenActivity={() => {
        setAssistantOpen(false)
        workspaceRoute.navigate({
          stage: activeStage,
          panel: location.panel === 'activity' ? undefined : 'activity',
        })
      }}
      onRefresh={() => void Promise.all([actions.reload(), assistantProposals.reload()])}
      onWorkspaceChange={nextWorkspaceId => workspaceRoute.navigateWorkspace(nextWorkspaceId, { stage: activeStage })}
      workspaceName={state.detail.workspace.name}
      workspaces={state.workspaces}
    />}
    rail={<StageRail
      activeStage={activeStage}
      items={stageItems}
      onSelect={stage => workspaceRoute.navigate({
        stage,
        panel: location.panel === 'assistant' ? 'assistant' : undefined,
      })}
    />}
  >
    <div className={`kanon-strategy-root strategy-stage-${activeStage}${activeStage === 'intake' ? ' conversation-active' : ''}`}>
      {state.error ? <div className="kanon-strategy-alert" role="alert">
        <AlertCircle size={15}/><span>{state.error}</span>
        <button aria-label="重新加载策略工作区" onClick={() => void actions.reload()}><RefreshCw size={14}/></button>
      </div> : null}
      {lifecycleLocked ? <div className="kanon-lifecycle-banner" role="status"><Archive size={15}/><span><b>{state.detail.current_task.discarded_at ? '任务已废弃' : '策略已归档'}</b>当前工作区为只读，完整对话、Brief、策略版本和评审记录均已保留。请从“策略任务 → 已归档”恢复后继续操作。</span></div> : null}
      <div className={`kanon-strategy-workspace ${supportingPanel ? 'with-supporting-panel' : 'rails-none'}`}>
        <main className="kanon-strategy-main" ref={mainRef}>
          <h2 ref={stageHeadingRef} className="strategy-stage-focus-heading" tabIndex={-1}>
            {activeStage === 'intake' ? '理解需求' : activeStage === 'brief' ? '确认 Brief' : activeStage === 'strategy' ? '制定策略' : activeStage === 'review' ? '确认与评审' : '创意交接'}
          </h2>
          {activeStage !== 'intake' ? <StageContextToolbar
            activePanel={location.panel}
            stage={activeStage}
            onPanel={panel => {
              setAssistantOpen(false)
              workspaceRoute.navigate({
                stage: activeStage,
                panel: location.panel === panel ? undefined : panel,
              })
            }}
          /> : null}
          <fieldset className="kanon-lifecycle-lock" disabled={lifecycleLocked}>
        {activeStage === 'intake' ? <StrategyConversationPane
          brief={state.brief}
          briefVersion={state.briefVersion}
          busy={state.busy}
          conversationCapabilities={state.conversationCapabilities}
          draftStorageKey={workspaceSessionKey(workspaceSessionOwner, workspaceSessionId, 'intake-composer')}
          documents={state.documents}
          key={workspaceSessionId}
          mediaArtifacts={state.mediaArtifacts}
          messages={state.messages}
          notice={state.conversationNotice}
          researchRuns={state.researchRuns}
          onConfirmRequirement={actions.confirmRequirement}
          onOpenBrief={() => workspaceRoute.navigate({ stage: 'brief' })}
          onOpenFullStrategy={() => workspaceRoute.navigate({ stage: 'strategy' })}
          onReadyViralRemake={taskId => onOpenCreative('video', '效果广告', taskId)}
          onSend={actions.sendMessage}
          onStartViralRemake={actions.createRequirementViralRemake}
          onUploadDocument={actions.uploadConversationDocument}
          onUploadMedia={actions.uploadConversationMedia}
          pending={Boolean(state.pendingAgentTaskId)}
        /> : null}
        {activeStage === 'brief' ? <BriefPane
          assistantBusyId={assistantProposals.busyId}
          assistantError={assistantProposals.error}
          assistantProposals={assistantProposals.proposals}
          brief={state.brief}
          busy={state.busy}
          onApplyProposal={assistantProposals.apply}
          onAutosaveField={actions.autosaveBriefField}
          onConfirm={actions.confirmBrief}
          onConfirmFields={actions.confirmBriefFields}
          onField={actions.patchBriefField}
          onIgnoreProposal={assistantProposals.ignore}
          onOpenAssistant={() => setAssistantOpen(true)}
          onSelectProduct={actions.selectBriefProductCandidate}
          sessionProjectId={workspaceSessionOwner}
          sessionWorkspaceId={workspaceSessionId}
        /> : null}
        {activeStage === 'strategy' ? <StrategyPane
          briefVersion={state.briefVersion}
          busy={state.busy}
          deepReview={state.deepReview}
          deepReviewError={state.deepReviewError}
          draft={state.draft}
          key={workspaceSessionId}
          reviewPolicy={state.reviewPolicy}
          readiness={state.readiness}
          sessionProjectId={workspaceSessionOwner}
          sessionWorkspaceId={workspaceSessionId}
          probe={state.probe}
          onCreateBriefRevision={async () => {
            const created = await actions.createBriefRevision()
            if (created) {
              workspaceRoute.navigate({ stage: 'brief' })
            }
            return created
          }}
          onGenerate={actions.generateStrategy}
          onDeepReview={actions.startDeepReview}
          onPatch={actions.patchStrategySection}
          onProbe={actions.probeGeneration}
          onRetry={actions.retryStrategy}
          onRevise={actions.reviseStrategy}
          onSubmit={actions.submitStrategy}
          pending={Boolean(state.pendingAgentTaskId)}
          /> : null}
        {activeStage === 'handoff' ? <CreativeHandoffWorkspace
          briefVersion={state.briefVersion}
          draft={state.draft}
          draftStorageKey={workspaceSessionKey(workspaceSessionOwner, workspaceSessionId, 'handoff-answers')}
          onOpenCreative={onOpenCreative}
          onOpenStrategy={() => workspaceRoute.navigate({ stage: 'strategy' })}
          projectId={currentProject.id}
        /> : null}
        {activeStage === 'review' ? <ReviewPane
          busy={state.busy}
          comments={state.comments}
          deepReview={state.deepReview}
          deepReviewError={state.deepReviewError}
          draft={state.draft}
          draftStorageKey={workspaceSessionKey(workspaceSessionOwner, workspaceSessionId, 'review-draft', state.review?.id ?? `revision-${state.draft?.current_revision ?? 0}`)}
          key={`${workspaceSessionId}:${state.review?.id ?? state.draft?.current_revision ?? 0}`}
          review={state.review}
          revisions={state.revisions}
          onAddComment={actions.addComment}
          onApprove={actions.approveReview}
          onDeepReview={actions.startDeepReview}
          onReturn={actions.returnReview}
          onOpenCreative={() => workspaceRoute.navigate({ stage: 'handoff' })}
          onConfirm={actions.submitStrategy}
          reviewPolicy={state.reviewPolicy}
        /> : null}
          </fieldset>
        </main>
        {supportingPanel}
      </div>
    </div>
  </StrategyWorkspaceShell>
}

function WorkspaceSupportingPanel({ children, detail, disabled = false, onClose, title }: {
  children: ReactNode
  detail: string
  disabled?: boolean
  onClose: () => void
  title: string
}) {
  return <aside className="strategy-supporting-panel" aria-label={title}>
    <header><div><b>{title}</b><small>{detail}</small></div><button type="button" aria-label={`关闭${title}`} onClick={onClose}>×</button></header>
    <fieldset disabled={disabled}>{children}</fieldset>
  </aside>
}

function WorkspaceActivityPanel({ activities, busy, connection, error, onCancel, onOpen, onRetry }: {
  activities: TaskActivity[]
  busy: string
  connection: ActivityConnection
  error: string
  onCancel: (activity: TaskActivity) => Promise<boolean>
  onOpen: (activity: TaskActivity) => void
  onRetry: (activity: TaskActivity) => Promise<boolean>
}) {
  const connectionLabel = connection === 'live'
    ? '状态实时同步'
    : connection === 'connecting'
      ? '正在连接任务状态'
      : connection === 'reconnecting'
        ? '连接恢复中，已核对服务端快照'
        : '暂时离线，系统会自动重试'
  return <div className="strategy-activity-panel">
    <div className="strategy-activity-connection" data-connection={connection} role="status">
      <span aria-hidden="true"/>
      <div><b>{connectionLabel}</b>{error ? <small>{error}</small> : null}</div>
    </div>
    {busy.startsWith('activity:') ? <div className="strategy-activity-command" role="status">
      <LoaderCircle className="spin" size={15}/><span><b>正在提交任务操作</b><small>服务端确认后会自动更新这里的状态。</small></span>
    </div> : null}
    {!activities.length ? <div className="strategy-activity-empty">
      <CircleCheck size={22}/><b>当前没有后台任务</b><p>上传资料、启动研究或运行 AI 后，进度和恢复操作会出现在这里。</p>
    </div> : <div className="strategy-activity-list">
      {activities.map(activity => {
        const progress = activity.progress.value
        const commandBusy = busy === `activity:cancel:${activity.id}` || busy === `activity:retry:${activity.id}`
        return <article className="strategy-activity-card" data-status={activity.status} key={activity.id}>
          <header>
            <div><span className="strategy-activity-kind">{activityKindLabel(activity.kind)}</span><b>{activityStatusLabel(activity.status)}</b></div>
            <small>{activity.phase === 'cancelling' ? '正在取消' : activityPhaseLabel(activity.phase)}</small>
          </header>
          <p>{activity.summary}</p>
          {progress !== null ? <div className="strategy-activity-progress">
            <progress max="100" value={progress} aria-label={`${activityKindLabel(activity.kind)}进度 ${progress}%`}/>
            <span>{progress}%</span>
          </div> : <div className="strategy-activity-indeterminate"><span/><small>等待下一里程碑</small></div>}
          <div className="strategy-activity-meta">
            {activity.round ? <span>循环 {activity.round.current} / {activity.round.max}</span> : null}
            <span>{activity.heartbeat_at ? `心跳 ${activityTimeLabel(activity.heartbeat_at)}` : `更新 ${activityTimeLabel(activity.updated_at)}`}</span>
            {activity.cancel_requested ? <span>取消请求已接收</span> : null}
          </div>
          {activity.confirmed_conclusions.length ? <div className="strategy-activity-conclusions">
            <b>已确定结论</b>
            {activity.confirmed_conclusions.slice(0, 3).map(conclusion => <p key={conclusion.id}>{conclusion.text}</p>)}
          </div> : null}
          {activity.failure ? <div className="strategy-activity-failure" role="alert">
            <AlertCircle size={14}/><span><b>{activity.status === 'stalled' ? '执行可能中断' : '当前阶段未完成'}</b><small>{activity.failure.message}</small></span>
          </div> : null}
          <footer>
            <button type="button" onClick={() => onOpen(activity)}>打开</button>
            {activity.actions.includes('cancel') ? <button type="button" disabled={commandBusy} onClick={() => void onCancel(activity)}>取消</button> : null}
            {activity.actions.includes('retry') ? <button type="button" disabled={commandBusy} onClick={() => void onRetry(activity)}>重试</button> : null}
          </footer>
        </article>
      })}
    </div>}
  </div>
}

function activityKindLabel(kind: TaskActivity['kind']) {
  return {
    assistant: '项目 AI 助手',
    quick_research: '联网搜索',
    deep_research: '深度研究',
    document_parse: '文档解析',
    brief_generation: 'Brief 补全',
    strategy_generation: '策略生成',
    deep_review: '第二视角',
  }[kind]
}

function activityStatusLabel(status: TaskActivity['status']) {
  return {
    queued: '等待执行', running: '正在运行', waiting_user: '等待输入',
    partially_completed: '部分完成', succeeded: '已完成', failed: '未完成',
    cancelled: '已取消', stalled: '需要恢复',
  }[status]
}

function activityPhaseLabel(phase: string) {
  return ({
    queued: '排队', scanning: '文件检查', extracting: '正文提取', chunking: '生成片段',
    planning: '规划', reading: '读取资料', searching: '联网检索', saving: '保存结果',
    understanding: '理解需求', drafting: '形成初稿', revising: '修订', reviewing: '检查',
    completed: '完成', failed: '失败', cancelled: '取消',
  } as Record<string, string>)[phase] ?? phase
}

function activityTimeLabel(value: string) {
  const elapsed = Math.max(0, Date.now() - new Date(value).getTime())
  if (elapsed < 60_000) return '刚刚'
  if (elapsed < 3_600_000) return `${Math.floor(elapsed / 60_000)} 分钟前`
  return new Date(value).toLocaleString('zh-CN', { month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit' })
}

function StageContextToolbar({ activePanel, onPanel, stage }: {
  activePanel?: StrategyPanel
  onPanel: (panel: StrategyPanel) => void
  stage: StrategyStage
}) {
  const actions: Array<{ panel: StrategyPanel; label: string; icon: ReactNode }> = []
  if (stage === 'brief' || stage === 'strategy') actions.push({ panel: 'research', label: '研究', icon: <Search size={14}/> })
  actions.push({ panel: 'materials', label: '资料', icon: <FileText size={14}/> })
  if (stage === 'strategy' || stage === 'review' || stage === 'handoff') {
    actions.push({ panel: 'history', label: '版本历史', icon: <RotateCcw size={14}/> })
  }
  return <nav className="strategy-stage-context-toolbar" aria-label="当前阶段辅助面板">
    <span>按需打开</span>
    {actions.map(action => <button
      key={action.panel}
      type="button"
      aria-pressed={activePanel === action.panel}
      onClick={() => onPanel(action.panel)}
    >{action.icon}{action.label}</button>)}
  </nav>
}

function readStoredAssistantState(key: string): boolean {
  try {
    return window.localStorage.getItem(key) === 'open'
  } catch {
    return false
  }
}

function storeAssistantState(key: string, open: boolean) {
  try {
    window.localStorage.setItem(key, open ? 'open' : 'closed')
  } catch {
    // Dock visibility is a convenience setting and must never block the workspace.
  }
}

function readStoredAssistantWidth(key: string) {
  const value = Number(readStoredString(key))
  return Number.isFinite(value) ? Math.max(320, Math.min(560, value)) : 356
}

function storeAssistantWidth(key: string, width: number) {
  storeString(key, String(Math.round(width)))
}

function readStoredString(key: string) {
  try {
    return window.localStorage.getItem(key) ?? ''
  } catch {
    return ''
  }
}

function storeString(key: string, value: string) {
  try {
    window.localStorage.setItem(key, value)
  } catch {
    // Assistant presentation state is recoverable and never blocks work.
  }
}

type WorkspaceState = ReturnType<typeof useStrategyWorkspace>['state']

type StrategyWorkspaceView = '对话' | 'Brief' | '研究' | '策略' | '评审' | '创意任务策略'

function OverviewPane({ state, onNavigate }: { state: WorkspaceState; onNavigate: (view: StrategyWorkspaceView) => void }) {
  const document = state.draft?.revision?.document
  const hasConversation = state.messages.some(message => message.role === 'assistant')
  const briefConfirmed = state.brief?.status === 'confirmed'
  const strategyReady = Boolean(state.draft?.current_revision)
  const reviewApproved = state.review?.status === 'approved'
  const packagePublished = Boolean(state.published)
  const stages = [
    {
      label: '需求对话',
      view: '对话' as const,
      complete: hasConversation,
      detail: `${state.messages.length} 条消息`,
    },
    {
      label: 'Brief',
      view: 'Brief' as const,
      complete: briefConfirmed,
      detail: briefConfirmed
        ? `已冻结 v${state.briefVersion?.version ?? state.brief?.version ?? 1}`
        : state.brief?.completeness.ready ? '可以确认' : `${state.brief?.completeness.blockers.length ?? 0} 个阻断项`,
    },
    {
      label: '策略',
      view: '策略' as const,
      complete: strategyReady,
      detail: state.draft ? `Revision ${state.draft.current_revision} · ${statusLabel(state.draft.status)}` : '等待生成',
    },
    {
      label: '评审',
      view: '评审' as const,
      complete: reviewApproved,
      detail: state.review ? statusLabel(state.review.status) : '尚未提交',
    },
    {
      label: '创意交接',
      view: '创意任务策略' as const,
      complete: false,
      available: packagePublished,
      detail: packagePublished ? `策略包 v${state.published?.version} 已就绪` : '等待策略包发布',
    },
  ]

  const next = !hasConversation
    ? { view: '对话' as const, eyebrow: '从这里开始', title: '把业务问题说清楚', detail: '先通过对话补齐目标、受众和边界，再形成可冻结 Brief。', action: '开始策略对话' }
    : !briefConfirmed
      ? { view: 'Brief' as const, eyebrow: '需要你的判断', title: '确认策略输入', detail: '核对目标、人群、渠道与限制条件，冻结后再进入策略生成。', action: '检查并确认 Brief' }
      : !strategyReady || ['draft', 'returned', 'failed'].includes(state.draft?.status ?? '')
        ? { view: '策略' as const, eyebrow: '下一步', title: strategyReady ? '完善当前策略 Revision' : '生成第一版策略', detail: '聚焦核心主张、渠道分工、创意方向和可验证指标。', action: strategyReady ? '继续完善策略' : '进入策略生成' }
        : !reviewApproved
          ? { view: '评审' as const, eyebrow: '等待决策', title: '完成策略评审', detail: '先看核心判断与风险，再决定批准发布或退回修订。', action: '进入策略评审' }
          : { view: '创意任务策略' as const, eyebrow: '策略已就绪', title: '把策略变成可执行创意', detail: '选择业务 Route，补齐任务级约束，并一键进入图文或品牌广告生产。', action: '开始创意交接' }

  const evidenceCount = document?.evidence_refs?.length ?? state.brief?.document.reference_ids?.length ?? 0
  const evidenceSummary = state.brief?.document.product?.evidence?.[0]
    || (evidenceCount > 0 ? '已绑定来源文档，可回到研究页核对原文和采用状态。' : '当前没有已确认的产品事实或来源，创意不能把推测写成卖点。')
  const primaryChannel = document?.channel_strategy?.[0]
  const progress = Math.round((stages.filter(stage => stage.complete).length / (stages.length - 1)) * 100)
  const qualityReport = state.metadata?.quality_report
  const usesDecisionQualityGate = state.metadata?.prompt_version === 'strategy.generate.v4'
  const p0Metrics = state.p0Metrics
  const requirementRate = p0Metrics && p0Metrics.funnel.conversations_engaged > 0
    ? Math.round(p0Metrics.funnel.requirements_confirmed / p0Metrics.funnel.conversations_engaged * 100)
    : null

  return <section className="kanon-strategy-overview">
    <div className="kanon-command-hero">
      <div className="kanon-command-copy">
        <span className="kanon-command-eyebrow">{next.eyebrow} · {String(Math.min(stages.findIndex(stage => !stage.complete) + 1 || stages.length, stages.length)).padStart(2, '0')}</span>
        <h2>{next.title}</h2>
        <p>{next.detail}</p>
        <button className="primary-button kanon-command-cta" onClick={() => onNavigate(next.view)}>
          <span>{next.action}</span><i><ChevronRight size={15} strokeWidth={1.6}/></i>
        </button>
      </div>
      <div className="kanon-command-progress" aria-label={`策略完成度 ${progress}%`}>
        <span>STRATEGY<br/>READINESS</span>
        <strong>{progress}<small>%</small></strong>
        <div><i style={{ width: `${progress}%` }}/></div>
        <small>{packagePublished ? '策略包已发布，可以交接创意' : '每一步均使用真实持久化状态'}</small>
        <footer className={`kanon-command-quality${usesDecisionQualityGate ? '' : ' legacy'}`}>
          <span>{usesDecisionQualityGate ? 'V4 决策质量门' : qualityReport ? '旧版基础校验' : '等待质量校验'}</span>
          <b>{qualityReport ? `${qualityReport.score}/100` : '—'}</b>
        </footer>
      </div>
    </div>

    <nav className="kanon-journey" aria-label="策略到创意决策旅程">
      {stages.map((stage, index) => <button
        className={`${stage.complete ? 'complete' : ''}${stage.view === next.view ? ' current' : ''}`}
        key={stage.label}
        onClick={() => onNavigate(stage.view)}
        type="button"
      >
        <span>{stage.complete ? <Check size={14} strokeWidth={1.8}/> : String(index + 1).padStart(2, '0')}</span>
        <b>{stage.label}</b>
        <small>{stage.detail}</small>
      </button>)}
    </nav>

    {p0Metrics ? <section className="kanon-p0-evidence" aria-label="P0 业务成效证据">
      <header>
        <div><span className="section-label">P0 RELEASE EVIDENCE</span><h3>观察到的业务漏斗</h3></div>
        <small>近 {p0Metrics.window.days} 天 · 仅代表行为记录，不代表因果提升</small>
      </header>
      <div>
        <article>
          <span>需求冻结率</span>
          <strong>{requirementRate === null ? '—' : `${requirementRate}%`}</strong>
          <small>{p0Metrics.funnel.requirements_confirmed} / {p0Metrics.funnel.conversations_engaged} 个有效对话</small>
        </article>
        <article>
          <span>需求确认 P50</span>
          <strong>{formatMetricDuration(p0Metrics.timings.median_seconds_to_requirement)}</strong>
          <small>{p0Metrics.timings.requirement_samples} 个样本 · 平均 {p0Metrics.timings.average_user_turns_to_requirement ?? '—'} 轮输入</small>
        </article>
        <article>
          <span>进入创意的路径</span>
          <strong>{p0Metrics.paths.quick_intakes}<i> quick</i> / {p0Metrics.paths.full_intakes}<i> full</i></strong>
          <small>{p0Metrics.funnel.creative_tasks_created} 个 Creative Task</small>
        </article>
        <article>
          <span>高级能力实用量</span>
          <strong>{p0Metrics.turns.deep_turns}<i> deep</i> / {p0Metrics.turns.web_search_turns}<i> web</i></strong>
          <small>{p0Metrics.turns.failed_agent_turns} 次对话 Agent 失败</small>
        </article>
        <article>
          <span>明确有用反馈</span>
          <strong>{p0Metrics.feedback.useful_rate === null ? '—' : `${Math.round(p0Metrics.feedback.useful_rate * 100)}%`}</strong>
          <small>{p0Metrics.feedback.responses} 份人工反馈，不能用模型评分替代</small>
        </article>
      </div>
    </section> : null}

    <div className="kanon-decision-grid">
      <article className="kanon-decision-card primary">
        <span className="section-label">核心判断</span>
        <h3>{document?.proposition || document?.executive_summary || '策略生成后，这里会沉淀唯一核心判断。'}</h3>
        <p>{document?.executive_summary || '把复杂输入收敛为团队可以共同执行的一句话。'}</p>
        <button className="text-button" onClick={() => onNavigate('策略')} type="button">查看完整策略 <ChevronRight size={13}/></button>
      </article>
      <article className="kanon-decision-card audience">
        <span className="section-label">为谁而做</span>
        <h3>{document?.audience.primary || state.brief?.document.audience.primary || '待明确核心人群'}</h3>
        <ul>{document?.audience.insights?.slice(0, 3).map(insight => <li key={insight}>{insight}</li>) ?? <li>完成策略后显示关键人群洞察</li>}</ul>
      </article>
      <article className="kanon-decision-card channel">
        <span className="section-label">渠道与内容切口</span>
        <h3>{primaryChannel ? `${primaryChannel.platform} · ${primaryChannel.role}` : '待明确首要渠道角色'}</h3>
        <p>{primaryChannel?.formats?.length ? primaryChannel.formats.join(' / ') : '策略会明确内容形式、渠道分工与交付目的。'}</p>
        <small>品牌广告还是效果广告，会在“创意任务策略”中作为创作路线明确确认。</small>
      </article>
      <article className="kanon-decision-card evidence">
        <span className="section-label">用户为什么相信</span>
        <div className="kanon-evidence-number"><strong>{evidenceCount}</strong><small>条已绑定证据</small></div>
        <p>{evidenceSummary}</p>
        <small>可信依据是可追溯的产品事实、客户材料或研究来源，不是策略自己写出的判断。</small>
        <button className="text-button" onClick={() => onNavigate('研究')} type="button">查看证据 <ChevronRight size={13}/></button>
      </article>
      <article className="kanon-decision-card ideas">
        <span className="section-label">可进入创意的方向</span>
        {(document?.creative_strategy?.territories?.length || document?.creative_recommendations?.length)
          ? <ol>{(document.creative_strategy?.territories.map(territory => territory.core_idea) ?? document.creative_recommendations ?? []).slice(0, 3).map((idea, index) => <li key={idea}><span>{String(index + 1).padStart(2, '0')}</span><p>{idea}</p></li>)}</ol>
          : <p>策略确认后，将在这里展示最值得进入创意生产的三个方向。</p>}
      </article>
    </div>

    <div className="kanon-strategy-note" role="status">
      <ShieldCheck size={18}/>
      <div><b>{packagePublished ? `策略包 v${state.published?.version} 已冻结并可追溯` : state.probe?.ready ? `真实模型已验证：${state.probe.model_version}` : '真实模型由服务端路由管理'}</b><p>{packagePublished ? '后续创意只继承已发布版本；任何修改都会创建新 Revision，不会悄悄覆盖当前交付。' : state.probe?.ready ? `结构化输出通过，耗时 ${state.probe.latency_ms} ms；路由与用量均已记录。` : '在“策略”步骤运行真实模型探针，可验证当前路由、结构化输出与凭据状态。'}</p></div>
    </div>
  </section>
}

function formatMetricDuration(seconds: number | null) {
  if (seconds === null) return '—'
  if (seconds < 60) return `${seconds}s`
  if (seconds < 3600) return `${Math.round(seconds / 60)}m`
  return `${Math.round(seconds / 360) / 10}h`
}

const briefFields: Array<{
  path: string
  label: string
  value: (brief: BriefDraft) => string
  parse?: (value: string) => unknown
  multiline?: boolean
}> = [
  { path: 'brand.name', label: '品牌', value: brief => brief.document.brand?.name ?? '' },
  { path: 'product.name', label: '产品', value: brief => brief.document.product?.name ?? '' },
  { path: 'product.category', label: '产品品类', value: brief => brief.document.product?.category ?? '' },
  { path: 'product.selling_points', label: '产品卖点', value: brief => brief.document.product?.selling_points?.join('\n') ?? '', parse: splitValues, multiline: true },
  { path: 'product.evidence', label: '产品证据', value: brief => brief.document.product?.evidence?.join('\n') ?? '', parse: splitValues, multiline: true },
  { path: 'industry', label: '行业', value: brief => brief.document.industry ?? '' },
  { path: 'region', label: '地区', value: brief => brief.document.region ?? '' },
  { path: 'language', label: '语言', value: brief => brief.document.language ?? '' },
  { path: 'campaign.objective', label: '推广目标', value: brief => brief.document.campaign.objective, multiline: true },
  { path: 'audience.primary', label: '核心受众', value: brief => brief.document.audience.primary, multiline: true },
  { path: 'proposition', label: '核心主张', value: brief => brief.document.proposition, multiline: true },
  { path: 'channels', label: '渠道', value: brief => brief.document.channels.join('、'), parse: splitValues },
  { path: 'budget.total', label: '预算', value: brief => brief.document.budget.total },
  { path: 'schedule.window', label: '周期', value: brief => brief.document.schedule.window },
  { path: 'measurement.primary_kpi', label: '核心 KPI', value: brief => brief.document.measurement.primary_kpi },
  { path: 'constraints', label: '约束条件', value: brief => brief.document.constraints.join('\n'), parse: splitValues, multiline: true },
  { path: 'creative.tone', label: '表达调性', value: brief => brief.document.creative?.tone?.join('\n') ?? '', parse: splitValues, multiline: true },
  { path: 'creative.mandatory_elements', label: '必提内容', value: brief => brief.document.creative?.mandatory_elements?.join('\n') ?? '', parse: splitValues, multiline: true },
  { path: 'creative.prohibited_claims', label: '禁用表达', value: brief => brief.document.creative?.prohibited_claims?.join('\n') ?? '', parse: splitValues, multiline: true },
]

const briefHighRiskFields = new Set([
  'proposition',
  'product.evidence',
  'budget.total',
  'constraints',
  'creative.mandatory_elements',
  'creative.prohibited_claims',
])

const briefDecisionGroups: Array<{
  id: string
  title: string
  description: string
  fields: string[]
}> = [
  { id: 'goal', title: '业务目标', description: '先对齐要解决的问题和对外核心主张。', fields: ['campaign.objective', 'proposition'] },
  { id: 'product', title: '产品与证据', description: '明确本次主推对象、可用卖点和证据边界。', fields: ['brand.name', 'product.name', 'product.category', 'product.selling_points', 'product.evidence'] },
  { id: 'audience', title: '受众与情境', description: '说明为谁沟通，以及所在行业、地区和语言环境。', fields: ['audience.primary', 'industry', 'region', 'language'] },
  { id: 'channel', title: '渠道与转化', description: '确定触达阵地和最终衡量方式。', fields: ['channels', 'measurement.primary_kpi'] },
  { id: 'resource', title: '资源与约束', description: '锁定预算、周期和必须遵守的执行条件。', fields: ['budget.total', 'schedule.window', 'constraints'] },
  { id: 'creative', title: '创意边界', description: '定义表达调性、必提内容和禁用表述。', fields: ['creative.tone', 'creative.mandatory_elements', 'creative.prohibited_claims'] },
]

function BriefPane({ assistantBusyId, assistantError, assistantProposals, brief, busy, onApplyProposal, onAutosaveField, onConfirm, onConfirmFields, onField, onIgnoreProposal, onOpenAssistant, onSelectProduct, sessionProjectId, sessionWorkspaceId }: {
  assistantBusyId: string
  assistantError: string
  assistantProposals: ArtifactProposal[]
  brief: BriefDraft | null
  busy: string
  onApplyProposal: (proposal: ArtifactProposal, operations?: BriefPatchOperation[]) => Promise<boolean>
  onAutosaveField: (path: string, value: unknown) => Promise<boolean>
  onConfirm: () => Promise<boolean>
  onConfirmFields: (operations: Array<{ fieldPath: string; value: unknown }>) => Promise<boolean>
  onField: (path: string, value: unknown) => Promise<boolean>
  onIgnoreProposal: (proposal: ArtifactProposal) => Promise<boolean>
  onOpenAssistant: () => void
  onSelectProduct: (candidate: BriefProductCandidate) => Promise<boolean>
  sessionProjectId: string
  sessionWorkspaceId: string
}) {
  if (!brief) return <UnavailablePane title="Brief 尚未创建" detail="请先进入对话并发送第一条需求信息。"/>
  const frozen = brief.status === 'confirmed'
  const productCandidates = brief.document.product?.candidates ?? []
  const populatedCount = briefFields.filter(field => field.value(brief).trim()).length
  const confirmedCount = briefFields.filter(field => field.value(brief).trim() && brief.field_states[field.path]?.confirmation === 'confirmed').length
  const optionalCreativeContext = briefFields.filter(field => ['brand.name', 'industry', 'region', 'language'].includes(field.path) && !field.value(brief).trim())
  const confirmationReadiness = brief.base_brief_version ? getFullStrategyDraftReadiness(brief) : brief.completeness
  return <section className="kanon-brief-pane">
    <div className="kanon-strategy-heading">
      <div><span className="section-label">{brief.base_brief_version ? `BRIEF REVISION · 基于 v${brief.base_brief_version}` : `BRIEF DRAFT v${brief.version}`}</span><h2>{brief.base_brief_version ? '补充完整策略输入' : '确认策略输入'}</h2><p>{brief.base_brief_version ? '原冻结版本保持不变；这里只补充完整策略缺少的信息，确认后将冻结为新的 BriefVersion。' : '字段修改使用服务端版本校验，确认后冻结为不可变 BriefVersion。'}</p></div>
      <div className="kanon-brief-heading-actions">
        {!frozen ? <button className="secondary-button" disabled={Boolean(busy)} onClick={onOpenAssistant} type="button"><Sparkles size={14}/>让 AI 帮我补充</button> : null}
        <span className={`source-chip ${confirmationReadiness.ready ? '' : 'alert'}`}>{frozen ? '已冻结' : confirmationReadiness.ready ? '可以确认' : `${confirmationReadiness.blockers.length} 个阻断项`}</span>
      </div>
    </div>
    <div className="kanon-brief-health" role="status">
      <div><b>{populatedCount}<small> / {briefFields.length}</small></b><span>已填写</span></div>
      <div><b>{confirmedCount}</b><span>已确认</span></div>
      <p>{optionalCreativeContext.length
        ? `${optionalCreativeContext.map(field => field.label).join('、')}未提供；这些信息会在需要时于创作前补充，不再阻断当前交接。`
        : '品牌、市场与语言上下文完整，可直接用于创意交接。'}</p>
    </div>
    {productCandidates.length ? <ProductCandidatePanel
      busy={busy === 'brief:select-product'}
      candidates={productCandidates}
      disabled={frozen || Boolean(busy)}
      onSelect={onSelectProduct}
      selectedName={brief.document.product?.name ?? ''}
    /> : null}
    {assistantProposals.length || assistantError ? <div className="kanon-brief-ai-proposals">
      <div className="kanon-brief-ai-proposals-copy"><Sparkles size={16}/><span><b>AI 候选补充</b><small>来自当前项目上下文；采用前可以编辑，任何建议都不会自动写入。</small></span></div>
      <AssistantProposalList
        brief={brief}
        busyId={assistantBusyId}
        disabled={frozen || Boolean(busy)}
        error={assistantError}
        onApply={onApplyProposal}
        onIgnore={onIgnoreProposal}
        proposals={assistantProposals}
      />
    </div> : null}
    <div className="kanon-brief-groups">
      {briefDecisionGroups.map((group, groupIndex) => {
        const fields = group.fields.map(path => briefFields.find(field => field.path === path)).filter((field): field is typeof briefFields[number] => Boolean(field))
        const populated = fields.filter(field => field.value(brief).trim()).length
        const confirmed = fields.filter(field => field.value(brief).trim() && brief.field_states[field.path]?.confirmation === 'confirmed').length
        const bulkConfirmable = fields.flatMap(field => {
          const value = field.value(brief)
          if (briefHighRiskFields.has(field.path) || !value.trim() || brief.field_states[field.path]?.confirmation === 'confirmed') return []
          return [{ fieldPath: field.path, value: field.parse ? field.parse(value) : value }]
        })
        return <section className="kanon-brief-group" key={group.id} aria-labelledby={`brief-group-${group.id}`}>
          <header>
            <div className="kanon-brief-group-index">{String(groupIndex + 1).padStart(2, '0')}</div>
            <div><h3 id={`brief-group-${group.id}`}>{group.title}</h3><p>{group.description}</p></div>
            <span>{confirmed}/{populated} 已确认</span>
          </header>
          <div className="kanon-field-grid">
            {fields.map(field => <EditableField
              busy={busy === `brief:${field.path}`}
              disabled={frozen || Boolean(busy)}
              frozen={frozen}
              highRisk={briefHighRiskFields.has(field.path)}
              key={field.path}
              label={field.label}
              multiline={field.multiline}
              onAutosave={value => onAutosaveField(field.path, field.parse ? field.parse(value) : value)}
              onSave={value => onField(field.path, field.parse ? field.parse(value) : value)}
              draftStorageKey={workspaceSessionKey(sessionProjectId, sessionWorkspaceId, 'brief-field', brief.id, field.path)}
              state={brief.field_states[field.path]}
              value={field.value(brief)}
            />)}
          </div>
          {!frozen && bulkConfirmable.length ? <footer><small>普通信息可一次确认；标记“需单独确认”的字段仍需逐项核对。</small><button className="secondary-button" disabled={Boolean(busy)} onClick={() => void onConfirmFields(bulkConfirmable)} type="button"><Check size={14}/>{busy === 'confirm-brief-fields' ? '确认中…' : `确认本组普通信息（${bulkConfirmable.length}）`}</button></footer> : null}
        </section>
      })}
    </div>
    <div className="kanon-brief-footer">
      <div>
        {confirmationReadiness.blockers.map(blocker => <span key={`${blocker.field}-${blocker.reason}`}><AlertCircle size={13}/>{fieldLabel(blocker.field)}：{blocker.reason}</span>)}
        {!confirmationReadiness.blockers.length ? <span><CircleCheck size={13}/>必填信息与确认状态满足冻结条件</span> : null}
      </div>
      <div className="kanon-brief-actions">
        <button className="primary-button" disabled={frozen || !confirmationReadiness.ready || Boolean(busy)} onClick={() => void onConfirm()}>
          <BadgeCheck size={16}/>{busy === 'confirm-brief' ? '确认中…' : frozen ? 'Brief 已冻结' : '确认并冻结 Brief'}
        </button>
      </div>
    </div>
  </section>
}

function ProductCandidatePanel({ busy, candidates, disabled, onSelect, selectedName }: {
  busy: boolean
  candidates: BriefProductCandidate[]
  disabled: boolean
  onSelect: (candidate: BriefProductCandidate) => Promise<boolean>
  selectedName: string
}) {
  return <section className="kanon-product-candidates" aria-labelledby="brief-product-candidates-title">
    <header>
      <div><span className="section-label">PRODUCT CANDIDATES</span><h3 id="brief-product-candidates-title">资料中识别到 {candidates.length} 个候选产品</h3></div>
      <p>选择主推后，会把该产品的名称、品类、卖点、证据、必提和禁用内容一次写入下方固定 Brief；不同产品不会混写。</p>
    </header>
    <div>
      {candidates.map(candidate => {
        const selected = selectedName.trim() === candidate.name.trim()
        return <article className={selected ? 'selected' : undefined} key={candidate.name}>
          <header><div><b>{candidate.name}</b><small>{candidate.category || '品类待确认'}</small></div><span>{candidate.source_refs?.length ?? 0} 处来源</span></header>
          {candidate.selling_points?.length ? <dl><dt>卖点</dt><dd>{candidate.selling_points.slice(0, 4).join(' · ')}</dd></dl> : null}
          {candidate.evidence?.length ? <dl><dt>证据</dt><dd>{candidate.evidence.slice(0, 3).join(' · ')}</dd></dl> : null}
          {candidate.mandatory_elements?.length ? <details><summary>必提内容（{candidate.mandatory_elements.length}）</summary><ul>{candidate.mandatory_elements.map(item => <li key={item}>{item}</li>)}</ul></details> : null}
          {candidate.prohibited_claims?.length ? <details><summary>禁用表达（{candidate.prohibited_claims.length}）</summary><ul>{candidate.prohibited_claims.map(item => <li key={item}>{item}</li>)}</ul></details> : null}
          <button disabled={disabled || selected} onClick={() => void onSelect(candidate)} type="button">{selected ? '已选为主推' : busy ? '正在选择…' : '选为主推产品'}</button>
        </article>
      })}
    </div>
  </section>
}

type EditableFieldSessionDraft = {
  baseValue: string
  draftValue: string
}

function EditableField({ busy, disabled, draftStorageKey, frozen, highRisk, label, multiline, onAutosave, onSave, state, value }: {
  busy: boolean
  disabled: boolean
  draftStorageKey: string
  frozen: boolean
  highRisk: boolean
  label: string
  multiline?: boolean
  onAutosave: (value: string) => Promise<boolean>
  onSave: (value: string) => Promise<boolean>
  state?: BriefDraft['field_states'][string]
  value: string
}) {
  const [initialDraft] = useState<EditableFieldSessionDraft | null>(() => frozen ? null : readEditableFieldDraft(draftStorageKey))
  const [draftValue, setDraftValue] = useState(() => initialDraft?.draftValue ?? value)
  const [serverChangedWhileEditing, setServerChangedWhileEditing] = useState(() => Boolean(
    initialDraft && initialDraft.baseValue !== value && initialDraft.draftValue !== value,
  ))
  const [saveState, setSaveState] = useState<'idle' | 'waiting' | 'saving' | 'saved' | 'error'>('idle')
  const autosaveGenerationRef = useRef(0)
  const autosaveSubmittedValueRef = useRef<string | null>(null)
  const onAutosaveRef = useRef(onAutosave)
  onAutosaveRef.current = onAutosave
  const draftValueRef = useRef(draftValue)
  const serverValueRef = useRef(value)
  useEffect(() => {
    if (frozen) {
      clearWorkspaceSessionValue(draftStorageKey)
      draftValueRef.current = value
      serverValueRef.current = value
      setDraftValue(value)
      setServerChangedWhileEditing(false)
      setSaveState('idle')
      return
    }
    const restored = readEditableFieldDraft(draftStorageKey)
    serverValueRef.current = value
    draftValueRef.current = restored?.draftValue ?? value
    setDraftValue(draftValueRef.current)
    setServerChangedWhileEditing(Boolean(restored && restored.baseValue !== value && restored.draftValue !== value))
    setSaveState(restored?.draftValue !== undefined && restored.draftValue.trim() !== value.trim() ? 'waiting' : 'idle')
  }, [draftStorageKey, frozen])
  useEffect(() => {
    const previousServerValue = serverValueRef.current
    if (previousServerValue === value) return
    serverValueRef.current = value
    const current = draftValueRef.current
    const acknowledgesOwnAutosave = autosaveSubmittedValueRef.current !== null &&
      autosaveSubmittedValueRef.current.trim() === value.trim()
    if (acknowledgesOwnAutosave) {
      autosaveSubmittedValueRef.current = null
      setServerChangedWhileEditing(false)
      if (current.trim() === value.trim()) {
        clearWorkspaceSessionValue(draftStorageKey)
        draftValueRef.current = value
        setDraftValue(value)
      } else {
        writeWorkspaceSessionValue<EditableFieldSessionDraft>(draftStorageKey, {
          baseValue: value,
          draftValue: current,
        })
        setSaveState('waiting')
      }
      return
    }
    if (frozen || current === previousServerValue || current === value) {
      clearWorkspaceSessionValue(draftStorageKey)
      draftValueRef.current = value
      setDraftValue(value)
      setServerChangedWhileEditing(false)
      return
    }
    setServerChangedWhileEditing(true)
    writeWorkspaceSessionValue<EditableFieldSessionDraft>(draftStorageKey, {
      baseValue: value,
      draftValue: current,
    })
  }, [draftStorageKey, frozen, value])
  const changed = draftValue.trim() !== value.trim()
  const needsConfirmation = Boolean(value.trim()) && state?.confirmation !== 'confirmed'
  const empty = !draftValue.trim()
  const changeDraft = (nextValue: string) => {
    autosaveGenerationRef.current += 1
    draftValueRef.current = nextValue
    setDraftValue(nextValue)
    if (nextValue.trim() === value.trim()) {
      clearWorkspaceSessionValue(draftStorageKey)
      setServerChangedWhileEditing(false)
      setSaveState('idle')
      return
    }
    setSaveState('waiting')
    writeWorkspaceSessionValue<EditableFieldSessionDraft>(draftStorageKey, {
      baseValue: value,
      draftValue: nextValue,
    })
  }
  useEffect(() => {
    if (frozen || disabled || serverChangedWhileEditing || !changed) return
    const generation = ++autosaveGenerationRef.current
    setSaveState('waiting')
    const timer = window.setTimeout(() => {
      if (generation !== autosaveGenerationRef.current) return
      setSaveState('saving')
      const submittedValue = draftValueRef.current
      autosaveSubmittedValueRef.current = submittedValue
      void onAutosaveRef.current(submittedValue).then(saved => {
        if (generation !== autosaveGenerationRef.current) return
        if (!saved) autosaveSubmittedValueRef.current = null
        setSaveState(saved ? 'saved' : 'error')
      })
    }, 750)
    return () => window.clearTimeout(timer)
  }, [changed, disabled, frozen, serverChangedWhileEditing, draftValue])
  const saveDraft = async () => {
    autosaveGenerationRef.current += 1
    if (!await onSave(draftValue)) return
    clearWorkspaceSessionValue(draftStorageKey)
    setServerChangedWhileEditing(false)
    setSaveState('saved')
  }
  const saveLabel = serverChangedWhileEditing
    ? '服务端已更新，请核对保留的输入'
    : saveState === 'waiting'
      ? '等待自动保存…'
      : saveState === 'saving'
        ? '正在自动保存…'
        : saveState === 'error'
          ? '自动保存失败 · 输入已保留'
          : saveState === 'saved'
            ? state?.confirmation === 'confirmed' ? '已保存 · 已确认' : highRisk ? '已自动保存 · 需单独确认' : '已自动保存 · 待确认'
            : empty
              ? '待补充'
              : state?.confirmation === 'confirmed'
                ? '服务端已保存 · 已确认'
                : highRisk
                  ? '服务端已保存 · 需单独确认'
                  : state
                    ? `服务端已保存 · ${state.confidence} 置信度`
                    : '服务端已保存 · 待确认'
  return <label className={`kanon-field ${multiline ? 'wide' : ''}${empty ? ' empty' : ''}`}>
    <span>{label}<small aria-live="polite">{saveLabel}</small></span>
    {frozen
      ? <output>{empty ? '未提供' : draftValue}</output>
      : multiline
      ? <textarea disabled={disabled} rows={3} value={draftValue} onChange={event => changeDraft(event.target.value)}/>
      : <input disabled={disabled} value={draftValue} onChange={event => changeDraft(event.target.value)}/>}
    {(changed || (needsConfirmation && highRisk)) && !disabled ? <button disabled={busy} type="button" onClick={() => void saveDraft()}>{busy ? '处理中…' : serverChangedWhileEditing ? '核对后保存' : changed ? '保存并确认' : '确认此字段'}</button> : null}
  </label>
}

function readEditableFieldDraft(key: string): EditableFieldSessionDraft | null {
  const value = readWorkspaceSessionValue<Partial<EditableFieldSessionDraft>>(key)
  return value && typeof value.baseValue === 'string' && typeof value.draftValue === 'string'
    ? { baseValue: value.baseValue, draftValue: value.draftValue }
    : null
}

function isCompatibleWorkspaceDraft(restored: unknown, original: unknown) {
  if (restored === null || restored === undefined || original === undefined) return false
  if (Array.isArray(original)) return Array.isArray(restored)
  if (original === null) return restored === null
  return typeof restored === typeof original && (typeof original !== 'object' || !Array.isArray(restored))
}

function readDirtyStrategySections(key: string) {
  const value = readWorkspaceSessionValue<unknown>(key)
  return Array.isArray(value) ? value.filter((item): item is string => typeof item === 'string') : []
}

function StrategyPane({ briefVersion, busy, deepReview, deepReviewError, draft, onCreateBriefRevision, onDeepReview, onGenerate, onPatch, onProbe, onRetry, onRevise, onSubmit, pending, probe, readiness, reviewPolicy, sessionProjectId, sessionWorkspaceId }: {
  briefVersion: BriefVersion | null
  busy: string
  deepReview: WorkspaceState['deepReview']
  deepReviewError: string
  draft: StrategyDraft | null
  reviewPolicy: WorkspaceState['reviewPolicy']
  onCreateBriefRevision: () => Promise<boolean>
  onDeepReview: () => Promise<boolean>
  onGenerate: () => Promise<boolean>
  onPatch: (section: string, value: unknown) => Promise<boolean>
  onProbe: () => Promise<boolean>
  onRetry: () => Promise<boolean>
  onRevise: (instruction: string) => Promise<boolean>
  onSubmit: () => Promise<boolean>
  pending: boolean
  probe: WorkspaceState['probe']
  readiness: WorkspaceState['readiness']
  sessionProjectId: string
  sessionWorkspaceId: string
}) {
  const sectionSelectionKey = workspaceSessionKey(sessionProjectId, sessionWorkspaceId, 'strategy-section-selection')
  const [section, setSection] = useState(() => {
    const restored = readWorkspaceSessionValue<string>(sectionSelectionKey)
    return typeof restored === 'string' && restored ? restored : 'objective'
  })
  const [sectionValue, setSectionValue] = useState<unknown>('')
  const document = draft?.revision?.document
  const revisionIdentity = draft ? `${draft.id}:${draft.current_revision}` : 'pending'
  const instructionStorageKey = workspaceSessionKey(sessionProjectId, sessionWorkspaceId, 'strategy-instruction', revisionIdentity)
  const sectionDraftStorageKey = workspaceSessionKey(sessionProjectId, sessionWorkspaceId, 'strategy-section-draft', revisionIdentity, section)
  const dirtySectionsStorageKey = workspaceSessionKey(sessionProjectId, sessionWorkspaceId, 'strategy-dirty-sections', revisionIdentity)
  const [instruction, setInstruction] = useState(() => {
    const restored = readWorkspaceSessionValue<string>(instructionStorageKey)
    return typeof restored === 'string' ? restored : ''
  })
  const [dirtySections, setDirtySections] = useState<string[]>(() => readDirtyStrategySections(dirtySectionsStorageKey))
  const strategyReadiness = briefVersion ? getFullStrategyReadiness(briefVersion) : null
  const briefReady = Boolean(strategyReadiness?.ready)
  const availableSections = document
    ? strategyEditorSectionGroups.flatMap(group => group.sections).filter(value => Object.prototype.hasOwnProperty.call(document, value))
    : []
  useEffect(() => {
    if (!availableSections.length || availableSections.includes(section)) return
    setSection(availableSections[0])
  }, [availableSections.join('|'), section])
  useEffect(() => {
    writeWorkspaceSessionValue(sectionSelectionKey, section)
  }, [section, sectionSelectionKey])
  useEffect(() => {
    setDirtySections(readDirtyStrategySections(dirtySectionsStorageKey))
  }, [dirtySectionsStorageKey])
  const markSectionDirty = (sectionName: string, dirty: boolean) => {
    setDirtySections(current => {
      const next = dirty
        ? current.includes(sectionName) ? current : [...current, sectionName]
        : current.filter(value => value !== sectionName)
      if (next.length) writeWorkspaceSessionValue(dirtySectionsStorageKey, next)
      else clearWorkspaceSessionValue(dirtySectionsStorageKey)
      return next
    })
  }
  useEffect(() => {
    if (!document) {
      setSectionValue('')
      return
    }
    const originalValue = (document as unknown as Record<string, unknown>)[section]
    const restored = readWorkspaceSessionValue<unknown>(sectionDraftStorageKey)
    const hasCompatibleDraft = isCompatibleWorkspaceDraft(restored, originalValue)
      && JSON.stringify(restored) !== JSON.stringify(originalValue)
    setSectionValue(structuredClone(hasCompatibleDraft ? restored : originalValue))
    markSectionDirty(section, hasCompatibleDraft)
  }, [sectionDraftStorageKey])
  useEffect(() => {
    const restored = readWorkspaceSessionValue<string>(instructionStorageKey)
    setInstruction(typeof restored === 'string' ? restored : '')
  }, [instructionStorageKey])

  const changeSectionValue = (value: unknown) => {
    setSectionValue(value)
    const originalValue = document ? (document as unknown as Record<string, unknown>)[section] : undefined
    if (JSON.stringify(value) === JSON.stringify(originalValue)) {
      clearWorkspaceSessionValue(sectionDraftStorageKey)
      markSectionDirty(section, false)
      return
    }
    writeWorkspaceSessionValue(sectionDraftStorageKey, value)
    markSectionDirty(section, true)
  }
  const resetSectionValue = () => {
    if (!document) return
    clearWorkspaceSessionValue(sectionDraftStorageKey)
    markSectionDirty(section, false)
    setSectionValue(structuredClone((document as unknown as Record<string, unknown>)[section]))
  }
  const saveSectionValue = async () => {
    if (!await onPatch(section, sectionValue)) return false
    clearWorkspaceSessionValue(sectionDraftStorageKey)
    markSectionDirty(section, false)
    return true
  }
  const changeInstruction = (value: string) => {
    setInstruction(value)
    if (value) writeWorkspaceSessionValue(instructionStorageKey, value)
    else clearWorkspaceSessionValue(instructionStorageKey)
  }
  const submitInstruction = async () => {
    if (!await onRevise(instruction)) return
    clearWorkspaceSessionValue(instructionStorageKey)
    setInstruction('')
  }

  if (!draft) {
    return <section className="kanon-strategy-generate">
      <Sparkles size={28}/>
      <h2>生成第一版策略</h2>
      <p>{briefReady ? '已确认 Brief 将作为不可变输入，生成结果会保存为 Strategy revision。' : briefVersion ? `Brief v${briefVersion.version} 已冻结，但还不满足完整策略生成条件。` : '请先完成并确认 Brief。'}</p>
      {briefVersion && strategyReadiness && !strategyReadiness.ready ? <div className="kanon-strategy-brief-blocker" role="alert">
        <div><AlertCircle size={18}/><span><b>还需补充 {strategyReadiness.blockers.length} 项策略输入</b><small>冻结版本不会被修改，系统会基于 Brief v{briefVersion.version} 创建补充修订。</small></span></div>
        <ul>{strategyReadiness.blockers.map(blocker => <li key={`${blocker.field}-${blocker.reason}`}><b>{fullStrategyFieldLabels[blocker.field] ?? blocker.field}</b><span>{blocker.reason}</span></li>)}</ul>
        <button className="secondary-button" disabled={Boolean(busy)} onClick={() => void onCreateBriefRevision()}><RotateCcw size={15}/>{busy === 'create-brief-revision' ? '正在创建修订…' : '创建 Brief 补充修订'}</button>
      </div> : null}
      <div className="kanon-generation-mode">
        <span>生成模式</span>
        <b>{probe?.ready ? probe.model_version : readiness?.generation_mode ?? '不可用'}</b>
        <small>{probe?.ready ? `真实探针通过 · ${probe.latency_ms} ms · ${probe.api_mode ?? '默认 API'}` : readiness?.reason_code ?? '尚未执行真实模型探针'}</small>
        {readiness?.generation_mode === 'provider' ? <button className="text-button" disabled={Boolean(busy)} onClick={() => void onProbe()}>{busy === 'generation-probe' ? '正在验证…' : '验证真实模型'}</button> : null}
      </div>
      <button className="primary-button" disabled={!briefReady || Boolean(busy)} onClick={() => void onGenerate()}><Sparkles size={16}/>{busy === 'generate-strategy' ? '正在创建…' : '生成第一版策略'}</button>
    </section>
  }

  if (draft.status === 'generating' || pending) {
    return <div className="kanon-strategy-state" role="status"><LoaderCircle className="spin" size={24}/><h2>策略生成中</h2><p>Agent 完成后会自动读取服务端 Revision。</p></div>
  }

  if (!document) {
    if (draft.status === 'failed') {
      return <section className="kanon-strategy-generate">
        <AlertCircle size={28}/>
        <h2>策略生成未完成</h2>
        <p>失败 Draft 和 AgentTask 记录已保留，可以基于同一份已冻结 Brief 重新生成。</p>
        <button className="primary-button" disabled={Boolean(busy)} onClick={() => void onRetry()}>
          <RefreshCw size={16}/>{busy === 'retry-strategy' ? '正在重新生成…' : '重新生成策略'}
        </button>
      </section>
    }
    return <UnavailablePane title="策略没有可用 Revision" detail={`当前状态：${statusLabel(draft.status)}。请重新加载或检查 AgentTask。`}/>
  }

  const sectionGroups = strategyEditorSectionGroups.map(group => ({
    ...group,
    sections: group.sections.filter(value => Object.prototype.hasOwnProperty.call(document, value)),
  })).filter(group => group.sections.length)
  const sectionCodes = new Map(sectionGroups.flatMap(group => group.sections).map((value, index) => [value, String(index + 1).padStart(2, '0')]))
  const original = (document as unknown as Record<string, unknown>)[section]
  const changed = JSON.stringify(sectionValue) !== JSON.stringify(original)
  // Editing an approved draft creates a new revision and leaves the published
  // StrategyPackage immutable. The backend already enforces that boundary.
  const legacyContract = document.contract_version !== 'strategy-draft/v3'
  const canEdit = !legacyContract && (draft.status === 'draft' || draft.status === 'returned' || draft.status === 'approved')
  const canDecide = !legacyContract && (draft.status === 'draft' || draft.status === 'returned')
  const selfConfirmation = reviewPolicy?.mode === 'self_confirmation'

  return <section className="kanon-strategy-editor-pane">
    <div className="kanon-strategy-heading kanon-strategy-document-heading">
      <div>
        <span className="section-label">STRATEGY REVISION {draft.current_revision}</span>
        <h2>{document.executive_summary || document.objective}</h2>
        <p>{document.proposition}</p>
      </div>
      <div className="kanon-strategy-document-meta">
        <span className="source-chip">{statusLabel(draft.status)}</span>
        <small title={draft.revision?.content_hash}>{draft.revision?.content_hash.slice(0, 18)}…</small>
      </div>
    </div>
    {legacyContract ? <div className="kanon-strategy-upgrade-banner" role="status"><Archive size={16}/><span><b>该策略使用旧版只读契约</b><small>{document.contract_version} 不再接受修订写入；请先运行 v3 successor 升级。历史 Revision 和已发布 Package 不会被改写。</small></span></div> : null}
    <div className="kanon-strategy-section-editor">
      <nav aria-label="策略区块">{sectionGroups.map(group => <section key={group.id}>
        <header><b>{group.title}</b><small>{group.description}</small></header>
        {group.sections.map(value => <button className={`${value === section ? 'active' : ''}${dirtySections.includes(value) ? ' has-draft' : ''}`} key={value} onClick={() => setSection(value)}>
          <span>{sectionCodes.get(value)}</span>{strategySectionLabel(value)}{dirtySections.includes(value) ? <em>未保存</em> : null}
        </button>)}
      </section>)}</nav>
      <div>
        <div className="surface-toolbar"><div><h3>{strategySectionLabel(section)}</h3><small>{strategySectionDescription(section)}</small></div><small>{legacyContract ? '旧版只读' : `当前 Revision ${draft.current_revision}`}</small></div>
        {section === 'creative_strategy'
          ? <CreativeStrategyEditor disabled={!canEdit || Boolean(busy)} onChange={changeSectionValue} value={sectionValue}/>
          : <StructuredStrategyEditor disabled={!canEdit || Boolean(busy)} onChange={changeSectionValue} section={section} value={sectionValue}/>}
        {changed ? <StrategySectionChangeImpact
          busy={busy === `strategy:${section}`}
          nextRevision={draft.current_revision + 1}
          onReset={resetSectionValue}
          onSave={saveSectionValue}
          original={original}
          section={section}
          value={sectionValue}
        /> : <div className="kanon-strategy-no-change"><CircleCheck size={14}/><span>当前章节与 Revision {draft.current_revision} 一致</span></div>}
      </div>
    </div>
    <StrategyPerspectivePanel
      analysis={deepReview}
      busy={busy}
      error={deepReviewError}
      onRun={onDeepReview}
      revision={draft.current_revision}
    />
    <div className="kanon-revise-box">
      <label htmlFor="strategy-revise">用自然语言修订</label>
      <textarea id="strategy-revise" disabled={!canEdit || Boolean(busy)} rows={2} value={instruction} onChange={event => changeInstruction(event.target.value)} placeholder="例如：把小红书的内容节奏调整为首周种草、第二周测评扩散。"/>
      <button className="secondary-button" disabled={!canEdit || Boolean(busy) || !instruction.trim()} onClick={() => void submitInstruction()}><RotateCcw size={14}/>{busy === 'revise-strategy' ? '修订中…' : '生成修订'}</button>
    </div>
    <div className="kanon-brief-footer">
      <span>{dirtySections.length
        ? `还有 ${dirtySections.length} 个章节的修改未保存；可切换章节继续核对，但提交前必须保存或撤销。`
        : selfConfirmation
        ? '个人模式无需提交给自己评审；确认后将直接发布绑定当前 Revision 与哈希的不可变策略包。'
        : '候选内容保存后提交给指定角色；评审与批准始终绑定 Revision 与内容哈希。'}</span>
      <button className="primary-button" disabled={!canDecide || Boolean(busy) || Boolean(dirtySections.length)} onClick={() => void onSubmit()}><BadgeCheck size={16}/>{selfConfirmation
        ? busy === 'confirm-publish' ? '正在确认并发布…' : '确认并发布策略包'
        : busy === 'submit-review' ? '正在提交…' : '提交正式评审'}</button>
    </div>
  </section>
}

function StructuredStrategyEditor({ disabled, onChange, section, value }: {
  disabled: boolean
  onChange: (value: unknown) => void
  section: string
  value: unknown
}) {
  if (typeof value === 'string') {
    const singleLine = ['平台', '核心 KPI', '实验变量', '衡量指标'].includes(section) && value.length < 54
    return <label className="kanon-structured-field wide"><span>{strategySectionLabel(section)}</span>{singleLine
      ? <input disabled={disabled} value={value} onChange={event => onChange(event.target.value)}/>
      : <textarea disabled={disabled} rows={Math.max(3, Math.min(6, Math.ceil(value.length / 70) + 2))} value={value} onChange={event => onChange(event.target.value)}/>}</label>
  }
  if (typeof value === 'number') return <label className="kanon-structured-field"><span>{strategySectionLabel(section)}</span><input disabled={disabled} type="number" value={value} onChange={event => onChange(Number(event.target.value))}/></label>
  if (typeof value === 'boolean') return <label className="kanon-check"><input checked={value} disabled={disabled} type="checkbox" onChange={event => onChange(event.target.checked)}/><span>{strategySectionLabel(section)}</span></label>
  if (Array.isArray(value)) {
    const objectArray = ['channel_strategy', 'experiment_matrix', 'platform_plans'].includes(section) || value.some(item => Boolean(item) && typeof item === 'object')
    if (!objectArray && value.every(item => typeof item === 'string')) {
      return <label className="kanon-structured-field wide"><span>{strategySectionLabel(section)}<small>每行一项</small></span><textarea disabled={disabled} rows={Math.max(5, Math.min(10, value.length + 2))} value={value.join('\n')} onChange={event => onChange(event.target.value.split('\n').map(item => item.trim()).filter(Boolean))}/></label>
    }
    return <div className="kanon-structured-list">
      {value.map((item, index) => <article key={`${section}-${index}`}><div className="surface-toolbar"><h4>{strategySectionLabel(section)} {index + 1}</h4><button aria-label={`删除${strategySectionLabel(section)} ${index + 1}`} className="text-button danger" disabled={disabled} onClick={() => onChange(value.filter((_, itemIndex) => itemIndex !== index))} type="button">删除</button></div><StructuredObjectFields disabled={disabled} onChange={next => onChange(value.map((current, itemIndex) => itemIndex === index ? next : current))} section={section} value={item}/></article>)}
      {!value.length ? <div className="panel-empty">当前没有条目，可按需新增。</div> : null}
      <button className="secondary-button" disabled={disabled} onClick={() => onChange([...value, strategyArrayTemplate(section)])} type="button"><Plus size={14}/>新增{strategySectionLabel(section)}条目</button>
    </div>
  }
  if (value && typeof value === 'object') return <StructuredObjectFields disabled={disabled} onChange={onChange} section={section} value={value}/>
  return <label className="kanon-structured-field"><span>{strategySectionLabel(section)}</span><input disabled={disabled} value="" onChange={event => onChange(event.target.value)}/></label>
}

function StructuredObjectFields({ disabled, onChange, section, value }: {
  disabled: boolean
  onChange: (value: unknown) => void
  section: string
  value: unknown
}) {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return <StructuredStrategyEditor disabled={disabled} onChange={onChange} section={section} value={value}/>
  const record = value as Record<string, unknown>
  return <div className="kanon-structured-grid">{Object.entries(record).map(([key, fieldValue]) => <div className={typeof fieldValue === 'string' ? '' : 'wide'} key={key}><StructuredStrategyEditor disabled={disabled} onChange={next => onChange({ ...record, [key]: next })} section={strategyFieldLabel(key)} value={fieldValue}/></div>)}</div>
}

function strategyArrayTemplate(section: string): unknown {
  if (section === 'channel_strategy') return { platform: '', role: '', formats: [] }
  if (section === 'experiment_matrix') return { hypothesis: '', variable: '', metric: '' }
  if (section === 'platform_plans') return { platform: '', role: '', audience_angle: '', content_pillars: [], formats: [], conversion_path: '', cadence: '', primary_kpi: '', creative_ideas: [], constraints: [] }
  return ''
}

type ReviewSessionDraft = {
  comment: string
  reason: string
}

function ReviewPane({ busy, comments, deepReview, deepReviewError, draft, draftStorageKey, onAddComment, onApprove, onConfirm, onDeepReview, onOpenCreative, onReturn, review, reviewPolicy, revisions }: {
  busy: string
  comments: WorkspaceState['comments']
  deepReview: WorkspaceState['deepReview']
  deepReviewError: string
  draft: StrategyDraft | null
  draftStorageKey: string
  onAddComment: (body: string) => Promise<boolean>
  onApprove: () => Promise<boolean>
  onConfirm: () => Promise<boolean>
  onDeepReview: () => Promise<boolean>
  onOpenCreative: () => void
  onReturn: (reason: string) => Promise<boolean>
  review: Review | null
  reviewPolicy: WorkspaceState['reviewPolicy']
  revisions: DraftRevision[]
}) {
  const [restoredDraft] = useState<ReviewSessionDraft>(() => readReviewSessionDraft(draftStorageKey) ?? { comment: '', reason: '' })
  const [comment, setComment] = useState(restoredDraft.comment)
  const [reason, setReason] = useState(restoredDraft.reason)
  const candidate = revisions.find(item => item.revision === review?.candidate_revision) ?? draft?.revision
  const previous = revisions.find(item => item.revision === (review?.candidate_revision ?? 1) - 1)
  const diffs = useMemo(() => strategyDiff(previous, candidate), [candidate, previous])
  const document = candidate?.document
  useEffect(() => {
    if (!comment && !reason) {
      clearWorkspaceSessionValue(draftStorageKey)
      return
    }
    writeWorkspaceSessionValue<ReviewSessionDraft>(draftStorageKey, { comment, reason })
  }, [comment, draftStorageKey, reason])

  const addComment = async () => {
    if (!await onAddComment(comment)) return
    setComment('')
  }
  const returnForChanges = async () => {
    if (!await onReturn(reason)) return
    clearWorkspaceSessionValue(draftStorageKey)
    setReason('')
  }

  if (!review && reviewPolicy?.mode === 'self_confirmation' && candidate?.document && draft) {
    const riskCount = candidate.document.assumptions_and_gaps.length + (candidate.document.compliance?.issues.length ?? 0)
    return <section className="kanon-review-pane kanon-self-confirmation">
      <div className="kanon-strategy-heading">
        <div><span className="section-label">SELF CONFIRMATION</span><h2>确认 Revision {candidate.revision}</h2><p>个人模式不会创建“提交给自己评审”的中间步骤。</p></div>
        <span className="source-chip">个人确认</span>
      </div>
      <section className="kanon-review-brief" aria-label="待确认的核心策略判断">
        <div><span className="section-label">一句话决策</span><h3>{candidate.document.proposition}</h3><p>{candidate.document.executive_summary || candidate.document.objective}</p></div>
        <dl>
          <div><dt>核心人群</dt><dd>{candidate.document.audience.primary}</dd></div>
          <div><dt>渠道方案</dt><dd>{candidate.document.channel_strategy.length} 个</dd></div>
          <div><dt>证据引用</dt><dd>{candidate.document.evidence_refs?.length ?? 0} 条</dd></div>
          <div><dt>待关注</dt><dd className={riskCount ? 'warn' : ''}>{riskCount} 项</dd></div>
        </dl>
        <small>确认动作会原子发布不可变 StrategyPackage；失败时不会留下半完成评审。</small>
      </section>
      <StrategyPerspectivePanel analysis={deepReview} busy={busy} error={deepReviewError} onRun={onDeepReview} revision={candidate.revision}/>
      <div className="kanon-self-confirm-action">
        <div><ShieldCheck size={19}/><span><b>确认后仍可继续修订</b><small>后续修改会创建 successor Revision 和新 Package，当前历史包及其哈希保持不变。</small></span></div>
        <button className="primary-button" disabled={Boolean(busy) || draft.status === 'approved'} onClick={() => void onConfirm()}><BadgeCheck size={15}/>{busy === 'confirm-publish' ? '正在确认并发布…' : '确认并发布策略包'}</button>
      </div>
      <details className="kanon-technical-details kanon-review-proof"><summary>版本与完整性校验</summary><code>{candidate.content_hash}</code></details>
    </section>
  }

  if (!review && reviewPolicy?.mode === 'self_confirmation') {
    return <UnavailablePane
      title="尚无可确认的策略版本"
      detail="请先在“策略”阶段生成或完善候选 Revision；个人模式会在这里直接确认并发布，不需要提交给自己评审。"
    />
  }

  if (!review) return <UnavailablePane title="尚未提交正式评审" detail="当前项目启用了多人评审。请在“策略”阶段完成候选 Revision 后提交给指定角色。"/>

  return <section className="kanon-review-pane">
    <div className="kanon-strategy-heading">
      <div><span className="section-label">REVIEW {review.id.slice(0, 12)}</span><h2>Revision {review.candidate_revision} 评审</h2><p>决策绑定候选内容哈希，过期版本不能批准。</p></div>
      <span className={`source-chip ${review.status === 'returned' ? 'alert' : ''}`}>{statusLabel(review.status)}</span>
    </div>
    {document ? <StrategyDecisionBrief document={document} review={review}/> : null}
    <StrategyPerspectivePanel analysis={deepReview} busy={busy} error={deepReviewError} onRun={onDeepReview} revision={review.candidate_revision}/>
    <div className="kanon-review-content">
      <div className="kanon-review-diffs">
        {!diffs.length && candidate ? <details className="kanon-review-full kanon-review-details"><summary>查看完整候选策略</summary><ReviewValue value={candidate.document}/></details> : diffs.map(diff => <article key={diff.section}>
          <h3>{strategySectionLabel(diff.section)}</h3>
          <div><section><small>之前</small><ReviewValue emptyLabel="暂无上一版本" value={diff.before}/></section><section><small>候选</small><ReviewValue value={diff.after}/></section></div>
        </article>)}
      </div>
      <aside className="kanon-review-comments">
        <h3>评论与决策</h3>
        {review.assignments?.map(assignment => <article key={assignment.id}><b>{assignment.review_mode === 'self_confirmation' ? '个人确认' : '指定审批人'}</b><p>{assignment.reviewer_user_id}</p><small>{assignment.status}</small></article>)}
        {comments.map(item => <article key={item.id}><b>{item.author_id}</b><p>{item.body}</p><small>{formatTime(item.created_at)}</small></article>)}
        {!comments.length ? <p className="panel-empty">尚无评论。</p> : null}
        {review.status === 'open' ? <>
          <textarea disabled={Boolean(busy)} rows={3} value={comment} onChange={event => setComment(event.target.value)} placeholder="留下可执行评论"/>
          <small>{comment || reason ? '未提交内容仅在当前浏览器会话保留；决策前需先提交评论或清空无关输入。' : '评论、批准和退回是三个独立动作。'}</small>
          <button className="secondary-button full" disabled={Boolean(busy) || !comment.trim()} onClick={() => void addComment()}>添加评论</button>
          <button className="primary-button full" disabled={Boolean(busy) || Boolean(comment.trim()) || Boolean(reason.trim())} onClick={() => void onApprove()}><BadgeCheck size={15}/>{busy === 'approve-review' ? '处理中…' : review.review_mode === 'self_confirmation' ? '确认并发布策略包' : '批准并发布策略包'}</button>
          <textarea disabled={Boolean(busy)} rows={2} value={reason} onChange={event => setReason(event.target.value)} placeholder="退回原因（必填）"/>
          <button className="secondary-button full" disabled={Boolean(busy) || !reason.trim() || Boolean(comment.trim())} onClick={() => void returnForChanges()}>退回修改</button>
        </> : review.status === 'approved' ? <div className="kanon-review-approved">
          <CircleCheck size={18}/>
          <span><b>策略已确认并发布</b><small>下一步将冻结的判断转换成可执行创意任务。</small></span>
          <button className="primary-button full" onClick={onOpenCreative}>开始创意交接 <ChevronRight size={14}/></button>
        </div> : <div className="kanon-review-decision"><AlertCircle size={17}/><span>{review.decision_reason || '该评审已结束'}</span></div>}
      </aside>
    </div>
    <details className="kanon-technical-details kanon-review-proof"><summary>版本与完整性校验</summary><code>{review.candidate_content_hash}</code></details>
  </section>
}

function StrategyDecisionBrief({ document, review }: { document: StrategyDocument; review: Review }) {
  const evidenceCount = document.evidence_refs?.length ?? 0
  const riskCount = document.assumptions_and_gaps.length + (document.compliance?.issues.length ?? 0)
  return <section className="kanon-review-brief" aria-label="本次评审的核心决策">
    <div>
      <span className="section-label">一句话决策</span>
      <h3>{document.proposition}</h3>
      <p>{document.executive_summary || '围绕这一核心主张检查人群、渠道、证据和执行边界。'}</p>
    </div>
    <dl>
      <div><dt>核心人群</dt><dd>{document.audience.primary}</dd></div>
      <div><dt>渠道方案</dt><dd>{document.channel_strategy.length} 个</dd></div>
      <div><dt>证据引用</dt><dd>{evidenceCount} 条</dd></div>
      <div><dt>待关注</dt><dd className={riskCount ? 'warn' : ''}>{riskCount} 项</dd></div>
    </dl>
    <small>{review.review_mode === 'self_confirmation' ? '你的确认将发布不可变策略包' : '批准后将发布不可变策略包'}</small>
  </section>
}

function readReviewSessionDraft(key: string): ReviewSessionDraft | null {
  const value = readWorkspaceSessionValue<Partial<ReviewSessionDraft>>(key)
  return value && typeof value.comment === 'string' && typeof value.reason === 'string'
    ? { comment: value.comment, reason: value.reason }
    : null
}

function StrategyPerspectivePanel({ analysis, busy, error, onRun, revision }: {
  analysis: WorkspaceState['deepReview']
  busy: string
  error: string
  onRun: () => Promise<boolean>
  revision: number
}) {
  return <section className="kanon-deep-review">
    <div className="surface-toolbar"><div><h3>AI 第二视角</h3><small>针对 Revision {revision} 检查证据、渠道协同、衡量方式与执行风险；不改变人工评审状态</small></div>{analysis?.status === 'succeeded' ? <span className="source-chip">已完成 · {analysis.model_version}</span> : null}</div>
    {error && !analysis ? <DeepReviewFailure busy={busy} message={error} onRetry={onRun}/> : !analysis ? <div className="kanon-deep-review-empty"><p>这是可选的异步建议，不会阻止继续编辑、提交或确认。结果只绑定当前 Revision。</p><button className="secondary-button" disabled={Boolean(busy)} onClick={() => void onRun()}><Sparkles size={14}/>{busy === 'deep-review' ? '正在启动…' : '运行第二视角检查'}</button></div> : analysis.status === 'pending' ? <div className="kanon-deep-review-pending" role="status"><LoaderCircle className="spin" size={16}/><span><b>第二视角正在后台检查 Revision {analysis.candidate_revision}</b><small>可以切换阶段或继续工作；完成后会自动恢复结果，不影响人工评审。</small></span></div> : analysis.status === 'failed' ? <DeepReviewFailure busy={busy} message={error || '本次模型任务未完成，请稍后重试。'} onRetry={onRun}/> : <><p className="kanon-deep-review-summary">{analysis.summary}</p><div className="kanon-deep-findings">{analysis.findings.map((finding, index) => <article className={finding.severity} key={`${finding.section}-${index}`}><header><span>{finding.severity === 'blocker' ? '阻断风险' : finding.severity === 'warning' ? '需要关注' : '优化机会'}</span><small>{strategySectionLabel(finding.section)}</small></header><h4>{finding.title}</h4><p>{finding.detail}</p><div><b>建议</b>{finding.recommendation}</div></article>)}</div><details className="kanon-technical-details"><summary>查看运行信息</summary><code>revision={analysis.candidate_revision} · {analysis.api_mode} · background={String(analysis.background)} · {analysis.latency_ms ?? 0} ms{analysis.usage ? ` · ${analysis.usage.total_tokens} tokens` : ''}</code></details></>}
  </section>
}

function DeepReviewFailure({ busy, message, onRetry }: {
  busy: string
  message: string
  onRetry: () => Promise<boolean>
}) {
  return <div className="kanon-deep-review-failed" role="status"><AlertCircle size={17}/><span><b>第二视角暂时没有完成</b><small>策略内容和人工评审状态均未改变，你可以继续工作或稍后重试。</small><details className="kanon-technical-details"><summary>查看技术原因</summary><code>{message}</code></details></span><button className="secondary-button" disabled={Boolean(busy)} onClick={() => void onRetry()}><RefreshCw size={14}/>重新运行</button></div>
}

type ResearchComposerSessionDraft = {
  category: 'general' | 'audience' | 'competitor' | 'industry'
  editedProposalValue: string
  editingProposalId: string
  query: string
  selectedDocumentIds: string[]
}

function ResearchPane({
	brief,
	busy,
	contextError,
	contextReady,
	documents,
	draft,
	onApplyProposal,
	onCancelResearch,
	onIgnoreProposal,
	onRemapProposal,
	onResearch,
	onRetryResearch,
	onUpload,
	proposalBusyId,
	proposalError,
	proposals,
	researchRun,
	sessionProjectId,
	sessionWorkspaceId,
}: {
  brief: BriefDraft | null
  busy: string
	contextError: string
	contextReady: boolean
  documents: WorkspaceState['documents']
	draft: StrategyDraft | null
  onResearch: (
    category: 'general' | 'audience' | 'competitor' | 'industry',
    query: string,
    documentIds: string[],
  ) => Promise<boolean>
	onApplyProposal: (proposal: ArtifactProposal, operations?: BriefPatchOperation[]) => Promise<boolean>
	onIgnoreProposal: (proposal: ArtifactProposal) => Promise<boolean>
	onRemapProposal: (proposal: ArtifactProposal) => Promise<boolean>
	onCancelResearch: (researchRunId: string) => Promise<boolean>
	onRetryResearch: (researchRunId: string) => Promise<boolean>
  onUpload: (file: File) => Promise<boolean>
	proposalBusyId: string
	proposalError: string
	proposals: ArtifactProposal[]
  researchRun: WorkspaceState['researchRun']
  sessionProjectId: string
  sessionWorkspaceId: string
}) {
  const composerStorageKey = workspaceSessionKey(sessionProjectId, sessionWorkspaceId, 'research-composer')
  const [restoredDraft] = useState<ResearchComposerSessionDraft>(() => readResearchComposerDraft(composerStorageKey) ?? {
    category: 'general',
    editedProposalValue: '',
    editingProposalId: '',
    query: '',
    selectedDocumentIds: [],
  })
  const [query, setQuery] = useState(restoredDraft.query)
  const [category, setCategory] = useState<ResearchComposerSessionDraft['category']>(restoredDraft.category)
  const [selectedDocumentIds, setSelectedDocumentIds] = useState<string[]>(restoredDraft.selectedDocumentIds)
	const [editingProposalId, setEditingProposalId] = useState(restoredDraft.editingProposalId)
	const [editedProposalValue, setEditedProposalValue] = useState(restoredDraft.editedProposalValue)
	const [proposalEditError, setProposalEditError] = useState('')
	const activeStatuses = new Set(['queued', 'planning', 'searching', 'reading', 'cross_checking', 'drafting', 'auditing'])
	const active = Boolean(researchRun && activeStatuses.has(researchRun.status))
  useEffect(() => {
    const readyIDs = new Set(documents.filter(document => document.status === 'ready').map(document => document.id))
    setSelectedDocumentIds(current => current.filter(id => readyIDs.has(id)))
  }, [documents])
  useEffect(() => {
    const value: ResearchComposerSessionDraft = {
      category,
      editedProposalValue: editingProposalId ? editedProposalValue : '',
      editingProposalId,
      query,
      selectedDocumentIds,
    }
    if (category === 'general' && !query && !selectedDocumentIds.length && !editingProposalId) {
      clearWorkspaceSessionValue(composerStorageKey)
      return
    }
    writeWorkspaceSessionValue(composerStorageKey, value)
  }, [category, composerStorageKey, editedProposalValue, editingProposalId, query, selectedDocumentIds])
  return <section className="kanon-research-pane">
    <div className="kanon-strategy-heading">
	  <div><span className="section-label">DEEP RESEARCH</span><h2>把外部与内部资料变成决策依据</h2><p>后台多轮搜索、正文核验和交叉引用；只有带目标字段与预览差异的结论才可采纳。</p></div>
      <span className="source-chip">{documents.length} 份项目资料</span>
    </div>
    <div className="kanon-research-grid">
      <section>
        <div className="surface-toolbar"><h3>品牌与项目资料</h3><label className="secondary-button" htmlFor="kanon-knowledge-file"><Upload size={14}/>上传资料</label></div>
		<input id="kanon-knowledge-file" type="file" accept=".md,.docx,.pdf" disabled={Boolean(busy)} onChange={event => { const file = event.target.files?.[0]; if (file) void onUpload(file) }}/>
        {documents.map(document => {
          const ready = document.status === 'ready'
          const selected = selectedDocumentIds.includes(document.id)
          return <article className={selected ? 'selected' : ''} key={document.id}>
            <FileText size={17}/>
            <span>
              <b>{document.title || document.filename}</b>
              <small>{document.source_type || document.mime_type} · {formatBytes(document.size_bytes)} · {documentStatusLabel(document)}</small>
            </span>
            <label className="kanon-document-select">
              <input
                aria-label={`选择资料 ${document.title || document.filename}`}
                checked={selected}
				disabled={!ready || Boolean(busy)}
                type="checkbox"
                onChange={event => setSelectedDocumentIds(current => event.target.checked
                  ? [...current, document.id]
                  : current.filter(id => id !== document.id))}
              />
              <span>{ready ? '用于本次研究' : '解析完成后可选'}</span>
            </label>
          </article>
        })}
        {!documents.length ? <div className="panel-empty">尚未导入品牌或项目资料。</div> : null}
      </section>
      <section className="kanon-research-form">
		<div className="surface-toolbar"><h3>发起深度研究</h3><span>后台运行 · 可继续编辑</span></div>
		<label>研究分类<select disabled={Boolean(busy)} value={category} onChange={event => setCategory(event.target.value as typeof category)}><option value="general">综合研究</option><option value="audience">受众研究</option><option value="competitor">竞品研究</option><option value="industry">行业研究</option></select></label>
		<label>要验证的问题<textarea disabled={Boolean(busy)} rows={4} value={query} onChange={event => setQuery(event.target.value)} placeholder="例如：近半年小红书工业品牌内容的有效切入点是什么？"/></label>
        <p className="kanon-research-disclosure">未发起的问题、已选资料和采纳编辑仅在当前浏览器会话保留。开始研究会把当前问题发送给 Seed 并记录披露字段；项目文件不会自动发送，若勾选资料，仅披露本地检索命中的最多 8 个片段，并记录实际片段 ID。</p>
		{!contextReady ? <p className="kanon-research-context-status" role={contextError ? 'alert' : 'status'}>{contextError ? `项目上下文暂不可用：${contextError}` : '正在固定当前 Brief、策略、资料和记忆版本…'}</p> : null}
		<button className="primary-button" disabled={Boolean(busy) || !query.trim() || !contextReady} onClick={() => void onResearch(category, query, selectedDocumentIds)}><Search size={15}/>{busy === 'research' ? '正在创建任务…' : !contextReady ? '正在准备项目上下文…' : '开始深度研究'}</button>
      </section>
    </div>
	{researchRun ? <div className="kanon-research-v2">
		<header className="kanon-research-run-header">
			<div><span className="section-label">RESEARCH RUN · {researchRun.id.slice(0, 12)}</span><h3>{researchRun.query}</h3><p>{researchStatusCopy(researchRun.status)}</p></div>
			<div className="kanon-research-run-actions">
				<span className={`source-chip ${researchRun.status === 'failed' || researchRun.status === 'partially_completed' ? 'alert' : ''}`}>{researchStatusLabel(researchRun.status)}</span>
				{active ? <button className="secondary-button" disabled={Boolean(busy)} onClick={() => void onCancelResearch(researchRun.id)}>取消研究</button> : null}
				{researchRun.status === 'failed' || researchRun.status === 'cancelled' || researchRun.status === 'partially_completed' ? <button className="secondary-button" disabled={Boolean(busy)} onClick={() => void onRetryResearch(researchRun.id)}><RotateCcw size={13}/>从断点重试</button> : null}
			</div>
		</header>
		<div className="kanon-research-metrics" aria-label="研究进度摘要">
			<div><small>循环轮次</small><b>{researchRun.current_round} / {researchRun.max_rounds}</b><span>{active ? researchStatusLabel(researchRun.status) : researchRun.stop_reason || '—'}</span></div>
			<div><small>来源</small><b>{uniqueResearchSources(researchRun).length}</b><span>{countVerifiedResearchSources(researchRun)} 个正文已核验</span></div>
			<div><small>已确认结论</small><b>{researchRun.findings.filter(item => item.status === 'verified').length}</b><span>{researchRun.findings.filter(item => item.status === 'conflicting').length} 条存在冲突</span></div>
			<div><small>待补缺口</small><b>{researchRun.open_gaps.length}</b><span>{researchRun.usage?.total_tokens ? `${researchRun.usage.total_tokens.toLocaleString()} tokens` : '按预算有界执行'}</span></div>
		</div>
		<section className="kanon-research-timeline" aria-label="研究循环时间线">
			<div className="surface-toolbar"><h3>执行过程</h3><span>每轮均已持久化，可从断点恢复</span></div>
			{researchRun.iterations.map(iteration => <article key={iteration.id} data-status={iteration.status}>
				<span className="kanon-research-round">{iteration.round}</span>
				<div><b>第 {iteration.round} 轮 · {iteration.objective}</b><p>{iteration.action_summary || '完成本轮搜索、读取与结构化校验。'}</p><small>{iteration.source_ids.length} 个来源 · {iteration.finding_ids.length} 条发现 · {iteration.open_gaps.length} 个缺口</small></div>
				<CircleCheck size={16}/>
			</article>)}
			{active ? <article data-status="running"><span className="kanon-research-round"><LoaderCircle className="spin" size={14}/></span><div><b>{researchStatusLabel(researchRun.status)}</b><p>{researchStatusCopy(researchRun.status)}</p></div></article> : null}
		</section>
		<section className="kanon-finding-ledger">
			<div className="surface-toolbar"><h3>结论与决策影响</h3><span>{researchRun.findings.length} 条结构化发现</span></div>
			{researchRun.findings.map(finding => {
				const proposal = proposals.find(item => item.finding_ids?.includes(finding.id))
				const operation = proposal?.operations[0]
				const editing = proposal?.id === editingProposalId
				const currentValue = operation ? currentResearchTargetValue(brief, draft, proposal, operation.field_path) : undefined
				return <article className="kanon-finding-card" data-status={finding.status} key={finding.id}>
					<header><span className={`source-chip ${finding.status === 'conflicting' ? 'alert' : ''}`}>{researchFindingStatusLabel(finding.status)}</span><small>第 {finding.round} 轮 · {finding.confidence} 置信度 · {finding.time_scope}</small></header>
					<h4>{finding.claim}</h4>
					<p>{finding.implication}</p>
					<div className="kanon-finding-evidence"><span>{finding.supporting_source_ids.length} 个支持来源</span><span>{finding.conflicting_source_ids.length} 个反对来源</span><span>目标：{researchTargetLabel(finding.target.artifact, finding.target.field_path)}</span></div>
					{proposal && operation ? <div className="kanon-research-proposal" data-status={proposal.status}>
						<div className="kanon-research-diff"><div><small>当前值</small><p>{formatResearchValue(currentValue)}</p></div><ChevronRight size={16}/><div><small>研究建议</small><p>{formatResearchValue(operation.value)}</p></div></div>
						{proposal.status === 'stale' ? <div className="kanon-proposal-stale"><AlertCircle size={14}/><span><b>目标内容已变化</b><small>{proposal.stale_reason}</small></span><button className="secondary-button" disabled={proposalBusyId === proposal.id} onClick={() => void onRemapProposal(proposal)}>重新映射</button></div> : null}
						{editing ? <div className="kanon-research-proposal-editor"><label>编辑后采纳<textarea rows={4} value={editedProposalValue} onChange={event => { setEditedProposalValue(event.target.value); setProposalEditError('') }}/></label>{proposalEditError ? <small className="kanon-research-edit-error" role="alert">{proposalEditError}</small> : null}<div><button className="secondary-button" onClick={() => { setEditingProposalId(''); setEditedProposalValue(''); setProposalEditError('') }}>取消</button><button className="primary-button" disabled={proposalBusyId === proposal.id} onClick={() => {
							const edited = parseResearchEditedValue(operation.value, editedProposalValue)
							if (edited === undefined) {
								setProposalEditError('请输入有效的 JSON 对象或数组；文本值可直接填写。')
								return
							}
							setProposalEditError('')
							void onApplyProposal(proposal, [{ ...operation, value: edited }]).then(ok => { if (ok) { setEditingProposalId(''); setEditedProposalValue('') } })
						}}>采纳修改值</button></div></div> : null}
						{proposal.status === 'proposed' && !editing ? <footer><button className="primary-button" disabled={proposalBusyId === proposal.id} onClick={() => void onApplyProposal(proposal)}>采纳</button><button className="secondary-button" disabled={proposalBusyId === proposal.id} onClick={() => { setEditingProposalId(proposal.id); setEditedProposalValue(editableResearchValue(operation.value)); setProposalEditError('') }}>编辑后采纳</button><button className="secondary-button" disabled={proposalBusyId === proposal.id} onClick={() => void onIgnoreProposal(proposal)}>忽略</button></footer> : null}
						{proposal.status === 'applied' || proposal.status === 'edited' ? <small className="kanon-proposal-applied"><Check size={13}/>已由用户确认并写入新版本</small> : null}
						{proposal.status === 'ignored' ? <small>已忽略，不会写入业务对象。</small> : null}
					</div> : <div className="kanon-finding-no-proposal">{finding.status === 'tentative' ? '仅有模型引用或独立来源不足，暂不可采纳。' : finding.status === 'invalid' ? '结构或目标字段校验未通过。' : '当前 Brief/策略版本没有可安全应用的目标；报告保留该结论但不会自动改写。'}</div>}
				</article>
			})}
			{!researchRun.findings.length ? <div className="panel-empty">研究尚未形成结构化结论；可以继续做其他工作，结果会在后台更新。</div> : null}
			{proposalError ? <div className="kanon-strategy-alert" role="alert"><AlertCircle size={14}/>{proposalError}</div> : null}
		</section>
		{researchRun.report_artifact_id ? <ResearchReport run={researchRun}/> : null}
		{researchRun.error_message ? <div className="kanon-strategy-alert" role="alert"><AlertCircle size={14}/>{researchRun.error_message}</div> : null}
	</div> : null}
  </section>
}

function readResearchComposerDraft(key: string): ResearchComposerSessionDraft | null {
  const value = readWorkspaceSessionValue<Partial<ResearchComposerSessionDraft>>(key)
  const categories = new Set(['general', 'audience', 'competitor', 'industry'])
  if (!value || typeof value.category !== 'string' || !categories.has(value.category)) return null
  if (typeof value.query !== 'string' || typeof value.editingProposalId !== 'string' || typeof value.editedProposalValue !== 'string') return null
  if (!Array.isArray(value.selectedDocumentIds) || !value.selectedDocumentIds.every(id => typeof id === 'string')) return null
  return {
    category: value.category as ResearchComposerSessionDraft['category'],
    editedProposalValue: value.editedProposalValue,
    editingProposalId: value.editingProposalId,
    query: value.query,
    selectedDocumentIds: value.selectedDocumentIds,
  }
}

const strategyEditorSectionGroups = [
  { id: 'decision', title: '核心判断', description: '方向与取舍', sections: ['executive_summary', 'objective', 'audience', 'proposition'] },
  { id: 'channel', title: '渠道协同', description: '平台分工与转化', sections: ['cross_platform_role', 'channel_strategy', 'platform_plans'] },
  { id: 'creative', title: '创意策略', description: '创意母题与适配', sections: ['creative_strategy', 'creative_recommendations'] },
  { id: 'validation', title: '验证与治理', description: '约束、实验与证据', sections: ['constraints', 'budget_and_cadence', 'experiment_matrix', 'measurement', 'assumptions_and_gaps', 'evidence_refs'] },
]

function StrategySectionChangeImpact({ busy, nextRevision, onReset, onSave, original, section, value }: {
  busy: boolean
  nextRevision: number
  onReset: () => void
  onSave: () => Promise<boolean>
  original: unknown
  section: string
  value: unknown
}) {
  return <section className="kanon-strategy-change-impact" aria-label="保存影响确认">
    <header><span><Sparkles size={15}/><span><b>待保存变更</b><small>{strategyChangeSummary(original, value)}</small></span></span><strong>Revision {nextRevision}</strong></header>
    <ul>
      <li>只更新“{strategySectionLabel(section)}”章节，其他章节保持不变</li>
      <li>创建新 Revision；开放中的评审将失效并需要重新提交</li>
      <li>历史 Revision 与已发布 StrategyPackage 保持不可变</li>
    </ul>
    <details><summary>查看保存前后对比</summary><div><section><small>当前版本</small><ReviewValue value={original}/></section><section><small>保存后</small><ReviewValue value={value}/></section></div></details>
    <footer><button className="text-button" disabled={busy} onClick={onReset} type="button">放弃本次修改</button><button className="primary-button" disabled={busy} onClick={() => void onSave()} type="button">{busy ? '保存中…' : `确认并创建 Revision ${nextRevision}`}</button></footer>
  </section>
}

function strategyChangeSummary(before: unknown, after: unknown) {
  if (Array.isArray(before) && Array.isArray(after)) return `条目从 ${before.length} 项变为 ${after.length} 项`
  if (typeof before === 'string' && typeof after === 'string') return `文本从 ${before.length} 字调整为 ${after.length} 字`
  if (before && after && typeof before === 'object' && typeof after === 'object') {
    const beforeRecord = before as Record<string, unknown>
    const afterRecord = after as Record<string, unknown>
    const keys = new Set([...Object.keys(beforeRecord), ...Object.keys(afterRecord)])
    const changedKeys = [...keys].filter(key => JSON.stringify(beforeRecord[key]) !== JSON.stringify(afterRecord[key]))
    return `${changedKeys.length} 个字段有调整：${changedKeys.map(strategyFieldLabel).join('、')}`
  }
  return '章节内容已调整'
}

function CreativeStrategyEditor({ disabled, onChange, value }: {
  disabled: boolean
  onChange: (value: unknown) => void
  value: unknown
}) {
  const creative = (value && typeof value === 'object' ? value : {}) as Partial<CreativeStrategy>
  const normalized: CreativeStrategy = {
    objective: creative.objective ?? '',
    message_hierarchy: creative.message_hierarchy ?? [],
    territories: creative.territories ?? [],
    tone: creative.tone ?? [],
    mandatories: creative.mandatories ?? [],
    avoidances: creative.avoidances ?? [],
  }
  const update = <K extends keyof CreativeStrategy,>(key: K, next: CreativeStrategy[K]) => onChange({ ...normalized, [key]: next })
  return <div className="kanon-creative-strategy-editor">
    <section className="kanon-creative-strategy-foundation">
      <label className="kanon-structured-field wide"><span>创意任务目标<small>创意要帮助策略完成什么</small></span><textarea disabled={disabled} rows={3} value={normalized.objective} onChange={event => update('objective', event.target.value)}/></label>
      <StrategyStringList disabled={disabled} label="信息层级" note="按优先级每行一项" onChange={value => update('message_hierarchy', value)} value={normalized.message_hierarchy}/>
    </section>
    <section className="kanon-creative-territories">
      <header><div><h4>创意母题</h4><small>每个母题都要包含受众张力、核心想法、证据和渠道适配。</small></div><button className="secondary-button" disabled={disabled} onClick={() => update('territories', [...normalized.territories, { name: '', audience_tension: '', core_idea: '', proof: [], channel_adaptations: [] }])} type="button"><Plus size={14}/>新增母题</button></header>
      {normalized.territories.map((territory, territoryIndex) => <article key={`territory-${territoryIndex}`}>
        <header><span>{String(territoryIndex + 1).padStart(2, '0')}</span><div><b>{territory.name || `未命名母题 ${territoryIndex + 1}`}</b><small>{territory.core_idea || '填写一个可被团队执行的核心想法'}</small></div><button className="text-button danger" disabled={disabled} onClick={() => update('territories', normalized.territories.filter((_, index) => index !== territoryIndex))} type="button">删除</button></header>
        <div className="kanon-structured-grid">
          <label className="kanon-structured-field"><span>母题名称</span><input disabled={disabled} value={territory.name} onChange={event => updateCreativeTerritory(normalized, territoryIndex, { ...territory, name: event.target.value }, onChange)}/></label>
          <label className="kanon-structured-field"><span>受众张力</span><input disabled={disabled} value={territory.audience_tension} onChange={event => updateCreativeTerritory(normalized, territoryIndex, { ...territory, audience_tension: event.target.value }, onChange)}/></label>
          <label className="kanon-structured-field wide"><span>核心想法</span><textarea disabled={disabled} rows={3} value={territory.core_idea} onChange={event => updateCreativeTerritory(normalized, territoryIndex, { ...territory, core_idea: event.target.value }, onChange)}/></label>
          <StrategyStringList disabled={disabled} label="可用证据" note="不要把假设写成事实" onChange={proof => updateCreativeTerritory(normalized, territoryIndex, { ...territory, proof }, onChange)} value={territory.proof}/>
        </div>
        <section className="kanon-channel-adaptations"><header><b>渠道适配</b><button className="text-button" disabled={disabled} onClick={() => updateCreativeTerritory(normalized, territoryIndex, { ...territory, channel_adaptations: [...territory.channel_adaptations, { platform: '', role: '', adaptation: '', formats: [] }] }, onChange)} type="button"><Plus size={13}/>新增渠道</button></header>
          {territory.channel_adaptations.map((adaptation, adaptationIndex) => <div key={`adaptation-${adaptationIndex}`}>
            <input aria-label="平台" disabled={disabled} placeholder="平台" value={adaptation.platform} onChange={event => updateCreativeAdaptation(normalized, territoryIndex, adaptationIndex, { ...adaptation, platform: event.target.value }, onChange)}/>
            <input aria-label="渠道角色" disabled={disabled} placeholder="渠道角色" value={adaptation.role} onChange={event => updateCreativeAdaptation(normalized, territoryIndex, adaptationIndex, { ...adaptation, role: event.target.value }, onChange)}/>
            <textarea aria-label="适配方式" disabled={disabled} placeholder="这个母题在该渠道如何成立" rows={2} value={adaptation.adaptation} onChange={event => updateCreativeAdaptation(normalized, territoryIndex, adaptationIndex, { ...adaptation, adaptation: event.target.value }, onChange)}/>
            <input aria-label="内容形式" disabled={disabled} placeholder="形式，用顿号分隔" value={(adaptation.formats ?? []).join('、')} onChange={event => updateCreativeAdaptation(normalized, territoryIndex, adaptationIndex, { ...adaptation, formats: splitValues(event.target.value) }, onChange)}/>
            <button aria-label="删除渠道适配" className="text-button danger" disabled={disabled} onClick={() => updateCreativeTerritory(normalized, territoryIndex, { ...territory, channel_adaptations: territory.channel_adaptations.filter((_, index) => index !== adaptationIndex) }, onChange)} type="button">删除</button>
          </div>)}
        </section>
      </article>)}
    </section>
    <section className="kanon-creative-guardrails">
      <StrategyStringList disabled={disabled} label="表达调性" note="每行一项" onChange={value => update('tone', value)} value={normalized.tone}/>
      <StrategyStringList disabled={disabled} label="必须包含" note="来自已确认 Brief" onChange={value => update('mandatories', value)} value={normalized.mandatories}/>
      <StrategyStringList disabled={disabled} label="避免事项" note="合规与品牌边界" onChange={value => update('avoidances', value)} value={normalized.avoidances}/>
    </section>
  </div>
}

function StrategyStringList({ disabled, label, note, onChange, value }: { disabled: boolean; label: string; note: string; onChange: (value: string[]) => void; value: string[] }) {
  return <label className="kanon-structured-field wide"><span>{label}<small>{note}</small></span><textarea disabled={disabled} rows={Math.max(3, Math.min(7, value.length + 2))} value={value.join('\n')} onChange={event => onChange(event.target.value.split('\n').map(item => item.trim()).filter(Boolean))}/></label>
}

function updateCreativeTerritory(creative: CreativeStrategy, index: number, territory: CreativeStrategy['territories'][number], onChange: (value: unknown) => void) {
  onChange({ ...creative, territories: creative.territories.map((current, currentIndex) => currentIndex === index ? territory : current) })
}

function updateCreativeAdaptation(creative: CreativeStrategy, territoryIndex: number, adaptationIndex: number, adaptation: CreativeStrategy['territories'][number]['channel_adaptations'][number], onChange: (value: unknown) => void) {
  const territory = creative.territories[territoryIndex]
  updateCreativeTerritory(creative, territoryIndex, { ...territory, channel_adaptations: territory.channel_adaptations.map((current, currentIndex) => currentIndex === adaptationIndex ? adaptation : current) }, onChange)
}

function ResearchReport({ run }: { run: NonNullable<WorkspaceState['researchRun']> }) {
	const report = run.artifacts.find(artifact => artifact.id === run.report_artifact_id)
	if (!report) return null
	return <section className="kanon-research-report">
		<div className="surface-toolbar"><div><h3>研究报告</h3><small>停止原因：{run.stop_reason || '—'}</small></div><div><span>{report.sources.length} 个引用来源</span><button className="secondary-button" onClick={() => downloadResearchReport(report, run.id)} type="button"><Download size={13}/>下载报告</button></div></div>
		<div className="kanon-research-report-body"><BookOpen size={18}/><p>{report.content}</p></div>
		<div className="kanon-research-sources">{report.sources.map(source => <a href={source.url} key={`${source.id}-${source.start_index}-${source.end_index}`} rel="noreferrer" target="_blank"><ExternalLink size={12}/>{source.title || source.domain}<small>{researchSourceStatusLabel(source.verification_status)}</small></a>)}</div>
	</section>
}

function downloadResearchReport(report: NonNullable<WorkspaceState['researchRun']>['artifacts'][number], runId: string) {
  const sources = report.sources.map((source, index) =>
    `${index + 1}. ${source.title || source.domain} — ${source.url} — ${researchSourceStatusLabel(source.verification_status)}`,
  )
  const content = [report.content, '', '---', '', '## 引用来源', '', ...sources].join('\n')
  const url = URL.createObjectURL(new Blob([content], { type: 'text/markdown;charset=utf-8' }))
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = `research-report-${runId}.md`
  document.body.appendChild(anchor)
  anchor.click()
  anchor.remove()
  window.setTimeout(() => URL.revokeObjectURL(url), 0)
}

function researchStatusLabel(status: NonNullable<WorkspaceState['researchRun']>['status']) {
	return ({ queued: '等待执行', planning: '规划研究', searching: '联网搜索', reading: '读取来源', cross_checking: '交叉核验', drafting: '生成报告', auditing: '引用审计', completed: '研究完成', partially_completed: '部分完成', failed: '研究失败', cancelled: '已取消' } as const)[status]
}

function researchStatusCopy(status: NonNullable<WorkspaceState['researchRun']>['status']) {
	return ({ queued: '任务已进入后台队列，不影响继续编辑 Brief 或策略。', planning: '正在固定研究问题、上下文快照和预算边界。', searching: '正在按当前高影响缺口搜索候选来源。', reading: '正在读取来源正文并核对可引用片段。', cross_checking: '正在比较独立来源、记录支持与冲突信号。', drafting: '正在把发现组织成报告和字段级建议。', auditing: '正在检查每个结论的引用与目标映射。', completed: '已形成可核验、可定位且可预览采纳的结论。', partially_completed: '已保留可用结果，并明确记录未完成原因与缺口。', failed: '没有生成无来源结论；可以从断点重试。', cancelled: '研究已停止，已保存的轮次和结论仍然可见。' } as const)[status]
}

function researchFindingStatusLabel(status: NonNullable<WorkspaceState['researchRun']>['findings'][number]['status']) {
	return ({ tentative: '待核验', verified: '交叉核验通过', conflicting: '存在冲突', invalid: '不可用' } as const)[status]
}

function researchSourceStatusLabel(status: NonNullable<WorkspaceState['researchRun']>['artifacts'][number]['sources'][number]['verification_status']) {
	return ({ model_cited: '模型引用 · 未核验正文', content_verified: '正文与摘录已核验', conflicted: '冲突来源', invalid: '来源无效' } as const)[status]
}

function uniqueResearchSources(run: NonNullable<WorkspaceState['researchRun']>) {
	const values = new Map<string, NonNullable<WorkspaceState['researchRun']>['artifacts'][number]['sources'][number]>()
	for (const artifact of run.artifacts) for (const source of artifact.sources) values.set(source.id, source)
	return [...values.values()]
}

function countVerifiedResearchSources(run: NonNullable<WorkspaceState['researchRun']>) {
	return uniqueResearchSources(run).filter(source => source.verification_status === 'content_verified' || source.verification_status === 'conflicted').length
}

function researchTargetLabel(artifact: 'brief' | 'strategy', fieldPath: string) {
	return `${artifact === 'brief' ? 'Brief' : '策略'} · ${fullStrategyFieldLabels[fieldPath] ?? strategySectionLabel(fieldPath)}`
}

function currentResearchTargetValue(brief: BriefDraft | null, draft: StrategyDraft | null, proposal: ArtifactProposal, fieldPath: string) {
	if (proposal.target_type === 'brief_draft') return briefFieldValue(brief?.document, fieldPath)
	return draft?.revision?.document?.[fieldPath as keyof StrategyDocument]
}

function briefFieldValue(document: BriefDraft['document'] | undefined, fieldPath: string): unknown {
	if (!document) return undefined
	return fieldPath.split('.').reduce<unknown>((value, key) => value && typeof value === 'object' ? (value as Record<string, unknown>)[key] : undefined, document)
}

function formatResearchValue(value: unknown) {
	if (value === undefined || value === null || value === '') return '尚未填写'
	if (Array.isArray(value)) return value.length ? value.map(item => typeof item === 'string' ? item : JSON.stringify(item)).join('；') : '空列表'
	if (typeof value === 'object') return JSON.stringify(value, null, 2)
	return String(value)
}

function editableResearchValue(value: unknown) {
	if (Array.isArray(value)) return value.map(item => typeof item === 'string' ? item : JSON.stringify(item)).join('\n')
	if (value && typeof value === 'object') return JSON.stringify(value, null, 2)
	return value == null ? '' : String(value)
}

function parseResearchEditedValue(original: unknown, edited: string): unknown | undefined {
	if (Array.isArray(original)) return edited.split('\n').map(value => value.trim()).filter(Boolean)
	if (original && typeof original === 'object') {
		try { return JSON.parse(edited) } catch { return undefined }
	}
	return edited.trim()
}

function ChangeLogPane({ comments, review, revisions }: {
  comments: WorkspaceState['comments']
  review: Review | null
  revisions: DraftRevision[]
}) {
  const items = [
    ...revisions.map(revision => ({
      id: `revision-${revision.revision}`,
      title: `Strategy Revision ${revision.revision}`,
      detail: `${revision.changed_sections.map(strategySectionLabel).join('、')} · ${revision.content_hash.slice(0, 16)}`,
      kind: '策略修订',
    })),
    ...comments.map(comment => ({
      id: comment.id,
      title: `${comment.author_id} 添加评审评论`,
      detail: comment.body,
      kind: formatTime(comment.created_at),
    })),
  ]
  return <section className="kanon-change-log">
    <div className="kanon-strategy-heading"><div><span className="section-label">CURRENT CHAIN</span><h2>当前工作链变更记录</h2><p>只展示服务端已有 revisions 和 review comments。</p></div>{review ? <span className="source-chip">{statusLabel(review.status)}</span> : null}</div>
    {items.map(item => <article key={item.id}><span/><div><small>{item.kind}</small><b>{item.title}</b><p>{item.detail}</p></div></article>)}
    {!items.length ? <div className="panel-empty">当前工作链尚无修订或评审评论。</div> : null}
  </section>
}

function SummaryRail({ brief, briefVersion, draft, publishedVersion, review, workspaceName }: {
  brief: BriefDraft | null
  briefVersion?: number
  draft: StrategyDraft | null
  publishedVersion?: number
  review: Review | null
  workspaceName: string
}) {
  const items = [
    ['工作区', workspaceName],
    ['Brief', brief ? `${brief.status === 'confirmed' ? '已冻结' : '草稿'} v${brief.status === 'confirmed' ? briefVersion ?? 1 : brief.version}` : '未创建'],
    ['完整度', brief ? brief.status === 'confirmed' ? '已确认' : brief.completeness.ready ? '可以确认' : `${brief.completeness.blockers.length} 个阻断项` : '—'],
    ['Strategy', draft ? `Revision ${draft.current_revision} · ${statusLabel(draft.status)}` : '未生成'],
    ['Review', review ? statusLabel(review.status) : '未提交'],
    ['Package', publishedVersion ? `已发布 v${publishedVersion}` : '未发布'],
  ]
  return <aside className="kanon-summary-rail">
    <div className="surface-toolbar"><h3>对象摘要</h3><span>真实状态</span></div>
    {items.map(([label, value]) => <div className="kv" key={label}><span>{label}</span><b>{value}</b></div>)}
    <div className="kanon-summary-checks">
      <b>发布检查</b>
      {[
        ['Brief 已冻结', brief?.status === 'confirmed'],
        ['策略 Revision', Boolean(draft?.current_revision)],
        ['人工确认/批准', review?.status === 'approved'],
        ['不可变策略包', Boolean(publishedVersion)],
      ].map(([label, complete]) => <div key={String(label)}>{complete ? <CircleCheck size={14}/> : <AlertCircle size={14}/>}<span>{label}</span></div>)}
    </div>
  </aside>
}

function EvidenceRail({ documents, referenceIds, researchArtifacts }: {
  documents: WorkspaceState['documents']
  referenceIds: string[]
  researchArtifacts: NonNullable<WorkspaceState['researchRun']>['artifacts']
}) {
  const referencedDocuments = documents.filter(document => referenceIds.includes(document.id))
  const artifacts = researchArtifacts.filter(artifact => referenceIds.includes(artifact.id))
  return <aside className="kanon-evidence-rail">
    <div className="surface-toolbar"><h3>证据</h3><span>{referenceIds.length} 个引用</span></div>
    {referencedDocuments.map(document => <article key={document.id}><span className="evidence-id">DOC</span><div><b>{document.title || document.filename}</b><small>{document.source_type || '项目资料'}</small><small>{document.id}</small></div></article>)}
    {artifacts.map(artifact => <article key={artifact.id}><span className="evidence-id">R</span><div><b>{artifact.title}</b><small>{artifact.citations[0] || '研究产物'}</small><small>{artifact.content_hash.slice(0, 18)}</small></div></article>)}
    {!referenceIds.length ? <div className="panel-empty">Brief 尚未引用资料或研究产物。</div> : null}
  </aside>
}

function UnavailablePane({ detail, title }: { detail: string; title: string }) {
  return <div className="kanon-strategy-state"><AlertCircle size={24}/><h2>{title}</h2><p>{detail}</p></div>
}

function strategyDiff(base?: DraftRevision, candidate?: DraftRevision) {
  if (!candidate) return []
  const before = base?.document ?? {} as StrategyDocument
  const sections = candidate.changed_sections.includes('all')
    ? Object.keys(candidate.document).filter(key => key !== 'contract_version')
    : candidate.changed_sections
  return sections.map(section => ({
    section,
    before: (before as unknown as Record<string, unknown>)[section],
    after: (candidate.document as unknown as Record<string, unknown>)[section],
  })).filter(item => JSON.stringify(item.before) !== JSON.stringify(item.after))
}

function splitValues(value: string) {
  return value.split(/[\n,，、]/).map(item => item.trim()).filter(Boolean)
}

function ReviewValue({ emptyLabel = '—', value }: { emptyLabel?: string; value: unknown }) {
  if (value === undefined || value === null || value === '') {
    return <p className="kanon-review-empty-value">{emptyLabel}</p>
  }
  if (Array.isArray(value)) {
    if (!value.length) return <p className="kanon-review-empty-value">{emptyLabel}</p>
    return <ul className="kanon-review-value-list">{value.map((item, index) => <li key={reviewValueKey(item, index)}><ReviewValue value={item}/></li>)}</ul>
  }
  if (typeof value === 'object') {
    const entries = Object.entries(value as Record<string, unknown>)
      .filter(([key]) => key !== 'contract_version')
    if (!entries.length) return <p className="kanon-review-empty-value">{emptyLabel}</p>
    return <dl className="kanon-review-value-object">{entries.map(([key, fieldValue]) => <div key={key}><dt>{strategyFieldLabel(key)}</dt><dd><ReviewValue value={fieldValue}/></dd></div>)}</dl>
  }
  if (typeof value === 'boolean') return <p className="kanon-review-value-text">{value ? '是' : '否'}</p>
  return <p className="kanon-review-value-text">{String(value)}</p>
}

function reviewValueKey(value: unknown, index: number) {
  if (typeof value === 'string' || typeof value === 'number') return `${index}-${String(value).slice(0, 32)}`
  return String(index)
}

function strategySectionLabel(value: string) {
  const labels: Record<string, string> = {
    objective: '目标',
    audience: '受众',
    proposition: '核心主张',
    channel_strategy: '渠道策略',
    creative_recommendations: '创意建议',
    creative_strategy: '创意策略',
    constraints: '约束',
    budget_and_cadence: '预算与节奏',
    experiment_matrix: '实验矩阵',
    measurement: '衡量指标',
    assumptions_and_gaps: '假设与缺口',
    executive_summary: '执行摘要',
    cross_platform_role: '跨平台协同',
    platform_plans: '分平台方案',
    evidence_refs: '证据引用',
    compliance: '合规报告',
    lineage: '生成追溯',
  }
  return labels[value] ?? value
}

function strategySectionDescription(value: string) {
  const descriptions: Record<string, string> = {
    executive_summary: '把全篇收敛为团队可以共同复述的策略判断。',
    objective: '明确策略要改变什么，以及成功意味着什么。',
    audience: '定义核心受众及驱动其决策的关键洞察。',
    proposition: '锁定面向受众的唯一核心主张。',
    cross_platform_role: '说明不同平台如何协同，而不是重复分发。',
    channel_strategy: '定义每个渠道的角色与适合的内容形式。',
    platform_plans: '把渠道角色细化为可执行的平台计划。',
    creative_strategy: '将策略转译为创意母题、证据和渠道适配。',
    creative_recommendations: '旧版创意建议仅供读取；升级后将转为结构化创意策略。',
    constraints: '明确执行必须遵守的硬边界。',
    budget_and_cadence: '对齐预算配置和内容节奏。',
    experiment_matrix: '定义需要验证的假设、变量与指标。',
    measurement: '确定用于判断效果的指标体系。',
    assumptions_and_gaps: '显式标记尚未证实的判断和信息缺口。',
    evidence_refs: '绑定支撑策略判断的可追溯来源。',
  }
  return descriptions[value] ?? '编辑当前策略章节。'
}

function strategyFieldLabel(value: string) {
  const labels: Record<string, string> = {
    primary: '核心人群', insights: '人群洞察', platform: '平台', role: '平台角色',
    formats: '内容形式', audience_angle: '人群切入点', content_pillars: '内容支柱',
    conversion_path: '转化路径', cadence: '内容节奏', primary_kpi: '核心 KPI',
    creative_ideas: '创意方向', hypothesis: '实验假设', variable: '实验变量',
    metric: '衡量指标', budget: '预算',
    brief_id: 'Brief ID', brief_version: 'Brief 版本', project_context_version: 'Project 上下文版本',
    skill_versions: '技能版本', passed: '是否通过', issues: '检查项', rule_id: '规则',
    severity: '级别', message: '说明', checked_at: '检查时间',
    name: '母题名称', audience_tension: '受众张力', core_idea: '核心想法', proof: '可用证据',
    channel_adaptations: '渠道适配', adaptation: '适配方式', tone: '表达调性',
    mandatories: '必须包含', avoidances: '避免事项', message_hierarchy: '信息层级', territories: '创意母题',
  }
  return labels[value] ?? strategySectionLabel(value)
}

function fieldLabel(value: string) {
  const field = briefFields.find(item => item.path === value)
  return field?.label ?? value
}

function statusLabel(value: string) {
  const labels: Record<string, string> = {
    active: '进行中',
    waiting_user: '等待补充',
    ready_to_confirm: '待确认',
    completed: '已完成',
    generating: '生成中',
    draft: '草稿',
    ready_for_review: '待评审',
    returned: '已退回',
    approved: '已批准',
    invalidated: '已失效',
    open: '评审中',
    failed: '失败',
    cancelled: '已取消',
  }
  return labels[value] ?? value
}

function formatTime(value: string) {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString('zh-CN', { hour12: false })
}

function formatBytes(value: number) {
  if (value < 1024) return `${value} B`
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`
  return `${(value / 1024 / 1024).toFixed(1)} MB`
}

function documentStatusLabel(document: KnowledgeDocument) {
  if (document.status === 'ready') return `${document.chunk_count} 个片段 · 已就绪`
  if (document.status === 'parse_failed') return document.parse_error_message || '解析失败'
  if (document.status === 'parsing') return '解析中'
  return '等待解析'
}
