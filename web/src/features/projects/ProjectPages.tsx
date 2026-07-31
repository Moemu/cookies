import { useEffect, useMemo, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { creativeTaskPath, creativeTasksPath, deliveryPath, insightPath, projectHomePath, projectManagePath, strategyPath } from '../../app/routes'
import { listProjectAssets } from '../assets/api'
import { listCreativeIntakes, listCreativePackages, listCreativeTasks, listCreativeVersions } from '../creative/api'
import type { CreativeIntake, CreativePackage, CreativeTask, CreativeVersion } from '../creative/types'
import { listDeliveryExecutions, listDeliveryPlans } from '../delivery/api'
import type { DeliveryExecutionResult, DeliveryPlan } from '../delivery/types'
import { listExperiences, listInsightReports } from '../insights/api'
import type { Experience, InsightReport } from '../insights/types'
import type { Project } from '../platform/types'
import { listStrategyPackages, listWorkspaces } from '../strategy/api'
import type { PackageVersion, Workspace } from '../strategy/types'
import './projects.css'

type ProjectPageProps = {
  project?: Project
  projects?: Project[]
}

type ProjectSnapshot = {
  workspaces: Workspace[]
  packages: PackageVersion[]
  intakes: CreativeIntake[]
  tasks: CreativeTask[]
  versions: CreativeVersion[]
  creativePackages: CreativePackage[]
  deliveryPlans: DeliveryPlan[]
  executions: DeliveryExecutionResult[]
  reports: InsightReport[]
  experiences: Experience[]
  assetCount: number
  generatedAssetCount: number
}

const emptySnapshot: ProjectSnapshot = {
  workspaces: [],
  packages: [],
  intakes: [],
  tasks: [],
  versions: [],
  creativePackages: [],
  deliveryPlans: [],
  executions: [],
  reports: [],
  experiences: [],
  assetCount: 0,
  generatedAssetCount: 0,
}

function projectLabel(project?: Project) {
  return project?.name || '未选择项目'
}

export function ProjectsPage({ projects = [] }: ProjectPageProps) {
  return <section className="project-index">
    <header className="project-page-header">
      <div>
        <span className="page-eyebrow">PROJECTS</span>
        <h1>项目中心</h1>
        <p>项目是策略、创意、素材、投放与洞察共享的业务上下文。</p>
      </div>
      <span className="project-count">{projects.length} 个项目</span>
    </header>
    {projects.length === 0 ? <div className="project-empty">
      <h2>还没有项目</h2>
      <p>使用顶部项目菜单创建第一个项目，系统会把它作为后续业务对象的归属容器。</p>
    </div> : <div className="project-table" role="list">
      {projects.map((project) => <article className="project-row" key={project.id} role="listitem">
        <div className="project-row__identity">
          <span className="project-row__mark">{project.name.slice(0, 1).toUpperCase()}</span>
          <div><strong>{project.name}</strong><small>{project.id}</small></div>
        </div>
        <span className={`status-chip status-chip--${project.status}`}>{project.status === 'active' ? '进行中' : project.status === 'draft' ? '草稿' : '已归档'}</span>
        <div className="project-row__facts">
          <span>Context v{project.project_context_version}</span>
          <span>{project.primary_brand_id ? '已绑定品牌' : '未绑定品牌'}</span>
        </div>
        <div className="project-row__actions">
          <Link className="text-link" to={projectManagePath(project.id)}>管理</Link>
          <Link className="button button--primary button--compact" to={projectHomePath(project.id)}>进入项目</Link>
        </div>
      </article>)}
    </div>}
  </section>
}

export function ProjectHomePage({ project }: ProjectPageProps) {
  const { projectId = '' } = useParams()
  const [snapshot, setSnapshot] = useState<ProjectSnapshot>(emptySnapshot)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    if (!projectId) return
    const controller = new AbortController()
    Promise.allSettled([
      listWorkspaces(projectId, controller.signal),
      listStrategyPackages(projectId, controller.signal),
      listCreativeIntakes(projectId, controller.signal),
      listCreativeTasks(projectId, controller.signal),
      listProjectAssets(projectId, controller.signal),
      listCreativeVersions(projectId, '', controller.signal),
      listCreativePackages(projectId, controller.signal),
      listDeliveryPlans(projectId, controller.signal),
      listDeliveryExecutions(projectId, controller.signal),
      listInsightReports(projectId, controller.signal),
      listExperiences(projectId, undefined, controller.signal),
    ]).then(([workspaces, packages, intakes, tasks, assets, versions, creativePackages, deliveryPlans, executions, reports, experiences]) => {
      if (controller.signal.aborted) return
      const failedCount = [workspaces, packages, intakes, tasks, assets, versions, creativePackages, deliveryPlans, executions, reports, experiences].filter((result) => result.status === 'rejected').length
      setSnapshot({
        workspaces: workspaces.status === 'fulfilled' ? workspaces.value.items : [],
        packages: packages.status === 'fulfilled' ? packages.value.items : [],
        intakes: intakes.status === 'fulfilled' ? intakes.value.items : [],
        tasks: tasks.status === 'fulfilled' ? tasks.value.items : [],
        versions: versions.status === 'fulfilled' ? versions.value.items : [],
        creativePackages: creativePackages.status === 'fulfilled' ? creativePackages.value.items : [],
        deliveryPlans: deliveryPlans.status === 'fulfilled' ? deliveryPlans.value.items : [],
        executions: executions.status === 'fulfilled' ? executions.value.items : [],
        reports: reports.status === 'fulfilled' ? reports.value.items : [],
        experiences: experiences.status === 'fulfilled' ? experiences.value.items : [],
        assetCount: assets.status === 'fulfilled' ? assets.value.items.length : 0,
        generatedAssetCount: assets.status === 'fulfilled'
          ? assets.value.items.filter((asset) => asset.version.source_type === 'provider_generated').length
          : 0,
      })
      if (failedCount > 0) setError(`${failedCount} 个模块的进度暂时不可用；已保留其余真实数据。`)
    }).catch((cause: unknown) => {
      if (cause instanceof DOMException && cause.name === 'AbortError') return
      setError(cause instanceof Error ? cause.message : '无法读取项目进度。')
    }).finally(() => {
      if (!controller.signal.aborted) setLoading(false)
    })
    return () => controller.abort()
  }, [projectId])

  const latestTask = snapshot.tasks[0]
  const currentStage = useMemo(() => {
    if (snapshot.experiences.length > 0) return 8
    if (snapshot.reports.length > 0) return 7
    if (snapshot.executions.length > 0 || snapshot.deliveryPlans.length > 0) return 6
    if (snapshot.creativePackages.length > 0 || snapshot.versions.length > 0 || snapshot.generatedAssetCount > 0) return 5
    if (snapshot.tasks.length > 0) return 3
    if (snapshot.packages.length > 0 || snapshot.intakes.length > 0) return 2
    return 1
  }, [snapshot])

  const stages = [
    { number: 1, name: '需求与策略', description: `${snapshot.workspaces.length} 个策略工作区`, href: strategyPath(projectId), available: true, complete: snapshot.workspaces.length > 0 || snapshot.packages.length > 0 },
    { number: 2, name: '脚本与创意', description: `${snapshot.packages.length} 个策略交付 · ${snapshot.intakes.length} 个创意输入`, href: creativeTasksPath(projectId), available: true, complete: snapshot.intakes.length > 0 || snapshot.tasks.length > 0 },
    { number: 3, name: '广告素材生成', description: `${snapshot.tasks.length} 个图文任务 · ${snapshot.generatedAssetCount} 个生成素材`, href: latestTask ? creativeTaskPath(projectId, latestTask.id, 'production') : creativeTasksPath(projectId), available: true, complete: snapshot.generatedAssetCount > 0 },
    { number: 4, name: '剪辑出片', description: '当前项目只有图文任务，视频 EditTask 尚未接入', href: '', available: false, complete: false, skipped: true },
    { number: 5, name: '广告评审', description: `${snapshot.creativePackages.length} 个 CreativePackage`, href: latestTask ? creativeTaskPath(projectId, latestTask.id, 'review') : creativeTasksPath(projectId), available: Boolean(latestTask), complete: snapshot.creativePackages.length > 0 },
    { number: 6, name: '广告投放', description: `${snapshot.deliveryPlans.length} 个计划 · ${snapshot.executions.length} 次受控执行`, href: deliveryPath(projectId), available: true, complete: snapshot.executions.length > 0 },
    { number: 7, name: '投放后分析', description: `${snapshot.reports.length} 份复盘报告`, href: insightPath(projectId, 'reports'), available: true, complete: snapshot.reports.some((item) => item.status === 'confirmed') },
    { number: 8, name: '经验沉淀', description: `${snapshot.experiences.length} 条已确认经验`, href: insightPath(projectId, 'experiences'), available: true, complete: snapshot.experiences.length > 0 },
  ]

  return <section className="project-home">
    <header className="project-page-header">
      <div>
        <nav className="breadcrumb" aria-label="面包屑"><Link to="/projects">项目</Link><span>/</span><span>{projectLabel(project)}</span></nav>
        <h1>{projectLabel(project)}</h1>
        <p>围绕同一个项目上下文，查看广告工作从策略输入到资产沉淀的真实进度。</p>
      </div>
      <Link className="button button--secondary button--compact" to={projectManagePath(projectId)}>项目管理</Link>
    </header>
    {error ? <div className="project-inline-error" role="alert"><strong>项目进度部分不可用</strong><span>{error}</span><button className="text-action" onClick={() => window.location.reload()} type="button">重新加载</button></div> : null}
    <div className="project-overview">
      <div className="project-overview__summary">
        <span className="page-eyebrow">CURRENT STAGE</span>
        <strong>{loading ? '正在读取…' : `0${currentStage} · ${stages[currentStage - 1].name}`}</strong>
        <p>这里不维护第二套业务状态，只根据真实模块对象推导当前阶段。</p>
      </div>
      <dl>
        <div><dt>项目状态</dt><dd>{project?.status === 'active' ? '进行中' : project?.status || '未知'}</dd></div>
        <div><dt>Project Context</dt><dd>v{project?.project_context_version ?? '—'}</dd></div>
        <div><dt>项目素材</dt><dd>{loading ? '—' : snapshot.assetCount}</dd></div>
      </dl>
    </div>
    <section className="project-flow" aria-labelledby="project-flow-title">
      <div className="project-section-heading">
        <div><span className="page-eyebrow">END-TO-END FLOW</span><h2 id="project-flow-title">八阶段广告工作流</h2></div>
        <small>数据来自 Strategy / Creative / Assets / Delivery / Insights API</small>
      </div>
      <ol className="project-stage-list">
        {stages.map((stage) => {
          const state = stage.skipped ? 'skipped' : stage.complete ? 'complete' : stage.number === currentStage ? 'current' : stage.number > currentStage + 1 ? 'blocked' : stage.available ? 'ready' : 'blocked'
          const content = <>
            <span className={`project-stage__number project-stage__number--${state}`}>{String(stage.number).padStart(2, '0')}</span>
            <div><strong>{stage.name}</strong><span>{loading && stage.available ? '正在读取真实进度…' : stage.description}</span></div>
            <span className={`project-stage__state project-stage__state--${state}`}>{state === 'complete' ? '已形成产物' : state === 'current' ? '当前阶段' : state === 'ready' ? '可进入' : state === 'skipped' ? '当前不适用' : '等待上游'}</span>
          </>
          return <li key={stage.number}>
            {stage.available && stage.href ? <Link className="project-stage" to={stage.href}>{content}</Link> : <div aria-disabled="true" className="project-stage project-stage--disabled">{content}</div>}
          </li>
        })}
      </ol>
    </section>
  </section>
}

export function ProjectManagePage({ project }: ProjectPageProps) {
  const { projectId = '' } = useParams()
  if (!project) return <section className="project-empty"><h1>找不到项目</h1><p>请返回项目中心重新选择。</p></section>
  return <section className="project-manage">
    <header className="project-page-header">
      <div>
        <nav className="breadcrumb" aria-label="面包屑"><Link to="/projects">项目</Link><span>/</span><Link to={projectHomePath(projectId)}>{project.name}</Link><span>/</span><span>管理</span></nav>
        <h1>项目管理</h1>
        <p>查看项目归属和不可混淆的共享上下文。当前页面不提供尚无后端契约的编辑操作。</p>
      </div>
      <Link className="button button--secondary button--compact" to={projectHomePath(projectId)}>返回项目</Link>
    </header>
    <div className="project-manage__layout">
      <section>
        <div className="project-section-heading"><div><span className="page-eyebrow">IDENTITY</span><h2>项目身份</h2></div></div>
        <dl className="project-detail-list">
          <div><dt>项目名称</dt><dd>{project.name}</dd></div>
          <div><dt>Project ID</dt><dd><code>{project.id}</code></dd></div>
          <div><dt>Organization ID</dt><dd><code>{project.organization_id}</code></dd></div>
          <div><dt>状态</dt><dd>{project.status}</dd></div>
        </dl>
      </section>
      <aside>
        <span className="page-eyebrow">SHARED CONTEXT</span>
        <h2>跨模块共享上下文</h2>
        <p>策略、创意和素材只能引用这里的项目身份，不应各自复制一份 Project。</p>
        <dl className="project-context-list">
          <div><dt>Context 版本</dt><dd>v{project.project_context_version}</dd></div>
          <div><dt>主品牌</dt><dd>{project.primary_brand_id || '未绑定'}</dd></div>
          <div><dt>品牌规范版本</dt><dd>{project.brand_guideline_version_id || '未绑定'}</dd></div>
        </dl>
      </aside>
    </div>
  </section>
}
