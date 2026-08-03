import { useEffect, useState } from 'react'
import { Check, FileText, Film, Image, LoaderCircle, Lock, RefreshCw, Sparkles, Upload, WandSparkles } from 'lucide-react'
import { useProject } from '../context/ProjectContext'
import { api, type ApiAssetVersionRef, type ApiBrandBriefAnalysis, type ApiBrandCreativeConcept, type ApiBrandFilmPlan, type ApiBrandFilmWorkspace } from '../data/api'

type Props = { onNotice: (message: string) => void }
const last = <T,>(items?: T[] | null) => items?.at(-1)
const compactLines = (items: string[]) => items.map(item => item.trim()).filter(Boolean)
const editableBriefPayload = (brief: ApiBrandBriefAnalysis): ApiBrandBriefAnalysis => ({
  ...brief,
  mandatory_elements: compactLines(brief.mandatory_elements),
  prohibited_claims: compactLines(brief.prohibited_claims),
  image_requirements: compactLines(brief.image_requirements),
  video_requirements: compactLines(brief.video_requirements),
  uncertainties: compactLines(brief.uncertainties),
})
const fixtureReferenceUri = (role: string, uri = '') => {
  if (uri && uri !== '/assets/guerlain-25x-bee-water.png') return uri
  return role === 'product_front' ? '/assets/guerlain-25x-bee-water-product-front.png' : role === 'logo' ? '/assets/guerlain-logo.png' : uri
}
const briefAssetSource = (sourceLocator: string, role: string) => {
  const match = sourceLocator.match(/page=(\d+)&image=([^&]+)/)
  if (match) return `Brief PDF 第 ${match[1]} 页 · 内嵌图片 ${match[2]}`
  if (sourceLocator.startsWith('fixture://') && role === 'product_front') return 'Brief PDF 第 9 页 · 内嵌图片 IM135'
  if (sourceLocator.startsWith('fixture://') && role === 'logo') return 'Brief PDF 第 1 页 · 内嵌图片 IM17'
  return sourceLocator
}

function ModelBadge({ alias, version }: { alias?: string; version?: string }) {
  if (!alias) return null
  const fallback = alias === 'fixture.deterministic'
  return <span className={fallback ? 'brand-model-badge fallback' : 'brand-model-badge'}>{fallback ? '固定样例回退' : alias}{version ? ` · ${version}` : ''}</span>
}

function ConceptCard({ concept, editing, busy, onChange, onSelect }: {
  concept: ApiBrandCreativeConcept
  editing: boolean
  busy: boolean
  onChange: (changes: Partial<ApiBrandCreativeConcept>) => void
  onSelect: () => void
}) {
  if (editing) return <article className="brand-concept-editor">
    <span>{concept.id}</span>
    <small>仅修改方向级表达；画面、运镜、旁白等执行细节在剧本分镜阶段编辑。</small>
    <label>方向标题<input value={concept.title} onChange={event => onChange({ title: event.target.value })}/></label>
    <label>核心创意句<textarea value={concept.one_liner} onChange={event => onChange({ one_liner: event.target.value })}/></label>
    <label>叙事机制<textarea value={concept.story_mechanism} onChange={event => onChange({ story_mechanism: event.target.value })}/></label>
  </article>
  return <article className={concept.selected ? 'selected' : ''}><span>{concept.id}</span><h4>{concept.title}</h4><b>{concept.one_liner}</b><p>{concept.story_mechanism}</p><dl><div><dt>品牌进入</dt><dd>{concept.brand_entrance}</dd></div><div><dt>视觉语言</dt><dd>{concept.visual_language.join('、')}</dd></div><div><dt>声音</dt><dd>{concept.sound_idea}</dd></div><div><dt>Brief 依据</dt><dd>{concept.brief_rationale}</dd></div><div><dt>风险</dt><dd>{concept.risk}</dd></div></dl><button className={concept.selected ? 'secondary-button' : 'primary-button'} disabled={busy || concept.selected} onClick={onSelect}>{concept.selected ? '已选择' : '选择此方向'}</button></article>
}

function ShotEditor({ shot, disabled, onChange }: { shot: ApiBrandFilmPlan['shots'][number]; disabled: boolean; onChange: (changes: Partial<ApiBrandFilmPlan['shots'][number]>) => void }) {
  return <article><header><b>镜头 {String(shot.order).padStart(2, '0')}</b><span>{shot.start_second}s–{shot.end_second}s</span><small>{shot.purpose}</small></header><div className="brand-shot-fields"><label>镜头目的<input disabled={disabled} value={shot.purpose} onChange={event => onChange({ purpose: event.target.value })}/></label><label>参考角色<input disabled={disabled} value={shot.reference_role} onChange={event => onChange({ reference_role: event.target.value })}/></label><label>画面<textarea disabled={disabled} value={shot.visual} onChange={event => onChange({ visual: event.target.value })}/></label><label>动作<textarea disabled={disabled} value={shot.action} onChange={event => onChange({ action: event.target.value })}/></label><label>运镜<textarea disabled={disabled} value={shot.camera} onChange={event => onChange({ camera: event.target.value })}/></label><label>旁白<textarea disabled={disabled} value={shot.voiceover} onChange={event => onChange({ voiceover: event.target.value })}/></label><label>屏幕字<textarea disabled={disabled} value={shot.on_screen_text} onChange={event => onChange({ on_screen_text: event.target.value })}/></label><label>光线<textarea disabled={disabled} value={shot.lighting} onChange={event => onChange({ lighting: event.target.value })}/></label><label className="wide">连贯性<textarea disabled={disabled} value={shot.continuity_notes} onChange={event => onChange({ continuity_notes: event.target.value })}/></label></div></article>
}

export function BrandFilmWorkspace({ onNotice }: Props) {
  const { currentProject } = useProject()
  const [workspace, setWorkspace] = useState<ApiBrandFilmWorkspace | null>(null)
  const [brief, setBrief] = useState<ApiBrandBriefAnalysis | null>(null)
  const [briefEditMode, setBriefEditMode] = useState(false)
  const [conceptCandidates, setConceptCandidates] = useState<ApiBrandCreativeConcept[]>([])
  const [conceptEditMode, setConceptEditMode] = useState(false)
  const [plan, setPlan] = useState<ApiBrandFilmPlan | null>(null)
  const [planEditMode, setPlanEditMode] = useState(false)
  const [busy, setBusy] = useState('loading')
  const [assetPreviews, setAssetPreviews] = useState<Record<string, string>>({})
  const [generationReference, setGenerationReference] = useState<ApiAssetVersionRef | null>(null)
  const [generationReferencePreview, setGenerationReferencePreview] = useState('')
  const [attemptPreviews, setAttemptPreviews] = useState<Record<string, string>>({})
  const [finalPreview, setFinalPreview] = useState('')
  const [feedbackByUnit, setFeedbackByUnit] = useState<Record<string, string>>({})

  useEffect(() => {
    let active = true
    setBusy('loading')
    void api.ensureBrandFilmFixtureWorkspace(currentProject.id).then(value => { if (active) setWorkspace(value) }).catch(cause => {
      if (active) onNotice(cause instanceof Error ? cause.message : '品牌广告开发样例初始化失败。')
    }).finally(() => { if (active) setBusy('') })
    return () => { active = false }
  }, [currentProject.id, onNotice])

  useEffect(() => {
    if (!workspace) return
    const brand = workspace.video_draft.brand_film
    const currentBrief = last(brand.brief_analysis_versions) ?? null
    const currentConceptSet = last(brand.concept_sets)
    const currentPlan = last(brand.film_plan_versions) ?? null
    setBrief(currentBrief)
    setBriefEditMode(Boolean(currentBrief && !currentBrief.confirmed))
    setConceptCandidates(currentConceptSet?.candidates ?? [])
    setConceptEditMode(false)
    setPlan(currentPlan)
    setPlanEditMode(Boolean(currentPlan && !currentPlan.confirmed))
    const product = last(brand.brief_analysis_versions)?.asset_candidates.find(asset => asset.role === 'product_front')?.asset_ref
    setGenerationReference(brand.generation?.reference_asset ?? product ?? null)
  }, [workspace])

  useEffect(() => {
    const assets = last(workspace?.video_draft.brand_film.brief_analysis_versions)?.asset_candidates ?? []
    let active = true
    void Promise.all(assets.map(async asset => {
      if (asset.asset_ref) return [asset.id, await api.getProjectAssetPreview(currentProject.id, asset.asset_ref)] as const
      return [asset.id, fixtureReferenceUri(asset.role, asset.fixture_uri)] as const
    })).then(entries => {
      if (active) setAssetPreviews(Object.fromEntries(entries))
    }).catch(() => undefined)
    return () => { active = false }
  }, [currentProject.id, workspace])

  useEffect(() => {
    const generation = workspace?.video_draft.brand_film.generation
    if (!generation) return
    const refs = generation.units.flatMap(unit => unit.attempts.filter(attempt => attempt.output_asset_ref).map(attempt => ({ id: attempt.id, ref: attempt.output_asset_ref! })))
    void Promise.all(refs.map(async item => [item.id, await api.getProjectAssetPreview(currentProject.id, item.ref)] as const))
      .then(entries => setAttemptPreviews(Object.fromEntries(entries))).catch(() => undefined)
    if (generation.preview_asset) {
      void api.getProjectAssetPreview(currentProject.id, generation.preview_asset).then(setFinalPreview).catch(() => setFinalPreview(''))
    }
  }, [currentProject.id, workspace?.video_draft.brand_film.generation])

  useEffect(() => () => {
    if (generationReferencePreview) URL.revokeObjectURL(generationReferencePreview)
  }, [generationReferencePreview])

  const commit = async (key: string, action: () => Promise<ApiBrandFilmWorkspace>, message: string) => {
    setBusy(key)
    try {
      const value = await action()
      setWorkspace(value)
      onNotice(message)
      return value
    } catch (cause) {
      onNotice(cause instanceof Error ? cause.message : '品牌广告工作流操作失败。')
      return null
    } finally { setBusy('') }
  }

  if (!workspace) return <div className="brand-film-loading"><LoaderCircle className="spin" size={18}/><span>{busy === 'loading' ? '正在载入娇兰开发样例…' : '尚未建立品牌广告工作区'}</span></div>

  const draft = workspace.video_draft.brand_film
  const conceptSet = last(draft.concept_sets)
  const source = draft.source_snapshot
  const revision = workspace.video_draft.revision
  const lockedBrief = Boolean(brief?.confirmed && !briefEditMode)
  const lockedPlan = Boolean(plan?.confirmed && !planEditMode)
  const planReady = Boolean(plan?.confirmed && !planEditMode)
  const allRequiredReferencesConfirmed = ['product_front', 'logo'].every(role => brief?.asset_candidates.some(asset => asset.role === role && asset.user_confirmed))

  const analyze = () => commit('analyze', () => api.analyzeBrandFilmBrief(currentProject.id, workspace.task.id, revision), 'Brief 已重新解析；请编辑并确认新的修订。')
  const reanalyze = () => {
    if ((draft.concept_sets?.length ?? 0) > 0 && !window.confirm('重新解析会清空当前创意、分镜、生成与交付结果，是否继续？')) return
    return analyze()
  }
  const saveBrief = () => brief && commit('save-brief', () => api.updateBrandFilmBrief(currentProject.id, workspace.task.id, revision, editableBriefPayload(brief)), 'Brief 修改已保存为新的待确认修订，后续内容已等待重新生成。')
  const confirmBrief = async () => {
    if (!brief) return
    setBusy('confirm-brief')
    try {
      const saved = await api.updateBrandFilmBrief(currentProject.id, workspace.task.id, revision, editableBriefPayload(brief))
      setWorkspace(await api.confirmBrandFilmBrief(currentProject.id, workspace.task.id, saved.video_draft.revision))
      onNotice('Brief 与商品参考图已确认。')
    } catch (cause) { onNotice(cause instanceof Error ? cause.message : 'Brief 确认失败。') } finally { setBusy('') }
  }
  const generateConcepts = () => commit('concepts', () => api.generateBrandFilmConcepts(currentProject.id, workspace.task.id, revision), '已生成 3 个有差异的创意方向。')
  const regenerateConcepts = () => {
    if ((draft.film_plan_versions?.length ?? 0) > 0 && !window.confirm('重新生成创意会清空当前选择、分镜、视频生成与交付结果，是否继续？')) return
    return generateConcepts()
  }
  const updateConcept = (conceptId: string, changes: Partial<ApiBrandCreativeConcept>) => setConceptCandidates(items => items.map(item => item.id === conceptId ? { ...item, ...changes } : item))
  const saveConcepts = () => commit('save-concepts', () => api.updateBrandFilmConcepts(currentProject.id, workspace.task.id, revision, conceptCandidates), '创意方向修改已保存为新的待选择修订，后续内容已等待重新生成。')
  const cancelConceptEdit = () => {
    setConceptCandidates(conceptSet?.candidates ?? [])
    setConceptEditMode(false)
  }
  const selectConcept = (conceptId: string) => {
    if (conceptId === draft.selected_concept_id) return
    if (draft.selected_concept_id && !window.confirm('切换创意方向会清空当前剧本、视频生成与交付结果，是否继续？')) return
    return commit('select', () => api.selectBrandFilmConcept(currentProject.id, workspace.task.id, revision, conceptId), '创意方向已选择并冻结。')
  }
  const generatePlan = () => commit('plan', () => api.generateBrandFilmPlan(currentProject.id, workspace.task.id, revision), '15 秒剧本与分镜已生成。')
  const savePlan = () => plan && commit('save-plan', () => api.updateBrandFilmPlan(currentProject.id, workspace.task.id, revision, plan), '剧本与分镜修改已保存为新的待确认修订，视频生成结果已等待重新生成。')
  const confirmPlan = async () => {
    if (!plan) return
    setBusy('confirm-plan')
    try {
      const saved = await api.updateBrandFilmPlan(currentProject.id, workspace.task.id, revision, plan)
      setWorkspace(await api.confirmBrandFilmPlan(currentProject.id, workspace.task.id, saved.video_draft.revision))
      onNotice('剧本与分镜已保存并确认。')
    } catch (cause) { onNotice(cause instanceof Error ? cause.message : '剧本与分镜确认失败。') } finally { setBusy('') }
  }

  const updateAsset = (id: string, changes: Partial<ApiBrandBriefAnalysis['asset_candidates'][number]>) => setBrief(value => value ? { ...value, asset_candidates: value.asset_candidates.map(asset => asset.id === id ? { ...asset, ...changes } : asset) } : value)
  const uploadReferenceAsset = async (assetId: string, file?: File) => {
    if (!file) return
    setBusy('upload')
    try {
      const ref = await api.uploadProjectAsset(currentProject.id, file)
      setAssetPreviews(value => ({ ...value, [assetId]: URL.createObjectURL(file) }))
      updateAsset(assetId, { asset_ref: ref, user_confirmed: true, rights_status: 'user_confirmed', replacement_note: `用户上传：${file.name}` })
      onNotice('参考素材已上传；保存 Brief 后写入新的修订。')
    } catch (cause) { onNotice(cause instanceof Error ? cause.message : '参考素材上传失败。') } finally { setBusy('') }
  }
  const uploadGenerationReference = async (file?: File) => {
    if (!file) return
    setBusy('generation-reference')
    try {
      const ref = await api.uploadProjectAsset(currentProject.id, file)
      setGenerationReference(ref)
      setGenerationReferencePreview(URL.createObjectURL(file))
      onNotice('商品参考图已上传，可用于生成。')
    } catch (cause) { onNotice(cause instanceof Error ? cause.message : '商品参考图上传失败。') } finally { setBusy('') }
  }

  const prepareGeneration = () => generationReference && commit('prepare-generation', () => api.prepareBrandFilmGeneration(currentProject.id, workspace.task.id, revision, generationReference), '已编排 GenerationUnit 并冻结 PromptPackage。')
  const generateUnit = async (unitId: string, feedback = '') => {
    setBusy(`generate-${unitId}`)
    try {
      const created = await api.generateBrandFilmUnit(currentProject.id, workspace.task.id, revision, unitId, feedback)
      setWorkspace(created.workspace)
      let job = created.job
      while (job.status === 'queued' || job.status === 'running') {
        await new Promise(resolve => window.setTimeout(resolve, 2000))
        job = await api.getBrandFilmUnitJob(currentProject.id, job.id)
      }
      setWorkspace(await api.reconcileBrandFilmUnit(currentProject.id, workspace.task.id, unitId, job.id))
      setFeedbackByUnit(value => ({ ...value, [unitId]: '' }))
      onNotice(job.status === 'succeeded' ? 'Seedance 片段已生成，请预览后锁定或反馈重生成。' : `片段生成未成功：${job.diagnostic ?? job.status}`)
    } catch (cause) { onNotice(cause instanceof Error ? cause.message : '片段生成失败。') } finally { setBusy('') }
  }
  const lockUnit = (unitId: string, attemptId: string) => commit(`lock-${unitId}`, () => api.lockBrandFilmUnit(currentProject.id, workspace.task.id, revision, unitId, attemptId), '片段已锁定。')
  const composePreview = () => commit('compose-preview', () => api.composeBrandFilmPreview(currentProject.id, workspace.task.id, revision), '15 秒品牌广告预览已完成裁切与拼接。')

  return <div className="brand-film-workspace">
    <aside className="brand-film-source"><span className="section-label">DEVELOPMENT FIXTURE</span><h3>{source.product_name}</h3><p>{source.brief_name}</p><dl><div><dt>来源</dt><dd>{source.fixture_id} v{source.fixture_version}</dd></div><div><dt>规格</dt><dd>{source.duration_seconds}s · {source.aspect_ratio} · {source.channel}</dd></div><div><dt>任务修订</dt><dd>r{revision}</dd></div></dl><div className="brand-film-stage"><span>当前阶段</span><b>{draft.stage}</b></div><p className="brand-film-scope">视频创作页负责 Brief、创意、分镜与成片生成；后续质量检查和交付统一进入独立中心。</p></aside>
    <main className="brand-film-main">
      <nav className="brand-film-steps" aria-label="品牌广告制作阶段"><span className={brief ? 'done' : 'active'}><b>01</b>Brief 分析确认</span><span className={draft.selected_concept_id ? 'done' : brief?.confirmed ? 'active' : ''}><b>02</b>创意候选选择</span><span className={plan?.confirmed ? 'done' : draft.selected_concept_id ? 'active' : ''}><b>03</b>剧本分镜确认</span><span className={draft.generation?.preview_asset ? 'done' : plan?.confirmed ? 'active' : ''}><b>04</b>生成与锁定</span></nav>

      <section className="brand-film-section"><header><div><span className="section-label">PHASE 01</span><h3>Brief 分析与事实确认</h3></div><ModelBadge alias={brief?.model_alias} version={brief?.model_version}/></header>
        {!brief ? <div className="brand-film-empty"><FileText size={24}/><p>使用 Seed-2-pro 解析固定娇兰 Brief；不可用时回退固定样例。</p><button className="primary-button" disabled={Boolean(busy)} onClick={() => void analyze()}><WandSparkles size={15}/>解析 Brief</button></div> : <>
          <div className="brand-form-grid"><label className="wide">Brief 摘要<textarea disabled={lockedBrief} value={brief.summary} onChange={event => setBrief({ ...brief, summary: event.target.value })}/></label><label>目标人群<textarea disabled={lockedBrief} value={brief.audience} onChange={event => setBrief({ ...brief, audience: event.target.value })}/></label><label>核心传播信息<textarea disabled={lockedBrief} value={brief.core_message} onChange={event => setBrief({ ...brief, core_message: event.target.value })}/></label><label className="wide">统一口播音色<textarea disabled={lockedBrief} value={brief.voice_direction} onChange={event => setBrief({ ...brief, voice_direction: event.target.value })}/></label></div>
          {!lockedBrief && brief.confirmed ? <div className="brand-edit-notice">当前正在修改已确认 Brief。保存后会创建新的待确认修订，并清空依赖旧 Brief 的创意、分镜、生成与交付结果。</div> : null}
          <div className="brand-fact-grid"><div><h4>广告要点 / 卖点</h4>{brief.selling_points.map((fact, index) => <label className="brand-fact" key={`${fact.locator}-${index}`}><input disabled={lockedBrief} value={fact.text} onChange={event => setBrief({ ...brief, selling_points: brief.selling_points.map((item, itemIndex) => itemIndex === index ? { ...item, text: event.target.value } : item) })}/><small>{fact.locator} · {Math.round(fact.confidence * 100)}% · {fact.status}</small></label>)}</div><EditableList title="必须保留" items={brief.mandatory_elements} disabled={lockedBrief} onChange={items => setBrief({ ...brief, mandatory_elements: items })}/><EditableList title="禁用表述" items={brief.prohibited_claims} disabled={lockedBrief} onChange={items => setBrief({ ...brief, prohibited_claims: items })}/><EditableList title="图片要求" items={brief.image_requirements} disabled={lockedBrief} onChange={items => setBrief({ ...brief, image_requirements: items })}/><EditableList title="视频要求" items={brief.video_requirements} disabled={lockedBrief} onChange={items => setBrief({ ...brief, video_requirements: items })}/><EditableList title="待人工确认" items={brief.uncertainties} disabled={lockedBrief} onChange={items => setBrief({ ...brief, uncertainties: items })}/></div>
          <div className="brand-assets"><h4>商品与品牌参考素材</h4><p>以下图片直接提取自当前 Brief PDF；请预览后逐项确认，也可以上传文件替换。</p>{brief.asset_candidates.map(asset => <article key={asset.id}><div className="brand-asset-thumb">{assetPreviews[asset.id] ? <img src={assetPreviews[asset.id]} alt={asset.label}/> : <Image size={22}/>}</div><div><b>{asset.label}</b><small>{briefAssetSource(asset.source_locator, asset.role)}</small><small>{asset.asset_ref ? `已替换为项目素材 · Asset ${asset.asset_ref.asset_id} v${asset.asset_ref.version}` : asset.replacement_note || 'Brief 原始内嵌素材'}</small></div><label className="brand-checkbox"><input type="checkbox" disabled={lockedBrief} checked={asset.user_confirmed} onChange={event => updateAsset(asset.id, { user_confirmed: event.target.checked, rights_status: event.target.checked ? 'user_confirmed' : 'needs_confirmation' })}/><Check size={13}/>确认使用</label>{!lockedBrief ? <label className="secondary-button brand-upload"><Upload size={13}/>替换图片<input type="file" accept="image/png,image/jpeg" onChange={event => { void uploadReferenceAsset(asset.id, event.target.files?.[0]) }}/></label> : null}</article>)}</div>
          <div className="brand-actions">{!lockedBrief ? <><button className="secondary-button" disabled={Boolean(busy)} onClick={() => void reanalyze()}><Sparkles size={14}/>重新解析</button><button className="secondary-button" disabled={Boolean(busy)} onClick={() => void saveBrief()}>保存修改</button><button className="primary-button" disabled={Boolean(busy) || !allRequiredReferencesConfirmed} onClick={() => void confirmBrief()}>确认 Brief</button></> : <><span className="brand-confirmed"><Check size={14}/>Brief 已确认</span><button className="secondary-button" disabled={Boolean(busy)} onClick={() => setBriefEditMode(true)}>编辑 Brief</button><button className="secondary-button" disabled={Boolean(busy)} onClick={() => void reanalyze()}><Sparkles size={14}/>重新解析</button></>}</div>
        </>}
      </section>

      <section className="brand-film-section" aria-disabled={!brief?.confirmed}><header><div><span className="section-label">PHASE 02A</span><h3>有差异的创意方向</h3></div><ModelBadge alias={conceptSet?.model_alias} version={conceptSet?.model_version}/></header>{!brief?.confirmed ? <p className="brand-locked">确认 Brief 后开放。</p> : !conceptSet ? <div className="brand-film-empty compact"><p>一次生成 3 个叙事机制不同的方向，生成后可逐项人工修改。</p><button className="primary-button" disabled={Boolean(busy)} onClick={() => void generateConcepts()}>生成创意候选</button></div> : <>{conceptEditMode ? <div className="brand-edit-notice">这里只修改方向标题、核心创意句与叙事机制。保存后形成新的待选择修订；镜头执行细节继续在剧本分镜阶段处理。</div> : null}<div className="brand-concepts">{conceptCandidates.map(concept => <ConceptCard key={concept.id} concept={concept} editing={conceptEditMode} busy={Boolean(busy)} onChange={changes => updateConcept(concept.id, changes)} onSelect={() => void selectConcept(concept.id)}/>)}</div><div className="brand-actions">{conceptEditMode ? <><button className="secondary-button" disabled={Boolean(busy)} onClick={cancelConceptEdit}>取消编辑，返回选择</button><button className="primary-button" disabled={Boolean(busy)} onClick={() => void saveConcepts()}>保存创意修改</button></> : <>{draft.selected_concept_id ? <span className="brand-confirmed"><Check size={14}/>已选择，可直接切换其他方向</span> : <span className="brand-pending">请选择一个创意方向</span>}<button className="secondary-button" disabled={Boolean(busy)} onClick={() => setConceptEditMode(true)}>编辑方向文案</button><button className="secondary-button" disabled={Boolean(busy)} onClick={() => void regenerateConcepts()}>重新生成整组</button></>}</div></>}</section>

      <section className="brand-film-section" aria-disabled={!draft.selected_concept_id || conceptEditMode}><header><div><span className="section-label">PHASE 02B</span><h3>15 秒剧本与镜头表</h3></div><ModelBadge alias={plan?.model_alias} version={plan?.model_version}/></header>{conceptEditMode ? <p className="brand-locked">请先保存创意修改并重新选择方向。</p> : !draft.selected_concept_id ? <p className="brand-locked">选择创意方向后开放。</p> : !plan ? <div className="brand-film-empty compact"><p>用户编辑剧本、旁白和镜头字段，底层 Prompt 不直接暴露。</p><button className="primary-button" disabled={Boolean(busy)} onClick={() => void generatePlan()}>生成剧本与分镜</button></div> : <>{!lockedPlan && plan.confirmed ? <div className="brand-edit-notice">当前正在修改已确认剧本。保存后会形成新的待确认修订，并清空旧视频生成与交付结果。</div> : null}<div className="brand-form-grid"><label>片名<input disabled={lockedPlan} value={plan.title} onChange={event => setPlan({ ...plan, title: event.target.value })}/></label><label>音乐方向<input disabled={lockedPlan} value={plan.music_direction} onChange={event => setPlan({ ...plan, music_direction: event.target.value })}/></label><label className="wide">故事概要<textarea disabled={lockedPlan} value={plan.story_summary} onChange={event => setPlan({ ...plan, story_summary: event.target.value })}/></label><label className="wide">口播方向<textarea disabled={lockedPlan} value={plan.voice_direction} onChange={event => setPlan({ ...plan, voice_direction: event.target.value })}/></label></div><div className="brand-shot-list">{plan.shots.map((shot, index) => <ShotEditor key={shot.id} shot={shot} disabled={lockedPlan} onChange={changes => setPlan({ ...plan, shots: plan.shots.map((item, itemIndex) => itemIndex === index ? { ...item, ...changes } : item) })}/>)}</div><div className="brand-actions">{!lockedPlan ? <><button className="secondary-button" disabled={Boolean(busy)} onClick={() => void generatePlan()}>重新生成整版</button><button className="primary-button" disabled={Boolean(busy)} onClick={() => void savePlan()}>保存剧本修改</button><button className="secondary-button" disabled={Boolean(busy)} onClick={() => void confirmPlan()}>确认剧本与分镜</button></> : <><span className="brand-confirmed"><Check size={14}/>剧本与分镜已确认</span><button className="secondary-button" disabled={Boolean(busy)} onClick={() => setPlanEditMode(true)}>编辑剧本与分镜</button></>}</div></>}</section>

      <section className="brand-film-section" aria-disabled={!planReady}><header><div><span className="section-label">PHASE 03</span><h3>视频生成、反馈重试与片段锁定</h3></div><span className="brand-model-badge">Seedance 2.0 · 单候选</span></header>{!planReady ? <p className="brand-locked">保存并确认剧本与分镜后开放。</p> : !draft.generation ? <div className="brand-film-empty"><Film size={24}/><p>确认商品参考图后，系统把镜头表编排为 4–15 秒生成单元并冻结 PromptPackage。</p><div className="brand-generation-reference">{generationReferencePreview ? <img src={generationReferencePreview} alt="商品参考图"/> : generationReference ? <span>Asset {generationReference.asset_id} v{generationReference.version}</span> : <span>尚未选择商品参考图</span>}<label className="secondary-button brand-upload"><Upload size={13}/>上传 / 更换<input type="file" accept="image/png,image/jpeg" onChange={event => { void uploadGenerationReference(event.target.files?.[0]) }}/></label></div><button className="primary-button" disabled={Boolean(busy) || !generationReference} onClick={() => void prepareGeneration()}>确认并编排生成单元</button></div> : <><div className="brand-generation-units">{draft.generation.units.map(unit => { const latestAttempt = unit.attempts.at(-1); const locked = Boolean(unit.locked_attempt_id); return <article key={unit.id} className={locked ? 'locked' : ''}><header><div><b>生成单元 {String(unit.order).padStart(2, '0')}</b><span>{unit.start_second}s–{unit.end_second}s · {unit.shot_ids.join(' + ')}</span></div>{locked ? <span className="brand-confirmed"><Lock size={13}/>已锁定</span> : null}</header><small>PromptPackage r{unit.prompt_packages.at(-1)?.revision} · {unit.prompt_packages.at(-1)?.content_hash.slice(0, 18)}…</small>{latestAttempt?.output_asset_ref ? <video controls src={attemptPreviews[latestAttempt.id]}/> : <div className="brand-unit-placeholder"><Film size={20}/><span>{latestAttempt ? `Attempt ${latestAttempt.ordinal} · ${latestAttempt.status}` : '尚未生成候选'}</span></div>}{!locked ? <div className="brand-unit-actions">{!latestAttempt ? <button className="primary-button" disabled={Boolean(busy)} onClick={() => void generateUnit(unit.id)}>生成此片段</button> : <><textarea placeholder="填写局部反馈，例如：稳定瓶身标签，减少镜头环绕" value={feedbackByUnit[unit.id] ?? ''} onChange={event => setFeedbackByUnit(value => ({ ...value, [unit.id]: event.target.value }))}/><button className="secondary-button" disabled={Boolean(busy) || !(feedbackByUnit[unit.id] ?? '').trim()} onClick={() => void generateUnit(unit.id, feedbackByUnit[unit.id])}><RefreshCw size={13}/>按反馈重生成</button>{latestAttempt.output_asset_ref ? <button className="primary-button" disabled={Boolean(busy)} onClick={() => void lockUnit(unit.id, latestAttempt.id)}><Lock size={13}/>锁定此片段</button> : null}</>}</div> : null}</article>})}</div>{finalPreview ? <div className="brand-final-preview"><div><span className="section-label">15 SECOND PREVIEW</span><h4>已锁定片段合成预览</h4><small>720×1280 · H.264/AAC · 项目素材可追溯</small></div><video controls src={finalPreview}/></div> : null}<div className="brand-actions"><button className="primary-button" disabled={Boolean(busy) || draft.generation.units.some(unit => !unit.locked_attempt_id) || Boolean(draft.generation.preview_asset)} onClick={() => void composePreview()}>裁切并拼接 15 秒预览</button>{draft.generation.preview_asset ? <span className="brand-confirmed"><Check size={14}/>预览 Asset {draft.generation.preview_asset.asset_id} v{draft.generation.preview_asset.version}</span> : null}</div></>}</section>

      {draft.generation?.preview_asset && planReady && !conceptEditMode ? <footer className="brand-generation-seam"><b>成片预览已完成</b><span>下一步：素材检查 → 交付中心</span><small>视频创作页不再重复承载质检、审批和交付操作，请通过左侧独立中心继续处理。</small></footer> : null}
    </main>
  </div>
}

function EditableList({ title, items, disabled, onChange }: { title: string; items: string[]; disabled: boolean; onChange: (items: string[]) => void }) {
  return <label><h4>{title}</h4><textarea disabled={disabled} value={items.join('\n')} onChange={event => onChange(event.target.value.split('\n'))}/></label>
}
