import { useEffect, useMemo, useState } from 'react'
import {
  ArrowRight, BarChart3, BookOpenCheck, Check, CircleAlert, CircleCheck,
  Database, FileInput, Filter, Layers3, Lightbulb, Link2, Play, RefreshCw,
  Search, Sparkles, Target, TrendingUp,
} from 'lucide-react'
import { useProject } from '../context/ProjectContext'
import { api, type ApiArtifact } from '../data/api'
import type { DataState, SystemKey } from '../types'
import { StateBoundary } from './StateBoundary'

type OpenProject = (id: string, system?: SystemKey, navId?: string, objectId?: string, view?: string) => void

const preLaunchInsights = [
  {
    id: 'PRE-2607-011',
    title: '将精度证据放在叙事前 3 秒',
    category: '证据结构',
    channel: '全渠道',
    format: '视频',
    confidence: '高',
    evidence: '12 个同类项目，46 个有效素材版本',
    recommendation: '首镜头直接呈现 ±0.01mm，再用制造过程解释可信来源。',
    boundary: '适用于线索获取和产品证明，不适用于纯品牌情绪片。',
  },
  {
    id: 'PRE-2607-008',
    title: '用交期冲突建立短剧前贴钩子',
    category: '转化钩子',
    channel: '巨量引擎',
    format: '视频',
    confidence: '中高',
    evidence: '短剧前贴 18 组对照，CTR 中位数提升 21%',
    recommendation: '在第 4 秒前完成客户催交、产线受阻和解决方向三步冲突。',
    boundary: '需要真实交付证据支持，避免制造无法兑现的紧迫感。',
  },
  {
    id: 'PRE-2607-005',
    title: '采购与研发使用不同的价值排序',
    category: '受众表达',
    channel: '腾讯广告',
    format: '图文',
    confidence: '高',
    evidence: '双受众分层实验，样本 1,820 条有效线索',
    recommendation: '采购先看准时交付和稳定性，研发先看公差、材料与加工能力。',
    boundary: '同一广告只保留一个主受众，避免首屏同时堆叠两套价值主张。',
  },
  {
    id: 'PRE-2607-003',
    title: '纯产品特写不能替代应用场景',
    category: '风险与反例',
    channel: '全渠道',
    format: '图文',
    confidence: '高',
    evidence: '7 组失败变体，平均 CTR 低于项目基线 0.83pp',
    recommendation: '产品特写必须与装配、测量或真实工作场景共同出现。',
    boundary: '电商详情页中的规格展示不受此限制。',
  },
]

export function PreLaunchInsightPage({ state, onOpenProject }: { state: DataState; onOpenProject: OpenProject }) {
  const { currentProject, reloadProjects } = useProject()
  const [channel, setChannel] = useState('全渠道')
  const [format, setFormat] = useState('全部形式')
  const [query, setQuery] = useState('')
  const [selectedId, setSelectedId] = useState(preLaunchInsights[0].id)
  const [references, setReferences] = useState<ApiArtifact[]>([])
  const [notice, setNotice] = useState('')
  const [busyTarget, setBusyTarget] = useState<'brief' | 'creative' | ''>('')
  const filtered = useMemo(() => preLaunchInsights.filter(insight =>
    (channel === '全渠道' || insight.channel === channel || insight.channel === '全渠道')
    && (format === '全部形式' || insight.format === format)
    && `${insight.id} ${insight.title} ${insight.category} ${insight.recommendation}`.toLowerCase().includes(query.trim().toLowerCase()),
  ), [channel, format, query])
  const selected = preLaunchInsights.find(insight => insight.id === selectedId) ?? preLaunchInsights[0]

  useEffect(() => {
    let active = true
    void api.listArtifacts(currentProject.id).then(artifacts => {
      if (active) setReferences(artifacts.filter(artifact => artifact.content.startsWith('[prelaunch-insight]')))
    }).catch(() => {
      if (active) setReferences([])
    })
    return () => { active = false }
  }, [currentProject.id])

  const citeInsight = async (target: 'brief' | 'creative') => {
    setBusyTarget(target)
    try {
      const reference = await api.createArtifact({
        projectId: currentProject.id,
        kind: 'document',
        status: 'ready',
        content: `[prelaunch-insight] ${selected.id} | ${selected.title} | 建议：${selected.recommendation} | 证据：${selected.evidence} | 边界：${selected.boundary} | 引用目标：${target}`,
      })
      if (target === 'brief' && currentProject.artifacts.brief.id) {
        const artifacts = await api.listArtifacts(currentProject.id)
        const brief = artifacts.find(artifact => artifact.id === currentProject.artifacts.brief.id)
        if (brief && !brief.content.includes(selected.id)) {
          await api.updateArtifact(brief.id, { content: `${brief.content}\n\n投前洞察引用 ${selected.id}：${selected.recommendation}` })
        }
      }
      if (target === 'creative') {
        const creativeTask = [...currentProject.tasks].reverse().find(task => task.type !== 'strategy')
        if (creativeTask) {
          await api.updateTask(creativeTask.id, {
            sourceArtifactIds: [...new Set([...creativeTask.sourceArtifactIds, reference.id])],
          })
        }
      }
      setReferences(current => [...current, reference])
      await reloadProjects()
      setNotice(target === 'brief'
        ? `已将 ${selected.id} 写入当前 Brief 的证据引用。`
        : `已将 ${selected.id} 关联到创意来源链，后续新建创意任务也会自动继承。`)
    } catch (cause) {
      setNotice(cause instanceof Error ? cause.message : '投前洞察引用失败，请稍后重试。')
    } finally {
      setBusyTarget('')
    }
  }

  return <StateBoundary state={state} onRetry={() => setNotice('投前证据已重新加载')} onCreate={() => setNotice('投前洞察无需新建任务，可直接筛选和引用')}>
    <div className="prelaunch-workspace">
      <section className="prelaunch-main">
        <div className="core-flow-toolbar">
          <div><span className="section-label">PRE-LAUNCH INSIGHT</span><h2>在生产前，让历史经验参与决策</h2><p>当前 Project：{currentProject.name}。结论区分建议、证据和适用边界，引用后进入真实产物来源链。</p></div>
          <div className="prelaunch-context"><Target size={16}/><span><small>项目目标</small><b>{currentProject.goal}</b></span></div>
        </div>
        <div className="prelaunch-filterbar">
          <div className="search-field"><Search size={15}/><input aria-label="搜索投前洞察" value={query} onChange={event => setQuery(event.target.value)} placeholder="搜索结论、证据或风险"/></div>
          <label>渠道<select aria-label="投前洞察渠道" value={channel} onChange={event => setChannel(event.target.value)}><option>全渠道</option><option>巨量引擎</option><option>腾讯广告</option></select></label>
          <label>形式<select aria-label="投前洞察形式" value={format} onChange={event => setFormat(event.target.value)}><option>全部形式</option><option>视频</option><option>图文</option></select></label>
          <span>{filtered.length} 条可用结论</span>
        </div>
        <div className="prelaunch-table" role="list" aria-label="投前洞察列表">
          <div className="prelaunch-row header"><span>结论</span><span>类型</span><span>证据</span><span>置信度</span></div>
          {filtered.map(insight => <button role="listitem" key={insight.id} className={selectedId === insight.id ? 'prelaunch-row active' : 'prelaunch-row'} onClick={() => setSelectedId(insight.id)}>
            <span><b>{insight.title}</b><small>{insight.id} · {insight.channel} · {insight.format}</small></span>
            <span>{insight.category}</span><span>{insight.evidence}</span><span><CircleCheck size={14}/>{insight.confidence}</span>
          </button>)}
          {!filtered.length ? <div className="panel-empty">没有符合当前渠道和形式的投前洞察，请调整筛选条件。</div> : null}
        </div>
      </section>
      <aside className="prelaunch-detail">
        <span className="section-label">当前结论</span><h3>{selected.title}</h3><p>{selected.id} · {selected.category} · 置信度 {selected.confidence}</p>
        <div className="prelaunch-fact"><Lightbulb size={17}/><span><small>建议</small><b>{selected.recommendation}</b></span></div>
        <div className="prelaunch-fact"><Database size={17}/><span><small>证据</small><b>{selected.evidence}</b></span></div>
        <div className="prelaunch-boundary"><CircleAlert size={16}/><span><small>适用边界</small>{selected.boundary}</span></div>
        <div className="prelaunch-actions">
          <button className="secondary-button full" disabled={Boolean(busyTarget)} onClick={() => void citeInsight('brief')}><FileInput size={15}/>{busyTarget === 'brief' ? '正在写入 Brief…' : '引用到 Brief'}</button>
          <button className="primary-button full" disabled={Boolean(busyTarget)} onClick={() => void citeInsight('creative')}><Link2 size={15}/>{busyTarget === 'creative' ? '正在关联创意…' : '引用到创意任务'}</button>
        </div>
        <button className="text-button prelaunch-deeplink" onClick={() => onOpenProject(currentProject.id, 'strategy', 'briefs')}>进入需求中心检查 Brief<ArrowRight size={14}/></button>
        <div className="reference-count"><BookOpenCheck size={15}/><span><b>{references.length} 条引用记录</b><small>保存在当前 Project，可跨刷新追溯</small></span></div>
        {notice ? <div className="inline-notice" role="status">{notice}</div> : null}
      </aside>
    </div>
  </StateBoundary>
}

const adRows = [
  { id: 'AD-2607-031', name: '精度证据·研发负责人', platform: '巨量引擎', format: '视频', spend: 28640, impressions: 682400, ctr: 4.18, cpa: 54.2, signal: '持续放量' },
  { id: 'AD-2607-028', name: '真实制造场景·采购线', platform: '腾讯广告', format: '图文', spend: 21800, impressions: 486200, ctr: 3.74, cpa: 61.8, signal: '稳定' },
  { id: 'AD-2607-019', name: '短剧前贴·交期冲突', platform: '巨量引擎', format: '视频', spend: 18420, impressions: 438900, ctr: 4.62, cpa: 49.6, signal: '优先扩量' },
  { id: 'AD-2607-014', name: '游戏前贴·精度挑战', platform: '快手磁力', format: '视频', spend: 15680, impressions: 326800, ctr: 3.26, cpa: 68.4, signal: '观察' },
  { id: 'AD-2607-008', name: '纯产品特写·对照组', platform: '腾讯广告', format: '图文', spend: 13320, impressions: 312100, ctr: 2.41, cpa: 82.7, signal: '建议降量' },
]

export function PostLaunchAnalysisPage({ state, onOpenProject }: { state: DataState; onOpenProject: OpenProject }) {
  const { currentProject, reloadProjects } = useProject()
  const [platform, setPlatform] = useState('全部平台')
  const [window, setWindow] = useState('近 30 天')
  const [query, setQuery] = useState('')
  const [selectedId, setSelectedId] = useState(adRows[0].id)
  const [notice, setNotice] = useState('')
  const filtered = useMemo(() => adRows.filter(row =>
    (platform === '全部平台' || row.platform === platform)
    && `${row.id} ${row.name} ${row.format}`.toLowerCase().includes(query.trim().toLowerCase()),
  ), [platform, query])
  const selected = filtered.find(row => row.id === selectedId) ?? filtered[0] ?? adRows[0]
  const spend = filtered.reduce((sum, row) => sum + row.spend, 0)
  const impressions = filtered.reduce((sum, row) => sum + row.impressions, 0)
  const averageCtr = filtered.length ? filtered.reduce((sum, row) => sum + row.ctr, 0) / filtered.length : 0
  const averageCpa = filtered.length ? filtered.reduce((sum, row) => sum + row.cpa, 0) / filtered.length : 0
  const [reportBusy, setReportBusy] = useState(false)

  const createReport = async () => {
    setReportBusy(true)
    try {
      const report = await api.createArtifact({
        projectId: currentProject.id,
        kind: 'document',
        status: 'ready',
        content: `[insight] 投后分析报告 | 窗口：${window} | 平台：${platform} | 消耗：¥${spend.toLocaleString('zh-CN')} | 曝光：${impressions.toLocaleString('zh-CN')} | 平均 CTR：${averageCtr.toFixed(2)}% | 平均 CPL：¥${averageCpa.toFixed(1)} | 关键素材：${selected.name} | 结论：精度证据前置与真实制造场景共同驱动表现。`,
      })
      await reloadProjects()
      setNotice(`投后分析报告 ${report.id.slice(0, 8)} 已确认，可进入经验库沉淀。`)
    } catch (cause) {
      setNotice(cause instanceof Error ? cause.message : '生成投后分析报告失败，请重试。')
    } finally {
      setReportBusy(false)
    }
  }

  return <StateBoundary state={state} onRetry={() => setNotice('广告数据已重新加载')} onCreate={() => setNotice('数据源接入向导已打开')}>
    <div className="ad-insight-workspace">
      <section className="ad-insight-main">
        <div className="core-flow-toolbar">
          <div><span className="section-label">POST-LAUNCH ANALYSIS</span><h2>投后分析</h2><p>统一查看平台消耗、曝光、点击率与线索成本，并把素材表现解释为下一轮可用经验。</p></div>
          <div className="core-flow-actions">
            <label>平台<select aria-label="广告平台" value={platform} onChange={event => setPlatform(event.target.value)}><option>全部平台</option><option>巨量引擎</option><option>腾讯广告</option><option>快手磁力</option></select></label>
            <label>窗口<select aria-label="数据窗口" value={window} onChange={event => setWindow(event.target.value)}><option>近 7 天</option><option>近 30 天</option><option>本季度</option></select></label>
            <button className="secondary-button" onClick={() => setNotice(`${platform} · ${window} 数据已刷新至 11:30`)}><RefreshCw size={15}/>刷新数据</button>
          </div>
        </div>
        <div className="ad-metric-grid">
          <div><span>广告消耗</span><b>¥{(spend / 1000).toFixed(1)}k</b><small><TrendingUp size={13}/>较前期 +12.4%</small></div>
          <div><span>曝光</span><b>{(impressions / 1_000_000).toFixed(2)}m</b><small>统一去重口径</small></div>
          <div><span>平均 CTR</span><b>{averageCtr.toFixed(2)}%</b><small className="positive">证据前置表现更好</small></div>
          <div><span>平均 CPL</span><b>¥{averageCpa.toFixed(1)}</b><small>高质量销售线索</small></div>
        </div>
        <div className="ad-data-panel">
          <div className="ad-data-heading"><div className="search-field"><Search size={15}/><input aria-label="搜索广告数据" value={query} onChange={event => setQuery(event.target.value)} placeholder="搜索广告、素材或编号"/></div><span>{filtered.length} 条广告 · {window}</span></div>
          <div className="ad-data-table">
            <div className="ad-data-row header"><span>广告</span><span>平台</span><span>消耗</span><span>曝光</span><span>CTR</span><span>CPL</span><span>信号</span></div>
            {filtered.map(row => <button key={row.id} className={selectedId === row.id ? 'ad-data-row active' : 'ad-data-row'} onClick={() => setSelectedId(row.id)}>
              <span><b>{row.name}</b><small>{row.id} · {row.format}</small></span><span>{row.platform}</span><span>¥{row.spend.toLocaleString('zh-CN')}</span><span>{row.impressions.toLocaleString('zh-CN')}</span><span>{row.ctr}%</span><span>¥{row.cpa}</span><span><i/>{row.signal}</span>
            </button>)}
            {!filtered.length ? <div className="panel-empty">没有匹配的广告数据</div> : null}
          </div>
        </div>
      </section>
      <aside className="ad-insight-detail">
        <span className="section-label">选中广告分析</span><h3>{selected.name}</h3><p>{selected.platform} · {selected.format} · 当前信号：{selected.signal}</p>
        <div className="mini-trend"><BarChart3 size={18}/><div><b>前三秒留存 71%</b><small>较账户视频均值 +14%</small></div></div>
        {['精度数字在第 1.2 秒出现', '制造过程提供可信证据', 'CTA 与线索目标一致'].map(item => <span className="analysis-check" key={item}><CircleCheck size={15}/>{item}</span>)}
        <button className="secondary-button full" disabled={reportBusy} onClick={() => void createReport()}><BarChart3 size={15}/>{reportBusy ? '正在生成报告…' : '生成项目复盘报告'}</button>
        <button className="primary-button full" onClick={() => onOpenProject(currentProject.id, 'insight', 'knowledge')}>进入经验沉淀<ArrowRight size={15}/></button>
        {notice ? <div className="inline-notice" role="status">{notice}</div> : null}
      </aside>
    </div>
  </StateBoundary>
}

const assets = [
  { id: 'AS-1042', name: '短剧前贴·交期冲突', type: '视频', source: 'AI 生成', status: '分析完成', ctr: '4.62%', feature: '冲突前置 / 4 秒钩子', experience: '用交期风险制造决策压力' },
  { id: 'AS-1038', name: '游戏前贴·精度挑战', type: '视频', source: 'AI 生成', status: '分析完成', ctr: '3.26%', feature: '挑战任务 / 结果反馈', experience: '目标、失败、反转需在 6 秒内闭环' },
  { id: 'AS-1027', name: 'CNC 精度证据主视觉', type: '图文', source: '品牌资产', status: '已沉淀', ctr: '3.74%', feature: '数字证据 / 工艺特写', experience: '证据数字应在首屏完整可见' },
  { id: 'AS-1019', name: '纯产品特写对照组', type: '图文', source: '历史投放', status: '待复审', ctr: '2.41%', feature: '产品单帧 / 弱证据', experience: '缺少场景时点击率明显下降' },
]

export function AssetExperiencePage({ state, mode }: { state: DataState; mode: 'assets' | 'knowledge' }) {
  const { currentProject, reloadProjects } = useProject()
  const [query, setQuery] = useState('')
  const [type, setType] = useState('全部')
  const [selectedId, setSelectedId] = useState(assets[0].id)
  const [notice, setNotice] = useState('')
  const [busy, setBusy] = useState(false)
  const [confirmed, setConfirmed] = useState<string[]>(['AS-1027'])
  const filtered = useMemo(() => assets.filter(asset =>
    (type === '全部' || asset.type === type)
    && `${asset.id} ${asset.name} ${asset.feature}`.toLowerCase().includes(query.trim().toLowerCase()),
  ), [query, type])
  const selected = assets.find(asset => asset.id === selectedId) ?? assets[0]
  const confirmExperience = async () => {
    setBusy(true)
    try {
      await api.createArtifact({
        projectId: currentProject.id,
        kind: 'document',
        content: `[knowledge] ${selected.experience} | 来源 ${selected.id} | 适用：B2B 制造、线索广告、首屏证据型创意 | 边界：纯品牌曝光任务`,
        status: 'ready',
      })
      await reloadProjects()
      setConfirmed(current => current.includes(selected.id) ? current : [...current, selected.id])
      setNotice(`已将「${selected.experience}」写入 Project 经验资产，可被下一轮 Brief 与创意引用。`)
    } catch (cause) {
      setNotice(cause instanceof Error ? cause.message : '经验沉淀失败，请稍后重试。')
    } finally {
      setBusy(false)
    }
  }

  return <StateBoundary state={state} onRetry={() => setNotice('素材索引已重新加载')} onCreate={() => setNotice('素材上传面板已打开')}>
    <div className="asset-experience-workspace">
      <section className="asset-library-panel">
        <div className="core-flow-toolbar">
          <div><span className="section-label">{mode === 'assets' ? 'ASSET MANAGEMENT' : 'EXPERIENCE LIBRARY'}</span><h2>{mode === 'assets' ? '素材管理与分析' : '素材经验沉淀'}</h2><p>从素材特征与广告表现中形成可复用、可追溯的创意经验。</p></div>
          <button className="primary-button" onClick={() => setNotice(mode === 'assets' ? '素材上传队列已打开' : '已创建一条空白候选经验')}><Sparkles size={15}/>{mode === 'assets' ? '导入素材' : '新建经验'}</button>
        </div>
        <div className="asset-filterbar">
          <div className="search-field"><Search size={15}/><input aria-label="搜索素材经验" value={query} onChange={event => setQuery(event.target.value)} placeholder="搜索素材、特征或经验"/></div>
          <label><Filter size={14}/><select aria-label="素材类型" value={type} onChange={event => setType(event.target.value)}><option>全部</option><option>视频</option><option>图文</option></select></label>
          <span>{filtered.length} 项 · 已沉淀 {confirmed.length}</span>
        </div>
        <div className="asset-card-grid">
          {filtered.map(asset => <button key={asset.id} className={selectedId === asset.id ? 'asset-analysis-card active' : 'asset-analysis-card'} onClick={() => setSelectedId(asset.id)}>
            <span className="asset-card-preview">{asset.type === '视频' ? <Play size={18} fill="currentColor"/> : <Layers3 size={18}/>}<small>{asset.type}</small></span>
            <span><small>{asset.id} · {asset.source}</small><b>{asset.name}</b><em>{asset.feature}</em></span>
            <span><strong>{asset.ctr}</strong><small>CTR</small></span>
            {confirmed.includes(asset.id) ? <i className="experience-confirmed"><Check size={12}/>已沉淀</i> : null}
          </button>)}
        </div>
      </section>
      <aside className="asset-analysis-detail">
        <span className="section-label">分析与经验</span><h3>{selected.name}</h3><p>{selected.status} · {selected.source}</p>
        <div className="feature-stack"><span>内容特征</span>{selected.feature.split(' / ').map(item => <b key={item}>{item}</b>)}</div>
        <div className="experience-card"><BookOpenCheck size={18}/><span><small>候选经验</small><b>{selected.experience}</b><p>适用条件：B2B 制造、线索广告、首屏证据型创意。反例：纯品牌曝光任务。</p></span></div>
        <button className="secondary-button full" onClick={() => setNotice(`已完成「${selected.name}」素材分析，特征与效果已对齐`)}><Database size={15}/>重新分析</button>
        <button className="primary-button full" onClick={() => { void confirmExperience() }} disabled={busy || confirmed.includes(selected.id)}><BookOpenCheck size={15}/>{busy ? '正在写入项目…' : confirmed.includes(selected.id) ? '经验已沉淀' : '确认并沉淀经验'}</button>
        {notice ? <div className="inline-notice" role="status">{notice}</div> : null}
      </aside>
    </div>
  </StateBoundary>
}
