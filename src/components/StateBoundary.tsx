import type { ReactNode } from 'react'
import { CircleAlert, FileQuestion, LockKeyhole, RefreshCw } from 'lucide-react'
import type { DataState } from '../types'

export function StatePreview({ value, onChange }: { value: DataState; onChange: (value: DataState) => void }) {
  const options: Array<{ value: DataState; label: string }> = [{ value: 'ready', label: '正常' }, { value: 'loading', label: '加载' }, { value: 'empty', label: '空数据' }, { value: 'error', label: '错误' }, { value: 'forbidden', label: '无权限' }]
  return <div className="state-preview" aria-label="页面状态预览"><span>Mock 状态</span>{options.map(option => <button key={option.value} className={value === option.value ? 'active' : ''} onClick={() => onChange(option.value)} aria-pressed={value === option.value}>{option.label}</button>)}</div>
}

export function StateBoundary({ state, children, onRetry, onCreate }: { state: DataState; children: ReactNode; onRetry?: () => void; onCreate?: () => void }) {
  if (state === 'loading') return <div className="state-surface skeleton-state" role="status" aria-live="polite" aria-busy="true" aria-label="正在加载"><span/><span/><span/><span/><span/></div>
  if (state === 'empty') return <div className="state-surface state-empty" role="status" aria-live="polite"><FileQuestion size={28} aria-hidden="true"/><h2>还没有可用内容</h2><p>先创建一个对象，系统会在这里展示版本、证据和后续动作。</p>{onCreate ? <button className="primary-button" onClick={onCreate}>创建第一项</button> : null}</div>
  if (state === 'error') return <div className="state-surface state-error" role="alert"><CircleAlert size={28} aria-hidden="true"/><h2>数据暂时无法读取</h2><p>最近一次同步失败，现有数据没有被覆盖。请重试或查看同步记录。</p>{onRetry ? <button className="secondary-button" onClick={onRetry}><RefreshCw size={15} aria-hidden="true"/>重新加载</button> : null}</div>
  if (state === 'forbidden') return <div className="state-surface state-forbidden" role="alert"><LockKeyhole size={28} aria-hidden="true"/><h2>当前角色没有访问权限</h2><p>需要 Project 管理员授予编辑或审批权限，申请不会改变现有内容。</p></div>
  return <>{children}</>
}
