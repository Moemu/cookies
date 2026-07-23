import { useCallback, useEffect, useRef, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import type { Project } from '../platform/types'
import {
  approveStrategy,
  createMutationKey,
  confirmBrief,
  createConversation,
  createStrategy,
  getBriefDraft,
  getGenerationMetadata,
  getGenerationReadiness,
  getReview,
  getStrategy,
  getWorkspace,
  listBriefVersions,
  listMessages,
  listStrategyPackages,
  patchBriefField,
  patchStrategySection,
  reviseStrategy,
  sendMessage,
  submitStrategy,
} from './api'
import { useConversationStream } from './conversation/useConversationStream'
import { createCreativeIntakeFromStrategy, listCreativeIntakes } from '../creative/api'
import type { BriefDraft, BriefVersion, GenerationMetadata, GenerationReadiness, Message, PackageVersion, Review, StrategyDraft, WorkspaceDetail } from './types'
import './strategy.css'

export function StrategyWorkspacePage({ project }: { project?: Project }) {
  const { projectId = '', workspaceId = '' } = useParams()
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
  const [error, setError] = useState('')
  const [busy, setBusy] = useState('')
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
    const [messageResult, briefResult, packageResult, readinessResult] = await Promise.all([
      conversation ? listMessages(conversation.id, signal) : Promise.resolve({ items: [] }),
      task ? getBriefDraft(task.id, signal) : Promise.resolve(null),
      listStrategyPackages(projectId, signal),
      getGenerationReadiness(projectId, signal),
    ])
    setMessages(messageResult.items)
    setBrief(briefResult)
    setReadiness(readinessResult)
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
    if (!draft || draft.status !== 'generating') return
    const timer = window.setInterval(() => {
      getStrategy(draft.id).then(setDraft).catch((cause: unknown) => setError(messageOf(cause)))
    }, 1500)
    return () => window.clearInterval(timer)
  }, [draft])

  const task = detail?.current_task
  const conversation = detail?.current_conversation
  const run = async (name: string, action: () => Promise<void>) => {
    setBusy(name)
    setError('')
    try {
      await action()
    } catch (cause) {
      setError(messageOf(cause))
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
    <section className="strategy-page">
      <header className="strategy-header">
        <div>
          <nav aria-label="策略面包屑"><Link to={`/strategy/projects/${projectId}`}>策略</Link><span>/</span><span>{detail.workspace.name}</span></nav>
          <h1>{project.name} · 策略工作区</h1>
          <p>从对话梳理 Brief，确认后生成、评审并发布不可变策略包。</p>
        </div>
        <span className="strategy-status">{published ? `已发布 v${published.version}` : draft ? statusLabel(draft.status) : task ? statusLabel(task.status) : '尚未开始'}</span>
      </header>

      {error ? <div className="strategy-alert" role="alert">{error}</div> : null}

      {!conversation || !task ? (
        <div className="strategy-start">
          <h2>开始一次策略梳理</h2>
          <p>系统会创建一条持久对话和可追溯 Brief 草稿。</p>
          <button className="button button--primary" disabled={busy !== ''} onClick={() => run('conversation', async () => {
            const bundle = await createConversation(projectId, workspaceId)
            setDetail({ ...detail, current_conversation: bundle.conversation, current_task: bundle.task })
            setBrief(bundle.brief_draft)
            navigate(`/strategy/projects/${projectId}/workspaces/${workspaceId}/conversation`, { replace: true })
          })} type="button">开始梳理</button>
        </div>
      ) : (
        <div className="strategy-grid">
          <ConversationPane
            busy={busy === 'message'}
            messages={messages}
            onSend={(content) => run('message', async () => {
              await sendMessage(conversation.id, content)
              const result = await listMessages(conversation.id)
              setMessages(result.items)
            })}
          />
          <BriefPane
            brief={brief}
            busy={busy !== ''}
            onConfirm={() => brief && run('confirm', async () => {
              const value = await confirmBrief(task.id, brief.version)
              setBriefVersion(value)
              setBrief({ ...brief, status: 'confirmed' })
              setDetail({
                ...detail,
                current_task: detail.current_task ? { ...detail.current_task, status: 'completed' } : undefined,
              })
            })}
            onField={(path, value) => brief && run(`brief:${path}`, async () => {
              setBrief(await patchBriefField(task.id, brief, path, value))
            })}
          />
          <StrategyPane
            busy={busy !== ''}
            canGenerate={briefVersion !== null}
            draft={draft}
            generationMetadata={generationMetadata}
            packageVersion={published}
            readiness={readiness}
            review={review}
            onApprove={() => draft && review && run('approve', async () => {
              if (!approveMutationKey.current) approveMutationKey.current = createMutationKey()
              const current = await getStrategy(draft.id)
              setDraft(current)
              setPublished(await approveStrategy(current, review, approveMutationKey.current))
              approveMutationKey.current = ''
              setDraft({ ...current, status: 'approved' })
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
              navigate(`/projects/${projectId}/creative`)
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
            })}
          />
        </div>
      )}
    </section>
  )
}

function ConversationPane({ busy, messages, onSend }: { busy: boolean; messages: Message[]; onSend: (content: string) => void }) {
  const [content, setContent] = useState('')
  return <article className="strategy-panel conversation-panel">
    <header><div><span className="eyebrow">01 · 对话</span><h2>需求梳理</h2></div><span className="live-dot">实时同步</span></header>
    <div className="conversation-list" aria-live="polite">
      {messages.length === 0 ? <div className="conversation-welcome"><strong>先说说这次广告要解决什么问题</strong><p>建议包含目标、受众、核心卖点；预算和排期可以稍后补充。</p></div> : null}
      {messages.map((message) => <div className={`strategy-message strategy-message--${message.role}`} key={message.id}>
        <span>{message.role === 'user' ? '你' : 'Strategy'}</span><p>{message.content}</p>
      </div>)}
    </div>
    <form className="strategy-composer" onSubmit={(event) => {
      event.preventDefault()
      const value = content.trim()
      if (!value) return
      onSend(value)
      setContent('')
    }}>
      <label htmlFor="strategy-message">补充需求</label>
      <textarea id="strategy-message" onChange={(event) => setContent(event.target.value)} placeholder="例如：目标是新品认知；受众：制造企业研发负责人；卖点：缩短研发周期…" rows={4} value={content} />
      <button className="button button--primary" disabled={busy || !content.trim()} type="submit">{busy ? '正在整理…' : '发送并整理'}</button>
    </form>
  </article>
}

function BriefPane({ brief, busy, onField, onConfirm }: {
  brief: BriefDraft | null
  busy: boolean
  onField: (path: string, value: unknown) => void
  onConfirm: () => void
}) {
  if (!brief) return <article className="strategy-panel"><header><div><span className="eyebrow">02 · Brief</span><h2>需求结构</h2></div></header><p className="panel-empty">等待对话开始。</p></article>
  const fields: Array<{ path: string; label: string; value: string }> = [
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
        <label>首期平台</label>
        <div className="platform-chip">小红书图文 <span>已冻结</span></div>
        {brief.field_states.channels?.confirmation !== 'confirmed' ? <button className="text-action" disabled={busy} onClick={() => onField('channels', ['xiaohongshu'])} type="button">确认该字段</button> : <small className="confirmed-mark">✓ 已确认</small>}
      </div>
    </div>
    {brief.completeness.warnings.length ? <div className="brief-warnings">{brief.completeness.warnings.map((warning) => <p key={warning.field}>{warning.reason}</p>)}</div> : null}
    <footer>
      <span>草稿 v{brief.version} · {brief.status === 'confirmed' ? '已冻结' : '可编辑'}</span>
      <button className="button button--primary" disabled={busy || !brief.completeness.ready || brief.status !== 'open'} onClick={onConfirm} type="button">{brief.status === 'confirmed' ? 'Brief 已确认' : '确认 Brief'}</button>
    </footer>
  </article>
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

function StrategyPane({ draft, review, packageVersion, readiness, generationMetadata, busy, canGenerate, onGenerate, onPatch, onRevise, onSubmit, onApprove, onCreateCreative }: {
  draft: StrategyDraft | null
  review: Review | null
  packageVersion: PackageVersion | null
  readiness: GenerationReadiness | null
  generationMetadata: GenerationMetadata | null
  busy: boolean
  canGenerate: boolean
  onGenerate: () => void
  onPatch: (section: string, value: unknown) => void
  onRevise: (instruction: string) => Promise<boolean>
  onSubmit: () => void
  onApprove: () => void
  onCreateCreative: () => void
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
    <EditableStrategyField disabled={busy || draft.status === 'approved'} label="目标" onSave={(value) => onPatch('objective', value)} value={document.objective} />
    <section><h3>核心受众</h3><p>{document.audience.primary}</p></section>
    <EditableStrategyField disabled={busy || draft.status === 'approved'} label="核心主张" onSave={(value) => onPatch('proposition', value)} value={document.proposition} />
    <section><h3>渠道策略</h3>{document.channel_strategy.map((channel) => <div className="channel-card" key={channel.platform}><strong>{channel.platform === 'xiaohongshu' ? '小红书' : channel.platform}</strong><p>{channel.role}</p><small>{channel.formats.join(' · ')}</small></div>)}</section>
    <section><h3>创意建议</h3><ul>{document.creative_recommendations.map((item) => <li key={item}>{item}</li>)}</ul></section>
    {document.assumptions_and_gaps.length ? <section className="gap-section"><h3>假设与缺口</h3><ul>{document.assumptions_and_gaps.map((item) => <li key={item}>{item}</li>)}</ul></section> : null}
    <StrategyRevisionForm disabled={busy || draft.status === 'approved'} mode={generationMetadata?.generation_mode || 'deterministic'} onSubmit={onRevise} />
    <footer className="strategy-review-actions">
      {packageVersion ? <div className="published-proof"><strong>策略包 v{packageVersion.version} 已发布</strong><code>{packageVersion.content_hash}</code><button className="button button--primary" disabled={busy || !packageVersion.snapshot.readiness.creative_ready} onClick={onCreateCreative} type="button">创建创意输入</button>{!packageVersion.snapshot.readiness.creative_ready ? <small>该策略包尚未满足创意输入条件。</small> : null}</div> : review?.status === 'open' ? <>
        <div><strong>评审候选已冻结</strong><small>{review.candidate_content_hash}</small></div>
        <button className="button button--primary" disabled={busy} onClick={onApprove} type="button">批准并发布</button>
      </> : <button className="button button--primary" disabled={busy || draft.status === 'approved'} onClick={onSubmit} type="button">提交评审</button>}
    </footer>
  </article>
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

function messageOf(value: unknown) {
  return value instanceof Error ? value.message : '操作失败，请稍后重试。'
}
