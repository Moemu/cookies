import { useEffect, useMemo, useReducer, useRef, useState } from 'react'
import { Check, ChevronRight, CircleAlert, Clapperboard, Clock3, Film, Image as ImageIcon, LoaderCircle, Play, RefreshCw, Sparkles, Upload, WandSparkles } from 'lucide-react'
import { useProject } from '../../context/ProjectContext'
import { api, type ApiProjectMediaAsset } from '../../data/api'
import { fixtureAnalysis, fixtureHooks, fixtureImages } from './fixtures'
import { canOpenShortDramaStep, initialShortDramaPrerollState, shortDramaPrerollReducer } from './reducer'
import type { PrerollDuration, ShortDramaPrerollState, ShortDramaStep } from './types'
import './short-drama-preroll-v2.css'

const steps: Array<{ id: ShortDramaStep; index: string; label: string; detail: string }> = [
  { id: 'understanding', index: '01', label: '素材理解', detail: '识别剧情与开场信息' },
  { id: 'direction', index: '02', label: '前贴方向', detail: '人工选择钩子方向' },
  { id: 'first-frame', index: '03', label: '首帧参考', detail: '生成并选择首帧图' },
  { id: 'video', index: '04', label: '视频生成', detail: '确认参数并生成前贴' },
]

const wait = (ms: number) => new Promise(resolve => window.setTimeout(resolve, ms))

function formatDuration(seconds?: number) {
  if (!seconds) return '时长待识别'
  const minutes = Math.floor(seconds / 60)
  return `${minutes}:${String(Math.round(seconds % 60)).padStart(2, '0')}`
}

function storageKey(projectId: string) { return `cookies.short-drama-preroll-v2:${projectId}` }

function readDraft(projectId: string): ShortDramaPrerollState | null {
  try {
    const raw = window.localStorage.getItem(storageKey(projectId))
    if (!raw) return null
    const parsed = JSON.parse(raw) as { version?: number; state?: ShortDramaPrerollState }
    if (parsed.version !== 1 || !parsed.state) return null
    return { ...parsed.state, analysisStatus: parsed.state.analysisStatus === 'loading' ? 'idle' : parsed.state.analysisStatus, hooksStatus: parsed.state.hooksStatus === 'loading' ? 'idle' : parsed.state.hooksStatus, imagesStatus: parsed.state.imagesStatus === 'loading' ? 'idle' : parsed.state.imagesStatus, videoStatus: parsed.state.videoStatus === 'loading' ? 'idle' : parsed.state.videoStatus }
  } catch { return null }
}

export function ShortDramaPrerollWorkspace({ onNotice }: { onNotice: (message: string) => void }) {
  const { currentProject } = useProject()
  const [state, dispatch] = useReducer(shortDramaPrerollReducer, initialShortDramaPrerollState)
  const [assets, setAssets] = useState<ApiProjectMediaAsset[]>([])
  const [mediaLoading, setMediaLoading] = useState(true)
  const [localPreviewUrl, setLocalPreviewUrl] = useState('')
  const hydratedProject = useRef('')
  const fileInput = useRef<HTMLInputElement>(null)

  useEffect(() => {
    let cancelled = false
    setMediaLoading(true)
    void api.listProjectMediaAssets(currentProject.id).then(result => {
      if (cancelled) return
      const videos = result.filter(asset => asset.kind === 'video')
      setAssets(videos)
      const restored = readDraft(currentProject.id)
      if (restored && !restored.source?.id.startsWith('local-')) dispatch({ type: 'restore', state: restored })
      else if (videos[0]) dispatch({ type: 'source-selected', source: videos[0] })
      hydratedProject.current = currentProject.id
    }).catch(() => {
      if (!cancelled) onNotice('项目视频素材暂时无法读取，也可以从本地选择视频继续搭建流程。')
    }).finally(() => { if (!cancelled) setMediaLoading(false) })
    return () => { cancelled = true }
  }, [currentProject.id, onNotice])

  useEffect(() => () => {
    if (localPreviewUrl) URL.revokeObjectURL(localPreviewUrl)
  }, [localPreviewUrl])

  useEffect(() => {
    if (hydratedProject.current !== currentProject.id) return
    window.localStorage.setItem(storageKey(currentProject.id), JSON.stringify({ version: 1, state }))
  }, [currentProject.id, state])

  const selectedHook = useMemo(() => state.hooks.find(item => item.id === state.selectedHookId) ?? null, [state.hooks, state.selectedHookId])
  const selectedImage = useMemo(() => state.images.find(item => item.id === state.selectedImageId) ?? null, [state.images, state.selectedImageId])
  const sourceUrl = localPreviewUrl || state.source?.contentUrl || ''

  const selectSource = (source: ApiProjectMediaAsset) => {
    if (localPreviewUrl) URL.revokeObjectURL(localPreviewUrl)
    setLocalPreviewUrl('')
    dispatch({ type: 'source-selected', source })
  }
  const selectLocalFile = (file?: File) => {
    if (!file) return
    if (localPreviewUrl) URL.revokeObjectURL(localPreviewUrl)
    const url = URL.createObjectURL(file)
    setLocalPreviewUrl(url)
    dispatch({ type: 'source-selected', source: { id: `local-${file.name}-${file.lastModified}`, projectId: currentProject.id, version: 1, kind: 'video', sourceType: 'upload', mimeType: file.type || 'video/mp4', sizeBytes: file.size, createdAt: new Date(file.lastModified).toISOString(), contentUrl: url } })
  }

  const analyze = async () => {
    if (!state.source) return
    dispatch({ type: 'analysis-started' })
    await wait(650)
    dispatch({ type: 'analysis-ready', analysis: fixtureAnalysis })
    onNotice('视频理解已完成；当前使用前端模拟数据，接口接入后将替换为真实分析结果。')
  }
  const generateHooks = async () => {
    dispatch({ type: 'hooks-started' })
    await wait(650)
    dispatch({ type: 'hooks-ready', hooks: fixtureHooks })
  }
  const generateImages = async () => {
    dispatch({ type: 'images-started' })
    await wait(800)
    dispatch({ type: 'images-ready', images: fixtureImages })
  }
  const generateVideo = async () => {
    dispatch({ type: 'video-started' })
    await wait(900)
    dispatch({ type: 'video-ready', output: { id: `fixture-${Date.now()}`, videoUrl: sourceUrl, duration: state.duration, createdAt: new Date().toISOString() } })
    onNotice('前贴视频演示结果已生成。当前为前端流程桩，后端接入后会返回真实独立前贴视频。')
  }

  return <section className="short-drama-v2" aria-label="短剧前贴创作工作区">
    <aside className="short-drama-v2-rail">
      <div className="short-drama-v2-source-card">
        <span className="short-drama-v2-kicker">SOURCE VIDEO</span>
        <div className="short-drama-v2-source-thumb">{sourceUrl ? <video src={sourceUrl} muted preload="metadata"/> : <Film size={28}/>}<span><Play size={13} fill="currentColor"/></span></div>
        <b>{state.analysis?.title || (state.source ? `项目视频 ${state.source.id.slice(0, 8)}` : '尚未选择短剧素材')}</b>
        <small>{state.analysis?.episode || formatDuration(state.source?.durationSeconds)} · {state.source?.width && state.source?.height ? `${state.source.width}×${state.source.height}` : '竖屏待识别'}</small>
        <button type="button" onClick={() => fileInput.current?.click()}><Upload size={14}/>更换视频</button>
        <input ref={fileInput} hidden type="file" accept="video/*" onChange={event => selectLocalFile(event.target.files?.[0])}/>
      </div>
      {assets.length > 1 ? <select aria-label="选择项目视频" value={localPreviewUrl ? '' : state.source?.id || ''} onChange={event => { const asset = assets.find(item => item.id === event.target.value); if (asset) selectSource(asset) }}><option value="">选择项目视频</option>{assets.map(asset => <option key={asset.id} value={asset.id}>{`视频 ${asset.id.slice(0, 8)} · ${formatDuration(asset.durationSeconds)}`}</option>)}</select> : null}
      <nav aria-label="短剧前贴流程"><ol>{steps.map((step, index) => {
        const enabled = canOpenShortDramaStep(state, step.id)
        const active = state.activeStep === step.id
        const completed = steps.findIndex(item => item.id === state.activeStep) > index || (step.id === 'video' && state.videoStatus === 'ready')
        return <li key={step.id} className={`${active ? 'active' : ''} ${completed ? 'completed' : ''}`}>
          <button type="button" disabled={!enabled} aria-current={active ? 'step' : undefined} onClick={() => dispatch({ type: 'open-step', step: step.id })}>
            <span className="short-drama-v2-step-index">{completed ? <Check size={14}/> : step.index}</span>
            <span><b>{step.label}</b><small>{step.detail}</small></span><ChevronRight size={15}/>
          </button>
        </li>
      })}</ol></nav>
      <div className="short-drama-v2-rail-note"><span>FLOW V2</span><small>当前仅生成独立前贴视频，不与短剧正片拼接。</small></div>
    </aside>

    <main className="short-drama-v2-main">
      <header><div><span className="short-drama-v2-kicker">SHORT DRAMA · PREROLL LAB</span><h3>{steps.find(step => step.id === state.activeStep)?.label}</h3><p>{steps.find(step => step.id === state.activeStep)?.detail}</p></div><span className="short-drama-v2-autosave"><Check size={13}/>草稿自动保存</span></header>
      {state.error ? <div className="short-drama-v2-error"><CircleAlert size={16}/>{state.error}</div> : null}
      {state.activeStep === 'understanding' ? <UnderstandingStage state={state} sourceUrl={sourceUrl} mediaLoading={mediaLoading} onAnalyze={() => void analyze()}/> : null}
      {state.activeStep === 'direction' ? <DirectionStage state={state} onSummary={value => dispatch({ type: 'summary-changed', value })} onGenerate={() => void generateHooks()} onSelect={id => dispatch({ type: 'hook-selected', id })}/> : null}
      {state.activeStep === 'first-frame' ? <FirstFrameStage state={state} onPrompt={value => dispatch({ type: 'image-prompt-changed', value })} onGenerate={() => void generateImages()} onSelect={id => dispatch({ type: 'image-selected', id })}/> : null}
      {state.activeStep === 'video' ? <VideoStage state={state} sourceUrl={sourceUrl} selectedImageUrl={selectedImage?.imageUrl || ''} onDescription={value => dispatch({ type: 'video-description-changed', value })} onPrompt={value => dispatch({ type: 'video-prompt-changed', value })} onGenerate={() => void generateVideo()}/> : null}
    </main>

    <aside className="short-drama-v2-inspector">
      <div className="short-drama-v2-inspector-head"><span>生成配置</span><b>{state.activeStep === 'understanding' ? '视频理解' : state.activeStep === 'direction' ? '方向选择' : state.activeStep === 'first-frame' ? '首帧生成' : '视频生成'}</b></div>
      {state.activeStep === 'understanding' ? <><InspectorBlock label="输入状态"><b>{state.source ? '素材已就绪' : '等待视频'}</b><small>{state.source ? `${state.source.mimeType} · ${(state.source.sizeBytes / 1024 / 1024).toFixed(1)} MB` : '请选择项目视频或本地文件'}</small></InspectorBlock><button className="short-drama-v2-primary" disabled={!state.source || state.analysisStatus === 'loading'} onClick={() => void analyze()}>{state.analysisStatus === 'loading' ? <LoaderCircle className="spin" size={16}/> : <Sparkles size={16}/>}理解视频内容</button></> : null}
      {state.activeStep === 'direction' ? <><InspectorBlock label="方向构成"><b>猎奇吸睛 × 2</b><b>剧情总结 × 2</b><small>必须人工选定一个方向，才会进入首帧生成。</small></InspectorBlock><button className="short-drama-v2-primary" disabled={!state.summaryDraft.trim() || state.hooksStatus === 'loading'} onClick={() => void generateHooks()}>{state.hooksStatus === 'loading' ? <LoaderCircle className="spin" size={16}/> : <WandSparkles size={16}/>}生成 4 个前贴方向</button></> : null}
      {state.activeStep === 'first-frame' ? <><InspectorBlock label="已选方向"><b>{selectedHook?.title || '尚未选择'}</b><small>{selectedHook?.hookCopy}</small></InspectorBlock><button className="short-drama-v2-primary" disabled={!state.imagePrompt.trim() || state.imagesStatus === 'loading'} onClick={() => void generateImages()}>{state.imagesStatus === 'loading' ? <LoaderCircle className="spin" size={16}/> : <ImageIcon size={16}/>}生成 3 张首帧图</button></> : null}
      {state.activeStep === 'video' ? <><InspectorBlock label="视频时长"><div className="short-drama-v2-duration">{([5, 6, 10, 12, 15] as PrerollDuration[]).map(duration => <button type="button" className={state.duration === duration ? 'active' : ''} key={duration} onClick={() => dispatch({ type: 'duration-changed', duration })}>{duration}s</button>)}</div></InspectorBlock><InspectorBlock label="参考链路"><small>起始帧：已选 AI 首帧</small><small>目标尾帧：输入视频首帧</small><small>输出：独立前贴视频</small></InspectorBlock><button className="short-drama-v2-primary" disabled={!state.selectedImageId || !state.videoPrompt.trim() || state.videoStatus === 'loading'} onClick={() => void generateVideo()}>{state.videoStatus === 'loading' ? <LoaderCircle className="spin" size={16}/> : <Clapperboard size={16}/>}生成前贴视频</button></> : null}
      <div className="short-drama-v2-contract"><span>FRONTEND CONTRACT</span><small>当前交互由 fixture gateway 驱动，字段结构可直接替换为后端 API。</small></div>
    </aside>
  </section>
}

function InspectorBlock({ label, children }: { label: string; children: React.ReactNode }) { return <section className="short-drama-v2-inspector-block"><span>{label}</span>{children}</section> }

function UnderstandingStage({ state, sourceUrl, mediaLoading, onAnalyze }: { state: ShortDramaPrerollState; sourceUrl: string; mediaLoading: boolean; onAnalyze: () => void }) {
  return <div className="short-drama-v2-stage">
    <section className="short-drama-v2-media-canvas">{sourceUrl ? <video src={sourceUrl} controls preload="metadata"/> : <div><Film size={34}/><b>{mediaLoading ? '正在读取项目素材…' : '选择一条短剧视频开始'}</b><small>支持从项目素材或本地视频进入</small></div>}<span>INPUT / SHORT DRAMA</span></section>
    {state.analysis ? <section className="short-drama-v2-analysis-grid"><article><span>剧情梗概</span><h4>{state.analysis.title}</h4><p>{state.summaryDraft}</p></article><article><span>开场信息</span><p>{state.analysis.openingBeat}</p><div className="short-drama-v2-tags">{state.analysis.visualKeywords.map(item => <small key={item}>{item}</small>)}</div></article></section> : <section className="short-drama-v2-empty-action"><Sparkles size={20}/><div><b>先让系统理解输入视频</b><small>将提取标题、梗概、人物、开场动作与视觉关键词，结果允许人工修改。</small></div><button disabled={!state.source || state.analysisStatus === 'loading'} onClick={onAnalyze}>{state.analysisStatus === 'loading' ? '分析中…' : '开始理解'}</button></section>}
  </div>
}

function DirectionStage({ state, onSummary, onGenerate, onSelect }: { state: ShortDramaPrerollState; onSummary: (value: string) => void; onGenerate: () => void; onSelect: (id: string) => void }) {
  return <div className="short-drama-v2-stage"><section className="short-drama-v2-editor-card"><div><span>EDITABLE STORY SUMMARY</span><b>视频梗概</b></div><textarea value={state.summaryDraft} onChange={event => onSummary(event.target.value)} rows={4}/><small>修改梗概会使已有方向、首帧与视频结果失效。</small></section>
    {state.hooks.length ? <div className="short-drama-v2-hook-groups">{(['curiosity', 'summary'] as const).map(category => <section key={category}><header><span>{category === 'curiosity' ? 'CURIOUSITY HOOK' : 'STORY SUMMARY'}</span><b>{category === 'curiosity' ? '猎奇吸睛' : '剧情总结'}</b></header><div>{state.hooks.filter(item => item.category === category).map(hook => <button type="button" key={hook.id} className={state.selectedHookId === hook.id ? 'selected' : ''} onClick={() => onSelect(hook.id)}><span>{hook.eyebrow}</span><h4>{hook.title}</h4><p>{hook.hookCopy}</p><small>{hook.description}</small><i>{state.selectedHookId === hook.id ? <Check size={13}/> : null}</i></button>)}</div></section>)}</div> : <section className="short-drama-v2-empty-action"><WandSparkles size={20}/><div><b>生成两类、四个方向</b><small>每类两个候选，方向之间改变的是钩子机制，不只是换一句标题。</small></div><button disabled={!state.summaryDraft || state.hooksStatus === 'loading'} onClick={onGenerate}>{state.hooksStatus === 'loading' ? '生成中…' : '生成方向'}</button></section>}
  </div>
}

function FirstFrameStage({ state, onPrompt, onGenerate, onSelect }: { state: ShortDramaPrerollState; onPrompt: (value: string) => void; onGenerate: () => void; onSelect: (id: string) => void }) {
  return <div className="short-drama-v2-stage"><section className="short-drama-v2-editor-card"><div><span>SEEDREAM IMAGE PROMPT</span><b>首帧图提示词</b></div><textarea value={state.imagePrompt} onChange={event => onPrompt(event.target.value)} rows={6}/><small>提示词包含主体、环境、构图、镜头、光影、风格与禁止项，可人工编辑。</small></section>
    {state.images.length ? <section className="short-drama-v2-image-grid"><header><div><span>FIRST FRAME OPTIONS</span><b>选择一张作为视频起始帧</b></div><button type="button" onClick={onGenerate}><RefreshCw size={14}/>重新生成</button></header><div>{state.images.map(image => <button type="button" key={image.id} className={state.selectedImageId === image.id ? 'selected' : ''} onClick={() => onSelect(image.id)}><img src={image.imageUrl} alt={image.composition}/><span><b>{image.label}</b><small>{image.composition}</small></span><i>{state.selectedImageId === image.id ? <Check size={14}/> : null}</i></button>)}</div></section> : <section className="short-drama-v2-empty-action"><ImageIcon size={20}/><div><b>生成 3 张构图不同的首帧参考</b><small>保持人物、剧情事实与视觉风格一致，只改变画面组织方式。</small></div><button disabled={!state.imagePrompt || state.imagesStatus === 'loading'} onClick={onGenerate}>{state.imagesStatus === 'loading' ? '生成中…' : '生成首帧'}</button></section>}
  </div>
}

function VideoStage({ state, sourceUrl, selectedImageUrl, onDescription, onPrompt, onGenerate }: { state: ShortDramaPrerollState; sourceUrl: string; selectedImageUrl: string; onDescription: (value: string) => void; onPrompt: (value: string) => void; onGenerate: () => void }) {
  return <div className="short-drama-v2-stage"><section className="short-drama-v2-reference-flow"><div><span>START FRAME</span><img src={selectedImageUrl} alt="已选前贴首帧"/></div><ChevronRight/><div><span>TARGET END FRAME</span>{sourceUrl ? <video src={sourceUrl} muted preload="metadata"/> : <div/>}</div><div className="short-drama-v2-reference-meta"><Clock3 size={16}/><b>{state.duration}s</b><small>独立前贴 · 9:16</small></div></section>
    <section className="short-drama-v2-editor-card compact"><div><span>VIDEO DESCRIPTION</span><b>视频描述</b></div><textarea value={state.videoDescription} onChange={event => onDescription(event.target.value)} rows={2}/></section>
    <section className="short-drama-v2-editor-card"><div><span>SEEDANCE VIDEO PROMPT</span><b>前贴视频提示词</b></div><textarea value={state.videoPrompt} onChange={event => onPrompt(event.target.value)} rows={7}/><small>提示词包含时间轴、镜头运动、情绪、字幕与 CTA；尾帧只作为过渡目标参考，不执行拼接。</small></section>
    {state.output ? <section className="short-drama-v2-output"><header><div><span>GENERATED PREROLL</span><b>前贴视频已生成</b></div><small>前端演示结果</small></header>{state.output.videoUrl ? <video src={state.output.videoUrl} controls/> : null}</section> : <section className="short-drama-v2-empty-action"><Clapperboard size={20}/><div><b>参数已就绪</b><small>确认提示词、描述与时长后，生成一条独立前贴视频。</small></div><button disabled={state.videoStatus === 'loading'} onClick={onGenerate}>{state.videoStatus === 'loading' ? '生成中…' : '生成视频'}</button></section>}
  </div>
}
