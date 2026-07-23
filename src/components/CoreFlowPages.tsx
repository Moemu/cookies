import { useMemo, useState } from 'react'
import {
  ArrowRight, BarChart3, BookOpenCheck, Check, ChevronDown, CircleCheck,
  Database, Filter, Layers3, Play, RefreshCw, Search, Sparkles, TrendingUp,
} from 'lucide-react'
import type { DataState } from '../types'
import { StateBoundary } from './StateBoundary'

const adRows = [
  { id: 'AD-2607-031', name: '精度证据·研发负责人', platform: '巨量引擎', format: '视频', spend: 28640, impressions: 682400, ctr: 4.18, cpa: 54.2, signal: '持续放量' },
  { id: 'AD-2607-028', name: '真实制造场景·采购线', platform: '腾讯广告', format: '图文', spend: 21800, impressions: 486200, ctr: 3.74, cpa: 61.8, signal: '稳定' },
  { id: 'AD-2607-019', name: '短剧前贴·交期冲突', platform: '巨量引擎', format: '视频', spend: 18420, impressions: 438900, ctr: 4.62, cpa: 49.6, signal: '优先扩量' },
  { id: 'AD-2607-014', name: '游戏前贴·精度挑战', platform: '快手磁力', format: '视频', spend: 15680, impressions: 326800, ctr: 3.26, cpa: 68.4, signal: '观察' },
  { id: 'AD-2607-008', name: '纯产品特写·对照组', platform: '腾讯广告', format: '图文', spend: 13320, impressions: 312100, ctr: 2.41, cpa: 82.7, signal: '建议降量' },
]

export function AdDataInsightPage({ state }: { state: DataState }) {
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

  return <StateBoundary state={state} onRetry={() => setNotice('广告数据已重新加载')} onCreate={() => setNotice('数据源接入向导已打开')}>
    <div className="ad-insight-workspace">
      <section className="ad-insight-main">
        <div className="core-flow-toolbar">
          <div><span className="section-label">AD DATA CONNECTOR</span><h2>广告数据洞察</h2><p>统一查看平台消耗、曝光、点击率与线索成本，当前为可交互路演数据。</p></div>
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
        <button className="primary-button full" onClick={() => setNotice(`已创建「${selected.name}」深度分析任务`)}>创建深度分析<ArrowRight size={15}/></button>
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
  const [query, setQuery] = useState('')
  const [type, setType] = useState('全部')
  const [selectedId, setSelectedId] = useState(assets[0].id)
  const [notice, setNotice] = useState('')
  const [confirmed, setConfirmed] = useState<string[]>(['AS-1027'])
  const filtered = useMemo(() => assets.filter(asset =>
    (type === '全部' || asset.type === type)
    && `${asset.id} ${asset.name} ${asset.feature}`.toLowerCase().includes(query.trim().toLowerCase()),
  ), [query, type])
  const selected = assets.find(asset => asset.id === selectedId) ?? assets[0]
  const confirmExperience = () => {
    setConfirmed(current => current.includes(selected.id) ? current : [...current, selected.id])
    setNotice(`已将「${selected.experience}」沉淀为 Project 经验，可被 Brief 与创意引用。`)
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
        <button className="primary-button full" onClick={confirmExperience} disabled={confirmed.includes(selected.id)}><BookOpenCheck size={15}/>{confirmed.includes(selected.id) ? '经验已沉淀' : '确认并沉淀经验'}</button>
        {notice ? <div className="inline-notice" role="status">{notice}</div> : null}
      </aside>
    </div>
  </StateBoundary>
}
