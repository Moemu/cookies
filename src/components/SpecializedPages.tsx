import { useEffect, useMemo, useState } from 'react'
import { ArrowRight, Check, ChevronDown, CircleAlert, CircleCheck, ClipboardCheck, Download, ExternalLink, FileText, Film, Image, Music2, Play, RotateCcw, Save, Scissors, Send, ShieldCheck, Sparkles, Subtitles, ThumbsDown, ThumbsUp, Upload, Video, Volume2, WandSparkles } from 'lucide-react'
import { useProject } from '../context/ProjectContext'
import { useModelConfig } from '../context/ModelConfigContext'
import { commerceHookTemplates, hookStoryboard } from '../data/commerceHooks'
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

const performanceModes = [
  { id: 'viral-remake', label: '视频爆款复刻', detail: '拆解爆款结构与节奏，完成品牌映射和原创改写。', guard: '相似性与授权检查' },
  { id: 'pre-roll', label: '视频前贴', detail: '为原视频生成 4–10 秒高注意力开场并无缝拼接。', guard: '静音可理解性检查' },
  { id: 'digital-human', label: 'AI 数字人', detail: '用已授权形象、声音与口播脚本生成成片和变体。', guard: '人物、口型与 AI 标识检查' },
  { id: 'ai-ad', label: 'AI 广告生成', detail: '从转化目标到镜头、音频与剪辑，整条广告由 AI 原生生成。', guard: '事实、版权与品牌检查' },
]

const brandSteps = [
  ['01', '解析 Brief', '提取品牌主张、受众、边界与交付目标。'],
  ['02', '编写剧本', '形成叙事主线、分镜、台词和声音设计。'],
  ['03', '生成资产', '按镜头生成画面、角色、配音、音乐与图形资产。'],
  ['04', '生成广告', '基于已确认资产和剧本生成可预览的广告草稿。'],
  ['05', '剪辑与交付', '完成多轨剪辑、品牌检查、导出和版本归档。'],
]

export function VideoCreationPage({ state, activeView, onOpenTask }: { state: DataState, activeView: string, onOpenTask: (id: string) => void }) {
  const { currentProject, updateArtifact } = useProject()
  const [selected, setSelected] = useState('pre-roll')
  const [notice, setNotice] = useState('')
  const category = activeView === '品牌广告' ? 'brand' : activeView === '素材剪辑' ? 'editing' : 'performance'
  const activeMode = performanceModes.find(item => item.id === selected) ?? performanceModes[0]
  const create = () => {
    const name = category === 'performance' ? activeMode.label : category === 'brand' ? '品牌广告' : '素材剪辑 EditTask'
    updateArtifact('creative', { status: '制作中', sourceVersion: `策略 ${currentProject.artifacts.strategy.version}`, summary: `${name}任务已从已批准策略创建` })
    setNotice(`${name}创作任务已创建；已保留策略、来源与版本链。`)
    onOpenTask(`${category === 'editing' ? 'ED' : 'CR'}-2607-${String(Date.now()).slice(-4)}`)
  }
  const title = category === 'performance' ? '效果广告，以可测试的转化表达组织创作。' : category === 'brand' ? '品牌广告，从 Brief 到剪辑交付形成完整叙事。' : '素材剪辑，将已授权素材组织为可交付的视频版本。'
  const description = category === 'performance' ? '选择一种生成类型，系统会继承策略、品牌规则、渠道规格与来源授权。' : category === 'brand' ? '沿着 Brief、剧本、资产、广告生成和剪辑的固定路径推进，所有产物均保留来源与确认记录。' : '独立 EditTask 可从品牌、效果任务或存量项目素材进入；字幕、音频与转场在编辑器内完成。'
  return <StateBoundary state={state} onRetry={() => setNotice('创作配置已重新加载')} onCreate={create}><section className="video-creation-workspace">
    <header className="video-workspace-header"><div><span className="section-label">视频创作 · {activeView}</span><h2>{title}</h2><p>{description}</p></div>{category !== 'editing' ? <button className="primary-button" onClick={create}><Video size={16}/>新建{category === 'performance' ? activeMode.label : '品牌广告'}</button> : null}</header>
    {category === 'performance' ? <><div className="performance-mode-tabs" role="tablist" aria-label="效果广告生成类型">{performanceModes.map(mode => <button key={mode.id} role="tab" aria-selected={selected === mode.id} className={selected === mode.id ? 'active' : ''} onClick={() => setSelected(mode.id)}><b>{mode.label}</b><small>{mode.guard}</small></button>)}</div>{selected === 'pre-roll' ? <CommerceHookWorkspace onNotice={setNotice}/> : <div className="performance-workflow">
      <aside className="performance-mode-list"><span className="section-label">当前生成类型</span><div className="mode-summary"><b>{activeMode.label}</b><p>{activeMode.detail}</p></div><span className="section-label">创建前检查</span>{['策略版本与证据', '品牌规则与禁用词', '渠道规格与转化目标', '素材、声音与参考授权'].map(item => <span className="mode-check" key={item}><Check size={14}/>{item}</span>)}</aside>
      <section className="performance-detail"><div className="video-preview"><div className="preview-grid"/><span>00:00 / 00:15</span><button aria-label="播放视频预览"><Play size={17} fill="currentColor"/></button></div><div className="performance-copy"><span className="section-label">当前路径</span><h3>{activeMode.label}</h3><p>{activeMode.detail}</p><div className="workflow-meta"><span><b>输入</b>已批准策略、渠道规格、授权素材</span><span><b>核心护栏</b>{activeMode.guard}</span></div></div></section>
      <aside className="video-job-rail"><span className="section-label">创建任务</span><h3>沿用 Project 上下文</h3>{['策略版本与证据', '品牌规则与禁用词', '渠道规格与转化目标', '素材、声音与参考授权'].map(item => <span key={item}><Check size={14}/>{item}</span>)}<button className="secondary-button full" onClick={() => setNotice('来源与授权清单已打开')}>查看来源与授权</button></aside>
    </div>}</> : category === 'brand' ? <div className="brand-workflow"><div className="brand-brief-card"><span className="section-label">品牌广告 · 生产主线</span><h3>{currentProject.artifacts.strategy.version} 已批准</h3><p>{currentProject.artifacts.strategy.summary}</p><div><Sparkles size={17}/><span>所有生成资产、剧本与剪辑版本都保留 Brief 来源与人工确认记录。</span></div></div><ol>{brandSteps.map(([id, title, detail], index) => <li key={id}><span>{id}</span><div><b>{title}</b><p>{detail}</p></div>{index < brandSteps.length - 1 ? <ArrowRight size={16}/> : <WandSparkles size={17}/>}</li>)}</ol></div> : <VideoEditingWorkspace onNotice={setNotice} onCreate={create}/>} 
    {notice ? <div className="inline-notice" role="status">{notice}</div> : null}
  </section></StateBoundary>
}

function CommerceHookWorkspace({ onNotice }: { onNotice: (message: string) => void }) {
  const { currentProject, updateArtifact } = useProject()
  const { providers } = useModelConfig()
  const [selectedId, setSelectedId] = useState(commerceHookTemplates[0].id)
  const [fidelity, setFidelity] = useState(commerceHookTemplates[0].fidelity)
  const [camera, setCamera] = useState(commerceHookTemplates[0].camera)
  const [motion, setMotion] = useState(commerceHookTemplates[0].motion)
  const [result, setResult] = useState(commerceHookTemplates[0].result)
  const [previewing, setPreviewing] = useState(false)
  const selected = commerceHookTemplates.find(item => item.id === selectedId) ?? commerceHookTemplates[0]
  const configuredProvider = providers.find(provider => provider.status === '已配置')

  useEffect(() => {
    setFidelity(selected.fidelity)
    setCamera(selected.camera)
    setMotion(selected.motion)
    setResult(selected.result)
    setPreviewing(false)
  }, [selected])

  const prompt = `${fidelity} ${camera} ${motion} ${selected.environment} ${result} ${selected.guardrails}`
  const save = () => {
    updateArtifact('creative', { status: '制作中', sourceVersion: `策略 ${currentProject.artifacts.strategy.version}`, summary: `广告前贴 · ${selected.name} · ${selected.frameStrategy}` })
    onNotice(`「${selected.name}」已保存为广告前贴策略草稿，并保留来源版本。`)
  }
  const copyPrompt = async () => {
    try { await navigator.clipboard.writeText(prompt); onNotice('完整视频提示词已复制。') }
    catch { onNotice('提示词已准备好，请从右侧字段中复制。') }
  }
  return <div className="commerce-hook-workspace">
    <aside className="hook-template-rail">
      <div className="hook-rail-heading"><span className="section-label">场景策略库</span><b>电商前贴 / 钩子</b><small>学习资料 revision 399</small></div>
      {commerceHookTemplates.map((template, index) => <button key={template.id} className={selectedId === template.id ? 'active' : ''} onClick={() => setSelectedId(template.id)}><span>{String(index + 1).padStart(2, '0')}</span><div><b>{template.name}</b><small>{template.category} · {template.duration}</small></div></button>)}
      <a href="https://bytedance.larkoffice.com/wiki/H5uQwNji9iYH0TkNXaxcvFhUn2c" target="_blank" rel="noreferrer"><ExternalLink size={13}/>查看学习来源</a>
    </aside>
    <section className="hook-canvas">
      <div className="hook-canvas-toolbar"><div><span className="source-chip">{selected.frameStrategy}</span><b>{selected.name}</b></div><button onClick={copyPrompt}><ClipboardCheck size={14}/>复制提示词</button></div>
      <div className="hook-preview-stage">
        <div className="hook-phone-frame"><img src={selected.image} alt={`${selected.name}${selected.imageLabel}`}/><div className="hook-preview-shade"/><span className="hook-frame-label">{selected.imageLabel}</span><div className="hook-preview-copy"><small>ECOMMERCE HOOK · 9:16</small><b>{selected.hook}</b><span>{selected.duration} · 静音可理解</span></div><button aria-label={previewing ? '暂停钩子预览' : '播放钩子预览'} onClick={() => setPreviewing(value => !value)}><Play size={17} fill="currentColor"/></button></div>
        <div className="hook-proof"><span className="section-label">策略依据</span><h3>先建立信息缺口，再完成一次清晰变化。</h3><p>一个主动作、一个结果状态、一个稳定的商品定格。环境只提供辅助运动。</p><div>{selected.tags.map(tag => <span key={tag}>{tag}</span>)}</div></div>
      </div>
      <div className="hook-storyboard">{hookStoryboard.map((step, index) => <div key={step.time}><span>{step.time}</span><i/><b>{String(index + 1).padStart(2, '0')} · {step.name}</b><small>{step.detail}</small></div>)}</div>
    </section>
    <aside className="hook-inspector">
      <div className="surface-toolbar"><h3>提示词构建器</h3><span>Mock</span></div>
      <label>商品保真约束<textarea value={fidelity} onChange={event => setFidelity(event.target.value)}/></label>
      <label>镜头与光影<textarea value={camera} onChange={event => setCamera(event.target.value)}/></label>
      <label>唯一主动作<textarea value={motion} onChange={event => setMotion(event.target.value)}/></label>
      <label>结果与停留<textarea value={result} onChange={event => setResult(event.target.value)}/></label>
      <div className="hook-guardrail"><ShieldCheck size={15}/><span><b>自动附加生成护栏</b><small>{selected.guardrails}</small></span></div>
      {configuredProvider ? <div className="hook-model"><CircleCheck size={15}/><span><b>{configuredProvider.name}</b><small>{configuredProvider.defaultModel}</small></span></div> : <div className="hook-model missing"><CircleAlert size={15}/><span><b>尚未配置模型</b><small>请在全局“模型与密钥”页面配置后生成。</small></span></div>}
      <div className="hook-actions"><button className="secondary-button" onClick={save}><Save size={14}/>保存策略</button><button className="primary-button" disabled={!configuredProvider} onClick={() => onNotice(`${selected.name}的 3 镜头分镜已进入生成队列。`)}><WandSparkles size={14}/>生成分镜</button></div>
    </aside>
  </div>
}

function VideoEditingWorkspace({ onNotice, onCreate }: { onNotice: (message: string) => void, onCreate: () => void }) {
  const [selectedAsset, setSelectedAsset] = useState('产品主镜头.mov')
  const [clipCount, setClipCount] = useState(6)
  const assets = [
    ['产品主镜头.mov', '视频 · 00:08'],
    ['工艺特写.mov', '视频 · 00:05'],
    ['精度证据.png', '图片 · 已授权'],
    ['品牌旁白.wav', '音频 · 00:15'],
  ]
  const addClip = () => { setClipCount(value => value + 1); onNotice(`已将「${selectedAsset}」添加到时间线`) }
  return <div className="video-editing-workspace">
    <div className="editing-toolbar"><div><span className="section-label">EditTask · ED-2607-12</span><b>15 秒竖版产品广告</b><small>来源：策略 v2.4 · Creative v1.3</small></div><div><button className="secondary-button" onClick={() => onNotice('低清预览渲染已创建')}><Play size={14} fill="currentColor"/>预览</button><button className="primary-button" onClick={() => onNotice('1080×1920 导出任务已创建')}><Download size={15}/>导出</button></div></div>
    <div className="editing-shell">
      <aside className="editing-assets"><div className="surface-toolbar"><h3>素材箱</h3><button aria-label="上传素材" onClick={() => onNotice('素材上传队列已打开')}><Upload size={15}/></button></div><div className="asset-group"><span>本任务素材 · 4</span>{assets.map(([name, meta]) => <button key={name} className={selectedAsset === name ? 'active' : ''} onClick={() => setSelectedAsset(name)}><Film size={15}/><div><b>{name}</b><small>{meta}</small></div></button>)}</div><button className="secondary-button full" onClick={addClip}><Scissors size={15}/>加入时间线</button></aside>
      <section className="editing-center"><div className="editing-preview"><div className="preview-grid"/><div className="editing-safe-frame"><span>9:16</span><b>精度，先于承诺被看见。</b><small>WHITE PRECISION</small></div><button aria-label="播放剪辑预览" onClick={() => onNotice('正在播放当前时间线')}><Play size={18} fill="currentColor"/></button><time>00:06.8 / 00:15.0</time></div><div className="timeline-toolbar"><span>时间线 · v1.3</span><div><button aria-label="撤销编辑" onClick={() => onNotice('已撤销上一步编辑')}>撤销</button><button aria-label="保存时间线" onClick={() => onNotice('时间线 v1.4 已保存')}><Save size={14}/>保存</button></div></div><div className="editing-timeline">{[['视频', 'clip video-a'], ['叠加', 'clip overlay'], ['字幕', 'clip caption'], ['配音', 'clip voice'], ['音乐', 'clip music']].map(([track, className], index) => <div className="timeline-row" key={track}><span>{index === 2 ? <Subtitles size={14}/> : index > 2 ? <Volume2 size={14}/> : <Film size={14}/>} {track}</span><div className="timeline-lane"><button className={className} onClick={() => onNotice(`${track}轨道已选中`)}>{index === 0 ? `${clipCount} 个镜头 · 00:15` : index === 2 ? '精度，先于承诺被看见。' : index === 3 ? '品牌旁白' : index === 4 ? <><Music2 size={13}/>品牌节奏</> : '产品卖点与品牌标识'}</button></div></div>)}</div></section>
      <aside className="editing-inspector"><div className="surface-toolbar"><h3>属性与检查</h3><span className="status success"><span/>通过</span></div><div className="inspector-section"><span>画面规格</span><b>1080 × 1920 · 9:16</b><small>抖音 / 快手信息流</small></div><div className="inspector-section"><span>当前选择</span><b>{selectedAsset}</b><small>允许商用 · 许可已记录</small></div><div className="editing-checks"><span><Check size={14}/>安全区未遮挡</span><span><Check size={14}/>字幕静音可理解</span><span><Check size={14}/>音乐与旁白无冲突</span><span><Check size={14}/>品牌检查通过</span></div><button className="secondary-button full" onClick={onCreate}><Video size={15}/>新建 EditTask</button></aside>
    </div>
  </div>
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
