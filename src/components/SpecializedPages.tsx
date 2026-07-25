import { useEffect, useMemo, useState } from 'react'
import { ArrowRight, Check, ChevronDown, CircleAlert, CircleCheck, ClipboardCheck, Download, ExternalLink, FileText, Film, Image, Music2, Play, RotateCcw, Save, Scissors, Send, ShieldCheck, Sparkles, Subtitles, ThumbsDown, ThumbsUp, Video, Volume2, WandSparkles } from 'lucide-react'
import { useProject } from '../context/ProjectContext'
import { useModelConfig } from '../context/ModelConfigContext'
import { commerceHookTemplates, hookStoryboard } from '../data/commerceHooks'
import { api, type ApiArtifact, type ApiGenerationJob, type ApiPrerollScope, type ApiShortDramaPrerollCandidate, type ApiShortDramaPrerollPlan, type ApiShortDramaStoryContext } from '../data/api'
import type { ArtifactKey, BusinessTaskType, DataState } from '../types'
import { deliveryApi, type DeliveryChangeSet } from '../api/delivery'
import { StateBoundary } from './StateBoundary'

export function ArtifactFlow({ compact = false }: { compact?: boolean }) {
  const { currentProject } = useProject()
  const order: ArtifactKey[] = ['brief', 'strategy', 'creative', 'insight', 'delivery']
  return <div className={compact ? 'artifact-flow compact' : 'artifact-flow'} aria-label="Project 产物链路">{order.map((key, index) => { const artifact = currentProject.artifacts[key]; return <div className="artifact-node" key={key}><span>{String(index + 1).padStart(2, '0')}</span><div><b>{artifact.label} {artifact.version}</b><small>{artifact.status} · {artifact.owner}</small><small>{artifact.sourceVersion ?? `更新于 ${artifact.updatedAt}`}</small></div>{index < order.length - 1 ? <ArrowRight size={14}/> : null}</div> })}</div>
}

export function ImageTextCreationPage({ state }: { state: DataState }) {
  const { currentProject, reloadProjects, updateArtifact } = useProject()
  const { providers } = useModelConfig()
  const [selected, setSelected] = useState(0)
  const [channel, setChannel] = useState('小红书 4:5')
  const [headline, setHeadline] = useState('看得见的精度，兑现你的创新。')
  const [version, setVersion] = useState(8)
  const [notice, setNotice] = useState('')
  const [job, setJob] = useState<ApiGenerationJob | null>(null)
  const [confirmedBriefId, setConfirmedBriefId] = useState('')
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
    <section className="image-canvas-workspace"><div className="canvas-toolbar light"><span>{currentProject.name} · 图文 v1.{version}</span><div><button onClick={() => setNotice('预览链接已生成')}><ExternalLink size={14}/>预览</button><button onClick={() => setNotice('PNG 导出任务已创建')}><Download size={14}/>导出</button></div></div><div className="portrait-stage"><div className="social-poster"><img src="/assets/white-precision-cnc.png" alt="CNC 设备加工高精度金属零件"/><div className="poster-copy"><small>WHITE PRECISION</small><h2>{headline}</h2><p>±0.01mm 精度 · 98%+ 准时交付</p></div><span className="poster-index">0{selected + 1} / 04</span></div></div><div className="page-strip">{pages.map((page, index) => <button key={page} className={selected === index ? 'active' : ''} onClick={() => setSelected(index)}><span>{index + 1}</span>{page}</button>)}</div></section>
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

export function VideoCreationPage({ state, activeView, onOpenTask }: { state: DataState, activeView: string, onOpenTask: (id: string) => void }) {
  const { currentProject, createTask } = useProject()
  const [selected, setSelected] = useState('short-drama')
  const [notice, setNotice] = useState('')
  const [brandGenerated, setBrandGenerated] = useState(false)
  const [brandStage, setBrandStage] = useState(0)
  const category = activeView === '品牌广告' ? 'brand' : activeView === '素材剪辑' ? 'editing' : 'performance'
  const activeMode = performanceModes.find(item => item.id === selected) ?? performanceModes[0]
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
    <header className="video-workspace-header"><div><span className="section-label">视频创作 · {activeView}</span><h2>{title}</h2><p>{description}</p></div>{category !== 'editing' ? <button className="primary-button" onClick={() => void create()}><Video size={16}/>新建{category === 'performance' ? activeMode.label : '品牌广告'}</button> : null}</header>
    {category === 'performance' ? <><div className="performance-mode-tabs" role="tablist" aria-label="效果广告生成类型">{performanceModes.map(mode => <button key={mode.id} role="tab" aria-selected={selected === mode.id} className={selected === mode.id ? 'active' : ''} onClick={() => { setSelected(mode.id); setNotice('') }}><b>{mode.label}</b><small>{mode.guard}</small></button>)}</div>{selected === 'pre-roll' ? <CommerceHookWorkspace onNotice={setNotice}/> : selected === 'short-drama' || selected === 'game' ? <PreRollWorkspace key={selected} mode={selected} onNotice={setNotice}/> : <div className="performance-workflow">
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
          setStoryContext({ ...persistedShortDrama.storyContext })
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
          storyContext,
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
  const planShortDrama = async () => {
    if (!confirmedBriefId) {
      setInteractionFeedback('请先在需求中心确认 Brief，系统才会允许规划短剧前贴候选。')
      return
    }
    setIsPlanning(true)
    try {
      const next = await api.planShortDramaPreroll(currentProject.id, confirmedBriefId, {
        ...storyContext,
        reviewedSellingPoints: storyContext.reviewedSellingPoints.filter(value => value.trim()),
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
  const [result, setResult] = useState(commerceHookTemplates[0].result)
  const [previewing, setPreviewing] = useState(false)
  const [job, setJob] = useState<ApiGenerationJob | null>(null)
  const [confirmedBriefId, setConfirmedBriefId] = useState('')
  const selected = commerceHookTemplates.find(item => item.id === selectedId) ?? commerceHookTemplates[0]
  const configuredProvider = providers.find(provider => provider.status === '已配置')

  useEffect(() => {
    let active = true
    void Promise.all([api.listArtifacts(currentProject.id), api.listJobs(currentProject.id)]).then(([artifacts, jobs]) => {
      const latest = jobs.filter(candidate => candidate.artifactKind === 'video').at(-1)
      const brief = artifacts.filter(artifact => artifact.kind === 'brief' && artifact.status === 'ready').at(-1)
      if (active) {
        setJob(latest ?? null)
        setConfirmedBriefId(brief?.id ?? '')
      }
    }).catch(() => undefined)
    return () => { active = false }
  }, [currentProject.id])

  useEffect(() => {
    setFidelity(selected.fidelity)
    setCamera(selected.camera)
    setMotion(selected.motion)
    setResult(selected.result)
    setPreviewing(false)
  }, [selected])

  const prompt = `${fidelity} ${camera} ${motion} ${selected.environment} ${result} ${selected.guardrails}`
  useEffect(() => {
    if (!job || !['queued', 'running'].includes(job.status)) return
    const timer = window.setInterval(() => {
      void api.getJob(job.id).then(next => {
        setJob(next)
        if (next.status === 'succeeded') {
          void reloadProjects()
          onNotice(`「${selected.name}」生成完成，资产已关联。`)
        }
      }).catch(cause => onNotice(cause instanceof Error ? cause.message : '任务状态读取失败'))
    }, 1500)
    return () => window.clearInterval(timer)
  }, [job, onNotice, selected.name])
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
    if (!confirmedBriefId) {
      onNotice('请先在需求中心确认 Brief，再生成视频分镜。')
      return
    }
    try {
      const next = await api.createMedia(currentProject.id, 'video', prompt, confirmedBriefId)
      setJob(next)
      onNotice(next.status === 'succeeded' ? '视频生成完成，资产已保存。' : '视频生成任务已创建，正在轮询。')
    } catch (cause) {
      onNotice(cause instanceof Error ? cause.message : '创建视频生成任务失败。')
    }
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
      {configuredProvider ? <div className="hook-model"><CircleCheck size={15}/><span><b>{configuredProvider.name}</b><small>服务端媒体模型目录</small></span></div> : <div className="hook-model missing"><CircleAlert size={15}/><span><b>尚未配置模型</b><small>请在服务端配置 ARK_API_KEY 后重新检查能力。</small></span></div>}
      {!confirmedBriefId ? <div className="hook-model missing"><CircleAlert size={15}/><span><b>缺少已确认 Brief</b><small>请先在需求中心确认 Brief，再发起视频生成。</small></span></div> : null}
      <div className="hook-actions"><button className="secondary-button" onClick={() => void save()}><Save size={14}/>保存策略</button><button className="primary-button" disabled={!configuredProvider || ['queued', 'running'].includes(job?.status ?? '')} onClick={() => void generate()}><WandSparkles size={14}/>{job && ['queued', 'running'].includes(job.status) ? '生成中…' : '生成分镜'}</button></div>
      {job ? <div className="inline-notice" role="status">任务 {job.id.slice(0, 8)} · {job.status} · {job.model ?? '模型待分配'}{job.diagnostic ? ` · ${job.diagnostic}` : ''}</div> : null}
    </aside>
  </div>
}

function VideoEditingWorkspace({ onNotice, onCreate }: { onNotice: (message: string) => void, onCreate: () => void }) {
  const { currentProject } = useProject()
  const [assets, setAssets] = useState<ApiArtifact[]>([])
  const [selectedAssets, setSelectedAssets] = useState<string[]>([])
  const [assetState, setAssetState] = useState<'loading' | 'ready' | 'error'>('loading')
  const [clipCount, setClipCount] = useState(0)
  const [packaging, setPackaging] = useState(['动态字幕', '品牌片尾'])
  useEffect(() => {
    let active = true
    setAssetState('loading')
    void Promise.all([api.listArtifacts(currentProject.id), api.listJobs(currentProject.id)]).then(([artifacts, jobs]) => {
      const succeededJobs = new Set(jobs.filter(job => job.status === 'succeeded').map(job => job.id))
      const nextAssets = artifacts.filter(artifact => (
        artifact.kind === 'video'
        && artifact.status === 'ready'
        && Boolean(artifact.sourceJobId)
        && succeededJobs.has(artifact.sourceJobId!)
      ))
      if (active) {
        setAssets(nextAssets)
        setSelectedAssets(current => current.filter(id => nextAssets.some(asset => asset.id === id)))
        setAssetState('ready')
      }
    }).catch(() => {
      if (active) {
        setAssets([])
        setSelectedAssets([])
        setAssetState('error')
      }
    })
    return () => { active = false }
  }, [currentProject.id])
  const toggleAsset = (id: string) => setSelectedAssets(current => current.includes(id) ? current.filter(item => item !== id) : [...current, id])
  const togglePackaging = (name: string) => setPackaging(current => current.includes(name) ? current.filter(item => item !== name) : [...current, name])
  const addClip = () => { setClipCount(value => value + selectedAssets.length); onNotice(`已将 ${selectedAssets.length} 段已持久化视频加入混剪时间线`) }
  return <div className="video-editing-workspace">
    <div className="editing-toolbar"><div><span className="section-label">EditTask · ED-2607-12</span><b>15 秒竖版产品广告</b><small>来源：策略 v2.4 · Creative v1.3</small></div><div><button className="secondary-button" onClick={() => onNotice('低清预览渲染已创建')}><Play size={14} fill="currentColor"/>预览</button><button className="primary-button" onClick={() => onNotice('1080×1920 导出任务已创建')}><Download size={15}/>导出</button></div></div>
    <div className="editing-shell">
      <aside className="editing-assets"><div className="surface-toolbar"><h3>视频素材箱</h3><span>当前 Project</span></div><div className="asset-group"><span>选择参与混剪的视频 · {selectedAssets.length}/{assets.length}</span>{assetState === 'loading' ? <div className="panel-empty">正在加载服务端持久化资产…</div> : null}{assetState === 'error' ? <div className="panel-empty">素材箱加载失败，请刷新后重试。</div> : null}{assetState === 'ready' && !assets.length ? <div className="panel-empty">当前 Project 暂无可用于混剪的已持久化视频资产。</div> : null}{assets.map(asset => <button key={asset.id} className={selectedAssets.includes(asset.id) ? 'active' : ''} onClick={() => toggleAsset(asset.id)}><span className="asset-check">{selectedAssets.includes(asset.id) ? <Check size={12}/> : null}</span><Film size={15}/><div><b>{asset.prerollType === 'short_drama' ? '短剧前贴视频' : asset.prerollType === 'game' ? '游戏前贴视频' : '服务端视频资产'}</b><small>已持久化 · {asset.id.slice(0, 8)}</small></div></button>)}</div><button className="secondary-button full" disabled={!selectedAssets.length} onClick={addClip}><Scissors size={15}/>加入混剪时间线</button></aside>
      <section className="editing-center"><div className="editing-preview"><div className="preview-grid"/><div className="editing-safe-frame"><span>9:16</span><b>精度，先于承诺被看见。</b><small>WHITE PRECISION</small></div><button aria-label="播放剪辑预览" onClick={() => onNotice('正在播放当前时间线')}><Play size={18} fill="currentColor"/></button><time>00:06.8 / 00:15.0</time></div><div className="timeline-toolbar"><span>时间线 · v1.3</span><div><button aria-label="撤销编辑" onClick={() => onNotice('已撤销上一步编辑')}>撤销</button><button aria-label="保存时间线" onClick={() => onNotice('时间线 v1.4 已保存')}><Save size={14}/>保存</button></div></div><div className="editing-timeline">{[['视频', 'clip video-a'], ['叠加', 'clip overlay'], ['字幕', 'clip caption'], ['配音', 'clip voice'], ['音乐', 'clip music']].map(([track, className], index) => <div className="timeline-row" key={track}><span>{index === 2 ? <Subtitles size={14}/> : index > 2 ? <Volume2 size={14}/> : <Film size={14}/>} {track}</span><div className="timeline-lane"><button className={className} onClick={() => onNotice(`${track}轨道已选中`)}>{index === 0 ? `${clipCount} 个镜头 · 00:15` : index === 2 ? '精度，先于承诺被看见。' : index === 3 ? '品牌旁白' : index === 4 ? <><Music2 size={13}/>品牌节奏</> : '产品卖点与品牌标识'}</button></div></div>)}</div></section>
      <aside className="editing-inspector"><div className="surface-toolbar"><h3>视频包装</h3><span className="status success"><span/>可导出</span></div><div className="inspector-section"><span>画面规格</span><b>1080 × 1920 · 9:16</b><small>抖音 / 快手信息流</small></div><div className="packaging-options"><span>包装组件</span>{['动态字幕', '节奏音效', '品牌片尾', '转化 CTA'].map(item => <button key={item} className={packaging.includes(item) ? 'active' : ''} onClick={() => togglePackaging(item)} aria-pressed={packaging.includes(item)}>{packaging.includes(item) ? <Check size={13}/> : null}{item}</button>)}</div><div className="editing-checks"><span><Check size={14}/>已选 {selectedAssets.length} 段生成视频</span><span><Check size={14}/>{packaging.length} 个包装组件启用</span><span><Check size={14}/>字幕静音可理解</span><span><Check size={14}/>品牌检查通过</span></div><button className="primary-button full" disabled={!selectedAssets.length} onClick={() => onNotice(`混剪版本 v1.4 已生成：${selectedAssets.length} 段视频 + ${packaging.length} 个包装组件`)}><Sparkles size={15}/>生成混剪版本</button><button className="secondary-button full" onClick={onCreate}><Video size={15}/>保存为 EditTask</button></aside>
    </div>
  </div>
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
  return <StateBoundary state={state}><div className="report-workspace">
    <aside className="report-outline"><div className="surface-toolbar"><h3>报告结构</h3><button aria-label="新增报告章节"><FileText size={15}/></button></div>{sections.map((item, index) => <button className={section === item ? 'active' : ''} key={item} onClick={() => setSection(item)}><span>{String(index + 1).padStart(2, '0')}</span>{item}</button>)}<div className="version-block"><span>报告版本</span><b>v1.{version}</b><small>数据截止 2026-07-22 16:00</small></div></aside>
    <article className="report-document"><div className="document-meta"><span>{currentProject.name}</span><span>效果分析报告 v1.{version}</span><button onClick={() => setNotice('PDF 导出任务已创建')}><Download size={14}/>导出 PDF</button></div><h1>{section === '执行摘要' ? metricField('summary', '暂无服务端指标摘要。') : section}</h1><p className="report-lead">{metric?.title ?? '暂无服务端趋势记录。'}</p><div className="report-metric-line"><div><small>当前指标</small><b>{metricField('latest', '—')}</b><span>{metricField('comparison', '暂无对比数据')}</span></div><div><small>样本</small><b>{metricField('sample', '—')}</b><span>服务端已存档</span></div><div><small>置信范围</small><b>{metricField('confidence', '—')}</b><span>{metricField('unit', '—')}</span></div></div><h2>结论与边界</h2><p>{metricField('scope', '暂无服务端适用范围说明。')}</p><div className="report-callout"><b>建议行动</b><p>{metricField('recommendation', '暂无服务端建议动作。')}</p></div></article>
    <aside className="report-sources"><div className="surface-toolbar"><h3>引用与版本</h3><button aria-label="报告更多操作"><ChevronDown size={15}/></button></div>{evidence.map(item => <button key={item.id}><span>{item.id}</span><div><b>{item.title}</b><small>{String(item.fields.source ?? '—')} · {new Date(item.occurredAt).toLocaleDateString('zh-CN')}</small></div><ExternalLink size={13}/></button>)}{!evidence.length ? <div className="panel-empty">暂无服务端证据记录。</div> : null}<button className="primary-button full" onClick={() => void save()}><Save size={15}/>保存报告版本</button>{notice ? <div className="inline-notice" role="status">{notice}</div> : null}</aside>
  </div></StateBoundary>
}

export function DeliveryPlanPage({ state }: { state: DataState }) {
  const { currentProject, addChangeSet, preflightChangeSet } = useProject()
  const [step, setStep] = useState('计划配置')
  const [notice, setNotice] = useState('')
  const [budget, setBudget] = useState(currentProject.budget)
  const [latest, setLatest] = useState<DeliveryChangeSet>()
  const [busy, setBusy] = useState(false)
  useEffect(() => {
    setBudget(currentProject.budget)
  }, [currentProject.id, currentProject.budget])
  useEffect(() => {
    let active = true
    void deliveryApi.listChangeSets(currentProject.id).then(records => {
      if (active) setLatest(records.at(-1))
    }).catch(() => undefined)
    return () => { active = false }
  }, [currentProject.id])
  const createChange = async () => {
    setBusy(true)
    try {
      const changeSet = await addChangeSet(budget)
      setLatest(changeSet)
      setNotice(`${changeSet.id} 已在服务端创建；尚未执行任何真实广告平台写入。`)
    } catch (error) {
      setNotice(error instanceof Error ? error.message : '创建 ChangeSet 失败')
    } finally {
      setBusy(false)
    }
  }
  const preflight = async () => {
    if (!latest) return
    setBusy(true)
    try {
      const changeSet = await preflightChangeSet(latest.id)
      setLatest(changeSet)
      setNotice(changeSet.preflight?.passed ? '预检通过，已进入人工审批队列。' : '预检未通过，请按修复建议补齐输入。')
    } catch (error) {
      setNotice(error instanceof Error ? error.message : '预检失败')
    } finally {
      setBusy(false)
    }
  }
  return <StateBoundary state={state}><div className="delivery-plan-workspace">
    <section className="plan-main"><ArtifactFlow compact/><div className="plan-tabs">{['计划配置', '素材组合', '预算与排期', '校验'].map(item => <button className={step === item ? 'active' : ''} key={item} onClick={() => setStep(item)}>{item}</button>)}</div><div className="plan-form"><div><label>计划名称<input defaultValue="销售线索增长计划 06"/></label><label>转化目标<select defaultValue="高质量销售线索"><option>高质量销售线索</option><option>表单提交</option></select></label></div><div><label>总预算（CNY）<input type="number" value={budget} onChange={event => setBudget(Number(event.target.value))}/></label><label>投放周期<input defaultValue="2026-07-25 至 2026-08-31"/></label></div><label>素材来源<div className="upstream-source"><Sparkles size={17}/><span><b>创意 {currentProject.artifacts.creative.version}</b><small>{currentProject.artifacts.creative.summary}</small></span><CircleCheck size={17}/></div></label></div><div className="validation-list"><h3>上线前校验</h3>{['商品与落地页绑定准确', '预算未超过 Project 护栏', '素材版权与品牌检查通过', '转化追踪最近 30 分钟有信号'].map(item => <span key={item}><CircleCheck size={16}/>{item}</span>)}</div></section>
    <aside className="changeset-panel"><div className="surface-toolbar"><h3>ChangeSet</h3><span className="source-chip">本地模拟</span></div>{latest ? <><div className="changeset-title"><span>{latest.id} · v{latest.version}</span><h2>{latest.name}</h2><small>预算边界 ¥{latest.budgetLimit?.toLocaleString('zh-CN') ?? 0} · {latest.status}</small></div>{latest.preflight ? <div className="validation-list">{latest.preflight.checks.map(check => <span key={check.code} className={check.passed ? '' : 'preflight-failed'}>{check.passed ? <CircleCheck size={16}/> : <CircleAlert size={16}/>}<b>{check.message}</b>{!check.passed ? <small>{check.repair}</small> : null}</span>)}</div> : <div className="rollback-copy"><ShieldCheck size={16}/><span><b>待运行预检</b><small>系统会校验 Brief、创意和预算边界。</small></span></div>}<div className="rollback-copy"><ShieldCheck size={16}/><span><b>模拟边界</b><small>仅生成本地执行证据，不连接真实广告平台。</small></span></div></> : <div className="panel-empty">尚未创建服务端 ChangeSet</div>}<button className="secondary-button full" onClick={createChange} disabled={busy}>生成 ChangeSet</button><button className="primary-button full" onClick={preflight} disabled={!latest || busy || latest.status !== 'draft'}><Send size={15}/>{latest?.status === 'preflight_passed' ? '已通过预检' : '运行上线前预检'}</button>{notice ? <div className="inline-notice" role="status">{notice}</div> : null}</aside>
  </div></StateBoundary>
}

export function ApprovalCenterPage({ state }: { state: DataState }) {
  const { currentProject, approveChangeSet, executeChangeSet, rollbackChangeSet } = useProject()
  const [changeSets, setChangeSets] = useState<DeliveryChangeSet[]>([])
  const [selectedId, setSelectedId] = useState('')
  const [notice, setNotice] = useState('')
  const [busy, setBusy] = useState(false)
  const selected = useMemo(() => changeSets.find(item => item.id === selectedId), [changeSets, selectedId])
  const refresh = async () => {
    setBusy(true)
    try {
      const records = await deliveryApi.listChangeSets(currentProject.id)
      setChangeSets(records)
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
  return <StateBoundary state={state}><div className="approval-workspace">
    <aside className="approval-queue"><div className="surface-toolbar"><h3>审批队列</h3><button onClick={() => void refresh()} disabled={busy} aria-label="刷新审批队列"><RotateCcw size={15}/></button></div>{changeSets.map(item => <button key={item.id} className={selectedId === item.id ? 'active' : ''} onClick={() => setSelectedId(item.id)}><span>{item.id.slice(0, 8)}</span><b>{item.name}</b><small>{item.status} · ¥{item.budgetLimit?.toLocaleString('zh-CN') ?? 0}</small></button>)}</aside>
    <section className="approval-detail">{selected ? <><div className="approval-heading"><div><span>{selected.id.slice(0, 8)} · v{selected.version}</span><h2>{selected.name}</h2><p>服务端受控投放模拟。只有预检通过且演示审批人批准后才能执行。</p></div><span className={`approval-status ${selected.status}`}>{selected.status}</span></div><div className="approval-evidence"><h3>预检与执行证据</h3>{selected.preflight?.checks.map(check => <div key={check.code}><ClipboardCheck size={16}/><span><b>{check.message}</b><small>{check.passed ? '预检通过' : check.repair}</small></span></div>)}{selected.execution?.evidence.map(item => <div key={item.step}><CircleCheck size={16}/><span><b>{item.message}</b><small>{item.recordedAt}</small></span></div>)}</div>{selected.rollback ? <div className="rollback-copy"><RotateCcw size={16}/><span><b>已完成模拟回滚</b><small>{selected.rollback.reason}</small></span></div> : null}<div className="approval-actions"><button className="secondary-button" onClick={() => void apply('rollback')} disabled={busy || selected.status !== 'executed'}><RotateCcw size={15}/>回滚模拟</button><button className="secondary-button" onClick={() => void apply('execute')} disabled={busy || selected.status !== 'approved'}><Play size={15}/>模拟执行</button><button className="primary-button" onClick={() => void apply('approve')} disabled={busy || selected.status !== 'preflight_passed'}><ThumbsUp size={15}/>以演示审批人批准</button></div></> : <div className="panel-empty">没有服务端 ChangeSet</div>}{notice ? <div className="inline-notice" role="status">{notice}</div> : null}</section>
    <aside className="approval-audit"><span className="section-label">权限与边界</span><div><time>演示角色</time><span>demo-approver</span></div><div><time>执行范围</time><span>本地模拟，无真实广告平台写入</span></div><div><time>审计</time><span>预检、审批、执行和回滚均由服务端记录</span></div></aside>
  </div></StateBoundary>
}
