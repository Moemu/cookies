import { useMemo, useState } from 'react'
import { ArrowRight, Check, ChevronDown, CircleCheck, ClipboardCheck, Download, ExternalLink, FileText, Image, RotateCcw, Save, Send, ShieldCheck, Sparkles, ThumbsDown, ThumbsUp } from 'lucide-react'
import { useProject } from '../context/ProjectContext'
import { projectEvidence } from '../data/projects'
import type { ArtifactKey, DataState } from '../types'
import { StateBoundary } from './StateBoundary'

export function ArtifactFlow({ compact = false }: { compact?: boolean }) {
  const { currentProject } = useProject()
  const order: ArtifactKey[] = ['brief', 'strategy', 'creative', 'insight', 'delivery']
  return <div className={compact ? 'artifact-flow compact' : 'artifact-flow'} aria-label="Project 产物链路">{order.map((key, index) => { const artifact = currentProject.artifacts[key]; return <div className="artifact-node" key={key}><span>{String(index + 1).padStart(2, '0')}</span><div><b>{artifact.label} {artifact.version}</b><small>{artifact.status} · {artifact.owner}</small></div>{index < order.length - 1 ? <ArrowRight size={14}/> : null}</div> })}</div>
}

export function ImageTextCreationPage({ state }: { state: DataState }) {
  const { currentProject, updateArtifact } = useProject()
  const [selected, setSelected] = useState(0)
  const [channel, setChannel] = useState('小红书 4:5')
  const [headline, setHeadline] = useState('看得见的精度，兑现你的创新。')
  const [version, setVersion] = useState(8)
  const [notice, setNotice] = useState('')
  const pages = ['封面主张', '精度证据', '制造场景', '行动引导']
  const save = () => { const nextVersion = `v1.${version + 1}`; setVersion(value => value + 1); updateArtifact('creative', { version: nextVersion, status: '制作中', sourceVersion: `策略 ${currentProject.artifacts.strategy.version}`, summary: `${channel} 图文 4 页，品牌检查通过` }); setNotice(`已保存为 ${nextVersion}`) }
  return <StateBoundary state={state} onRetry={() => setNotice('已重新加载')} onCreate={() => setNotice('已创建空白画板')}><div className="image-editor-specialized">
    <aside className="creative-structure"><div className="surface-toolbar"><h3>图文结构</h3><button aria-label="新增图文页面"><Image size={16}/></button></div>{pages.map((page, index) => <button key={page} className={selected === index ? 'creative-page active' : 'creative-page'} onClick={() => setSelected(index)}><span>{String(index + 1).padStart(2, '0')}</span><b>{page}</b><small>{index === 0 ? '主视觉' : index === 3 ? 'CTA' : '内容页'}</small></button>)}<div className="version-block"><span>来源</span><b>{currentProject.artifacts.strategy.version}</b><small>{currentProject.artifacts.strategy.summary}</small></div></aside>
    <section className="image-canvas-workspace"><div className="canvas-toolbar light"><span>{currentProject.name} · 图文 v1.{version}</span><div><button onClick={() => setNotice('预览链接已生成')}><ExternalLink size={14}/>预览</button><button onClick={() => setNotice('PNG 导出任务已创建')}><Download size={14}/>导出</button></div></div><div className="portrait-stage"><div className="social-poster"><img src="/assets/white-precision-cnc.png" alt="CNC 设备加工高精度金属零件"/><div className="poster-copy"><small>WHITE PRECISION</small><h2>{headline}</h2><p>±0.01mm 精度 · 98%+ 准时交付</p></div><span className="poster-index">0{selected + 1} / 04</span></div></div><div className="page-strip">{pages.map((page, index) => <button key={page} className={selected === index ? 'active' : ''} onClick={() => setSelected(index)}><span>{index + 1}</span>{page}</button>)}</div></section>
    <aside className="creative-inspector"><div className="surface-toolbar"><h3>页面属性</h3><span className="status success"><span/>品牌检查通过</span></div><label>渠道与画幅<select value={channel} onChange={event => setChannel(event.target.value)}><option>小红书 4:5</option><option>公众号 16:9</option><option>信息流 1:1</option></select></label><label>主标题<textarea value={headline} onChange={event => setHeadline(event.target.value)} maxLength={24}/><small>{headline.length} / 24 字</small></label><div className="check-list"><span><Check size={14}/>安全区未遮挡</span><span><Check size={14}/>核心信息有证据</span><span><Check size={14}/>品牌用语一致</span></div><button className="primary-button full" onClick={save}><Save size={15}/>保存新版本</button>{notice ? <div className="inline-notice" role="status">{notice}</div> : null}</aside>
  </div></StateBoundary>
}

export function ReportCenterPage({ state }: { state: DataState }) {
  const { currentProject, updateArtifact } = useProject()
  const [section, setSection] = useState('执行摘要')
  const [version, setVersion] = useState(4)
  const [notice, setNotice] = useState('')
  const sections = ['执行摘要', '发生了什么', '为什么发生', '创意样本', '下一步行动']
  const save = () => { const nextVersion = `v1.${version + 1}`; setVersion(value => value + 1); updateArtifact('insight', { version: nextVersion, status: '已确认', sourceVersion: `创意 ${currentProject.artifacts.creative.version}`, summary: '证据前置版本点击率较基线提升 18%，95% 置信范围 +12% 至 +23%' }); setNotice(`报告 ${nextVersion} 已保存`) }
  return <StateBoundary state={state}><div className="report-workspace">
    <aside className="report-outline"><div className="surface-toolbar"><h3>报告结构</h3><button aria-label="新增报告章节"><FileText size={15}/></button></div>{sections.map((item, index) => <button className={section === item ? 'active' : ''} key={item} onClick={() => setSection(item)}><span>{String(index + 1).padStart(2, '0')}</span>{item}</button>)}<div className="version-block"><span>报告版本</span><b>v1.{version}</b><small>数据截止 2026-07-22 16:00</small></div></aside>
    <article className="report-document"><div className="document-meta"><span>{currentProject.name}</span><span>效果分析报告 v1.{version}</span><button onClick={() => setNotice('PDF 导出任务已创建')}><Download size={14}/>导出 PDF</button></div><h1>{section === '执行摘要' ? '证据前置版本，正在形成稳定增量。' : section}</h1><p className="report-lead">过去 12 周，核心素材完成度从 68% 提升到 86%。第 9 周后，增长主要来自“精度证据 + 真实制造场景”的组合。</p><div className="report-metric-line"><div><small>素材完成度</small><b>86%</b><span>较基线 +18%</span></div><div><small>样本</small><b>48</b><span>有效素材版本</span></div><div><small>置信范围</small><b>95%</b><span>差异 +12% 至 +23%</span></div></div><h2>结论与边界</h2><p>该结论适用于白域精工在中国大陆的销售线索广告。样本仍集中于精密制造题材，跨区域复用前需重新验证。</p><div className="report-callout"><b>建议行动</b><p>下一轮将证据前置版本扩大到 30% 素材覆盖，并保留纯产品特写作为对照组。</p></div></article>
    <aside className="report-sources"><div className="surface-toolbar"><h3>引用与版本</h3><button aria-label="报告更多操作"><ChevronDown size={15}/></button></div>{projectEvidence.map(item => <button key={item.id}><span>{item.id}</span><div><b>{item.title}</b><small>{item.source} · {item.date}</small></div><ExternalLink size={13}/></button>)}<button className="primary-button full" onClick={save}><Save size={15}/>保存报告版本</button>{notice ? <div className="inline-notice" role="status">{notice}</div> : null}</aside>
  </div></StateBoundary>
}

export function DeliveryPlanPage({ state }: { state: DataState }) {
  const { currentProject, addChangeSet, updateChangeSetStatus, updateArtifact } = useProject()
  const [step, setStep] = useState('计划配置')
  const [notice, setNotice] = useState('')
  const [budget, setBudget] = useState(currentProject.budget)
  const latest = currentProject.changeSets[0]
  const createChange = () => { const next = addChangeSet(); setNotice(`${next.id} 已创建`) }
  const submit = () => { if (!latest) return; updateChangeSetStatus(latest.id, '待审批'); updateArtifact('delivery', { status: '待审批', sourceVersion: `洞察 ${currentProject.artifacts.insight.version}`, summary: `预算 ¥${budget.toLocaleString('zh-CN')}，${latest.id} 待审批` }); setNotice(`${latest.id} 已提交审批`) }
  return <StateBoundary state={state}><div className="delivery-plan-workspace">
    <section className="plan-main"><ArtifactFlow compact/><div className="plan-tabs">{['计划配置', '素材组合', '预算与排期', '校验'].map(item => <button className={step === item ? 'active' : ''} key={item} onClick={() => setStep(item)}>{item}</button>)}</div><div className="plan-form"><div><label>计划名称<input defaultValue="销售线索增长计划 06"/></label><label>转化目标<select defaultValue="高质量销售线索"><option>高质量销售线索</option><option>表单提交</option></select></label></div><div><label>总预算（CNY）<input type="number" value={budget} onChange={event => setBudget(Number(event.target.value))}/></label><label>投放周期<input defaultValue="2026-07-25 至 2026-08-31"/></label></div><label>素材来源<div className="upstream-source"><Sparkles size={17}/><span><b>创意 {currentProject.artifacts.creative.version}</b><small>{currentProject.artifacts.creative.summary}</small></span><CircleCheck size={17}/></div></label></div><div className="validation-list"><h3>上线前校验</h3>{['商品与落地页绑定准确', '预算未超过 Project 护栏', '素材版权与品牌检查通过', '转化追踪最近 30 分钟有信号'].map(item => <span key={item}><CircleCheck size={16}/>{item}</span>)}</div></section>
    <aside className="changeset-panel"><div className="surface-toolbar"><h3>ChangeSet</h3><button aria-label="刷新 ChangeSet"><RotateCcw size={15}/></button></div>{latest ? <><div className="changeset-title"><span>{latest.id} · v{latest.version}</span><h2>{latest.title}</h2><small>风险 {latest.risk} · 预算影响 ¥{latest.budgetImpact.toLocaleString('zh-CN')}</small></div><div className="diff-list">{latest.changes.map(change => <div key={change.field}><b>{change.field}</b><span>{change.before}</span><ArrowRight size={13}/><strong>{change.after}</strong></div>)}</div><div className="rollback-copy"><ShieldCheck size={16}/><span><b>回滚方案</b><small>{latest.rollbackPlan}</small></span></div></> : <div className="panel-empty">尚未创建 ChangeSet</div>}<button className="secondary-button full" onClick={createChange}>生成 ChangeSet</button><button className="primary-button full" onClick={submit} disabled={!latest || latest.status === '待审批'}><Send size={15}/>{latest?.status === '待审批' ? '等待审批' : '提交审批'}</button>{notice ? <div className="inline-notice" role="status">{notice}</div> : null}</aside>
  </div></StateBoundary>
}

export function ApprovalCenterPage({ state }: { state: DataState }) {
  const { currentProject, updateChangeSetStatus, rollbackChangeSet, updateArtifact } = useProject()
  const [selectedId, setSelectedId] = useState(currentProject.changeSets[0]?.id ?? '')
  const [notice, setNotice] = useState('')
  const selected = useMemo(() => currentProject.changeSets.find(item => item.id === selectedId), [currentProject.changeSets, selectedId])
  const decide = (status: '已批准' | '已拒绝') => { if (!selected) return; updateChangeSetStatus(selected.id, status); updateArtifact('delivery', { status: status === '已批准' ? '执行中' : '待审批', summary: status === '已批准' ? `${selected.id} 已批准，等待受控执行` : `${selected.id} 已拒绝，计划保持原版本` }); setNotice(`${selected.id} ${status}`) }
  const rollback = () => { if (!selected) return; rollbackChangeSet(selected.id); updateArtifact('delivery', { status: '待审批', summary: `${selected.id} 已回滚，恢复前一计划版本` }); setNotice(`${selected.id} 已回滚到前一版本`) }
  return <StateBoundary state={state}><div className="approval-workspace">
    <aside className="approval-queue"><div className="surface-toolbar"><h3>审批队列</h3><span>{currentProject.changeSets.length}</span></div>{currentProject.changeSets.map(item => <button key={item.id} className={selectedId === item.id ? 'active' : ''} onClick={() => setSelectedId(item.id)}><span>{item.id}</span><b>{item.title}</b><small>{item.status} · 风险 {item.risk} · ¥{item.budgetImpact.toLocaleString('zh-CN')}</small></button>)}</aside>
    <section className="approval-detail">{selected ? <><div className="approval-heading"><div><span>{selected.id} · v{selected.version}</span><h2>{selected.title}</h2><p>由 {selected.createdBy} 提交于 {selected.createdAt}，涉及预算、广告组合和新素材探索。</p></div><span className={`approval-status ${selected.status}`}>{selected.status}</span></div><div className="approval-diff"><h3>变更前后</h3>{selected.changes.map(change => <div key={change.field}><b>{change.field}</b><span>{change.before}</span><ArrowRight size={15}/><strong>{change.after}</strong></div>)}</div><div className="approval-evidence"><h3>决策证据</h3>{projectEvidence.filter(item => selected.evidenceIds.includes(item.id)).map(item => <div key={item.id}><ClipboardCheck size={16}/><span><b>{item.title}</b><small>{item.source} · 可信度 {item.confidence}</small></span></div>)}</div><div className="approval-actions"><button className="secondary-button danger-text" onClick={() => decide('已拒绝')}><ThumbsDown size={15}/>拒绝并说明</button><button className="secondary-button" onClick={rollback} disabled={!['已批准', '已执行'].includes(selected.status)}><RotateCcw size={15}/>回滚</button><button className="primary-button" onClick={() => decide('已批准')} disabled={selected.status !== '待审批'}><ThumbsUp size={15}/>批准 ChangeSet</button></div>{notice ? <div className="inline-notice" role="status">{notice}</div> : null}</> : <div className="panel-empty">没有待处理审批</div>}</section>
    <aside className="approval-audit"><span className="section-label">审计记录</span>{[{ time: '16:30', text: 'Noah Xu 提交 v3' }, { time: '16:12', text: '系统完成预算护栏校验' }, { time: '15:58', text: '引用洞察 INS-014' }].map(item => <div key={item.time}><time>{item.time}</time><span>{item.text}</span></div>)}</aside>
  </div></StateBoundary>
}
