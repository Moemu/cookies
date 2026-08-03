import { useEffect, useState } from 'react'
import { Check, FileText, Image, LoaderCircle, Sparkles, Upload, WandSparkles } from 'lucide-react'
import { useProject } from '../context/ProjectContext'
import {
  api,
  type ApiBrandBriefAnalysis,
  type ApiBrandFilmPlan,
  type ApiBrandFilmWorkspace,
} from '../data/api'

type Props = { onNotice: (message: string) => void }

const last = <T,>(items?: T[] | null) => items?.at(-1)

function ModelBadge({ alias, version }: { alias?: string; version?: string }) {
  if (!alias) return null
  const fallback = alias === 'fixture.deterministic'
  return <span className={fallback ? 'brand-model-badge fallback' : 'brand-model-badge'}>
    {fallback ? '固定样例回退' : alias}{version ? ` · ${version}` : ''}
  </span>
}

export function BrandFilmWorkspace({ onNotice }: Props) {
  const { currentProject } = useProject()
  const [workspace, setWorkspace] = useState<ApiBrandFilmWorkspace | null>(null)
  const [brief, setBrief] = useState<ApiBrandBriefAnalysis | null>(null)
  const [plan, setPlan] = useState<ApiBrandFilmPlan | null>(null)
  const [busy, setBusy] = useState('loading')
  const [productPreview, setProductPreview] = useState('')

  useEffect(() => {
    let active = true
    setBusy('loading')
    void api.ensureBrandFilmFixtureWorkspace(currentProject.id).then(value => {
      if (active) setWorkspace(value)
    }).catch(cause => {
      if (active) onNotice(cause instanceof Error ? cause.message : '品牌广告开发样例初始化失败。')
    }).finally(() => {
      if (active) setBusy('')
    })
    return () => { active = false }
  }, [currentProject.id, onNotice])

  useEffect(() => {
    if (!workspace) return
    const brand = workspace.video_draft.brand_film
    setBrief(last(brand.brief_analysis_versions) ?? null)
    setPlan(last(brand.film_plan_versions) ?? null)
  }, [workspace])

  useEffect(() => () => {
    if (productPreview) URL.revokeObjectURL(productPreview)
  }, [productPreview])

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
    } finally {
      setBusy('')
    }
  }

  if (!workspace) return <div className="brand-film-loading">
    {busy === 'loading' ? <LoaderCircle className="spin" size={18}/> : <FileText size={18}/>}<span>{busy === 'loading' ? '正在载入娇兰开发样例…' : '尚未建立品牌广告工作区'}</span>
  </div>

  const draft = workspace.video_draft.brand_film
  const conceptSet = last(draft.concept_sets)
  const source = draft.source_snapshot
  const revision = workspace.video_draft.revision
  const lockedBrief = Boolean(brief?.confirmed)
  const lockedPlan = Boolean(plan?.confirmed)

  const analyze = () => commit('analyze', () => api.analyzeBrandFilmBrief(currentProject.id, workspace.task.id, revision), 'Brief 已解析并保存为可编辑修订。')
  const saveBrief = () => brief && commit('save-brief', () => api.updateBrandFilmBrief(currentProject.id, workspace.task.id, revision, brief), 'Brief 修改已持久化。')
  const confirmBrief = async () => {
    if (!brief) return
    setBusy('confirm-brief')
    try {
      const saved = await api.updateBrandFilmBrief(currentProject.id, workspace.task.id, revision, brief)
      const confirmed = await api.confirmBrandFilmBrief(currentProject.id, workspace.task.id, saved.video_draft.revision)
      setWorkspace(confirmed)
      onNotice('Brief 与商品参考图已保存并确认，可以生成创意候选。')
    } catch (cause) {
      onNotice(cause instanceof Error ? cause.message : 'Brief 确认失败。')
    } finally {
      setBusy('')
    }
  }
  const generateConcepts = () => commit('concepts', () => api.generateBrandFilmConcepts(currentProject.id, workspace.task.id, revision), '已生成 3 个有差异的创意方向，请人工选择。')
  const selectConcept = (conceptId: string) => commit('select', () => api.selectBrandFilmConcept(currentProject.id, workspace.task.id, revision, conceptId), '创意方向已选择并冻结。')
  const generatePlan = () => commit('plan', () => api.generateBrandFilmPlan(currentProject.id, workspace.task.id, revision), '15 秒剧本与分镜已生成，可逐镜头编辑。')
  const savePlan = () => plan && commit('save-plan', () => api.updateBrandFilmPlan(currentProject.id, workspace.task.id, revision, plan), '剧本与分镜修改已持久化。')
  const confirmPlan = async () => {
    if (!plan) return
    setBusy('confirm-plan')
    try {
      const saved = await api.updateBrandFilmPlan(currentProject.id, workspace.task.id, revision, plan)
      const confirmed = await api.confirmBrandFilmPlan(currentProject.id, workspace.task.id, saved.video_draft.revision)
      setWorkspace(confirmed)
      onNotice('剧本与分镜已保存并确认；Phase 0–2 闭环完成。')
    } catch (cause) {
      onNotice(cause instanceof Error ? cause.message : '剧本与分镜确认失败。')
    } finally {
      setBusy('')
    }
  }

  const updateAsset = (id: string, changes: Partial<ApiBrandBriefAnalysis['asset_candidates'][number]>) => {
    setBrief(value => value ? {
      ...value,
      asset_candidates: value.asset_candidates.map(asset => asset.id === id ? { ...asset, ...changes } : asset),
    } : value)
  }

  const uploadProduct = async (assetId: string, file?: File) => {
    if (!file) return
    setBusy('upload')
    try {
      const ref = await api.uploadProjectAsset(currentProject.id, file)
      if (productPreview) URL.revokeObjectURL(productPreview)
      setProductPreview(URL.createObjectURL(file))
      updateAsset(assetId, {
        asset_ref: ref,
        user_confirmed: true,
        rights_status: 'user_confirmed',
        replacement_note: `用户上传：${file.name}`,
      })
      onNotice('商品参考图已上传；点击“保存修改”后写入当前 Brief 修订。')
    } catch (cause) {
      onNotice(cause instanceof Error ? cause.message : '商品参考图上传失败。')
    } finally {
      setBusy('')
    }
  }

  return <div className="brand-film-workspace">
    <aside className="brand-film-source">
      <span className="section-label">DEVELOPMENT FIXTURE</span>
      <h3>{source.product_name}</h3>
      <p>{source.brief_name}</p>
      <dl>
        <div><dt>来源</dt><dd>{source.fixture_id} v{source.fixture_version}</dd></div>
        <div><dt>规格</dt><dd>{source.duration_seconds}s · {source.aspect_ratio} · {source.channel}</dd></div>
        <div><dt>任务修订</dt><dd>r{revision}</dd></div>
      </dl>
      <div className="brand-film-stage"><span>当前阶段</span><b>{draft.stage}</b></div>
      <p className="brand-film-scope">本轮只做到 Brief、创意方向与剧本分镜确认。视频生成接口已预留，但不会调用 Seedance。</p>
    </aside>

    <main className="brand-film-main">
      <nav className="brand-film-steps" aria-label="品牌广告制作阶段">
        <span className={brief ? 'done' : 'active'}><b>01</b>Brief 分析确认</span>
        <span className={draft.selected_concept_id ? 'done' : brief?.confirmed ? 'active' : ''}><b>02</b>创意候选选择</span>
        <span className={plan?.confirmed ? 'done' : draft.selected_concept_id ? 'active' : ''}><b>03</b>剧本分镜确认</span>
        <span className="future"><b>SEAM</b>生成包 / 尝试</span>
      </nav>

      <section className="brand-film-section">
        <header><div><span className="section-label">PHASE 01</span><h3>Brief 分析与事实确认</h3></div><ModelBadge alias={brief?.model_alias} version={brief?.model_version}/></header>
        {!brief ? <div className="brand-film-empty"><FileText size={24}/><p>使用 Seed-2-pro 解析固定的娇兰 Brief；模型不可用时会明确标记并回退固定样例。</p><button className="primary-button" disabled={Boolean(busy)} onClick={() => void analyze()}><WandSparkles size={15}/>解析 Brief</button></div> : <>
          <div className="brand-form-grid">
            <label className="wide">Brief 摘要<textarea disabled={lockedBrief} value={brief.summary} onChange={event => setBrief({ ...brief, summary: event.target.value })}/></label>
            <label>目标人群<textarea disabled={lockedBrief} value={brief.audience} onChange={event => setBrief({ ...brief, audience: event.target.value })}/></label>
            <label>核心传播信息<textarea disabled={lockedBrief} value={brief.core_message} onChange={event => setBrief({ ...brief, core_message: event.target.value })}/></label>
            <label className="wide">统一口播音色<textarea disabled={lockedBrief} value={brief.voice_direction} onChange={event => setBrief({ ...brief, voice_direction: event.target.value })}/></label>
          </div>
          <div className="brand-fact-grid">
            <div><h4>广告要点 / 卖点</h4>{brief.selling_points.map((fact, index) => <label className="brand-fact" key={`${fact.locator}-${index}`}><input disabled={lockedBrief} value={fact.text} onChange={event => setBrief({ ...brief, selling_points: brief.selling_points.map((item, itemIndex) => itemIndex === index ? { ...item, text: event.target.value } : item) })}/><small>{fact.locator} · {Math.round(fact.confidence * 100)}% · {fact.status}</small></label>)}</div>
            <EditableList title="图片要求" items={brief.image_requirements} disabled={lockedBrief} onChange={items => setBrief({ ...brief, image_requirements: items })}/>
            <EditableList title="视频要求" items={brief.video_requirements} disabled={lockedBrief} onChange={items => setBrief({ ...brief, video_requirements: items })}/>
          </div>
          <div className="brand-assets"><h4>商品与品牌参考素材</h4><p>模型只负责识别候选，最终素材必须由人确认；不满意可上传替换图。</p>{brief.asset_candidates.map(asset => <article key={asset.id}>
            <div className="brand-asset-thumb">{productPreview && asset.role === 'product_front' ? <img src={productPreview} alt={asset.label}/> : <Image size={22}/>}</div>
            <div><b>{asset.label}</b><small>{asset.role} · {asset.source_locator}</small><small>{asset.asset_ref ? `Asset ${asset.asset_ref.asset_id} v${asset.asset_ref.version}` : asset.replacement_note || 'Brief 内嵌候选'}</small></div>
            <label className="brand-checkbox"><input type="checkbox" disabled={lockedBrief} checked={asset.user_confirmed} onChange={event => updateAsset(asset.id, { user_confirmed: event.target.checked, rights_status: event.target.checked ? 'user_confirmed' : 'needs_confirmation' })}/><Check size={13}/>确认使用</label>
            {asset.role === 'product_front' && !lockedBrief ? <label className="secondary-button brand-upload"><Upload size={13}/>替换图片<input type="file" accept="image/png,image/jpeg" onChange={event => { void uploadProduct(asset.id, event.target.files?.[0]) }}/></label> : null}
          </article>)}</div>
          <div className="brand-actions">
            {!lockedBrief ? <><button className="secondary-button" disabled={Boolean(busy)} onClick={() => void analyze()}><Sparkles size={14}/>重新解析</button><button className="secondary-button" disabled={Boolean(busy)} onClick={() => void saveBrief()}>保存修改</button><button className="primary-button" disabled={Boolean(busy) || !brief.asset_candidates.some(asset => asset.role === 'product_front' && asset.user_confirmed)} onClick={() => void confirmBrief()}>确认 Brief</button></> : <span className="brand-confirmed"><Check size={14}/>Brief 已确认</span>}
          </div>
        </>}
      </section>

      <section className="brand-film-section" aria-disabled={!brief?.confirmed}>
        <header><div><span className="section-label">PHASE 02A</span><h3>有差异的创意方向</h3></div><ModelBadge alias={conceptSet?.model_alias} version={conceptSet?.model_version}/></header>
        {!brief?.confirmed ? <p className="brand-locked">确认 Brief 后开放。</p> : !conceptSet ? <div className="brand-film-empty compact"><p>一次生成 3 个叙事机制不同的方向，不在这里生成视频。</p><button className="primary-button" disabled={Boolean(busy)} onClick={() => void generateConcepts()}>生成创意候选</button></div> : <div className="brand-concepts">{conceptSet.candidates.map(concept => <article className={concept.selected ? 'selected' : ''} key={concept.id}>
          <span>{concept.id}</span><h4>{concept.title}</h4><b>{concept.one_liner}</b><p>{concept.story_mechanism}</p><dl><div><dt>品牌进入</dt><dd>{concept.brand_entrance}</dd></div><div><dt>声音</dt><dd>{concept.sound_idea}</dd></div><div><dt>Brief 依据</dt><dd>{concept.brief_rationale}</dd></div><div><dt>风险</dt><dd>{concept.risk}</dd></div></dl>
          <button className={concept.selected ? 'secondary-button' : 'primary-button'} disabled={Boolean(busy) || Boolean(draft.selected_concept_id)} onClick={() => void selectConcept(concept.id)}>{concept.selected ? '已选择' : '选择此方向'}</button>
        </article>)}</div>}
      </section>

      <section className="brand-film-section" aria-disabled={!draft.selected_concept_id}>
        <header><div><span className="section-label">PHASE 02B</span><h3>15 秒剧本与镜头表</h3></div><ModelBadge alias={plan?.model_alias} version={plan?.model_version}/></header>
        {!draft.selected_concept_id ? <p className="brand-locked">选择创意方向后开放。</p> : !plan ? <div className="brand-film-empty compact"><p>剧本、旁白和镜头字段可编辑；底层 Prompt 暂不暴露给用户。</p><button className="primary-button" disabled={Boolean(busy)} onClick={() => void generatePlan()}>生成剧本与分镜</button></div> : <>
          <div className="brand-form-grid"><label>片名<input disabled={lockedPlan} value={plan.title} onChange={event => setPlan({ ...plan, title: event.target.value })}/></label><label>音乐方向<input disabled={lockedPlan} value={plan.music_direction} onChange={event => setPlan({ ...plan, music_direction: event.target.value })}/></label><label className="wide">故事概要<textarea disabled={lockedPlan} value={plan.story_summary} onChange={event => setPlan({ ...plan, story_summary: event.target.value })}/></label></div>
          <div className="brand-shot-list">{plan.shots.map((shot, index) => <article key={shot.id}><header><b>镜头 {String(shot.order).padStart(2, '0')}</b><span>{shot.start_second}s–{shot.end_second}s</span><small>{shot.purpose}</small></header><div><label>画面<textarea disabled={lockedPlan} value={shot.visual} onChange={event => setPlan({ ...plan, shots: plan.shots.map((item, itemIndex) => itemIndex === index ? { ...item, visual: event.target.value } : item) })}/></label><label>动作 / 运镜<textarea disabled={lockedPlan} value={`${shot.action}\n${shot.camera}`} onChange={event => { const [action, ...camera] = event.target.value.split('\n'); setPlan({ ...plan, shots: plan.shots.map((item, itemIndex) => itemIndex === index ? { ...item, action, camera: camera.join('\n') } : item) }) }}/></label><label>旁白 / 屏幕字<textarea disabled={lockedPlan} value={`${shot.voiceover}\n${shot.on_screen_text}`} onChange={event => { const [voiceover, ...text] = event.target.value.split('\n'); setPlan({ ...plan, shots: plan.shots.map((item, itemIndex) => itemIndex === index ? { ...item, voiceover, on_screen_text: text.join('\n') } : item) }) }}/></label><label>光线 / 连贯性<textarea disabled={lockedPlan} value={`${shot.lighting}\n${shot.continuity_notes}`} onChange={event => { const [lighting, ...notes] = event.target.value.split('\n'); setPlan({ ...plan, shots: plan.shots.map((item, itemIndex) => itemIndex === index ? { ...item, lighting, continuity_notes: notes.join('\n') } : item) }) }}/></label></div></article>)}</div>
          <div className="brand-actions">{!lockedPlan ? <><button className="secondary-button" disabled={Boolean(busy)} onClick={() => void generatePlan()}>重新生成整版</button><button className="secondary-button" disabled={Boolean(busy)} onClick={() => void savePlan()}>保存修改</button><button className="primary-button" disabled={Boolean(busy)} onClick={() => void confirmPlan()}>确认剧本与分镜</button></> : <span className="brand-confirmed"><Check size={14}/>Phase 0–2 已完成并持久化</span>}</div>
        </>}
      </section>

      <footer className="brand-generation-seam"><b>后续生成接口已预留</b><span>{draft.generation_seam.unit_policy} · {draft.generation_seam.prompt_contract} · {draft.generation_seam.attempt_policy}</span><small>当前不会创建 PromptPackage、GenerationUnit 或 Seedance Attempt。</small></footer>
    </main>
  </div>
}

function EditableList({ title, items, disabled, onChange }: { title: string; items: string[]; disabled: boolean; onChange: (items: string[]) => void }) {
  return <label><h4>{title}</h4><textarea disabled={disabled} value={items.join('\n')} onChange={event => onChange(event.target.value.split('\n').filter(Boolean))}/></label>
}
