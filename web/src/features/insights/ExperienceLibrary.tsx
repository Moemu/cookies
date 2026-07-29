import { useCallback, useEffect, useMemo, useState } from 'react'
import type { InsightExperienceView } from '../../app/routes'
import {
  confirmExperience,
  listExperienceAudits,
  listExperienceLineage,
  listExperienceReferences,
  listProjectExperienceReferences,
  recordExperienceReference,
  rejectExperience,
  requestExperienceReview,
  retireExperience,
  reviseExperience,
} from './api'
import {
  consumerKindLabel,
  consumerKinds,
  experienceStatusLabels,
  groupExperiencesByStatus,
  referenceOutcomeLabels,
  toLines,
} from './experience-format'
import type {
  Experience,
  ExperienceAudit,
  ExperienceReference,
  ExperienceReferenceOutcome,
  ExperienceStatus,
} from './types'

// 需要写明理由的状态流转。理由会进入审计轨迹，让"为什么改结论"始终可追溯。
type ReasonAction = 'reject' | 'request-review' | 'retire'

const reasonActionLabels: Record<ReasonAction, string> = {
  reject: '拒绝沉淀',
  'request-review': '标记待复审',
  retire: '废除经验',
}

// 经验库的四个状态视图对应 URL 上的 view，不再是组件内部的页签状态。
const viewStatus: Record<Exclude<InsightExperienceView, 'references'>, ExperienceStatus> = {
  pending: 'pending',
  confirmed: 'confirmed',
  'needs-review': 'needs_review',
  retired: 'retired',
}

export function ExperienceLibrary({ busy, evidence, experiences, onAction, projectId, view }: {
  busy: boolean
  /** 报告 ID → 证据说明；经验本身只带主键，人话版本在报告里。 */
  evidence: Map<string, string>
  experiences: Experience[]
  onAction: (operation: () => Promise<unknown>) => void
  projectId: string
  view: Exclude<InsightExperienceView, 'references'>
}) {
  const [selectedId, setSelectedId] = useState('')
  const status = viewStatus[view]
  const grouped = useMemo(() => groupExperiencesByStatus(experiences), [experiences])
  const visible = grouped[status]
  // 切换页签后原来选中的经验通常不在新列表里，退回该列表第一条。
  const selected = visible.find((item) => item.id === selectedId) || visible[0] || undefined

  return <div className="outcome-grid">
    <aside className="outcome-list">
      <div className="outcome-list__heading"><strong>{experienceStatusLabels[status]}</strong><span>{visible.length}</span></div>
      {visible.map((item) => <button
        className={`outcome-list__item${selected?.id === item.id ? ' outcome-list__item--active' : ''}`}
        key={item.id}
        onClick={() => setSelectedId(item.id)}
        type="button"
      >
        <strong>{item.conclusion}</strong>
        <span>第 {item.revision} 版</span>
      </button>)}
      {visible.length === 0 ? <p className="outcome-list__empty">当前没有{experienceStatusLabels[status]}的经验。</p> : null}
    </aside>
    <main className="outcome-detail">
      {selected
        ? <ExperienceDetail busy={busy} evidence={evidence.get(selected.report_id) || ''} experience={selected} key={selected.id} onAction={onAction} projectId={projectId} />
        : <div className="outcome-empty"><span aria-hidden="true">◎</span><h2>还没有{experienceStatusLabels[status]}的经验</h2><p>确认复盘报告后填写适用条件与反例，经验会先进入待确认队列，由人工确认后才可被下游引用。</p></div>}
    </main>
  </div>
}

/**
 * 引用记录：整个项目里"哪条经验被谁用过、结果如何"。
 * 它回答的是经验有没有真的产生作用，而不是某一条经验的细节。
 */
export function ExperienceReferencesView({ experiences, projectId }: { experiences: Experience[], projectId: string }) {
  const [references, setReferences] = useState<ExperienceReference[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    const controller = new AbortController()
    const timer = window.setTimeout(() => {
      void listProjectExperienceReferences(projectId, controller.signal)
        .then((value) => { setReferences(value.items); setLoading(false) })
        .catch((caught: unknown) => {
          if (caught instanceof DOMException && caught.name === 'AbortError') return
          setError(caught instanceof Error ? caught.message : '无法读取引用记录。')
          setLoading(false)
        })
    }, 0)
    return () => { window.clearTimeout(timer); controller.abort() }
  }, [projectId])

  const conclusions = useMemo(() => new Map(experiences.map((item) => [item.id, item])), [experiences])

  if (loading) return <div className="outcome-loading" role="status">正在读取引用记录…</div>
  if (error) return <div className="workspace-alert" role="alert"><span>{error}</span></div>
  if (references.length === 0) {
    return <div className="outcome-empty">
      <span aria-hidden="true">◎</span>
      <h2>还没有引用记录</h2>
      <p>下游用了某条经验之后，在这条经验的详情页记录一次引用，这里就能看到它到底被采纳还是被改掉了。</p>
    </div>
  }
  // 披露块和时间线都是整行的内容，不能塞进 .outcome-cards 的分列网格里。
  return <>
    <div className="simulation-banner" role="note">
      <strong>引用记录</strong>
      <span>共 {references.length} 条。引用记录只反映下游主动登记的使用情况，没有登记的引用不会出现在这里。</span>
    </div>
    <section className="outcome-timeline">
      {references.map((item) => {
        const source = conclusions.get(item.experience_id)
        return <article className="timeline-card" key={item.id}>
          <div>
            {/* 先说被引用的是哪条结论，下游对象编号退到后面：编号只有查记录时才有用。 */}
            <strong>{source ? source.conclusion : '这条经验已不在当前列表中'}</strong>
            <span>{item.note || '未填写说明'}</span>
            <span>使用方：{consumerKindLabel(item.consumer_kind)}（编号 {item.consumer_id}）</span>
          </div>
          <span>{referenceOutcomeLabels[item.outcome]}</span>
        </article>
      })}
    </section>
  </>
}

function ExperienceDetail({ busy, evidence, experience, onAction, projectId }: {
  busy: boolean
  evidence: string
  experience: Experience
  onAction: (operation: () => Promise<unknown>) => void
  projectId: string
}) {
  const [lineage, setLineage] = useState<Experience[]>([])
  const [audits, setAudits] = useState<ExperienceAudit[]>([])
  const [references, setReferences] = useState<ExperienceReference[]>([])
  const [panel, setPanel] = useState<ReasonAction | 'revise' | 'reference' | ''>('')
  // 记录引用不会改经验本身的版本号，光靠 experience.version 触发不了重新拉取，
  // 所以另外留一个计数器：写成功之后加一，下面的 effect 才会去刷新引用列表。
  const [refreshToken, setRefreshToken] = useState(0)

  const loadDetail = useCallback(async (signal?: AbortSignal) => {
    const [lineageValue, auditValue, referenceValue] = await Promise.all([
      listExperienceLineage(projectId, experience.id, signal),
      listExperienceAudits(projectId, experience.id, signal),
      listExperienceReferences(projectId, experience.id, signal),
    ])
    setLineage(lineageValue.items)
    setAudits(auditValue.items)
    setReferences(referenceValue.items)
  }, [experience.id, projectId])

  useEffect(() => {
    const controller = new AbortController()
    const timer = window.setTimeout(() => void loadDetail(controller.signal).catch(() => undefined), 0)
    return () => { window.clearTimeout(timer); controller.abort() }
  }, [loadDetail, experience.version, refreshToken])

  const run = (operation: () => Promise<unknown>) => {
    setPanel('')
    onAction(operation)
  }
  const reusable = experience.status === 'confirmed'
  const open = experience.status === 'pending' || experience.status === 'needs_review'

  return <>
    <section className="outcome-detail__hero">
      <div className="outcome-card__top">
        <span className="status-chip status-chip--active">{experienceStatusLabels[experience.status]}</span>
        <span className="status-chip">第 {experience.revision} 版</span>
        {/* 经验库是可追溯视图，记录本身的编号保留在这里，不出现在投前引用页上。 */}
        <code>{experience.id}</code>
      </div>
      <h2>{experience.conclusion}</h2>
      <dl className="outcome-facts">
        <div><dt>适用条件</dt><dd>{experience.conditions.join(' · ') || '未限定'}</dd></div>
        <div><dt>反例 / 限制</dt><dd>{experience.counterexamples.join(' · ') || '暂无'}</dd></div>
        <div><dt>最近状态说明</dt><dd>{experience.status_reason || '无'}</dd></div>
      </dl>
      <div className="simulation-banner" role="note">
        <strong>这条结论来自哪次投放</strong>
        <span>{evidence || '来源报告不在当前项目的报告列表里，无法展示证据说明。'}</span>
      </div>
      <div className="simulation-banner" role="note">
        <strong>可引用性</strong>
        <span>{reusable
          ? '已确认且未失效，下游环节可以引用该结论。'
          : '未确认或已失效的经验不会出现在投前引用中，但会保留在库内可追溯。'}</span>
      </div>
      <div className="report-card__actions">
        {open ? <button className="button button--primary button--compact" disabled={busy} type="button"
          onClick={() => run(() => confirmExperience(projectId, experience.id, experience.version))}>确认经验</button> : null}
        {experience.status === 'pending' ? <button className="button button--secondary button--compact" disabled={busy} type="button"
          onClick={() => setPanel('reject')}>拒绝沉淀</button> : null}
        {reusable ? <button className="button button--secondary button--compact" disabled={busy} type="button"
          onClick={() => setPanel('request-review')}>标记待复审</button> : null}
        {experience.status !== 'retired' && !experience.superseded_by_id ? <button className="button button--secondary button--compact" disabled={busy} type="button"
          onClick={() => setPanel('revise')}>修订出新版本</button> : null}
        {reusable || experience.status === 'needs_review' ? <button className="button button--secondary button--compact" disabled={busy} type="button"
          onClick={() => setPanel('retire')}>废除经验</button> : null}
        {reusable ? <button className="button button--secondary button--compact" disabled={busy} type="button"
          onClick={() => setPanel('reference')}>记录下游引用</button> : null}
      </div>
    </section>

    {panel === 'reject' || panel === 'request-review' || panel === 'retire'
      ? <ReasonForm action={panel} busy={busy} onCancel={() => setPanel('')} onSubmit={(reason) => {
        const apply = panel === 'reject' ? rejectExperience : panel === 'request-review' ? requestExperienceReview : retireExperience
        run(() => apply(projectId, experience.id, experience.version, reason))
      }} /> : null}

    {panel === 'revise' ? <ReviseForm busy={busy} experience={experience} onCancel={() => setPanel('')} onSubmit={(input) =>
      run(() => reviseExperience(projectId, experience.id, { expected_version: experience.version, ...input }))} /> : null}

    {panel === 'reference' ? <ReferenceForm busy={busy} onCancel={() => setPanel('')} onSubmit={(input) =>
      run(async () => {
        const created = await recordExperienceReference(projectId, experience.id, input)
        setRefreshToken((value) => value + 1)
        return created
      })} /> : null}

    {lineage.length > 1 ? <section className="outcome-timeline">
      <h3>版本链</h3>
      {lineage.map((item) => <article className="timeline-card" key={item.id}>
        <div><strong>第 {item.revision} 版 · {experienceStatusLabels[item.status]}</strong><span>{item.conclusion}</span></div>
        <span>{item.id === experience.id ? '当前查看' : ''}</span>
      </article>)}
    </section> : null}

    <section className="outcome-timeline">
      <h3>下游引用与反馈</h3>
      {references.map((item) => <article className="timeline-card" key={item.id}>
        <div>
          <strong>{consumerKindLabel(item.consumer_kind)}（编号 {item.consumer_id}）</strong>
          <span>{item.note || '未填写说明'}</span>
        </div>
        <span>{referenceOutcomeLabels[item.outcome]}</span>
      </article>)}
      {references.length === 0 ? <p className="outcome-list__empty">还没有下游引用记录。</p> : null}
    </section>

    <section className="outcome-timeline">
      <h3>状态轨迹</h3>
      {audits.map((item) => <article className="timeline-card" key={item.id}>
        <div>
          <strong>{item.from_status ? experienceStatusLabels[item.from_status] : '沉淀'} → {experienceStatusLabels[item.to_status]}</strong>
          <span>{item.reason || '未填写理由'}</span>
        </div>
        <span>{new Date(item.created_at).toLocaleString()}</span>
      </article>)}
    </section>
  </>
}

function ReasonForm({ action, busy, onCancel, onSubmit }: {
  action: ReasonAction
  busy: boolean
  onCancel: () => void
  onSubmit: (reason: string) => void
}) {
  const [reason, setReason] = useState('')
  return <form className="report-composer" onSubmit={(event) => { event.preventDefault(); onSubmit(reason.trim()) }}>
    <span className="page-eyebrow">{reasonActionLabels[action]}</span>
    <h2>说明这次状态变更的原因</h2>
    <label>理由<textarea maxLength={1000} onChange={(event) => setReason(event.target.value)} required value={reason} /></label>
    <div className="report-card__actions">
      <button className="button button--primary button--compact" disabled={busy || !reason.trim()} type="submit">提交</button>
      <button className="button button--secondary button--compact" onClick={onCancel} type="button">取消</button>
    </div>
  </form>
}

function ReviseForm({ busy, experience, onCancel, onSubmit }: {
  busy: boolean
  experience: Experience
  onCancel: () => void
  onSubmit: (input: { conclusion: string, conditions: string[], counterexamples: string[], reason: string }) => void
}) {
  const [conclusion, setConclusion] = useState(experience.conclusion)
  const [conditions, setConditions] = useState(experience.conditions.join('\n'))
  const [counterexamples, setCounterexamples] = useState(experience.counterexamples.join('\n'))
  const [reason, setReason] = useState('')
  return <form className="report-composer" onSubmit={(event) => {
    event.preventDefault()
    onSubmit({ conclusion: conclusion.trim(), conditions: toLines(conditions), counterexamples: toLines(counterexamples), reason: reason.trim() })
  }}>
    <span className="page-eyebrow">修订版本</span>
    <h2>修订为第 {experience.revision + 1} 版</h2>
    <p>修订不会覆盖原结论。新版本先进入待确认，确认后原版本才会自动失效。</p>
    <label>结论<textarea maxLength={2000} onChange={(event) => setConclusion(event.target.value)} required value={conclusion} /></label>
    <label>适用条件（每行一条）<textarea onChange={(event) => setConditions(event.target.value)} value={conditions} /></label>
    <label>反例 / 限制（每行一条）<textarea onChange={(event) => setCounterexamples(event.target.value)} value={counterexamples} /></label>
    <label>修订原因<textarea maxLength={1000} onChange={(event) => setReason(event.target.value)} value={reason} /></label>
    <div className="report-card__actions">
      <button className="button button--primary button--compact" disabled={busy || !conclusion.trim()} type="submit">创建修订版本</button>
      <button className="button button--secondary button--compact" onClick={onCancel} type="button">取消</button>
    </div>
  </form>
}

function ReferenceForm({ busy, onCancel, onSubmit }: {
  busy: boolean
  onCancel: () => void
  onSubmit: (input: { consumer_kind: string, consumer_id: string, outcome: ExperienceReferenceOutcome, note: string }) => void
}) {
  const [consumerKind, setConsumerKind] = useState('strategy')
  const [consumerId, setConsumerId] = useState('')
  const [outcome, setOutcome] = useState<ExperienceReferenceOutcome>('adopted')
  const [note, setNote] = useState('')
  return <form className="report-composer" onSubmit={(event) => {
    event.preventDefault()
    onSubmit({ consumer_kind: consumerKind.trim(), consumer_id: consumerId.trim(), outcome, note: note.trim() })
  }}>
    <span className="page-eyebrow">下游反馈</span>
    <h2>记录这条经验被谁引用、结果如何</h2>
    {/* 下游类型后端存的是英文取值，这里只让人选中文，不让人手打 strategy 之类的词。 */}
    <div className="outcome-form-grid">
      <label>用在哪里<select onChange={(event) => setConsumerKind(event.target.value)} value={consumerKind}>
        {consumerKinds.map((key) => <option key={key} value={key}>{consumerKindLabel(key)}</option>)}
      </select></label>
      <label>对象编号<input maxLength={96} onChange={(event) => setConsumerId(event.target.value)}
        placeholder="填那个策略包 / 任务 / 计划的编号" required value={consumerId} /></label>
    </div>
    <label>采纳结果<select onChange={(event) => setOutcome(event.target.value as ExperienceReferenceOutcome)} value={outcome}>
      {(Object.keys(referenceOutcomeLabels) as ExperienceReferenceOutcome[]).map((key) =>
        <option key={key} value={key}>{referenceOutcomeLabels[key]}</option>)}
    </select></label>
    <label>补充说明<textarea maxLength={1000} onChange={(event) => setNote(event.target.value)} value={note} /></label>
    <div className="report-card__actions">
      <button className="button button--primary button--compact" disabled={busy || !consumerId.trim()} type="submit">记录引用</button>
      <button className="button button--secondary button--compact" onClick={onCancel} type="button">取消</button>
    </div>
  </form>
}
