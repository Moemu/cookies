import { useCallback, useEffect, useMemo, useState } from 'react'
import { CircleAlert, FlaskConical, Layers3, Lock, RefreshCw, Ruler, TriangleAlert } from 'lucide-react'
import { useProject } from '../context/ProjectContext'
import {
  api,
  type ApiConfidenceLevel,
  type ApiExperiment,
  type ApiExperimentComparison,
  type ApiExperimentDetail,
  type ApiExperimentStatus,
  type ApiExperimentVerdict,
  type ApiFeatureMatrix,
  type ApiFeatureValue,
  type ApiInsightAsset,
  type ApiInsightAssetType,
  type ApiFeatureSchema,
  type ApiVariantSample,
} from '../data/api'
import type { DataState } from '../types'
import { ExperimentCreateForm } from './ExperimentCreateForm'
import { StateBoundary } from './StateBoundary'

/**
 * 实验中心的五个二级视图：实验列表 · A/B 变体 · 变量矩阵 · 样本检查 · 实验结论
 * （视图清单出自 03 §7.3 与 19 §284，冲突登记见基线文档 §5 冲突 12）。
 *
 * 这一页和投后分析「素材对比」算的是同一套东西——后端共用 compareGroups——差别只有
 * 一个：**这里的分组是在看到数据之前定的**。所以同样的区间判定，这里能说到「归因到
 * 这个变量」，那边只能说到「相关」。两页永远不会给出互相矛盾的数字，因为算法只有一份。
 *
 * 全页最硬的一条纪律：**样本不达标时不显示任何对比数字**。后端 blocked 为 true 时
 * 连 result 都不返回，前端也就没得显示。人会先看见数字再看见提示，然后只记住数字
 * ——所以数字根本不能出现，而不是出现之后加一行小字。
 */
type ViewTarget = 'list' | 'variants' | 'matrix' | 'samples' | 'conclusion'

const viewTargets: Record<string, ViewTarget> = {
  实验列表: 'list',
  'A/B 变体': 'variants',
  变量矩阵: 'matrix',
  样本检查: 'samples',
  实验结论: 'conclusion',
}

const headings: Record<ViewTarget, { label: string; title: string; lead: string }> = {
  list: {
    label: 'EXPERIMENTS',
    title: '有哪些实验在跑，各自要验证什么',
    lead: '实验的价值全在「事先」两个字上：假设、变量、分组和样本门槛在投放之前写死，事后就没有挑数据的余地。',
  },
  variants: {
    label: 'VARIANTS',
    title: '每一组放哪些素材，取值对不对得上',
    lead: '素材由人从素材库挑，系统只把关：变量取值对不上直接拦下；要求控住的特征不一致给黄牌，但放行。',
  },
  matrix: {
    label: 'VARIABLE MATRIX',
    title: '各组之间到底只差被测那一个变量吗',
    lead: '把被测变量和要控住的特征按组摊开。一行里出现两种取值，说明这一项在组间也变了——结论要连它一起解释。',
  },
  samples: {
    label: 'SAMPLE CHECK',
    title: '每组的量够不够事先定的门槛',
    lead: '门槛是开跑前定的，不能事后调。没到门槛的组，它参与的所有对比都不出数字——不是不显著，是还不能看。',
  },
  conclusion: {
    label: 'READOUT',
    title: '判定由系统给，解读由人写',
    lead: '系统只按门槛和置信区间给「支持 / 推翻 / 看不出差别」。这条结论在业务上意味着什么、下一轮怎么做，人来写。',
  },
}

const statusLabels: Record<ApiExperimentStatus, string> = {
  draft: '草稿',
  running: '进行中',
  concluded: '已结论',
}

const statusTone: Record<ApiExperimentStatus, string> = {
  draft: 'muted',
  running: 'warning',
  concluded: 'ok',
}

// 措辞刻意不写「成功 / 失败」：假设被推翻同样是有用的结论，写成失败会让人不愿如实登记。
const verdictLabels: Record<ApiExperimentVerdict, string> = {
  supported: '支持这条假设',
  refuted: '推翻这条假设',
  inconclusive: '看不出差别',
}

const verdictTone: Record<ApiExperimentVerdict, string> = {
  supported: 'ok',
  refuted: 'danger',
  inconclusive: 'muted',
}

const confidenceLabels: Record<ApiConfidenceLevel, string> = {
  sufficient: '充分',
  directional: '方向性',
  low_sample: '样本不足',
  confounded: '存在混杂',
}

const assetTypeLabels: Record<ApiInsightAssetType, string> = {
  xiaohongshu_note: '小红书图文',
  wechat_article: '公众号图文',
  brand_ad: '品牌广告',
  digital_human_ad: '数字人效果广告',
  preroll_ad: '广告前贴',
  hit_replica_ad: '爆款复刻',
}

/** 投前洞察「拿去验证」的交接。跳转发生在两个页面之间，没有共享状态，用一次性的存储传。 */
const handoffKey = 'insight.experiment.handoff'

export type ExperimentHandoff = { experienceId: string; hypothesis: string; title: string }

export function stashExperimentHandoff(payload: ExperimentHandoff) {
  sessionStorage.setItem(handoffKey, JSON.stringify(payload))
}

function takeExperimentHandoff(): ExperimentHandoff | null {
  const raw = sessionStorage.getItem(handoffKey)
  if (!raw) return null
  // 读完就清：这是一次交接，不是一份配置。留着会让人下次进这一页又看到一张预填好的表单。
  sessionStorage.removeItem(handoffKey)
  try {
    return JSON.parse(raw) as ExperimentHandoff
  } catch {
    return null
  }
}

export function ExperimentCenterPage({ state, activeView }: { state: DataState; activeView: string }) {
  const { currentProject } = useProject()
  const target = viewTargets[activeView] ?? 'list'

  const [experiments, setExperiments] = useState<ApiExperiment[]>([])
  const [statusFilter, setStatusFilter] = useState<'all' | ApiExperimentStatus>('all')
  const [selectedId, setSelectedId] = useState('')
  const [detail, setDetail] = useState<ApiExperimentDetail | null>(null)
  const [schemas, setSchemas] = useState<ApiFeatureSchema[]>([])
  const [candidates, setCandidates] = useState<ApiInsightAsset[]>([])
  const [matrix, setMatrix] = useState<ApiFeatureMatrix | null>(null)
  const [handoff, setHandoff] = useState<ExperimentHandoff | null>(null)
  const [creating, setCreating] = useState(false)
  const [warnings, setWarnings] = useState<string[]>([])
  const [interpretation, setInterpretation] = useState('')
  const [notice, setNotice] = useState('')
  const [listState, setListState] = useState<'loading' | 'ready' | 'error'>('loading')
  const [busy, setBusy] = useState(false)

  const loadList = useCallback(async () => {
    if (!currentProject.id) return
    setListState('loading')
    try {
      const [list, schemaList] = await Promise.all([
        api.listInsightExperiments(currentProject.id, statusFilter === 'all' ? undefined : statusFilter),
        api.listFeatureSchemas(currentProject.id),
      ])
      const items = list.items ?? []
      setExperiments(items)
      setSchemas(schemaList.items ?? [])
      setSelectedId(current => current && items.some(item => item.id === current) ? current : items[0]?.id ?? '')
      setListState('ready')
    } catch (cause) {
      setExperiments([])
      setListState('error')
      setNotice(cause instanceof Error ? cause.message : '实验列表读取失败。')
    }
  }, [currentProject.id, statusFilter])

  useEffect(() => { void loadList() }, [loadList])

  // 一进这一页就把交接接下来。开表单要等 schemas 到位，所以只记下来，渲染时再用。
  useEffect(() => {
    const payload = takeExperimentHandoff()
    if (!payload) return
    setHandoff(payload)
    setCreating(true)
  }, [])

  const loadDetail = useCallback(async (experimentId: string) => {
    if (!currentProject.id || !experimentId) { setDetail(null); return }
    try {
      const next = await api.getInsightExperiment(currentProject.id, experimentId)
      setDetail(next)
      setInterpretation(next.experiment.interpretation ?? '')
    } catch (cause) {
      setDetail(null)
      setNotice(cause instanceof Error ? cause.message : '实验详情读取失败。')
    }
  }, [currentProject.id])

  useEffect(() => { void loadDetail(selectedId) }, [loadDetail, selectedId])

  // 候选素材按实验的素材类型过滤：类型不同的素材连特征体系都不是同一套，放进来也比不了。
  useEffect(() => {
    const assetType = detail?.experiment.asset_type
    if (!currentProject.id || !assetType || target !== 'variants') return
    let cancelled = false
    void api.listInsightAssets(currentProject.id, { assetTypes: [assetType], limit: 200 })
      .then(result => { if (!cancelled) setCandidates(result.items ?? []) })
      .catch(() => { if (!cancelled) setCandidates([]) })
    return () => { cancelled = true }
  }, [currentProject.id, detail?.experiment.asset_type, target])

  const experimentAssetIds = useMemo(
    () => (detail?.experiment.variants ?? []).flatMap(variant => variant.asset_ids ?? []),
    [detail],
  )

  useEffect(() => {
    if (!currentProject.id || target !== 'matrix' || !experimentAssetIds.length) { setMatrix(null); return }
    let cancelled = false
    void api.getFeatureMatrix(currentProject.id, experimentAssetIds)
      .then(result => { if (!cancelled) setMatrix(result) })
      .catch(() => { if (!cancelled) setMatrix(null) })
    return () => { cancelled = true }
  }, [currentProject.id, target, experimentAssetIds])

  const assetTitles = useMemo(() => {
    const map = new Map<string, string>()
    candidates.forEach(asset => map.set(asset.id, asset.title))
    matrix?.assets?.forEach(asset => map.set(asset.id, asset.title))
    return map
  }, [candidates, matrix])

  async function run(action: () => Promise<void>) {
    setBusy(true)
    setNotice('')
    try {
      await action()
    } catch (cause) {
      setNotice(cause instanceof Error ? cause.message : '操作失败。')
    } finally {
      setBusy(false)
    }
  }

  const attachAsset = (variantId: string, assetId: string) => run(async () => {
    const result = await api.attachInsightExperimentAsset(currentProject.id, selectedId, variantId, assetId)
    setWarnings(result.warnings ?? [])
    await loadDetail(selectedId)
  })

  const detachAsset = (variantId: string, assetId: string) => run(async () => {
    await api.detachInsightExperimentAsset(currentProject.id, selectedId, variantId, assetId)
    setWarnings([])
    await loadDetail(selectedId)
  })

  const startExperiment = () => run(async () => {
    if (!detail) return
    await api.startInsightExperiment(currentProject.id, selectedId, detail.experiment.version)
    setNotice('已开跑。从现在起分组锁死，加不了也移不走素材。')
    await Promise.all([loadDetail(selectedId), loadList()])
  })

  const concludeExperiment = () => run(async () => {
    if (!detail) return
    await api.concludeInsightExperiment(currentProject.id, selectedId, detail.experiment.version, interpretation.trim())
    setNotice('结论已登记。判定和解读一起冻住，之后只能读不能改。')
    await Promise.all([loadDetail(selectedId), loadList()])
  })

  const heading = headings[target]
  const experiment = detail?.experiment
  const readout = detail?.readout

  const troubles = useMemo(() => [
    ...(readout?.notes ?? []),
    ...(readout && !readout.comparable
      ? [readout.comparable_reason || '窗口内口径不一致，这次实验的判定不能当成干净对照。']
      : []),
  ], [readout])

  return <StateBoundary state={state} onRetry={() => { void loadList() }}>
    <div className="prelaunch-workspace">
      <section className="prelaunch-main">
        <div className="core-flow-toolbar">
          <div>
            <span className="section-label">{heading.label}</span>
            <h2>{heading.title}</h2>
            <p>{heading.lead}</p>
          </div>
          <div className="core-flow-actions">
            {target === 'list'
              ? <label>状态<select aria-label="实验状态" value={statusFilter}
                  onChange={event => setStatusFilter(event.target.value as 'all' | ApiExperimentStatus)}>
                  <option value="all">全部</option>
                  <option value="draft">草稿</option>
                  <option value="running">进行中</option>
                  <option value="concluded">已结论</option>
                </select></label>
              : <label>实验<select aria-label="当前实验" value={selectedId}
                  onChange={event => setSelectedId(event.target.value)}>
                  {experiments.map(item => <option key={item.id} value={item.id}>{item.title}</option>)}
                  {!experiments.length ? <option value="">还没有实验</option> : null}
                </select></label>}
            <button className="secondary-button" disabled={listState === 'loading'} onClick={() => { void loadList() }}>
              <RefreshCw size={15}/>刷新
            </button>
            {target === 'list' ? <button className="primary-button" onClick={() => { setHandoff(null); setCreating(true) }}>
              新建实验
            </button> : null}
          </div>
        </div>

        {troubles.length && target !== 'list' ? <div className="feature-stack">
          <span>先看这些（会影响下面每一条结论怎么读）</span>
          {troubles.map((item, index) => <b key={index}>{item}</b>)}
        </div> : null}

        {creating && target === 'list' ? <ExperimentCreateForm
          projectId={currentProject.id}
          schemas={schemas}
          sourceExperienceId={handoff?.experienceId}
          presetHypothesis={handoff?.hypothesis}
          onCancel={() => { setCreating(false); setHandoff(null) }}
          onCreated={created => {
            setCreating(false)
            setHandoff(null)
            setNotice('实验已建为草稿。下一步去「A/B 变体」把素材放进各组。')
            setSelectedId(created.id)
            void loadList()
          }}/> : null}

        {listState === 'loading' ? <div className="panel-empty">正在取数…</div>
          : listState === 'error' ? <div className="panel-empty">读取失败，请重试。</div>
          : target === 'list'
            ? (experiments.length
                ? <ExperimentList items={experiments} selectedId={selectedId} onSelect={setSelectedId}/>
                : <div className="panel-empty">
                    还没有实验。实验和「投后分析 · 素材对比」的差别不在算法，在于分组是不是事先定的
                    ——想让某条结论撑得起因果，就得在投放之前把它登记下来。
                  </div>)
          : !experiment || !readout ? <div className="panel-empty">先在「实验列表」里建一个实验。</div>
          : target === 'variants' ? <VariantBoard
              detail={detail!} candidates={candidates} assetTitles={assetTitles} warnings={warnings} busy={busy}
              onAttach={attachAsset} onDetach={detachAsset}/>
          : target === 'matrix' ? <VariableMatrix detail={detail!} matrix={matrix}/>
          : target === 'samples' ? <SampleCheck detail={detail!}/>
          : <ConclusionBoard detail={detail!} interpretation={interpretation} busy={busy}
              onInterpretationChange={setInterpretation} onConclude={() => { void concludeExperiment() }}/>}
      </section>

      <aside className="prelaunch-detail">
        <span className="section-label">这一页怎么读</span>
        <h3>{heading.title}</h3>
        <p>{heading.lead}</p>

        {experiment && readout ? <>
          <div className="prelaunch-fact"><FlaskConical size={17}/><span><small>当前实验</small><b>
            {experiment.title}（{statusLabels[experiment.status]}）·
            被测变量「{experiment.variable_label}」·
            {assetTypeLabels[experiment.asset_type]}
          </b></span></div>
          <div className="prelaunch-fact"><Layers3 size={17}/><span><small>观察窗口</small><b>
            {formatDate(readout.window.start)} ~ {formatDate(readout.window.end)} ·
            每组门槛 {formatCount(experiment.min_impressions)} 次展示
          </b></span></div>
          <div className="prelaunch-fact"><Ruler size={17}/><span><small>口径</small><b>
            {readout.caliber.currency} · 归因 {readout.caliber.attribution_window} ·
            指标口径 {readout.caliber.metric_schema_version} · 时区 {readout.caliber.time_zone}
          </b></span></div>

          {experiment.status === 'draft' ? <>
            <div className="prelaunch-boundary"><Lock size={16}/><span><small>还没开跑</small>
              草稿阶段才能改分组。点「开跑」之后素材加不了也移不走——冻结是事先登记的全部依据，
              否则谁都可以在看完数据之后往表现好的那组补两条。
            </span></div>
            <button className="primary-button full" disabled={busy} onClick={() => { void startExperiment() }}>
              开跑并锁死分组
            </button>
          </> : null}

          {experiment.status === 'running' ? <div className="prelaunch-boundary"><Lock size={16}/><span>
            <small>分组已锁死</small>
            这次实验的变量、分组和样本门槛都改不了了。要换条件，只能另建一次实验。
          </span></div> : null}

          {experiment.status === 'concluded' ? <div className="prelaunch-fact"><FlaskConical size={17}/><span>
            <small>结论</small><b>
              {experiment.verdict ? verdictLabels[experiment.verdict] : '未登记判定'}
              {experiment.concluded_at ? ` · ${formatDate(experiment.concluded_at)}` : ''}
              {experiment.interpretation ? ` · ${experiment.interpretation}` : ''}
            </b></span></div> : null}
        </> : null}

        <div className="prelaunch-boundary"><CircleAlert size={16}/><span><small>这一页给不了什么</small>
          这里不会替人判断实验值不值得做，也不会自动给素材分组。变量是人定的，素材是人挑的
          ——系统只做两件事：把取值对不上的挡在门外，以及在样本不够时不给数字。
        </span></div>

        {notice ? <div className="inline-notice" role="status">{notice}</div> : null}
      </aside>
    </div>
  </StateBoundary>
}

function ExperimentList({ items, selectedId, onSelect }: {
  items: ApiExperiment[]
  selectedId: string
  onSelect: (id: string) => void
}) {
  return <div className="prelaunch-table" role="list" aria-label="实验列表">
    <div className="prelaunch-row insight-experiment-row header">
      <span>实验与假设</span><span>状态</span><span>被测变量</span><span>分组</span><span>判定</span>
    </div>
    {items.map(item => <button role="listitem" type="button"
      className={`prelaunch-row insight-experiment-row${item.id === selectedId ? ' active' : ''}`}
      key={item.id} onClick={() => onSelect(item.id)}>
      <span><b>{item.title}</b><small>{item.hypothesis || '没有写假设。事后再补的假设，和事先写下的不是一回事。'}</small></span>
      <span className={`insight-experiment-status ${statusTone[item.status]}`}>{statusLabels[item.status]}</span>
      <span>{item.variable_label}<br/><small>{assetTypeLabels[item.asset_type]}</small></span>
      <span>{(item.variants ?? []).length} 组 · 门槛 {formatCount(item.min_impressions)}</span>
      <span>{item.verdict ? verdictLabels[item.verdict] : '—'}</span>
    </button>)}
  </div>
}

function VariantBoard({ detail, candidates, assetTitles, warnings, busy, onAttach, onDetach }: {
  detail: ApiExperimentDetail
  candidates: ApiInsightAsset[]
  assetTitles: Map<string, string>
  warnings: string[]
  busy: boolean
  onAttach: (variantId: string, assetId: string) => void
  onDetach: (variantId: string, assetId: string) => void
}) {
  const { experiment } = detail
  const editable = experiment.status === 'draft'
  const taken = new Set(experiment.variants.flatMap(variant => variant.asset_ids ?? []))

  return <>
    {warnings.length ? <div className="experiment-warnings" role="status">
      <TriangleAlert size={15}/>
      <span>
        <b>加进去了，但有这些不一致</b>
        {warnings.map((line, index) => <em key={index}>{line}</em>)}
      </span>
    </div> : null}

    <div className="insight-analysis-list" role="list" aria-label="A/B 变体">
      {experiment.variants.map(variant => {
        const assetIds = variant.asset_ids ?? []
        return <article className="insight-analysis-card" role="listitem" key={variant.id}>
          <header>
            <b>{variant.name}</b>
            <em className={variant.is_baseline ? 'ok' : 'muted'}>{variant.is_baseline ? '基线组' : '变体组'}</em>
          </header>
          <p>{experiment.variable_label} = {variant.variable_value}。这一组里的素材，这一项必须都是这个取值。</p>

          <div className="experiment-asset-list">
            {assetIds.length ? assetIds.map(assetId => <div key={assetId}>
              <span>{assetTitles.get(assetId) ?? assetId}</span>
              {editable ? <button className="text-button" disabled={busy}
                onClick={() => onDetach(variant.id, assetId)}>移出</button> : null}
            </div>) : <div><span>这一组还没有素材。</span></div>}
          </div>

          {editable ? <label className="experiment-field">
            <span>从素材库挑一条加进来</span>
            <select value="" disabled={busy} onChange={event => {
              if (event.target.value) onAttach(variant.id, event.target.value)
            }}>
              <option value="">请选择</option>
              {candidates.filter(asset => !taken.has(asset.id)).map(asset => <option key={asset.id} value={asset.id}>
                {asset.title}
              </option>)}
            </select>
          </label> : null}
        </article>
      })}
    </div>
  </>
}

/**
 * 变量矩阵。行是「被测变量 + 要控住的特征」，列是各组。
 *
 * 一行里出现两种取值就标黄——这不是错误，是**这次实验没能只改一个变量**这件事本身。
 * 藏起来，结论会被当成干净对照；硬拦，真实素材下这实验一个都建不起来。
 */
function VariableMatrix({ detail, matrix }: { detail: ApiExperimentDetail; matrix: ApiFeatureMatrix | null }) {
  const { experiment } = detail
  const keys = [experiment.variable_key, ...(experiment.controlled_keys ?? [])]
  const assetIds = experiment.variants.flatMap(variant => variant.asset_ids ?? [])

  if (!assetIds.length) return <div className="panel-empty">各组还没有素材，摊不开矩阵。先去「A/B 变体」把素材放进去。</div>
  if (!matrix) return <div className="panel-empty">正在读取素材特征…</div>

  const rows = keys.map(key => {
    const row = matrix.rows?.find(item => item.key === key)
    const cells = experiment.variants.map(variant => {
      const values = (variant.asset_ids ?? []).map(assetId => {
        const cell = row?.cells?.find(item => item.asset_id === assetId)
        return cell ? formatValue(cell.value) : '未记录'
      })
      const distinct = Array.from(new Set(values))
      return { variantId: variant.id, values: distinct, mixed: distinct.length > 1 }
    })
    return {
      key,
      label: row?.label ?? (key === experiment.variable_key ? experiment.variable_label : key),
      tested: key === experiment.variable_key,
      cells,
    }
  })

  return <div className="experiment-matrix" role="table" aria-label="变量矩阵">
    <div className="experiment-matrix-row header" role="row">
      <span role="columnheader">特征</span>
      {experiment.variants.map(variant => <span key={variant.id} role="columnheader">
        {variantLabel(variant.name, variant.is_baseline)}
      </span>)}
    </div>
    {rows.map(row => <div className="experiment-matrix-row" role="row" key={row.key}>
      <span role="rowheader">
        <b>{row.label}</b>
        <small>{row.tested ? '被测变量' : '要求控住'}</small>
      </span>
      {row.cells.map(cell => <span role="cell" key={cell.variantId} className={cell.mixed ? 'mixed' : ''}>
        {cell.values.join(' / ') || '未记录'}
        {cell.mixed ? <small>组内就不一致</small> : null}
      </span>)}
    </div>)}
    <p className="experiment-matrix-foot">
      被测变量那一行各组必须不同——不同才有对照。其余各行各组应当相同；出现不同，
      说明这次比较里不止一个变量在变，结论要连它一起解释。
    </p>
  </div>
}

function SampleCheck({ detail }: { detail: ApiExperimentDetail }) {
  const { experiment, readout } = detail
  const samples = readout.samples ?? []

  return <>
    <div className="prelaunch-table" role="list" aria-label="样本检查">
      <div className="prelaunch-row insight-sample-row header">
        <span>分组</span><span>素材</span><span>展示</span><span>点击</span><span>够不够</span>
      </div>
      {samples.map(sample => <div role="listitem" className="prelaunch-row insight-sample-row" key={sample.variant_id}>
        <span>
          <b>{variantLabel(sample.name, sample.is_baseline)}</b>
          <small>{experiment.variable_label} = {sample.variable_value}</small>
        </span>
        <span>{sample.assets} 条<br/><small>其中 {sample.assets_with_data} 条有数据</small></span>
        <span>{formatCount(sample.impressions)}</span>
        <span>{formatCount(sample.clicks)}</span>
        <span className={sample.meets_threshold ? 'insight-experiment-status ok' : 'insight-experiment-status warning'}>
          {sample.meets_threshold ? '已达标' : `还差 ${formatCount(sample.short_by)}`}
        </span>
      </div>)}
      {!samples.length ? <div className="panel-empty">还没有分组。</div> : null}
    </div>

    <div className="insight-analysis-diff">
      <span>门槛怎么来的</span>
      <b>
        每组 {formatCount(experiment.min_impressions)} 次展示，是建实验时定的，开跑之后不能改。
        事后再调门槛等于允许「凑够了就说显著」——那条结论就没有约束力了。
      </b>
      <b>
        {readout.ready
          ? '所有组都过了门槛，可以去看实验结论。'
          : '还有组没到门槛。在那之前，实验结论那一页不会显示任何对比数字——不是不显著，是还不能看。'}
      </b>
    </div>
  </>
}

function ConclusionBoard({ detail, interpretation, busy, onInterpretationChange, onConclude }: {
  detail: ApiExperimentDetail
  interpretation: string
  busy: boolean
  onInterpretationChange: (value: string) => void
  onConclude: () => void
}) {
  const { experiment, readout } = detail
  const comparisons = readout.comparisons ?? []
  const concluded = experiment.status === 'concluded'

  return <>
    <div className="insight-analysis-list" role="list" aria-label="实验结论">
      {comparisons.map(item => <ComparisonCard key={item.variant_id} item={item}/>)}
      {!comparisons.length ? <div className="panel-empty">
        还没有可比较的组。实验至少要有一个基线组和一个变体组，两边都放上素材。
      </div> : null}
    </div>

    <div className="experiment-conclude">
      <span className="section-label">登记结论</span>
      {readout.verdict ? <p className="experiment-verdict">
        系统判定：<b className={verdictTone[readout.verdict]}>{verdictLabels[readout.verdict]}</b>
      </p> : <p className="experiment-verdict">系统还给不出判定。</p>}

      {!readout.ready ? <div className="experiment-hint">
        <CircleAlert size={14}/>
        <span>还有组没到样本门槛，现在下结论会被拒绝。去「样本检查」看还差多少。</span>
      </div> : null}

      <label className="experiment-field">
        <span>你的解读（这条结论在业务上意味着什么、下一轮怎么用）</span>
        <textarea value={interpretation} disabled={concluded || busy}
          onChange={event => onInterpretationChange(event.target.value)}
          placeholder="例：开场用人脸确实更容易被点开，但这批素材都是同一个出镜人，换人之后还成不成立不知道，下一轮换人再验一次。"/>
      </label>

      {concluded ? <p className="experiment-form-foot">
        这次实验已经结论，判定和解读一起冻住了。要修正认识，另建一次实验，而不是改这一条。
      </p> : <button className="primary-button" disabled={busy || !readout.ready || !interpretation.trim()}
        onClick={onConclude}>登记结论</button>}
      <p className="experiment-form-foot">
        判定不由人填。判定要是能填，事先定的样本门槛就形同虚设——所以这里只收解读。
      </p>
    </div>
  </>
}

function ComparisonCard({ item }: { item: ApiExperimentComparison }) {
  return <article className="insight-analysis-card" role="listitem">
    <header>
      <b>{item.variant_name} ↔ {variantLabel(item.baseline_name, true)}</b>
      <em className={item.blocked ? 'muted' : item.verdict ? verdictTone[item.verdict] : 'muted'}>
        {item.blocked ? '还不能看' : item.verdict ? verdictLabels[item.verdict] : '没有判定'}
      </em>
    </header>

    {/* blocked 时后端连 result 都不返回，这里也就没得显示。这是有意的：
        样本不达标却先显示一个 CTR 差异，人会记住数字，忘掉那行小字。 */}
    {item.blocked ? <p>{item.blocker || '这一组的样本量还没到事先定的门槛，所以不显示任何对比数字。'}</p>
      : item.result ? <>
        <p>{item.result.note}</p>
        <div className="insight-analysis-metrics">
          <div>
            <small>{item.variant_name}（{item.variant_value}）</small>
            <b>{formatRate(item.result.rates.ctr)}</b>
            <span>
              展示 {formatCount(item.result.counts.impressions)} · 转化成本 {formatMoney(item.result.rates.cpa_cents)}
              <br/>{item.result.ctr_interval
                ? `95% 区间 ${formatRate(item.result.ctr_interval.low)} ~ ${formatRate(item.result.ctr_interval.high)}`
                : '样本不足以给出区间'}
            </span>
          </div>
          <div>
            <small>{item.baseline_name}（{item.baseline_value}）</small>
            <b>{formatRate(item.result.rest_rates.ctr)}</b>
            <span>
              展示 {formatCount(item.result.rest_counts.impressions)} · 转化成本 {formatMoney(item.result.rest_rates.cpa_cents)}
              <br/>{item.result.rest_ctr_interval
                ? `95% 区间 ${formatRate(item.result.rest_ctr_interval.low)} ~ ${formatRate(item.result.rest_ctr_interval.high)}`
                : '样本不足以给出区间'}
            </span>
          </div>
        </div>
        <footer>
          点击率相对变化 {formatSigned(item.result.ctr_lift)} ·
          {item.result.intervals_overlap ? ' 两侧置信区间重叠，这个差异可能只是波动' : ' 两侧置信区间不重叠'} ·
          置信{confidenceLabels[item.result.confidence]}
        </footer>
      </> : <p>没有可显示的对比结果。</p>}
  </article>
}

// 下面的格式化函数共用一条规则：undefined 是「算不出来」，不是 0。
function formatRate(value?: number): string {
  return value === undefined ? '不可用' : `${(value * 100).toFixed(2)}%`
}

function formatSigned(value?: number): string {
  if (value === undefined) return '不可用'
  return `${value > 0 ? '+' : ''}${(value * 100).toFixed(1)}%`
}

function formatMoney(cents?: number): string {
  return cents === undefined ? '不可用' : `¥${(cents / 100).toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`
}

/**
 * 组名 + 基线标记。名字里已经写了「基线」就不再加一遍——建实验的人常把它写进组名，
 * 无条件追加就会出现「利益开场（基线）（基线）」。组名开跑后锁死，改不了。
 */
function variantLabel(name: string, isBaseline: boolean): string {
  if (!isBaseline || name.includes('基线')) return name
  return `${name}（基线）`
}

function formatCount(value: number): string {
  return value.toLocaleString('zh-CN')
}

function formatDate(value: string): string {
  const time = new Date(value)
  return Number.isNaN(time.valueOf()) ? value : time.toLocaleDateString('zh-CN')
}

function formatValue(value: ApiFeatureValue): string {
  if (value.kind === 'bool') return value.bool ? '是' : '否'
  if (value.kind === 'number') return String(value.number ?? 0)
  if (value.kind === 'duration_seconds') return `${value.number ?? 0} 秒`
  if (value.terms?.length) return value.terms.join('、')
  return value.text || '（空）'
}

// 只是为了让 ApiVariantSample 这个类型被显式引用一次，避免 samples 表格的字段改名后无声漂移。
export type ExperimentSampleRow = ApiVariantSample
