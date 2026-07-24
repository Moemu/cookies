import { useCallback, useEffect, useRef, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import type { Project } from '../platform/types'
import {
  approveStrategy,
  createMutationKey,
  confirmBrief,
  createConversation,
  createStrategy,
  getAgentTask,
  getBriefDraft,
  getConversationMemory,
  getGenerationMetadata,
  getGenerationReadiness,
  getReview,
  getStrategy,
  getWorkspace,
  listBriefVersions,
  listMessages,
  listKnowledgeDocuments,
  listSkillRuns,
  listStrategyPackages,
  patchBriefField,
  patchStrategySection,
  reviseStrategy,
  sendMessage,
  submitStrategy,
  uploadKnowledgeDocument,
  runExternalResearch,
  createStrategyFeedback,
} from './api'
import { useConversationStream } from './conversation/useConversationStream'
import { createCreativeIntakeFromStrategy, listCreativeIntakes } from '../creative/api'
import type { BriefDraft, BriefVersion, GenerationMetadata, GenerationReadiness, KnowledgeDocument, Message, PackageVersion, ResearchRun, Review, SkillRun, StrategyDraft, WorkspaceDetail } from './types'
import './strategy.css'

export function StrategyWorkspacePage({ project }: { project?: Project }) {
  const { projectId = '', workspaceId = '', stage = 'conversation' } = useParams()
  const navigate = useNavigate()
  const [detail, setDetail] = useState<WorkspaceDetail | null>(null)
  const [messages, setMessages] = useState<Message[]>([])
  const [brief, setBrief] = useState<BriefDraft | null>(null)
  const [briefVersion, setBriefVersion] = useState<BriefVersion | null>(null)
  const [draft, setDraft] = useState<StrategyDraft | null>(null)
  const [review, setReview] = useState<Review | null>(null)
  const [published, setPublished] = useState<PackageVersion | null>(null)
  const [readiness, setReadiness] = useState<GenerationReadiness | null>(null)
  const [generationMetadata, setGenerationMetadata] = useState<GenerationMetadata | null>(null)
  const [documents, setDocuments] = useState<KnowledgeDocument[]>([])
  const [memory, setMemory] = useState<{ summary: string; open_questions: string[]; version: number } | null>(null)
  const [skillRuns, setSkillRuns] = useState<SkillRun[]>([])
  const [researchRun, setResearchRun] = useState<ResearchRun | null>(null)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState('')
  const [pendingAgentTaskId, setPendingAgentTaskId] = useState('')
  const approveMutationKey = useRef('')

  useEffect(() => {
    approveMutationKey.current = ''
  }, [review?.id])

  const load = useCallback(async (signal?: AbortSignal) => {
    if (!workspaceId) return
    const workspace = await getWorkspace(workspaceId, signal)
    if (workspace.workspace.project_id !== projectId) {
      throw new Error('工作区不属于当前项目。')
    }
    setDetail(workspace)
    setDraft(null)
    setReview(null)
    setPublished(null)
    setBriefVersion(null)
    setGenerationMetadata(null)
    const task = workspace.current_task
    const conversation = workspace.current_conversation
    const [messageResult, briefResult, packageResult, readinessResult, documentResult] = await Promise.all([
      conversation ? listMessages(conversation.id, signal) : Promise.resolve({ items: [] }),
      task ? getBriefDraft(task.id, signal) : Promise.resolve(null),
      listStrategyPackages(projectId, signal),
      getGenerationReadiness(projectId, signal),
      listKnowledgeDocuments(projectId, signal),
    ])
    setMessages(messageResult.items)
    setBrief(briefResult)
    setReadiness(readinessResult)
    setDocuments(documentResult.items)
    setMemory(conversation ? await getConversationMemory(conversation.id, signal).catch(() => null) : null)
    setSkillRuns(task?.current_agent_task_id ? (await listSkillRuns(task.current_agent_task_id, signal).catch(() => ({ items: [] }))).items : [])
    if (task && briefResult?.status === 'confirmed') {
      const versions = await listBriefVersions(task.brief_id, signal)
      setBriefVersion(versions.items[0] ?? null)
    }
    if (task?.current_strategy_id) {
      const currentDraft = await getStrategy(task.current_strategy_id, signal)
      setDraft(currentDraft)
      if (currentDraft.current_revision > 0) {
        setGenerationMetadata(await getGenerationMetadata(currentDraft.id, signal))
      }
      const packageVersion = packageResult.items
        .filter((item) => item.status === 'published' && item.snapshot.strategy_id === currentDraft.id)
        .sort((left, right) => right.version - left.version)[0]
      setPublished(packageVersion ?? null)
      if (currentDraft.current_review_id) {
        setReview(await getReview(currentDraft.current_review_id, signal))
      }
    }
  }, [projectId, workspaceId])

  useEffect(() => {
    const controller = new AbortController()
    const timer = window.setTimeout(() => {
      load(controller.signal).catch((cause: unknown) => {
        if (!(cause instanceof DOMException && cause.name === 'AbortError')) setError(messageOf(cause))
      })
    }, 0)
    return () => {
      window.clearTimeout(timer)
      controller.abort()
    }
  }, [load])

  useConversationStream(detail?.current_conversation?.id, () => {
    load().catch((cause: unknown) => setError(messageOf(cause)))
  })

  useEffect(() => {
    if (!pendingAgentTaskId) return
    const controller = new AbortController()
    let timer = 0
    const inspect = async () => {
      try {
        const agentTask = await getAgentTask(pendingAgentTaskId, controller.signal)
        if (agentTask.status === 'failed' || agentTask.status === 'cancelled') {
          setPendingAgentTaskId('')
          setError(agentTask.error?.message || '本轮模型生成未完成，请重试。')
          return
        }
        if (agentTask.status === 'succeeded') {
          await load()
          return
        }
        timer = window.setTimeout(inspect, 1500)
      } catch (cause) {
        if (!(cause instanceof DOMException && cause.name === 'AbortError')) {
          timer = window.setTimeout(inspect, 2500)
        }
      }
    }
    timer = window.setTimeout(inspect, 1500)
    return () => {
      controller.abort()
      window.clearTimeout(timer)
    }
  }, [load, pendingAgentTaskId])

  useEffect(() => {
    if (!draft || draft.status !== 'generating') return
    const timer = window.setInterval(() => {
      getStrategy(draft.id).then(setDraft).catch((cause: unknown) => setError(messageOf(cause)))
    }, 1500)
    return () => window.clearInterval(timer)
  }, [draft])

  const task = detail?.current_task
  const conversation = detail?.current_conversation
  const currentStage = ['conversation', 'brief', 'strategy'].includes(stage) ? stage : 'conversation'
  const streamingAssistant = pendingAgentTaskId
    ? messages.find((message) => message.role === 'assistant' && message.agent_task_id === pendingAgentTaskId)
    : undefined
  const run = async (name: string, action: () => Promise<void>) => {
    setBusy(name)
    setError('')
    try {
      await action()
      return true
    } catch (cause) {
      setError(messageOf(cause))
      return false
    } finally {
      setBusy('')
    }
  }

  if (!project || project.id !== projectId) {
    return <section className="strategy-empty"><h1>策略工作区</h1><p>请选择一个可访问的项目。</p></section>
  }

  if (!detail) {
    return <section className="strategy-empty"><h1>{project.name} · 策略</h1><p>{error || '正在读取工作区…'}</p></section>
  }

  return (
    <section className={`strategy-page strategy-page--${currentStage}`}>
      <header className="strategy-header">
        <div>
          <nav aria-label="策略面包屑"><Link to={`/projects/${projectId}/strategy/workspaces`}>策略</Link><span>/</span><span>{detail.workspace.name}</span></nav>
          <h1>{project.name} · 策略工作区</h1>
          <p>从对话梳理 Brief，确认后生成、评审并发布不可变策略包。</p>
        </div>
        <span className="strategy-status">{published ? `已发布 v${published.version}` : draft ? statusLabel(draft.status) : task ? statusLabel(task.status) : '尚未开始'}</span>
      </header>

      {error ? <div className="strategy-alert" role="alert">{error}</div> : null}

      <nav aria-label="策略阶段" className="strategy-stage-nav">
        {[
          ['conversation', '01', '对话梳理'],
          ['brief', '02', '确认 Brief'],
          ['strategy', '03', '生成策略'],
        ].map(([value, index, label]) => (
          <Link
            aria-current={currentStage === value ? 'page' : undefined}
            className={currentStage === value ? 'strategy-stage-nav__item strategy-stage-nav__item--active' : 'strategy-stage-nav__item'}
            key={value}
            to={`/projects/${projectId}/strategy/workspaces/${workspaceId}/${value}`}
          ><small>{index}</small><span>{label}</span></Link>
        ))}
      </nav>

      {!conversation || !task ? (
        <div className="strategy-start">
          <h2>开始一次策略梳理</h2>
          <p>系统会创建一条持久对话和可追溯 Brief 草稿。</p>
          <button className="button button--primary" disabled={busy !== ''} onClick={() => run('conversation', async () => {
            const bundle = await createConversation(projectId, workspaceId)
            setDetail({ ...detail, current_conversation: bundle.conversation, current_task: bundle.task })
            setBrief(bundle.brief_draft)
            navigate(`/projects/${projectId}/strategy/workspaces/${workspaceId}/conversation`, { replace: true })
          })} type="button">开始梳理</button>
        </div>
      ) : (
        <div className={`strategy-grid strategy-grid--${currentStage}`}>
          {currentStage === 'conversation' ? <ConversationPane
            brief={brief}
            busy={busy === 'message' || Boolean(pendingAgentTaskId && !streamingAssistant)}
            messages={messages}
            onStreamComplete={() => setPendingAgentTaskId((current) => current === pendingAgentTaskId ? '' : current)}
            onSend={(content) => run('message', async () => {
              const result = await sendMessage(conversation.id, content)
              setPendingAgentTaskId(result.agent_task.id)
              setMessages((current) => current.some((message) => message.id === result.message.id)
                ? current
                : [...current, result.message])
            })}
            streamingMessageId={streamingAssistant?.id}
          /> : null}
          {currentStage === 'conversation' ? <BriefCompanion
            brief={brief}
            memory={memory}
            onOpen={() => navigate(`/projects/${projectId}/strategy/workspaces/${workspaceId}/brief`)}
          /> : null}
          {currentStage === 'brief' ? <BriefPane
            brief={brief}
            busy={busy !== ''}
            documents={documents}
            memory={memory}
            onConfirm={() => brief && run('confirm', async () => {
              const value = await confirmBrief(task.id, brief.version)
              setBriefVersion(value)
              setBrief({ ...brief, status: 'confirmed' })
              setDetail({
                ...detail,
                current_task: detail.current_task ? { ...detail.current_task, status: 'completed' } : undefined,
              })
              navigate(`/projects/${projectId}/strategy/workspaces/${workspaceId}/strategy`)
            })}
            onField={(path, value) => brief && run(`brief:${path}`, async () => {
              setBrief(await patchBriefField(task.id, brief, path, value))
            })}
            onResearch={(mode, query) => brief && run('research', async () => {
              const knownDocuments = new Set(documents.map((document) => document.id))
              const documentIds = (brief.document.reference_ids || []).filter((id) => knownDocuments.has(id))
              const value = await runExternalResearch(projectId, {
                mode,
                query,
                document_ids: documentIds,
                disclosed_fields: documentIds.length > 0 ? ['query', 'document_content'] : ['query'],
                confirmed: true,
              })
              setResearchRun(value)
              const artifactIds = value.artifacts.map((artifact) => artifact.id)
              if (artifactIds.length > 0) {
                setBrief(await patchBriefField(task.id, brief, 'reference_ids', Array.from(new Set([
                  ...documentIds,
                  ...artifactIds,
                ]))))
              }
            })}
            onUpload={(file) => brief && run('document', async () => {
              const uploaded = await uploadKnowledgeDocument(projectId, file)
              const nextDocuments = [uploaded, ...documents]
              setDocuments(nextDocuments)
              if (!(brief.document.reference_ids || []).includes(uploaded.id)) {
                setBrief(await patchBriefField(task.id, brief, 'reference_ids', [
                  ...(brief.document.reference_ids || []),
                  uploaded.id,
                ]))
              }
            })}
            researchRun={researchRun}
          /> : null}
          {currentStage === 'strategy' ? <StrategyPane
            busy={busy !== ''}
            canGenerate={briefVersion !== null}
            draft={draft}
            generationMetadata={generationMetadata}
            skillRuns={skillRuns}
            packageVersion={published}
            readiness={readiness}
            review={review}
            onApprove={() => draft && review && run('approve', async () => {
              if (!approveMutationKey.current) approveMutationKey.current = createMutationKey()
              const current = await getStrategy(draft.id)
              setDraft(current)
              const packageVersion = await approveStrategy(current, review, approveMutationKey.current)
              setPublished(packageVersion)
              approveMutationKey.current = ''
              setDraft({ ...current, status: 'approved' })
              navigate(`/projects/${projectId}/strategy/packages/${packageVersion.package_id}`)
            })}
            onCreateCreative={() => published && run('creative', async () => {
              const intakes = await listCreativeIntakes(projectId)
              const alreadyHandedOff = intakes.items.some((intake) => {
                const strategyPackage = intake.request.strategy_package
                return intake.source === 'strategy_package'
                  && strategyPackage?.package_id === published.package_id
                  && strategyPackage.package_version === published.version
                  && strategyPackage.expected_content_hash === published.content_hash
              })

              if (!alreadyHandedOff) {
                await createCreativeIntakeFromStrategy(
                  projectId,
                  published.package_id,
                  published.version,
                  published.content_hash,
                )
              }
              navigate(`/projects/${projectId}/creative/tasks`)
            })}
            onGenerate={() => briefVersion && run('generate', async () => {
              const result = await createStrategy(task.id, briefVersion)
              setDraft(result.strategy_draft)
            })}
            onPatch={(section, value) => draft && run(`strategy:${section}`, async () => {
              setDraft(await patchStrategySection(draft, section, value))
              setReview(null)
            })}
            onRevise={async (instruction) => {
              if (!draft) return false
              let revised = false
              await run('revise', async () => {
                await reviseStrategy(draft, instruction)
                setDraft({ ...draft, status: 'generating', version: draft.version + 1 })
                revised = true
              })
              return revised
            }}
            onSubmit={() => draft && run('submit', async () => {
              const created = await submitStrategy(draft)
              setReview(created)
              setDraft(await getStrategy(draft.id))
              navigate(`/projects/${projectId}/strategy/reviews/${created.id}`)
            })}
            onFeedback={(rating, comment) => run('feedback', async () => {
              if (!published) return
              await createStrategyFeedback(projectId, {
                target_type: 'strategy_package',
                target_id: published.package_id,
                target_version: published.version,
                rating,
                comment,
              })
            })}
          /> : null}
        </div>
      )}
    </section>
  )
}

export function ConversationPane({ brief, busy, messages, onSend, onStreamComplete, streamingMessageId }: {
  brief: BriefDraft | null
  busy: boolean
  messages: Message[]
  onSend: (content: string) => void
  streamingMessageId?: string
  onStreamComplete?: () => void
}) {
  const [content, setContent] = useState('')
  const listRef = useRef<HTMLDivElement>(null)
  const formRef = useRef<HTMLFormElement>(null)
  useEffect(() => {
    const list = listRef.current
    if (!list) return
    if (typeof list.scrollTo === 'function') {
      list.scrollTo({ top: list.scrollHeight, behavior: streamingMessageId ? 'auto' : 'smooth' })
    } else {
      list.scrollTop = list.scrollHeight
    }
  }, [busy, messages, streamingMessageId])
  const quickStarts = [
    '我有一个新品牌，需要从零梳理推广需求',
    '我已经有产品和目标人群，想制定多平台策略',
    '我有一份品牌资料，希望结合文档开始',
  ]
  return <article className="strategy-panel conversation-panel">
    <header className="conversation-panel__header">
      <div><span className="eyebrow">Strategy copilot</span><h2>把模糊想法聊清楚</h2><p>助手会记录明确的信息，只追问真正缺失的关键条件。</p></div>
      <span className="live-dot">{brief?.completeness.ready ? '信息已完整' : '实时整理 Brief'}</span>
    </header>
    <div className="conversation-list" aria-live="polite" ref={listRef}>
      {messages.length === 0 ? <section className="conversation-welcome">
        <span className="conversation-welcome__mark">S</span>
        <div><strong>从你现在最确定的部分开始</strong><p>不用填写完整表单。说说品牌、产品，或者这次推广最想解决的问题，我会陪你逐步补齐。</p>
          <div className="conversation-prompts">{quickStarts.map((prompt) => <button disabled={busy} key={prompt} onClick={() => onSend(prompt)} type="button">{prompt}</button>)}</div>
        </div>
      </section> : null}
      {messages.map((message) => {
        const streaming = message.id === streamingMessageId
        return <article className={`strategy-message strategy-message--${message.role}${streaming ? ' strategy-message--streaming' : ''}`} key={message.id}>
          <span className="strategy-message__avatar">{message.role === 'user' ? '你' : 'S'}</span>
          <div><header><strong>{message.role === 'user' ? '你' : 'Strategy 助手'}</strong>{message.ai_generated ? <small>{streaming ? '正在输出' : 'AI 辅助'}</small> : null}</header>
            {streaming
              ? <StreamingAssistantText
                  content={message.content}
                  onComplete={onStreamComplete}
                  onProgress={() => {
                    const list = listRef.current
                    if (list) list.scrollTop = list.scrollHeight
                  }}
                />
              : <p>{message.content}</p>}
          </div>
        </article>
      })}
      {busy ? <article className="strategy-message strategy-message--assistant strategy-message--thinking">
        <span className="strategy-message__avatar">S</span>
        <div><header><strong>Strategy 助手</strong><small>正在理解</small></header><p><i /><i /><i /><span>正在对照会话记忆并更新 Brief</span></p></div>
      </article> : null}
    </div>
    <form className="strategy-composer" ref={formRef} onSubmit={(event) => {
      event.preventDefault()
      const value = content.trim()
      if (!value) return
      onSend(value)
      setContent('')
    }}>
      <div className="strategy-composer__field">
        <label htmlFor="strategy-message">继续描述需求</label>
        <textarea
          id="strategy-message"
          onChange={(event) => setContent(event.target.value)}
          onKeyDown={(event) => {
            if ((event.ctrlKey || event.metaKey) && event.key === 'Enter') formRef.current?.requestSubmit()
          }}
          placeholder="例如：品牌是灵裁，产品是电商创作工具，首期希望在小红书建立认知……"
          rows={3}
          value={content}
        />
        <small>Ctrl + Enter 发送 · 明确信息会同步到右侧</small>
      </div>
      <button className="button button--primary" disabled={busy || !content.trim()} type="submit">{busy ? '正在整理' : '发送'}</button>
    </form>
  </article>
}

function StreamingAssistantText({ content, onComplete, onProgress }: {
  content: string
  onComplete?: () => void
  onProgress: () => void
}) {
  const characters = Array.from(content)
  const reducedMotion = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches ?? false
  const [visibleCharacters, setVisibleCharacters] = useState(reducedMotion ? characters.length : 0)
  const completeRef = useRef(onComplete)
  const progressRef = useRef(onProgress)
  useEffect(() => {
    completeRef.current = onComplete
    progressRef.current = onProgress
  }, [onComplete, onProgress])
  useEffect(() => {
    if (reducedMotion) {
      const timer = window.setTimeout(() => completeRef.current?.(), 0)
      return () => window.clearTimeout(timer)
    }
    let cursor = 0
    const chunkSize = Math.max(1, Math.ceil(characters.length / 80))
    const timer = window.setInterval(() => {
      cursor = Math.min(characters.length, cursor + chunkSize)
      setVisibleCharacters(cursor)
      progressRef.current()
      if (cursor === characters.length) {
        window.clearInterval(timer)
        completeRef.current?.()
      }
    }, 24)
    return () => window.clearInterval(timer)
  }, [characters.length, reducedMotion])
  return <p data-testid="streaming-assistant">
    {characters.slice(0, visibleCharacters).join('')}
    <span aria-hidden="true" className="strategy-stream-cursor" />
  </p>
}

export function BriefCompanion({ brief, memory, onOpen }: {
  brief: BriefDraft | null
  memory: { summary: string; open_questions: string[]; version: number } | null
  onOpen: () => void
}) {
  if (!brief) return <aside className="brief-companion"><div className="brief-companion__empty"><span>Brief</span><strong>等待第一轮对话</strong><p>助手理解到的信息会在这里逐项出现。</p></div></aside>
  const items = briefCompanionItems(brief)
  const filled = items.filter((item) => item.value).length
  const confirmed = items.filter((item) => item.status === 'confirmed').length
  const progress = Math.round((filled / items.length) * 100)
  return <aside className="brief-companion">
    <header>
      <div><span className="eyebrow">Live brief</span><h2>当前理解</h2></div>
      <strong>{progress}%</strong>
    </header>
    <progress aria-label={`Brief 已收集 ${filled} / ${items.length} 项`} className="brief-progress" max="100" value={progress}>{progress}%</progress>
    <p className="brief-companion__summary">{memory?.summary || '对话中的明确信息会持续沉淀到 Brief。'}</p>
    <div className="brief-companion__stats">
      <span><strong>{filled}</strong> 已记录</span>
      <span><strong>{confirmed}</strong> 已确认</span>
      <span><strong>{items.length - filled}</strong> 待补充</span>
    </div>
    <div className="brief-companion__items">
      {items.map((item) => <div className={`brief-companion__item brief-companion__item--${item.status}`} key={item.path}>
        <span className="brief-companion__status" />
        <div><small>{item.label}</small><strong>{item.value || '等待补充'}</strong></div>
        <em>{item.status === 'confirmed' ? '已确认' : item.status === 'captured' ? '已记录' : '待补充'}</em>
      </div>)}
    </div>
    {memory?.open_questions.length ? <section className="brief-companion__next"><span>下一步</span><p>{memory.open_questions.slice(0, 2).join('；')}</p></section> : null}
    <footer><button className="button button--secondary" onClick={onOpen} type="button">查看并确认完整 Brief</button></footer>
  </aside>
}

function briefCompanionItems(brief: BriefDraft) {
  const document = brief.document
  const channels = document.channels.map((channel) => ({
    xiaohongshu: '小红书', douyin: '抖音', taobao_tmall: '淘宝天猫', wechat_ecosystem: '微信生态',
  })[channel] || channel).join('、')
  const definitions = document.contract_version === 'strategy-brief-version/v2'
    ? [
        ['brand.name', '品牌', document.brand?.name || ''],
        ['product.name', '产品 / 服务', document.product?.name || ''],
        ['industry', '行业', document.industry || ''],
        ['campaign.objective', '业务目标', document.campaign.objective],
        ['audience.primary', '核心受众', document.audience.primary],
        ['proposition', '核心卖点', document.proposition],
        ['channels', '投放平台', channels],
        ['region', '地区', document.region || ''],
        ['language', '内容语言', document.language || ''],
      ]
    : [
        ['campaign.objective', '业务目标', document.campaign.objective],
        ['audience.primary', '核心受众', document.audience.primary],
        ['proposition', '核心卖点', document.proposition],
        ['channels', '投放平台', channels],
      ]
  return definitions.map(([path, label, value]) => ({
    path,
    label,
    value,
    status: !value ? 'missing' : brief.field_states[path]?.confirmation === 'confirmed' ? 'confirmed' : 'captured',
  }))
}

function BriefPane({ brief, busy, documents, memory, researchRun, onField, onConfirm, onUpload, onResearch }: {
  brief: BriefDraft | null
  busy: boolean
  documents: KnowledgeDocument[]
  memory: { summary: string; open_questions: string[]; version: number } | null
  researchRun: ResearchRun | null
  onField: (path: string, value: unknown) => void
  onConfirm: () => void
  onUpload: (file: File) => void
  onResearch: (mode: 'web' | 'mcp', query: string) => void
}) {
  if (!brief) return <article className="strategy-panel"><header><div><span className="eyebrow">02 · Brief</span><h2>需求结构</h2></div></header><p className="panel-empty">等待对话开始。</p></article>
  const fields: Array<{ path: string; label: string; value: string }> = [
    ...(brief.document.contract_version === 'strategy-brief-version/v2' ? [
      { path: 'brand.name', label: '品牌', value: brief.document.brand?.name || '' },
      { path: 'product.name', label: '产品 / 服务', value: brief.document.product?.name || '' },
      { path: 'industry', label: '行业', value: brief.document.industry || '' },
      { path: 'region', label: '地区', value: brief.document.region || '' },
      { path: 'language', label: '内容语言', value: brief.document.language || '' },
    ] : []),
    { path: 'campaign.objective', label: '广告目标', value: brief.document.campaign.objective },
    { path: 'audience.primary', label: '核心受众', value: brief.document.audience.primary },
    { path: 'proposition', label: '核心卖点', value: brief.document.proposition },
    { path: 'budget.total', label: '预算', value: brief.document.budget.total },
    { path: 'schedule.window', label: '排期', value: brief.document.schedule.window },
    { path: 'measurement.primary_kpi', label: '核心指标', value: brief.document.measurement.primary_kpi },
  ]
  return <article className="strategy-panel brief-panel">
    <header><div><span className="eyebrow">02 · Brief</span><h2>需求结构</h2></div><span className={brief.completeness.ready ? 'readiness readiness--ready' : 'readiness'}>{brief.completeness.ready ? '可以确认' : `${brief.completeness.blockers.length} 项待处理`}</span></header>
    <div className="brief-fields">
      {fields.map((field) => <BriefField busy={busy} field={field} key={`${field.path}:${brief.version}`} onSave={onField} state={brief.field_states[field.path]} />)}
      <div className="brief-field">
        <label>策略平台</label>
        {brief.document.contract_version === 'strategy-brief-version/v2' ? <PlatformSelector
          disabled={busy}
          onChange={(channels) => onField('channels', channels)}
          value={brief.document.channels}
        /> : <div className="platform-chip">小红书图文 <span>v1 冻结契约</span></div>}
      </div>
    </div>
    {brief.document.contract_version === 'strategy-brief-version/v2' ? <KnowledgeSources
      busy={busy}
      documents={documents}
      memory={memory}
      onAttach={(id) => onField('reference_ids', Array.from(new Set([...(brief.document.reference_ids || []), id])))}
      onResearch={onResearch}
      onUpload={onUpload}
      referenceIds={brief.document.reference_ids || []}
      researchRun={researchRun}
    /> : null}
    {brief.completeness.warnings.length ? <div className="brief-warnings">{brief.completeness.warnings.map((warning) => <p key={warning.field}>{warning.reason}</p>)}</div> : null}
    <footer>
      <span>草稿 v{brief.version} · {brief.status === 'confirmed' ? '已冻结' : '可编辑'}</span>
      <button className="button button--primary" disabled={busy || !brief.completeness.ready || brief.status !== 'open'} onClick={onConfirm} type="button">{brief.status === 'confirmed' ? 'Brief 已确认' : '确认 Brief'}</button>
    </footer>
  </article>
}

function PlatformSelector({ value, disabled, onChange }: {
  value: string[]
  disabled: boolean
  onChange: (value: string[]) => void
}) {
  const options = [
    ['xiaohongshu', '小红书'],
    ['douyin', '抖音'],
    ['taobao_tmall', '淘宝天猫'],
    ['wechat_ecosystem', '微信生态'],
  ]
  return <div className="platform-options">
    {options.map(([id, label]) => <label key={id}>
      <input
        checked={value.includes(id)}
        disabled={disabled}
        onChange={(event) => {
          const next = event.target.checked ? [...value, id] : value.filter((item) => item !== id)
          if (next.length > 0) onChange(next)
        }}
        type="checkbox"
      />
      <span>{label}</span>
    </label>)}
  </div>
}

function KnowledgeSources({ documents, referenceIds, memory, researchRun, busy, onUpload, onAttach, onResearch }: {
  documents: KnowledgeDocument[]
  referenceIds: string[]
  memory: { summary: string; open_questions: string[]; version: number } | null
  researchRun: ResearchRun | null
  busy: boolean
  onUpload: (file: File) => void
  onAttach: (id: string) => void
  onResearch: (mode: 'web' | 'mcp', query: string) => void
}) {
  const [mode, setMode] = useState<'web' | 'mcp'>('web')
  const [query, setQuery] = useState('')
  const [confirmed, setConfirmed] = useState(false)
  const referencedDocumentCount = documents.filter((document) => referenceIds.includes(document.id)).length
  return <section className="knowledge-sources">
    <div className="strategy-section-title"><div><h3>资料与研究</h3><small>支持 Markdown、Word；资料在 Project 内保存。</small></div>
      <label className="button button--secondary">
        上传资料
        <input
          accept=".md,.docx,text/markdown,application/vnd.openxmlformats-officedocument.wordprocessingml.document"
          disabled={busy}
          hidden
          onChange={(event) => {
            const file = event.target.files?.[0]
            if (file) onUpload(file)
            event.target.value = ''
          }}
          type="file"
        />
      </label>
    </div>
    {memory ? <div className="memory-card"><strong>会话记忆 v{memory.version}</strong><p>{memory.summary}</p>{memory.open_questions.length > 0 ? <small>待确认：{memory.open_questions.join('；')}</small> : null}</div> : null}
    <div className="knowledge-document-list">
      {documents.length === 0 ? <p>尚未上传资料。</p> : documents.map((document) => {
        const attached = referenceIds.includes(document.id)
        return <div key={document.id}><span><strong>{document.filename}</strong><small>{formatBytes(document.size_bytes)} · {document.text_sha256.slice(0, 10)}</small></span>
          {attached ? <em>已引用</em> : <button className="text-action" disabled={busy} onClick={() => onAttach(document.id)} type="button">引用</button>}
        </div>
      })}
    </div>
    <div className="research-consent">
      <div><select aria-label="外部研究方式" disabled={busy} onChange={(event) => setMode(event.target.value as 'web' | 'mcp')} value={mode}><option value="web">联网搜索</option><option value="mcp">MCP</option></select>
        <input disabled={busy} onChange={(event) => setQuery(event.target.value)} placeholder="要补充验证的问题" value={query} /></div>
      <label><input checked={confirmed} disabled={busy} onChange={(event) => setConfirmed(event.target.checked)} type="checkbox" />{referencedDocumentCount > 0
        ? `我确认本次将查询词和 ${referencedDocumentCount} 份已引用文档全文发送给所选外部服务。`
        : '我确认本次将查询词发送给所选外部服务；当前没有文档内容会被发送。'}</label>
      <button className="button" disabled={busy || !query.trim() || !confirmed} onClick={() => {
        onResearch(mode, query.trim())
        setConfirmed(false)
      }} type="button">确认并执行本次调用</button>
      {researchRun ? <p className={`research-status research-status--${researchRun.status}`}>{researchRun.status === 'unavailable' ? '外部执行器尚未配置，本次没有发送任何资料。' : researchRun.status === 'succeeded' ? `已生成 ${researchRun.artifacts.length} 条 ResearchArtifact。` : researchRun.error_message || researchRun.status}</p> : null}
    </div>
  </section>
}

function BriefField({ field, state, busy, onSave }: {
  field: { path: string; label: string; value: string }
  state?: { confirmation: string; confidence: string; source: { type: string } }
  busy: boolean
  onSave: (path: string, value: unknown) => void
}) {
  const [value, setValue] = useState(field.value)
  return <div className="brief-field">
    <label htmlFor={`brief-${field.path}`}>{field.label}</label>
    <textarea id={`brief-${field.path}`} onChange={(event) => setValue(event.target.value)} placeholder="待补充" rows={2} value={value} />
    <div className="field-meta">
      <span>{state ? `${sourceLabel(state.source.type)} · ${state.confidence}` : '暂无来源'}</span>
      {state?.confirmation === 'confirmed' && value === field.value ? <small className="confirmed-mark">✓ 已确认</small> : <button className="text-action" disabled={busy || !value.trim()} onClick={() => onSave(field.path, value)} type="button">{field.value ? '确认 / 保存' : '保存'}</button>}
    </div>
  </div>
}

function StrategyPane({ draft, review, packageVersion, readiness, generationMetadata, skillRuns, busy, canGenerate, onGenerate, onPatch, onRevise, onSubmit, onApprove, onCreateCreative, onFeedback }: {
  draft: StrategyDraft | null
  review: Review | null
  packageVersion: PackageVersion | null
  readiness: GenerationReadiness | null
  generationMetadata: GenerationMetadata | null
  skillRuns: SkillRun[]
  busy: boolean
  canGenerate: boolean
  onGenerate: () => void
  onPatch: (section: string, value: unknown) => void
  onRevise: (instruction: string) => Promise<boolean>
  onSubmit: () => void
  onApprove: () => void
  onCreateCreative: () => void
  onFeedback: (rating: 'useful' | 'partly_useful' | 'not_useful', comment: string) => Promise<boolean>
}) {
  const readinessText = readiness?.generation_mode === 'provider'
    ? readiness.ready ? `真实模型 · ${readiness.upstream_model || readiness.model_alias}` : '真实模型尚未就绪'
    : '演示模板'
  if (!draft) return <article className="strategy-panel strategy-document"><header><div><span className="eyebrow">03 · Strategy</span><h2>策略文档</h2></div><span className={`generation-mode generation-mode--${readiness?.generation_mode || 'unknown'}`}>{readinessText}</span></header><div className="panel-empty"><p>确认 Brief 后，基于固定版本生成策略。</p>{readiness && !readiness.ready ? <p className="generation-warning">{generationReadinessMessage(readiness.reason_code)}</p> : null}<button className="button button--primary" disabled={busy || !canGenerate || readiness?.ready === false} onClick={onGenerate} type="button">生成策略</button></div></article>
  if (draft.status === 'generating' || !draft.revision) return <article className="strategy-panel strategy-document"><header><div><span className="eyebrow">03 · Strategy</span><h2>策略文档</h2></div></header><div className="panel-empty"><span className="strategy-spinner" /><p>正在生成结构化策略，刷新或断线不会丢失任务。</p></div></article>
  const document = draft.revision.document
  return <article className="strategy-panel strategy-document">
    <header><div><span className="eyebrow">03 · Strategy</span><h2>策略文档</h2></div><div className="strategy-generation-badges"><span className="revision-chip">Revision {draft.current_revision}</span><span className={`generation-mode generation-mode--${generationMetadata?.generation_mode || 'unknown'}`}>{generationMetadata?.generation_mode === 'provider' ? generationMetadata.model_version : generationMetadata?.generation_mode === 'fake_template' ? 'Fake 模板' : '演示模板'}</span></div></header>
    {generationMetadata ? <div className="generation-proof"><span>Prompt {generationMetadata.prompt_version || 'unknown'}</span><span>规则检查 {generationMetadata.quality_report?.score ?? '—'}</span><span>校验 {generationMetadata.validation_attempts} 次</span>{generationMetadata.usage ? <span>{generationMetadata.usage.total_tokens} tokens</span> : null}</div> : null}
    {document.executive_summary ? <section><h3>执行摘要</h3><p>{document.executive_summary}</p></section> : null}
    <EditableStrategyField disabled={busy || draft.status === 'approved'} label="目标" onSave={(value) => onPatch('objective', value)} value={document.objective} />
    <section><h3>核心受众</h3><p>{document.audience.primary}</p></section>
    <EditableStrategyField disabled={busy || draft.status === 'approved'} label="核心主张" onSave={(value) => onPatch('proposition', value)} value={document.proposition} />
    <section><h3>渠道策略</h3>{document.channel_strategy.map((channel) => <div className="channel-card" key={channel.platform}><strong>{channel.platform === 'xiaohongshu' ? '小红书' : channel.platform}</strong><p>{channel.role}</p><small>{channel.formats.join(' · ')}</small></div>)}</section>
    {document.platform_plans?.length ? <section><h3>分平台执行方案</h3><div className="platform-plan-list">{document.platform_plans.map((plan) => <article key={plan.platform}><header><strong>{platformLabel(plan.platform)}</strong><span>{plan.primary_kpi}</span></header><p>{plan.role}</p><dl><div><dt>内容支柱</dt><dd>{plan.content_pillars.join('、')}</dd></div><div><dt>转化路径</dt><dd>{plan.conversion_path}</dd></div><div><dt>节奏</dt><dd>{plan.cadence}</dd></div></dl><ul>{plan.creative_ideas.map((idea) => <li key={idea}>{idea}</li>)}</ul></article>)}</div></section> : null}
    <section><h3>创意建议</h3><ul>{document.creative_recommendations.map((item) => <li key={item}>{item}</li>)}</ul></section>
    {document.assumptions_and_gaps.length ? <section className="gap-section"><h3>假设与缺口</h3><ul>{document.assumptions_and_gaps.map((item) => <li key={item}>{item}</li>)}</ul></section> : null}
    <section className="skill-run-section"><h3>Skill 与合规运行</h3>{skillRuns.length === 0 ? <p>运行记录正在同步。</p> : <div>{skillRuns.map((run) => <article key={run.id}><span className={`skill-status skill-status--${run.status}`} /> <strong>{run.skill_name}</strong><small>{run.skill_version} · {run.latency_ms}ms</small></article>)}</div>}
      {document.compliance ? <div className={document.compliance.passed ? 'compliance-card compliance-card--passed' : 'compliance-card compliance-card--blocked'}><strong>{document.compliance.passed ? '合规检查通过' : '合规检查存在阻断'}</strong>{document.compliance.issues.length ? <ul>{document.compliance.issues.map((issue) => <li key={`${issue.rule_id}:${issue.evidence || ''}`}>{issue.message}{issue.evidence ? `：${issue.evidence}` : ''}</li>)}</ul> : <small>未发现规则问题。</small>}</div> : null}
    </section>
    <StrategyRevisionForm disabled={busy || draft.status === 'approved'} mode={generationMetadata?.generation_mode || 'deterministic'} onSubmit={onRevise} />
    <footer className="strategy-review-actions">
      {packageVersion ? <div className="published-proof"><strong>策略包 v{packageVersion.version} 已发布</strong><code>{packageVersion.content_hash}</code><button className="button button--primary" disabled={busy || !packageVersion.snapshot.readiness.creative_ready} onClick={onCreateCreative} type="button">创建创意输入</button>{!packageVersion.snapshot.readiness.creative_ready ? <small>Creative 首期只接收含小红书执行方案的策略包。</small> : null}<FeedbackForm busy={busy} onSubmit={onFeedback} /></div> : review?.status === 'open' ? <>
        <div><strong>评审候选已冻结</strong><small>{review.candidate_content_hash}</small></div>
        <button className="button button--primary" disabled={busy} onClick={onApprove} type="button">批准并发布</button>
      </> : <button className="button button--primary" disabled={busy || draft.status === 'approved'} onClick={onSubmit} type="button">提交评审</button>}
    </footer>
  </article>
}

function FeedbackForm({ busy, onSubmit }: {
  busy: boolean
  onSubmit: (rating: 'useful' | 'partly_useful' | 'not_useful', comment: string) => Promise<boolean>
}) {
  const [rating, setRating] = useState<'useful' | 'partly_useful' | 'not_useful'>('useful')
  const [comment, setComment] = useState('')
  const [sent, setSent] = useState(false)
  if (sent) return <p className="feedback-sent">反馈已记录，会进入后续策略评测闭环。</p>
  return <div className="strategy-feedback"><strong>这份策略是否有用？</strong><div><select disabled={busy} onChange={(event) => setRating(event.target.value as typeof rating)} value={rating}><option value="useful">有用</option><option value="partly_useful">部分有用</option><option value="not_useful">无用</option></select><input disabled={busy} onChange={(event) => setComment(event.target.value)} placeholder="可选：哪些内容需要改进" value={comment} /><button className="button" disabled={busy} onClick={async () => { if (await onSubmit(rating, comment)) setSent(true) }} type="button">提交反馈</button></div></div>
}

function StrategyRevisionForm({ disabled, mode, onSubmit }: {
  disabled: boolean
  mode: GenerationMetadata['generation_mode']
  onSubmit: (instruction: string) => Promise<boolean>
}) {
  const [instruction, setInstruction] = useState('')
  const providerEnabled = mode === 'provider'
  return <section className="strategy-revision-form">
    <h3>{providerEnabled ? 'AI 定向修订' : '定向修订'}</h3>
    {!providerEnabled ? <p className="generation-warning">当前是演示模板；接入真实文本 Provider 后才会启用 AI 定向修订。</p> : null}
    <textarea disabled={!providerEnabled} onChange={(event) => setInstruction(event.target.value)} placeholder="例如：把创意建议改得更适合 B2B 技术决策者，其他章节保持不变。" rows={3} value={instruction} />
    <button className="button" disabled={disabled || !providerEnabled || !instruction.trim()} onClick={async () => {
      if (await onSubmit(instruction.trim())) setInstruction('')
    }} type="button">生成新 Revision</button>
  </section>
}

function EditableStrategyField({ label, value, disabled, onSave }: { label: string; value: string; disabled: boolean; onSave: (value: string) => void }) {
  const [editing, setEditing] = useState(false)
  const [next, setNext] = useState(value)
  return <section><div className="strategy-section-title"><h3>{label}</h3>{!editing ? <button className="text-action" disabled={disabled} onClick={() => { setNext(value); setEditing(true) }} type="button">修改</button> : null}</div>
    {editing ? <div className="strategy-inline-editor"><textarea onChange={(event) => setNext(event.target.value)} rows={3} value={next} /><button className="button button--primary" disabled={!next.trim()} onClick={() => { onSave(next); setEditing(false) }} type="button">保存为新 Revision</button></div> : <p>{value}</p>}
  </section>
}

function sourceLabel(source: string) {
  if (source === 'conversation_message') return '来自对话'
  if (source === 'user_edit') return '用户编辑'
  return source
}

function generationReadinessMessage(reason?: string) {
  switch (reason) {
    case 'PROVIDER_CREDENTIAL_UNAVAILABLE':
      return '真实 Provider 的凭证不可用，请先完成凭证配置。'
    case 'MODEL_ROUTE_NOT_FOUND':
      return '未找到 cookies.text.standard 的可用模型路由。'
    case 'MODEL_ROUTE_POLICY_INVALID':
      return '模型路由策略不符合 Strategy 生成要求，请检查响应模式、令牌数和温度。'
    default:
      return '真实 Provider 当前不可用，请检查模型路由和凭证配置。'
  }
}

function statusLabel(status: string) {
  const labels: Record<string, string> = {
    active: '梳理中', waiting_user: '等待补充', ready_to_confirm: '可以确认',
    completed: 'Brief 已确认', generating: '生成中', draft: '策略草稿',
    ready_for_review: '评审中', returned: '已退回', approved: '已批准',
  }
  return labels[status] || status
}

function platformLabel(platform: string) {
  const labels: Record<string, string> = {
    xiaohongshu: '小红书',
    douyin: '抖音',
    taobao_tmall: '淘宝天猫',
    wechat_ecosystem: '微信生态',
  }
  return labels[platform] || platform
}

function formatBytes(bytes: number) {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${Math.ceil(bytes / 1024)} KB`
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`
}

function messageOf(value: unknown) {
  return value instanceof Error ? value.message : '操作失败，请稍后重试。'
}
