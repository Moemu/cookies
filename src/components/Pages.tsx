import { useEffect, useMemo, useState, type CSSProperties, type FormEvent } from 'react'
import { ArrowRight, Bot, Check, ChevronDown, CircleAlert, CircleCheck, Clock3, Download, ExternalLink, Filter, MoreHorizontal, Pencil, Plus, Search, Send, ShieldCheck, SlidersHorizontal } from 'lucide-react'
import { systems, quickActions } from '../data/navigation'
import { activity, chartPoints, deliveryActions, deliveryDiagnostics, evidence, manhuaMethods, manhuaMix, workItems } from '../data/mock'
import { unifiedRecords } from '../data/projects'
import { api, type ApiArtifact, type ApiAuditEvent } from '../data/api'
import { useProject } from '../context/ProjectContext'
import { useModelConfig } from '../context/ModelConfigContext'
import type { BusinessTaskRecord, BusinessTaskType, DataState, NavItem, SystemDefinition, SystemKey } from '../types'
import { TrendChart } from './Icons'
import { ApprovalCenterPage, ArtifactFlow, DeliveryPlanPage, ImageTextCreationPage, ReportCenterPage, VideoCreationPage } from './SpecializedPages'
import { AssetExperiencePage, PostLaunchAnalysisPage, PreLaunchInsightPage } from './CoreFlowPages'
import { TaskCenterPage, TaskCreateDialog } from './BusinessTaskPages'
import { StateBoundary, StatePreview } from './StateBoundary'

type OpenProject = (id: string, system?: SystemKey, navId?: string, objectId?: string, view?: string) => void

function creativeTaskDestination(task: BusinessTaskRecord): { navId: string; view?: string } {
  if (task.type === 'creative') return { navId: 'image-text' }
  if (task.type === 'brand_video') return { navId: 'video', view: '品牌广告' }
  if (task.type === 'video_edit') return { navId: 'video', view: '素材剪辑' }
  return { navId: 'video', view: '效果广告' }
}

const dashboardJourneys: Record<SystemKey, Array<{ label: string; detail: string; navId: string }>> = {
  strategy: [
    { label: '需求完整度', detail: '补齐目标、受众、边界和成功指标', navId: 'briefs' },
    { label: '策略任务', detail: '从 Brief 创建可追溯的策略任务', navId: 'tasks' },
    { label: '研究证据', detail: '组织受众、竞品与行业依据', navId: 'research' },
    { label: '策略评审', detail: '确认策略版本并交给创意生产', navId: 'reviews' },
  ],
  creative: [
    { label: '创意任务', detail: '继承已批准策略和渠道交付规格', navId: 'tasks' },
    { label: '图文创作', detail: '组织文案、版式、品牌和渠道检查', navId: 'image-text' },
    { label: '视频创作', detail: '品牌、效果广告与视频包装', navId: 'video' },
    { label: '生产与评审', detail: '跟踪生成、失败恢复和交付版本', navId: 'production' },
  ],
  insight: [
    { label: '投前洞察', detail: '为 Brief、策略与创意引用证据', navId: 'prelaunch' },
    { label: '投后分析', detail: '查看消耗、CTR、转化与素材驱动因素', navId: 'performance' },
    { label: '素材管理', detail: '统一管理视频、图片与授权来源', navId: 'assets' },
    { label: '经验沉淀', detail: '把验证结论转成可复用策略证据', navId: 'knowledge' },
  ],
  delivery: [
    { label: '投放计划', detail: '选择创意组合、预算和排期', navId: 'plans' },
    { label: '策略优化', detail: '把洞察转成受控 ChangeSet', navId: 'optimization' },
    { label: '审批中心', detail: '完成预检、审批和权限控制', navId: 'approvals' },
    { label: '执行与回滚', detail: '保留执行证据和回滚能力', navId: 'execution' },
  ],
}

function Status({ value }: { value: string }) {
  const kind = value.includes('完成') || value.includes('通过') ? 'success' : value.includes('失败') || value.includes('需处理') ? 'danger' : value.includes('生成') || value.includes('执行') ? 'info' : 'warning'
  return <span className={`status ${kind}`}><span />{value}</span>
}

export function HomePage({ onSystemChange, onOpenProject, onManageProject }: { onSystemChange: (key: SystemKey) => void; onOpenProject: (id: string, system?: SystemKey, navId?: string) => void; onManageProject: (id: string) => void }) {
  const { projects, createProject: createProjectRecord, error: projectError, isLoading } = useProject()
  const [creating, setCreating] = useState(false)
  const [filter, setFilter] = useState<'进行中' | '已完成' | '全部'>('进行中')
  const [name, setName] = useState('')
  const [brand, setBrand] = useState('白域精工')
  const [goal, setGoal] = useState('')

  const submitProject = async (event: FormEvent) => {
    event.preventDefault()
    if (!name.trim() || !goal.trim()) return
    try {
      const project = await createProjectRecord({ name: name.trim(), brand: brand.trim() || '未指定品牌', goal: goal.trim() })
      setName('')
      setGoal('')
      setCreating(false)
      onManageProject(project.id)
    } catch {
      // The provider surfaces the connection failure through the shared project state.
    }
  }
  const visibleProjects = filter === '全部' ? projects : projects.filter(project => project.status === filter)

  return <div className="home-page">
    <section className="home-hero">
      <div><span className="section-label">PROJECT HOME</span><h1>让每个增长项目，拥有清晰的下一步。</h1><p>项目连接需求、策略、创意、洞察与投放。创建后，四个系统将共享同一项目上下文。</p></div>
      <button className="primary-button" onClick={() => setCreating(true)}><Plus size={16}/>创建 Project</button>
    </section>
    {projectError ? <div className="page-notice" role="status"><CircleAlert size={16}/>{projectError}，当前显示上次可用的演示数据。</div> : null}
    {creating && <form className="project-create" onSubmit={submitProject}>
      <div className="create-intro"><span className="create-index">01</span><div><h2>创建新 Project</h2><p>先定义项目边界，进入系统后再完善 Brief、策略与执行配置。</p></div></div>
      <label><span>项目名称</span><input value={name} onChange={event => setName(event.target.value)} placeholder="例如：夏季新品增长计划" autoFocus /></label>
      <label><span>品牌或产品</span><input value={brand} onChange={event => setBrand(event.target.value)} placeholder="输入品牌或产品名称" /></label>
      <label className="goal-field"><span>项目目标</span><input value={goal} onChange={event => setGoal(event.target.value)} placeholder="例如：获得 1,000 条高质量销售线索" /></label>
      <div className="create-actions"><button type="button" className="secondary-button" onClick={() => setCreating(false)}>取消</button><button type="submit" className="primary-button" disabled={!name.trim() || !goal.trim()}>创建并进入项目</button></div>
    </form>}
    <div className="home-grid">
      <section className="projects-section">
        <div className="section-header"><div><span className="section-label">项目</span><h2>项目管理</h2></div><div className="project-filters">{(['进行中', '已完成', '全部'] as const).map(item => <button key={item} className={filter === item ? 'active' : ''} onClick={() => setFilter(item)} aria-pressed={filter === item}>{item}</button>)}</div></div>
        <div className="project-list">
          {visibleProjects.map((project, index) => <div className="project-row-shell" key={project.id}><button className="project-row" onClick={() => onOpenProject(project.id)}>
              <span className="project-order">{String(index + 1).padStart(2, '0')}</span>
              <span className="project-primary"><b>{project.name}</b><small>{project.brand} · {project.goal}</small></span>
              <span className="project-stage"><small>当前阶段</small><b>{project.stage}</b></span>
              <span className="project-progress"><i><em style={{width: `${project.progress}%`}}/></i><b>{project.progress}%</b></span>
              <span className="project-updated">{project.updatedAt.slice(5)}</span><ArrowRight size={16}/>
            </button><button className="project-manage-button" onClick={() => onManageProject(project.id)}>管理</button></div>)}
          {!visibleProjects.length ? <div className="project-empty">{isLoading ? '正在恢复 Project…' : '当前筛选下没有 Project'}</div> : null}
        </div>
      </section>
      <aside className="home-rail">
        <span className="section-label">四个系统</span><h2>进入当前工作</h2>
        {systems.map((item, index) => <button key={item.key} onClick={() => onSystemChange(item.key)}><span className="home-system-index">{String(index + 1).padStart(2, '0')}</span><item.icon size={17}/><span><b>{item.label}</b><small>{item.statement}</small></span><ArrowRight size={15}/></button>)}
      </aside>
    </div>
  </div>
}

function PageHeader({ system, item, activeView, onViewChange, onPrimaryAction, busy, actionLabel }: { system: SystemDefinition; item: NavItem; activeView: string; onViewChange: (v: string) => void; onPrimaryAction: () => void; busy: boolean; actionLabel?: string }) {
  return <>
    <div className="page-header">
      <div><div className="breadcrumb">{system.label} <span>/</span> {item.label}</div><h1>{item.label}</h1><p>{item.description}</p></div>
      {actionLabel ? <button className="primary-button" onClick={onPrimaryAction} disabled={busy}>{busy ? '正在保存…' : <><Plus size={16} />{actionLabel}</>}</button> : <span className="page-context-label">Project 数据自动关联 · 无需重复建任务</span>}
    </div>
    {item.views.length > 1 ? <div className="tabs" role="tablist" aria-label={`${item.label}视图`}>
      {item.views.map(view => <button key={view} role="tab" aria-selected={view === activeView} className={view === activeView ? 'tab active' : 'tab'} onClick={() => onViewChange(view)}>{view}</button>)}
    </div> : null}
  </>
}

export function DashboardPage({ system, onSystemChange, onOpenProject }: { system: SystemDefinition; onSystemChange: (key: SystemKey) => void; onOpenProject: OpenProject }) {
  const { currentProject } = useProject()
  const { configuredCount } = useModelConfig()
  const [notice, setNotice] = useState('')
  const [taskDomain, setTaskDomain] = useState<'strategy' | 'creative' | null>(null)
  const systemIndex = systems.findIndex(s => s.key === system.key)
  const currentItem = workItems[systemIndex]
  const journey = dashboardJourneys[system.key]
  const dashboardAction = system.key === 'strategy' ? '新建策略任务' : system.key === 'creative' ? '新建创意任务' : system.key === 'insight' ? '查看广告数据' : '配置投放计划'
  const runDashboardAction = () => {
    if (system.key === 'strategy' || system.key === 'creative') setTaskDomain(system.key)
    else onOpenProject(currentProject.id, system.key, system.key === 'insight' ? 'performance' : 'plans')
  }
  const taskCreated = (task: BusinessTaskRecord) => {
    setTaskDomain(null)
    setNotice(`${task.name} 已写入服务端并关联当前 Project`)
    onOpenProject(currentProject.id, system.key, 'tasks', task.id)
  }
  return <div className="page-frame dashboard-page">
    <div className="dashboard-intro">
      <div><div className="eyeline">2026 年 7 月 22 日，星期三</div><h1>早上好，Amelia</h1><p>{system.statement} 这里优先呈现需要你判断的工作。</p></div>
      <button className="primary-button" onClick={runDashboardAction}><Plus size={16} />{dashboardAction}</button>
    </div>
    {notice ? <div className="page-notice" role="status"><CircleCheck size={16}/>{notice}<button aria-label="关闭提示" onClick={() => setNotice('')}>×</button></div> : null}
    <section className={`demo-guide system-${system.key}`} aria-label={`${system.label}业务路径`}>
      <div className="demo-guide-heading"><div><span className="section-label">{system.shortLabel.toUpperCase()} WORKFLOW</span><h2>{system.label}的独立工作路径</h2><p>{system.statement} 所有页面都读取当前 Project 的同一条业务链路。</p></div><span className="source-chip">{currentProject.code}</span></div>
      <div className="demo-step-list">{journey.map((step, index) => <button key={step.label} onClick={() => onOpenProject(currentProject.id, system.key, step.navId)}><span>{String(index + 1).padStart(2, '0')}</span><div><b>{step.label}</b><small>{step.detail}</small></div><ArrowRight size={15}/></button>)}</div>
      {configuredCount === 0 ? <div className="demo-provider-notice" role="status"><CircleAlert size={15}/><span>未配置方舟 Provider：可完整讲解预置项目、预检、审批与审计；AI 生成按钮会保持禁用，不会使用或展示浏览器端密钥。</span></div> : null}
    </section>
    <section className="focus-band">
      <div className="focus-number">01</div>
      <div className="focus-main"><span className="section-label">现在需要关注</span><h2>{currentProject.name}</h2><p>{currentProject.stage}已推进至 {currentProject.progress}%，下一步需要确认关键决策与证据边界。</p><div className="focus-meta"><Status value={currentItem.status} /><span>负责人 {currentItem.owner}</span><span>更新于 {currentProject.updatedAt}</span></div></div>
      <div className="focus-progress"><div className="progress-ring" style={{'--progress': `${currentProject.progress * 3.6}deg`} as CSSProperties}><span>{currentProject.progress}<small>%</small></span></div><button className="text-button" onClick={() => onOpenProject(currentProject.id, system.key, system.key === 'strategy' ? 'workspaces' : system.key === 'creative' ? 'tasks' : system.key === 'insight' ? 'knowledge' : 'approvals')}>继续工作<ArrowRight size={15} /></button></div>
    </section>
    <div className="dashboard-grid">
      <section className="open-section workstream">
        <div className="section-header"><div><span className="section-label">跨系统进度</span><h2>{currentProject.name}</h2></div><button className="secondary-button" onClick={() => onOpenProject(currentProject.id, 'strategy', 'workspaces')}>查看项目总览</button></div>
        <ArtifactFlow compact/>
        <div className="work-list">
          {workItems.slice(0, 4).map(item => <div className="work-row" key={item.name}><div className="work-name"><b>{item.name}</b><small>{item.type} · {item.owner}</small></div><div className="inline-progress"><span style={{width: `${item.progress}%`}} /></div><strong>{item.progress}%</strong><Status value={item.status} /><button aria-label="更多操作"><MoreHorizontal size={17} /></button></div>)}
        </div>
      </section>
      <aside className="attention-rail">
        <div className="section-header"><div><span className="section-label">你的队列</span><h2>3 项待处理</h2></div></div>
        <div className="queue-list">
          <button onClick={() => onOpenProject(currentProject.id, 'strategy', 'workspaces', 'STR-2607-08')}><span className="queue-icon warning"><Clock3 size={16} /></span><span><b>确认品牌核心信息</b><small>策略 · 今天 12:00 前</small></span><ArrowRight size={15} /></button>
          <button onClick={() => onOpenProject(currentProject.id, 'insight', 'assets', 'EV-2607-24')}><span className="queue-icon danger"><CircleAlert size={16} /></span><span><b>处理素材映射异常</b><small>洞察 · 影响 12 个素材</small></span><ArrowRight size={15} /></button>
          <button onClick={() => onOpenProject(currentProject.id, 'delivery', 'approvals', 'CS-2607-018')}><span className="queue-icon info"><Bot size={16} /></span><span><b>审批投放 ChangeSet</b><small>投放 · 预计 ¥8,600</small></span><ArrowRight size={15} /></button>
        </div>
        <div className="quick-actions"><span className="section-label">快速开始</span>{quickActions.map(action => <button key={action.label} onClick={() => onSystemChange(action.system)}><span><b>{action.label}</b><small>{action.detail}</small></span><ArrowRight size={15} /></button>)}</div>
      </aside>
    </div>
    {taskDomain ? <TaskCreateDialog domain={taskDomain} onClose={() => setTaskDomain(null)} onCreated={taskCreated}/> : null}
  </div>
}

function WorkspaceSurface({ item, activeView }: { item: NavItem; activeView: string }) {
  const { currentProject, reloadProjects, updateArtifact } = useProject()
  const [briefPrompt, setBriefPrompt] = useState('')
  const [brief, setBrief] = useState<ApiArtifact | null>(null)
  const [briefModel, setBriefModel] = useState('')
  const [briefNotice, setBriefNotice] = useState('')
  const [isGenerating, setIsGenerating] = useState(false)

  useEffect(() => {
    let active = true
    void Promise.all([api.listArtifacts(currentProject.id), api.listJobs(currentProject.id)]).then(([artifacts, jobs]) => {
      const latest = artifacts.filter(artifact => artifact.kind === 'brief').at(-1)
      if (active && latest) {
        setBrief(latest)
        setBriefModel(jobs.find(job => job.id === latest.sourceJobId)?.model ?? '服务端已存档')
      }
    }).catch(() => undefined)
    return () => { active = false }
  }, [currentProject.id])

  const generateBrief = async () => {
    if (!briefPrompt.trim()) {
      setBriefNotice('请先输入需求、受众或业务目标。')
      return
    }
    setIsGenerating(true)
    try {
      const result = await api.generateBrief(currentProject.id, briefPrompt)
      setBrief(result.artifact)
      setBriefModel(result.job.model ?? '服务端默认文本模型')
      await reloadProjects()
      setBriefNotice('策略 Brief 草稿已生成，等待人工确认。')
    } catch (cause) {
      setBriefNotice(cause instanceof Error ? cause.message : '生成策略 Brief 失败，请重试。')
    } finally {
      setIsGenerating(false)
    }
  }

  const confirmBrief = async () => {
    if (!brief) return
    try {
      await updateArtifact('brief', { status: '已确认', summary: brief.content.slice(0, 52) })
      setBrief({ ...brief, status: 'ready' })
      setBriefNotice('Brief 已确认，可进入创意生成。')
    } catch (cause) {
      setBriefNotice(cause instanceof Error ? cause.message : '确认 Brief 失败，请重试。')
    }
  }

  return <div className="workspace-surface">
    <section className="document-panel">
      <div className="surface-toolbar"><div><span className="ai-chip"><Bot size={14} />{activeView}</span><span>{currentProject.artifacts.strategy.version} · 已引用 4 条证据</span></div><button className="secondary-button"><Pencil size={14} />编辑</button></div>
      {[
        ['推荐定位', '白域精工，以精密制造的可靠性为品牌核心，为创新产品提供高精度、高一致性与稳定交付。'],
        ['目标受众', '电子消费品品牌采购与供应链负责人（25–45 岁）\n产品研发工程师与工业设计师（25–40 岁）'],
        ['核心信息', '看得见的精度，兑现你的创新。精度 ±0.01mm，交付准时率 98%+。'],
        ['创意路线', '理性证据线：以精度、良率、交期为核心证据\n场景应用线：展示真实应用案例与制造过程'],
        ['成功指标', '官网表单提交量提升 ≥30%\n关键行业线索成本（CPL）降低 ≥20%'],
      ].map(([label, content], index) => <div className="strategy-row" key={label}><h3>{label}</h3><p>{content}</p><span className="citation">[{index + 1}]</span><button aria-label={`编辑${label}`}><Pencil size={15} /></button></div>)}
      <div className="prompt-box"><label htmlFor="ai-prompt">输入广告需求，生成策略 Brief</label><div><input id="ai-prompt" value={briefPrompt} onChange={event => setBriefPrompt(event.target.value)} placeholder="例如：面向研发工程师，突出新品精度与交期，获取销售线索"/><button aria-label="生成策略 Brief" onClick={() => void generateBrief()} disabled={isGenerating}>{isGenerating ? '生成中…' : <Send size={18} />}</button></div></div>
      {brief ? <div className="insight-note"><span>AI 生成 Brief · {brief.status === 'ready' ? '已确认' : '草稿'}</span><p>{brief.content}</p><small>模型：{briefModel || '服务端已存档'} · 任务：{brief.sourceJobId ?? '手工创建'}</small>{brief.status !== 'ready' ? <button className="secondary-button" onClick={() => void confirmBrief()}>确认 Brief</button> : null}</div> : null}
      {briefNotice ? <div className="inline-notice" role="status">{briefNotice}</div> : null}
    </section>
    <section className="brief-panel"><div className="surface-toolbar"><h3>对象摘要</h3><button className="text-button">编辑</button></div>{[['项目', currentProject.name], ['目标', currentProject.goal], ['核心产品', currentProject.product], ['主要区域', '中国大陆（华东、华南）'], ['预算', `¥${currentProject.budget.toLocaleString('zh-CN')}`], ['周期', '2026-07-25 至 2026-08-31']].map(([label, value]) => <div className="kv" key={label}><span>{label}</span><b>{value}</b></div>)}<div className="decision-block"><div><b>关键决策</b><span>4/5 已确认</span></div>{['品牌主张', '核心信息', '受众定义', '创意路线', '成功指标'].map((v, i) => <div key={v}><span>{v}</span><Status value={i === 0 ? '已确认' : 'AI 建议'} /></div>)}</div></section>
    <aside className="evidence-panel"><div className="surface-toolbar"><h3>证据</h3><button className="text-button">收起</button></div>{evidence.map(item => <button className="evidence-item" key={item.id}><span className="evidence-id">{item.id}</span><span><b>{item.title}</b><small>来源：{item.source}</small><small>{item.date} · {item.confidence}相关</small></span><ExternalLink size={14} /></button>)}</aside>
  </div>
}

function AnalysisSurface({ item, activeView }: { item: NavItem; activeView: string }) {
  return <div className="analysis-layout">
    <section className="analysis-main"><div className="analysis-heading"><div><span className="section-label">{activeView}</span><h2>{item.label}中，什么正在改变？</h2><p>观察窗口为 2026-05-01 至 2026-07-22，指标口径为进入投放的有效素材版本。</p></div><div className="metric-pair"><span><small>当前</small><b>86%</b></span><span><small>较基线</small><b className="positive">+18%</b></span></div></div><TrendChart points={chartPoints} /><div className="chart-axis"><span>W1</span><span>W4</span><span>W8</span><span>W12</span></div><div className="insight-note"><span>关键转折</span><p>{activeView}视图显示，第 9 周上线的“精度证据 + 真实场景”组合显著优于纯产品特写，95% 置信范围内差异为 +12% 至 +23%。</p></div></section>
    <aside className="analysis-rail"><span className="section-label">解释与行动</span><h3>三个主要驱动因素</h3>{[['01', '首屏主张更具体', '+8.4%'], ['02', '制造过程可见', '+6.1%'], ['03', '客户证据前置', '+3.5%']].map(([id, title, value]) => <div className="driver" key={id}><span>{id}</span><b>{title}</b><strong>{value}</strong></div>)}<button className="secondary-button full">查看证据与样本</button></aside>
  </div>
}

function MaterialInsightSurface() {
  const { advanceArtifact } = useProject()
  const [notice, setNotice] = useState('')
  const createMaterials = async () => {
    try {
      await advanceArtifact('creative', '制作中')
      setNotice('4 组测试素材已保存到创意制作队列')
    } catch (cause) {
      setNotice(cause instanceof Error ? cause.message : '创建测试素材失败，请重试。')
    }
  }
  return <div className="strategy-analysis-layout">
    <section className="strategy-analysis-main">
      <div className="analysis-heading"><div><span className="section-label">漫剧供需结构</span><h2>供给多，不等于消耗贡献高。</h2><p>动态漫与仿真人在来源样本中仅占 14% 供给，却贡献 38.13% 消耗。当前结论是“优先补充验证”，不是直接扩量。</p></div><span className="source-chip">来源样本 · 待账户验证</span></div>
      <div className="mix-legend"><span><i className="supply"/>供给占比</span><span><i className="spend"/>消耗占比</span></div>
      <div className="mix-table">
        {manhuaMix.map(row => <div className="mix-row" key={row.name}>
          <div><b>{row.name}</b><small>{row.signal}</small></div>
          <div className="mix-bars"><span className="mix-bar supply" style={{width: `${row.supply * 1.55}%`}}/><span className="mix-bar spend" style={{width: `${row.spend * 1.55}%`}}/></div>
          <div className="mix-values"><span>{row.supply}%</span><strong>{row.spend}%</strong></div>
        </div>)}
      </div>
      <div className="insight-note"><span>策略建议</span><p>先用同商品、同人群、同预算的小样本测试验证结构机会。首轮只改变制作方法或钩子，避免同时改变多个变量。</p></div>
    </section>
    <aside className="strategy-method-rail"><span className="section-label">推荐首轮素材池</span><h3>从低成本验证开始</h3>{manhuaMethods.map(item => <div className="method-card" key={item.id}><span>{item.id}</span><div><b>{item.name}</b><small>{item.detail}</small></div></div>)}<button className="primary-button full" onClick={() => void createMaterials()}>创建 4 组测试素材</button>{notice ? <div className="inline-notice" role="status">{notice}</div> : null}<p className="source-note">数据来自学习资料《漫剧素材分析》，比例保留原始样本语境。</p></aside>
  </div>
}

function DeliveryStrategySurface() {
  const { addChangeSet } = useProject()
  const [notice, setNotice] = useState('')
  const createChangeSet = async () => {
    try {
      const change = await addChangeSet()
      setNotice(`${change.id} 已在服务端创建`)
    } catch (cause) {
      setNotice(cause instanceof Error ? cause.message : '创建 ChangeSet 失败，请重试。')
    }
  }
  return <div className="strategy-analysis-layout">
    <section className="strategy-analysis-main delivery-strategy">
      <div className="analysis-heading"><div><span className="section-label">商品 × 素材诊断</span><h2>先减少重复，再为新素材留出探索空间。</h2><p>当前同时出现起量放缓和组合重复信号，建议生成 ChangeSet；任何暂停、删除和预算动作仍需人工审批。</p></div><span className="source-chip alert">3 项需处理</span></div>
      <div className="diagnostic-grid">{deliveryDiagnostics.map(item => <div className={`diagnostic-card ${item.tone}`} key={item.id}><span>{item.id}</span><small>{item.name}</small><b>{item.value}</b><p>{item.detail}</p></div>)}</div>
      <div className="action-table"><div className="action-head"><span>优先级</span><span>建议动作</span><span>依据</span><span>预计影响</span></div>{deliveryActions.map(item => <div className="action-row" key={item.priority}><strong>{item.priority}</strong><b>{item.name}</b><span>{item.detail}</span><em>{item.impact}</em></div>)}</div>
    </section>
    <aside className="strategy-method-rail"><span className="section-label">执行边界</span><h3>自动建议，人工决策</h3>{['准确绑定商品与资产', '统计重复组合与无消耗广告', '新素材改变核心内容', '变更进入 ChangeSet 审批'].map((item, index) => <div className="guardrail" key={item}><CircleCheck size={16}/><span><b>{String(index + 1).padStart(2, '0')}</b>{item}</span></div>)}<button className="primary-button full" onClick={() => void createChangeSet()}>生成优化 ChangeSet</button>{notice ? <div className="inline-notice" role="status">{notice}</div> : null}<p className="source-note">60% / 90% 差异与 5–10% 探索预算均为来源建议，不是平台保证。</p></aside>
  </div>
}

function EditorSurface({ item, activeView }: { item: NavItem; activeView: string }) {
  const { providers } = useModelConfig()
  const { currentProject, reloadProjects } = useProject()
  const [selected, setSelected] = useState(1)
  const [description, setDescription] = useState('高速主轴切削金属零件的微距镜头，冷白光，真实工业质感。')
  const [notice, setNotice] = useState('')
  const [job, setJob] = useState<Awaited<ReturnType<typeof api.createMedia>> | null>(null)
  const [confirmedBriefId, setConfirmedBriefId] = useState('')
  const configuredProvider = providers.find(provider => provider.status === '已配置')
  const mediaKind = item.id === 'video' ? 'video' : 'image'

  useEffect(() => {
    let active = true
    void Promise.all([api.listArtifacts(currentProject.id), api.listJobs(currentProject.id)]).then(([artifacts, jobs]) => {
      const latest = jobs.filter(candidate => candidate.artifactKind === mediaKind).at(-1)
      const brief = artifacts.filter(artifact => artifact.kind === 'brief' && artifact.status === 'ready').at(-1)
      if (active) {
        setJob(latest ?? null)
        setConfirmedBriefId(brief?.id ?? '')
      }
    }).catch(() => undefined)
    return () => { active = false }
  }, [currentProject.id, mediaKind])

  useEffect(() => {
    if (!job || !['queued', 'running'].includes(job.status)) return
    const timer = window.setInterval(() => {
      void api.getJob(job.id).then(next => {
        setJob(next)
        if (next.status === 'succeeded') {
          void reloadProjects()
          setNotice('生成完成，稳定资产已关联到当前任务。')
        }
      }).catch(cause => setNotice(cause instanceof Error ? cause.message : '任务状态读取失败'))
    }, 1500)
    return () => window.clearInterval(timer)
  }, [job, mediaKind, reloadProjects])

  const generate = async () => {
    if (!confirmedBriefId) {
      setNotice('请先在需求中心确认 Brief，再发起图片或视频生成。')
      return
    }
    try {
      const next = await api.createMedia(currentProject.id, mediaKind, description, confirmedBriefId)
      setJob(next)
      setNotice(next.status === 'succeeded' ? '生成完成，资产已保存。' : '生成任务已创建，正在轮询状态。')
    } catch (cause) {
      setNotice(cause instanceof Error ? cause.message : '创建生成任务失败，请重试。')
    }
  }

  const cancel = async () => {
    if (!job) return
    try {
      setJob(await api.cancelJob(job.id))
      setNotice('生成任务已取消。')
    } catch (cause) {
      setNotice(cause instanceof Error ? cause.message : '取消任务失败。')
    }
  }

  return <div className="editor-layout">
    <aside className="asset-rail"><div className="surface-toolbar"><h3>结构与素材</h3><button aria-label="新增镜头"><Plus size={15}/></button></div>{['开场：精度的瞬间', '产品与制造过程', '真实应用场景', '品牌主张与 CTA'].map((label, i) => <button className={i === selected ? 'asset-row active' : 'asset-row'} onClick={() => setSelected(i)} key={label}><span>{String(i + 1).padStart(2, '0')}</span><b>{label}</b><small>{i === 1 ? '00:06–00:18' : `${i * 8 + 1} 秒`}</small></button>)}</aside>
    <section className="canvas-area"><div className="canvas-toolbar"><span>{item.label} · v1.2</span><div><button>50%</button><button><Download size={15}/>导出预览</button></div></div><div className="media-canvas"><div className="precision-art"><img src="/assets/white-precision-cnc.png" alt="高精度 CNC 设备加工金属零件"/><div className="art-copy"><small>WHITE PRECISION</small><h2>看得见的精度，<br/>兑现你的创新。</h2><p>±0.01mm · 98%+ 准时交付</p></div></div></div><div className="timeline"><div className="time-ruler">00:00 <span>00:06</span><span>00:12</span><span>00:18</span><span>00:24</span><span>00:30</span></div>{['画面', '字幕', '音乐'].map((track, index) => <div className="track" key={track}><b>{track}</b><span className={`clip clip-${index + 1}`}>{index === 0 ? '精密加工 · 06–18s' : index === 1 ? '品牌主张' : 'Precision Theme.wav'}</span></div>)}</div></section>
    <aside className="inspector"><div className="surface-toolbar"><h3>{activeView}属性</h3><button aria-label="属性更多操作"><MoreHorizontal size={16}/></button></div>{['内容', '画面', '声音', '品牌检查'].map((tab, i) => <button className={i === 0 ? 'inspector-tab active' : 'inspector-tab'} key={tab}>{tab}<ChevronDown size={14}/></button>)}<div className="field"><label>镜头描述</label><textarea value={description} onChange={event => setDescription(event.target.value)}/></div><div className="field"><label>生成模型</label><button className="select-field">{configuredProvider ? `${configuredProvider.name} · 服务端模型目录` : '服务端未配置模型'}<ChevronDown size={14}/></button></div>{!configuredProvider ? <div className="model-required"><CircleAlert size={15}/><span>请在服务端设置 ARK_API_KEY 后重新检查能力。</span></div> : null}{!confirmedBriefId ? <div className="model-required"><CircleAlert size={15}/><span>请先在需求中心确认 Brief，系统才会允许生成媒体。</span></div> : null}<button className="primary-button full" disabled={!configuredProvider || !confirmedBriefId || ['queued', 'running'].includes(job?.status ?? '')} onClick={() => void generate()}>{job && ['queued', 'running'].includes(job.status) ? '正在生成…' : `生成选中${mediaKind === 'image' ? '图片' : '视频'}`}</button>{job ? <div className="inline-notice" role="status">任务 {job.id.slice(0, 8)} · {job.status} · {job.model ?? '模型待分配'}{job.diagnostic ? ` · ${job.diagnostic}` : ''}{['queued', 'running'].includes(job.status) ? <button onClick={() => void cancel()}>取消</button> : job.status === 'failed' || job.status === 'cancelled' ? <button onClick={() => void generate()}>重试</button> : null}</div> : null}{notice ? <div className="inline-notice" role="status">{notice}</div> : null}</aside>
  </div>
}

function TableSurface({ item, activeView, onOpenRecord }: { item: NavItem; activeView: string; onOpenRecord: (id: string) => void }) {
  const [search, setSearch] = useState('')
  const [attentionOnly, setAttentionOnly] = useState(false)
  const [showOwner, setShowOwner] = useState(true)
  const [page, setPage] = useState(0)
  const pageSize = 4
  const filtered = useMemo(() => unifiedRecords.filter(record => `${record.id} ${record.name} ${record.kind} ${record.status}`.toLowerCase().includes(search.toLowerCase()) && (!attentionOnly || ['待审批', '待确认'].includes(record.status))), [search, attentionOnly])
  const pageCount = Math.max(1, Math.ceil(filtered.length / pageSize))
  const rows = filtered.slice(page * pageSize, page * pageSize + pageSize)
  return <section className="table-surface"><div className="table-toolbar"><div className="search-field"><Search size={16}/><input aria-label="搜索列表" value={search} onChange={event => { setSearch(event.target.value); setPage(0) }} placeholder={`搜索${item.label}`}/></div><button className={attentionOnly ? 'secondary-button active-filter' : 'secondary-button'} onClick={() => { setAttentionOnly(value => !value); setPage(0) }} aria-pressed={attentionOnly}><Filter size={15}/>待处理</button><button className="secondary-button" onClick={() => setShowOwner(value => !value)} aria-pressed={showOwner}><SlidersHorizontal size={15}/>{showOwner ? '隐藏负责人' : '显示负责人'}</button><span className="table-count">{activeView} · 共 {filtered.length} 条</span></div><table><thead><tr><th>编号</th><th>名称</th><th>类型</th><th>状态</th>{showOwner ? <th>负责人</th> : null}<th>最后更新</th><th aria-label="操作"/></tr></thead><tbody>{rows.map(row => <tr key={row.id}><td className="code">{row.id}</td><td><button className="table-object-link" onClick={() => onOpenRecord(row.id)}><b>{row.name}</b><small>春季新品上市</small></button></td><td>{row.kind}</td><td><Status value={row.status}/></td>{showOwner ? <td>{row.owner}</td> : null}<td>{row.updatedAt}</td><td><button aria-label={`${row.name}更多操作`} onClick={() => onOpenRecord(row.id)}><MoreHorizontal size={17}/></button></td></tr>)}</tbody></table>{!rows.length ? <div className="table-empty">没有匹配记录，请调整搜索或筛选条件。</div> : null}<div className="table-footer"><span>第 {page + 1} / {pageCount} 页</span><div><button disabled={page === 0} onClick={() => setPage(value => Math.max(0, value - 1))}>上一页</button><button disabled={page >= pageCount - 1} onClick={() => setPage(value => Math.min(pageCount - 1, value + 1))}>下一页</button></div></div></section>
}

function OperationsSurface({ item }: { item: NavItem }) {
  return <div className="ops-layout"><section className="ops-main"><div className="ops-status"><span className="signal ok"><CircleCheck size={18}/></span><div><span className="section-label">系统状态</span><h2>{item.label}运行稳定</h2><p>截至 2026-07-22 16:30，过去 24 小时完成 128 个任务，3 个任务等待人工输入。</p></div><button className="secondary-button">查看运行记录</button></div><div className="ops-list">{[['队列吞吐', '128 个任务', '正常'], ['平均处理时间', '4 分 12 秒', '正常'], ['等待用户', '3 个任务', '需关注'], ['失败重试', '1 个任务', '已恢复']].map(([name, value, status]) => <div key={name}><span>{name}</span><b>{value}</b><Status value={status}/><button aria-label={`查看${name}详情`}><ArrowRight size={15}/></button></div>)}</div></section><aside className="ops-rail"><span className="section-label">最近活动</span>{activity.map(item => <div className="activity-item" key={item.title}><time>{item.time}</time><span><b>{item.title}</b><small>{item.meta}</small></span></div>)}</aside></div>
}

function AuditEvidenceSurface() {
  const { currentProject } = useProject()
  const [events, setEvents] = useState<ApiAuditEvent[]>([])
  const [notice, setNotice] = useState('')

  useEffect(() => {
    let active = true
    void api.listAuditEvents(currentProject.id).then(records => {
      if (active) setEvents(records)
    }).catch(cause => {
      if (active) setNotice(cause instanceof Error ? cause.message : '读取审计记录失败')
    })
    return () => { active = false }
  }, [currentProject.id])

  return <div className="audit-evidence-surface">
    <section>
      <div className="audit-evidence-heading"><div><span className="section-label">SERVER AUDIT</span><h2>服务端审计轨迹</h2><p>记录预置项目的创建、产物确认、预检、审批、模拟执行与回滚；不会连接真实广告平台。</p></div><span className="source-chip">不可变事件</span></div>
      <div className="audit-event-list">{events.length ? events.map(event => <article key={event.id}><span>{new Date(event.createdAt).toLocaleString('zh-CN', { hour12: false })}</span><div><b>{auditActionLabel(event.action)}</b><small>{event.actor} · {event.entityType} · {event.entityId.slice(0, 8)}</small></div><CircleCheck size={16}/></article>) : <div className="panel-empty">正在读取服务端审计记录…</div>}</div>
    </section>
    <aside className="audit-boundary"><ShieldCheck size={18}/><h3>模拟边界</h3><p>这些事件只记录本地 MVP 的受控投放模拟。审批、执行和回滚不会对广告账户或外部平台写入。</p></aside>
    {notice ? <div className="inline-notice" role="status">{notice}</div> : null}
  </div>
}

function auditActionLabel(action: string): string {
  const labels: Record<string, string> = {
    'project.created': '已创建路演项目',
    'artifact.created': '已保存路演产物',
    'artifact.updated': '已更新产物状态',
    'change_set.created': '已创建 ChangeSet',
    'change_set.preflight_completed': '已完成投放预检',
    'change_set.approved': '已通过人工审批',
    'change_set.simulation_started': '已开始模拟执行',
    'change_set.simulation_completed': '已完成模拟执行',
    'change_set.rollback_started': '已开始模拟回滚',
    'change_set.rolled_back': '已完成模拟回滚',
  }
  return labels[action] ?? action
}

function SettingsSurface() {
  const [section, setSection] = useState('基础配置')
  const [autoSave, setAutoSave] = useState(true)
  return <div className="settings-layout"><aside className="settings-index">{['基础配置', '流程与状态', '通知规则', '权限边界', '导出与命名'].map(v => <button className={section === v ? 'active' : ''} onClick={() => setSection(v)} key={v}>{v}</button>)}</aside><section className="settings-form"><div><h2>{section}</h2><p>这些配置适用于当前组织和全部新建项目。</p></div>{[['默认项目时区', 'Asia/Shanghai'], ['默认货币', '人民币（CNY）'], ['数据保留期', '365 天'], ['自动保存', autoSave ? '开启' : '关闭']].map(([label, value], i) => <div className="setting-row" key={label}><div><b>{label}</b><small>{i === 3 ? '编辑内容后每 30 秒保存一个草稿版本。' : '用于新对象和报表的默认值。'}</small></div>{i === 3 ? <button className={autoSave ? 'switch active' : 'switch'} onClick={() => setAutoSave(value => !value)} aria-label={autoSave ? '关闭自动保存' : '开启自动保存'} aria-pressed={autoSave}><span/></button> : <button className="select-field">{value}<ChevronDown size={14}/></button>}</div>)}</section></div>
}

function ObjectDetail({ system, item, objectId, onOpenProject }: { system: SystemDefinition; item: NavItem; objectId: string; onOpenProject: OpenProject }) {
  const { currentProject } = useProject()
  const record = unifiedRecords.find(value => value.id === objectId)
  const name = record?.name ?? `${item.label}草稿 ${objectId}`
  const next = system.key === 'strategy' ? ['creative', 'tasks', 'CR-2607-42', '基于此策略创建创意任务'] as const : system.key === 'creative' ? ['creative', 'reviews', 'CR-2607-42', '提交评审'] as const : system.key === 'insight' ? ['strategy', 'workspaces', 'STR-2607-08', '将洞察应用到策略'] as const : ['delivery', 'execution', objectId, '进入执行中心'] as const
  return <aside className="object-detail" aria-label={`${name}详情`}><div><span className="section-label">Mock 对象详情</span><h2>{name}</h2><p>{record ? `${record.kind} · ${record.status} · ${record.owner}` : `当前 Project：${currentProject.name}`}</p></div><div className="detail-kv"><span>对象 ID</span><b>{objectId}</b></div><div className="detail-kv"><span>来源版本</span><b>{currentProject.artifacts.strategy.version} → {currentProject.artifacts.creative.version}</b></div><button className="primary-button full" onClick={() => onOpenProject(currentProject.id, next[0], next[1], next[2])}>{next[3]}<ArrowRight size={15}/></button><button className="secondary-button full" onClick={() => onOpenProject(currentProject.id, system.key, item.id)}>返回{item.label}列表</button></aside>
}

export function ModulePage({ system, item, objectId, routeView, onOpenProject }: { system: SystemDefinition; item: NavItem; objectId?: string; routeView?: string; onOpenProject: OpenProject }) {
  const [activeView, setActiveView] = useState(() => routeView && item.views.includes(routeView) ? routeView : item.views[0])
  const [dataState, setDataState] = useState<DataState>('ready')
  const [busy, setBusy] = useState(false)
  const [notice, setNotice] = useState('')
  const [taskDialog, setTaskDialog] = useState<{ domain: 'strategy' | 'creative'; initialType?: BusinessTaskType } | null>(null)
  const { currentProject, addChangeSet } = useProject()

  useEffect(() => { if (routeView && item.views.includes(routeView)) setActiveView(routeView) }, [item.views, routeView])

  const primaryAction = async () => {
    if (system.key === 'strategy' && item.id === 'tasks') {
      setTaskDialog({ domain: 'strategy', initialType: 'strategy' })
      return
    }
    if (system.key === 'creative' && item.id === 'tasks') {
      setTaskDialog({ domain: 'creative', initialType: 'creative' })
      return
    }
    setBusy(true)
    try {
      if (system.key === 'delivery' && item.id === 'optimization') {
        const change = await addChangeSet()
        setNotice(`${change.id} 已在服务端创建，已进入审批中心继续处理。`)
        onOpenProject(currentProject.id, 'delivery', 'approvals', change.id)
      } else if (item.layout === 'settings') {
        setNotice(`${system.label}配置已保存为新版本。`)
      }
    } catch (cause) {
      setNotice(cause instanceof Error ? cause.message : '保存失败，请在服务恢复后重试。')
    } finally {
      setBusy(false)
    }
  }

  let surface
  const taskDomain = system.key === 'strategy' || system.key === 'creative' ? system.key : null
  const taskCenter = item.id === 'tasks' && taskDomain !== null
  const specialized = taskCenter && taskDomain ? <TaskCenterPage state={dataState} domain={taskDomain} activeView={activeView} selectedId={objectId} onOpenTask={id => onOpenProject(currentProject.id, taskDomain, 'tasks', id, activeView)} onRequestCreate={() => setTaskDialog({ domain: taskDomain, initialType: taskDomain === 'strategy' ? 'strategy' : 'creative' })} onContinueTask={taskDomain === 'creative' ? task => { const destination = creativeTaskDestination(task); onOpenProject(currentProject.id, 'creative', destination.navId, task.id, destination.view) } : undefined}/>
    : system.key === 'creative' && item.id === 'image-text' ? <ImageTextCreationPage state={dataState} activeTaskId={objectId}/>
    : system.key === 'creative' && item.id === 'video' ? <VideoCreationPage state={dataState} activeView={activeView} activeTaskId={objectId} onOpenTask={id => onOpenProject(currentProject.id, 'creative', 'tasks', id)}/>
    : system.key === 'insight' && item.id === 'prelaunch' ? <PreLaunchInsightPage state={dataState} onOpenProject={onOpenProject}/>
    : system.key === 'insight' && item.id === 'performance' ? <PostLaunchAnalysisPage state={dataState} onOpenProject={onOpenProject}/>
    : system.key === 'insight' && item.id === 'assets' ? <AssetExperiencePage state={dataState} mode="assets"/>
    : system.key === 'insight' && item.id === 'knowledge' ? <AssetExperiencePage state={dataState} mode="knowledge"/>
    : system.key === 'insight' && item.id === 'reports' ? <ReportCenterPage state={dataState}/>
    : system.key === 'delivery' && item.id === 'plans' ? <DeliveryPlanPage state={dataState}/>
    : system.key === 'delivery' && item.id === 'approvals' ? <ApprovalCenterPage state={dataState}/>
    : system.key === 'delivery' && item.id === 'evidence' ? <AuditEvidenceSurface/>
    : null
  if (specialized) surface = specialized
  else {
    const analysisSurface = system.key === 'insight' && item.id === 'content' ? <MaterialInsightSurface/> : system.key === 'delivery' && item.id === 'optimization' ? <DeliveryStrategySurface/> : <AnalysisSurface item={item} activeView={activeView}/>
    const genericSurface = item.layout === 'workspace' ? <WorkspaceSurface item={item} activeView={activeView}/> : item.layout === 'analysis' ? analysisSurface : item.layout === 'editor' ? <EditorSurface item={item} activeView={activeView}/> : item.layout === 'table' ? <TableSurface item={item} activeView={activeView} onOpenRecord={id => onOpenProject(currentProject.id, system.key, item.id, id, activeView)}/> : item.layout === 'settings' ? <SettingsSurface/> : <OperationsSurface item={item}/>
    surface = <StateBoundary state={dataState} onRetry={() => setDataState('ready')} onCreate={primaryAction}>{genericSurface}</StateBoundary>
  }

  const actionLabel = system.key === 'strategy' && item.id === 'tasks' ? '新建策略任务'
    : system.key === 'creative' && item.id === 'tasks' ? '新建创意任务'
    : system.key === 'delivery' && item.id === 'optimization' ? '生成 ChangeSet'
    : item.layout === 'settings' ? '保存配置'
    : undefined
  const taskCreated = (task: BusinessTaskRecord) => {
    setTaskDialog(null)
    setNotice(`${task.name} 已写入服务端并关联当前 Project`)
    onOpenProject(currentProject.id, system.key, 'tasks', task.id)
  }

  return <div className={`module-page page-frame layout-${item.layout}`}><PageHeader system={system} item={item} activeView={activeView} onViewChange={view => { setActiveView(view); onOpenProject(currentProject.id, system.key, item.id, undefined, view) }} onPrimaryAction={() => { void primaryAction() }} busy={busy} actionLabel={actionLabel}/>{import.meta.env.VITE_SHOW_STATE_PREVIEW === 'true' ? <StatePreview value={dataState} onChange={setDataState}/> : null}{notice ? <div className="page-notice" role="status"><CircleCheck size={16}/>{notice}<button aria-label="关闭提示" onClick={() => setNotice('')}>×</button></div> : null}<div className={objectId && !taskCenter ? 'page-surface with-object-detail' : 'page-surface'}>{surface}{objectId && !taskCenter ? <ObjectDetail system={system} item={item} objectId={objectId} onOpenProject={onOpenProject}/> : null}</div><footer className="statusbar"><span>Project：{currentProject.name}</span><span>业务任务：{currentProject.tasks.length}</span><span>更新时间：{currentProject.updatedAt}</span><span>预算：¥{currentProject.budget.toLocaleString('zh-CN')}</span><strong>状态：已同步</strong></footer>{taskDialog ? <TaskCreateDialog domain={taskDialog.domain} initialType={taskDialog.initialType} onClose={() => setTaskDialog(null)} onCreated={taskCreated}/> : null}</div>
}
