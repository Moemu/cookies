import { useEffect, useMemo, useRef, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import {
  addReviewComment,
  approveStrategy,
  createMutationKey,
  getReview,
  getStrategy,
  listReviewComments,
  listStrategyRevisions,
  returnReview,
} from './api'
import type { DraftRevision, Review, ReviewComment, StrategyDraft } from './types'
import './strategy.css'

export function StrategyReviewPage() {
  const { projectId = '', reviewId = '' } = useParams()
  const navigate = useNavigate()
  const [review, setReview] = useState<Review | null>(null)
  const [draft, setDraft] = useState<StrategyDraft | null>(null)
  const [revisions, setRevisions] = useState<DraftRevision[]>([])
  const [comments, setComments] = useState<ReviewComment[]>([])
  const [comment, setComment] = useState('')
  const [returnReason, setReturnReason] = useState('')
  const [busy, setBusy] = useState('')
  const [error, setError] = useState('')
  const approvalKey = useRef(createMutationKey())

  useEffect(() => {
    const controller = new AbortController()
    getReview(reviewId, controller.signal).then(async (value) => {
      if (value.project_id !== projectId) throw new Error('评审不属于当前 Project。')
      const [strategy, revisionResult, commentResult] = await Promise.all([
        getStrategy(value.strategy_id, controller.signal),
        listStrategyRevisions(value.strategy_id, controller.signal),
        listReviewComments(value.id, controller.signal),
      ])
      setReview(value)
      setDraft(strategy)
      setRevisions(revisionResult.items)
      setComments(commentResult.items)
    }).catch((cause: unknown) => {
      if (!(cause instanceof DOMException && cause.name === 'AbortError')) {
        setError(cause instanceof Error ? cause.message : '评审加载失败。')
      }
    })
    return () => controller.abort()
  }, [projectId, reviewId])

  const candidate = revisions.find((value) => value.revision === review?.candidate_revision)
  const base = revisions.find((value) => value.revision === (review?.candidate_revision || 1) - 1)
  const diffs = useMemo(() => strategyDiff(base, candidate), [base, candidate])

  async function run(name: string, action: () => Promise<void>) {
    setBusy(name)
    setError('')
    try {
      await action()
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : '操作失败。')
    } finally {
      setBusy('')
    }
  }

  if (!review || !draft || !candidate) {
    return <section className="strategy-empty"><h1>策略评审</h1><p>{error || '正在加载候选版本…'}</p></section>
  }

  return <section className="strategy-page strategy-object-page">
    <header className="strategy-header"><div><nav><Link to={`/projects/${projectId}/strategy/workspaces`}>策略</Link><span>/</span><span>评审 {review.id}</span></nav><h1>Revision {review.candidate_revision} 评审</h1><p>差异、证据、评论和人工决策绑定同一候选内容哈希。</p></div><span className="strategy-status">{review.status}</span></header>
    {error ? <div className="strategy-alert" role="alert">{error}</div> : null}
    <div className="review-layout">
      <main className="review-diff">
        <div className="review-proof"><span>候选哈希</span><code>{review.candidate_content_hash}</code></div>
        {diffs.length === 0 ? <p>首个 Revision 无前序版本，以下为完整候选内容。</p> : diffs.map((diff) => <article key={diff.section}><h2>{sectionLabel(diff.section)}</h2><div><section><small>之前</small><pre>{diff.before}</pre></section><section><small>候选</small><pre>{diff.after}</pre></section></div></article>)}
      </main>
      <aside className="review-sidebar">
        <section><h2>评论</h2>{comments.length === 0 ? <p>尚无评论。</p> : comments.map((value) => <article key={value.id}><strong>{value.author_id}</strong><p>{value.body}</p><small>{new Date(value.created_at).toLocaleString()}</small></article>)}
          {review.status === 'open' ? <form onSubmit={(event) => {
            event.preventDefault()
            const body = comment.trim()
            if (!body) return
            void run('comment', async () => {
              const value = await addReviewComment(review.id, body)
              setComments((current) => [...current, value])
              setComment('')
            })
          }}><textarea disabled={busy !== ''} onChange={(event) => setComment(event.target.value)} placeholder="对候选版本留下可执行评论" rows={3} value={comment} /><button className="button" disabled={busy !== '' || !comment.trim()} type="submit">添加评论</button></form> : null}
        </section>
        <section><h2>决策</h2>{review.status === 'open' ? <>
          <button className="button button--primary" disabled={busy !== ''} onClick={() => void run('approve', async () => {
            const current = await getStrategy(draft.id)
            const value = await approveStrategy(current, review, approvalKey.current)
            navigate(`/projects/${projectId}/strategy/packages/${value.package_id}`)
          })} type="button">批准并生成 StrategyPackage</button>
          <textarea disabled={busy !== ''} onChange={(event) => setReturnReason(event.target.value)} placeholder="退回原因（必填）" rows={3} value={returnReason} />
          <button className="button" disabled={busy !== '' || !returnReason.trim()} onClick={() => void run('return', async () => {
            setReview(await returnReview(review.id, returnReason.trim()))
          })} type="button">退回修改</button>
        </> : <p>该评审已{review.status === 'approved' ? '批准' : '结束'}。</p>}</section>
      </aside>
    </div>
  </section>
}

function strategyDiff(base?: DraftRevision, candidate?: DraftRevision) {
  if (!candidate) return []
  const before = base?.document || {} as DraftRevision['document']
  const sections = candidate.changed_sections.includes('all')
    ? Object.keys(candidate.document).filter((key) => key !== 'lineage')
    : candidate.changed_sections
  return sections.map((section) => ({
    section,
    before: pretty((before as unknown as Record<string, unknown>)[section]),
    after: pretty((candidate.document as unknown as Record<string, unknown>)[section]),
  })).filter((value) => value.before !== value.after)
}

function pretty(value: unknown) {
  if (value === undefined) return '—'
  return typeof value === 'string' ? value : JSON.stringify(value, null, 2)
}

function sectionLabel(value: string) {
  const labels: Record<string, string> = {
    objective: '目标', audience: '受众', proposition: '核心主张',
    channel_strategy: '渠道策略', creative_recommendations: '创意建议',
    constraints: '约束', budget_and_cadence: '预算与节奏',
    experiment_matrix: '实验矩阵', measurement: '衡量指标',
    assumptions_and_gaps: '假设与缺口', executive_summary: '执行摘要',
    cross_platform_role: '跨平台协同', platform_plans: '分平台方案',
    evidence_refs: '证据引用', compliance: '合规报告',
  }
  return labels[value] || value
}
