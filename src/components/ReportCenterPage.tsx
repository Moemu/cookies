import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  ArrowRight, BookOpenCheck, CalendarRange, CircleAlert, CircleCheck, Database, FlaskConical,
  RefreshCw, Save, ShieldCheck, Trash2,
} from 'lucide-react'
import { useProject } from '../context/ProjectContext'
import {
  api,
  type ApiExperience,
  type ApiInsightReport,
  type ApiReportSectionKind,
  type ApiReportStatus,
  type ApiVariantVerdict,
} from '../data/api'
import type { DataState, SystemKey } from '../types'
import { StateBoundary } from './StateBoundary'

/**
 * 报告中心（03-asset-management-prd.md AM-015，导航见 19 §5.2）。
 *
 * AM-015 只要求一件事：按任务汇总素材表现、实验结论、经验和下一轮建议。
 * 一次投放执行对应一份报告，报告确认之后从它沉淀经验——这条链是整个素材洞察
 * 模块的出口，「一次性复盘变成可复用经验」（03 §2 目标④）就发生在这里。
 *
 * 三个视图，各查各的（22 §8.3）：
 *   全部复盘 —— 这个 Project 的所有报告，含已确认的；
 *   待确认   —— status=draft 的队列，是「还需要人做事」的那一批；
 *   已沉淀经验 —— 查的不是报告而是 experiences 按 report_id 的归属，
 *                答的是「这些复盘最后到底留下了什么」。一份确认了却没沉淀出
 *                任何经验的报告，等于这次复盘白做了，这个视图专门让它显出来。
 *
 * 导航原本还有「周期报告 / 自定义报告 / 协作 / 版本与导出」四个视图（03 §5、
 * 19 §5.2 的二级功能表），但 §9 的功能需求表里没有任何一条 FR 支撑它们，后端也
 * 没有对应模型。做成空标签页会让人以为功能坏了，做成假内容更糟，所以先收到三个
 * ——这条记进基线文档 §5 冲突登记。
 */
type ViewTarget = 'all' | 'pending' | 'harvest'

const viewTargets: Record<string, ViewTarget> = {
  全部复盘: 'all',
  任务复盘: 'all',
  待确认: 'pending',
  已沉淀经验: 'harvest',
}

const statusLabels: Record<ApiReportStatus, string> = {
  draft: '待确认',
  confirmed: '已确认',
}

// 四块的顺序和后端一致（report_digest.go 的 ReportSectionOrder）：
// 先看这轮素材跑得怎么样，再看实验说明了什么，然后是能引用的老经验，
// 最后才是下一轮怎么做。倒过来读，建议就成了没有依据的指令。
const sectionOrder: ApiReportSectionKind[] = ['asset_performance', 'experiment', 'experience', 'recommendation']

const sectionLabels: Record<ApiReportSectionKind, string> = {
  asset_performance: '素材表现',
  experiment: '实验结论',
  experience: '相关经验',
  recommendation: '下一轮建议',
}

// 和投后分析里同一套说法：读者先要知道这条结论能不能用，再看它说了什么。
const strengthLabels: Record<ApiVariantVerdict, string> = {
  attributable: '可归因',
  directional: '只能看方向',
  confounded: '归不了因',
  low_sample: '样本不足',
  no_features: '缺内容特征',
}

const strengthTone: Record<ApiVariantVerdict, string> = {
  attributable: 'ok',
  directional: 'warning',
  confounded: 'danger',
  low_sample: 'muted',
  no_features: 'muted',
}

const headings: Record<ViewTarget, { title: string; blurb: string }> = {
  all: {
    title: '这个 Project 的所有复盘',
    blurb: '一次投放执行对应一份复盘报告。报告本身不是结论，是把这次投放里发生的事、量到的数和看出来的规律放在一起，好让人判断哪些值得留下来。',
  },
  pending: {
    title: '还等着人确认的复盘',
    blurb: '报告在确认之前不能沉淀经验。这不是流程繁琐——没人看过就变成「经验」的结论，会在下一轮投前洞察里被当成证据引用。',
  },
  harvest: {
    title: '这些复盘最后留下了什么',
    blurb: '按报告看它沉淀出的经验。一份已确认却一条经验都没沉淀的报告，说明这次复盘的结论没人愿意为它背书——这比没做复盘更值得注意。',
  },
}

const emptyHints: Record<ViewTarget, string> = {
  all: '这个 Project 还没有复盘报告。报告在一次投放执行结束、有了效果证据之后生成，可以先去智能投放看看有没有已完成的执行。',
  pending: '没有待确认的复盘。所有报告都已经有人看过并确认了。',
  harvest: '还没有从复盘沉淀出的经验。可以在左边选一份已确认的报告，把它的结论沉淀成经验卡。',
}

type OpenProject = (id: string, system?: SystemKey, navId?: string, objectId?: string, view?: string) => void

export function ReportCenterPage({ state, activeView, objectId, onOpenProject }: {
  state: DataState
  activeView: string
  objectId?: string
  onOpenProject: OpenProject
}) {
  const { currentProject } = useProject()
  const target = viewTargets[activeView] ?? 'all'
  const [reports, setReports] = useState<ApiInsightReport[]>([])
  const [experiences, setExperiences] = useState<ApiExperience[]>([])
  const [selectedId, setSelectedId] = useState('')
  const [notice, setNotice] = useState('')
  const [busy, setBusy] = useState(false)
  const [listState, setListState] = useState<'loading' | 'ready' | 'error'>('loading')

  const load = useCallback(async () => {
    if (!currentProject.id) return
    setListState('loading')
    try {
      const [reportPage, experiencePage] = await Promise.all([
        api.listReports(currentProject.id),
        api.listExperiences(currentProject.id),
      ])
      setReports(reportPage.items)
      setExperiences(experiencePage.items)
      setListState('ready')
    } catch (cause) {
      setReports([])
      setExperiences([])
      setListState('error')
      setNotice(cause instanceof Error ? cause.message : '复盘报告读取失败。')
    }
  }, [currentProject.id])

  useEffect(() => { void load() }, [load])
  useEffect(() => { setNotice('') }, [target])

  // 一份报告沉淀出的经验按 report_id 归属。这个映射三个视图都要用：
  // 「沉淀了几条」是判断一份复盘有没有落地的唯一依据。
  const harvested = useMemo(() => {
    const map = new Map<string, ApiExperience[]>()
    experiences.forEach(experience => {
      if (!experience.report_id) return
      map.set(experience.report_id, [...(map.get(experience.report_id) ?? []), experience])
    })
    return map
  }, [experiences])

  const visible = useMemo(() => {
    if (target === 'pending') return reports.filter(report => report.status === 'draft')
    if (target === 'harvest') return reports.filter(report => report.status === 'confirmed')
    return reports
  }, [reports, target])

  // 从投后分析定格过来时带着报告 ID：刚定格的那份要直接打开，
  // 否则人会落在列表顶上，还得自己找刚才那一份。
  useEffect(() => {
    const ids = visible.map(report => report.id)
    setSelectedId(current => {
      if (objectId && ids.includes(objectId)) return objectId
      return ids.includes(current) ? current : ids[0] ?? ''
    })
  }, [visible, objectId])

  const selected = visible.find(report => report.id === selectedId)

  const confirm = useCallback(async (report: ApiInsightReport) => {
    setBusy(true)
    setNotice('')
    try {
      await api.confirmReport(currentProject.id, report.id, report.version)
      setNotice('已确认。现在可以从这份复盘沉淀经验了。')
      await load()
    } catch (cause) {
      setNotice(cause instanceof Error ? cause.message : '确认失败，请重试。')
    } finally {
      setBusy(false)
    }
  }, [currentProject.id, load])

  // 人工删减。删掉的条目不会真的消失，只是标记——报告要能说清
  // 「系统给了什么、人拿掉了哪几条」，物理删掉就查不回来了。
  const dropFinding = useCallback(async (report: ApiInsightReport, index: number, dropped: boolean) => {
    setBusy(true)
    setNotice('')
    try {
      await api.dropReportFinding(currentProject.id, report.id, {
        expected_version: report.version, index, dropped,
      })
      await load()
    } catch (cause) {
      setNotice(cause instanceof Error ? cause.message : '改不动这条发现，请重试。')
    } finally {
      setBusy(false)
    }
  }, [currentProject.id, load])

  const harvest = useCallback(async (report: ApiInsightReport, conclusion: string): Promise<boolean> => {
    setBusy(true)
    setNotice('')
    try {
      // 类型和置信留空，后端会落到最保守的一格（假设 / 方向性）。这是故意的：
      // 从一句话直接沉淀出来的结论确实还没有依据，替录入的人标成「事实」是伪造。
      // 补依据要去经验库对它做修订。
      // 适用条件留空。来源报告后端自己记着（经验库那一栏就是从 report_id 渲染的），
      // 往适用条件里塞一句「来自复盘 xxx」等于把这一栏填满了却什么都没说——经验库
      // 会把它当成「已经写了适用条件」，而适用条件问的是「这条结论在什么情况下成立」。
      // 空着，页面就会明写「未填写适用条件」，人才知道还欠什么。
      await api.createExperienceFromReport(currentProject.id, report.id, {
        expected_report_version: report.version,
        conclusion,
      })
      setNotice('已沉淀为待确认的经验。它现在是「假设 / 方向性」，去经验库补上适用范围和数据依据后才能当证据用。')
      await load()
      return true
    } catch (cause) {
      setNotice(cause instanceof Error ? cause.message : '沉淀经验失败，请重试。')
      return false
    } finally {
      setBusy(false)
    }
  }, [currentProject.id, load])

  const pendingCount = reports.filter(report => report.status === 'draft').length
  const barrenCount = reports.filter(report =>
    report.status === 'confirmed' && !(harvested.get(report.id)?.length)).length

  return <StateBoundary state={state} onRetry={() => { void load() }}>
    <div className="prelaunch-workspace">
      <section className="prelaunch-main">
        <div className="core-flow-toolbar">
          <div>
            <span className="section-label">REPORT CENTER</span>
            <h2>{headings[target].title}</h2>
            <p>当前 Project：{currentProject.name}。{headings[target].blurb}</p>
          </div>
          <div className="core-flow-actions">
            <button className="secondary-button" disabled={listState === 'loading'} onClick={() => { void load() }}>
              <RefreshCw size={15}/>刷新
            </button>
          </div>
        </div>

        {/* 已确认却没沉淀出经验的报告，是这一页最该被看见的事：复盘做了，什么也没留下。 */}
        {target === 'harvest' && barrenCount > 0 ? <div className="feature-stack">
          <span>有 {barrenCount} 份已确认的复盘没沉淀出任何经验</span>
          <b>确认只表示「这份报告写的是真的」，不表示有人愿意把它当成下一轮的依据。这几份复盘的结论目前不会出现在投前洞察里。</b>
        </div> : null}

        <div className="prelaunch-filterbar">
          <span>{visible.length} 份报告</span>
          <span>全项目 {reports.length} 份 · 待确认 {pendingCount} 份 · 已沉淀经验 {experiences.filter(item => item.report_id).length} 条</span>
        </div>

        {listState === 'loading' ? <div className="panel-empty">正在读取当前 Project 的复盘报告…</div> : null}
        {listState === 'error' ? <div className="panel-empty">读取失败，请重试。</div> : null}
        {listState === 'ready' && !visible.length ? <div className="panel-empty">{emptyHints[target]}</div> : null}

        {listState === 'ready' && visible.length ? <div className="prelaunch-table" role="list" aria-label={headings[target].title}>
          <div className="prelaunch-row header">
            <span>复盘</span><span>数据来源</span><span>沉淀经验</span><span>状态</span>
          </div>
          {visible.map(report => <button role="listitem" key={report.id}
            className={report.id === selectedId ? 'prelaunch-row active' : 'prelaunch-row'}
            onClick={() => setSelectedId(report.id)}>
            <span>
              <b>{report.summary || '（这份报告还没写摘要）'}</b>
              <small>{report.id} · 执行 {report.execution_id} · {formatTime(report.created_at)}</small>
            </span>
            <span>{report.is_simulated ? '模拟投放' : '真实投放'} · {report.dataset_version || '未标数据集版本'}</span>
            <span>{harvested.get(report.id)?.length ?? 0} 条经验</span>
            <span>
              {report.status === 'confirmed' ? <CircleCheck size={14}/> : <CircleAlert size={14}/>}
              {statusLabels[report.status]}
            </span>
          </button>)}
        </div> : null}
      </section>

      <aside className="prelaunch-detail">
        {selected
          ? <ReportDetail report={selected} harvested={harvested.get(selected.id) ?? []}
            busy={busy} onConfirm={confirm} onHarvest={harvest} onDropFinding={dropFinding}
            onOpenExperience={() => onOpenProject(currentProject.id, 'insight', 'knowledge', '', '候选经验')}/>
          : <div className="panel-empty">左边选一份复盘，查看它的证据、发现和沉淀出的经验。</div>}
        {notice ? <div className="inline-notice" role="status">{notice}</div> : null}
      </aside>
    </div>
  </StateBoundary>
}

function ReportDetail({ report, harvested, busy, onConfirm, onHarvest, onDropFinding, onOpenExperience }: {
  report: ApiInsightReport
  harvested: ApiExperience[]
  busy: boolean
  onConfirm: (report: ApiInsightReport) => Promise<void>
  onHarvest: (report: ApiInsightReport, conclusion: string) => Promise<boolean>
  onDropFinding: (report: ApiInsightReport, index: number, dropped: boolean) => Promise<void>
  onOpenExperience: () => void
}) {
  const [draft, setDraft] = useState('')
  const [showDropped, setShowDropped] = useState(false)

  // 换一份报告就把输入框清空：上一份的结论留在框里，很容易被顺手沉淀到这一份上。
  useEffect(() => { setDraft(''); setShowDropped(false) }, [report.id])

  const alreadyHarvested = new Set(harvested.map(experience => experience.conclusion))
  // 保留原始下标：删减接口按下标定位，过滤之后再传下标会删错另一条。
  const digest = (report.digest ?? []).map((finding, index) => ({ finding, index }))
  const kept = digest.filter(item => !item.finding.dropped)
  const dropped = digest.filter(item => item.finding.dropped)

  return <>
    <span className="section-label">{statusLabels[report.status]} · 第 {report.version} 版</span>
    <h3>{report.summary || '（这份报告还没写摘要）'}</h3>
    <p>{report.id} · 执行 {report.execution_id} · 更新于 {formatTime(report.updated_at)}</p>

    {/* 模拟投放要说在最前面：它的结论不能和真实投放的结论平起平坐。 */}
    {report.is_simulated ? <div className="prelaunch-boundary">
      <CircleAlert size={16}/>
      <span>
        <small>这是模拟投放的复盘</small>
        数据来自演示数据集 {report.dataset_version || '（未标版本）'}，不是任何一次真实投放的结果。
        从它沉淀出的经验同样只是演示，不能作为真实决策依据。
      </span>
    </div> : null}

    <div className="prelaunch-fact"><Database size={17}/><span><small>证据</small><b>
      {report.evidence_summary || '这份报告没有附证据摘要。'}
    </b></span></div>
    <div className="prelaunch-fact"><FlaskConical size={17}/><span><small>数据来源</small><b>
      投放方式 {report.delivery_mode || '未记录'} · 指标快照 {report.metric_snapshot_id || '无'} · 创意包 {report.creative_package_id || '无'}
    </b></span></div>
    <div className="prelaunch-fact"><ShieldCheck size={17}/><span><small>确认状态</small><b>
      {report.status === 'confirmed'
        ? `${report.confirmed_by || '某人'} 于 ${report.confirmed_at ? formatTime(report.confirmed_at) : '（时间未记录）'} 确认`
        : '还没有人确认。确认之前不能从它沉淀经验。'}
    </b></span></div>

    {/* 四块发现。数据窗口写在最前面：报告说的是「在这个窗口的数据上、
        做了这个判断」，没有窗口，下面每个数字都无法复核。 */}
    {digest.length ? <>
      {report.window_start && report.window_end ? <div className="prelaunch-fact">
        <CalendarRange size={17}/><span><small>定格的数据窗口</small><b>
          {formatDay(report.window_start)} ~ {formatDay(report.window_end)}
        </b></span>
      </div> : null}

      {sectionOrder.map(kind => {
        const items = kept.filter(item => item.finding.kind === kind)
        if (!items.length) return null
        return <div className="report-section" key={kind}>
          <span className="section-label">{sectionLabels[kind]}</span>
          {items.map(item => <div className="report-finding" key={item.index}>
            <span>
              {item.finding.text}
              {alreadyHarvested.has(item.finding.text) ? <em>已沉淀</em> : null}
              {item.finding.strength ? <em className={strengthTone[item.finding.strength]}>
                {strengthLabels[item.finding.strength]}
              </em> : null}
            </span>
            {/* 只有草稿能删。确认过的报告改不动——确认的意思就是有人为这一份背书了。 */}
            {report.status === 'draft' ? <button className="text-button" disabled={busy}
              aria-label={`删掉这条发现：${item.finding.text}`}
              onClick={() => { void onDropFinding(report, item.index, true) }}>
              <Trash2 size={13}/>
            </button> : null}
          </div>)}
        </div>
      })}

      {dropped.length ? <div className="report-dropped">
        <button className="text-button" onClick={() => setShowDropped(current => !current)}>
          人工删掉的 {dropped.length} 条{showDropped ? '（收起）' : '（展开）'}
        </button>
        {showDropped ? dropped.map(item => <div className="report-finding muted" key={item.index}>
          <span>{item.finding.text}</span>
          {report.status === 'draft' ? <button className="text-button" disabled={busy}
            onClick={() => { void onDropFinding(report, item.index, false) }}>放回去</button> : null}
        </div>) : null}
      </div> : null}

      {!kept.length ? <div className="prelaunch-boundary">
        <CircleAlert size={16}/><span><small>这份报告的发现被删光了</small>
          一条不留的报告没法承载任何结论。可以从上面「人工删掉的」里放回来几条。
        </span></div> : null}
    </> : report.findings.length ? <div className="feature-stack">
      {/* 老报告只有一串没有分块的发现。原样显示，不硬塞进四块里——
          它们本来就没有 kind，归到哪一块都是编的。 */}
      <span>这次复盘看出来的 {report.findings.length} 件事（这是定格四块发现之前生成的老报告）</span>
      {report.findings.map(finding => <b key={finding}>
        {finding}{alreadyHarvested.has(finding) ? '（已沉淀）' : ''}
      </b>)}
    </div> : null}

    {harvested.length ? <div className="feature-stack">
      <span>从这份复盘沉淀出的 {harvested.length} 条经验</span>
      {harvested.map(experience => <b key={experience.id}>{experience.conclusion}</b>)}
    </div> : <div className="prelaunch-boundary">
      <CircleAlert size={16}/>
      <span>
        <small>这份复盘还没留下任何经验</small>
        报告本身不会进入下一轮投前洞察，只有从它沉淀出并确认的经验会。
      </span>
    </div>}

    {report.status === 'draft' ? <div className="prelaunch-actions">
      <button className="primary-button full" disabled={busy} onClick={() => void onConfirm(report)}>
        <CircleCheck size={15}/>{busy ? '正在确认…' : '确认这份复盘'}
      </button>
    </div> : <div className="experience-reason">
      <span className="section-label">沉淀成经验</span>
      <textarea aria-label="要沉淀的结论" rows={3} value={draft}
        onChange={event => setDraft(event.target.value)}
        placeholder="写一句结论，或从上面的发现里挑一条粘过来"/>
      <div className="prelaunch-actions">
        {/* 同一句结论沉淀两次会变成两条经验，投前洞察里就会看到重复的卡。
            成功后清空输入框，字面重复的也直接拦住。 */}
        <button className="primary-button full"
          disabled={busy || !draft.trim() || alreadyHarvested.has(draft.trim())}
          onClick={() => { void onHarvest(report, draft.trim()).then(ok => { if (ok) setDraft('') }) }}>
          <Save size={15}/>{busy ? '正在沉淀…' : alreadyHarvested.has(draft.trim()) ? '这条结论已经沉淀过了' : '沉淀为待确认经验'}
        </button>
      </div>
    </div>}

    <button className="text-button prelaunch-deeplink" onClick={onOpenExperience}>
      去经验库补齐依据并确认<ArrowRight size={14}/>
    </button>
    <div className="reference-count">
      <BookOpenCheck size={15}/>
      <span><b>{harvested.length} 条经验来自这份复盘</b><small>沉淀出来还要确认，确认后才会进入投前洞察</small></span>
    </div>
  </>
}

function formatTime(value: string): string {
  const date = new Date(value)
  return Number.isNaN(date.valueOf()) ? value : date.toLocaleString('zh-CN', { hour12: false })
}

// 窗口存的就是「2026-07-01」这样的日子，不是时间戳。走一遍 Date 会按本地时区
// 换算，可能差一天——而报告上写的是定格时人看到的那个区间。
function formatDay(value: string): string {
  const parts = value.split('-')
  return parts.length === 3 ? `${parts[0]}/${Number(parts[1])}/${Number(parts[2])}` : value
}
