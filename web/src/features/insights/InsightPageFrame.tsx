import type { ReactNode } from 'react'
import { Link, NavLink, useParams } from 'react-router-dom'
import type { InsightEntry } from '../../app/routes'
import { insightViewPath, projectHomePath } from '../../app/routes'
import type { Project } from '../platform/types'
import './insight-workspace.css'

/**
 * 素材洞察所有页面共用的外壳：面包屑、标题、二级页签。
 * 已实现和「尚未开放」的视图都挂在同一套页签上，
 * 这样从任意一个视图都能看到这一组最终包含哪些内容。
 */
export function InsightPageFrame({ children, counts, entry, project, view }: {
  children: ReactNode
  /** 只有经验库这类有明确条数的视图才传，其余不显示数字。 */
  counts?: Record<string, number>
  entry: InsightEntry
  project?: Project
  view: string
}) {
  const { projectId = '' } = useParams()
  return <section className="outcome-workspace insight-workspace">
    <header className="insight-page-head">
      <div>
        <nav className="breadcrumb" aria-label="面包屑">
          <Link to="/projects">项目</Link><span>/</span>
          <Link to={projectHomePath(projectId)}>{project?.name || projectId}</Link><span>/</span>
          <span>素材洞察</span>
        </nav>
        <h1>{entry.label}</h1>
        <p>{entry.purpose}</p>
      </div>
      <span className="insight-page-head__note">跟着当前项目走，不需要在这里重新建一遍任务。</span>
    </header>
    <nav aria-label={`${entry.label}视图`} className="insight-tabs">
      {entry.views.map((item) => <NavLink
        className={[
          item.key === view ? 'insight-tabs__item--active' : '',
          item.built ? '' : 'insight-tabs__item--planned',
        ].filter(Boolean).join(' ')}
        key={item.key}
        title={item.built ? undefined : '尚未开放'}
        to={insightViewPath(projectId, entry.key, item.key)}
      >
        {item.label}
        {counts?.[item.key] === undefined ? null : <small>{counts[item.key]}</small>}
      </NavLink>)}
    </nav>
    {children}
  </section>
}

/** 面板顶部的工具条：左侧小标题 + 标题 + 说明，右侧是当前项目。 */
export function InsightToolbar({ description, label, project, title }: {
  description: string
  label: string
  project?: Project
  title: string
}) {
  return <div className="insight-toolbar">
    <div>
      <span className="insight-section-label">{label}</span>
      <h2>{title}</h2>
      <p>{description}</p>
    </div>
    {project ? <div className="insight-toolbar__context">
      <small>当前项目</small><b>{project.name}</b>
    </div> : null}
  </div>
}
