import type { ReactNode } from 'react'
import { CircleAlert, FileQuestion, LockKeyhole, RefreshCw } from 'lucide-react'
import type { DataState } from '../types'

export function StatePreview({ value, onChange }: { value: DataState; onChange: (value: DataState) => void }) {
  const options: Array<{ value: DataState; label: string }> = [{ value: 'ready', label: '正常' }, { value: 'loading', label: '加载' }, { value: 'empty', label: '空数据' }, { value: 'error', label: '错误' }, { value: 'forbidden', label: '无权限' }]
  return <div className="state-preview" aria-label="页面状态预览"><span>状态预览</span>{options.map(option => <button key={option.value} className={value === option.value ? 'active' : ''} onClick={() => onChange(option.value)} aria-pressed={value === option.value}>{option.label}</button>)}</div>
}

interface StateBoundaryProps {
  state: DataState
  children: ReactNode
  contextLabel?: string
  emptyTitle?: string
  emptyDetail?: string
  errorTitle?: string
  errorDetail?: string
  forbiddenTitle?: string
  forbiddenDetail?: string
  createLabel?: string
  retryLabel?: string
  onRetry?: () => void
  onCreate?: () => void
}

export function StateBoundary({
  state,
  children,
  contextLabel = '当前页面',
  emptyTitle,
  emptyDetail,
  errorTitle,
  errorDetail,
  forbiddenTitle,
  forbiddenDetail,
  createLabel = '创建第一项',
  retryLabel = '重新加载',
  onRetry,
  onCreate,
}: StateBoundaryProps) {
  if (state === 'loading') return <div className="state-surface skeleton-state" role="status" aria-live="polite" aria-busy="true" aria-label={`${contextLabel}正在加载`}><span/><span/><span/><span/><span/></div>
  if (state === 'empty') return <div className="state-surface state-empty" role="status" aria-live="polite" aria-label={`${contextLabel}没有可用内容`}><FileQuestion size={28} aria-hidden="true"/><span className="state-context">{contextLabel}</span><h2>{emptyTitle ?? '还没有可用内容'}</h2><p>{emptyDetail ?? '当前 Project 暂无此类服务端记录。先完成上游任务或创建第一项后，这里会展示版本、证据和后续动作。'}</p>{onCreate ? <button className="primary-button" onClick={onCreate}>{createLabel}</button> : null}</div>
  if (state === 'error') return <div className="state-surface state-error" role="alert" aria-label={`${contextLabel}服务不可用`}><CircleAlert size={28} aria-hidden="true"/><span className="state-context">{contextLabel}</span><h2>{errorTitle ?? '服务暂时不可用'}</h2><p>{errorDetail ?? '最近一次读取失败，现有数据没有被覆盖。请重新加载；如果仍失败，请检查本地 API 服务和网络连接。'}</p>{onRetry ? <button className="secondary-button" onClick={onRetry}><RefreshCw size={15} aria-hidden="true"/>{retryLabel}</button> : null}</div>
  if (state === 'forbidden') return <div className="state-surface state-forbidden" role="alert" aria-label={`${contextLabel}无权限`}><LockKeyhole size={28} aria-hidden="true"/><span className="state-context">{contextLabel}</span><h2>{forbiddenTitle ?? '当前角色没有访问权限'}</h2><p>{forbiddenDetail ?? '需要 Project 管理员授予查看、编辑或审批权限。权限申请不会改变现有内容，也不会触发投放动作。'}</p></div>
  return <>{children}</>
}
