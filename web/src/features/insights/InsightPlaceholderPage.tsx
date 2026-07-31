import { Link, useParams } from 'react-router-dom'
import type { InsightEntry } from '../../app/routes'
import { insightPath } from '../../app/routes'
import type { Project } from '../platform/types'
import { InsightPageFrame, InsightToolbar } from './InsightPageFrame'
import '../outcomes/outcome-workspace.css'

/**
 * 导航按规范列全 11 个入口、每个入口列全它的二级视图，但真正做完的只是其中一部分。
 * 没做完的视图进来看到的是这一页：写清楚将来会有什么，不放任何编造的数据。
 */
export function InsightPlaceholderPage({ entry, project, view }: {
  entry: InsightEntry
  project?: Project
  view: string
}) {
  const { projectId = '' } = useParams()
  const current = entry.views.find((item) => item.key === view)
  return <InsightPageFrame entry={entry} project={project} view={view}>
    <div className="insight-panel">
      <InsightToolbar
        description={`「${entry.label} · ${current?.label || view}」还没有实现。导航完整列出是为了让你看清这个模块最终包含什么；这里不会显示任何占位或编造的数据。`}
        label="尚未开放"
        project={project}
        title={`${current?.label || view}还没有做`}
      />
      <div className="insight-panel__body">
        <section className="outcome-cards">
          <article className="outcome-card">
            <div className="outcome-card__top"><span className="status-chip">规划中</span></div>
            <h2>{entry.label}这一组最终包含</h2>
            <p>{entry.purpose}</p>
            <ul>{entry.views.map((item) => <li key={item.key}>
              {item.label}{item.built ? '（已实现）' : ''}
            </li>)}</ul>
          </article>
          <article className="outcome-card">
            <div className="outcome-card__top"><span className="status-chip status-chip--active">现在可用</span></div>
            <h2>当前能用的入口</h2>
            <p>投前洞察、投后分析、经验库、报告中心已经跑通，可以先从这几处开始。</p>
            <div className="report-card__actions">
              <Link className="button button--secondary button--compact" to={insightPath(projectId, 'prelaunch')}>投前洞察</Link>
              <Link className="button button--secondary button--compact" to={insightPath(projectId, 'experiences')}>经验库</Link>
              <Link className="button button--secondary button--compact" to={insightPath(projectId, 'reports')}>报告中心</Link>
            </div>
          </article>
        </section>
      </div>
    </div>
  </InsightPageFrame>
}
