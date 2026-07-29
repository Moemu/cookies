import { useCallback, useEffect, useMemo, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { buildBulkRemixPlan, isVideoAsset, type BulkRemixPlan, type RemixPace, type RemixSegment, type RemixSelection } from './aiRemixPlanner'
import { applyRemixPreroll, createAgentRun, createEvalRun, createFeedbackEvent, createQualityReport, createRemixPlan, createRemixPreroll, createRemixRenderJob, getAssetPreview, getRenderJobQualityReport, getRemixPlan, getRemixRenderJob, listAssetFeatures, listProjectAssets, listRemixPlans, searchKnowledge, type AgentRun, type EvalRun, type FeedbackEvent, type KnowledgeCitation, type QualityReport, type RemixPreroll, type RemixRenderJob, type SavedRemixPlan } from './api'
import type { AssetFeature, ProjectAsset } from './types'

const segmentCopy: Record<RemixSegment, { label: string; hint: string }> = {
  opening: { label: '前段', hint: '开场钩子，适合强视觉、强动作、强利益点。' },
  middle: { label: '中段', hint: '承接叙事，适合过程、证明、卖点展开。' },
  ending: { label: '后段', hint: '收束转化，适合结果展示、品牌记忆、CTA。' },
}

const paceLabels: Record<RemixPace, string> = {
  fast: '快节奏',
  balanced: '均衡',
  story: '叙事',
}

const segmentOrder: RemixSegment[] = ['opening', 'middle', 'ending']

function assetKey(asset: ProjectAsset) {
  return `${asset.asset.id}:${asset.version.version}`
}

function formatBytes(bytes: number) {
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`
}

function assetDimensions(asset: ProjectAsset) {
  if (!asset.version.width_pixels || !asset.version.height_pixels) return '尺寸未记录'
  return `${asset.version.width_pixels} × ${asset.version.height_pixels}`
}

function featureSummary(feature?: AssetFeature) {
  if (!feature) return '暂无特征 · 使用基础元数据'
  const point = feature.selling_points[0] ? ` · ${feature.selling_points[0]}` : ''
  return `Hook ${Math.round(feature.hook_strength * 100)} · 商品露出 ${Math.round(feature.product_visibility * 100)} · 风险 ${feature.similarity_risk}${point}`
}

function qualityVerdictLabel(verdict: QualityReport['verdict']) {
  return verdict === 'critical' ? '严重阻断' : verdict === 'major' ? '需要复核' : '质检通过'
}

export function ProjectAssetRemixPage() {
  const { projectId = '' } = useParams()
  const [assets, setAssets] = useState<ProjectAsset[]>([])
  const [features, setFeatures] = useState<AssetFeature[]>([])
  const [previewUrls, setPreviewUrls] = useState<Record<string, string>>({})
  const [query, setQuery] = useState('')
  const [targetSeconds, setTargetSeconds] = useState(30)
  const [pace, setPace] = useState<RemixPace>('balanced')
  const [selection, setSelection] = useState<Record<RemixSegment, Set<string>>>(() => ({
    opening: new Set(),
    middle: new Set(),
    ending: new Set(),
  }))
  const [plan, setPlan] = useState<BulkRemixPlan | null>(null)
  const [savedPlan, setSavedPlan] = useState<SavedRemixPlan | null>(null)
  const [savedPlans, setSavedPlans] = useState<SavedRemixPlan[]>([])
  const [renderJob, setRenderJob] = useState<RemixRenderJob | null>(null)
  const [qualityReport, setQualityReport] = useState<QualityReport | null>(null)
  const [preroll, setPreroll] = useState<RemixPreroll | null>(null)
  const [agentRun, setAgentRun] = useState<AgentRun | null>(null)
  const [knowledgeCitations, setKnowledgeCitations] = useState<KnowledgeCitation[]>([])
  const [evalRun, setEvalRun] = useState<EvalRun | null>(null)
  const [feedbackDraft, setFeedbackDraft] = useState({ rating: 5, comment: '节奏清晰，卖点明确。' })
  const [feedbackError, setFeedbackError] = useState('')
  const [feedbackEvent, setFeedbackEvent] = useState<FeedbackEvent | null>(null)
  const [savingPlan, setSavingPlan] = useState(false)
  const [submittingRender, setSubmittingRender] = useState(false)
  const [runningPreroll, setRunningPreroll] = useState(false)
  const [applyingPreroll, setApplyingPreroll] = useState(false)
  const [runningAgent, setRunningAgent] = useState(false)
  const [runningEval, setRunningEval] = useState(false)
  const [submittingFeedback, setSubmittingFeedback] = useState(false)
  const [loadingSavedPlan, setLoadingSavedPlan] = useState('')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [copied, setCopied] = useState(false)

  const loadAssets = useCallback(async (signal?: AbortSignal) => {
    setLoading(true)
    setError('')
    try {
      const [result, planResult, featureResult] = await Promise.all([
        listProjectAssets(projectId, signal),
        listRemixPlans(projectId, 8, signal),
        listAssetFeatures(projectId, signal),
      ])
      const videos = result.items.filter((asset) => asset.asset.status === 'ready' && isVideoAsset(asset))
      setAssets(videos)
      setSavedPlans(planResult.items)
      setFeatures(featureResult.items)
      const previews = await Promise.all(videos.slice(0, 80).map(async (asset) => {
        try {
          const signed = await getAssetPreview(projectId, asset.asset.id, asset.version.version, signal)
          return [assetKey(asset), signed.url] as const
        } catch (caught) {
          if (caught instanceof DOMException && caught.name === 'AbortError') throw caught
          return [assetKey(asset), ''] as const
        }
      }))
      setPreviewUrls(Object.fromEntries(previews.filter(([, url]) => Boolean(url))))
    } catch (caught) {
      if (caught instanceof DOMException && caught.name === 'AbortError') return
      setError(caught instanceof Error ? caught.message : '混剪素材加载失败。')
    } finally {
      if (!signal?.aborted) setLoading(false)
    }
  }, [projectId])

  useEffect(() => {
    const controller = new AbortController()
    const task = window.setTimeout(() => void loadAssets(controller.signal), 0)
    return () => {
      window.clearTimeout(task)
      controller.abort()
    }
  }, [loadAssets])

  const assetByKey = useMemo(() => new Map(assets.map((asset) => [assetKey(asset), asset])), [assets])
  const featuresByAsset = useMemo(() => new Map(features.map((feature) => [`${feature.asset_id}:${feature.asset_version}`, feature])), [features])
  const filteredAssets = useMemo(() => {
    const keyword = query.trim().toLowerCase()
    if (!keyword) return assets
    return assets.filter((asset) => `${asset.asset.id} ${asset.version.mime_type} ${asset.version.source_type}`.toLowerCase().includes(keyword))
  }, [assets, query])
  const selectedCounts = segmentOrder.map((segment) => selection[segment].size)
  const totalSelected = selectedCounts.reduce((sum, count) => sum + count, 0)
  const referenceAsset = useMemo(() => {
    const key = selection.opening.values().next().value ?? selection.middle.values().next().value ?? selection.ending.values().next().value
    const asset = typeof key === 'string' ? assetByKey.get(key) : undefined
    return asset ? { asset_id: asset.asset.id, version: asset.version.version } : null
  }, [assetByKey, selection])

  function toggleAsset(segment: RemixSegment, key: string) {
    setPlan(null)
    setSavedPlan(null)
    setRenderJob(null)
    setQualityReport(null)
    setPreroll(null)
    setAgentRun(null)
    setKnowledgeCitations([])
    setEvalRun(null)
    setFeedbackEvent(null)
    setFeedbackError('')
    setSelection((current) => {
      const next = { opening: new Set(current.opening), middle: new Set(current.middle), ending: new Set(current.ending) }
      if (next[segment].has(key)) next[segment].delete(key)
      else next[segment].add(key)
      return next
    })
  }

  function buildSelection(): RemixSelection {
    return {
      opening: Array.from(selection.opening).map((key) => assetByKey.get(key)).filter(Boolean) as ProjectAsset[],
      middle: Array.from(selection.middle).map((key) => assetByKey.get(key)).filter(Boolean) as ProjectAsset[],
      ending: Array.from(selection.ending).map((key) => assetByKey.get(key)).filter(Boolean) as ProjectAsset[],
    }
  }

  async function generatePlan() {
    setCopied(false)
    setSavedPlan(null)
    setRenderJob(null)
    setError('')
    setSavingPlan(true)
    try {
      const draft = buildBulkRemixPlan({ selection: buildSelection(), targetSeconds, pace, assetFeatures: features })
      setPlan(draft)
      const created = await createRemixPlan(projectId, draft)
      const hydrated = await getRemixPlan(projectId, created.id)
      setSavedPlan(hydrated)
      try {
        const knowledge = await searchKnowledge(projectId, `${draft.summary.strategy} ${draft.warnings.join(' ')}`)
        setKnowledgeCitations(knowledge.items.flatMap((item) => item.citations).slice(0, 3))
      } catch {
        setKnowledgeCitations([])
      }
      const refreshed = await listRemixPlans(projectId, 8)
      setSavedPlans(refreshed.items)
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : '混剪草案保存失败。')
    } finally {
      setSavingPlan(false)
    }
  }

  async function copyPlan() {
    if (!plan) return
    await navigator.clipboard.writeText(JSON.stringify({ draft: plan, saved: savedPlan }, null, 2))
    setCopied(true)
  }

  async function openSavedPlan(planId: string) {
    setError('')
    setLoadingSavedPlan(planId)
    try {
      setSavedPlan(await getRemixPlan(projectId, planId))
      setRenderJob(null)
      setQualityReport(null)
      setPreroll(null)
      setAgentRun(null)
      setKnowledgeCitations([])
      setEvalRun(null)
      setFeedbackEvent(null)
      setFeedbackError('')
      setPlan(null)
      setCopied(false)
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : '历史混剪草案读取失败。')
    } finally {
      setLoadingSavedPlan('')
    }
  }

  async function submitRenderJob() {
    if (!savedPlan) return
    setError('')
    setSubmittingRender(true)
    try {
      const created = await createRemixRenderJob(projectId, savedPlan.id, 'draft')
      setRenderJob(await getRemixRenderJob(projectId, created.id))
      setQualityReport(null)
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : '渲染任务提交失败。')
    } finally {
      setSubmittingRender(false)
    }
  }

  async function runQualityCheck() {
    if (!renderJob) return
    setError('')
    try {
      const report = await createQualityReport(projectId, renderJob.id)
      setQualityReport(report)
      setRenderJob(await getRemixRenderJob(projectId, renderJob.id))
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : '质量检查失败。')
    }
  }

  async function loadQualityReport() {
    if (!renderJob) return
    setError('')
    try {
      const result = await getRenderJobQualityReport(projectId, renderJob.id)
      setQualityReport(result.quality_report)
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : '质量报告读取失败。')
    }
  }

  async function generatePreroll(constraints: string[] = []) {
    if (!savedPlan || !referenceAsset) return
    setError('')
    setRunningPreroll(true)
    try {
      const result = await createRemixPreroll(projectId, savedPlan.id, referenceAsset, constraints)
      setPreroll(result)
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : 'AI 前贴生成失败。')
    } finally {
      setRunningPreroll(false)
    }
  }

  async function applyPrerollToPlan() {
    if (!preroll) return
    setError('')
    setApplyingPreroll(true)
    try {
      setSavedPlan(await applyRemixPreroll(projectId, preroll.id))
      setPreroll({ ...preroll, status: 'applied' })
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : '前贴插入失败，请重新生成或调整输入。')
    } finally {
      setApplyingPreroll(false)
    }
  }

  async function runAgentDiagnosis() {
    if (!renderJob) return
    setError('')
    setRunningAgent(true)
    try {
      setAgentRun(await createAgentRun(projectId, renderJob.id))
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : 'Agent 诊断启动失败。')
    } finally {
      setRunningAgent(false)
    }
  }

  async function runEval() {
    setError('')
    setRunningEval(true)
    try {
      setEvalRun(await createEvalRun(projectId))
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : 'Remix-MMLU 评测运行失败。')
    } finally {
      setRunningEval(false)
    }
  }

  async function submitFeedback() {
    if (!savedPlan && !renderJob?.output_asset) return
    setFeedbackError('')
    setSubmittingFeedback(true)
    try {
      const output = renderJob?.output_asset?.asset_version
      const targetID = output?.asset_id ?? savedPlan?.id ?? ''
      setFeedbackEvent(await createFeedbackEvent(projectId, targetID, feedbackDraft.rating, feedbackDraft.comment, output))
    } catch (caught) {
      setFeedbackError(caught instanceof Error ? caught.message : '评分提交失败，已保留输入，可重试。')
    } finally {
      setSubmittingFeedback(false)
    }
  }

  return <section className="remix-page">
    <header className="page-header">
      <div>
        <h1>AI 海量素材混剪</h1>
        <p>把视频素材分配到前、中、后三段，生成可解释的混剪时间线草案。当前版本先做智能编排，后续接入 RenderJob 实际合成。</p>
      </div>
      <Link className="button button--secondary" to={`/projects/${encodeURIComponent(projectId)}/assets`}>返回素材库</Link>
    </header>

    <div className="remix-hero" aria-label="混剪策略">
      <div><span>三段叙事</span><strong>Hook → Proof → CTA</strong></div>
      <div><span>已选素材</span><strong>{totalSelected}</strong></div>
      <div><span>目标时长</span><strong>{targetSeconds}s</strong></div>
      <p>Planner 会按新鲜度、来源、画幅、尺寸完整度和节奏风格打分，并输出每个 clip 的选择理由。</p>
    </div>

    {savedPlans.length > 0 ? <section className="remix-history" aria-label="最近混剪草案">
      <div className="remix-section-heading"><div><h2>最近草案</h2><p>来自服务端真实接口，可回读已保存的混剪计划。</p></div></div>
      <div className="remix-history-list">
        {savedPlans.map((item) => <button className={savedPlan?.id === item.id ? 'is-active' : ''} disabled={loadingSavedPlan === item.id} key={item.id} onClick={() => void openSavedPlan(item.id)} type="button">
          <strong>{item.id}</strong>
          <span>{item.pace} · {item.actual_seconds}s · {item.summary.used_assets} 个素材</span>
        </button>)}
      </div>
    </section> : null}

    <div className="remix-controls">
      <label className="search-control"><span className="sr-only">搜索视频素材</span><input onChange={(event) => setQuery(event.target.value)} placeholder="搜索视频资产 ID / MIME / 来源" value={query} /></label>
      <label className="select-control"><span className="sr-only">节奏风格</span><select onChange={(event) => setPace(event.target.value as RemixPace)} value={pace}>{Object.entries(paceLabels).map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select></label>
      <label className="remix-duration"><span>目标时长</span><input max={180} min={9} onChange={(event) => setTargetSeconds(Number(event.target.value))} type="number" value={targetSeconds} /></label>
      <button className="button button--primary" disabled={totalSelected === 0 || savingPlan} onClick={() => void generatePlan()} type="button">{savingPlan ? '保存草案中…' : '生成混剪草案'}</button>
    </div>

    {error ? <div className="library-error" role="alert"><div><strong>无法加载混剪素材</strong><span>{error}</span></div><button className="text-button" onClick={() => void loadAssets()} type="button">重试</button></div> : null}
    {loading ? <div className="asset-loading remix-loading" aria-label="正在加载视频素材"><span /><span /><span /></div> : null}

    {!loading && assets.length === 0 ? <div className="library-empty">
      <div className="empty-image">🎞</div>
      <h2>当前素材库没有可混剪的视频</h2>
      <p>需要先导入或生成 video 类型素材。图片素材不会进入混剪候选，避免生成不可渲染的时间线。</p>
    </div> : null}

    {assets.length > 0 ? <div className="remix-layout">
      <section className="remix-bank">
        <div className="remix-section-heading"><h2>视频素材池</h2><span>{filteredAssets.length} / {assets.length}</span></div>
        <div className="remix-asset-list">
          {filteredAssets.map((asset) => {
            const key = assetKey(asset)
            return <article className="remix-asset" key={key}>
              <div className="remix-thumb">{previewUrls[key] ? <video muted playsInline preload="metadata" src={previewUrls[key]} /> : <span>{asset.version.mime_type.replace('video/', '').toUpperCase()}</span>}</div>
              <div className="remix-asset__body">
                <strong title={asset.asset.id}>{asset.asset.id}</strong>
                <span>{assetDimensions(asset)} · {formatBytes(asset.version.size_bytes)} · {asset.version.source_type}</span>
                <small className="asset-feature-summary">{featureSummary(featuresByAsset.get(key))}</small>
                <div className="remix-assignments">
                  {segmentOrder.map((segment) => <button aria-pressed={selection[segment].has(key)} key={segment} onClick={() => toggleAsset(segment, key)} type="button">{segmentCopy[segment].label}</button>)}
                </div>
              </div>
            </article>
          })}
        </div>
      </section>

      <section className="remix-segments" aria-label="三段素材选择">
        {segmentOrder.map((segment, index) => <div className={`remix-segment remix-segment--${segment}`} key={segment}>
          <div className="remix-segment__header"><span>0{index + 1}</span><div><h2>{segmentCopy[segment].label}</h2><p>{segmentCopy[segment].hint}</p></div><strong>{selection[segment].size}</strong></div>
          <div className="remix-selected-list">
            {Array.from(selection[segment]).map((key) => {
              const asset = assetByKey.get(key)
              if (!asset) return null
              return <button key={key} onClick={() => toggleAsset(segment, key)} title="点击移出该段" type="button">{asset.asset.id}<span>v{asset.version.version}</span></button>
            })}
            {selection[segment].size === 0 ? <p>尚未选择素材。</p> : null}
          </div>
        </div>)}
      </section>
    </div> : null}

    {plan || savedPlan ? <section className="remix-plan">
      <div className="remix-section-heading">
        <div><h2>AI 混剪时间线草案</h2><p>{plan?.summary.strategy ?? savedPlan?.summary.strategy}</p></div>
        <div className="remix-plan-actions">
          {savedPlan ? <button className="button button--primary" disabled={submittingRender} onClick={() => void submitRenderJob()} type="button">{submittingRender ? '提交中…' : '提交渲染'}</button> : null}
          {plan ? <button className="button button--secondary" onClick={() => void copyPlan()} type="button">{copied ? '已复制 JSON' : '复制 JSON'}</button> : null}
        </div>
      </div>
      <div className="remix-plan-summary">
        <div><span>服务端草案</span><strong>{savedPlan ? savedPlan.id : '保存中'}</strong></div>
        <div><span>实际时长</span><strong>{plan?.actualSeconds ?? savedPlan?.actual_seconds}s</strong></div>
        <div><span>目标覆盖</span><strong>{plan?.summary.coveragePercent ?? savedPlan?.summary.coverage_percent}%</strong></div>
        <div><span>使用素材</span><strong>{plan?.summary.usedAssets ?? savedPlan?.summary.used_assets}</strong></div>
      </div>
      {(plan?.warnings ?? savedPlan?.warnings ?? []).length > 0 ? <div className="remix-warnings">{(plan?.warnings ?? savedPlan?.warnings ?? []).map((warning) => <span key={warning}>{warning}</span>)}</div> : null}
      <div className="remix-ai-panels">
        <section className="remix-ai-panel" aria-label="Knowledge citation 来源">
          <div className="remix-ai-panel__heading"><h3>Knowledge Citations</h3><span>{knowledgeCitations.length} 条</span></div>
          {knowledgeCitations.length > 0 ? <ul>{knowledgeCitations.map((citation) => <li key={`${citation.document_id}:${citation.chunk_id}`}><strong>{citation.title}</strong><span>{citation.section} L{citation.start_line}-L{citation.end_line}</span><p>{citation.snippet}</p></li>)}</ul> : <p>生成草案后会搜索策略库，并在这里展示 Planner 可追溯引用来源。</p>}
        </section>
        <section className="remix-ai-panel" aria-label="AI 前贴失败恢复">
          <div className="remix-ai-panel__heading"><h3>AI 前贴 Hook</h3><span>{preroll?.status ?? '未生成'}</span></div>
          {preroll ? <div className={preroll.status === 'failed' ? 'remix-recoverable-error' : 'remix-preroll-result'} role={preroll.status === 'failed' ? 'alert' : 'status'}>
            <strong>{preroll.quality_verdict === 'pass' ? '质检通过' : '质检未通过'}</strong>
            <p>{preroll.error_message || preroll.prompt_draft}</p>
          </div> : <p>用首个已选素材生成 3-10 秒 opening 前贴，失败时保留原因并允许重试。</p>}
          <div className="remix-panel-actions">
            <button className="button button--secondary" disabled={!savedPlan || !referenceAsset || runningPreroll} onClick={() => void generatePreroll()} type="button">{runningPreroll ? '生成中…' : '生成前贴'}</button>
            <button className="button button--secondary" disabled={!savedPlan || !referenceAsset || runningPreroll} onClick={() => void generatePreroll(['quality:critical'])} type="button">模拟失败重试</button>
            <button className="button button--primary" disabled={preroll?.status !== 'ready' || applyingPreroll} onClick={() => void applyPrerollToPlan()} type="button">{applyingPreroll ? '插入中…' : '插入 opening'}</button>
          </div>
        </section>
      </div>
      {renderJob ? <div className="remix-render-status">
        <span>渲染任务</span>
        <strong>{renderJob.id}</strong>
        <em>{renderJob.status} · {renderJob.progress}% · {renderJob.target_quality} · {renderJob.target_format}{renderJob.requires_review ? ' · 需复核' : ''}</em>
        <div className="remix-render-actions">
          <button className="button button--secondary" disabled={Boolean(qualityReport)} onClick={() => void runQualityCheck()} type="button">执行质检</button>
          <button className="button button--secondary" onClick={() => void loadQualityReport()} type="button">读取质检报告</button>
          <button className="button button--secondary" disabled={runningAgent} onClick={() => void runAgentDiagnosis()} type="button">{runningAgent ? '诊断中…' : 'Agent 诊断'}</button>
        </div>
      </div> : null}
      {qualityReport ? <div className={`remix-quality-report remix-quality-report--${qualityReport.verdict}`}>
        <div><span>QualityReport</span><strong>{qualityVerdictLabel(qualityReport.verdict)} · {Math.round(qualityReport.score * 100)}分</strong></div>
        <div className="remix-quality-dimensions">{qualityReport.dimensions.slice(0, 4).map((dimension) => <article key={dimension.name}><span>{dimension.name}</span><strong>{Math.round(dimension.score * 100)}%</strong><p>{dimension.summary}</p></article>)}</div>
        {qualityReport.issues.length > 0 ? <ul>{qualityReport.issues.map((issue) => <li key={issue.code}><strong>{issue.severity} · {issue.dimension}</strong><span>{issue.start_seconds.toFixed(1)}s-{issue.end_seconds.toFixed(1)}s · {issue.description}</span><em>{issue.repair_suggestion}</em></li>)}</ul> : <p>未发现 critical/major 质量问题，可继续进入成片回流。</p>}
      </div> : null}
      {agentRun ? <section className="remix-agent-trace" aria-label="Agent Trace UI">
        <div className="remix-ai-panel__heading"><h3>Agent Trace</h3><span>{agentRun.status}</span></div>
        <div className="remix-trace-grid">
          <div><strong>步骤</strong>{agentRun.steps.map((step) => <p key={step.id}>{step.label} · {step.status}<span>{step.summary}</span></p>)}</div>
          <div><strong>工具调用</strong>{agentRun.tool_calls.map((tool) => <p key={tool.id}>{tool.name} · {tool.status}<span>{tool.error_message || `${tool.references?.length ?? 0} 个引用`}</span></p>)}</div>
          <div><strong>模型 Span</strong>{agentRun.trace_spans.map((span) => <p key={span.id}>{span.kind}:{span.name} · {span.status}<span>{span.model || span.error_message || span.parent_id || 'root'}</span></p>)}</div>
        </div>
        <button className="button button--secondary" onClick={() => void runAgentDiagnosis()} type="button">重试 Agent 诊断</button>
      </section> : null}
      <div className="remix-ai-panels">
        <section className="remix-ai-panel" aria-label="Remix-MMLU 评测结果">
          <div className="remix-ai-panel__heading"><h3>Remix-MMLU</h3><span>{evalRun ? `${Math.round(evalRun.score * 100)}分` : '未运行'}</span></div>
          {evalRun ? <ul>{evalRun.results.map((result) => <li key={result.id}><strong>{result.case_id}</strong><span>{result.passed ? '通过' : '失败'} · {Math.round(result.score * 100)}分</span><p>{result.reason}</p></li>)}</ul> : <p>运行固定评测集，查看分数、失败样例和 Planner/Prompt 版本。</p>}
          <div className="remix-panel-actions"><button className="button button--secondary" disabled={runningEval} onClick={() => void runEval()} type="button">{runningEval ? '评测中…' : '运行评测'}</button></div>
        </section>
        <section className="remix-ai-panel" aria-label="Feedback 评分">
          <div className="remix-ai-panel__heading"><h3>Feedback</h3><span>{feedbackEvent ? '已提交' : '待评分'}</span></div>
          <label className="remix-feedback-field"><span>评分</span><input aria-label="反馈评分" max={5} min={1} onChange={(event) => setFeedbackDraft((current) => ({ ...current, rating: Number(event.target.value) }))} type="number" value={feedbackDraft.rating} /></label>
          <label className="remix-feedback-field"><span>评论</span><textarea aria-label="反馈评论" onChange={(event) => setFeedbackDraft((current) => ({ ...current, comment: event.target.value }))} value={feedbackDraft.comment} /></label>
          {feedbackError ? <p className="remix-recoverable-error" role="alert">{feedbackError}</p> : null}
          {feedbackEvent ? <p className="remix-preroll-result" role="status">评分已记录为 append-only 事件：{feedbackEvent.id}</p> : null}
          <div className="remix-panel-actions"><button className="button button--primary" disabled={submittingFeedback} onClick={() => void submitFeedback()} type="button">{submittingFeedback ? '提交中…' : '提交评分'}</button></div>
        </section>
      </div>
      {plan ? <div className="remix-timeline">
        {plan.segments.map((segment) => <div className="remix-timeline-segment" key={segment.segment}>
          <header><strong>{segment.label}</strong><span>{segment.actualSeconds}s / {segment.targetSeconds}s</span></header>
          {segment.clips.map((clip) => <article className="remix-clip" key={clip.id} style={{ ['--clip-width' as string]: `${Math.max(18, clip.durationSeconds * 18)}px` }}>
            <strong>{clip.label}</strong>
            <span>{clip.startSeconds}s - {(clip.startSeconds + clip.durationSeconds).toFixed(1)}s · {Math.round(clip.score * 100)}</span>
            <p>{clip.reason}</p>
            {(segment.shots.find((shot) => shot.id === clip.id)?.risks ?? []).length > 0 ? <div className="remix-risk-list" aria-label={`${clip.label} 风险`}>{segment.shots.find((shot) => shot.id === clip.id)?.risks.map((risk) => <em key={risk}>{risk}</em>)}</div> : null}
          </article>)}
          {segment.clips.length === 0 ? <p className="remix-empty-segment">该段没有可用 clip。</p> : null}
        </div>)}
      </div> : <pre className="remix-saved-json">{JSON.stringify(savedPlan, null, 2)}</pre>}
    </section> : null}
  </section>
}
