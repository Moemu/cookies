import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  ArrowRight, BarChart3, BookOpenCheck, Check, CircleAlert, CircleCheck,
  Database, FileInput, Filter, Layers3, Lightbulb, Link2, Play, RefreshCw,
  Search, Sparkles, Target, TrendingUp,
} from 'lucide-react'
import { useProject } from '../context/ProjectContext'
import { api, type ApiArtifact, type ApiKnowledgeDocument, type ApiKnowledgeSearchResult, type ApiProjectMediaAsset } from '../data/api'
import type { DataState, SystemKey } from '../types'
import { StateBoundary } from './StateBoundary'

type OpenProject = (id: string, system?: SystemKey, navId?: string, objectId?: string, view?: string) => void

type PerformanceAd = {
  id: string
  name: string
  platform: string
  format: string
  spend: number
  impressions: number
  ctr: number
  cpa: number
  signal: string
  occurredAt: string
}

function performanceAd(record: import('../data/api').ApiOperationalRecord): PerformanceAd | null {
  if (record.kind !== 'performance_ad') return null
  const { platform, format, spend, impressions, ctr, cpa } = record.fields
  if (
    typeof platform !== 'string' || typeof format !== 'string' || typeof spend !== 'number'
    || typeof impressions !== 'number' || typeof ctr !== 'number' || typeof cpa !== 'number'
  ) return null
  return { id: record.id, name: record.title, platform, format, spend, impressions, ctr, cpa, signal: record.status, occurredAt: record.occurredAt }
}

function inWindow(occurredAt: string, window: string, latestTime: number): boolean {
  if (window === '本季度') return true
  const days = window === '近 7 天' ? 7 : 30
  const time = new Date(occurredAt).valueOf()
  return !Number.isNaN(time) && time >= latestTime - days * 24 * 60 * 60 * 1000
}

function trendPoints(fields: Record<string, unknown>): number[] {
  if (typeof fields.points !== 'string') return []
  return fields.points
    .split(',')
    .map(value => Number(value.trim()))
    .filter(value => Number.isFinite(value))
}

function textOperationField(fields: Record<string, unknown>, key: string, fallback: string): string {
  const value = fields[key]
  return typeof value === 'string' ? value : fallback
}

export function PostLaunchAnalysisPage({ state, onOpenProject }: { state: DataState; onOpenProject: OpenProject }) {
  const { currentProject, reloadProjects } = useProject()
  const [platform, setPlatform] = useState('全部平台')
  const [window, setWindow] = useState('近 30 天')
  const [query, setQuery] = useState('')
  const [selectedId, setSelectedId] = useState('')
  const [notice, setNotice] = useState('')
  const ads = useMemo(() => currentProject.operations.map(performanceAd).filter((row): row is PerformanceAd => Boolean(row)), [currentProject.operations])
  const platforms = useMemo(() => [...new Set(ads.map(row => row.platform))], [ads])
  const latestTime = useMemo(() => Math.max(...ads.map(row => new Date(row.occurredAt).valueOf())), [ads])
  const filtered = useMemo(() => ads.filter(row =>
    (platform === '全部平台' || row.platform === platform)
    && inWindow(row.occurredAt, window, latestTime)
    && `${row.id} ${row.name} ${row.format}`.toLowerCase().includes(query.trim().toLowerCase()),
  ), [ads, latestTime, platform, query, window])
  const selected = filtered.find(row => row.id === selectedId) ?? filtered[0]
  const spend = filtered.reduce((sum, row) => sum + row.spend, 0)
  const impressions = filtered.reduce((sum, row) => sum + row.impressions, 0)
  const averageCtr = filtered.length ? filtered.reduce((sum, row) => sum + row.ctr, 0) / filtered.length : 0
  const averageCpa = filtered.length ? filtered.reduce((sum, row) => sum + row.cpa, 0) / filtered.length : 0
  const [reportBusy, setReportBusy] = useState(false)
  const topPerformer = [...filtered].sort((left, right) => right.ctr - left.ctr)[0]
  const lowestCpa = [...filtered].sort((left, right) => left.cpa - right.cpa)[0]
  const metrics = useMemo(() => currentProject.operations.filter(record => record.kind === 'metric'), [currentProject.operations])
  const reasons = useMemo(() => currentProject.operations.filter(record => record.kind === 'evidence'), [currentProject.operations])
  const actions = useMemo(() => currentProject.operations.filter(record => record.kind === 'delivery_action'), [currentProject.operations])
  const metric = metrics[0]
  const recommendation = actions[0]
  const points = useMemo(() => metric ? trendPoints(metric.fields) : [], [metric])
  const maxPoint = Math.max(...points, 1)
  const resetFilters = () => {
    setPlatform('全部平台')
    setWindow('近 30 天')
    setQuery('')
    setSelectedId('')
  }
  const refreshOperations = async () => {
    try {
      await reloadProjects()
      setNotice('已从服务端刷新当前 Project 的运营记录。')
    } catch (cause) {
      setNotice(cause instanceof Error ? cause.message : '刷新运营记录失败，请重试。')
    }
  }

  const createReport = async () => {
    if (!selected || !topPerformer || !lowestCpa) return
    setReportBusy(true)
    try {
      const report = await api.createArtifact({
        projectId: currentProject.id,
        kind: 'document',
        status: 'ready',
        content: `[insight] 投后分析报告 | 窗口：${window} | 平台：${platform} | 消耗：¥${spend.toLocaleString('zh-CN')} | 曝光：${impressions.toLocaleString('zh-CN')} | 平均 CTR：${averageCtr.toFixed(2)}% | 平均 CPL：¥${averageCpa.toFixed(1)} | 关键素材：${selected.name} | 趋势：${metric ? textOperationField(metric.fields, 'summary', '无服务端趋势记录') : '无服务端趋势记录'} | 建议：${recommendation?.title ?? '无服务端建议动作'}`,
      })
      await reloadProjects()
      setNotice(`投后分析报告 ${report.id.slice(0, 8)} 已确认，可进入经验库沉淀。`)
    } catch (cause) {
      setNotice(cause instanceof Error ? cause.message : '生成投后分析报告失败，请重试。')
    } finally {
      setReportBusy(false)
    }
  }

  return <StateBoundary
    state={state}
    contextLabel="素材洞察 / 投后分析"
    emptyTitle="当前 Project 暂无投后运营记录"
    emptyDetail="接入或导入广告平台运营记录后，这里会展示表现变化、原因和建议动作；不会用固定前端结论替代真实数据。"
    errorDetail="投后运营记录读取失败。请确认服务端和数据源连接后重新拉取，已保存报告不会被覆盖。"
    createLabel="打开数据接入向导"
    onRetry={() => void refreshOperations()}
    onCreate={() => setNotice('数据源接入向导已打开')}
  >
    <div className="ad-insight-workspace">
      <section className="ad-insight-main">
        <div className="core-flow-toolbar">
          <div><span className="section-label">POST-LAUNCH ANALYSIS</span><h2>投后结论</h2><p>先确认表现变化、可解释的驱动因素与下一步动作，再进入指标口径和广告明细验证。</p></div>
          <div className="core-flow-actions">
            <label>平台<select aria-label="广告平台" value={platform} onChange={event => setPlatform(event.target.value)}><option>全部平台</option>{platforms.map(item => <option key={item}>{item}</option>)}</select></label>
            <label>窗口<select aria-label="数据窗口" value={window} onChange={event => setWindow(event.target.value)}><option>近 7 天</option><option>近 30 天</option><option>本季度</option></select></label>
            <button className="secondary-button" onClick={() => void refreshOperations()}><RefreshCw size={15}/>刷新数据</button>
          </div>
        </div>

        {filtered.length ? <><div className="postlaunch-brief">
          <section className="postlaunch-conclusion">
            <span className="section-label">发生什么</span>
            <h3>{metric ? textOperationField(metric.fields, 'summary', '当前筛选范围内已汇总服务端广告表现。') : '当前筛选范围内已汇总服务端广告表现。'}</h3>
            <p>在 {window}、{platform} 范围内，{topPerformer?.name} 以 {topPerformer?.ctr}% CTR 领跑；{lowestCpa?.name} 的 CPL 为 ¥{lowestCpa?.cpa}，当前平均为 ¥{averageCpa.toFixed(1)}。</p>
            <div className="postlaunch-signal-line">
              <span><b>{averageCtr.toFixed(2)}%</b><small>平均 CTR</small></span>
              <span><b>¥{averageCpa.toFixed(1)}</b><small>平均 CPL</small></span>
              <span><b>{filtered.length}</b><small>纳入分析的广告</small></span>
            </div>
          </section>
          <section className="postlaunch-trend" aria-label="核心趋势">
            <div><span className="section-label">核心趋势</span><b>{metric?.title ?? '暂无服务端趋势记录'}</b><small>{String(metric?.fields.comparison ?? '请接入运营指标后重试')}</small></div>
            {points.length ? <>
              <div className="postlaunch-sparkline" aria-label={`服务端趋势共 ${points.length} 个点位`}>
                {points.map((point, index) => <i key={`${metric?.id}-${index}`} style={{ height: `${Math.max((point / maxPoint) * 100, 4)}%`, opacity: 0.45 + ((index + 1) / points.length) * 0.55 }}/>)}
              </div>
              <div className="postlaunch-trend-labels"><span>点位 1</span><span>点位 {points.length}</span></div>
            </> : <p className="postlaunch-trend-empty">当前趋势记录未提供可视化点位。</p>}
          </section>
        </div>

        <div className="postlaunch-decision-grid">
          <section className="postlaunch-reasons">
            <span className="section-label">为什么发生</span>
            <h3>可复核的表现驱动</h3>
            <div className="postlaunch-reason-list">
              {reasons.map((reason, index) => <article key={reason.id}><span>{String(index + 1).padStart(2, '0')}</span><div><b>{reason.title}</b><p>{String(reason.fields.source ?? '服务端运营记录')} · 置信度：{String(reason.fields.confidence ?? '未标注')}</p></div></article>)}
              {!reasons.length ? <div className="panel-empty">当前 Project 没有可解释原因记录。</div> : null}
            </div>
          </section>
          <section className="postlaunch-action">
            <span className="section-label">推荐动作</span>
            <h3>下一轮优先验证</h3>
            <b>{recommendation?.title ?? '暂无服务端建议动作'}</b>
            <p>{String(recommendation?.fields.detail ?? '接入并保存建议动作后，可在此执行。')}</p>
            <div><span>预计影响</span><strong>{String(recommendation?.fields.impact ?? '待评估')}</strong></div>
            <button className="primary-button full" onClick={() => onOpenProject(currentProject.id, 'delivery', 'optimization')}><TrendingUp size={15}/>进入优化中心执行</button>
          </section>
        </div>

        <details className="postlaunch-drilldown">
          <summary><span><b>广告明细与指标口径</b><small>用于验证结论、筛选广告并查看单条表现</small></span><span>{filtered.length} 条广告 · {window}</span></summary>
          <div className="ad-data-panel">
            <div className="ad-data-heading"><div className="search-field"><Search size={15}/><input aria-label="搜索广告数据" value={query} onChange={event => setQuery(event.target.value)} placeholder="搜索广告、素材或编号"/></div><span>消耗、曝光按当前平台与窗口汇总；CPL 为每条高质量销售线索成本</span></div>
            <div className="ad-data-table">
              <div className="ad-data-row header"><span>广告</span><span>平台</span><span>消耗</span><span>曝光</span><span>CTR</span><span>CPL</span><span>信号</span></div>
              {filtered.map(row => <button key={row.id} className={selectedId === row.id ? 'ad-data-row active' : 'ad-data-row'} onClick={() => setSelectedId(row.id)}>
                <span><b>{row.name}</b><small>{row.id} · {row.format}</small></span><span>{row.platform}</span><span>¥{row.spend.toLocaleString('zh-CN')}</span><span>{row.impressions.toLocaleString('zh-CN')}</span><span>{row.ctr}%</span><span>¥{row.cpa}</span><span><i/>{row.signal}</span>
              </button>)}
              {!filtered.length ? <div className="panel-empty">没有匹配的广告数据</div> : null}
            </div>
          </div>
        </details></> : <section className="panel-empty"><b>没有可用的投后运营数据</b><p>当前 Project 在 {platform}、{window} 与搜索条件下没有服务端广告表现记录。可清除筛选或重新拉取运营记录。</p><button className="secondary-button" onClick={resetFilters}>清除筛选</button><button className="primary-button" onClick={() => void refreshOperations()}><RefreshCw size={15}/>重新拉取</button></section>}
      </section>
      <aside className="ad-insight-detail">{selected ? <>
        <span className="section-label">明细下钻</span><h3>{selected.name}</h3><p>{selected.platform} · {selected.format} · 当前信号：{selected.signal}</p>
        <div className="mini-trend"><BarChart3 size={18}/><div><b>{selected.ctr}% CTR</b><small>CPL ¥{selected.cpa} · 来自服务端运营记录</small></div></div>
        {reasons.slice(0, 3).map(item => <span className="analysis-check" key={item.id}><CircleCheck size={15}/>{item.title}</span>)}
        <button className="secondary-button full" disabled={reportBusy} onClick={() => void createReport()}><BarChart3 size={15}/>{reportBusy ? '正在生成报告…' : '生成项目复盘报告'}</button>
        <button className="primary-button full" onClick={() => onOpenProject(currentProject.id, 'insight', 'knowledge')}>进入经验沉淀<ArrowRight size={15}/></button></> : <div className="panel-empty">选择或恢复服务端广告记录后可查看复盘输入。</div>}
        {notice ? <div className="inline-notice" role="status">{notice}</div> : null}
      </aside>
    </div>
  </StateBoundary>
}

export function AssetExperiencePage({ state, mode }: { state: DataState; mode: 'assets' | 'knowledge' }) {
  const { currentProject } = useProject()
  const [query, setQuery] = useState('')
  const [type, setType] = useState('全部')
  const [selectedId, setSelectedId] = useState('')
  const [assets, setAssets] = useState<ApiProjectMediaAsset[]>([])
  const [knowledgeDocuments, setKnowledgeDocuments] = useState<ApiKnowledgeDocument[]>([])
  const [knowledgeResults, setKnowledgeResults] = useState<ApiKnowledgeSearchResult[]>([])
  const [assetState, setAssetState] = useState<'loading' | 'ready' | 'error'>('loading')
  const loadArtifacts = useCallback(async () => {
    if (!currentProject.id) return
    setAssetState('loading')
    try {
      if (mode === 'knowledge') {
        const next = await api.listKnowledgeDocuments(currentProject.id, 50)
        setKnowledgeDocuments(next.items)
        setKnowledgeResults([])
      } else {
        setAssets(await api.listProjectMediaAssets(currentProject.id))
      }
      setAssetState('ready')
    } catch {
      setAssets([])
      setKnowledgeDocuments([])
      setKnowledgeResults([])
      setAssetState('error')
    }
  }, [currentProject.id, mode])
  useEffect(() => { void loadArtifacts() }, [loadArtifacts])
  const filtered = useMemo(() => assets.filter(asset =>
    (type === '全部' || assetType(asset) === type)
    && `${asset.id} ${asset.mimeType} ${asset.kind}`.toLowerCase().includes(query.trim().toLowerCase()),
  ), [assets, query, type])
  const filteredKnowledgeDocuments = useMemo(() => knowledgeDocuments.filter(document =>
    `${document.id} ${document.title} ${document.source_uri}`.toLowerCase().includes(query.trim().toLowerCase()),
  ), [knowledgeDocuments, query])
  useEffect(() => {
    if (mode !== 'knowledge' || !query.trim()) {
      setKnowledgeResults([])
      return
    }
    let active = true
    const timer = window.setTimeout(() => {
      void api.searchKnowledge(currentProject.id, query.trim(), 10).then(response => {
        if (active) setKnowledgeResults(response.items)
      }).catch(() => {
        if (active) setKnowledgeResults([])
      })
    }, 250)
    return () => {
      active = false
      window.clearTimeout(timer)
    }
  }, [currentProject.id, mode, query])
  useEffect(() => {
    setSelectedId(current => filtered.some(asset => asset.id === current) ? current : filtered[0]?.id ?? '')
  }, [filtered])
  const selected = filtered.find(asset => asset.id === selectedId)
  const importProjectKnowledge = async () => {
    setAssetState('loading')
    try {
      await api.importKnowledgeDocument(currentProject.id, {
        title: `${currentProject.name} 策略复盘`,
        source_uri: 'docs/策略/README.md',
        source_type: 'docs',
        text: `${currentProject.name}\n目标：${currentProject.goal}\n策略：首屏 Hook、商品露出、证据闭环与 citation 必须可追溯。`,
      })
      await loadArtifacts()
    } catch {
      setAssetState('error')
    }
  }

  if (mode === 'knowledge') {
    const highlightedResult = knowledgeResults[0]
    return <StateBoundary
      state={state}
      contextLabel="素材洞察 / 经验沉淀"
      emptyTitle="当前 Project 暂无知识文档"
      emptyDetail="导入项目 docs 或复盘报告后，系统会提供可追溯的 citation 检索结果。"
      errorDetail="知识库读取失败，请检查平台 API 后重新加载。"
      createLabel="导入 Project Docs"
      onRetry={() => { void loadArtifacts() }}
      onCreate={() => { void importProjectKnowledge() }}
    >
      <div className="asset-experience-workspace">
        <section className="asset-library-panel">
          <div className="core-flow-toolbar">
            <div><span className="section-label">KNOWLEDGE / RAG</span><h2>项目知识策略库</h2><p>导入项目 docs 文本，使用确定性关键词检索，并在输出中保留 citation。</p></div>
            <button className="primary-button" onClick={() => void importProjectKnowledge()}><FileInput size={15}/>导入 Project Docs</button>
          </div>
          <div className="asset-filterbar">
            <div className="search-field"><Search size={15}/><input aria-label="搜索知识库" value={query} onChange={event => setQuery(event.target.value)} placeholder="搜索 Hook、商品露出、质检或 citation"/></div>
            <span>{query.trim() ? knowledgeResults.length : filteredKnowledgeDocuments.length} 项 · 当前 Project</span>
          </div>
          <div className="asset-card-grid">
            {assetState === 'loading' ? <div className="panel-empty">正在读取当前 Project 的 Knowledge/RAG 策略库…</div> : null}
            {assetState === 'error' ? <div className="panel-empty">知识库读取失败，请检查平台 API 或重试。</div> : null}
            {assetState === 'ready' && !query.trim() && !filteredKnowledgeDocuments.length ? <div className="panel-empty">当前 Project 暂无知识文档，可先导入项目 docs 文本。</div> : null}
            {assetState === 'ready' && query.trim() && !knowledgeResults.length ? <div className="panel-empty">没有命中的知识片段；换一个关键词试试。</div> : null}
            {!query.trim() ? filteredKnowledgeDocuments.map(document => <button key={document.id} className="asset-analysis-card" onClick={() => setQuery(document.title)}>
              <span className="asset-card-preview"><BookOpenCheck size={18}/><small>{document.source_type}</small></span>
              <span><small>{document.id.slice(0, 8)} · {document.chunk_count} chunks</small><b>{document.title}</b><em>{document.source_uri || 'project docs'}</em></span>
            </button>) : knowledgeResults.map(result => <button key={result.chunk.id} className={highlightedResult?.chunk.id === result.chunk.id ? 'asset-analysis-card active' : 'asset-analysis-card'}>
              <span className="asset-card-preview"><Search size={18}/><small>score {result.score}</small></span>
              <span><small>{result.citations[0]?.title ?? result.chunk.document_id} · L{result.chunk.start_line}-L{result.chunk.end_line}</small><b>{result.citations[0]?.snippet || result.chunk.text}</b><em>{result.citations[0]?.source_uri || result.chunk.source_uri}</em></span>
            </button>)}
          </div>
        </section>
        <aside className="asset-analysis-detail">
          {highlightedResult ? <><span className="section-label">Citation</span><h3>{highlightedResult.citations[0]?.title ?? '知识片段'}</h3><p>{highlightedResult.chunk.section} · score {highlightedResult.score}</p>
            <div className="feature-stack"><span>引用来源</span>{highlightedResult.citations.map(citation => <b key={citation.chunk_id}>{citation.source_uri || 'project docs'} · L{citation.start_line}-L{citation.end_line}</b>)}</div>
            <div className="experience-card"><BookOpenCheck size={18}/><span><small>可追溯片段</small><b>{highlightedResult.citations[0]?.snippet || highlightedResult.chunk.text}</b></span></div>
          </> : <div className="panel-empty">输入关键词后查看命中的知识片段、来源文档和行号 citation；Planner 或 Agent 可携带这些引用输出。</div>}
        </aside>
      </div>
    </StateBoundary>
  }

  return <StateBoundary
    state={state}
    contextLabel="素材洞察 / 素材管理"
    emptyTitle="当前 Project 暂无已持久化素材"
    emptyDetail="完成创意生成或上传素材后，这里会展示图片、视频、来源任务和多模态特征。"
    errorDetail="素材读取失败，请确认 API 服务和资源接口可用后重新加载。"
    onRetry={() => { void loadArtifacts() }}
  >
    <div className="asset-experience-workspace">
      <section className="asset-library-panel">
        <div className="core-flow-toolbar">
          <div><span className="section-label">{mode === 'assets' ? 'ASSET MANAGEMENT' : 'EXPERIENCE LIBRARY'}</span><h2>{mode === 'assets' ? '当前 Project 素材' : '当前 Project 经验'}</h2><p>仅展示服务端已持久化、归属当前 Project 的产物及其可追溯元数据。</p></div>
        </div>
        <div className="asset-filterbar">
          <div className="search-field"><Search size={15}/><input aria-label="搜索素材经验" value={query} onChange={event => setQuery(event.target.value)} placeholder="搜索素材、特征或经验"/></div>
          <label><Filter size={14}/><select aria-label="素材类型" value={type} onChange={event => setType(event.target.value)}><option>全部</option><option>视频</option><option>图文</option><option>文档</option></select></label>
          <span>{filtered.length} 项 · 当前 Project</span>
        </div>
        <div className="asset-card-grid">
          {assetState === 'loading' ? <div className="panel-empty">正在读取当前 Project 的服务端产物…</div> : null}
          {assetState === 'error' ? <div className="panel-empty">素材读取失败，请重试。</div> : null}
          {assetState === 'ready' && !filtered.length ? <div className="panel-empty">{mode === 'assets' ? '当前 Project 暂无已持久化的图片或视频资产。' : '当前 Project 暂无已生成并持久化的经验结论。'}</div> : null}
          {filtered.map(asset => <button key={`${asset.id}-${asset.version}`} className={selectedId === asset.id ? 'asset-analysis-card active' : 'asset-analysis-card'} onClick={() => setSelectedId(asset.id)}>
            <span className="asset-card-preview">{asset.kind === 'video' ? <Play size={18} fill="currentColor"/> : <Layers3 size={18}/>}<small>{assetType(asset)}</small></span>
            <span><small>{asset.id.slice(0, 12)} · v{asset.version} · 服务端存档</small><b>{assetTitle(asset)}</b><em>{assetMetadata(asset)}</em></span>
          </button>)}
        </div>
      </section>
      <aside className="asset-analysis-detail">
        {selected ? <><span className="section-label">服务端素材详情</span><h3>{assetTitle(selected)}</h3><p>{assetType(selected)} · 已持久化 · v{selected.version}</p>
          <div className="feature-stack"><span>可追溯元数据</span><b>Asset {selected.id}</b><b>{selected.mimeType}</b><b>{assetMetadata(selected)}</b><b>入库时间 {new Date(selected.createdAt).toLocaleString('zh-CN')}</b></div>
          {selected.kind === 'video' ? <video className="project-asset-preview" controls preload="metadata" src={selected.contentUrl}>当前浏览器不支持视频预览。</video> : <a className="secondary-button full" href={selected.contentUrl} target="_blank" rel="noreferrer"><FileInput size={15}/>打开导入 Brief / 文档</a>}
        </> : <div className="panel-empty">选择当前 Project 的服务端产物后查看详情；系统不会以固定 CTR 或 AI 结论替代真实结果。</div>}
      </aside>
    </div>
  </StateBoundary>
}

function assetType(artifact: ApiProjectMediaAsset): string {
  return artifact.kind === 'image' ? '图文' : artifact.kind === 'video' ? '视频' : '文档'
}

function assetTitle(artifact: ApiProjectMediaAsset): string {
  if (artifact.mimeType === 'application/pdf') return 'Guerlain KOL Brief（导入 PDF）'
  return artifact.kind === 'video' ? `导入演示视频 · ${artifact.id.slice(-8)}` : `${assetType(artifact)}产物`
}
function assetMetadata(asset: ApiProjectMediaAsset): string {
  const dimensions = asset.width && asset.height ? `${asset.width} × ${asset.height}` : ''
  const duration = asset.durationSeconds ? `${asset.durationSeconds.toFixed(1)} 秒` : ''
  const size = asset.sizeBytes ? `${(asset.sizeBytes / 1024 / 1024).toFixed(1)} MB` : ''
  return [dimensions, duration, size].filter(Boolean).join(' · ') || asset.mimeType
}
