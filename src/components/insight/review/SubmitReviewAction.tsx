import { useEffect, useState } from 'react'
import { Camera, CircleAlert } from 'lucide-react'
import { api, type ApiDeliveryExecutionResult } from '../../../data/api'
import { useProject } from '../../../context/ProjectContext'

/**
 * 「提交这一轮复盘」。提交不是重新算一遍，是把这一轮冻起来：人记的那几笔，
 * 加上系统这一刻补齐的发现。所以这里只额外问一件事——这一轮算哪次投放。
 *
 * 投放执行只能从清单里挑，不给手打的口子：打错一个 ID，复盘就挂在了另一次
 * 投放上，而两边都不会报错，等到复盘会上才发现对不上。这里是全流程唯一
 * 回答「这份复盘算哪次投放」的地方——草稿是记一笔时自动建的，那会儿还没到这个问题。
 */
export function SubmitReviewAction({ window, busy, onSubmit }: {
  /** 这份复盘定格的窗口，原样传进来。这里不自己算窗口——一算就可能和屏幕上写的差一天。 */
  window: { start: string; end: string }
  busy: boolean
  onSubmit: (executionId: string) => Promise<void>
}) {
  const { currentProject } = useProject()
  const [open, setOpen] = useState(false)
  const [executions, setExecutions] = useState<ApiDeliveryExecutionResult[]>([])
  const [pickState, setPickState] = useState<'loading' | 'ready' | 'error'>('loading')
  const [chosen, setChosen] = useState('')

  useEffect(() => {
    if (!open || !currentProject.id) return
    let cancelled = false
    setPickState('loading')
    api.listDeliveryExecutions(currentProject.id).then(page => {
      if (cancelled) return
      setExecutions(page.items ?? [])
      setChosen(page.items?.[0]?.execution.id ?? '')
      setPickState('ready')
    }).catch(() => {
      if (cancelled) return
      setExecutions([])
      setPickState('error')
    })
    return () => { cancelled = true }
  }, [open, currentProject.id])

  const windowText = `${formatDate(window.start)} ~ ${formatDate(window.end)}`

  if (!open) {
    return <div className="prelaunch-actions">
      <button className="secondary-button full" onClick={() => setOpen(true)}>
        <Camera size={15}/>提交这一轮复盘
      </button>
    </div>
  }

  return <div className="experience-reason">
    <span className="section-label">提交这一轮复盘（{windowText}）</span>
    <p className="prelaunch-disclosure">
      提交会把这一轮的复盘冻起来：你记的那几笔，加上系统这一刻补齐的素材表现、
      实验结论、相关经验和下一轮建议。系统补的只算这一次，之后再换窗口不影响它。
      提交之后这份复盘不能再改——它会被下一轮引用，改一条等于让引用它的人手上那份变成假的。
    </p>

    {pickState === 'loading' ? <div className="panel-empty">正在读取投放执行…</div>
      : pickState === 'error' ? <div className="panel-empty">投放执行读取失败，请重试。</div>
      : !executions.length ? <div className="prelaunch-boundary">
          <CircleAlert size={16}/><span><small>这个 Project 还没有投放执行</small>
            复盘要挂在一次投放上，否则「这次复盘的是哪一轮」谁也答不上来。先去智能投放跑一次执行。
          </span></div>
      : <label className="freeze-pick">
          <small>这份复盘算哪次投放？</small>
          <select aria-label="投放执行" value={chosen} onChange={event => setChosen(event.target.value)}>
            {executions.map(item => <option key={item.execution.id} value={item.execution.id}>
              {formatDate(item.execution.completed_at || item.execution.started_at)} · {item.evidence.summary || item.execution.id}
            </option>)}
          </select>
        </label>}

    <div className="prelaunch-actions">
      <button className="primary-button full" disabled={busy || !chosen}
        onClick={() => { void onSubmit(chosen) }}>
        <Camera size={15}/>{busy ? '正在提交…' : '提交'}
      </button>
      <button className="text-button" onClick={() => setOpen(false)}>取消</button>
    </div>
  </div>
}

function formatDate(value: string): string {
  const date = new Date(value)
  return Number.isNaN(date.valueOf()) ? value : date.toLocaleDateString('zh-CN')
}
