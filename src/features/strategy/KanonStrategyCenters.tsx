import {
  ArrowRight, BookOpen, CheckCircle2, CircleAlert, FileSearch, FileText,
  GitCompareArrows, Library, RefreshCw, Search, ShieldCheck, Target,
} from 'lucide-react'
import { useEffect, useMemo, useState, type ReactNode } from 'react'
import { useProject } from '../../context/ProjectContext'
import { strategyApi } from './api'
import type {
  BriefCenterSummary, DraftRevision, EvidenceReference, KnowledgeDocument,
  ResearchArtifact, ResearchRun, StrategyCenterSummary,
} from './types'

type OpenWorkspace = (workspaceId: string, view: string) => void

export function KanonBriefCenter({ activeView, onOpenWorkspace }: {
  activeView: string
  onOpenWorkspace: OpenWorkspace
}) {
  const { currentProject } = useProject()
  const [items, setItems] = useState<BriefCenterSummary[]>([])
  const [selectedId, setSelectedId] = useState('')
  const [state, setState] = useState<'loading' | 'ready' | 'error'>('loading')
  const [error, setError] = useState('')

  const load = async (signal?: AbortSignal) => {
    setState('loading')
    try {
      const result = await strategyApi.listProjectBriefs(currentProject.id, signal)
      setItems(result.items)
      setSelectedId(current => result.items.some(item => item.brief_id === current)
        ? current : result.items[0]?.brief_id ?? '')
      setError('')
      setState('ready')
    } catch (cause) {
      if (!(cause instanceof DOMException && cause.name === 'AbortError')) {
        setError(cause instanceof Error ? cause.message : '需求中心读取失败')
        setState('error')
      }
    }
  }

  useEffect(() => {
    const controller = new AbortController()
    void load(controller.signal)
    return () => controller.abort()
  }, [currentProject.id])

  const visible = useMemo(() => items.filter(item => {
    if (activeView === '待补充') return item.status === 'open' && !item.ready
    if (activeView === '待确认') return item.status === 'open' && item.ready
    if (activeView === '版本库') return item.latest_confirmed_version > 0
    if (activeView === '冲突队列') return item.conflict_count > 0
    return !item.discarded_at
  }), [activeView, items])
  const selected = visible.find(item => item.brief_id === selectedId) ?? visible[0]

  if (state === 'loading') return <CenterState icon={<RefreshCw className="spin"/>} title="正在读取真实 Brief"/>
  if (state === 'error') return <CenterState icon={<CircleAlert/>} title="需求中心暂时不可用" detail={error} action={() => void load()}/>
  if (!visible.length) return <CenterState icon={<FileText/>} title={`没有“${activeView}”Brief`} detail="这里仅展示当前 Project 的 Strategy Brief，不混入创意或投放对象。"/>

  return <div className="kanon-center-layout">
    <aside className="kanon-center-list">
      <header><div><span>BRIEF CENTER</span><h3>需求中心</h3></div><strong>{visible.length}</strong></header>
      {visible.map(item => <button className={item.brief_id === selected?.brief_id ? 'active' : ''} key={item.brief_id} onClick={() => setSelectedId(item.brief_id)}>
        <FileText size={17}/><span><b>{item.name}</b><small>{item.objective || '等待补充业务目标'}</small></span><em>{briefStatus(item)}</em>
      </button>)}
    </aside>
    <main className="kanon-center-detail">
      {selected ? <>
        <header><div><span>Brief · v{selected.version}</span><h2>{selected.name}</h2><p>{selected.objective}</p></div><em>{briefStatus(selected)}</em></header>
        <section className="kanon-center-metrics">
          <Metric label="完整度" value={selected.ready ? '可确认' : `${selected.blocker_count} 个阻断`} good={selected.ready}/>
          <Metric label="字段冲突" value={`${selected.conflict_count} 个`} good={selected.conflict_count === 0}/>
          <Metric label="确认版本" value={selected.latest_confirmed_version ? `v${selected.latest_confirmed_version}` : '尚未确认'} good={selected.latest_confirmed_version > 0}/>
        </section>
        <div className="kanon-center-note"><ShieldCheck size={18}/><div><b>同一条 Strategy 主链</b><p>详情编辑、字段确认和版本冻结复用所属工作区，不创建第二套 Brief 状态。</p></div></div>
        <footer><small>更新于 {formatTime(selected.updated_at)}</small><button className="primary-button" onClick={() => onOpenWorkspace(selected.workspace_id, 'Brief')}>进入 Brief 工作区<ArrowRight size={15}/></button></footer>
      </> : null}
    </main>
  </div>
}

export function KanonStrategyLibrary({ activeView, onOpenWorkspace }: {
  activeView: string
  onOpenWorkspace: OpenWorkspace
}) {
  const { currentProject } = useProject()
  const [items, setItems] = useState<StrategyCenterSummary[]>([])
  const [selectedId, setSelectedId] = useState('')
  const [revisions, setRevisions] = useState<DraftRevision[]>([])
  const [state, setState] = useState<'loading' | 'ready' | 'error'>('loading')
  const [error, setError] = useState('')

  useEffect(() => {
    const controller = new AbortController()
    setState('loading')
    void strategyApi.listProjectStrategies(currentProject.id, controller.signal).then(result => {
      setItems(result.items)
      setSelectedId(current => result.items.some(item => item.strategy_id === current)
        ? current : result.items[0]?.strategy_id ?? '')
      setState('ready')
      setError('')
    }).catch(cause => {
      if (!(cause instanceof DOMException && cause.name === 'AbortError')) {
        setError(cause instanceof Error ? cause.message : '策略资产库读取失败')
        setState('error')
      }
    })
    return () => controller.abort()
  }, [currentProject.id])

  const visible = useMemo(() => items.filter(item => {
    if (activeView === '方案对比') return item.current_revision > 1
    if (activeView === '版本库') return item.current_revision > 0
    if (activeView === '实验方案') return item.current_revision > 0
    return true
  }), [activeView, items])
  const selected = visible.find(item => item.strategy_id === selectedId) ?? visible[0]

  useEffect(() => {
    if (!selected?.strategy_id || selected.current_revision < 1) {
      setRevisions([])
      return
    }
    const controller = new AbortController()
    void strategyApi.listStrategyRevisions(selected.strategy_id, controller.signal)
      .then(result => setRevisions(result.items))
      .catch(() => setRevisions([]))
    return () => controller.abort()
  }, [selected?.strategy_id, selected?.current_revision])

  if (state === 'loading') return <CenterState icon={<RefreshCw className="spin"/>} title="正在读取策略资产"/>
  if (state === 'error') return <CenterState icon={<CircleAlert/>} title="策略资产库暂时不可用" detail={error}/>
  if (!visible.length) return <CenterState icon={<Library/>} title={`没有“${activeView}”策略`} detail="策略需先在工作区基于已冻结 Brief 生成。"/>

  const latest = [...revisions].sort((left, right) => right.revision - left.revision)[0]
  const previous = [...revisions].sort((left, right) => right.revision - left.revision)[1]
  return <div className="kanon-center-layout">
    <aside className="kanon-center-list">
      <header><div><span>STRATEGY LIBRARY</span><h3>策略资产库</h3></div><strong>{visible.length}</strong></header>
      {visible.map(item => <button className={item.strategy_id === selected?.strategy_id ? 'active' : ''} key={item.strategy_id} onClick={() => setSelectedId(item.strategy_id)}>
        <Target size={17}/><span><b>{item.name}</b><small>Brief v{item.brief_version} · Strategy r{item.current_revision}</small></span><em>{strategyStatus(item)}</em>
      </button>)}
    </aside>
    <main className="kanon-center-detail">
      {selected ? <>
        <header><div><span>Strategy · r{selected.current_revision}</span><h2>{selected.name}</h2><p>{selected.objective}</p></div><em>{strategyStatus(selected)}</em></header>
        <section className="kanon-center-metrics">
          <Metric label="策略修订" value={`${selected.current_revision} 个`} good={selected.current_revision > 0}/>
          <Metric label="评审状态" value={selected.review_status || '未提交'} good={selected.review_status === 'approved'}/>
          <Metric label="发布版本" value={selected.package_version ? `v${selected.package_version}` : '未发布'} good={selected.package_version > 0}/>
        </section>
        {activeView === '方案对比' ? <div className="kanon-center-compare">
          <GitCompareArrows size={18}/><div><b>r{latest?.revision ?? '—'} 对比 r{previous?.revision ?? '—'}</b><p>{latest?.changed_sections.length ? `最新修订涉及：${latest.changed_sections.join('、')}` : '暂无可比较的修订差异。'}</p></div>
        </div> : activeView === '实验方案' ? <div className="kanon-center-note">
          <Target size={18}/><div><b>{latest?.document.experiment_matrix.length ?? 0} 个实验假设</b><p>{latest?.document.experiment_matrix.map(item => item.hypothesis).join('；') || '当前策略尚未形成实验矩阵。'}</p></div>
        </div> : <div className="kanon-center-note"><CheckCircle2 size={18}/><div><b>资产库只读治理</b><p>策略修改仍回到所属工作区；这里负责版本、评审、发布与交接状态。</p></div></div>}
        <footer><small>更新于 {formatTime(selected.updated_at)}</small><button className="primary-button" onClick={() => onOpenWorkspace(selected.workspace_id, '策略')}>进入策略工作区<ArrowRight size={15}/></button></footer>
      </> : null}
    </main>
  </div>
}

export function KanonResearchEvidenceCenter({ activeView }: { activeView: string }) {
  const { currentProject } = useProject()
  const [documents, setDocuments] = useState<KnowledgeDocument[]>([])
  const [runs, setRuns] = useState<ResearchRun[]>([])
  const [artifacts, setArtifacts] = useState<ResearchArtifact[]>([])
  const [references, setReferences] = useState<EvidenceReference[]>([])
  const [query, setQuery] = useState('')
  const [selectedId, setSelectedId] = useState('')
  const [state, setState] = useState<'loading' | 'ready' | 'error'>('loading')
  const [error, setError] = useState('')

  useEffect(() => {
    const controller = new AbortController()
    setState('loading')
    void Promise.all([
      strategyApi.listKnowledgeDocuments(currentProject.id, controller.signal),
      strategyApi.listResearchRuns(currentProject.id, controller.signal),
      strategyApi.listResearchArtifacts(currentProject.id, 'all', controller.signal),
      strategyApi.listEvidenceReferences(currentProject.id, '', controller.signal),
    ]).then(([documentResult, runResult, artifactResult, referenceResult]) => {
      setDocuments(documentResult.items)
      setRuns(runResult.items)
      setArtifacts(artifactResult.items)
      setReferences(referenceResult.items)
      setSelectedId(current => artifactResult.items.some(item => item.id === current)
        ? current : artifactResult.items[0]?.id ?? '')
      setState('ready')
      setError('')
    }).catch(cause => {
      if (!(cause instanceof DOMException && cause.name === 'AbortError')) {
        setError(cause instanceof Error ? cause.message : '研究与证据读取失败')
        setState('error')
      }
    })
    return () => controller.abort()
  }, [currentProject.id])

  const category = ({ 受众: 'audience', 竞品: 'competitor', 行业: 'industry' } as const)[activeView as '受众' | '竞品' | '行业']
  const filtered = artifacts.filter(item => (!category || item.category === category)
    && `${item.title} ${item.content}`.toLowerCase().includes(query.trim().toLowerCase()))
  const selected = filtered.find(item => item.id === selectedId) ?? filtered[0]
  const usage = selected ? references.filter(item => item.evidence_id === selected.id) : []

  if (state === 'loading') return <CenterState icon={<RefreshCw className="spin"/>} title="正在读取研究与证据"/>
  if (state === 'error') return <CenterState icon={<CircleAlert/>} title="研究与证据暂时不可用" detail={error}/>
  if (activeView === '资料来源') return <SourceList documents={documents}/>
  if (activeView === '研究任务') return <RunList runs={runs}/>

  return <div className="kanon-evidence-center">
    <section>
      <div className="kanon-evidence-toolbar"><div><span className="section-label">RESEARCH EVIDENCE</span><h2>{activeView}</h2></div><label><Search size={15}/><input aria-label="搜索研究证据" value={query} onChange={event => setQuery(event.target.value)} placeholder="搜索标题或结论"/></label></div>
      <div className="kanon-evidence-table">
        {filtered.map(item => <button className={item.id === selected?.id ? 'active' : ''} key={item.id} onClick={() => setSelectedId(item.id)}>
          <BookOpen size={17}/><span><b>{item.title}</b><small>{researchCategoryLabel(item.category)} · {item.source_type}</small></span><em>{references.filter(reference => reference.evidence_id === item.id).length} 次引用</em>
        </button>)}
        {!filtered.length ? <div className="panel-empty">当前分类没有真实 ResearchArtifact。请在策略工作区发起对应分类的研究任务。</div> : null}
      </div>
    </section>
    <aside>
      {selected ? <>
        <span className="section-label">证据详情</span><h3>{selected.title}</h3><p>{selected.content}</p>
        <div className="kanon-evidence-meta"><span>内容哈希</span><b>{selected.content_hash.slice(0, 18)}</b><span>来源引用</span><b>{selected.citations.length} 条</b></div>
        <div className="surface-toolbar"><h3>被引用记录</h3><strong>{usage.length}</strong></div>
        {usage.map(item => <article key={`${item.target_type}-${item.target_id}-${item.target_version}`}><FileSearch size={15}/><span><b>{targetLabel(item.target_type)} v{item.target_version}</b><small>{item.target_id}</small></span></article>)}
        {!usage.length ? <div className="panel-empty">这条证据尚未被 Brief 或策略引用。</div> : null}
      </> : <div className="panel-empty">选择一条研究证据查看来源和使用记录。</div>}
    </aside>
  </div>
}

function SourceList({ documents }: { documents: KnowledgeDocument[] }) {
  return <div className="kanon-source-list"><div className="kanon-strategy-heading"><div><span className="section-label">KNOWLEDGE SOURCES</span><h2>资料来源</h2><p>项目资料和投前洞察统一进入 Knowledge Gateway。</p></div><span className="source-chip">{documents.length} 份</span></div>{documents.map(document => <article key={document.id}><FileText size={18}/><span><b>{document.title || document.filename}</b><small>{document.source_type || document.mime_type} · {formatTime(document.created_at)}</small></span><em>{document.status}</em></article>)}{!documents.length ? <div className="panel-empty">当前 Project 尚未导入资料。</div> : null}</div>
}

function RunList({ runs }: { runs: ResearchRun[] }) {
  return <div className="kanon-source-list"><div className="kanon-strategy-heading"><div><span className="section-label">RESEARCH RUNS</span><h2>研究任务</h2><p>外部研究由后端任务队列执行，并保留披露字段和产物。</p></div><span className="source-chip">{runs.length} 个</span></div>{runs.map(run => <article key={run.id}><RefreshCw className={run.status === 'running' ? 'spin' : ''} size={18}/><span><b>{run.query}</b><small>{researchCategoryLabel(run.category)} · {run.mode} · {run.artifacts.length} 个产物</small></span><em>{run.status}</em></article>)}{!runs.length ? <div className="panel-empty">当前 Project 尚未执行研究任务。</div> : null}</div>
}

function CenterState({ icon, title, detail, action }: { icon: ReactNode; title: string; detail?: string; action?: () => void }) {
  return <div className="kanon-task-state" role="status">{icon}<b>{title}</b>{detail ? <p>{detail}</p> : null}{action ? <button className="secondary-button" onClick={action}><RefreshCw size={14}/>重新加载</button> : null}</div>
}

function Metric({ label, value, good }: { label: string; value: string; good: boolean }) {
  return <article><small>{label}</small><b>{value}</b><span className={good ? 'positive' : ''}>{good ? '状态正常' : '需要处理'}</span></article>
}

function briefStatus(item: BriefCenterSummary) {
  if (item.discarded_at) return '已废弃'
  if (item.status === 'confirmed') return '已确认'
  if (item.conflict_count) return '有冲突'
  return item.ready ? '待确认' : '待补充'
}

function strategyStatus(item: StrategyCenterSummary) {
  if (item.archived_at) return '已归档'
  if (item.package_version) return '已发布'
  if (item.review_status === 'open') return '待评审'
  if (item.status === 'failed') return '生成失败'
  return item.current_revision ? '策略草稿' : '生成中'
}

function researchCategoryLabel(value: ResearchArtifact['category']) {
  return ({ general: '综合', audience: '受众', competitor: '竞品', industry: '行业' })[value]
}

function targetLabel(value: EvidenceReference['target_type']) {
  return ({ brief_draft: 'Brief 草稿', brief_version: 'Brief 版本', strategy_revision: '策略修订' })[value]
}

function formatTime(value: string) {
  return new Date(value).toLocaleString('zh-CN', { hour12: false })
}
