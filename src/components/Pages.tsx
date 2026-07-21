import { useState, type CSSProperties, type FormEvent } from 'react'
import { ArrowRight, Bot, Check, ChevronDown, CircleAlert, CircleCheck, Clock3, Download, ExternalLink, Filter, MoreHorizontal, Pencil, Plus, Search, Send, SlidersHorizontal } from 'lucide-react'
import { systems, quickActions } from '../data/navigation'
import { activity, chartPoints, deliveryActions, deliveryDiagnostics, evidence, manhuaMethods, manhuaMix, records, workItems } from '../data/mock'
import type { NavItem, SystemDefinition, SystemKey } from '../types'
import { TrendChart } from './Icons'

function Status({ value }: { value: string }) {
  const kind = value.includes('完成') || value.includes('通过') ? 'success' : value.includes('失败') || value.includes('需处理') ? 'danger' : value.includes('生成') || value.includes('执行') ? 'info' : 'warning'
  return <span className={`status ${kind}`}><span />{value}</span>
}

interface HomeProject {
  id: string
  name: string
  brand: string
  goal: string
  stage: string
  progress: number
  updated: string
}

const seededProjects: HomeProject[] = [
  { id: 'PRJ-2506-01', name: '春季新品上市', brand: '白域精工', goal: '新品上市与销售线索增长', stage: '创意制作', progress: 58, updated: '12 分钟前' },
  { id: 'PRJ-2505-08', name: '华东行业增长计划', brand: '白域精工', goal: '重点行业客户拓展', stage: '素材洞察', progress: 76, updated: '昨天' },
  { id: 'PRJ-2504-12', name: '精密制造品牌升级', brand: '白域精工', goal: '品牌认知与可信证据建设', stage: '智能投放', progress: 88, updated: '3 天前' },
]

export function HomePage({ onSystemChange }: { onSystemChange: (key: SystemKey) => void }) {
  const [projects, setProjects] = useState(seededProjects)
  const [creating, setCreating] = useState(false)
  const [name, setName] = useState('')
  const [brand, setBrand] = useState('白域精工')
  const [goal, setGoal] = useState('')

  const createProject = (event: FormEvent) => {
    event.preventDefault()
    if (!name.trim() || !goal.trim()) return
    setProjects(current => [{ id: `PRJ-2506-${String(current.length + 2).padStart(2, '0')}`, name: name.trim(), brand: brand.trim() || '未指定品牌', goal: goal.trim(), stage: '需求确认', progress: 8, updated: '刚刚' }, ...current])
    setName('')
    setGoal('')
    setCreating(false)
  }

  return <div className="home-page">
    <section className="home-hero">
      <div><span className="section-label">PROJECT HOME</span><h1>让每个增长项目，拥有清晰的下一步。</h1><p>项目连接需求、策略、创意、洞察与投放。创建后，四个系统将共享同一项目上下文。</p></div>
      <button className="primary-button" onClick={() => setCreating(true)}><Plus size={16}/>创建 Project</button>
    </section>
    {creating && <form className="project-create" onSubmit={createProject}>
      <div className="create-intro"><span className="create-index">01</span><div><h2>创建新 Project</h2><p>先定义项目边界，进入系统后再完善 Brief、策略与执行配置。</p></div></div>
      <label><span>项目名称</span><input value={name} onChange={event => setName(event.target.value)} placeholder="例如：夏季新品增长计划" autoFocus /></label>
      <label><span>品牌或产品</span><input value={brand} onChange={event => setBrand(event.target.value)} placeholder="输入品牌或产品名称" /></label>
      <label className="goal-field"><span>项目目标</span><input value={goal} onChange={event => setGoal(event.target.value)} placeholder="例如：获得 1,000 条高质量销售线索" /></label>
      <div className="create-actions"><button type="button" className="secondary-button" onClick={() => setCreating(false)}>取消</button><button type="submit" className="primary-button" disabled={!name.trim() || !goal.trim()}>创建并进入项目</button></div>
    </form>}
    <div className="home-grid">
      <section className="projects-section">
        <div className="section-header"><div><span className="section-label">项目</span><h2>最近 Project</h2></div><div className="project-filters"><button className="active">进行中</button><button>已完成</button><button>全部</button></div></div>
        <div className="project-list">
          {projects.map((project, index) => <button className="project-row" key={project.id} onClick={() => onSystemChange('strategy')}>
            <span className="project-order">{String(index + 1).padStart(2, '0')}</span>
            <span className="project-primary"><b>{project.name}</b><small>{project.brand} · {project.goal}</small></span>
            <span className="project-stage"><small>当前阶段</small><b>{project.stage}</b></span>
            <span className="project-progress"><i><em style={{width: `${project.progress}%`}}/></i><b>{project.progress}%</b></span>
            <span className="project-updated">{project.updated}</span><ArrowRight size={16}/>
          </button>)}
        </div>
      </section>
      <aside className="home-rail">
        <span className="section-label">四个系统</span><h2>进入当前工作</h2>
        {systems.map((item, index) => <button key={item.key} onClick={() => onSystemChange(item.key)}><span className="home-system-index">{String(index + 1).padStart(2, '0')}</span><item.icon size={17}/><span><b>{item.label}</b><small>{item.statement}</small></span><ArrowRight size={15}/></button>)}
      </aside>
    </div>
  </div>
}

function PageHeader({ system, item, activeView, onViewChange }: { system: SystemDefinition; item: NavItem; activeView: string; onViewChange: (v: string) => void }) {
  return <>
    <div className="page-header">
      <div><div className="breadcrumb">{system.label} <span>/</span> {item.label}</div><h1>{item.label}</h1><p>{item.description}</p></div>
      <button className="primary-button"><Plus size={16} />{item.layout === 'settings' ? '保存配置' : `新建${item.label.replace('中心', '').replace('工作台', '任务')}`}</button>
    </div>
    <div className="tabs" role="tablist" aria-label={`${item.label}视图`}>
      {item.views.map(view => <button key={view} role="tab" aria-selected={view === activeView} className={view === activeView ? 'tab active' : 'tab'} onClick={() => onViewChange(view)}>{view}{view.includes('待') && <em>{view === '待我评审' ? 3 : 2}</em>}</button>)}
    </div>
  </>
}

export function DashboardPage({ system, onSystemChange }: { system: SystemDefinition; onSystemChange: (key: SystemKey) => void }) {
  const systemIndex = systems.findIndex(s => s.key === system.key)
  const currentItem = workItems[systemIndex]
  return <div className="page-frame dashboard-page">
    <div className="dashboard-intro">
      <div><div className="eyeline">周二，6 月 17 日</div><h1>早上好，Amelia</h1><p>{system.statement} 这里优先呈现需要你判断的工作。</p></div>
      <button className="primary-button"><Plus size={16} />新建{system.shortLabel}任务</button>
    </div>
    <section className="focus-band">
      <div className="focus-number">01</div>
      <div className="focus-main"><span className="section-label">现在需要关注</span><h2>{currentItem.name}</h2><p>{currentItem.type}已推进至 {currentItem.progress}%，下一步需要确认关键决策与证据边界。</p><div className="focus-meta"><Status value={currentItem.status} /><span>负责人 {currentItem.owner}</span><span>更新于 {currentItem.updated}</span></div></div>
      <div className="focus-progress"><div className="progress-ring" style={{'--progress': `${currentItem.progress * 3.6}deg`} as CSSProperties}><span>{currentItem.progress}<small>%</small></span></div><button className="text-button">继续工作<ArrowRight size={15} /></button></div>
    </section>
    <div className="dashboard-grid">
      <section className="open-section workstream">
        <div className="section-header"><div><span className="section-label">跨系统进度</span><h2>春季新品上市</h2></div><button className="secondary-button">查看项目总览</button></div>
        <div className="blueprint-line">
          {systems.map((item, index) => <button key={item.key} className={item.key === system.key ? 'blueprint-step active' : 'blueprint-step'} onClick={() => onSystemChange(item.key)}><span className="step-node"><item.icon size={17} /></span><b>{item.shortLabel}</b><small>{index === 0 ? '已确认' : index === 1 ? '制作中' : index === 2 ? '待分析' : '待审批'}</small></button>)}
        </div>
        <div className="work-list">
          {workItems.slice(0, 4).map(item => <div className="work-row" key={item.name}><div className="work-name"><b>{item.name}</b><small>{item.type} · {item.owner}</small></div><div className="inline-progress"><span style={{width: `${item.progress}%`}} /></div><strong>{item.progress}%</strong><Status value={item.status} /><button aria-label="更多操作"><MoreHorizontal size={17} /></button></div>)}
        </div>
      </section>
      <aside className="attention-rail">
        <div className="section-header"><div><span className="section-label">你的队列</span><h2>3 项待处理</h2></div></div>
        <div className="queue-list">
          <button><span className="queue-icon warning"><Clock3 size={16} /></span><span><b>确认品牌核心信息</b><small>策略 · 今天 12:00 前</small></span><ArrowRight size={15} /></button>
          <button><span className="queue-icon danger"><CircleAlert size={16} /></span><span><b>处理素材映射异常</b><small>洞察 · 影响 12 个素材</small></span><ArrowRight size={15} /></button>
          <button><span className="queue-icon info"><Bot size={16} /></span><span><b>审批投放 ChangeSet</b><small>投放 · 预计 ¥8,600</small></span><ArrowRight size={15} /></button>
        </div>
        <div className="quick-actions"><span className="section-label">快速开始</span>{quickActions.map(action => <button key={action.label} onClick={() => onSystemChange(action.system)}><span><b>{action.label}</b><small>{action.detail}</small></span><ArrowRight size={15} /></button>)}</div>
      </aside>
    </div>
  </div>
}

function WorkspaceSurface({ item }: { item: NavItem }) {
  return <div className="workspace-surface">
    <section className="document-panel">
      <div className="surface-toolbar"><div><span className="ai-chip"><Bot size={14} />AI 建议</span><span>已引用 5 条证据</span></div><button className="secondary-button"><Pencil size={14} />编辑</button></div>
      {[
        ['推荐定位', '白域精工，以精密制造的可靠性为品牌核心，为创新产品提供高精度、高一致性与稳定交付。'],
        ['目标受众', '电子消费品品牌采购与供应链负责人（25–45 岁）\n产品研发工程师与工业设计师（25–40 岁）'],
        ['核心信息', '看得见的精度，兑现你的创新。精度 ±0.01mm，交付准时率 98%+。'],
        ['创意路线', '理性证据线：以精度、良率、交期为核心证据\n场景应用线：展示真实应用案例与制造过程'],
        ['成功指标', '官网表单提交量提升 ≥30%\n关键行业线索成本（CPL）降低 ≥20%'],
      ].map(([label, content], index) => <div className="strategy-row" key={label}><h3>{label}</h3><p>{content}</p><span className="citation">[{index + 1}]</span><button aria-label={`编辑${label}`}><Pencil size={15} /></button></div>)}
      <div className="prompt-box"><label htmlFor="ai-prompt">向 AI 提问或提出优化需求</label><div><input id="ai-prompt" placeholder="例如：请优化核心信息，使其更适合研发工程师受众"/><button aria-label="发送"><Send size={18} /></button></div></div>
    </section>
    <section className="brief-panel"><div className="surface-toolbar"><h3>对象摘要</h3><button className="text-button">编辑</button></div>{[['项目', '春季新品上市'], ['目标', '获取销售线索'], ['核心产品', '高精度 CNC 加工零部件'], ['主要区域', '中国大陆（华东、华南）'], ['预算', '¥1,000,000'], ['周期', '2025-03-15 至 2025-06-15']].map(([label, value]) => <div className="kv" key={label}><span>{label}</span><b>{value}</b></div>)}<div className="decision-block"><div><b>关键决策</b><span>4/5 已确认</span></div>{['品牌主张', '核心信息', '受众定义', '创意路线', '成功指标'].map((v, i) => <div key={v}><span>{v}</span><Status value={i === 0 ? '已确认' : 'AI 建议'} /></div>)}</div></section>
    <aside className="evidence-panel"><div className="surface-toolbar"><h3>证据</h3><button className="text-button">收起</button></div>{evidence.map(item => <button className="evidence-item" key={item.id}><span className="evidence-id">{item.id}</span><span><b>{item.title}</b><small>来源：{item.source}</small><small>{item.date} · {item.confidence}相关</small></span><ExternalLink size={14} /></button>)}</aside>
  </div>
}

function AnalysisSurface({ item }: { item: NavItem }) {
  return <div className="analysis-layout">
    <section className="analysis-main"><div className="analysis-heading"><div><span className="section-label">核心问题</span><h2>{item.label}中，什么正在改变？</h2><p>过去 12 周，优质素材完成度持续提升；第 9 周之后，增长主要来自更清晰的首屏信息与证据表达。</p></div><div className="metric-pair"><span><small>当前</small><b>86%</b></span><span><small>较基线</small><b className="positive">+18%</b></span></div></div><TrendChart points={chartPoints} /><div className="chart-axis"><span>W1</span><span>W4</span><span>W8</span><span>W12</span></div><div className="insight-note"><span>关键转折</span><p>第 9 周上线的“精度证据 + 真实场景”组合显著优于纯产品特写，95% 置信范围内差异为 +12% 至 +23%。</p></div></section>
    <aside className="analysis-rail"><span className="section-label">解释与行动</span><h3>三个主要驱动因素</h3>{[['01', '首屏主张更具体', '+8.4%'], ['02', '制造过程可见', '+6.1%'], ['03', '客户证据前置', '+3.5%']].map(([id, title, value]) => <div className="driver" key={id}><span>{id}</span><b>{title}</b><strong>{value}</strong></div>)}<button className="secondary-button full">查看证据与样本</button></aside>
  </div>
}

function MaterialInsightSurface() {
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
    <aside className="strategy-method-rail"><span className="section-label">推荐首轮素材池</span><h3>从低成本验证开始</h3>{manhuaMethods.map(item => <div className="method-card" key={item.id}><span>{item.id}</span><div><b>{item.name}</b><small>{item.detail}</small></div></div>)}<button className="primary-button full">创建 4 组测试素材</button><p className="source-note">数据来自学习资料《漫剧素材分析》，比例保留原始样本语境。</p></aside>
  </div>
}

function DeliveryStrategySurface() {
  return <div className="strategy-analysis-layout">
    <section className="strategy-analysis-main delivery-strategy">
      <div className="analysis-heading"><div><span className="section-label">商品 × 素材诊断</span><h2>先减少重复，再为新素材留出探索空间。</h2><p>当前同时出现起量放缓和组合重复信号，建议生成 ChangeSet；任何暂停、删除和预算动作仍需人工审批。</p></div><span className="source-chip alert">3 项需处理</span></div>
      <div className="diagnostic-grid">{deliveryDiagnostics.map(item => <div className={`diagnostic-card ${item.tone}`} key={item.id}><span>{item.id}</span><small>{item.name}</small><b>{item.value}</b><p>{item.detail}</p></div>)}</div>
      <div className="action-table"><div className="action-head"><span>优先级</span><span>建议动作</span><span>依据</span><span>预计影响</span></div>{deliveryActions.map(item => <div className="action-row" key={item.priority}><strong>{item.priority}</strong><b>{item.name}</b><span>{item.detail}</span><em>{item.impact}</em></div>)}</div>
    </section>
    <aside className="strategy-method-rail"><span className="section-label">执行边界</span><h3>自动建议，人工决策</h3>{['准确绑定商品与资产', '统计重复组合与无消耗广告', '新素材改变核心内容', '变更进入 ChangeSet 审批'].map((item, index) => <div className="guardrail" key={item}><CircleCheck size={16}/><span><b>{String(index + 1).padStart(2, '0')}</b>{item}</span></div>)}<button className="primary-button full">生成优化 ChangeSet</button><p className="source-note">60% / 90% 差异与 5–10% 探索预算均为来源建议，不是平台保证。</p></aside>
  </div>
}

function EditorSurface({ item }: { item: NavItem }) {
  return <div className="editor-layout">
    <aside className="asset-rail"><div className="surface-toolbar"><h3>结构与素材</h3><button><Plus size={15}/></button></div>{['开场：精度的瞬间', '产品与制造过程', '真实应用场景', '品牌主张与 CTA'].map((label, i) => <button className={i === 1 ? 'asset-row active' : 'asset-row'} key={label}><span>{String(i + 1).padStart(2, '0')}</span><b>{label}</b><small>{i === 1 ? '00:06–00:18' : `${i * 8 + 1} 秒`}</small></button>)}</aside>
    <section className="canvas-area"><div className="canvas-toolbar"><span>{item.label} · v1.2</span><div><button>50%</button><button><Download size={15}/>导出预览</button></div></div><div className="media-canvas"><div className="precision-art"><img src="/assets/white-precision-cnc.png" alt="高精度 CNC 设备加工金属零件"/><div className="art-copy"><small>WHITE PRECISION</small><h2>看得见的精度，<br/>兑现你的创新。</h2><p>±0.01mm · 98%+ 准时交付</p></div></div></div><div className="timeline"><div className="time-ruler">00:00 <span>00:06</span><span>00:12</span><span>00:18</span><span>00:24</span><span>00:30</span></div>{['画面', '字幕', '音乐'].map((track, index) => <div className="track" key={track}><b>{track}</b><span className={`clip clip-${index + 1}`}>{index === 0 ? '精密加工 · 06–18s' : index === 1 ? '品牌主张' : 'Precision Theme.wav'}</span></div>)}</div></section>
    <aside className="inspector"><div className="surface-toolbar"><h3>属性</h3><button><MoreHorizontal size={16}/></button></div>{['内容', '画面', '声音', '品牌检查'].map((tab, i) => <button className={i === 0 ? 'inspector-tab active' : 'inspector-tab'} key={tab}>{tab}<ChevronDown size={14}/></button>)}<div className="field"><label>镜头描述</label><textarea defaultValue="高速主轴切削金属零件的微距镜头，冷白光，真实工业质感。" /></div><div className="field"><label>生成模型</label><button className="select-field">cookies.video-pro v2<ChevronDown size={14}/></button></div><button className="primary-button full">生成选中镜头</button></aside>
  </div>
}

function TableSurface({ item }: { item: NavItem }) {
  return <section className="table-surface"><div className="table-toolbar"><div className="search-field"><Search size={16}/><input aria-label="搜索列表" placeholder={`搜索${item.label}`}/></div><button className="secondary-button"><Filter size={15}/>筛选</button><button className="secondary-button"><SlidersHorizontal size={15}/>字段</button><span className="table-count">共 24 条</span></div><table><thead><tr><th>编号</th><th>名称</th><th>状态</th><th>负责人</th><th>最后更新</th><th aria-label="操作"/></tr></thead><tbody>{records.map(row => <tr key={row[0]}><td className="code">{row[0]}</td><td><b>{row[1]}</b><small>春季新品上市</small></td><td><Status value={row[2]} /></td><td>{row[3]}</td><td>{row[4]}</td><td><button aria-label={`${row[1]}更多操作`}><MoreHorizontal size={17}/></button></td></tr>)}</tbody></table><div className="table-footer"><span>已显示 5 / 24 条</span><div><button disabled>上一页</button><button>下一页</button></div></div></section>
}

function OperationsSurface({ item }: { item: NavItem }) {
  return <div className="ops-layout"><section className="ops-main"><div className="ops-status"><span className="signal ok"><CircleCheck size={18}/></span><div><span className="section-label">系统状态</span><h2>{item.label}运行稳定</h2><p>过去 24 小时完成 128 个任务，3 个任务等待人工输入，没有阻断性故障。</p></div><button className="secondary-button">查看运行记录</button></div><div className="ops-list">{[['队列吞吐', '128 个任务', '正常'], ['平均处理时间', '4 分 12 秒', '正常'], ['等待用户', '3 个任务', '需关注'], ['失败重试', '1 个任务', '已恢复']].map(([name, value, status]) => <div key={name}><span>{name}</span><b>{value}</b><Status value={status}/><button><ArrowRight size={15}/></button></div>)}</div></section><aside className="ops-rail"><span className="section-label">最近活动</span>{activity.map(item => <div className="activity-item" key={item.title}><time>{item.time}</time><span><b>{item.title}</b><small>{item.meta}</small></span></div>)}</aside></div>
}

function SettingsSurface() {
  return <div className="settings-layout"><aside className="settings-index">{['基础配置', '流程与状态', '通知规则', '权限边界', '导出与命名'].map((v, i) => <button className={i === 0 ? 'active' : ''} key={v}>{v}</button>)}</aside><section className="settings-form"><div><h2>基础配置</h2><p>这些配置适用于当前组织和全部新建项目。</p></div>{[['默认项目时区', 'Asia/Shanghai'], ['默认货币', '人民币（CNY）'], ['数据保留期', '365 天'], ['自动保存', '开启']].map(([label, value], i) => <div className="setting-row" key={label}><div><b>{label}</b><small>{i === 3 ? '编辑内容后每 30 秒保存一个草稿版本。' : '用于新对象和报表的默认值。'}</small></div>{i === 3 ? <button className="switch active" aria-label="关闭自动保存"><span/></button> : <button className="select-field">{value}<ChevronDown size={14}/></button>}</div>)}</section></div>
}

export function ModulePage({ system, item }: { system: SystemDefinition; item: NavItem }) {
  const [activeView, setActiveView] = useState(item.views[0])
  const analysisSurface = system.key === 'insight' && item.id === 'content' ? <MaterialInsightSurface/> : system.key === 'delivery' && item.id === 'optimization' ? <DeliveryStrategySurface/> : <AnalysisSurface item={item}/>
  return <div className={`module-page page-frame layout-${item.layout}`}><PageHeader system={system} item={item} activeView={activeView} onViewChange={setActiveView}/><div className="page-surface">{item.layout === 'workspace' ? <WorkspaceSurface item={item}/> : item.layout === 'analysis' ? analysisSurface : item.layout === 'editor' ? <EditorSurface item={item}/> : item.layout === 'table' ? <TableSurface item={item}/> : item.layout === 'settings' ? <SettingsSurface/> : <OperationsSurface item={item}/>}</div><footer className="statusbar"><span>模型：cookies.ai-core-v1.0</span><span>策略源：4</span><span>更新时间：2026-07-22 16:30</span><span>预计成本：¥18.60</span><strong>状态：已同步</strong></footer></div>
}
