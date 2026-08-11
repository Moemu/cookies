import { useCallback, useEffect, useMemo, useState } from 'react'
import { BookOpenCheck, CalendarRange, CircleAlert, CircleCheck, Database, RefreshCw } from 'lucide-react'
import { useProject } from '../../../context/ProjectContext'
import { api, type ApiExperience, type ApiInsightReport } from '../../../data/api'
import type { DataState } from '../../../types'
import { StateBoundary } from '../../StateBoundary'
import { DraftPanel } from './DraftPanel'

/**
 * 「复盘」入口。一轮投放收尾时人真正要做事的地方。
 *
 * 一屏干一件事：左边挑一份复盘，右边看「这一轮我记的」和「系统补的」，
 * 逐条决定留不留，然后提交。提交之后这份复盘冻住，才谈得上沉淀经验。
 *
 * 三个视图分的是「现在要做什么」，不是「按什么排序」：
 *   本轮     —— 只列还没提交的草稿，这是日常最常进的一屏；
 *   全部复盘 —— 含已提交的，用来回头查一轮投放当时是怎么判的；
 *   已沉淀经验 —— 查的不是报告而是经验按 report_id 的归属，答的是
 *                「这些复盘最后到底留下了什么」。一份提交了却没沉淀出
 *                任何经验的复盘，等于这次收尾白做了，这个视图专门让它显出来。
 */
export type ReviewView = 'current' | 'all' | 'harvest'

const headings: Record<ReviewView, { label: string; title: string; lead: string }> = {
  current: {
    label: 'REVIEW',
    title: '这一轮我记了什么，该收尾了',
    lead: '还没提交的复盘草稿。草稿是你在分析页按「记一笔」时自动建的，一个窗口一份。提交时系统才会把漏掉的补齐。',
  },
  all: {
    label: 'REVIEW · ALL',
    title: '这个 Project 的所有复盘',
    lead: '含已提交的。提交过的不能再改——它会被下一轮引用，改一条等于让引用它的人手上那份变成假的。',
  },
  harvest: {
    label: 'REVIEW · HARVEST',
    title: '这些复盘最后留下了什么',
    lead: '按复盘看它沉淀出的经验。一份已确认却一条经验都没沉淀的报告，说明这次复盘的结论没人愿意为它背书——这比没做复盘更值得注意。',
  },
}

const emptyHints: Record<ReviewView, string> = {
  current: '这一轮还没有复盘草稿。去「分析」里看，看到值得留的按「记一笔」，草稿会自动建出来。',
  all: '这个 Project 还没有复盘。复盘从「记一笔」开始，不是等系统生成的。',
  harvest: '还没有提交过的复盘。提交之后才能从它沉淀经验。',
}

export function ReviewPage({ state, view, objectId }: {
  state: DataState
  view: ReviewView
  /** 从别处跳进来时带的复盘 ID，比如刚记完一笔想直接看草稿。 */
  objectId?: string
}) {
  const { currentProject } = useProject()
  const [reports, setReports] = useState<ApiInsightReport[]>([])
  const [experiences, setExperiences] = useState<ApiExperience[]>([])
  const [selectedId, setSelectedId] = useState('')
  const [notice, setNotice] = useState('')
  const [listState, setListState] = useState<'loading' | 'ready' | 'error'>('loading')

  const load = useCallback(async () => {
    if (!currentProject.id) return
    setListState('loading')
    try {
      const [reportPage, experiencePage] = await Promise.all([
        api.listReports(currentProject.id),
        api.listExperiences(currentProject.id),
      ])
      setReports(reportPage.items ?? [])
      setExperiences(experiencePage.items ?? [])
      setListState('ready')
    } catch (cause) {
      setReports([])
      setExperiences([])
      setListState('error')
      setNotice(cause instanceof Error ? cause.message : '复盘读取失败。')
    }
  }, [currentProject.id])

  useEffect(() => { void load() }, [load])
  useEffect(() => { setNotice('') }, [view])

  // 一份复盘沉淀出的经验按 report_id 归属。三个视图都要用它：
  // 「沉淀了几条」是判断一次收尾有没有落地的唯一依据。
  const harvested = useMemo(() => {
    const map = new Map<string, ApiExperience[]>()
    experiences.forEach(experience => {
      if (!experience.report_id) return
      map.set(experience.report_id, [...(map.get(experience.report_id) ?? []), experience])
    })
    return map
  }, [experiences])

  const visible = useMemo(() => {
    if (view === 'current') return reports.filter(report => report.status === 'draft')
    if (view === 'harvest') return reports.filter(report => report.status === 'confirmed')
    return reports
  }, [reports, view])

  // 默认选最近那份。列表按后端给的顺序（新的在前），所以取第一条。
  useEffect(() => {
    const ids = visible.map(report => report.id)
    setSelectedId(current => {
      if (objectId && ids.includes(objectId)) return objectId
      return ids.includes(current) ? current : ids[0] ?? ''
    })
  }, [visible, objectId])

  const selected = visible.find(report => report.id === selectedId)

  const draftCount = reports.filter(report => report.status === 'draft').length
  const barrenCount = reports.filter(report =>
    report.status === 'confirmed' && !(harvested.get(report.id)?.length)).length
  const heading = headings[view]

  return <StateBoundary state={state} onRetry={() => { void load() }}>
    <div className="prelaunch-workspace">
      <section className="prelaunch-main">
        <div className="core-flow-toolbar">
          <div>
            <span className="section-label">{heading.label}</span>
            <h2>{heading.title}</h2>
            <p>当前 Project：{currentProject.name}。{heading.lead}</p>
          </div>
          <div className="core-flow-actions">
            <button className="secondary-button" disabled={listState === 'loading'} onClick={() => { void load() }}>
              <RefreshCw size={15}/>刷新
            </button>
          </div>
        </div>

        {/* 已提交却没沉淀出经验的复盘，是这一页最该被看见的事：做了，什么也没留下。 */}
        {view === 'harvest' && barrenCount > 0 ? <div className="feature-stack">
          <span>有 {barrenCount} 份已提交的复盘没沉淀出任何经验</span>
          <b>提交只表示「这一轮的账算清了」，不表示有人愿意把它当成下一轮的依据。这几份复盘的结论目前不会出现在投前洞察里。</b>
        </div> : null}

        <div className="prelaunch-filterbar">
          <span>{visible.length} 份复盘</span>
          <span>全项目 {reports.length} 份 · 未提交 {draftCount} 份 · 已沉淀经验 {experiences.filter(item => item.report_id).length} 条</span>
        </div>

        {listState === 'loading' ? <div className="panel-empty">正在读取当前 Project 的复盘…</div> : null}
        {listState === 'error' ? <div className="panel-empty">读取失败，请重试。</div> : null}
        {listState === 'ready' && !visible.length ? <div className="panel-empty">{emptyHints[view]}</div> : null}

        {listState === 'ready' && visible.length ? <div className="prelaunch-table" role="list" aria-label={heading.title}>
          <div className="prelaunch-row header">
            <span>复盘</span><span>数据窗口</span><span>沉淀经验</span><span>状态</span>
          </div>
          {visible.map(report => <button role="listitem" key={report.id}
            className={report.id === selectedId ? 'prelaunch-row active' : 'prelaunch-row'}
            onClick={() => setSelectedId(report.id)}>
            <span>
              <b>{report.summary || '（这一轮还没写摘要）'}</b>
              <small>记了 {countPinned(report)} 笔 · {formatTime(report.created_at)}</small>
            </span>
            <span>{report.window_start && report.window_end
              ? `${formatDay(report.window_start)} ~ ${formatDay(report.window_end)}`
              : '未定格窗口'}</span>
            <span>{harvested.get(report.id)?.length ?? 0} 条经验</span>
            <span>
              {report.status === 'confirmed' ? <CircleCheck size={14}/> : <CircleAlert size={14}/>}
              {report.status === 'confirmed' ? '已提交' : '未提交'}
            </span>
          </button>)}
        </div> : null}
      </section>

      <aside className="prelaunch-detail">
        {selected ? <>
          <span className="section-label">
            {selected.status === 'confirmed' ? '已提交' : '未提交'} · 第 {selected.version} 版
          </span>
          <h3>{selected.summary || '（这一轮还没写摘要）'}</h3>

          {/* 模拟投放要说在最前面：它的结论不能和真实投放的结论平起平坐。 */}
          {selected.is_simulated ? <div className="prelaunch-boundary">
            <CircleAlert size={16}/>
            <span>
              <small>这是模拟投放的复盘</small>
              数据来自演示数据集 {selected.dataset_version || '（未标版本）'}，不是任何一次真实投放的结果。
              从它沉淀出的经验同样只是演示，不能作为真实决策依据。
            </span>
          </div> : null}

          {selected.window_start && selected.window_end ? <div className="prelaunch-fact">
            <CalendarRange size={17}/><span><small>这一轮的数据窗口</small><b>
              {formatDay(selected.window_start)} ~ {formatDay(selected.window_end)}
            </b></span>
          </div> : null}

          {selected.status === 'confirmed' ? <div className="prelaunch-fact">
            <Database size={17}/><span><small>这一轮算哪次投放</small><b>
              执行 {selected.execution_id || '未记录'} ·
              {selected.confirmed_by || '某人'} 于 {selected.confirmed_at ? formatTime(selected.confirmed_at) : '（时间未记录）'} 提交
            </b></span>
          </div> : null}

          <DraftPanel report={selected} projectId={currentProject.id} onChanged={() => { void load() }}/>

          <div className="reference-count">
            <BookOpenCheck size={15}/>
            <span><b>{harvested.get(selected.id)?.length ?? 0} 条经验来自这份复盘</b>
              <small>沉淀出来还要确认，确认后才会进入投前洞察</small></span>
          </div>
        </> : <div className="panel-empty">左边选一份复盘，看这一轮记了什么、系统补了什么。</div>}
        {notice ? <div className="inline-notice" role="status">{notice}</div> : null}
      </aside>
    </div>
  </StateBoundary>
}

function countPinned(report: ApiInsightReport): number {
  return (report.digest ?? []).filter(finding => finding.origin === 'pinned').length
}

function formatTime(value: string): string {
  const date = new Date(value)
  return Number.isNaN(date.valueOf()) ? value : date.toLocaleString('zh-CN', { hour12: false })
}

// 窗口存的就是「2026-07-01」这样的日子，不是时间戳。走一遍 Date 会按本地时区
// 换算，可能差一天——而复盘上写的是记这一笔时人看到的那个区间。
function formatDay(value: string): string {
  const parts = value.split('-')
  return parts.length === 3 ? `${parts[0]}/${Number(parts[1])}/${Number(parts[2])}` : value
}
