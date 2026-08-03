import { useEffect, useState } from 'react'
import { Camera, CircleAlert } from 'lucide-react'
import { api, type ApiDeliveryExecutionResult } from '../data/api'
import { useProject } from '../context/ProjectContext'

/**
 * 「定格这一屏」。报告不是重新算一遍，是把此刻这一屏冻起来：同一个窗口、
 * 同一批结论。所以这里只额外问一件事——这份报告算哪次投放。
 *
 * 投放执行只能从清单里挑，不给手打的口子：打错一个 ID，报告就挂在了另一次
 * 投放上，而两边都不会报错，等到复盘会上才发现对不上。
 *
 * 单独成文件是因为投后分析有两种页面：指标总览是一套布局，另外五个列表视图是另一套。
 * 两边都要能定格——人看完总览就想留档是最常见的路径。放在其中一边再互相 import
 * 会绕出循环依赖。
 */
export function FreezeReportAction({ window, busy, onFreeze }: {
  /** 人现在看的这个窗口，原样传进来。这里不自己算窗口——一算就可能和屏幕上写的差一天。 */
  window: { start: string; end: string }
  busy: boolean
  onFreeze: (executionId: string) => Promise<void>
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
        <Camera size={15}/>定格这一屏为复盘报告
      </button>
    </div>
  }

  return <div className="experience-reason">
    <span className="section-label">定格成复盘报告</span>
    <p className="prelaunch-disclosure">
      会把 {windowText} 这个窗口下算出来的发现整份冻起来：素材表现、实验结论、相关经验、下一轮建议。
      之后再换窗口不影响已经定格的报告。发现是自动带进去的，进了报告还能删，但加不了新的
      ——每一条都得能回溯到某次对比、某个实验或某条经验。
    </p>

    {pickState === 'loading' ? <div className="panel-empty">正在读取投放执行…</div>
      : pickState === 'error' ? <div className="panel-empty">投放执行读取失败，请重试。</div>
      : !executions.length ? <div className="prelaunch-boundary">
          <CircleAlert size={16}/><span><small>这个 Project 还没有投放执行</small>
            报告要挂在一次投放上，否则「这次复盘的是哪一轮」谁也答不上来。先去智能投放跑一次执行。
          </span></div>
      : <label className="freeze-pick">
          <small>这份报告算哪次投放？</small>
          <select aria-label="投放执行" value={chosen} onChange={event => setChosen(event.target.value)}>
            {executions.map(item => <option key={item.execution.id} value={item.execution.id}>
              {formatDate(item.execution.completed_at || item.execution.started_at)} · {item.evidence.summary || item.execution.id}
            </option>)}
          </select>
        </label>}

    <div className="prelaunch-actions">
      <button className="primary-button full" disabled={busy || !chosen}
        onClick={() => { void onFreeze(chosen) }}>
        <Camera size={15}/>{busy ? '正在定格…' : '定格并去报告中心'}
      </button>
      <button className="text-button" onClick={() => setOpen(false)}>取消</button>
    </div>
  </div>
}

/** 后端收的是 2006-01-02，这里把窗口的 ISO 时间戳切回同一个口径。 */
export function isoDay(value: string): string {
  return value.slice(0, 10)
}

function formatDate(value: string): string {
  const date = new Date(value)
  return Number.isNaN(date.valueOf()) ? value : date.toLocaleDateString('zh-CN')
}
