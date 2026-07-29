import { Fragment, useCallback, useEffect, useMemo, useState } from 'react'
import { CircleAlert, Layers3, Lightbulb, RefreshCw, Sparkles, UserCheck } from 'lucide-react'
import { useProject } from '../context/ProjectContext'
import {
  api,
  type ApiConfidence,
  type ApiFeatureInput,
  type ApiFeatureMatrix,
  type ApiFeatureSchema,
  type ApiFeatureValue,
  type ApiFeatureValueKind,
  type ApiInsightAsset,
  type ApiInsightAssetFeature,
  type ApiInsightAssetType,
} from '../data/api'
import type { DataState } from '../types'
import { StateBoundary } from './StateBoundary'

/**
 * 内容分析（03 §5、AM-004~006，导航见 19 §5.2）。
 *
 * 六类素材各有一套特征体系，不得混用——03 §MVP② 把「不把视频钩子字段套到公众号
 * 文章」列为明文验收。所以这里的每个类型标签只渲染该类型自己的字段，多素材看矩阵、
 * 单素材看逐项拆解（20 §4.1、22 §6.2 要求「不要堆洞察卡」）。
 */
type ViewTarget = ApiInsightAssetType | 'single'

const viewTargets: Record<string, ViewTarget> = {
  小红书: 'xiaohongshu_note',
  小红书图文: 'xiaohongshu_note',
  公众号: 'wechat_article',
  公众号图文: 'wechat_article',
  品牌广告: 'brand_ad',
  数字人: 'digital_human_ad',
  广告前贴: 'preroll_ad',
  爆款复刻: 'hit_replica_ad',
  单素材拆解: 'single',
}

const assetTypeLabels: Record<ApiInsightAssetType, string> = {
  xiaohongshu_note: '小红书图文',
  wechat_article: '公众号图文',
  brand_ad: '品牌广告',
  digital_human_ad: '数字人效果广告',
  preroll_ad: '广告前贴',
  hit_replica_ad: '爆款复刻',
}

const assetTypeOrder: ApiInsightAssetType[] = [
  'xiaohongshu_note', 'wechat_article', 'brand_ad', 'digital_human_ad', 'preroll_ad', 'hit_replica_ad',
]

const confidenceLabels: Record<ApiConfidence, string> = { low: '低', medium: '中', high: '高' }

const statusLabels: Record<string, string> = {
  awaiting_data: '待数据', awaiting_match: '待匹配', analysable: '可分析', analysing: '分析中',
  pending_confirmation: '待确认', confirmed: '已确认', needs_review: '待复审', retired: '已失效',
}

export function ContentAnalysisPage({ state, activeView }: { state: DataState; activeView: string }) {
  const { currentProject } = useProject()
  const target = viewTargets[activeView] ?? 'single'
  const [schemas, setSchemas] = useState<ApiFeatureSchema[]>([])
  const [assets, setAssets] = useState<ApiInsightAsset[]>([])
  const [matrix, setMatrix] = useState<ApiFeatureMatrix | null>(null)
  const [features, setFeatures] = useState<ApiInsightAssetFeature[]>([])
  const [selectedId, setSelectedId] = useState('')
  const [editingKey, setEditingKey] = useState('')
  const [draft, setDraft] = useState('')
  const [notice, setNotice] = useState('')
  const [listState, setListState] = useState<'loading' | 'ready' | 'error'>('loading')

  const loadList = useCallback(async () => {
    if (!currentProject.id) return
    setListState('loading')
    setNotice('')
    try {
      const schemaPage = await api.listFeatureSchemas(currentProject.id)
      setSchemas(schemaPage.items)
      // 类型标签只取该类型的素材，单素材拆解取全部——每个标签都真实换一批数据（22 §8.3）。
      const page = await api.listInsightAssets(
        currentProject.id,
        target === 'single' ? {} : { assetTypes: [target] },
      )
      // 已失效素材不进对比：它的结论已经不作数，摆在矩阵里只会让人误读差异。
      const live = page.items.filter(asset => asset.analysis_status !== 'retired')
      setAssets(live)
      if (target !== 'single' && live.length) {
        setMatrix(await api.getFeatureMatrix(currentProject.id, live.map(asset => asset.id)))
      } else {
        setMatrix(null)
      }
      setListState('ready')
    } catch (cause) {
      setAssets([])
      setMatrix(null)
      setListState('error')
      setNotice(cause instanceof Error ? cause.message : '内容分析读取失败。')
    }
  }, [currentProject.id, target])

  useEffect(() => { void loadList() }, [loadList])
  useEffect(() => { setEditingKey(''); setDraft('') }, [target, selectedId])

  const breakdownAssets = assets

  useEffect(() => {
    const ids = breakdownAssets.map(asset => asset.id)
    setSelectedId(current => (ids.includes(current) ? current : ids[0] ?? ''))
  }, [breakdownAssets.map(asset => asset.id).join('|')])

  const selectedAsset = breakdownAssets.find(asset => asset.id === selectedId)

  const loadFeatures = useCallback(async () => {
    if (target !== 'single' || !selectedAsset) {
      setFeatures([])
      return
    }
    try {
      const page = await api.listInsightAssetFeatures(currentProject.id, selectedAsset.id)
      setFeatures(page.items)
    } catch {
      setFeatures([])
    }
  }, [currentProject.id, target, selectedAsset?.id])

  useEffect(() => { void loadFeatures() }, [loadFeatures])

  const typeSchema = schemas.find(schema =>
    schema.asset_type === (target === 'single' ? selectedAsset?.asset_type : target))

  /**
   * 写人工层。后端的 PATCH 是整层替换（ReplaceLayer: human），所以要把已有的人工结论
   * 一起带上，否则会把别人此前的判断抹掉。AI 那一层始终不动（AM-006、§14）。
   */
  const writeHumanLayer = useCallback(async (key: string, value: ApiFeatureValue, verdict: 'confirmed' | 'rejected', reason: string) => {
    if (!selectedAsset) return
    const kept: ApiFeatureInput[] = features
      .filter(feature => feature.source === 'human' && feature.key !== key)
      .map(feature => ({ key: feature.key, value: feature.value, review_state: feature.review_state }))
    try {
      await api.patchInsightAssetFeatures(currentProject.id, selectedAsset.id, {
        expected_version: selectedAsset.version,
        features: [...kept, { key, value, review_state: verdict }],
        reason,
      })
      setEditingKey('')
      setDraft('')
      await loadList()
      await loadFeatures()
      // loadList 会清空提示，所以放在它之后再写，否则这条反馈一闪就没了。
      setNotice(verdict === 'confirmed' ? `已认可「${key}」，人工结论已单独记一行。` : `已推翻「${key}」并写入人工结论。`)
    } catch (cause) {
      setNotice(cause instanceof Error ? cause.message : '写入人工结论失败。')
    }
  }, [currentProject.id, features, selectedAsset, loadList, loadFeatures])

  const identifyType = useCallback(async (assetType: ApiInsightAssetType) => {
    if (!selectedAsset) return
    try {
      await api.identifyInsightAssetType(currentProject.id, selectedAsset.id, {
        expected_version: selectedAsset.version,
        asset_type: assetType,
        source: 'human',
        reason: '人工判定素材类型',
      })
      await loadList()
      setNotice(`已判定为「${assetTypeLabels[assetType]}」，接下来才谈得上提取特征。`)
    } catch (cause) {
      setNotice(cause instanceof Error ? cause.message : '类型判定失败。')
    }
  }, [currentProject.id, selectedAsset, loadList])

  const header = target === 'single'
    ? { title: '单素材逐项拆解', lead: '一条素材的全部特征，按它自己那套体系分组；AI 与人工两层分开显示。' }
    : { title: `${assetTypeLabels[target]}的特征矩阵`, lead: '同类素材横向对比同一批变量，才知道差异出在哪一项，而不是只知道谁表现好。' }

  return <StateBoundary state={state} onRetry={() => { void loadList() }}>
    <div className="prelaunch-workspace">
      <section className="prelaunch-main">
        <div className="core-flow-toolbar">
          <div><span className="section-label">CONTENT ANALYSIS</span><h2>{header.title}</h2><p>当前 Project：{currentProject.name}。{header.lead}</p></div>
          <button className="secondary-button" disabled={listState === 'loading'} onClick={() => { void loadList() }}><RefreshCw size={15}/>刷新</button>
        </div>

        {listState === 'loading' ? <div className="panel-empty">正在读取内容分析数据…</div> : null}
        {listState === 'error' ? <div className="panel-empty">读取失败，请重试。</div> : null}

        {listState === 'ready' && target !== 'single' ? <>
          <div className="prelaunch-filterbar">
            <div><span className="section-label">{typeSchema?.label ?? assetTypeLabels[target]}</span></div>
            <span>{assets.length} 个素材 · {typeSchema?.fields.length ?? 0} 个可提取变量 · {matrix?.rows.filter(row => row.cells?.length).length ?? 0} 个已有取值</span>
          </div>
          {matrix && matrix.rows.some(row => row.cells?.length)
            ? <FeatureMatrixTable matrix={matrix}/>
            : <div className="panel-empty">{assets.length
              ? '这批素材还没有提取过特征。类型识别完成后要先跑一次提取，矩阵才有内容。'
              : `当前 Project 还没有${assetTypeLabels[target]}素材。右侧列出的是这类素材能问出的全部变量。`}</div>}
        </> : null}

        {listState === 'ready' && target === 'single' ? <>
          <div className="content-asset-strip">
            {breakdownAssets.length
              ? breakdownAssets.map(asset => <button key={asset.id} className={asset.id === selectedId ? 'active' : ''} onClick={() => setSelectedId(asset.id)}>
                {asset.title} · v{asset.revision}
              </button>)
              : <span className="panel-empty">当前 Project 还没有可分析素材。</span>}
          </div>
          {selectedAsset && !selectedAsset.asset_type
            ? <div className="content-breakdown">
              <div className="prelaunch-boundary"><CircleAlert size={16}/><span><small>类型待识别</small>不知道这是哪类素材，就不知道该问哪套变量。先判定类型（AM-004），再谈特征提取。</span></div>
              <div className="content-asset-strip">
                {assetTypeOrder.map(assetType => <button key={assetType} onClick={() => { void identifyType(assetType) }}>判定为 {assetTypeLabels[assetType]}</button>)}
              </div>
            </div>
            : null}
          {selectedAsset?.asset_type && typeSchema ? <FeatureBreakdown
            schema={typeSchema}
            features={features}
            editingKey={editingKey}
            draft={draft}
            onDraft={setDraft}
            onEdit={key => { setEditingKey(key); setDraft('') }}
            onCancel={() => { setEditingKey('') }}
            onConfirm={(key, value) => { void writeHumanLayer(key, value, 'confirmed', '人工认可 AI 提取结果') }}
            onReject={(key, value) => { void writeHumanLayer(key, value, 'rejected', '人工推翻并改写 AI 提取结果') }}
          /> : null}
        </> : null}
      </section>

      <aside className="prelaunch-detail">
        {target === 'single' && selectedAsset ? <>
          <span className="section-label">素材</span><h3>{selectedAsset.title}</h3>
          <p>第 {selectedAsset.revision} 版 · {statusLabels[selectedAsset.analysis_status] ?? selectedAsset.analysis_status}</p>
          <div className="prelaunch-fact"><Layers3 size={17}/><span><small>内容类型</small><b>
            {selectedAsset.asset_type ? assetTypeLabels[selectedAsset.asset_type] : '待识别'}
            {selectedAsset.asset_type_source === 'ai' ? `（AI 识别 · 置信 ${confidenceLabels[selectedAsset.asset_type_confidence ?? 'low']}）`
              : selectedAsset.asset_type_source === 'human' ? '（人工判定）' : ''}
          </b></span></div>
          <div className="prelaunch-fact"><Sparkles size={17}/><span><small>已提取</small><b>
            AI {features.filter(feature => feature.source === 'ai').length} 项 · 人工 {features.filter(feature => feature.source === 'human').length} 项
            {typeSchema ? ` · 这类素材共 ${typeSchema.fields.length} 个变量` : ''}
          </b></span></div>
          {selectedAsset.analysis_status_reason
            ? <div className="prelaunch-fact"><Lightbulb size={17}/><span><small>最近一次状态说明</small><b>{selectedAsset.analysis_status_reason}</b></span></div>
            : null}
          <div className="prelaunch-fact"><UserCheck size={17}/><span><small>两层为什么不合并</small><b>人工结论单独成行，后台再跑一次提取也不会盖掉它；AI 那一行也保留，方便回头看机器当时判断成了什么。</b></span></div>
        </> : target !== 'single' ? <>
          <span className="section-label">特征体系</span><h3>{typeSchema?.label ?? assetTypeLabels[target]}</h3>
          <p>{typeSchema?.fields.length ?? 0} 个变量 · 来源 {typeSchema?.source ?? '03 §5'}</p>
          <div className="prelaunch-fact"><Sparkles size={17}/><span><small>为什么按类型分开</small><b>视频的钩子类型问不到公众号文章头上。每类素材只带它自己的内容支持的变量，混用会得到一列没有意义的对比。</b></span></div>
          {typeSchema ? groupsOf(typeSchema).map(group => <div className="feature-stack" key={group}>
            <span>{group}</span>
            {typeSchema.fields.filter(field => field.group === group).map(field => <b key={field.key}>{field.label}</b>)}
          </div>) : null}
          {matrix?.disclosure ? <div className="prelaunch-boundary"><CircleAlert size={16}/><span><small>口径</small>{matrix.disclosure}</span></div> : null}
        </> : <div className="panel-empty">选择上方素材后查看它的类型与提取情况。</div>}
        {notice ? <div className="inline-notice" role="status">{notice}</div> : null}
      </aside>
    </div>
  </StateBoundary>
}

function FeatureMatrixTable({ matrix }: { matrix: ApiFeatureMatrix }) {
  const groups = [...new Set(matrix.rows.map(row => row.group))]
  return <div className="content-matrix" style={{ '--matrix-columns': matrix.assets.length } as React.CSSProperties}>
    <div className="content-matrix-row header">
      <span>特征</span>
      {matrix.assets.map(asset => <span key={asset.id}>{asset.title}<small>v{asset.revision}</small></span>)}
    </div>
    {groups.map(group => <Fragment key={group}>
      <div className="content-matrix-group">{group}</div>
      {matrix.rows.filter(row => row.group === group).map(row => <div className="content-matrix-row" key={row.key}>
        <span><b>{row.label}</b><small>{row.key}</small></span>
        {matrix.assets.map(asset => {
          const cells = (row.cells ?? []).filter(cell => cell.asset_id === asset.id)
          const ai = cells.find(cell => cell.source === 'ai')
          const human = cells.find(cell => cell.source === 'human')
          return <span key={asset.id}>
            {human ? <b className="content-layer human">人工 · {formatValue(human.value)}</b> : null}
            {ai ? <b className="content-layer ai">AI · {formatValue(ai.value)} · 置信{confidenceLabels[ai.confidence ?? 'low']}</b> : null}
            {!ai && !human ? <b className="content-layer">—</b> : null}
          </span>
        })}
      </div>)}
    </Fragment>)}
  </div>
}

function FeatureBreakdown({ schema, features, editingKey, draft, onDraft, onEdit, onCancel, onConfirm, onReject }: {
  schema: ApiFeatureSchema
  features: ApiInsightAssetFeature[]
  editingKey: string
  draft: string
  onDraft: (value: string) => void
  onEdit: (key: string) => void
  onCancel: () => void
  onConfirm: (key: string, value: ApiFeatureValue) => void
  onReject: (key: string, value: ApiFeatureValue) => void
}) {
  return <div className="content-breakdown">
    {groupsOf(schema).map(group => <Fragment key={group}>
      <div className="content-matrix-group">{group}</div>
      {schema.fields.filter(field => field.group === group).map(field => {
        const ai = features.find(feature => feature.source === 'ai' && feature.key === field.key)
        const human = features.find(feature => feature.source === 'human' && feature.key === field.key)
        const editing = editingKey === field.key
        return <div className="content-breakdown-row" key={field.key}>
          <span><b>{field.label}</b><small>{field.key} · {kindLabels[field.kind] ?? field.kind}{field.unit ? ` · ${field.unit}` : ''}</small></span>
          <span>
            {ai ? <b className="content-layer ai">AI · {formatValue(ai.value)} · 置信{confidenceLabels[ai.confidence ?? 'low']}</b> : <b className="content-layer">AI 还没提取这一项</b>}
            {human ? <b className="content-layer human">人工 · {formatValue(human.value)} · {human.review_state === 'rejected' ? '推翻 AI' : '认可'}</b> : null}
            {editing ? <input
              autoFocus
              aria-label={`改写 ${field.label}`}
              value={draft}
              placeholder={placeholderFor(field.kind, field.vocabulary)}
              onChange={event => onDraft(event.target.value)}
            /> : null}
          </span>
          <span className="actions">
            {editing ? <>
              <button className="secondary-button" disabled={!draft.trim()} onClick={() => onReject(field.key, parseValue(field.kind, draft))}>保存人工结论</button>
              <button className="secondary-button" onClick={onCancel}>取消</button>
            </> : <>
              {ai && !human ? <button className="secondary-button" onClick={() => onConfirm(field.key, ai.value)}>认可</button> : null}
              <button className="secondary-button" onClick={() => onEdit(field.key)}>{human ? '改写' : ai ? '推翻并改写' : '人工填写'}</button>
            </>}
          </span>
        </div>
      })}
    </Fragment>)}
  </div>
}

const kindLabels: Record<string, string> = {
  text: '文本', tags: '开放标签', enum: '单选', enum_multi: '多选',
  number: '数值', bool: '是否', duration_seconds: '时长（秒）',
}

function groupsOf(schema: ApiFeatureSchema): string[] {
  return [...new Set(schema.fields.map(field => field.group))]
}

function placeholderFor(kind: ApiFeatureValueKind, vocabulary?: string[]): string {
  if (kind === 'bool') return '填「是」或「否」'
  if (kind === 'number') return '填数字'
  if (kind === 'duration_seconds') return '填秒数'
  if (vocabulary?.length) return `可选：${vocabulary.slice(0, 6).join('、')}`
  if (kind === 'text') return '填写人工结论'
  return '多个取值用顿号分隔'
}

function parseValue(kind: ApiFeatureValueKind, raw: string): ApiFeatureValue {
  const text = raw.trim()
  if (kind === 'bool') return { kind, bool: /^(是|yes|true|1)$/i.test(text) }
  if (kind === 'number' || kind === 'duration_seconds') return { kind, number: Number(text) || 0 }
  if (kind === 'text') return { kind, text }
  return { kind, terms: text.split(/[、,，/|\s]+/).filter(Boolean) }
}

function formatValue(value: ApiFeatureValue): string {
  if (value.kind === 'bool') return value.bool ? '是' : '否'
  if (value.kind === 'number') return String(value.number ?? 0)
  if (value.kind === 'duration_seconds') return `${value.number ?? 0} 秒`
  if (value.terms?.length) return value.terms.join('、')
  return value.text || '（空）'
}
