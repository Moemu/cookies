import { useState } from 'react'
import { CircleCheck } from 'lucide-react'
import { api, type ApiInsightReport } from '../../../data/api'
import { FindingRow } from './FindingRow'
import { HarvestPanel } from './HarvestPanel'
import { SubmitReviewAction } from './SubmitReviewAction'

/**
 * 一份复盘的正文。
 *
 * 草稿态和已提交态是同一套布局，差别只在能不能改——两套布局会让人在提交前后
 * 对不上号，以为自己看的是另一份东西。
 */
export function DraftPanel({ report, projectId, onChanged }: {
  report: ApiInsightReport
  projectId: string
  onChanged: () => void
}) {
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const draft = report.status === 'draft'

  // 保留在完整 digest 里的下标：删减接口按下标定位，传分段后的下标会删错另一条。
  const rows = (report.digest ?? []).map((finding, index) => ({ finding, index }))
  const pinned = rows.filter(item => item.finding.origin === 'pinned')
  const system = rows.filter(item => item.finding.origin !== 'pinned')

  const drop = (index: number, dropped: boolean) => {
    setBusy(true); setError('')
    api.dropReportFinding(projectId, report.id, {
      index, dropped, expected_version: report.version,
    }).then(() => onChanged())
      .catch(() => setError('这一条没改成，页面可能不是最新的，刷新后再试。'))
      .finally(() => setBusy(false))
  }

  const submit = async (executionId: string) => {
    setBusy(true); setError('')
    try {
      await api.submitReview(projectId, report.id, {
        execution_id: executionId, expected_version: report.version,
      })
      onChanged()
    } catch {
      setError('提交没成。可能是这一轮已经有人提交过同一次投放的复盘了，刷新看看。')
    } finally {
      setBusy(false)
    }
  }

  return <div className="review-body">
    <section>
      <span className="section-label">这一轮我记的（{pinned.length}）</span>
      {pinned.length ? <ul className="finding-list">
        {pinned.map(item => <FindingRow key={item.index} finding={item.finding}
          index={item.index} editable={draft && !busy} onDrop={drop}/>)}
      </ul> : <p className="panel-empty">
        这一轮还没记过。去「分析」里看，看到值得留的按「记一笔」。
      </p>}
    </section>

    <section>
      <span className="section-label">系统补的（{system.length}）</span>
      {draft ? <p className="prelaunch-disclosure">
        系统发现在提交那一刻才算——草稿还在改，现在补进来的数字到提交时就不是这个数了。
      </p> : system.length ? <ul className="finding-list">
        {system.map(item => <FindingRow key={item.index} finding={item.finding}
          index={item.index} editable={false} onDrop={drop}/>)}
      </ul> : <p className="panel-empty">这一轮系统没有补出新的发现。</p>}
    </section>

    {error ? <div className="inline-notice" role="status">{error}</div> : null}

    {draft
      ? <SubmitReviewAction window={{ start: report.window_start ?? '', end: report.window_end ?? '' }}
          busy={busy} onSubmit={submit}/>
      : <>
          <p className="review-sealed"><CircleCheck size={15}/>这份复盘已提交，不能再改。</p>
          <HarvestPanel report={report} projectId={projectId} onChanged={onChanged}/>
        </>}
  </div>
}
