import { useEffect, useState } from 'react'
import { BookOpenCheck, CircleAlert, Save } from 'lucide-react'
import { api, type ApiExperience, type ApiInsightReport } from '../../../data/api'

/**
 * 沉淀成经验。
 *
 * 复盘提交完不等于这一轮有产出：报告本身不会进下一轮投前洞察，只有从它沉淀出、
 * 并且有人确认过的经验会。所以这一段只在提交之后出现——后端也只认已确认的复盘。
 */
export function HarvestPanel({ report, projectId, onChanged }: {
  report: ApiInsightReport
  projectId: string
  onChanged: () => void
}) {
  const [harvested, setHarvested] = useState<ApiExperience[]>([])
  const [draft, setDraft] = useState('')
  const [busy, setBusy] = useState(false)
  const [notice, setNotice] = useState('')

  // 换一份复盘就把输入框清空：上一份的结论留在框里，很容易被顺手沉淀到这一份上。
  useEffect(() => {
    setDraft('')
    setNotice('')
    if (!projectId) return
    let cancelled = false
    api.listExperiences(projectId).then(page => {
      if (cancelled) return
      setHarvested((page.items ?? []).filter(item => item.report_id === report.id))
    }).catch(() => { if (!cancelled) setHarvested([]) })
    return () => { cancelled = true }
  }, [projectId, report.id, report.version])

  const already = new Set(harvested.map(experience => experience.conclusion))

  const harvest = async () => {
    const conclusion = draft.trim()
    setBusy(true)
    setNotice('')
    try {
      // 类型和置信留空，后端会落到最保守的一格（假设 / 方向性）。这是故意的：
      // 从一句话直接沉淀出来的结论确实还没有依据，替录入的人标成「事实」是伪造。
      // 适用条件同理留空——空着，经验库才会明写「未填写适用条件」，人才知道还欠什么。
      await api.createExperienceFromReport(projectId, report.id, {
        expected_report_version: report.version,
        conclusion,
      })
      setDraft('')
      setNotice('已沉淀为待确认的经验。它现在是「假设 / 方向性」，去经验库补上适用范围和数据依据后才能当证据用。')
      onChanged()
    } catch (cause) {
      setNotice(cause instanceof Error ? cause.message : '沉淀经验失败，请重试。')
    } finally {
      setBusy(false)
    }
  }

  return <div className="experience-reason">
    <span className="section-label">沉淀成经验</span>

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

    <textarea aria-label="要沉淀的结论" rows={3} value={draft}
      onChange={event => setDraft(event.target.value)}
      placeholder="写一句结论，或从上面的发现里挑一条粘过来"/>
    <div className="prelaunch-actions">
      {/* 同一句结论沉淀两次会变成两条经验，投前洞察里就会看到重复的卡。
          成功后清空输入框，字面重复的也直接拦住。 */}
      <button className="primary-button full"
        disabled={busy || !draft.trim() || already.has(draft.trim())}
        onClick={() => { void harvest() }}>
        <Save size={15}/>{busy ? '正在沉淀…' : already.has(draft.trim()) ? '这条结论已经沉淀过了' : '沉淀为待确认经验'}
      </button>
    </div>
    {notice ? <div className="inline-notice" role="status">{notice}</div> : null}

    <div className="reference-count">
      <BookOpenCheck size={15}/>
      <span><b>{harvested.length} 条经验来自这份复盘</b><small>沉淀出来还要确认，确认后才会进入投前洞察</small></span>
    </div>
  </div>
}
