import { useCallback, useEffect, useMemo, useState } from 'react'
import { Plus, Search, X } from 'lucide-react'
import { useProject } from '../../../context/ProjectContext'
import {
  api,
  type ApiFeatureSchema,
  type ApiInsightAsset,
  type ApiSimilarAssetResult,
} from '../../../data/api'
import { shortId } from '../../../data/shortId'
import { SimilarPanel } from './SimilarPanel'

/**
 * 「找相似」视图。
 *
 * 两种问法都从这里进：给一条素材问「和它像的还有哪些」，或者直接给一组变量问
 * 「时长 15 秒的还有哪些」。后一种是 ❓「算不出来」的升级通道——本轮样本不够时，
 * 从库里把同样取值的素材拉过来，样本就厚了。
 *
 * 素材那一问用下拉选，不让人手填 ID：填错一个字符只会得到一句「没找到」，
 * 而人会以为是「库里真的没有像它的」。
 */
export function SimilarView() {
  const { currentProject } = useProject()
  const [mode, setMode] = useState<'asset' | 'features'>('asset')
  const [assets, setAssets] = useState<ApiInsightAsset[]>([])
  const [schemas, setSchemas] = useState<ApiFeatureSchema[]>([])
  const [assetId, setAssetId] = useState('')
  const [featureKey, setFeatureKey] = useState('')
  const [featureValue, setFeatureValue] = useState('')
  const [features, setFeatures] = useState<Record<string, string>>({})
  const [result, setResult] = useState<ApiSimilarAssetResult | null>(null)
  const [state, setState] = useState<'idle' | 'loading' | 'ready' | 'error'>('idle')
  const [notice, setNotice] = useState('')

  const load = useCallback(async () => {
    if (!currentProject.id) return
    try {
      const [assetPage, schemaPage] = await Promise.all([
        api.listInsightAssets(currentProject.id, {}),
        api.listFeatureSchemas(currentProject.id),
      ])
      // 已失效素材不做种子：拿一条已经不作数的素材去找相似，找回来的一组从一开始就不该用。
      setAssets(assetPage.items.filter(asset => asset.analysis_status !== 'retired'))
      setSchemas(schemaPage.items)
    } catch {
      setAssets([])
      setSchemas([])
    }
  }, [currentProject.id])

  useEffect(() => { void load() }, [load])

  // 变量按键去重：六套特征体系里同名的键说的是同一件事，列六遍只会让人以为要选对类型。
  const fields = useMemo(() => {
    const seen = new Map<string, string>()
    for (const schema of schemas) {
      for (const field of schema.fields) {
        if (!seen.has(field.key)) seen.set(field.key, field.label)
      }
    }
    return [...seen.entries()].map(([key, label]) => ({ key, label }))
  }, [schemas])

  const conditions = Object.entries(features)
  const canSearch = mode === 'asset' ? Boolean(assetId) : conditions.length > 0

  const search = () => {
    if (!currentProject.id || !canSearch) return
    setState('loading')
    setNotice('')
    const body = mode === 'asset' ? { asset_id: assetId } : { features }
    api.findSimilarAssets(currentProject.id, body)
      .then(value => { setResult(value); setState('ready') })
      .catch(cause => {
        setResult(null)
        setState('error')
        setNotice(cause instanceof Error ? cause.message : '没找成，稍后再试。')
      })
  }

  const addCondition = () => {
    const key = featureKey.trim()
    const value = featureValue.trim()
    if (!key || !value) return
    setFeatures(current => ({ ...current, [key]: value }))
    setFeatureValue('')
  }

  return <section className="similar-view">
    <p className="prelaunch-disclosure">
      按变量重叠找，不按整体相似度。所以每条结果都能说清「像在哪」——
      拿这批素材去把样本做厚的时候，你得说得出它们为什么能凑成一组。
    </p>

    <div className="similar-search">
      <label>问法<select aria-label="问法" value={mode} onChange={event => {
        setMode(event.target.value as 'asset' | 'features')
        setResult(null)
        setState('idle')
      }}>
        <option value="asset">和这条素材像的</option>
        <option value="features">这些变量取值一样的</option>
      </select></label>

      {mode === 'asset'
        ? <label>素材<select aria-label="素材" value={assetId} onChange={event => setAssetId(event.target.value)}>
          <option value="">请选择</option>
          {assets.map(asset => <option key={asset.id} value={asset.id}>
            {asset.title} · {shortId(asset.id)}
          </option>)}
        </select></label>
        : <>
          <label>变量<select aria-label="变量" value={featureKey} onChange={event => setFeatureKey(event.target.value)}>
            <option value="">请选择</option>
            {fields.map(field => <option key={field.key} value={field.key}>{field.label}（{field.key}）</option>)}
          </select></label>
          <input aria-label="取值" value={featureValue} placeholder="取值，例如 15"
            onChange={event => setFeatureValue(event.target.value)}/>
          <button type="button" className="secondary-button" onClick={addCondition}
            disabled={!featureKey.trim() || !featureValue.trim()}>
            <Plus size={15}/>加一条
          </button>
        </>}

      <button type="button" className="secondary-button" onClick={search}
        disabled={state === 'loading' || !canSearch}>
        <Search size={15}/>{state === 'loading' ? '正在找…' : '找相似'}
      </button>
    </div>

    {mode === 'features' ? <div className="feature-stack">
      <span>要一起满足的变量（最多 20 条，全部一致才算重叠）</span>
      {conditions.length ? conditions.map(([key, value]) => <b key={key}>
        {fields.find(field => field.key === key)?.label ?? key} = {value}
        <button type="button" aria-label={`删掉 ${key}`} onClick={() => setFeatures(current => {
          const next = { ...current }
          delete next[key]
          return next
        })}><X size={11}/></button>
      </b>) : <b>还没加条件。先挑一个变量、填上取值。</b>}
    </div> : null}

    {notice ? <p className="form-error">{notice}</p> : null}
    {state === 'ready' && result ? <SimilarPanel result={result}/> : null}
    {state === 'idle' ? <p className="panel-empty">选好之后按「找相似」。</p> : null}
  </section>
}
