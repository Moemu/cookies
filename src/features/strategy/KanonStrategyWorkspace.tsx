import {
  AlertCircle,
  Archive,
  BadgeCheck,
  BookOpen,
  Bot,
  Check,
  ChevronRight,
  CircleCheck,
  ExternalLink,
  FileText,
  LoaderCircle,
  LockKeyhole,
  MessageSquare,
  Plus,
  RefreshCw,
  RotateCcw,
  Search,
  Send,
  ShieldCheck,
  Sparkles,
  Upload,
} from 'lucide-react'
import { useEffect, useMemo, useRef, useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { useProject } from '../../context/ProjectContext'
import { CreativeTaskPlanner } from './CreativeTaskPlanner'
import { useStrategyWorkspace } from './useStrategyWorkspace'
import type {
  BriefDraft,
  DraftRevision,
  KnowledgeDocument,
  Review,
  StrategyDocument,
  StrategyDraft,
} from './types'

export function KanonStrategyWorkspace({ activeView, workspaceId }: { activeView: string; workspaceId?: string }) {
  const { currentProject } = useProject()
  const navigate = useNavigate()
  const { state, actions } = useStrategyWorkspace(currentProject.id, workspaceId)
  const mainRef = useRef<HTMLElement>(null)
  useEffect(() => {
    mainRef.current?.scrollTo({ top: 0 })
  }, [activeView])

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
    return <div className="kanon-strategy-state">
      <Sparkles size={28}/>
      <h2>开始需求对话</h2>
      <p>系统将创建持久对话和 Brief 草稿；刷新页面后可以继续。</p>
      {state.error ? <div className="kanon-strategy-alert" role="alert"><AlertCircle size={15}/>{state.error}</div> : null}
      <button className="primary-button" disabled={Boolean(state.busy)} onClick={() => void actions.startConversation()}>
        <MessageSquare size={16}/>{state.busy === 'conversation' ? '正在启动…' : '开始策略梳理'}
      </button>
    </div>
  }

  const lifecycleLocked = Boolean(state.detail.current_task.discarded_at || state.draft?.archived_at)
  const showSummaryRail = activeView === '概览' || activeView === '评审'
  const hasEvidence = Boolean(
    state.documents.length ||
    state.brief?.document.reference_ids?.length ||
    state.researchRun?.artifacts.length,
  )
  const showEvidenceRail = activeView === '研究' || (activeView === '策略' && hasEvidence)
  const railMode = showSummaryRail ? 'rail-summary' : showEvidenceRail ? 'rail-evidence' : 'rails-none'
  return <div className={`kanon-strategy-root${activeView === '对话' ? ' conversation-active' : ''}`}>
    {state.error ? <div className="kanon-strategy-alert" role="alert">
      <AlertCircle size={15}/><span>{state.error}</span>
      <button aria-label="重新加载策略工作区" onClick={() => void actions.reload()}><RefreshCw size={14}/></button>
    </div> : null}
    {lifecycleLocked ? <div className="kanon-lifecycle-banner" role="status"><Archive size={15}/><span><b>{state.detail.current_task.discarded_at ? '任务已废弃' : '策略已归档'}</b>当前工作区为只读，完整对话、Brief、策略版本和评审记录均已保留。请从“策略任务 → 已归档”恢复后继续操作。</span></div> : null}
    <div className="kanon-workspace-contextbar">
      <div><span>当前工作链</span><strong>{state.detail.workspace.name}</strong><small><LockKeyhole size={12}/>已锁定到当前 Project</small></div>
      <label><span>切换工作区</span><select
        aria-label="切换策略工作区"
        value={state.detail.workspace.id}
        onChange={event => navigate({
          pathname: `/projects/${encodeURIComponent(currentProject.id)}/strategy/workspaces/${encodeURIComponent(event.target.value)}`,
          search: `?view=${encodeURIComponent(activeView)}`,
        })}
      >{state.workspaces.map(workspace => <option key={workspace.id} value={workspace.id}>{workspace.name}{workspace.is_primary ? ' · 主工作区' : ''}</option>)}</select></label>
      <button className="icon-button" aria-label="刷新当前策略工作区" disabled={Boolean(state.busy)} onClick={() => void actions.reload()}><RefreshCw size={15}/></button>
    </div>
    <div className={`kanon-strategy-workspace ${railMode}`}>
      <main className="kanon-strategy-main" ref={mainRef}>
        <fieldset className="kanon-lifecycle-lock" disabled={lifecycleLocked}>
        {activeView === '概览' ? <OverviewPane state={state}/> : null}
        {activeView === '对话' ? <ConversationPane
          brief={state.brief}
          busy={state.busy === 'message' || Boolean(state.pendingAgentTaskId)}
          messages={state.messages}
          onSend={actions.sendMessage}
          pending={Boolean(state.pendingAgentTaskId)}
        /> : null}
        {activeView === 'Brief' ? <BriefPane
          brief={state.brief}
          busy={state.busy}
          onConfirm={actions.confirmBrief}
          onConfirmFields={actions.confirmBriefFields}
          onField={actions.patchBriefField}
        /> : null}
        {activeView === '策略' ? <StrategyPane
          busy={state.busy}
          draft={state.draft}
          readiness={state.readiness}
          probe={state.probe}
          briefReady={Boolean(state.briefVersion)}
          onGenerate={actions.generateStrategy}
          onPatch={actions.patchStrategySection}
          onProbe={actions.probeGeneration}
          onRetry={actions.retryStrategy}
          onRevise={actions.reviseStrategy}
          onSubmit={actions.submitStrategy}
          pending={Boolean(state.pendingAgentTaskId)}
        /> : null}
        {activeView === '创意任务策略' ? <CreativeTaskPlanner
          briefVersion={state.briefVersion}
          draft={state.draft}
          projectId={currentProject.id}
        /> : null}
        {activeView === '评审' ? <ReviewPane
          busy={state.busy}
          comments={state.comments}
          deepReview={state.deepReview}
          draft={state.draft}
          review={state.review}
          revisions={state.revisions}
          onAddComment={actions.addComment}
          onApprove={actions.approveReview}
          onDeepReview={actions.startDeepReview}
          onReturn={actions.returnReview}
        /> : null}
        {activeView === '研究' ? <ResearchPane
          brief={state.brief}
          busy={state.busy}
          documents={state.documents}
          researchRun={state.researchRun}
          onAdoption={actions.setResearchArtifactAdoption}
          onResearch={actions.runResearch}
          onUpload={actions.uploadDocument}
        /> : null}
        {activeView === '实验' ? <UnavailablePane
          title="实验编排尚未开放"
          detail="当前不会用静态实验结果冒充真实数据。策略中的实验矩阵会如实保留，后续接入素材与投放实验执行。"
        /> : null}
        {activeView === '变更记录' ? <ChangeLogPane
          comments={state.comments}
          revisions={state.revisions}
          review={state.review}
        /> : null}
        </fieldset>
      </main>
      {showSummaryRail ? <SummaryRail
        brief={state.brief}
        briefVersion={state.briefVersion?.version}
        draft={state.draft}
        publishedVersion={state.published?.version}
        review={state.review}
        workspaceName={state.detail.workspace.name}
      /> : null}
      {showEvidenceRail ? <EvidenceRail
        documents={state.documents}
        referenceIds={state.brief?.document.reference_ids ?? []}
        researchArtifacts={state.researchRun?.artifacts ?? []}
      /> : null}
    </div>
  </div>
}

type WorkspaceState = ReturnType<typeof useStrategyWorkspace>['state']

function OverviewPane({ state }: { state: WorkspaceState }) {
  const stages = [
    {
      label: '需求对话',
      complete: state.messages.some(message => message.role === 'assistant'),
      detail: `${state.messages.length} 条消息`,
    },
    {
      label: 'Brief',
      complete: state.brief?.status === 'confirmed',
      detail: state.brief?.status === 'confirmed'
        ? `已冻结 v${state.briefVersion?.version ?? state.brief.version}`
        : state.brief?.completeness.ready ? '可以确认' : `${state.brief?.completeness.blockers.length ?? 0} 个阻断项`,
    },
    {
      label: '策略',
      complete: Boolean(state.draft?.current_revision),
      detail: state.draft ? `Revision ${state.draft.current_revision} · ${statusLabel(state.draft.status)}` : '等待生成',
    },
    {
      label: '评审',
      complete: state.review?.status === 'approved',
      detail: state.review ? statusLabel(state.review.status) : '尚未提交',
    },
    {
      label: '策略包',
      complete: Boolean(state.published),
      detail: state.published ? `已发布 v${state.published.version}` : '等待批准',
    },
  ]

  return <section className="kanon-strategy-overview">
    <div className="kanon-strategy-heading">
      <div><span className="section-label">真实后端状态</span><h2>从需求共识推进到可交付策略</h2><p>页面只展示当前 Project 已持久化的数据。</p></div>
      <span className={`source-chip ${state.readiness?.generation_mode === 'provider' ? '' : 'alert'}`}>
        {state.readiness?.generation_mode === 'provider' ? '真实模型' : 'Deterministic 模式'}
      </span>
    </div>
    <div className="kanon-stage-list">
      {stages.map((stage, index) => <article className={stage.complete ? 'complete' : ''} key={stage.label}>
        <span>{stage.complete ? <Check size={16}/> : String(index + 1).padStart(2, '0')}</span>
        <div><b>{stage.label}</b><small>{stage.detail}</small></div>
        <ChevronRight size={15}/>
      </article>)}
    </div>
    <div className="kanon-strategy-note">
      <ShieldCheck size={18}/>
      <div><b>{state.probe?.ready ? `真实模型已验证：${state.probe.model_version}` : '真实模型由服务端路由管理'}</b><p>{state.probe?.ready ? `结构化输出通过，耗时 ${state.probe.latency_ms} ms；路由与用量均已记录。` : '在“策略”步骤运行真实模型探针，可验证当前路由、结构化输出与凭据状态。'}</p></div>
    </div>
  </section>
}

function ConversationPane({ brief, busy, messages, onSend, pending }: {
  brief: BriefDraft | null
  busy: boolean
  messages: WorkspaceState['messages']
  onSend: (content: string) => Promise<boolean>
  pending: boolean
}) {
  const [content, setContent] = useState('')
  const listRef = useRef<HTMLDivElement>(null)
  const textareaRef = useRef<HTMLTextAreaElement>(null)
  useEffect(() => {
    const list = listRef.current
    if (list) list.scrollTop = list.scrollHeight
  }, [messages, pending])
  useEffect(() => {
    const textarea = textareaRef.current
    if (!textarea) return
    textarea.style.height = 'auto'
    textarea.style.height = `${Math.min(Math.max(textarea.scrollHeight, 88), 180)}px`
  }, [content])

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    const value = content.trim()
    if (!value) return
    setContent('')
    const sent = await onSend(value)
    if (!sent) setContent(current => current.trim() ? current : value)
  }

  return <section className="kanon-conversation">
    <div className="kanon-strategy-heading">
      <div><span className="section-label">STRATEGY COPILOT</span><h2>把模糊需求聊清楚</h2><p>同一 Conversation 持续更新服务端 Brief 草稿。</p></div>
      <span className="source-chip">{brief?.completeness.ready ? '信息已完整' : '实时整理 Brief'}</span>
    </div>
    <div className="kanon-message-list" ref={listRef}>
      {!messages.length ? <div className="kanon-conversation-empty">
        <Bot size={22}/><b>从现在最确定的部分开始</b>
        <p>描述品牌、产品、目标受众或这次推广最想解决的问题。</p>
      </div> : null}
      {messages.map(message => <article className={`kanon-message ${message.role}`} key={message.id}>
        <span>{message.role === 'user' ? '我' : message.role === 'assistant' ? 'AI' : '•'}</span>
        <div><small>{message.role === 'user' ? '需求方' : message.role === 'assistant' ? 'Strategy 助手' : '系统事件'} · {formatTime(message.created_at)}</small><p>{message.content}</p></div>
      </article>)}
      {pending ? <article className="kanon-message assistant thinking">
        <span>AI</span><div><small>Strategy 助手</small><p><LoaderCircle className="spin" size={14}/>正在更新对话共识与 Brief…</p></div>
      </article> : null}
    </div>
    <form className="kanon-composer" onSubmit={submit}>
      <label htmlFor="kanon-strategy-message"><span>继续描述需求</span><small>{brief?.completeness.ready ? 'Brief 信息已完整，可继续补充' : 'AI 会同步整理到 Brief'}</small></label>
      <textarea
        aria-describedby="kanon-strategy-message-help"
        id="kanon-strategy-message"
        maxLength={4000}
        onChange={event => setContent(event.target.value)}
        onKeyDown={event => {
          if (event.key === 'Enter' && !event.shiftKey && !event.nativeEvent.isComposing) {
            event.preventDefault()
            event.currentTarget.form?.requestSubmit()
          }
        }}
        placeholder="输入品牌、产品、目标受众、预算或希望解决的问题…"
        ref={textareaRef}
        rows={3}
        value={content}
      />
      <div>
        <span id="kanon-strategy-message-help"><small>Enter 发送 · Shift + Enter 换行</small><small className="kanon-composer-count">{content.length} / 4000</small></span>
        <button aria-label="发送需求消息" className="primary-button" disabled={busy || !content.trim()} type="submit"><Send size={15}/>{busy ? '处理中…' : '发送'}</button>
      </div>
    </form>
  </section>
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
]

function BriefPane({ brief, busy, onConfirm, onConfirmFields, onField }: {
  brief: BriefDraft | null
  busy: string
  onConfirm: () => Promise<boolean>
  onConfirmFields: (operations: Array<{ fieldPath: string; value: unknown }>) => Promise<boolean>
  onField: (path: string, value: unknown) => Promise<boolean>
}) {
  if (!brief) return <UnavailablePane title="Brief 尚未创建" detail="请先进入对话并发送第一条需求信息。"/>
  const frozen = brief.status === 'confirmed'
  const unconfirmedFields = briefFields.flatMap(field => {
    const value = field.value(brief)
    if (!value.trim() || brief.field_states[field.path]?.confirmation === 'confirmed') return []
    return [{ fieldPath: field.path, value: field.parse ? field.parse(value) : value }]
  })
  return <section className="kanon-brief-pane">
    <div className="kanon-strategy-heading">
      <div><span className="section-label">BRIEF DRAFT v{brief.version}</span><h2>确认策略输入</h2><p>字段修改使用服务端版本校验，确认后冻结为不可变 BriefVersion。</p></div>
      <span className={`source-chip ${brief.completeness.ready ? '' : 'alert'}`}>{frozen ? '已冻结' : brief.completeness.ready ? '可以确认' : `${brief.completeness.blockers.length} 个阻断项`}</span>
    </div>
    <div className="kanon-field-grid">
      {briefFields.map(field => <EditableField
        busy={busy === `brief:${field.path}`}
        disabled={frozen || Boolean(busy)}
        key={field.path}
        label={field.label}
        multiline={field.multiline}
        onSave={value => onField(field.path, field.parse ? field.parse(value) : value)}
        state={brief.field_states[field.path]}
        value={field.value(brief)}
      />)}
    </div>
    <div className="kanon-brief-footer">
      <div>
        {brief.completeness.blockers.map(blocker => <span key={`${blocker.field}-${blocker.reason}`}><AlertCircle size={13}/>{fieldLabel(blocker.field)}：{blocker.reason}</span>)}
        {!brief.completeness.blockers.length ? <span><CircleCheck size={13}/>必填信息与确认状态满足冻结条件</span> : null}
      </div>
      <div className="kanon-brief-actions">
        {!frozen && unconfirmedFields.length ? <button className="secondary-button" disabled={Boolean(busy)} onClick={() => void onConfirmFields(unconfirmedFields)}>
          <Check size={15}/>{busy === 'confirm-brief-fields' ? '确认中…' : `确认全部已填写字段（${unconfirmedFields.length}）`}
        </button> : null}
        <button className="primary-button" disabled={frozen || !brief.completeness.ready || Boolean(busy)} onClick={() => void onConfirm()}>
          <BadgeCheck size={16}/>{busy === 'confirm-brief' ? '确认中…' : frozen ? 'Brief 已冻结' : '确认并冻结 Brief'}
        </button>
      </div>
    </div>
  </section>
}

function EditableField({ busy, disabled, label, multiline, onSave, state, value }: {
  busy: boolean
  disabled: boolean
  label: string
  multiline?: boolean
  onSave: (value: string) => Promise<boolean>
  state?: BriefDraft['field_states'][string]
  value: string
}) {
  const [draftValue, setDraftValue] = useState(value)
  useEffect(() => setDraftValue(value), [value])
  const changed = draftValue.trim() !== value.trim()
  const needsConfirmation = Boolean(value.trim()) && state?.confirmation !== 'confirmed'
  return <label className={`kanon-field ${multiline ? 'wide' : ''}`}>
    <span>{label}<small>{state?.confirmation === 'confirmed' ? '已确认' : state ? `${state.confidence} 置信度` : '待补充'}</small></span>
    {multiline
      ? <textarea disabled={disabled} rows={3} value={draftValue} onChange={event => setDraftValue(event.target.value)}/>
      : <input disabled={disabled} value={draftValue} onChange={event => setDraftValue(event.target.value)}/>}
    {(changed || needsConfirmation) && !disabled ? <button disabled={busy} type="button" onClick={() => void onSave(draftValue)}>{busy ? '处理中…' : changed ? '保存并确认' : '确认此字段'}</button> : null}
  </label>
}

function StrategyPane({ briefReady, busy, draft, onGenerate, onPatch, onProbe, onRetry, onRevise, onSubmit, pending, probe, readiness }: {
  briefReady: boolean
  busy: string
  draft: StrategyDraft | null
  onGenerate: () => Promise<boolean>
  onPatch: (section: string, value: unknown) => Promise<boolean>
  onProbe: () => Promise<boolean>
  onRetry: () => Promise<boolean>
  onRevise: (instruction: string) => Promise<boolean>
  onSubmit: () => Promise<boolean>
  pending: boolean
  probe: WorkspaceState['probe']
  readiness: WorkspaceState['readiness']
}) {
  const [section, setSection] = useState('objective')
  const [sectionValue, setSectionValue] = useState<unknown>('')
  const [instruction, setInstruction] = useState('')
  const document = draft?.revision?.document
  useEffect(() => {
    if (!document) {
      setSectionValue('')
      return
    }
    const value = (document as unknown as Record<string, unknown>)[section]
    setSectionValue(structuredClone(value))
  }, [document, section])

  if (!draft) {
    return <section className="kanon-strategy-generate">
      <Sparkles size={28}/>
      <h2>生成第一版策略</h2>
      <p>{briefReady ? '已确认 Brief 将作为不可变输入，生成结果会保存为 Strategy revision。' : '请先完成并确认 Brief。'}</p>
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

  const sections = Object.keys(document).filter(key => !['contract_version', 'compliance'].includes(key))
  const original = (document as unknown as Record<string, unknown>)[section]
  const changed = JSON.stringify(sectionValue) !== JSON.stringify(original)
  const canEdit = draft.status === 'draft' || draft.status === 'returned'

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
    <div className="kanon-strategy-section-editor">
      <nav aria-label="策略区块">{sections.map((value, index) => <button className={value === section ? 'active' : ''} key={value} onClick={() => setSection(value)}>
        <span>{String(index + 1).padStart(2, '0')}</span>{strategySectionLabel(value)}
      </button>)}</nav>
      <div>
        <div className="surface-toolbar"><div><h3>{strategySectionLabel(section)}</h3><small>编辑当前区块</small></div><small>保存将创建 Revision {draft.current_revision + 1}</small></div>
        <StructuredStrategyEditor disabled={!canEdit || Boolean(busy)} onChange={setSectionValue} section={section} value={sectionValue}/>
        <button className="secondary-button" disabled={!canEdit || Boolean(busy) || !changed} onClick={() => void onPatch(section, sectionValue)}>
          {busy === `strategy:${section}` ? '保存中…' : '保存为新 Revision'}
        </button>
      </div>
    </div>
    <div className="kanon-revise-box">
      <label htmlFor="strategy-revise">用自然语言修订</label>
      <textarea id="strategy-revise" disabled={!canEdit || Boolean(busy)} rows={2} value={instruction} onChange={event => setInstruction(event.target.value)} placeholder="例如：把小红书的内容节奏调整为首周种草、第二周测评扩散。"/>
      <button className="secondary-button" disabled={!canEdit || Boolean(busy) || !instruction.trim()} onClick={() => { void onRevise(instruction).then(ok => { if (ok) setInstruction('') }) }}><RotateCcw size={14}/>{busy === 'revise-strategy' ? '修订中…' : '生成修订'}</button>
    </div>
    <div className="kanon-brief-footer">
      <span>候选内容保存后才可提交评审；批准会绑定 Revision 与内容哈希。</span>
      <button className="primary-button" disabled={!canEdit || Boolean(busy)} onClick={() => void onSubmit()}><BadgeCheck size={16}/>{busy === 'submit-review' ? '提交中…' : '提交评审'}</button>
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
    const singleLine = ['平台', '核心 KPI', '实验变量', '衡量指标', '预算', '内容节奏'].includes(section)
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

function ReviewPane({ busy, comments, deepReview, draft, onAddComment, onApprove, onDeepReview, onReturn, review, revisions }: {
  busy: string
  comments: WorkspaceState['comments']
  deepReview: WorkspaceState['deepReview']
  draft: StrategyDraft | null
  onAddComment: (body: string) => Promise<boolean>
  onApprove: () => Promise<boolean>
  onDeepReview: () => Promise<boolean>
  onReturn: (reason: string) => Promise<boolean>
  review: Review | null
  revisions: DraftRevision[]
}) {
  const [comment, setComment] = useState('')
  const [reason, setReason] = useState('')
  const candidate = revisions.find(item => item.revision === review?.candidate_revision) ?? draft?.revision
  const previous = revisions.find(item => item.revision === (review?.candidate_revision ?? 1) - 1)
  const diffs = useMemo(() => strategyDiff(previous, candidate), [candidate, previous])

  if (!review) return <UnavailablePane title="尚未提交评审" detail="在“策略”页签完成候选 Revision 后提交评审。"/>

  return <section className="kanon-review-pane">
    <div className="kanon-strategy-heading">
      <div><span className="section-label">REVIEW {review.id.slice(0, 12)}</span><h2>Revision {review.candidate_revision} 评审</h2><p>决策绑定候选内容哈希，过期版本不能批准。</p></div>
      <span className={`source-chip ${review.status === 'returned' ? 'alert' : ''}`}>{statusLabel(review.status)}</span>
    </div>
    <div className="kanon-review-proof"><span>候选哈希</span><code>{review.candidate_content_hash}</code></div>
    <section className="kanon-deep-review">
      <div className="surface-toolbar"><div><h3>GPT‑5.5 Pro 深度评审</h3><small>Responses · 后台运行 · 仅提供决策辅助</small></div>{deepReview?.status === 'succeeded' ? <span className="source-chip">{deepReview.model_version}</span> : null}</div>
      {!deepReview ? <div className="kanon-deep-review-empty"><p>从证据、渠道协同、可衡量性与执行风险对候选 Revision 做第二视角检查。</p><button className="secondary-button" disabled={Boolean(busy) || review.status !== 'open'} onClick={() => void onDeepReview()}><Sparkles size={14}/>{busy === 'deep-review' ? '正在启动…' : '启动深度评审'}</button></div> : deepReview.status === 'pending' ? <div className="kanon-deep-review-pending" role="status"><LoaderCircle className="spin" size={16}/><span><b>深度评审正在后台运行</b><small>可以离开页面，AgentTask 完成后会自动恢复结果。</small></span></div> : deepReview.status === 'failed' ? <div className="kanon-deep-review-empty"><p>本次深度评审未完成，人工评审不受影响。</p><button className="secondary-button" disabled={Boolean(busy)} onClick={() => void onDeepReview()}><RefreshCw size={14}/>重新运行</button></div> : <><p className="kanon-deep-review-summary">{deepReview.summary}</p><div className="kanon-deep-findings">{deepReview.findings.map((finding, index) => <article className={finding.severity} key={`${finding.section}-${index}`}><header><span>{finding.severity === 'blocker' ? '阻断风险' : finding.severity === 'warning' ? '需要关注' : '优化机会'}</span><small>{strategySectionLabel(finding.section)}</small></header><h4>{finding.title}</h4><p>{finding.detail}</p><div><b>建议</b>{finding.recommendation}</div></article>)}</div><small className="kanon-deep-review-meta">{deepReview.api_mode} · background={String(deepReview.background)} · {deepReview.latency_ms ?? 0} ms{deepReview.usage ? ` · ${deepReview.usage.total_tokens} tokens` : ''}</small></>}
    </section>
    <div className="kanon-review-content">
      <div className="kanon-review-diffs">
        {!diffs.length && candidate ? <pre>{JSON.stringify(candidate.document, null, 2)}</pre> : diffs.map(diff => <article key={diff.section}>
          <h3>{strategySectionLabel(diff.section)}</h3>
          <div><section><small>之前</small><pre>{diff.before}</pre></section><section><small>候选</small><pre>{diff.after}</pre></section></div>
        </article>)}
      </div>
      <aside className="kanon-review-comments">
        <h3>评论与决策</h3>
        {review.assignments?.map(assignment => <article key={assignment.id}><b>{assignment.review_mode === 'self_confirmation' ? '个人确认' : '指定审批人'}</b><p>{assignment.reviewer_user_id}</p><small>{assignment.status}</small></article>)}
        {comments.map(item => <article key={item.id}><b>{item.author_id}</b><p>{item.body}</p><small>{formatTime(item.created_at)}</small></article>)}
        {!comments.length ? <p className="panel-empty">尚无评论。</p> : null}
        {review.status === 'open' ? <>
          <textarea disabled={Boolean(busy)} rows={3} value={comment} onChange={event => setComment(event.target.value)} placeholder="留下可执行评论"/>
          <button className="secondary-button full" disabled={Boolean(busy) || !comment.trim()} onClick={() => { void onAddComment(comment).then(ok => { if (ok) setComment('') }) }}>添加评论</button>
          <button className="primary-button full" disabled={Boolean(busy)} onClick={() => void onApprove()}><BadgeCheck size={15}/>{busy === 'approve-review' ? '处理中…' : review.review_mode === 'self_confirmation' ? '确认并发布策略包' : '批准并发布策略包'}</button>
          <textarea disabled={Boolean(busy)} rows={2} value={reason} onChange={event => setReason(event.target.value)} placeholder="退回原因（必填）"/>
          <button className="secondary-button full" disabled={Boolean(busy) || !reason.trim()} onClick={() => void onReturn(reason)}>退回修改</button>
        </> : <div className="kanon-review-decision"><CircleCheck size={17}/><span>{review.status === 'approved' ? '该候选已批准并发布' : review.decision_reason || '该评审已结束'}</span></div>}
      </aside>
    </div>
  </section>
}

function ResearchPane({ brief, busy, documents, onAdoption, onResearch, onUpload, researchRun }: {
  brief: BriefDraft | null
  busy: string
  documents: WorkspaceState['documents']
  onResearch: (
    category: 'general' | 'audience' | 'competitor' | 'industry',
    query: string,
    documentIds: string[],
  ) => Promise<boolean>
  onAdoption: (artifactId: string, adopted: boolean) => Promise<boolean>
  onUpload: (file: File) => Promise<boolean>
  researchRun: WorkspaceState['researchRun']
}) {
  const [query, setQuery] = useState('')
  const [category, setCategory] = useState<'general' | 'audience' | 'competitor' | 'industry'>('general')
  const [selectedDocumentIds, setSelectedDocumentIds] = useState<string[]>([])
  const frozen = brief?.status === 'confirmed'
  const referenceIds = brief?.document.reference_ids ?? []
  useEffect(() => {
    const readyIDs = new Set(documents.filter(document => document.status === 'ready').map(document => document.id))
    setSelectedDocumentIds(current => current.filter(id => readyIDs.has(id)))
  }, [documents])
  return <section className="kanon-research-pane">
    <div className="kanon-strategy-heading">
      <div><span className="section-label">RESEARCH & EVIDENCE</span><h2>把外部与内部资料变成可引用证据</h2><p>研究结果由后端落为 ResearchArtifact，再写入 Brief reference IDs。</p></div>
      <span className="source-chip">{documents.length} 份项目资料</span>
    </div>
    <div className="kanon-research-grid">
      <section>
        <div className="surface-toolbar"><h3>品牌与项目资料</h3><label className="secondary-button" htmlFor="kanon-knowledge-file"><Upload size={14}/>上传资料</label></div>
        <input id="kanon-knowledge-file" type="file" accept=".md,.docx,.pdf" disabled={Boolean(busy) || frozen} onChange={event => { const file = event.target.files?.[0]; if (file) void onUpload(file) }}/>
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
                disabled={!ready || Boolean(busy) || frozen}
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
        <div className="surface-toolbar"><h3>外部研究</h3><span>Seed 联网搜索</span></div>
        <label>研究分类<select disabled={Boolean(busy) || frozen} value={category} onChange={event => setCategory(event.target.value as typeof category)}><option value="general">综合研究</option><option value="audience">受众研究</option><option value="competitor">竞品研究</option><option value="industry">行业研究</option></select></label>
        <label>要验证的问题<textarea disabled={Boolean(busy) || frozen} rows={4} value={query} onChange={event => setQuery(event.target.value)} placeholder="例如：近半年小红书工业品牌内容的有效切入点是什么？"/></label>
        <p className="kanon-research-disclosure">开始研究会把当前问题发送给 Seed 并记录披露字段。项目文件不会自动发送；若勾选资料，仅披露本地检索命中的最多 8 个片段，并记录实际片段 ID。</p>
        <button className="primary-button" disabled={Boolean(busy) || frozen || !query.trim()} onClick={() => void onResearch(category, query, selectedDocumentIds)}><Search size={15}/>{busy === 'research' ? '研究中…' : '开始联网研究'}</button>
      </section>
    </div>
    {researchRun ? <div className="kanon-research-result">
      <div><b>Research Run {researchRun.id.slice(0, 12)}</b><span>{researchRun.model_version || researchRun.provider_code}</span><span className={`source-chip ${researchRun.status === 'failed' || researchRun.status === 'unavailable' ? 'alert' : ''}`}>{researchRun.status}</span></div>
      {researchRun.error_message ? <p>{researchRun.error_message}</p> : null}
      {researchRun.artifacts.map(artifact => {
        const adopted = referenceIds.includes(artifact.id)
        return <article key={artifact.id}>
          <BookOpen size={17}/>
          <div>
            <b>{artifact.title}</b>
            <p>{artifact.content}</p>
            <div className="kanon-research-sources">
              {artifact.sources.map(source => <a href={source.url} key={`${source.id}-${source.start_index}-${source.end_index}`} rel="noreferrer" target="_blank">
                <ExternalLink size={12}/>{source.title || source.domain}<small>{source.verification_status === 'content_verified' ? '已核验' : '模型引用'}</small>
              </a>)}
            </div>
          </div>
          <button className={adopted ? 'secondary-button' : 'primary-button'} disabled={Boolean(busy) || frozen} onClick={() => void onAdoption(artifact.id, !adopted)}>
            {adopted ? '取消采纳' : '采纳到 Brief'}
          </button>
        </article>
      })}
    </div> : null}
  </section>
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
    before: pretty((before as unknown as Record<string, unknown>)[section]),
    after: pretty((candidate.document as unknown as Record<string, unknown>)[section]),
  })).filter(item => item.before !== item.after)
}

function splitValues(value: string) {
  return value.split(/[\n,，、]/).map(item => item.trim()).filter(Boolean)
}

function pretty(value: unknown) {
  if (value === undefined) return '—'
  return typeof value === 'string' ? value : JSON.stringify(value, null, 2)
}

function strategySectionLabel(value: string) {
  const labels: Record<string, string> = {
    objective: '目标',
    audience: '受众',
    proposition: '核心主张',
    channel_strategy: '渠道策略',
    creative_recommendations: '创意建议',
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
  }
  return labels[value] ?? value
}

function strategyFieldLabel(value: string) {
  const labels: Record<string, string> = {
    primary: '核心人群', insights: '人群洞察', platform: '平台', role: '平台角色',
    formats: '内容形式', audience_angle: '人群切入点', content_pillars: '内容支柱',
    conversion_path: '转化路径', cadence: '内容节奏', primary_kpi: '核心 KPI',
    creative_ideas: '创意方向', hypothesis: '实验假设', variable: '实验变量',
    metric: '衡量指标', budget: '预算',
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
