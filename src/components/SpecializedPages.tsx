import { useEffect, useMemo, useState } from 'react'
import { ArrowRight, Check, ChevronDown, CircleAlert, CircleCheck, ClipboardCheck, Download, ExternalLink, FileText, Film, Image, Music2, Play, RotateCcw, Save, Scissors, Send, ShieldCheck, Sparkles, Subtitles, ThumbsDown, ThumbsUp, Video, Volume2, WandSparkles } from 'lucide-react'
import { useProject } from '../context/ProjectContext'
import { useModelConfig } from '../context/ModelConfigContext'
import { commerceHookTemplates, commerceTemplateApiId, guerlainPromptCopy, hookStoryboard } from '../data/commerceHooks'
import { api, buildHitAnalysisInput, buildLocalHitAnalysis, buildVideoReplicationPrompt, type ApiAdAccountBinding, type ApiAgencyWorkbench, type ApiArtifact, type ApiAssetFeature, type ApiAssetVersionPointer, type ApiCreativeSourceOption, type ApiGenerationJob, type ApiHitAnalysis, type ApiMaterialConfirmation, type ApiPreparedCommercePreroll, type ApiPrerollScope, type ApiQualityReport, type ApiRemixRenderJob, type ApiShortDramaPrerollCandidate, type ApiShortDramaPrerollPlan, type ApiShortDramaStoryContext, type ApiViralRemakeWorkspace, type ApiVideoPromptDimension, type ApiVideoReplicationPrompt } from '../data/api'
import type { ArtifactKey, BusinessTaskType, DataState } from '../types'
import { deliveryApi, type DeliveryChangeSet } from '../api/delivery'
import { StateBoundary } from './StateBoundary'

export function ArtifactFlow({ compact = false }: { compact?: boolean }) {
  const { currentProject } = useProject()
  const order: ArtifactKey[] = ['brief', 'strategy', 'creative', 'insight', 'delivery']
  return <div className={compact ? 'artifact-flow compact' : 'artifact-flow'} aria-label="Project 产物链路">{order.map((key, index) => { const artifact = currentProject.artifacts[key]; return <div className="artifact-node" key={key}><span>{String(index + 1).padStart(2, '0')}</span><div><b>{artifact.label} {artifact.version}</b><small>{artifact.status} · {artifact.owner}</small><small>{artifact.sourceVersion ?? `更新于 ${artifact.updatedAt}`}</small></div>{index < order.length - 1 ? <ArrowRight size={14}/> : null}</div> })}</div>
}

export function ImageTextCreationPage({ state, activeTaskId }: { state: DataState, activeTaskId?: string }) {
  const { currentProject, reloadProjects, updateArtifact } = useProject()
  const { providers } = useModelConfig()
  const [selected, setSelected] = useState(0)
  const [channel, setChannel] = useState('小红书 4:5')
  const [headline, setHeadline] = useState('看得见的精度，兑现你的创新。')
  const [version, setVersion] = useState(8)
  const [notice, setNotice] = useState('')
  const [job, setJob] = useState<ApiGenerationJob | null>(null)
  const [confirmedBriefId, setConfirmedBriefId] = useState('')
  const activeTask = currentProject.tasks.find(task => task.id === activeTaskId)
  const pages = ['封面主张', '精度证据', '制造场景', '行动引导']
  const configuredProvider = providers.find(provider => provider.status === '已配置')
  const save = async () => {
    const nextVersion = `v1.${version + 1}`
    try {
      await updateArtifact('creative', { version: nextVersion, status: '制作中', sourceVersion: `策略 ${currentProject.artifacts.strategy.version}`, summary: `${channel} 图文 4 页，品牌检查通过` })
      setVersion(value => value + 1)
      setNotice(`已保存为 ${nextVersion}`)
    } catch (cause) {
      setNotice(cause instanceof Error ? cause.message : '保存创意版本失败，请重试。')
    }
  }
  useEffect(() => {
    let active = true
    void Promise.all([api.listArtifacts(currentProject.id), api.listJobs(currentProject.id)]).then(([artifacts, jobs]) => {
      const latest = jobs.filter(candidate => candidate.artifactKind === 'image').at(-1)
      const brief = artifacts.filter(artifact => artifact.kind === 'brief' && artifact.status === 'ready').at(-1)
      if (active) {
        setJob(latest ?? null)
        setConfirmedBriefId(brief?.id ?? '')
      }
    }).catch(() => undefined)
    return () => { active = false }
  }, [currentProject.id])
  useEffect(() => {
    if (!job || !['queued', 'running'].includes(job.status)) return
    const timer = window.setInterval(() => {
      void api.getJob(job.id).then(next => {
        setJob(next)
        if (next.status === 'succeeded') {
          void reloadProjects()
          setNotice('图片生成完成，稳定资产已关联到当前 Project。')
        }
      }).catch(cause => setNotice(cause instanceof Error ? cause.message : '任务状态读取失败'))
    }, 1500)
    return () => window.clearInterval(timer)
  }, [job, updateArtifact])
  const generateImage = async () => {
    const prompt = `${headline}。${channel}，工业制造品牌图文主视觉，产品精度证据，品牌安全区清晰，中文排版。`
    if (!confirmedBriefId) {
      setNotice('请先在需求中心确认 Brief，再生成当前主视觉。')
      return
    }
    try {
      const next = await api.createMedia(currentProject.id, 'image', prompt, confirmedBriefId)
      setJob(next)
      setNotice(next.status === 'succeeded' ? '图片生成完成，资产已保存。' : '图片生成任务已创建，正在轮询。')
    } catch (cause) {
      setNotice(cause instanceof Error ? cause.message : '创建图片生成任务失败，请重试。')
    }
  }
  return <StateBoundary state={state} onRetry={() => setNotice('已重新加载')} onCreate={() => setNotice('已创建空白画板')}><div className="image-editor-specialized">
    <aside className="creative-structure"><div className="surface-toolbar"><h3>图文结构</h3><button aria-label="新增图文页面"><Image size={16}/></button></div>{pages.map((page, index) => <button key={page} className={selected === index ? 'creative-page active' : 'creative-page'} onClick={() => setSelected(index)}><span>{String(index + 1).padStart(2, '0')}</span><b>{page}</b><small>{index === 0 ? '主视觉' : index === 3 ? 'CTA' : '内容页'}</small></button>)}<div className="version-block"><span>来源</span><b>{currentProject.artifacts.strategy.version}</b><small>{currentProject.artifacts.strategy.summary}</small></div></aside>
    <section className="image-canvas-workspace"><div className="canvas-toolbar light"><span>{activeTask ? `${activeTask.name} · 图文 v1.${version}` : `${currentProject.name} · 图文 v1.${version}`}</span><div><button onClick={() => setNotice('预览链接已生成')}><ExternalLink size={14}/>预览</button><button onClick={() => setNotice('PNG 导出任务已创建')}><Download size={14}/>导出</button></div></div>{activeTask ? <div className="creative-task-banner"><span>统一创意任务入口</span><b>{activeTask.name}</b><small>{activeTask.objective}</small></div> : null}<div className="portrait-stage"><div className="social-poster"><img src="/assets/white-precision-cnc.png" alt="CNC 设备加工高精度金属零件"/><div className="poster-copy"><small>WHITE PRECISION</small><h2>{headline}</h2><p>±0.01mm 精度 · 98%+ 准时交付</p></div><span className="poster-index">0{selected + 1} / 04</span></div></div><div className="page-strip">{pages.map((page, index) => <button key={page} className={selected === index ? 'active' : ''} onClick={() => setSelected(index)}><span>{index + 1}</span>{page}</button>)}</div></section>
    <aside className="creative-inspector"><div className="surface-toolbar"><h3>页面属性</h3><span className="status success"><span/>品牌检查通过</span></div><label>渠道与画幅<select value={channel} onChange={event => setChannel(event.target.value)}><option>小红书 4:5</option><option>公众号 16:9</option><option>信息流 1:1</option></select></label><label>主标题<textarea value={headline} onChange={event => setHeadline(event.target.value)} maxLength={24}/><small>{headline.length} / 24 字</small></label><div className="check-list"><span><Check size={14}/>安全区未遮挡</span><span><Check size={14}/>核心信息有证据</span><span><Check size={14}/>品牌用语一致</span></div>{!configuredProvider ? <div className="model-required"><CircleAlert size={15}/><span>服务端尚未配置 ARK_API_KEY，无法发起图片生成。</span></div> : null}{!confirmedBriefId ? <div className="model-required"><CircleAlert size={15}/><span>请先在需求中心确认 Brief，系统才会允许生成图片。</span></div> : null}<button className="primary-button full" disabled={!configuredProvider || !confirmedBriefId || ['queued', 'running'].includes(job?.status ?? '')} onClick={() => void generateImage()}><WandSparkles size={15}/>{job && ['queued', 'running'].includes(job.status) ? '图片生成中…' : '生成当前主视觉'}</button><button className="secondary-button full" onClick={save}><Save size={15}/>保存新版本</button>{job ? <div className="inline-notice" role="status">任务 {job.id.slice(0, 8)} · {job.status} · {job.model ?? '模型待分配'}{job.diagnostic ? ` · ${job.diagnostic}` : ''}</div> : null}{notice ? <div className="inline-notice" role="status">{notice}</div> : null}</aside>
  </div></StateBoundary>
}

const performanceModes = [
  { id: 'short-drama', label: '短剧前贴', detail: '用人物冲突、风险升级和结果反转，在 6 秒内建立继续观看的理由。', guard: '人物连续性与静音可理解' },
  { id: 'game', label: '游戏前贴', detail: '用可读目标、失败瞬间和即时反馈建立挑战感，再衔接产品或正片。', guard: '玩法真实性与结果可读性' },
  { id: 'pre-roll', label: '电商前贴', detail: '为商品视频生成 4–10 秒高注意力开场并无缝拼接。', guard: '商品保真与静音可理解' },
  { id: 'viral-remake', label: '爆款复刻', detail: '拆解爆款结构与节奏，完成品牌映射和原创改写。', guard: '相似性与授权检查' },
]

const preRollPresets = {
  'short-drama': {
    eyebrow: 'SHORT DRAMA HOOK',
    title: '“交期又要延期？”——先把冲突推到观众面前。',
    detail: '00:00 采购负责人收到延期消息；00:02 切入精密加工现场；00:05 用 98%+ 准时交付完成反转。',
    source: '客户访谈 × 交期风险策略',
    shots: ['消息弹窗与人物停顿', '切入高速 CNC 现场', '交付数据与品牌定格'],
  },
  game: {
    eyebrow: 'GAMEPLAY HOOK',
    title: '±0.01mm 精度挑战：你能一次过关吗？',
    detail: '00:00 显示目标公差；00:02 第一次加工失败；00:04 参数修正并成功过关；00:06 衔接真实制造画面。',
    source: '挑战机制 × 精度证据策略',
    shots: ['展示公差挑战目标', '失败反馈与进度掉落', '修正参数、一击过关'],
  },
}

const brandSteps = [
  ['01', '解析 Brief', '提取品牌主张、受众、边界与交付目标。'],
  ['02', '编写剧本', '形成叙事主线、分镜、台词和声音设计。'],
  ['03', '生成资产', '按镜头生成画面、角色、配音、音乐与图形资产。'],
  ['04', '生成广告', '基于已确认资产和剧本生成可预览的广告草稿。'],
  ['05', '剪辑与交付', '完成多轨剪辑、品牌检查、导出和版本归档。'],
]

export function VideoCreationPage({ state, activeView, activeTaskId, onOpenTask }: { state: DataState, activeView: string, activeTaskId?: string, onOpenTask: (id: string) => void }) {
  const { currentProject, createTask } = useProject()
  const [selected, setSelected] = useState('short-drama')
  const [notice, setNotice] = useState('')
  const [brandGenerated, setBrandGenerated] = useState(false)
  const [brandStage, setBrandStage] = useState(0)
  const activeTask = currentProject.tasks.find(task => task.id === activeTaskId)
  const category = activeView === '品牌广告' ? 'brand' : activeView === '素材剪辑' ? 'editing' : 'performance'
  const activeMode = performanceModes.find(item => item.id === selected) ?? performanceModes[0]
  useEffect(() => {
    if (!activeTask) return
    const modeByType: Partial<Record<BusinessTaskType, string>> = {
      short_drama_preroll: 'short-drama',
      game_preroll: 'game',
      commerce_preroll: 'pre-roll',
      viral_remake: 'viral-remake',
      video: 'short-drama',
    }
    const nextMode = modeByType[activeTask.type]
    if (nextMode) setSelected(nextMode)
  }, [activeTask])
  const create = async () => {
    const name = category === 'performance' ? activeMode.label : category === 'brand' ? '品牌广告' : '素材剪辑 EditTask'
    const type: BusinessTaskType = category === 'brand' ? 'brand_video'
      : category === 'editing' ? 'video_edit'
      : activeMode.id === 'short-drama' ? 'short_drama_preroll'
      : activeMode.id === 'game' ? 'game_preroll'
      : activeMode.id === 'pre-roll' ? 'commerce_preroll'
      : 'viral_remake'
    try {
      const task = await createTask({
        type,
        name: `${currentProject.name} · ${name}`,
        objective: `${currentProject.goal}；继承策略 ${currentProject.artifacts.strategy.version} 与品牌约束。`,
      })
      setNotice(`${name}创作任务已写入服务端；已保留 Project、策略、来源与版本链。`)
      onOpenTask(task.id)
    } catch (cause) {
      setNotice(cause instanceof Error ? cause.message : '创建创作任务失败，请重试。')
    }
  }
  const title = category === 'performance' ? '效果广告，以可测试的转化表达组织创作。' : category === 'brand' ? '品牌广告，从 Brief 到剪辑交付形成完整叙事。' : '素材剪辑，将已授权素材组织为可交付的视频版本。'
  const description = category === 'performance' ? '选择一种生成类型，系统会继承策略、品牌规则、渠道规格与来源授权。' : category === 'brand' ? '沿着 Brief、剧本、资产、广告生成和剪辑的固定路径推进，所有产物均保留来源与确认记录。' : '独立 EditTask 可从品牌、效果任务或存量项目素材进入；字幕、音频与转场在编辑器内完成。'
  return <StateBoundary state={state} onRetry={() => setNotice('创作配置已重新加载')} onCreate={() => { void create() }}><section className="video-creation-workspace">
    <header className="video-workspace-header"><div><span className="section-label">视频创作 · {activeView}</span><h2>{title}</h2><p>{description}</p>{activeTask ? <div className="creative-task-banner compact"><span>统一创意任务入口</span><b>{activeTask.name}</b><small>{activeTask.objective}</small></div> : null}</div>{category !== 'editing' ? <button className="primary-button" onClick={() => void create()}><Video size={16}/>新建{category === 'performance' ? activeMode.label : '品牌广告'}</button> : null}</header>
    {category === 'performance' ? <><div className="performance-mode-tabs" role="tablist" aria-label="效果广告生成类型">{performanceModes.map(mode => <button key={mode.id} role="tab" aria-selected={selected === mode.id} className={selected === mode.id ? 'active' : ''} onClick={() => { setSelected(mode.id); setNotice('') }}><b>{mode.label}</b><small>{mode.guard}</small></button>)}</div>{selected === 'pre-roll' ? <CommerceHookWorkspace onNotice={setNotice}/> : selected === 'short-drama' || selected === 'game' ? <PreRollWorkspace key={selected} mode={selected} onNotice={setNotice}/> : selected === 'viral-remake' ? <ViralRemixWorkspace onNotice={setNotice}/> : <div className="performance-workflow">
      <aside className="performance-mode-list"><span className="section-label">当前生成类型</span><div className="mode-summary"><b>{activeMode.label}</b><p>{activeMode.detail}</p></div><span className="section-label">创建前检查</span>{['策略版本与证据', '品牌规则与禁用词', '渠道规格与转化目标', '素材、声音与参考授权'].map(item => <span className="mode-check" key={item}><Check size={14}/>{item}</span>)}</aside>
      <section className="performance-detail"><div className="video-preview"><div className="preview-grid"/><span>00:00 / 00:15</span><button aria-label="播放视频预览"><Play size={17} fill="currentColor"/></button></div><div className="performance-copy"><span className="section-label">当前路径</span><h3>{activeMode.label}</h3><p>{activeMode.detail}</p><div className="workflow-meta"><span><b>输入</b>已批准策略、渠道规格、授权素材</span><span><b>核心护栏</b>{activeMode.guard}</span></div></div></section>
      <aside className="video-job-rail"><span className="section-label">创建任务</span><h3>沿用 Project 上下文</h3>{['策略版本与证据', '品牌规则与禁用词', '渠道规格与转化目标', '素材、声音与参考授权'].map(item => <span key={item}><Check size={14}/>{item}</span>)}<button className="secondary-button full" onClick={() => setNotice('来源与授权清单已打开')}>查看来源与授权</button></aside>
    </div>}</> : category === 'brand' ? <div className="brand-workflow">
      <div className="brand-brief-card"><span className="section-label">BRIEF → BRAND FILM</span><h3>{currentProject.artifacts.brief.version} · {currentProject.artifacts.brief.status}</h3><p>{currentProject.artifacts.brief.summary}</p><div><Sparkles size={17}/><span>核心主张：看得见的精度，兑现你的创新。所有镜头保留 Brief、证据与版本来源。</span></div><button className="primary-button full" onClick={() => { setBrandGenerated(true); setBrandStage(1); setNotice('品牌广告方案已从已确认 Brief 解析完成') }}><WandSparkles size={15}/>解析 Brief 并生成方案</button></div>
      <section className="brand-generation-panel">
        <div className="brand-generation-heading"><div><span className="section-label">品牌广告生成方案</span><h3>{brandGenerated ? '《精度，先于承诺被看见》' : '等待解析 Brief'}</h3></div>{brandGenerated ? <span className="status success"><span/>方案就绪</span> : null}</div>
        {brandGenerated ? <><ol>{brandSteps.map(([id, stepTitle, detail], index) => <li className={brandStage === index + 1 ? 'active' : ''} key={id}><button onClick={() => setBrandStage(index + 1)}><span>{id}</span><div><b>{stepTitle}</b><p>{detail}</p></div>{index < brandSteps.length - 1 ? <ArrowRight size={16}/> : <WandSparkles size={17}/>}</button></li>)}</ol><div className="brand-output-card"><div className="brand-film-frame"><Play size={19} fill="currentColor"/><span>00:30 · 16:9</span></div><div><small>当前阶段 0{brandStage} / 05</small><b>{brandSteps[Math.max(0, brandStage - 1)][1]}</b><p>冷白工业光影、微距切削镜头与真实应用场景，结尾用 ±0.01mm 和 98%+ 准时交付完成品牌证明。</p><button className="secondary-button" onClick={() => setNotice('品牌广告预览已打开，当前为路演样片')}>预览品牌广告</button><button className="primary-button" onClick={() => void create()}>保存为创意任务</button></div></div></> : <div className="panel-empty">点击“解析 Brief 并生成方案”后展示剧本、分镜与品牌广告样片。</div>}
      </section>
    </div> : <VideoEditingWorkspace onNotice={setNotice} onCreate={() => { void create() }}/>}
    {notice ? <div className="inline-notice" role="status">{notice}</div> : null}
  </section></StateBoundary>
}

const viralDimensionLabels: Record<ApiVideoPromptDimension['id'], string> = {
  task_goal_type: '任务目标类型',
  quality_style_lighting: '画质&风格&光影规范',
  environment_atmosphere: '环境氛围',
  camera_content: '镜头画面内容',
  music_sound: '音乐&音效',
}

function promptFromViralWorkspace(
  workspace: ApiViralRemakeWorkspace,
  sourceTitle: string,
  sourceFileName: string,
  referenceImageName: string,
): ApiVideoReplicationPrompt | null {
  const viral = workspace.video_draft.viral_remake
  const promptDraft = viral.prompt_draft
  if (!promptDraft) return null
  const evidence = new Map(
    (viral.analysis_snapshot?.dimensions ?? []).map(dimension => [
      dimension.id,
      dimension.evidence_refs.length > 0
        ? dimension.evidence_refs.join('；')
        : `Seed-2-pro 置信度 ${Math.round(dimension.confidence * 100)}%`,
    ]),
  )
  return {
    source_asset: viral.input_snapshot.reference_video,
    source_title: sourceTitle,
    source_file_name: sourceFileName || undefined,
    reference_image_name: referenceImageName || undefined,
    user_instruction: viral.input_snapshot.user_instruction,
    dimensions: Object.entries(promptDraft.dimensions).map(([id, prompt]) => ({
      id: id as ApiVideoPromptDimension['id'],
      label: viralDimensionLabels[id as ApiVideoPromptDimension['id']],
      prompt,
      evidence: evidence.get(id as ApiVideoPromptDimension['id']) ?? '来自已持久化的分析快照',
    })),
    composite_prompt: promptDraft.composite_prompt,
    model_directive: '只复用抽象节奏、镜头功能与转化结构，替换原片人物、商标、字幕、音乐和受保护表达',
  }
}

function ViralRemixWorkspace({ onNotice }: { onNotice: (message: string) => void }) {
  const { currentProject, reloadProjects } = useProject()
  const { providers } = useModelConfig()
  const [sourceAssetId, setSourceAssetId] = useState('source_video')
  const [sourceVersion, setSourceVersion] = useState(1)
  const [sourceTitle, setSourceTitle] = useState('30 秒爆款结构样本')
  const [durationSeconds, setDurationSeconds] = useState(30)
  const [sourceFileName, setSourceFileName] = useState('')
  const [sourcePreviewUrl, setSourcePreviewUrl] = useState('')
  const [referenceImageName, setReferenceImageName] = useState('')
  const [referenceImagePreviewUrl, setReferenceImagePreviewUrl] = useState('')
  const [referenceImageAsset, setReferenceImageAsset] = useState<{ asset_id: string; version: number } | undefined>()
  const [userInstruction, setUserInstruction] = useState('保留原视频的强停留节奏，但改写为当前产品的原创广告表达。')
  const [productName, setProductName] = useState(currentProject.product)
  const [sellingPoint, setSellingPoint] = useState('±0.01mm 精度')
  const [secondSellingPoint, setSecondSellingPoint] = useState('98% 准时交付')
  const [cta, setCta] = useState('预约获取打样方案')
  const [viralWorkspace, setViralWorkspace] = useState<ApiViralRemakeWorkspace | null>(null)
  const [analysis, setAnalysis] = useState<ApiHitAnalysis | null>(null)
  const [replicationPrompt, setReplicationPrompt] = useState<ApiVideoReplicationPrompt | null>(null)
  const [job, setJob] = useState<ApiGenerationJob | null>(null)
  const [confirmedBriefId, setConfirmedBriefId] = useState('')
  const [viralTaskId, setViralTaskId] = useState('')
  const [generationReady, setGenerationReady] = useState(false)
  const [rightsConfirmed, setRightsConfirmed] = useState(false)
  const [uploadingSource, setUploadingSource] = useState(false)
  const [uploadingReference, setUploadingReference] = useState(false)
  const [busyStep, setBusyStep] = useState<'analysis' | 'generate' | ''>('')
  const configuredProvider = providers.find(provider => provider.status === '已配置')
  const isGenerating = job?.status === 'queued' || job?.status === 'running'
  const makeAsset = (assetId: string, version = 1) => ({ asset_id: assetId.trim(), version })
  useEffect(() => {
    let active = true
    void Promise.all([
      api.listArtifacts(currentProject.id),
      api.getLatestViralRemakeWorkspace(currentProject.id),
    ]).then(([artifacts, workspace]) => {
      if (!active) return
      setConfirmedBriefId(artifacts.filter(artifact => artifact.kind === 'brief' && artifact.status === 'ready').at(-1)?.id ?? '')
      if (!workspace) return
      const input = workspace.video_draft.viral_remake.input_snapshot
      setViralTaskId(workspace.task.id)
      setViralWorkspace(workspace)
      setGenerationReady(workspace.video_draft.viral_remake.readiness.generation_ready)
      setRightsConfirmed(
        input.reference_video_rights === 'confirmed'
        && (!input.reference_image || input.reference_image_rights === 'confirmed'),
      )
      setSourceAssetId(input.reference_video.asset_id)
      setSourceVersion(input.reference_video.version)
      setReferenceImageAsset(input.reference_image)
      setProductName(input.product_name)
      setSellingPoint(input.selling_points[0] ?? '')
      setSecondSellingPoint(input.selling_points[1] ?? '')
      setCta(input.call_to_action)
      setUserInstruction(input.user_instruction)
      setReplicationPrompt(promptFromViralWorkspace(workspace, input.reference_video.asset_id, '', ''))
      const latestCandidate = (workspace.video_draft.viral_remake.candidates ?? []).at(-1)
      if (latestCandidate) {
        void api.getViralVideoJob(currentProject.id, latestCandidate.provider_job_id).then(setJob).catch(() => undefined)
      }
      onNotice(`已恢复爆款复刻任务 ${workspace.task.id.slice(0, 8)}，素材引用和手工输入未丢失。`)
    }).catch(() => undefined)
    return () => { active = false }
  }, [currentProject.id, onNotice])
  useEffect(() => () => {
    if (sourcePreviewUrl) window.URL.revokeObjectURL(sourcePreviewUrl)
  }, [sourcePreviewUrl])
  useEffect(() => () => {
    if (referenceImagePreviewUrl) window.URL.revokeObjectURL(referenceImagePreviewUrl)
  }, [referenceImagePreviewUrl])
  useEffect(() => {
    if (!job || !isGenerating) return
    const timer = window.setInterval(() => {
      void api.getViralVideoJob(currentProject.id, job.id).then(next => {
        setJob(next)
        if (next.status === 'succeeded') {
          void reloadProjects()
          if (viralTaskId) {
            void api.getViralRemakeWorkspace(currentProject.id, viralTaskId).then(workspace => {
              setViralWorkspace(workspace)
              setGenerationReady(workspace.video_draft.viral_remake.readiness.generation_ready)
            }).catch(() => undefined)
          }
          onNotice('复刻视频生成完成，已作为新视频资产关联到当前 Project。')
        }
      }).catch(cause => onNotice(cause instanceof Error ? cause.message : '复刻视频任务状态读取失败。'))
    }, 1500)
    return () => window.clearInterval(timer)
  }, [currentProject.id, job, isGenerating, reloadProjects, onNotice, viralTaskId])
  const composePrompt = (prompt: ApiVideoReplicationPrompt, dimensions = prompt.dimensions) => [
    `源视频参考：${prompt.source_title}${prompt.source_file_name ? `（${prompt.source_file_name}）` : ''}，Asset ${prompt.source_asset.asset_id} v${prompt.source_asset.version}。`,
    `多模态输入：源视频用于复刻节奏、镜头功能和声音结构；${prompt.user_instruction ? `文本指令优先约束内容改写：${prompt.user_instruction}；` : ''}${prompt.reference_image_name ? `参考图片用于约束主体外观、产品形态、色彩或构图气质：${prompt.reference_image_name}；` : ''}`,
    ...dimensions.map(dimension => `【${dimension.label}】${dimension.prompt}`),
    '生成要求：视频参考负责节奏和镜头功能，图片参考负责主体视觉，文本指令负责内容改写和约束；三者冲突时以文本指令和版权安全为最高优先级。不复制原视频人物、商标、字幕、画面构图或受版权保护的表达。',
  ].join('\n')
  const handleSourceFile = async (file?: File) => {
    if (sourcePreviewUrl) window.URL.revokeObjectURL(sourcePreviewUrl)
    if (!file) {
      setSourceFileName('')
      setSourcePreviewUrl('')
      return
    }
    setSourceFileName(file.name)
    setSourceTitle(file.name.replace(/\.[^.]+$/, '') || sourceTitle)
    setSourcePreviewUrl(window.URL.createObjectURL(file))
    setUploadingSource(true)
    try {
      const ref = await api.uploadProjectAsset(currentProject.id, file)
      setSourceAssetId(ref.asset_id)
      setSourceVersion(ref.version)
      setViralTaskId('')
      setViralWorkspace(null)
      setReplicationPrompt(null)
      setJob(null)
      setGenerationReady(false)
      setRightsConfirmed(false)
      onNotice(`源视频 ${file.name} 已上传并固定为 Asset ${ref.asset_id} v${ref.version}。`)
    } catch (cause) {
      onNotice(cause instanceof Error ? cause.message : '源视频上传失败。')
    } finally {
      setUploadingSource(false)
    }
  }
  const handleReferenceImage = async (file?: File) => {
    if (referenceImagePreviewUrl) window.URL.revokeObjectURL(referenceImagePreviewUrl)
    if (!file) {
      setReferenceImageName('')
      setReferenceImagePreviewUrl('')
      return
    }
    setReferenceImageName(file.name)
    setReferenceImagePreviewUrl(window.URL.createObjectURL(file))
    setUploadingReference(true)
    try {
      const ref = await api.uploadProjectAsset(currentProject.id, file)
      setReferenceImageAsset(ref)
      setViralTaskId('')
      setViralWorkspace(null)
      setReplicationPrompt(null)
      setJob(null)
      setGenerationReady(false)
      setRightsConfirmed(false)
      onNotice(`参考图片 ${file.name} 已上传并固定为 Asset ${ref.asset_id} v${ref.version}。`)
    } catch (cause) {
      onNotice(cause instanceof Error ? cause.message : '参考图片上传失败。')
    } finally {
      setUploadingReference(false)
    }
  }
  const updateDimension = (id: ApiVideoPromptDimension['id'], promptText: string) => {
    setReplicationPrompt(current => {
      if (!current) return current
      const dimensions = current.dimensions.map(dimension => dimension.id === id ? { ...dimension, prompt: promptText } : dimension)
      return { ...current, dimensions, composite_prompt: composePrompt(current, dimensions) }
    })
  }
  const analyze = async () => {
    if (!sourceAssetId.trim() || sourceAssetId === 'source_video') {
      onNotice('请先上传爆款源视频，系统需要真实的 AssetVersionRef。')
      return
    }
    setBusyStep('analysis')
    try {
      let taskId = viralTaskId
      if (!taskId) {
        const workspace = await api.createManualViralRemakeWorkspace(currentProject.id, {
          sourceVideo: makeAsset(sourceAssetId, sourceVersion),
          referenceImage: referenceImageAsset,
          productName,
          sellingPoints: [sellingPoint, secondSellingPoint],
          callToAction: cta,
          userInstruction,
          objective: '复用高停留结构，生成当前产品的原创转化广告',
          audience: '当前 Project 的目标受众（手工输入，待 Strategy 接管）',
          coreMessage: [sellingPoint, secondSellingPoint].filter(Boolean).join('；'),
          durationSeconds,
        })
        taskId = workspace.task.id
        setViralTaskId(taskId)
        setViralWorkspace(workspace)
        setGenerationReady(workspace.video_draft.viral_remake.readiness.generation_ready)
        onNotice(`已创建并持久化爆款复刻任务 ${taskId.slice(0, 8)}，正在使用 Seed-2-pro 分析源视频。`)
      }
      const workspace = await api.analyzeViralRemake(currentProject.id, taskId)
      const prompt = promptFromViralWorkspace(workspace, sourceTitle, sourceFileName, referenceImageName)
      if (!prompt) throw new Error('分析已返回，但没有生成可编辑的五维提示词。')
      setViralWorkspace(workspace)
      setReplicationPrompt(prompt)
      setGenerationReady(workspace.video_draft.viral_remake.readiness.generation_ready)
      setJob(null)
      onNotice(`Seed-2-pro 已完成真实拆解并保存不可变分析快照，生成 ${prompt.dimensions.length} 个可编辑维度。`)
    } catch (cause) {
      onNotice(cause instanceof Error ? cause.message : '视觉理解拆解失败。')
    } finally {
      setBusyStep('')
    }
  }
  const generateReplica = async () => {
    if (!replicationPrompt || !viralWorkspace || !viralTaskId) {
      onNotice('请先完成五维视觉理解拆解。')
      return
    }
    if (!rightsConfirmed) {
      onNotice('请先确认源视频和参考图片具有可用于本次生成的授权。')
      return
    }
    if (!configuredProvider) {
      onNotice('服务端尚未配置视频生成模型，无法生成复刻视频。')
      return
    }
    setBusyStep('generate')
    try {
      const dimensions = Object.fromEntries(
        replicationPrompt.dimensions.map(dimension => [dimension.id, dimension.prompt.trim()]),
      ) as Record<ApiVideoPromptDimension['id'], string>
      const updated = await api.updateViralPrompt(
        currentProject.id,
        viralTaskId,
        viralWorkspace.video_draft.revision,
        dimensions,
      )
      const confirmed = await api.confirmViralGeneration(
        currentProject.id,
        viralTaskId,
        updated.video_draft.revision,
        Boolean(referenceImageAsset),
      )
      setViralWorkspace(confirmed)
      setGenerationReady(true)
      const created = await api.createViralVideoJob(currentProject.id, viralTaskId)
      setJob(created)
      onNotice(created.status === 'succeeded'
        ? '复刻视频生成完成，候选视频已进入项目素材库。'
        : '提示词与版权确认已冻结为 PromptPackage，Seedance 生成任务正在运行。')
    } catch (cause) {
      onNotice(cause instanceof Error ? cause.message : '创建复刻视频生成任务失败。')
    } finally {
      setBusyStep('')
    }
  }
  const latestCandidate = (viralWorkspace?.video_draft.viral_remake.candidates ?? []).at(-1)
  const submitCandidateReview = async () => {
    if (!viralTaskId || !latestCandidate) return
    setBusyStep('generate')
    try {
      const workspace = await api.submitViralCandidateReview(currentProject.id, viralTaskId, latestCandidate.id)
      setViralWorkspace(workspace)
      onNotice('候选视频已通过最小检查并提交评审，完整 Prompt 与素材血缘已保留。')
    } catch (cause) {
      onNotice(cause instanceof Error ? cause.message : '候选视频提交评审失败。')
    } finally {
      setBusyStep('')
    }
  }
  return <div className="viral-remake-lab">
    <aside className="viral-source-panel">
      <span className="section-label">多模态输入</span>
      <label className="viral-upload">上传爆款源视频<input type="file" accept="video/mp4" onChange={event => { void handleSourceFile(event.target.files?.[0]) }}/><small>{uploadingSource ? '正在上传并写入项目素材库…' : sourceFileName || '视频用于拆解节奏、镜头功能和声音结构。'}</small></label>
      <div className="viral-source-preview">{sourcePreviewUrl ? <video src={sourcePreviewUrl} controls muted aria-label="源视频预览"/> : <><Film size={24}/><span>等待源视频预览</span></>}</div>
      <label className="viral-upload image">上传参考图片<input type="file" accept="image/png,image/jpeg" onChange={event => { void handleReferenceImage(event.target.files?.[0]) }}/><small>{uploadingReference ? '正在上传并写入项目素材库…' : referenceImageName || '图片用于约束主体外观、产品形态和视觉风格。'}</small></label>
      {referenceImagePreviewUrl ? <div className="viral-image-preview"><img src={referenceImagePreviewUrl} alt="参考图片预览"/><span>{referenceImageName}</span></div> : null}
      <label>文本指令<textarea className="viral-text-instruction" value={userInstruction} onChange={event => setUserInstruction(event.target.value)} placeholder="例如：更年轻化，突出夏季户外场景，避免出现原视频人物和 Logo。"/></label>
      <label>源视频 Asset ID<input value={sourceAssetId} onChange={event => setSourceAssetId(event.target.value)}/></label>
      <label>源视频版本<input type="number" min={1} value={sourceVersion} onChange={event => setSourceVersion(Number(event.target.value))}/></label>
      <label>视频标题<input value={sourceTitle} onChange={event => setSourceTitle(event.target.value)}/></label>
      <label>时长（秒）<input type="number" min={9} max={180} value={durationSeconds} onChange={event => setDurationSeconds(Number(event.target.value))}/></label>
      <button className="primary-button full" disabled={busyStep === 'analysis' || uploadingSource || uploadingReference} onClick={() => void analyze()}><WandSparkles size={15}/>{busyStep === 'analysis' ? '保存任务与准备分析…' : '视觉理解拆解五维提示词'}</button>
    </aside>
    <section className="viral-dimension-panel">
      <div className="viral-dimension-hero"><div><span className="section-label">VLM PROMPT DNA</span><h3>{viralWorkspace?.video_draft.viral_remake.analysis_snapshot ? sourceTitle : '等待视觉理解模型拆解'}</h3><p>{viralWorkspace?.video_draft.viral_remake.analysis_snapshot ? '已将源视频拆为任务、画质风格、环境氛围、镜头内容和音乐音效五个可编辑提示词维度。' : '输入源视频后，系统会把爆款视频拆成可控的生成指令，再送入视频生成模型。'}</p></div><div><b>{replicationPrompt ? replicationPrompt.dimensions.length : 0}</b><small>Prompt 维度</small></div></div>
      <div className="viral-dimension-grid">{replicationPrompt ? replicationPrompt.dimensions.map(dimension => <article className="viral-dimension-card" key={dimension.id}><div><span>{dimension.label}</span><small>{dimension.evidence}</small></div><textarea aria-label={dimension.label} value={dimension.prompt} onChange={event => updateDimension(dimension.id, event.target.value)}/></article>) : ['任务目标类型', '画质&风格&光影规范', '环境氛围', '镜头画面内容', '音乐&音效'].map(label => <article className="viral-dimension-card empty" key={label}><div><span>{label}</span><small>等待模型输出</small></div><p>上传或填写源视频后点击拆解。</p></article>)}</div>
    </section>
    <aside className="viral-generation-panel">
      <span className="section-label">复刻视频生成</span>
      <label>目标产品<input value={productName} onChange={event => setProductName(event.target.value)}/></label>
      <label>卖点 1<input value={sellingPoint} onChange={event => setSellingPoint(event.target.value)}/></label>
      <label>卖点 2<input value={secondSellingPoint} onChange={event => setSecondSellingPoint(event.target.value)}/></label>
      <label>CTA<input value={cta} onChange={event => setCta(event.target.value)}/></label>
      {!configuredProvider ? <div className="model-required"><CircleAlert size={15}/><span>服务端尚未配置视频生成模型。</span></div> : null}
      {!confirmedBriefId ? <div className="model-required"><CircleAlert size={15}/><span>Strategy 尚未接线；当前任务明确使用 manual Intake，不伪造 Brief。</span></div> : null}
      {!generationReady ? <div className="model-required"><CircleAlert size={15}/><span>完成分析并确认提示词与素材授权后，系统会冻结 PromptPackage 再开始正式生成。</span></div> : null}
      <label className="viral-rights-confirmation"><input type="checkbox" checked={rightsConfirmed} onChange={event => setRightsConfirmed(event.target.checked)}/><span>我确认源视频及参考图片可用于本次分析与原创广告生成</span></label>
      <label>复刻视频总提示词<textarea className="viral-composite-prompt" readOnly value={replicationPrompt?.composite_prompt ?? '五维拆解完成后自动生成总提示词。'}/></label>
      <button className="primary-button full" disabled={!replicationPrompt || !rightsConfirmed || !configuredProvider || isGenerating || busyStep === 'generate'} onClick={() => void generateReplica()}><Video size={15}/>{isGenerating || busyStep === 'generate' ? '生成中…' : latestCandidate?.status === 'failed' ? '重试生成复刻视频' : '生成复刻视频'}</button>
      {job ? <div className="inline-notice" role="status">复刻任务 {job.id.slice(0, 8)} · {job.status} · {job.model ?? '模型待分配'}{job.diagnostic ? ` · ${job.diagnostic}` : ''}</div> : null}
      {latestCandidate ? <div className="viral-candidate-summary">
        <b>候选 {latestCandidate.id.slice(0, 8)} · {latestCandidate.status}</b>
        {latestCandidate.output_asset_ref ? <small>已入库 Asset {latestCandidate.output_asset_ref.asset_id} v{latestCandidate.output_asset_ref.version}</small> : null}
        {latestCandidate.checks.map(check => <small key={check.code}>{check.passed ? '✓' : '×'} {check.message}</small>)}
        {latestCandidate.error_message ? <small>{latestCandidate.error_code} · {latestCandidate.error_message}</small> : null}
        {latestCandidate.status === 'succeeded' && latestCandidate.checks.every(check => check.passed)
          ? <button className="secondary-button" disabled={busyStep === 'generate'} onClick={() => void submitCandidateReview()}>提交候选评审</button>
          : null}
      </div> : null}
      {replicationPrompt ? <div className="viral-safety-note"><ShieldCheck size={15}/><span>{replicationPrompt.model_directive}；只复刻结构与生成指令，不复制原片受保护表达。</span></div> : null}
    </aside>
  </div>
}

function PreRollWorkspace({ mode, onNotice }: { mode: 'short-drama' | 'game'; onNotice: (message: string) => void }) {
  const { currentProject, reloadProjects } = useProject()
  const { providers } = useModelConfig()
  const preset = preRollPresets[mode]
  const isShortDrama = mode === 'short-drama'
  const scope: ApiPrerollScope = {
    projectId: currentProject.id,
    purpose: 'preroll',
    prerollType: mode === 'short-drama' ? 'short_drama' : 'game',
  }
  const [selectedShot, setSelectedShot] = useState(0)
  const [job, setJob] = useState<ApiGenerationJob | null>(null)
  const [confirmedBriefId, setConfirmedBriefId] = useState('')
  const [hasPersistedAsset, setHasPersistedAsset] = useState(false)
  const [interactionFeedback, setInteractionFeedback] = useState('请选择一个镜头以更新中央预览。')
  const [storyContext, setStoryContext] = useState<ApiShortDramaStoryContext>({
    title: '',
    synopsis: '',
    reviewedSellingPoints: [''],
    openingLine: '',
  })
  const [plan, setPlan] = useState<ApiShortDramaPrerollPlan | null>(null)
  const [selectedCandidateId, setSelectedCandidateId] = useState('')
  const [isPlanning, setIsPlanning] = useState(false)
  const selectedCandidate = plan?.candidates.find(candidate => candidate.id === selectedCandidateId)
  const currentShot = isShortDrama ? selectedCandidate?.visualIntent ?? '请先生成并人工选择一个短剧前贴候选。' : preset.shots[selectedShot]
  const configuredProvider = providers.find(provider => provider.status === '已配置')
  const generated = job?.status === 'succeeded' && hasPersistedAsset
  const isGenerating = job?.status === 'queued' || job?.status === 'running'
  const selectShot = (index: number) => {
    setSelectedShot(index)
    setInteractionFeedback(`当前镜头：${String(index + 1).padStart(2, '0')} · ${preset.shots[index]}。`)
  }
  const moveShotFocus = (index: number, key: string) => {
    const nextIndex = key === 'Home' ? 0
      : key === 'End' ? preset.shots.length - 1
      : key === 'ArrowUp' || key === 'ArrowLeft' ? Math.max(0, index - 1)
      : Math.min(preset.shots.length - 1, index + 1)
    selectShot(nextIndex)
    document.getElementById(`preroll-shot-${mode}-${nextIndex}`)?.focus()
  }
  useEffect(() => {
    let active = true
    void Promise.all([
      api.listArtifacts(currentProject.id),
      api.listPrerollArtifacts(scope),
      api.listPrerollJobs(scope),
    ]).then(([artifacts, prerollArtifacts, jobs]) => {
      const latest = jobs.at(-1) ?? null
      const persisted = latest?.status === 'succeeded'
        && prerollArtifacts.some(artifact => artifact.id === latest.artifactId && artifact.kind === 'video' && artifact.status === 'ready')
      if (active) {
        setJob(latest)
        setConfirmedBriefId(artifacts.filter(artifact => artifact.kind === 'brief' && artifact.status === 'ready').at(-1)?.id ?? '')
        setHasPersistedAsset(persisted)
        const persistedShortDrama = isShortDrama
          ? prerollArtifacts.find(artifact => artifact.id === latest?.artifactId)?.shortDramaPreroll
          : undefined
        if (persistedShortDrama) {
          setStoryContext(context => ({ ...persistedShortDrama.storyContext, openingLine: context.openingLine }))
          setPlan({
            version: persistedShortDrama.planVersion,
            candidates: [persistedShortDrama.selectedCandidate],
          })
          setSelectedCandidateId(persistedShortDrama.selectedCandidate.id)
          setInteractionFeedback('已从服务端持久化产物恢复已选短剧候选与预览。')
        }
        if (latest?.status === 'succeeded' && !persisted) {
          setInteractionFeedback('任务已成功，但服务端产物尚未就绪；暂不能加入素材箱。')
        }
      }
    }).catch(cause => {
      if (active) setInteractionFeedback(cause instanceof Error ? cause.message : '无法读取服务端任务状态。')
    })
    return () => { active = false }
  }, [currentProject.id, isShortDrama, scope.prerollType])
  useEffect(() => {
    if (!job || !isGenerating) return
    const timer = window.setInterval(() => {
      void api.getPrerollJob(job.id, scope).then(async next => {
        setJob(next)
        if (next.status === 'succeeded') {
          const artifacts = await api.listPrerollArtifacts(scope)
          const persisted = artifacts.some(artifact => artifact.id === next.artifactId && artifact.kind === 'video' && artifact.status === 'ready')
          setHasPersistedAsset(persisted)
          if (persisted) {
            void reloadProjects()
            setInteractionFeedback('前贴分镜已生成且产物已持久化，可以加入混剪素材箱。')
            onNotice(`${mode === 'short-drama' ? '短剧' : '游戏'}前贴分镜已生成，稳定资产已关联到当前 Project。`)
          } else {
            setInteractionFeedback('任务已成功，但服务端产物尚未就绪；暂不能加入素材箱。')
          }
        } else if (next.status === 'failed' || next.status === 'cancelled') {
          setHasPersistedAsset(false)
          setInteractionFeedback(next.status === 'cancelled' ? '前贴分镜任务已取消，可以修改配置后重试。' : `前贴分镜生成失败${next.diagnostic ? `：${next.diagnostic}` : '，请重试。'}`)
        }
      }).catch(cause => setInteractionFeedback(cause instanceof Error ? cause.message : '任务状态读取失败。'))
    }, 1500)
    return () => window.clearInterval(timer)
  }, [isGenerating, job, mode, onNotice, reloadProjects, scope.projectId, scope.prerollType])
  const generateStoryboard = async () => {
    if (!configuredProvider) {
      setInteractionFeedback('服务端尚未配置 ARK_API_KEY，无法发起前贴分镜生成。')
      return
    }
    if (!confirmedBriefId) {
      setInteractionFeedback('请先在需求中心确认 Brief，再生成前贴分镜。')
      return
    }
    if (isShortDrama && (!plan || !selectedCandidate)) {
      setInteractionFeedback('请先从 AI 生成候选中明确选择一个短剧前贴方案，再创建视频任务。')
      return
    }
    // A retry never presents a prior successful asset as the pending request's result.
    setJob(null)
    setHasPersistedAsset(false)
    setInteractionFeedback('正在创建新的前贴分镜任务，旧结果不会用于本次生成。')
    try {
      let next: ApiGenerationJob
      if (isShortDrama) {
        if (!plan || !selectedCandidate) return
        next = await api.createShortDramaPrerollVideo(
          { ...scope, prerollType: 'short_drama' },
          confirmedBriefId,
          plan.version,
          selectedCandidate.id,
          normalizedStoryContext(),
        )
      } else {
        next = await api.createPrerollVideo(
          scope,
          `${preset.title}。${preset.detail}。分镜：${preset.shots.join('；')}。9:16 竖版，6 秒，静音可理解，品牌事实已校验，结尾保留稳定拼接点。`,
          confirmedBriefId,
        )
      }
      setJob(next)
      if (next.status === 'succeeded') {
        const artifacts = await api.listPrerollArtifacts(scope)
        const persisted = artifacts.some(artifact => artifact.id === next.artifactId && artifact.kind === 'video' && artifact.status === 'ready')
        setHasPersistedAsset(persisted)
        setInteractionFeedback(persisted
          ? '前贴分镜已生成且产物已持久化，可以加入混剪素材箱。'
          : '任务已成功，但服务端产物尚未就绪；暂不能加入素材箱。')
      } else {
        setHasPersistedAsset(false)
        setInteractionFeedback('前贴分镜任务已创建，正在查询服务端状态。')
      }
    } catch (cause) {
      setJob(null)
      setHasPersistedAsset(false)
      setInteractionFeedback(cause instanceof Error ? cause.message : '创建前贴分镜任务失败，请重试。')
    }
  }
  const cancelStoryboard = async () => {
    if (!job || !isGenerating) return
    try {
      setJob(await api.cancelJob(job.id, scope))
      setHasPersistedAsset(false)
      setInteractionFeedback('前贴分镜任务已取消，可以修改配置后重试。')
    } catch (cause) {
      setInteractionFeedback(cause instanceof Error ? cause.message : '取消前贴分镜任务失败，请重试。')
    }
  }
  const updateStoryContext = (field: keyof ApiShortDramaStoryContext, value: string) => {
    setStoryContext(context => ({ ...context, [field]: value }))
  }
  const normalizedStoryContext = (): ApiShortDramaStoryContext => {
    const openingLine = storyContext.openingLine?.trim()
    return {
      ...storyContext,
      reviewedSellingPoints: storyContext.reviewedSellingPoints.filter(value => value.trim()),
      openingLine: openingLine || undefined,
    }
  }
  const planShortDrama = async () => {
    if (!confirmedBriefId) {
      setInteractionFeedback('请先在需求中心确认 Brief，系统才会允许规划短剧前贴候选。')
      return
    }
    setIsPlanning(true)
    try {
      const next = await api.planShortDramaPreroll(currentProject.id, confirmedBriefId, {
        ...normalizedStoryContext(),
      })
      setPlan(next)
      setSelectedCandidateId('')
      setInteractionFeedback('AI 候选已生成。请人工核对证据、口播与画面意图后明确选择一个候选。')
    } catch (cause) {
      setInteractionFeedback(cause instanceof Error ? cause.message : '短剧前贴候选规划失败。请检查故事上下文后重试。')
    } finally {
      setIsPlanning(false)
    }
  }
  return <div className="preroll-workspace">
    {isShortDrama ? <aside className="preroll-candidate-panel" aria-label="短剧前贴 AI 候选">
      <details open>
        <summary><span className="section-label">AI 候选</span><b>需人工选择</b><ChevronDown size={15}/></summary>
        <p>评分仅表示钩子机制相关性，不代表转化效果预测。</p>
        {!plan ? <div className="preroll-candidate-empty">填写故事上下文并生成候选后，在此处完成人工选择。</div> : plan.candidates.map(candidate => <button type="button" key={candidate.id} className={selectedCandidateId === candidate.id ? 'active' : ''} aria-pressed={selectedCandidateId === candidate.id} onClick={() => {
          setSelectedCandidateId(candidate.id)
          setInteractionFeedback(`已人工选择${candidate.hookType}候选；中央预览已更新，可在确认后创建视频任务。`)
        }}><span><b>{candidate.hookType}</b><small>相关性 {candidate.score}</small></span><strong>{candidate.voiceover}</strong><small>{candidate.evidence.join(' ')}</small></button>)}
      </details>
    </aside> : <aside className="preroll-storyboard" aria-label="6 秒前贴分镜">
      <div className="surface-toolbar"><h3>镜头</h3><span>{generated ? 'v1.1' : '草稿'}</span></div>
      <p className="preroll-keyboard-hint">上下方向键切换镜头</p>
      {preset.shots.map((shot, index) => <button id={`preroll-shot-${mode}-${index}`} key={shot} className={selectedShot === index ? 'active' : ''} aria-current={selectedShot === index ? 'step' : undefined} onClick={() => selectShot(index)} onKeyDown={event => {
        if (['ArrowUp', 'ArrowDown', 'ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) {
          event.preventDefault()
          moveShotFocus(index, event.key)
        }
      }}><span>0{index + 1}</span><div><b>{shot}</b><small>00:0{index * 2}–00:0{index * 2 + 2} · {index === 2 ? '稳定拼接点' : '保持节奏推进'}</small></div><ArrowRight size={15}/></button>)}
    </aside>}
    <section className={`preroll-preview ${mode}`} aria-label="当前镜头预览">
      <div className="preroll-preview-header"><span className="section-label">当前镜头</span><b>{isShortDrama ? (selectedCandidate?.hookType ?? '待选择') : `0${selectedShot + 1} / 03`}</b><span>{generated ? '分镜已生成' : isGenerating ? '正在生成' : job?.status === 'failed' ? '生成失败' : job?.status === 'cancelled' ? '已取消' : '待生成'}</span></div>
      <div className="preroll-screen"><span>{isShortDrama ? 'SHORT DRAMA · HUMAN SELECTED' : preset.eyebrow}</span><h3>{currentShot}</h3><p>{isShortDrama ? selectedCandidate?.voiceover ?? '候选生成后，请从辅助面板选择一个已审核的钩子方案。' : preset.detail}</p><button aria-label={`播放${mode === 'short-drama' ? '短剧' : '游戏'}前贴预览`} disabled={!generated} onClick={() => onNotice(`${mode === 'short-drama' ? '短剧' : '游戏'}前贴预览正在播放：${currentShot}`)}><Play size={20} fill="currentColor"/></button><small>{isShortDrama ? selectedCandidate?.transitionLine ?? '等待人工选择后显示衔接语。' : `00:0${selectedShot * 2} / 00:06 · 9:16`}</small></div>
      <div className="preroll-source"><span className="section-label">策略来源</span><b>{isShortDrama ? (selectedCandidate ? `已选候选 · ${plan?.version}` : '等待人工选择') : preset.source}</b><small>已确认 Brief · 品牌规则通过 · 无真实平台写入</small></div>
      <p className="preroll-feedback" role="status" aria-live="polite">{interactionFeedback}</p>
    </section>
    <aside className="preroll-config">
      <span className="section-label">生成配置</span><h3>{mode === 'short-drama' ? '冲突反转型' : '挑战反馈型'}</h3>
      {isShortDrama ? <div className="short-drama-context">
        <label>短剧标题<input value={storyContext.title} onChange={event => updateStoryContext('title', event.target.value)} placeholder="已审核短剧标题"/></label>
        <label>故事梗概<textarea value={storyContext.synopsis} onChange={event => updateStoryContext('synopsis', event.target.value)} placeholder="至少 40 字，描述已审核的剧情上下文。"/></label>
        <label>已审核卖点<input value={storyContext.reviewedSellingPoints[0] ?? ''} onChange={event => setStoryContext(context => ({ ...context, reviewedSellingPoints: [event.target.value] }))} placeholder="至少一条已审核卖点"/></label>
        <label>正片首句（可选）<textarea value={storyContext.openingLine} onChange={event => updateStoryContext('openingLine', event.target.value)} placeholder="仅用于避免逐字复用，不会写入产物。"/></label>
        <button className="secondary-button full" disabled={!confirmedBriefId || isPlanning} aria-busy={isPlanning} onClick={() => void planShortDrama()}><Sparkles size={15}/>{isPlanning ? '正在规划候选…' : '生成 AI 候选'}</button>
      </div> : null}
      <label>正片衔接<select defaultValue="硬切"><option>硬切</option><option>动作匹配</option><option>音效桥接</option></select></label>
      <label>字幕样式<select defaultValue="高对比动态字幕"><option>高对比动态字幕</option><option>品牌极简字幕</option></select></label>
      <label>钩子强度<input aria-label="钩子强度" type="range" min="1" max="5" defaultValue="4"/></label>
      {['静音可理解', '品牌事实已校验', '人物与画面连续', '结尾存在稳定拼接点'].map(item => <span className="analysis-check" key={item}><Check size={14}/>{item}</span>)}
      {!configuredProvider ? <div className="model-required"><CircleAlert size={15}/><span>服务端尚未配置 ARK_API_KEY，无法发起前贴分镜生成。</span></div> : null}
      {!confirmedBriefId ? <div className="model-required"><CircleAlert size={15}/><span>请先在需求中心确认 Brief，系统才会允许生成前贴分镜。</span></div> : null}
      <button className="primary-button full" disabled={!configuredProvider || !confirmedBriefId || isGenerating || (isShortDrama && !selectedCandidate)} aria-busy={isGenerating} onClick={() => void generateStoryboard()}><WandSparkles size={15}/>{isGenerating ? '正在生成分镜…' : generated ? '重新生成前贴' : '生成前贴分镜'}</button>
      {isGenerating ? <button className="secondary-button full" onClick={() => void cancelStoryboard()}>取消生成</button> : null}
      <button className="secondary-button full" disabled={!generated} aria-describedby={!generated ? `preroll-export-hint-${mode}` : undefined} onClick={() => onNotice('前贴视频产物已持久化，可在素材剪辑中选择。')}>加入混剪素材箱</button>
      {!generated ? <small className="preroll-action-hint" id={`preroll-export-hint-${mode}`}>仅任务成功且服务端产物持久化后，才能加入混剪素材箱。</small> : null}
      {job ? <div className="inline-notice">任务 {job.id.slice(0, 8)} · {job.status} · {job.model ?? '模型待分配'}{job.diagnostic ? ` · ${job.diagnostic}` : ''}</div> : null}
    </aside>
  </div>
}

function CommerceHookWorkspace({ onNotice }: { onNotice: (message: string) => void }) {
  const { currentProject, reloadProjects, updateArtifact } = useProject()
  const { providers } = useModelConfig()
  const [selectedId, setSelectedId] = useState(commerceHookTemplates[0].id)
  const [fidelity, setFidelity] = useState(commerceHookTemplates[0].fidelity)
  const [camera, setCamera] = useState(commerceHookTemplates[0].camera)
  const [motion, setMotion] = useState(commerceHookTemplates[0].motion)
  const [environment, setEnvironment] = useState(commerceHookTemplates[0].environment)
  const [result, setResult] = useState(commerceHookTemplates[0].result)
  const [guardrails, setGuardrails] = useState(commerceHookTemplates[0].guardrails)
  const [previewing, setPreviewing] = useState(false)
  const [job, setJob] = useState<ApiGenerationJob | null>(null)
  const [generatedAsset, setGeneratedAsset] = useState<ApiArtifact | null>(null)
  const [sourceOptions, setSourceOptions] = useState<ApiCreativeSourceOption[]>([])
  const [selectedSourceKey, setSelectedSourceKey] = useState('fixture:guerlain')
  const [prepared, setPrepared] = useState<ApiPreparedCommercePreroll | null>(null)
  const [sourceNotice, setSourceNotice] = useState('')
  const [preparing, setPreparing] = useState(false)
  const selected = commerceHookTemplates.find(item => item.id === selectedId) ?? commerceHookTemplates[0]
  const configuredProvider = providers.find(provider => provider.status === '已配置')
  const selectedSource = sourceOptions.find(option =>
    `${option.source_ref.kind}:${option.source_ref.id}:${option.source_ref.version}` === selectedSourceKey,
  )
  const usingFixture = !selectedSource
  const selectedProductAsset = selectedSource?.product.product_asset_refs[0]
  const sourcePreview = selectedProductAsset
    ? `/platform/v1/projects/${encodeURIComponent(currentProject.id)}/assets/${encodeURIComponent(selectedProductAsset.asset_id)}/versions/${selectedProductAsset.version}/preview`
    : selected.image

  useEffect(() => {
    let active = true
    void Promise.all([
      api.listArtifacts(currentProject.id),
      api.listJobs(currentProject.id),
      api.listCommercePrerollSources(currentProject.id).catch(() => []),
    ]).then(([artifacts, jobs, sources]) => {
      const latest = jobs.filter(candidate => candidate.artifactKind === 'video').at(-1)
      const latestVideo = artifacts
        .filter(artifact => artifact.kind === 'video' && artifact.status === 'ready' && Boolean(artifact.sourceJobId))
        .at(-1)
      if (active) {
        setJob(latest ?? null)
        setGeneratedAsset(latestVideo ?? null)
        setSourceOptions(sources)
        const preferred = sources.find(source => source.preferred) ?? sources[0]
        setSelectedSourceKey(preferred
          ? `${preferred.source_ref.kind}:${preferred.source_ref.id}:${preferred.source_ref.version}`
          : 'fixture:guerlain')
      }
    }).catch(() => undefined)
    return () => { active = false }
  }, [currentProject.id])

  useEffect(() => {
    let active = true
    setPreviewing(false)
    setPrepared(null)
    if (!selectedSource) {
      const copy = guerlainPromptCopy(selected.id)
      setFidelity(copy.fidelity)
      setCamera(copy.camera)
      setMotion(copy.motion)
      setEnvironment(copy.environment)
      setResult(copy.result)
      setGuardrails(copy.guardrails)
      setSourceNotice('正在使用娇兰固定样例；接入已确认 Brief 后会由服务端动态编译。')
      return () => { active = false }
    }
    setPreparing(true)
    setSourceNotice('正在根据已确认来源编译提示词…')
    void api.prepareCommercePreroll(
      currentProject.id,
      selectedSource,
      commerceTemplateApiId(selected.id),
    ).then(value => {
      if (!active) return
      const timeline = value.plan.prompt.timeline
      setPrepared(value)
      setFidelity(value.plan.prompt.fidelity)
      setCamera(value.plan.prompt.camera)
      setEnvironment(value.plan.prompt.environment)
      setMotion(timeline.find(item => item.purpose === 'single_transformation')?.instruction ?? '')
      setResult(timeline.find(item => item.purpose === 'product_hold')?.instruction ?? '')
      setGuardrails(value.plan.prompt.guardrails.join('；'))
      setSourceNotice(value.readiness.generation_ready
        ? '提示词已根据所选来源编译，可以人工确认后生成。'
        : `提示词已生成，但正式生成仍缺少：${value.readiness.blockers.join('、')}`)
    }).catch(cause => {
      if (!active) return
      setSourceNotice(cause instanceof Error ? cause.message : '提示词编译失败。')
    }).finally(() => {
      if (active) setPreparing(false)
    })
    return () => { active = false }
  }, [currentProject.id, selected.id, selectedSource])

  const prompt = `${fidelity}\n${camera}\n${motion}\n${environment}\n${result}\n${guardrails}`
  const storyboard = prepared
    ? prepared.plan.prompt.timeline.map((segment, index) => ({
        time: `00:${segment.start_seconds.toFixed(1).padStart(4, '0')}–00:${segment.end_seconds.toFixed(1).padStart(4, '0')}`,
        name: hookStoryboard[index]?.name ?? `阶段 ${index + 1}`,
        detail: segment.instruction.replace(/^[^：]+：/, ''),
      }))
    : hookStoryboard
  useEffect(() => {
    if (!job || !['queued', 'running'].includes(job.status)) return
    const timer = window.setInterval(() => {
      void api.getViralVideoJob(currentProject.id, job.id).then(next => {
        setJob(next)
        if (next.status === 'succeeded') {
          void Promise.all([reloadProjects(), api.listArtifacts(currentProject.id)]).then(([, artifacts]) => {
            const asset = artifacts.find(candidate =>
              candidate.kind === 'video'
              && candidate.status === 'ready'
              && (candidate.id === next.artifactId || candidate.sourceJobId === next.id),
            )
            if (asset) setGeneratedAsset(asset)
            onNotice(`「${selected.name}」生成完成，已保存到素材库并进入素材检查队列。`)
          }).catch(cause => onNotice(cause instanceof Error ? cause.message : '生成资产读取失败'))
        }
      }).catch(cause => onNotice(cause instanceof Error ? cause.message : '任务状态读取失败'))
    }, 1500)
    return () => window.clearInterval(timer)
  }, [currentProject.id, job, onNotice, reloadProjects, selected.name])
  const save = async () => {
    try {
      await updateArtifact('creative', { status: '制作中', sourceVersion: `策略 ${currentProject.artifacts.strategy.version}`, summary: `广告前贴 · ${selected.name} · ${selected.frameStrategy}` })
      onNotice(`「${selected.name}」已保存为广告前贴策略草稿，并保留来源版本。`)
    } catch (cause) {
      onNotice(cause instanceof Error ? cause.message : '保存广告前贴策略失败，请重试。')
    }
  }
  const copyPrompt = async () => {
    try { await navigator.clipboard.writeText(prompt); onNotice('完整视频提示词已复制。') }
    catch { onNotice('提示词已准备好，请从右侧字段中复制。') }
  }
  const generate = async () => {
    if (selectedSource && !prepared) {
      onNotice('当前 Brief 的提示词还没有准备完成，请稍后再试。')
      return
    }
    if (selectedSource && !prepared?.readiness.generation_ready) {
      onNotice(`当前 Brief 还不能正式生成：${prepared?.readiness.blockers.join('、') || '缺少商品素材'}`)
      return
    }
    try {
      setPreparing(true)
      setGeneratedAsset(null)
      const sourceId = selectedSource?.source_ref.id ?? 'creative-video-intake-commerce-preroll-guerlain-v1'
      const productAsset = selectedSource?.product.product_asset_refs[0]
      const next = usingFixture && selected.id === 'window-reveal'
        ? await api.createCommercePrerollVideo(currentProject.id, prompt, sourceId)
        : productAsset
          ? await api.createPreparedCommercePrerollVideo(currentProject.id, prompt, sourceId, productAsset)
          : await api.createMedia(currentProject.id, 'video', prompt, sourceId)
      setJob(next)
      if (next.status === 'succeeded') {
        const artifacts = await api.listArtifacts(currentProject.id)
        setGeneratedAsset(artifacts.find(candidate =>
          candidate.kind === 'video'
          && candidate.status === 'ready'
          && (candidate.id === next.artifactId || candidate.sourceJobId === next.id),
        ) ?? null)
      }
      onNotice(next.status === 'succeeded' ? '视频生成完成，已进入素材库和素材检查。' : '视频生成任务已创建，正在轮询。')
    } catch (cause) {
      onNotice(cause instanceof Error ? cause.message : '创建视频生成任务失败。')
    } finally {
      setPreparing(false)
    }
  }
  return <div className="commerce-hook-workspace">
    <aside className="hook-template-rail">
      <div className="hook-rail-heading"><span className="section-label">场景策略库</span><b>电商前贴 / 钩子</b><small>学习资料 revision 399</small></div>
      {commerceHookTemplates.map((template, index) => <button key={template.id} className={selectedId === template.id ? 'active' : ''} onClick={() => setSelectedId(template.id)}><span>{String(index + 1).padStart(2, '0')}</span><div><b>{template.name}</b><small>{template.category} · {template.duration}</small></div></button>)}
      <a href="https://bytedance.larkoffice.com/wiki/H5uQwNji9iYH0TkNXaxcvFhUn2c" target="_blank" rel="noreferrer"><ExternalLink size={13}/>查看学习来源</a>
    </aside>
    <section className="hook-canvas">
      <div className="hook-canvas-toolbar"><div><span className="source-chip">{selected.frameStrategy}</span><b>{selected.name}</b>{generatedAsset ? <span className="source-chip ready">已进入素材检查</span> : null}</div><button onClick={copyPrompt}><ClipboardCheck size={14}/>复制提示词</button></div>
      <div className="hook-preview-stage">
        <div className={generatedAsset ? 'hook-phone-frame has-generated-video' : 'hook-phone-frame'}>
          {generatedAsset
            ? <video key={`${generatedAsset.id}-v${generatedAsset.version}`} controls playsInline preload="metadata" src={generatedAsset.content} aria-label={`${selected.name}生成视频`}/>
            : <><img src={sourcePreview} alt={`${selected.name}${selected.imageLabel}`}/><div className="hook-preview-shade"/><span className="hook-frame-label">{selectedProductAsset ? 'Brief 商品素材' : selected.imageLabel}</span><div className="hook-preview-copy"><small>ECOMMERCE HOOK · 9:16</small><b>{selected.hook}</b><span>{selected.duration} · 静音可理解</span></div><button aria-label={previewing ? '暂停钩子预览' : '播放钩子预览'} onClick={() => setPreviewing(value => !value)}><Play size={17} fill="currentColor"/></button></>}
        </div>
        <div className="hook-proof"><span className="section-label">策略依据</span><h3>先建立信息缺口，再完成一次清晰变化。</h3><p>一个主动作、一个结果状态、一个稳定的商品定格。环境只提供辅助运动。</p><div>{selected.tags.map(tag => <span key={tag}>{tag}</span>)}</div></div>
      </div>
      <div className="hook-storyboard">{storyboard.map((step, index) => <div key={step.time}><span>{step.time}</span><i/><b>{String(index + 1).padStart(2, '0')} · {step.name}</b><small>{step.detail}</small></div>)}</div>
    </section>
    <aside className="hook-inspector">
      <div className="surface-toolbar"><h3>提示词构建器</h3><span>{usingFixture ? '娇兰固定样例' : selectedSource?.status === 'approved' ? '策略包已批准' : 'Brief 已确认'}</span></div>
      <label>创意来源<select value={selectedSourceKey} onChange={event => setSelectedSourceKey(event.target.value)}>
        {sourceOptions.map(option => {
          const key = `${option.source_ref.kind}:${option.source_ref.id}:${option.source_ref.version}`
          const sourceType = option.source_ref.kind === 'strategy_package' ? '策略包' : 'Brief'
          return <option key={key} value={key}>{option.product.brand_name || '未命名品牌'} · {option.product.product_name || '未命名商品'} · {sourceType} v{option.source_ref.version}</option>
        })}
        <option value="fixture:guerlain">娇兰第三代黄金复原蜜 · 固定样例</option>
      </select></label>
      <label>商品保真约束<textarea value={fidelity} onChange={event => setFidelity(event.target.value)}/></label>
      <label>镜头与光影<textarea value={camera} onChange={event => setCamera(event.target.value)}/></label>
      <label>唯一主动作<textarea value={motion} onChange={event => setMotion(event.target.value)}/></label>
      <label>结果与停留<textarea value={result} onChange={event => setResult(event.target.value)}/></label>
      <div className="hook-guardrail"><ShieldCheck size={15}/><span><b>自动附加生成护栏</b><small>{guardrails}</small></span></div>
      {configuredProvider ? <div className="hook-model"><CircleCheck size={15}/><span><b>{configuredProvider.name}</b><small>服务端媒体模型目录</small></span></div> : <div className="hook-model missing"><CircleAlert size={15}/><span><b>尚未配置模型</b><small>请在服务端配置 ARK_API_KEY 后重新检查能力。</small></span></div>}
      {sourceNotice ? <div className={prepared && !prepared.readiness.generation_ready ? 'hook-model missing' : 'hook-model'}><CircleAlert size={15}/><span><b>来源与准备状态</b><small>{sourceNotice}</small></span></div> : null}
      <div className="hook-actions"><button className="secondary-button" onClick={() => void save()}><Save size={14}/>保存策略</button><button className="primary-button" disabled={!configuredProvider || preparing || Boolean(selectedSource && !prepared?.readiness.generation_ready) || ['queued', 'running'].includes(job?.status ?? '')} onClick={() => void generate()}><WandSparkles size={14}/>{preparing ? '准备素材…' : job && ['queued', 'running'].includes(job.status) ? '生成中…' : '生成视频'}</button></div>
      {job ? <div className="inline-notice" role="status">任务 {job.id.slice(0, 8)} · {job.status} · {job.model ?? '模型待分配'}{job.diagnostic ? ` · ${job.diagnostic}` : ''}</div> : null}
    </aside>
  </div>
}

function VideoEditingWorkspace({ onNotice, onCreate }: { onNotice: (message: string) => void, onCreate: () => void }) {
  const { currentProject } = useProject()
  const [assets, setAssets] = useState<ApiArtifact[]>([])
  const [assetFeatures, setAssetFeatures] = useState<ApiAssetFeature[]>([])
  const [selectedAssets, setSelectedAssets] = useState<string[]>([])
  const [previewAssetId, setPreviewAssetId] = useState<string>('')
  const [assetState, setAssetState] = useState<'loading' | 'ready' | 'error'>('loading')
  const [clipCount, setClipCount] = useState(0)
  const [packaging, setPackaging] = useState(['动态字幕', '品牌片尾'])
  const [renderPlanId, setRenderPlanId] = useState('')
  const [renderJob, setRenderJob] = useState<ApiRemixRenderJob | null>(null)
  const [qualityReport, setQualityReport] = useState<ApiQualityReport | null>(null)
  const [renderNotice, setRenderNotice] = useState('输入已保存的 RemixPlan ID 后，可创建持久化 RenderJob。')
  const [feedbackRating, setFeedbackRating] = useState(5)
  const [feedbackComment, setFeedbackComment] = useState('结构清晰，商品卖点表达完整。')
  const [feedbackNotice, setFeedbackNotice] = useState('反馈将以 append-only 事件写入，不会修改历史 RemixPlan 或 RenderJob。')
  useEffect(() => {
    let active = true
    setAssetState('loading')
    void Promise.all([api.listArtifacts(currentProject.id), api.listJobs(currentProject.id), api.listAssetFeatures(currentProject.id)]).then(([artifacts, jobs, features]) => {
      const succeededJobs = new Set(jobs.filter(job => job.status === 'succeeded').map(job => job.id))
      const nextAssets = artifacts.filter(artifact => (
        artifact.kind === 'video'
        && artifact.status === 'ready'
        && Boolean(artifact.sourceJobId)
        && succeededJobs.has(artifact.sourceJobId!)
      ))
      if (active) {
        setAssets(nextAssets)
        setAssetFeatures(features.items)
        setSelectedAssets(current => current.filter(id => nextAssets.some(asset => asset.id === id)))
        setAssetState('ready')
      }
    }).catch(() => {
      if (active) {
        setAssets([])
        setAssetFeatures([])
        setSelectedAssets([])
        setAssetState('error')
      }
    })
    return () => { active = false }
  }, [currentProject.id])
  const selectedAssetObjects = assets.filter(asset => selectedAssets.includes(asset.id))
  const activePreview = assets.find(asset => asset.id === previewAssetId) ?? selectedAssetObjects[0] ?? assets[0]
  const toggleAsset = (id: string) => {
    setPreviewAssetId(id)
    setSelectedAssets(current => current.includes(id) ? current.filter(item => item !== id) : [...current, id])
    onNotice('视频封面已进入播放预览，点击素材可加入或移出混剪队列。')
  }
  useEffect(() => {
    if (!renderJob || !['queued', 'running'].includes(renderJob.status)) return
    const timer = window.setInterval(() => {
      void api.getRemixRenderJob(currentProject.id, renderJob.id).then(next => {
        setRenderJob(next)
        if (next.quality_report_id) {
          void api.getRemixRenderJobQualityReport(currentProject.id, next.id).then(envelope => setQualityReport(envelope.quality_report))
        }
        if (next.status === 'succeeded') {
          setRenderNotice(next.output_asset ? '渲染完成，成片已回流素材库并生成血缘记录。' : '渲染完成，等待成片资产回流。')
        } else if (next.status === 'failed') {
          setRenderNotice(`渲染失败${next.error_message ? `：${next.error_message}` : '，请检查任务日志。'}`)
        } else if (next.status === 'requires_review') {
          setRenderNotice('渲染进入人工复核，请查看质量报告或诊断结果。')
        }
      }).catch(cause => setRenderNotice(cause instanceof Error ? cause.message : 'RenderJob 状态读取失败。'))
    }, 1500)
    return () => window.clearInterval(timer)
  }, [currentProject.id, renderJob])
  const togglePackaging = (name: string) => setPackaging(current => current.includes(name) ? current.filter(item => item !== name) : [...current, name])
  const addClip = () => { setClipCount(value => value + selectedAssets.length); onNotice(`已将 ${selectedAssets.length} 段已持久化视频加入混剪时间线`) }
  const createRenderJob = async () => {
    if (!renderPlanId.trim()) {
      setRenderNotice('请先填写已保存的 RemixPlan ID。')
      return
    }
    try {
      const key = `remix-render-${currentProject.id}-${renderPlanId.trim()}-${selectedAssets.slice().sort().join('-') || 'timeline'}`
      const next = await api.createRemixRenderJob(currentProject.id, {
        plan_id: renderPlanId.trim(),
        target_format: 'mp4',
        target_quality: 'standard',
      }, key)
      setRenderJob(next)
      setQualityReport(null)
      setRenderNotice(next.status === 'succeeded' && next.output_asset ? 'RenderJob 已完成，成片已回流素材库。' : 'RenderJob 已持久化，正在轮询状态。')
    } catch (cause) {
      setRenderNotice(cause instanceof Error ? cause.message : '创建 RenderJob 失败，请确认 RemixPlan 已保存。')
    }
  }
  const createQualityReport = async () => {
    if (!renderJob) {
      setRenderNotice('请先创建 RenderJob 后再执行质量检查。')
      return
    }
    try {
      const report = await api.createRemixQualityReport(currentProject.id, {
        render_job_id: renderJob.id,
        policy: 'fail_critical',
      })
      setQualityReport(report)
      const next = await api.getRemixRenderJob(currentProject.id, renderJob.id)
      setRenderJob(next)
      setRenderNotice(report.verdict === 'pass' ? '质量报告通过，成片可继续进入回流流程。' : `质量报告为 ${report.verdict}，已同步 RenderJob 状态。`)
    } catch (cause) {
      setRenderNotice(cause instanceof Error ? cause.message : '质量检查失败，请稍后重试。')
    }
  }
  const submitPlanFeedback = async () => {
    if (!renderPlanId.trim()) {
      setFeedbackNotice('请先填写 RemixPlan ID 后再提交计划反馈。')
      return
    }
    try {
      const event = await api.createRemixFeedbackEvent(currentProject.id, {
        event_type: 'rating',
        target_type: 'remix_plan',
        target_id: renderPlanId.trim(),
        rating: feedbackRating,
        comment: feedbackComment,
      })
      setFeedbackNotice(`计划反馈已记录为事件 ${event.id.slice(0, 8)}，历史计划保持不变。`)
    } catch (cause) {
      setFeedbackNotice(cause instanceof Error ? cause.message : '提交计划反馈失败。')
    }
  }
  const submitOutputFeedback = async () => {
    const output = renderJob?.output_asset
    if (!renderJob || !output) {
      setFeedbackNotice('RenderJob 生成成片资产后才能提交资产反馈。')
      return
    }
    try {
      await api.createRemixFeedbackEvent(currentProject.id, {
        event_type: 'render_succeeded',
        target_type: 'render_job',
        target_id: renderJob.id,
        asset_version: output.asset_version,
      })
      const event = await api.createRemixFeedbackEvent(currentProject.id, {
        event_type: 'rating',
        target_type: 'asset',
        target_id: String(output.asset_version.asset_id),
        asset_version: output.asset_version,
        rating: feedbackRating,
        comment: feedbackComment,
      })
      const snapshot = await api.createPlannerWeightSnapshot(currentProject.id)
      setFeedbackNotice(`成片反馈 ${event.id.slice(0, 8)} 已写入，Planner 权重快照包含 ${snapshot.asset_weights.length} 个素材。`)
    } catch (cause) {
      setFeedbackNotice(cause instanceof Error ? cause.message : '提交成片反馈失败。')
    }
  }
  const titleForAsset = (asset: ApiArtifact) => asset.shortDramaPreroll?.selectedCandidate.voiceover ?? asset.content
  const labelForAsset = (asset: ApiArtifact) => asset.prerollType === 'short_drama' ? '短剧前贴' : asset.prerollType === 'game' ? '游戏前贴' : asset.prerollType === 'commerce' ? '电商前贴' : '服务端视频'
  const genreTabs = ['推荐', '热榜', '榜单', '逆袭', '爱情', '剧情', '反转', '亲情', '悬疑', '喜剧']
  const renderBusy = renderJob?.status === 'queued' || renderJob?.status === 'running'
  const outputAssetLabel = renderJob?.output_asset ? `${renderJob.output_asset.asset_version.asset_id} v${renderJob.output_asset.asset_version.version}` : ''
  const outputPreviewURL = renderJob?.output_preview?.url ?? ''
  const provenanceLabel = renderJob?.provenance ? `血缘：Plan ${renderJob.provenance.plan_id.slice(0, 12)} · Render ${renderJob.provenance.render_job_id.slice(0, 12)} · 输入素材 ${renderJob.provenance.input_assets.length} 个` : ''
  return <div className="video-editing-workspace">
    <div className="editing-toolbar"><div><span className="section-label">EditTask · ED-2607-12</span><b>15 秒竖版产品广告</b><small>来源：策略 v2.4 · Creative v1.3</small></div><div><button className="secondary-button" onClick={() => onNotice('低清预览渲染已创建')}><Play size={14} fill="currentColor"/>预览</button><button className="primary-button" disabled={!selectedAssets.length || renderBusy} onClick={() => void createRenderJob()}><Download size={15}/>{renderBusy ? '导出中…' : '导出'}</button></div></div>
    <div className="editing-shell">
      <aside className="editing-assets video-asset-library"><div className="surface-toolbar"><h3>视频素材箱</h3><span>当前 Project</span></div><div className="video-platform-tabs" aria-label="短剧素材分类">{genreTabs.map((tab, index) => <span key={tab} className={index === 0 ? 'active' : ''}>{tab}</span>)}</div><div className="asset-group video-asset-stage"><span>选择参与混剪的视频 · {selectedAssets.length}/{assets.length}</span>{assetState === 'loading' ? <div className="panel-empty">正在加载服务端持久化资产…</div> : null}{assetState === 'error' ? <div className="panel-empty">素材箱加载失败，请刷新后重试。</div> : null}{assetState === 'ready' && !assets.length ? <div className="panel-empty">当前 Project 暂无可用于混剪的已持久化视频资产。</div> : null}<div className="asset-card-flow">{assets.map((asset, index) => { const selected = selectedAssets.includes(asset.id); const previewing = activePreview?.id === asset.id; const feature = featureForVideoAsset(asset, assetFeatures); return <button key={asset.id} className={`video-asset-card poster-${index % 6}${selected ? ' active' : ''}${previewing ? ' previewing' : ''}`} onMouseEnter={() => setPreviewAssetId(asset.id)} onFocus={() => setPreviewAssetId(asset.id)} onClick={() => toggleAsset(asset.id)} aria-pressed={selected}><span className="video-poster-frame"><span className="poster-glow"/><span className="poster-cast"><span/><span/><span/></span><span className="poster-play"><Play size={13} fill="currentColor"/></span><b>{titleForAsset(asset)}</b><small>{labelForAsset(asset)} · {58 + index * 7}集</small><em>{feature ? `Hook ${featurePercent(feature.hookStrength)} · ${riskText(feature.similarityRisk)}` : previewing ? 'AUTO PREVIEW' : `${(5.2 + index * .6).toFixed(1)}亿`}</em></span><span className="video-card-meta"><span className="asset-check">{selected ? <Check size={12}/> : null}</span><span><b>{labelForAsset(asset)}视频</b><small>{feature ? videoFeatureSummary(feature) : previewing ? '悬停自动预览 · 暂无特征' : `已持久化 · ${asset.id.slice(0, 8)}`}</small></span></span></button> })}</div></div><div className="video-library-preview" aria-live="polite"><span>{activePreview ? labelForAsset(activePreview) : '等待素材'}</span><b>{activePreview ? titleForAsset(activePreview) : '选择一个视频素材开始预览'}</b><small>{activePreview ? (featureForVideoAsset(activePreview, assetFeatures)?.sellingPoints[0] ?? '沉浸式封面预览 · 9:16 竖屏内容 · 已通过服务端持久化门禁') : '素材成功生成并保存后会出现在这里。'}</small><div><Play size={14} fill="currentColor"/>{activePreview && featureForVideoAsset(activePreview, assetFeatures) ? 'Planner 可读取 Hook、商品露出与相似度风险' : '点击卡片播放，悬停自动预览'}</div></div><button className="secondary-button full" disabled={!selectedAssets.length} onClick={addClip}><Scissors size={15}/>加入混剪时间线</button></aside>
      <section className="editing-center"><div className="editing-preview"><div className="preview-grid"/><div className="editing-safe-frame"><span>9:16</span><b>精度，先于承诺被看见。</b><small>WHITE PRECISION</small></div><button aria-label="播放剪辑预览" onClick={() => onNotice('正在播放当前时间线')}><Play size={18} fill="currentColor"/></button><time>00:06.8 / 00:15.0</time></div><div className="timeline-toolbar"><span>时间线 · v1.3</span><div><button aria-label="撤销编辑" onClick={() => onNotice('已撤销上一步编辑')}>撤销</button><button aria-label="保存时间线" onClick={() => onNotice('时间线 v1.4 已保存')}><Save size={14}/>保存</button></div></div><div className="editing-timeline">{[['视频', 'clip video-a'], ['叠加', 'clip overlay'], ['字幕', 'clip caption'], ['配音', 'clip voice'], ['音乐', 'clip music']].map(([track, className], index) => <div className="timeline-row" key={track}><span>{index === 2 ? <Subtitles size={14}/> : index > 2 ? <Volume2 size={14}/> : <Film size={14}/>} {track}</span><div className="timeline-lane"><button className={className} onClick={() => onNotice(`${track}轨道已选中`)}>{index === 0 ? `${clipCount} 个镜头 · 00:15` : index === 2 ? '精度，先于承诺被看见。' : index === 3 ? '品牌旁白' : index === 4 ? <><Music2 size={13}/>品牌节奏</> : '产品卖点与品牌标识'}</button></div></div>)}</div></section>
      <aside className="editing-inspector"><div className="surface-toolbar"><h3>视频包装</h3><span className={`status ${qualityReport ? qualityStatusClass(qualityReport.verdict) : 'success'}`}><span/>{qualityReport ? qualityVerdictText(qualityReport.verdict) : '可导出'}</span></div><div className="inspector-section"><span>画面规格</span><b>1080 × 1920 · 9:16</b><small>抖音 / 快手信息流</small></div><label>RemixPlan ID<input value={renderPlanId} onChange={event => setRenderPlanId(event.target.value)} placeholder="remixplan_xxx"/></label><div className="packaging-options"><span>包装组件</span>{['动态字幕', '节奏音效', '品牌片尾', '转化 CTA'].map(item => <button key={item} className={packaging.includes(item) ? 'active' : ''} onClick={() => togglePackaging(item)} aria-pressed={packaging.includes(item)}>{packaging.includes(item) ? <Check size={13}/> : null}{item}</button>)}</div><div className="editing-checks"><span><Check size={14}/>已选 {selectedAssets.length} 段生成视频</span><span><Check size={14}/>{packaging.length} 个包装组件启用</span><span><Check size={14}/>字幕静音可理解</span><span><Check size={14}/>品牌检查通过</span></div><button className="primary-button full" disabled={!selectedAssets.length} onClick={() => onNotice(`混剪版本 v1.4 已生成：${selectedAssets.length} 段视频 + ${packaging.length} 个包装组件`)}><Sparkles size={15}/>生成混剪版本</button><button className="secondary-button full" onClick={onCreate}><Video size={15}/>保存为 EditTask</button><button className="secondary-button full" disabled={!selectedAssets.length || renderBusy} onClick={() => void createRenderJob()}><Download size={15}/>{renderBusy ? '导出中…' : '创建 RenderJob'}</button><button className="secondary-button full" disabled={!renderJob || renderBusy || Boolean(qualityReport)} onClick={() => void createQualityReport()}><ShieldCheck size={15}/>执行质量检查</button>{renderJob ? <div className="inline-notice" role="status">RenderJob {renderJob.id.slice(0, 8)} · {renderJob.status} · {renderJob.progress}%{renderJob.requires_review ? ' · 需人工复核' : ''}{renderJob.quality_report_id ? ` · 报告 ${renderJob.quality_report_id.slice(0, 8)}` : ''}{renderJob.error_message ? ` · ${renderJob.error_message}` : ''}<progress value={renderJob.progress} max={100}/>{outputAssetLabel ? <span>成片资产：{outputAssetLabel}</span> : null}{outputPreviewURL ? <a href={outputPreviewURL} target="_blank" rel="noreferrer">打开成片预览</a> : null}{provenanceLabel ? <small>{provenanceLabel}</small> : null}</div> : null}{qualityReport ? <div className="quality-report-card"><div><span>QualityReport</span><b>{qualityVerdictText(qualityReport.verdict)} · {Math.round(qualityReport.score * 100)}分</b></div><ul>{qualityReport.dimensions.slice(0, 3).map(dimension => <li key={dimension.name}><span>{dimension.name}</span><b>{Math.round(dimension.score * 100)}%</b><small>{dimension.summary}</small></li>)}</ul>{qualityReport.issues[0] ? <p>{qualityReport.issues[0].start_seconds.toFixed(1)}s-{qualityReport.issues[0].end_seconds.toFixed(1)}s · {qualityReport.issues[0].description} · {qualityReport.issues[0].repair_suggestion}</p> : <p>未发现 critical/major 问题，可继续进入成片回流。</p>}</div> : null}<div className="feedback-card"><div><span>反馈飞轮</span><b>人工评分与评论</b></div><div className="feedback-rating"><button aria-label="降低评分" onClick={() => setFeedbackRating(value => Math.max(1, value - 1))}><ThumbsDown size={13}/></button><strong>{feedbackRating}/5</strong><button aria-label="提高评分" onClick={() => setFeedbackRating(value => Math.min(5, value + 1))}><ThumbsUp size={13}/></button></div><textarea value={feedbackComment} onChange={event => setFeedbackComment(event.target.value)} maxLength={1000}/><button className="secondary-button full" onClick={() => void submitPlanFeedback()}><Save size={15}/>提交计划反馈</button><button className="secondary-button full" disabled={!renderJob?.output_asset} onClick={() => void submitOutputFeedback()}><Sparkles size={15}/>提交成片反馈并生成权重快照</button><small>{feedbackNotice}</small></div><div className="inline-notice" role="status">{renderNotice}</div></aside>
    </div>
  </div>
}

function featureForVideoAsset(asset: ApiArtifact, features: ApiAssetFeature[]): ApiAssetFeature | undefined {
  return features
    .filter(feature => feature.assetId === asset.id && feature.assetVersion === asset.version)
    .sort((left, right) => right.updatedAt.localeCompare(left.updatedAt))[0]
}

function videoFeatureSummary(feature: ApiAssetFeature): string {
  const sellingPoint = feature.sellingPoints[0] ? ` · ${feature.sellingPoints[0]}` : ''
  return `商品露出 ${featurePercent(feature.productVisibility)} · ${riskText(feature.similarityRisk)}${sellingPoint}`
}

function featurePercent(value: number): string {
  return `${Math.round(value * 100)}%`
}

function riskText(risk: ApiAssetFeature['similarityRisk']): string {
  return risk === 'high' ? '高相似风险' : risk === 'medium' ? '中相似风险' : '低相似风险'
}

function qualityVerdictText(verdict: ApiQualityReport['verdict']): string {
  return verdict === 'critical' ? '严重阻断' : verdict === 'major' ? '需复核' : '质检通过'
}

function qualityStatusClass(verdict: ApiQualityReport['verdict']): 'success' | 'warning' | 'danger' {
  return verdict === 'critical' ? 'danger' : verdict === 'major' ? 'warning' : 'success'
}

export function ReportCenterPage({ state }: { state: DataState }) {
  const { currentProject, updateArtifact } = useProject()
  const [section, setSection] = useState('执行摘要')
  const [version, setVersion] = useState(4)
  const [notice, setNotice] = useState('')
  const sections = ['执行摘要', '发生了什么', '为什么发生', '创意样本', '下一步行动']
  const evidence = currentProject.operations.filter(record => record.kind === 'evidence')
  const metric = currentProject.operations.find(record => record.kind === 'metric')
  const metricField = (key: string, fallback: string) => String(metric?.fields[key] ?? fallback)
  const save = async () => {
    const nextVersion = `v1.${version + 1}`
    try {
      await updateArtifact('insight', { version: nextVersion, status: '已确认', sourceVersion: `创意 ${currentProject.artifacts.creative.version}`, summary: '证据前置版本点击率较基线提升 18%，95% 置信范围 +12% 至 +23%' })
      setVersion(value => value + 1)
      setNotice(`报告 ${nextVersion} 已保存`)
    } catch (cause) {
      setNotice(cause instanceof Error ? cause.message : '保存报告失败，请重试。')
    }
  }
  return <StateBoundary
    state={state}
    contextLabel="素材洞察 / 报告中心"
    emptyTitle="当前 Project 暂无可保存报告"
    emptyDetail="生成投后复盘或沉淀经验后，报告中心会展示版本、引用来源和导出动作。"
    errorDetail="报告版本或引用证据读取失败。请确认服务端可用后重新加载。"
  ><div className="report-workspace">
    <aside className="report-outline"><div className="surface-toolbar"><h3>报告结构</h3><button aria-label="新增报告章节"><FileText size={15}/></button></div>{sections.map((item, index) => <button className={section === item ? 'active' : ''} key={item} onClick={() => setSection(item)}><span>{String(index + 1).padStart(2, '0')}</span>{item}</button>)}<div className="version-block"><span>报告版本</span><b>v1.{version}</b><small>数据截止 2026-07-22 16:00</small></div></aside>
    <article className="report-document"><div className="document-meta"><span>{currentProject.name}</span><span>效果分析报告 v1.{version}</span><button onClick={() => setNotice('PDF 导出任务已创建')}><Download size={14}/>导出 PDF</button></div><h1>{section === '执行摘要' ? metricField('summary', '暂无服务端指标摘要。') : section}</h1><p className="report-lead">{metric?.title ?? '暂无服务端趋势记录。'}</p><div className="report-metric-line"><div><small>当前指标</small><b>{metricField('latest', '—')}</b><span>{metricField('comparison', '暂无对比数据')}</span></div><div><small>样本</small><b>{metricField('sample', '—')}</b><span>服务端已存档</span></div><div><small>置信范围</small><b>{metricField('confidence', '—')}</b><span>{metricField('unit', '—')}</span></div></div><h2>结论与边界</h2><p>{metricField('scope', '暂无服务端适用范围说明。')}</p><div className="report-callout"><b>建议行动</b><p>{metricField('recommendation', '暂无服务端建议动作。')}</p></div></article>
    <aside className="report-sources"><div className="surface-toolbar"><h3>引用与版本</h3><button aria-label="报告更多操作"><ChevronDown size={15}/></button></div>{evidence.map(item => <button key={item.id}><span>{item.id}</span><div><b>{item.title}</b><small>{String(item.fields.source ?? '—')} · {new Date(item.occurredAt).toLocaleDateString('zh-CN')}</small></div><ExternalLink size={13}/></button>)}{!evidence.length ? <div className="panel-empty">暂无服务端证据记录。</div> : null}<button className="primary-button full" onClick={() => void save()}><Save size={15}/>保存报告版本</button>{notice ? <div className="inline-notice" role="status">{notice}</div> : null}</aside>
  </div></StateBoundary>
}

type DeliveryGateCheck = {
  code: string
  label: string
  passed: boolean
  repair: string
}

type DeliveryGateGroup = {
  title: string
  checks: DeliveryGateCheck[]
}

function statusIsHealthy(status?: ApiAdAccountBinding['permissionStatus']) {
  return status === 'normal'
}

function confirmedMaterialFor(pointer: ApiAssetVersionPointer, confirmations: ApiMaterialConfirmation[]) {
  return confirmations.find(item => item.projectId === pointer.projectId && item.assetId === pointer.assetId && item.assetVersion === pointer.workingVersion && item.status === 'confirmed')
}

function deliveryPlanSignature(account: ApiAdAccountBinding | undefined, budget: number, materials: ApiAssetVersionPointer[]) {
  const materialPart = materials.map(item => `${item.assetId}@v${item.workingVersion}`).sort().join('|') || 'no-material'
  return [account?.id ?? 'no-account', budget, materialPart].join(':')
}

function deliveryPlanVersion(signature: string) {
  let hash = 0
  for (const char of signature) hash = (hash * 31 + char.charCodeAt(0)) % 100000
  return `plan-v${String(hash).padStart(5, '0')}`
}

function buildDeliveryGateGroups(account: ApiAdAccountBinding | undefined, budget: number, budgetLimit: number, materials: ApiAssetVersionPointer[], confirmations: ApiMaterialConfirmation[]): DeliveryGateGroup[] {
  const confirmedCount = materials.filter(pointer => confirmedMaterialFor(pointer, confirmations)).length
  return [
    {
      title: '输入完整性',
      checks: [
        { code: 'account', label: account ? `账户已选择：${account.accountName}` : '未选择广告账户', passed: Boolean(account), repair: '选择与当前 Project 绑定的广告账户。' },
        { code: 'budget', label: `预算 ¥${budget.toLocaleString('zh-CN')} / 护栏 ¥${budgetLimit.toLocaleString('zh-CN')}`, passed: budget > 0 && budget <= budgetLimit, repair: '预算必须大于 0 且不超过 Project 护栏。' },
        { code: 'materials', label: `素材组合 ${materials.length} 个版本`, passed: materials.length > 0, repair: '至少选择一个已纳入当前 Project 的素材版本。' },
      ],
    },
    {
      title: '账户权限',
      checks: [
        { code: 'permission', label: `权限：${account?.permissionStatus ?? '未连接'}`, passed: statusIsHealthy(account?.permissionStatus), repair: '重新授权广告账户或联系账户负责人。' },
        { code: 'login', label: `登录：${account?.loginStatus ?? '未连接'}`, passed: statusIsHealthy(account?.loginStatus), repair: '恢复账户登录状态后重新预检。' },
      ],
    },
    {
      title: '素材品牌版权',
      checks: [
        { code: 'human-confirmed', label: `人工确认版本 ${confirmedCount}/${materials.length}`, passed: materials.length > 0 && confirmedCount === materials.length, repair: '仅允许使用 MaterialConfirmation 已确认的当前素材版本。' },
        { code: 'brand-scope', label: '品牌、版权和使用范围绑定到当前 Project', passed: materials.length > 0 && confirmedCount === materials.length, repair: '回到素材检查页完成品牌版权复核和人工确认。' },
      ],
    },
    {
      title: '预算追踪回滚',
      checks: [
        { code: 'tracking', label: `像素追踪：${account?.trackingStatus ?? '未连接'}`, passed: statusIsHealthy(account?.trackingStatus), repair: '修复像素或转化 API 追踪异常。' },
        { code: 'rollback', label: '已配置模拟执行证据和回滚说明', passed: Boolean(account) && budget > 0, repair: '补齐账户与预算后才能生成可回滚执行证据。' },
      ],
    },
  ]
}

function gateGroupsPassed(groups: DeliveryGateGroup[]) {
  return groups.every(group => group.checks.every(check => check.passed))
}

export function DeliveryPlanPage({ state }: { state: DataState }) {
  const { currentProject, addChangeSet, preflightChangeSet } = useProject()
  const [step, setStep] = useState('计划配置')
  const [notice, setNotice] = useState('')
  const [budget, setBudget] = useState(currentProject.budget)
  const [latest, setLatest] = useState<DeliveryChangeSet>()
  const [busy, setBusy] = useState(false)
  const [workbench, setWorkbench] = useState<ApiAgencyWorkbench | null>(null)
  const [selectedAccountId, setSelectedAccountId] = useState('')
  const [preflightSignature, setPreflightSignature] = useState('')
  const planPeriod = '2026-07-25 至 2026-08-31'
  const audience = `${currentProject.brand} 高意向人群 / 近 30 天互动用户`
  const landingPage = `https://demo.cookies.local/lp/${currentProject.code.toLowerCase()}`
  const pixelId = `PX-${currentProject.code}-LEAD`
  const namingRule = `${currentProject.code}_{{account}}_{{asset}}_{{date}}`
  const projectAccounts = useMemo(() => workbench?.adAccountBindings.filter(account => account.projectIds.includes(currentProject.id)) ?? [], [currentProject.id, workbench])
  const selectedAccount = projectAccounts.find(account => account.id === selectedAccountId) ?? projectAccounts[0]
  const materials = useMemo(() => workbench?.assetVersionPointers.filter(pointer => pointer.projectId === currentProject.id) ?? [], [currentProject.id, workbench])
  const confirmations = workbench?.materialConfirmations ?? []
  const gateGroups = useMemo(() => buildDeliveryGateGroups(selectedAccount, budget, currentProject.budget, materials, confirmations), [budget, confirmations, currentProject.budget, materials, selectedAccount])
  const planSignature = deliveryPlanSignature(selectedAccount, budget, materials)
  const planVersion = deliveryPlanVersion(planSignature)
  const preflightStale = Boolean(latest?.preflight) && preflightSignature !== planSignature
  const canRunPreflight = latest !== undefined && latest.status === 'draft' && gateGroupsPassed(gateGroups)
  useEffect(() => {
    setBudget(currentProject.budget)
    setSelectedAccountId('')
    setPreflightSignature('')
  }, [currentProject.id, currentProject.budget])
  useEffect(() => {
    let active = true
    void Promise.all([deliveryApi.listChangeSets(currentProject.id), api.listAgencyWorkbench()]).then(([records, agency]) => {
      if (!active) return
      const changeSet = records.at(-1)
      setLatest(changeSet)
      setWorkbench(agency)
      if (changeSet?.preflight) setPreflightSignature('')
    }).catch(() => undefined)
    return () => { active = false }
  }, [currentProject.id])
  const createChange = async () => {
    setBusy(true)
    try {
      const changeSet = await addChangeSet(budget)
      setLatest(changeSet)
      setPreflightSignature('')
      setNotice(`${changeSet.id} 已在服务端创建；当前计划版本为 ${planVersion}，尚未执行任何真实广告平台写入。`)
    } catch (error) {
      setNotice(error instanceof Error ? error.message : '创建 ChangeSet 失败')
    } finally {
      setBusy(false)
    }
  }
  const preflight = async () => {
    if (!latest) return
    if (!gateGroupsPassed(gateGroups)) {
      setNotice('预检未通过：请先修复账户权限、预算、素材人工确认或追踪回滚问题。')
      return
    }
    setBusy(true)
    try {
      const changeSet = await preflightChangeSet(latest.id)
      setLatest(changeSet)
      setPreflightSignature(planSignature)
      setNotice(changeSet.preflight?.passed ? `预检通过并绑定 ${planVersion}，可进入执行确认。` : '预检未通过，请按修复建议补齐输入。')
    } catch (error) {
      setNotice(error instanceof Error ? error.message : '预检失败')
    } finally {
      setBusy(false)
    }
  }
  return <StateBoundary state={state}><div className="delivery-plan-workspace">
    <section className="plan-main"><ArtifactFlow compact/><div className="plan-tabs">{['计划配置', '素材组合', '预算与排期', '校验'].map(item => <button className={step === item ? 'active' : ''} key={item} onClick={() => setStep(item)}>{item}</button>)}</div><div className="plan-form"><div><label>计划名称<input defaultValue="销售线索增长计划 06"/></label><label>广告账户<select value={selectedAccount?.id ?? ''} onChange={event => setSelectedAccountId(event.target.value)}>{projectAccounts.length ? projectAccounts.map(account => <option key={account.id} value={account.id}>{account.platform} · {account.accountName}</option>) : <option value="">无绑定账户</option>}</select></label></div><div><label>总预算（CNY）<input type="number" value={budget} onChange={event => setBudget(Number(event.target.value))}/></label><label>投放周期<input readOnly value={planPeriod}/></label></div><div><label>受众<input readOnly value={audience}/></label><label>落地页<input readOnly value={landingPage}/></label></div><div><label>像素<input readOnly value={pixelId}/></label><label>命名规则<input readOnly value={namingRule}/></label></div><label>素材组合<div className="delivery-material-combo">{materials.map(pointer => { const confirmation = confirmedMaterialFor(pointer, confirmations); return <span key={pointer.id} className={confirmation ? 'confirmed' : 'blocked'}><b>{pointer.assetId} v{pointer.workingVersion}</b><small>{confirmation ? `已人工确认 · ${confirmation.confirmedBy}` : '未人工确认，禁止执行'}</small></span> })}{materials.length === 0 ? <span className="blocked"><b>暂无素材版本</b><small>请先完成素材制作和人工确认。</small></span> : null}</div></label></div><div className="validation-list delivery-gate-list"><h3>上线前预检 · {planVersion}</h3>{gateGroups.flatMap(group => group.checks.map(check => <span key={`${group.title}-${check.code}`} className={check.passed ? '' : 'preflight-failed'}>{check.passed ? <CircleCheck size={16}/> : <CircleAlert size={16}/>}<b>{group.title} · {check.label}</b>{!check.passed ? <small>{check.repair}</small> : null}</span>))}</div></section>
    <aside className="changeset-panel"><div className="surface-toolbar"><h3>ChangeSet</h3><span className="source-chip">本地模拟</span></div>{latest ? <><div className="changeset-title"><span>{latest.id} · v{latest.version}</span><h2>{latest.name}</h2><small>预算边界 ¥{latest.budgetLimit?.toLocaleString('zh-CN') ?? 0} · {latest.status}</small><small>当前计划版本 {planVersion}</small></div>{latest.preflight ? <div className="validation-list delivery-gate-list">{preflightStale ? <span className="preflight-failed"><CircleAlert size={16}/><b>预检版本已失效</b><small>计划账户、预算或素材组合变化后，必须重新生成 ChangeSet 并预检。</small></span> : <span><CircleCheck size={16}/><b>预检绑定 {planVersion}</b><small>{latest.preflight.checkedAt}</small></span>}{latest.preflight.checks.map(check => <span key={check.code} className={check.passed ? '' : 'preflight-failed'}>{check.passed ? <CircleCheck size={16}/> : <CircleAlert size={16}/>}<b>{check.message}</b>{!check.passed ? <small>{check.repair}</small> : null}</span>)}</div> : <div className="rollback-copy"><ShieldCheck size={16}/><span><b>待运行预检</b><small>系统会校验输入完整性、账户权限、素材品牌版权、预算追踪回滚四组门禁。</small></span></div>}<div className="rollback-copy"><ShieldCheck size={16}/><span><b>执行确认门禁</b><small>仅当预检绑定当前计划版本且素材均为人工确认版本时允许执行。</small></span></div></> : <div className="panel-empty">尚未创建服务端 ChangeSet</div>}<button className="secondary-button full" onClick={createChange} disabled={busy}>生成 ChangeSet</button><button className="primary-button full" onClick={preflight} disabled={!canRunPreflight || busy}><Send size={15}/>{latest?.status === 'preflight_passed' && !preflightStale ? '已通过预检' : '运行上线前预检'}</button>{notice ? <div className="inline-notice" role="status">{notice}</div> : null}</aside>
  </div></StateBoundary>
}

export function ApprovalCenterPage({ state }: { state: DataState }) {
  const { currentProject, approveChangeSet, executeChangeSet, rollbackChangeSet } = useProject()
  const [changeSets, setChangeSets] = useState<DeliveryChangeSet[]>([])
  const [selectedId, setSelectedId] = useState('')
  const [notice, setNotice] = useState('')
  const [busy, setBusy] = useState(false)
  const [workbench, setWorkbench] = useState<ApiAgencyWorkbench | null>(null)
  const selected = useMemo(() => changeSets.find(item => item.id === selectedId), [changeSets, selectedId])
  const projectAccounts = workbench?.adAccountBindings.filter(account => account.projectIds.includes(currentProject.id)) ?? []
  const selectedAccount = projectAccounts[0]
  const materials = workbench?.assetVersionPointers.filter(pointer => pointer.projectId === currentProject.id) ?? []
  const confirmations = workbench?.materialConfirmations ?? []
  const approvalGateGroups = buildDeliveryGateGroups(selectedAccount, selected?.budgetLimit ?? currentProject.budget, currentProject.budget, materials, confirmations)
  const executionGatePassed = gateGroupsPassed(approvalGateGroups)
  const objectCount = Math.max(materials.length, 1) * (selectedAccount ? 1 : 0)
  const riskLabel = executionGatePassed ? '低：账户、素材、预算和回滚均已满足' : '高：存在未确认素材、账户异常或预算追踪阻断'
  const refresh = async () => {
    setBusy(true)
    try {
      const [records, agency] = await Promise.all([deliveryApi.listChangeSets(currentProject.id), api.listAgencyWorkbench()])
      setChangeSets(records)
      setWorkbench(agency)
      setSelectedId(current => records.some(item => item.id === current) ? current : records[0]?.id ?? '')
      setNotice(records.length ? '已从服务端加载投放模拟队列。' : '尚未创建服务端 ChangeSet，请先在投放计划中生成。')
    } catch (error) {
      setNotice(error instanceof Error ? error.message : '加载审批队列失败')
    } finally {
      setBusy(false)
    }
  }
  useEffect(() => { void refresh() }, [currentProject.id])
  const apply = async (action: 'approve' | 'execute' | 'rollback') => {
    if (!selected) return
    if (action === 'execute' && !executionGatePassed) {
      setNotice('执行被拦截：素材必须是人工确认版本，且账户、预算、追踪和回滚门禁均需通过。')
      return
    }
    setBusy(true)
    try {
      const updated = action === 'approve' ? await approveChangeSet(selected.id) : action === 'execute' ? await executeChangeSet(selected.id) : await rollbackChangeSet(selected.id, '演示用户确认回滚模拟结果')
      setChangeSets(current => current.map(item => item.id === updated.id ? updated : item))
      setNotice(action === 'approve' ? '已由演示审批人批准，可执行本地模拟。' : action === 'execute' ? '模拟执行完成，未写入真实广告平台。' : '模拟回滚完成，原计划未受真实平台影响。')
    } catch (error) {
      setNotice(error instanceof Error ? error.message : '投放模拟操作失败')
    } finally {
      setBusy(false)
    }
  }
  return <StateBoundary
    state={state}
    contextLabel="智能投放 / 审批中心"
    emptyTitle="当前 Project 暂无待审批 ChangeSet"
    emptyDetail="从投放计划生成并通过预检后，ChangeSet 会进入审批队列；这里不会展示其他 Project 的审批状态。"
    errorDetail="审批队列暂时无法读取。请确认投放服务可用后刷新队列。"
    retryLabel="刷新审批队列"
    onRetry={() => { void refresh() }}
  ><div className="approval-workspace">
    <aside className="approval-queue"><div className="surface-toolbar"><h3>审批队列</h3><button onClick={() => void refresh()} disabled={busy} aria-label="刷新审批队列"><RotateCcw size={15}/></button></div>{changeSets.map(item => <button key={item.id} className={selectedId === item.id ? 'active' : ''} onClick={() => setSelectedId(item.id)}><span>{item.id.slice(0, 8)}</span><b>{item.name}</b><small>{item.status} · ¥{item.budgetLimit?.toLocaleString('zh-CN') ?? 0}</small></button>)}</aside>
    <section className="approval-detail">{selected ? <><div className="approval-heading"><div><span>{selected.id.slice(0, 8)} · ChangeSet v{selected.version}</span><h2>{selected.name}</h2><p>服务端受控投放模拟。只有预检通过、人工批准且执行确认门禁通过后才能执行。</p></div><span className={`approval-status ${selected.status}`}>{selected.status}</span></div><div className="execution-confirmation"><h3>执行确认</h3><div><b>账户</b><span>{selectedAccount ? `${selectedAccount.platform} · ${selectedAccount.accountName}` : '无绑定账户'}</span></div><div><b>预算</b><span>¥{selected.budgetLimit?.toLocaleString('zh-CN') ?? currentProject.budget.toLocaleString('zh-CN')}</span></div><div><b>对象数量</b><span>{objectCount} 个广告对象 / {materials.length} 个素材版本</span></div><div><b>预计影响</b><span>仅本地模拟执行，记录投放对象、预算和审计证据。</span></div><div><b>风险</b><span className={executionGatePassed ? '' : 'danger-text'}>{riskLabel}</span></div><div><b>回滚能力</b><span>支持模拟回滚并保留原因、时间和执行证据。</span></div></div><div className="approval-evidence"><h3>素材人工确认版本</h3>{materials.map(pointer => { const confirmation = confirmedMaterialFor(pointer, confirmations); return <div key={pointer.id}><ClipboardCheck size={16}/><span><b>{pointer.assetId} v{pointer.workingVersion}</b><small>{confirmation ? `已确认 · ${confirmation.confirmedBy} · ${confirmation.createdAt}` : '未人工确认，禁止执行'}</small></span></div> })}{materials.length === 0 ? <div><CircleAlert size={16}/><span><b>暂无素材版本</b><small>请先完成素材检查和人工确认。</small></span></div> : null}</div><div className="approval-evidence"><h3>预检与执行证据</h3>{selected.preflight?.checks.map(check => <div key={check.code}><ClipboardCheck size={16}/><span><b>{check.message}</b><small>{check.passed ? '预检通过' : check.repair}</small></span></div>)}{approvalGateGroups.flatMap(group => group.checks.filter(check => !check.passed).map(check => <div key={`${group.title}-${check.code}`}><CircleAlert size={16}/><span><b>{group.title} · {check.label}</b><small>{check.repair}</small></span></div>))}{selected.execution?.evidence.map(item => <div key={item.step}><CircleCheck size={16}/><span><b>{item.message}</b><small>{item.recordedAt}</small></span></div>)}</div>{selected.rollback ? <div className="rollback-copy"><RotateCcw size={16}/><span><b>已完成模拟回滚</b><small>{selected.rollback.reason}</small></span></div> : null}<div className="approval-actions"><button className="secondary-button" onClick={() => void apply('rollback')} disabled={busy || selected.status !== 'executed'}><RotateCcw size={15}/>回滚模拟</button><button className="secondary-button" onClick={() => void apply('execute')} disabled={busy || selected.status !== 'approved' || !executionGatePassed}><Play size={15}/>模拟执行</button><button className="primary-button" onClick={() => void apply('approve')} disabled={busy || selected.status !== 'preflight_passed'}><ThumbsUp size={15}/>以演示审批人批准</button></div></> : <div className="panel-empty">没有服务端 ChangeSet</div>}{notice ? <div className="inline-notice" role="status">{notice}</div> : null}</section>
    <aside className="approval-audit"><span className="section-label">权限与边界</span><div><time>演示角色</time><span>demo-approver</span></div><div><time>执行范围</time><span>本地模拟，无真实广告平台写入</span></div><div><time>审计</time><span>预检、审批、执行和回滚均由服务端记录</span></div><div><time>硬门禁</time><span>未人工确认素材不能执行</span></div></aside>
  </div></StateBoundary>
}
