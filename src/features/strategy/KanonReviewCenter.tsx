import { BadgeCheck, CircleAlert, Clock3, ShieldCheck, UserRoundCheck, UsersRound } from 'lucide-react'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useProject } from '../../context/ProjectContext'
import { strategyApi } from './api'
import type { Review, ReviewPolicy } from './types'

export function KanonReviewCenter({ activeView, onOpenReview }: {
  activeView: string
  onOpenReview: (reviewId: string) => void
}) {
  const { currentProject } = useProject()
  const [reviews, setReviews] = useState<Review[]>([])
  const [policy, setPolicy] = useState<ReviewPolicy | null>(null)
  const [mode, setMode] = useState<ReviewPolicy['mode']>('self_confirmation')
  const [approvers, setApprovers] = useState('')
  const [allowSelf, setAllowSelf] = useState(false)
  const [busy, setBusy] = useState('')
  const [error, setError] = useState('')
  const policyFormDirty = useRef(false)

  useEffect(() => {
    const controller = new AbortController()
    const filter = activeView === '待我评审' ? 'assigned_to_me'
      : activeView === '我发起的' ? 'requested_by_me'
      : 'all'
    setReviews([])
    void strategyApi.listReviews(currentProject.id, filter, '', controller.signal).then(reviewResult => {
      setReviews(reviewResult.items)
    }).catch(cause => {
      if (!(cause instanceof DOMException && cause.name === 'AbortError')) {
        setError(cause instanceof Error ? cause.message : '评审中心读取失败。')
      }
    })
    return () => controller.abort()
  }, [activeView, currentProject.id])

  useEffect(() => {
    const controller = new AbortController()
    policyFormDirty.current = false
    setPolicy(null)
    setMode('self_confirmation')
    setApprovers('')
    setAllowSelf(false)
    void strategyApi.getReviewPolicy(currentProject.id, controller.signal).then(reviewPolicy => {
      setPolicy(reviewPolicy)
      // A slow initial read may finish after the user has started editing.
      // Keep the newer local intent; only untouched forms are hydrated.
      if (policyFormDirty.current) return
      setMode(reviewPolicy.mode)
      setApprovers(reviewPolicy.approver_user_ids.join(', '))
      setAllowSelf(reviewPolicy.allow_self_approval)
    }).catch(cause => {
      if (!(cause instanceof DOMException && cause.name === 'AbortError')) {
        setError(cause instanceof Error ? cause.message : '评审策略读取失败。')
      }
    })
    return () => controller.abort()
  }, [currentProject.id])

  const visible = useMemo(() => {
    if (activeView === '已完成') return reviews.filter(review => review.status !== 'open')
    if (activeView === '待我评审') return reviews.filter(review => review.status === 'open')
    if (activeView === '评论与提及' || activeView === '变更记录') return []
    return reviews
  }, [activeView, reviews])

  const savePolicy = async () => {
    if (!policy) return
    setBusy('policy')
    setError('')
    try {
      const value = await strategyApi.updateReviewPolicy(currentProject.id, {
        mode,
        approver_user_ids: mode === 'self_confirmation'
          ? []
          : approvers.split(/[,，\n]/).map(value => value.trim()).filter(Boolean),
        allow_self_approval: mode === 'self_confirmation' ? true : allowSelf,
        version: policy.version,
      })
      setPolicy(value)
      policyFormDirty.current = false
      setMode(value.mode)
      setApprovers(value.approver_user_ids.join(', '))
      setAllowSelf(value.allow_self_approval)
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : '评审策略保存失败。')
    } finally {
      setBusy('')
    }
  }

  return <div className="kanon-review-center">
    <section>
      <div className="kanon-strategy-heading">
        <div><span className="section-label">REVIEW QUEUE</span><h2>{activeView}</h2><p>列表来自真实 Strategy Review 与 ReviewAssignment。</p></div>
        <span className="source-chip">{visible.length} 条</span>
      </div>
      {error ? <div className="kanon-strategy-alert" role="alert"><CircleAlert size={15}/>{error}</div> : null}
      {(activeView === '评论与提及' || activeView === '变更记录') ? <div className="kanon-strategy-state">
        <CircleAlert size={22}/><h2>{activeView}聚合尚未开放</h2><p>当前评论和变更可以在具体评审及策略工作区内查看。</p>
      </div> : <div className="kanon-review-queue">
        {visible.map(review => <button key={review.id} onClick={() => onOpenReview(review.id)}>
          <span>{review.review_mode === 'self_confirmation' ? <UserRoundCheck size={18}/> : <UsersRound size={18}/>}</span>
          <div><b>Strategy Revision {review.candidate_revision}</b><small>{review.id} · 发起人 {review.created_by}</small></div>
          <em>{reviewModeLabel(review.review_mode)}</em>
          <strong>{reviewStatusLabel(review.status)}</strong>
        </button>)}
        {!visible.length ? <div className="panel-empty">当前筛选没有评审记录。</div> : null}
      </div>}
    </section>
    <aside>
      <div className="surface-toolbar"><h3>当前 Project 评审策略</h3><ShieldCheck size={16}/></div>
      <label>评审模式<select disabled={Boolean(busy)} value={mode} onChange={event => {
        policyFormDirty.current = true
        setMode(event.target.value as ReviewPolicy['mode'])
      }}>
        <option value="self_confirmation">个人确认</option>
        <option value="leader_approval">Leader 评审</option>
        <option value="designated_approvers">指定审批人</option>
      </select></label>
      {mode !== 'self_confirmation' ? <label>审批人 User ID<textarea disabled={Boolean(busy)} rows={4} value={approvers} onChange={event => {
        policyFormDirty.current = true
        setApprovers(event.target.value)
      }} placeholder={mode === 'leader_approval' ? '留空自动使用组织 owner / admin' : '多个用户以逗号或换行分隔'}/></label> : null}
      {mode !== 'self_confirmation' ? <label className="kanon-check"><input checked={allowSelf} disabled={Boolean(busy)} type="checkbox" onChange={event => {
        policyFormDirty.current = true
        setAllowSelf(event.target.checked)
      }}/><span>允许发起人同时作为指定审批人</span></label> : null}
      <button className="primary-button full" disabled={!policy || Boolean(busy) || (mode === 'designated_approvers' && !approvers.trim())} onClick={() => void savePolicy()}><BadgeCheck size={15}/>{busy ? '保存中…' : !policy ? '正在加载评审策略…' : '保存评审策略'}</button>
      <div className="kanon-policy-summary">
        <Clock3 size={16}/><div><b>策略按提交时快照</b><p>修改设置不会改变已经打开的 Review Assignment。</p></div>
      </div>
    </aside>
  </div>
}

function reviewModeLabel(value?: Review['review_mode']) {
  if (value === 'leader_approval') return 'Leader 评审'
  if (value === 'designated_approvers') return '指定审批'
  return '个人确认'
}

function reviewStatusLabel(value: Review['status']) {
  const labels: Record<Review['status'], string> = {
    open: '待处理',
    returned: '已退回',
    approved: '已批准',
    invalidated: '已失效',
  }
  return labels[value]
}
