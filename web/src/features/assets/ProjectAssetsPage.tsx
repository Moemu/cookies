import { useCallback, useDeferredValue, useEffect, useMemo, useRef, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { ApiProblem } from '../../shared/api/client'
import { getProjectContext } from '../platform/api'
import type { Project, ProjectContext } from '../platform/types'
import { getAssetPreview, listAssetFeatures, listProjectAssets, removeProjectAsset } from './api'
import { AssetIcon } from './AssetIcon'
import type { AssetFeature, AssetSource, AssetStatus, ProjectAsset, UploadSession } from './types'
import { RemoveAssetDialog } from './RemoveAssetDialog'
import { UploadDrawer } from './UploadDrawer'

type ViewMode = 'grid' | 'list'

const sourceLabels: Record<AssetSource, string> = {
  upload: '用户上传',
  provider_generated: 'Provider 生成',
  imported: '导入',
  captured: '采集',
}

const statusLabels: Record<AssetStatus, string> = {
  processing: '处理中',
  ready: '已就绪',
  quarantined: '已隔离',
  failed: '失败',
  archived: '已归档',
}

function formatBytes(bytes: number) {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / 1024 / 1024).toFixed(2)} MB`
}

function formatDate(value: string) {
  return new Intl.DateTimeFormat('zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit' }).format(new Date(value))
}

function assetLabel(asset: ProjectAsset) {
  return `${asset.asset.id} · v${asset.version.version}`
}

function assetFeatureKey(asset: ProjectAsset) {
  return `${asset.asset.id}:${asset.version.version}`
}

function percent(value: number) {
  return `${Math.round(value * 100)}%`
}

function riskLabel(risk: AssetFeature['similarity_risk']) {
  if (risk === 'high') return '高'
  if (risk === 'medium') return '中'
  return '低'
}

function featureSummary(feature?: AssetFeature) {
  if (!feature) return '暂无特征，Planner 使用基础元数据降级。'
  const point = feature.selling_points[0] ? ` · 卖点：${feature.selling_points[0]}` : ''
  return `Hook ${percent(feature.hook_strength)} · 商品露出 ${percent(feature.product_visibility)} · 相似度风险 ${riskLabel(feature.similarity_risk)}${point}`
}

function removeErrorMessage(error: unknown) {
  if (error instanceof ApiProblem) return `${error.problem.error.message}（${error.problem.error.code}）`
  if (error instanceof Error) return error.message
  return '素材删除失败，请稍后重试。'
}

function AssetCard({ asset, editUrl, feature, onRemove, previewUnavailable, previewUrl, view }: { asset: ProjectAsset; editUrl?: string; feature?: AssetFeature; onRemove: () => void; previewUnavailable?: boolean; previewUrl?: string; view: ViewMode }) {
  const dimensions = asset.version.width_pixels && asset.version.height_pixels
    ? `${asset.version.width_pixels} × ${asset.version.height_pixels}`
    : '尺寸未记录'
  const content = previewUrl
    ? <img alt={`${asset.asset.id} 预览`} loading="lazy" src={previewUrl} />
    : <div className="asset-thumbnail__fallback" title={previewUnavailable ? '\u539f\u59cb\u6587\u4ef6\u4e0d\u5b58\u5728\uff0c\u8bf7\u91cd\u65b0\u4e0a\u4f20' : undefined}><AssetIcon name="image" size={30} /><span>{asset.version.mime_type.replace('image/', '').toUpperCase()}</span>{previewUnavailable ? <small>{'\u9884\u89c8\u4e0d\u53ef\u7528'}</small> : null}</div>

  return <article className={view === 'list' ? 'asset-card asset-card--list' : 'asset-card'}>
    {editUrl ? <Link aria-label={`编辑 ${assetLabel(asset)}`} className="asset-card__edit" title="基于此素材编辑图片" to={editUrl}>编辑</Link> : null}
    <button aria-label={`删除 ${assetLabel(asset)}`} className="asset-card__remove" onClick={onRemove} title="从项目中删除" type="button"><AssetIcon name="delete" size={17} /></button>
    {previewUrl
      ? <a className="asset-thumbnail" href={previewUrl} rel="noreferrer" target="_blank" title="在新窗口查看预览">{content}</a>
      : <div className="asset-thumbnail">{content}</div>}
    <div className="asset-card__body">
      <h3 title={asset.asset.id}>{assetLabel(asset)}</h3>
      <div className="asset-source"><span className={`source-dot source-dot--${asset.version.source_type}`} />{sourceLabels[asset.version.source_type]}</div>
      <div className="asset-facts"><span>{dimensions}</span><span>{formatBytes(asset.version.size_bytes)}</span></div>
      <p className={feature ? 'asset-feature-summary' : 'asset-feature-summary asset-feature-summary--empty'}>{featureSummary(feature)}</p>
      <div className="asset-footer"><span className={`asset-status asset-status--${asset.asset.status}`}><i />{statusLabels[asset.asset.status]}</span><time dateTime={asset.created_at}>{formatDate(asset.created_at)}</time></div>
    </div>
  </article>
}

export function ProjectAssetsPage({ project }: { project?: Pick<Project, 'name' | 'status'> }) {
  const { projectId = '' } = useParams()
  const [assets, setAssets] = useState<ProjectAsset[]>([])
  const [features, setFeatures] = useState<AssetFeature[]>([])
  const [context, setContext] = useState<ProjectContext | null>(null)
  const [previewUrls, setPreviewUrls] = useState<Record<string, string>>({})
  const [unavailablePreviewIds, setUnavailablePreviewIds] = useState<Set<string>>(() => new Set())
  const [localPreviewUrls, setLocalPreviewUrls] = useState<Record<string, string>>({})
  const ownedObjectUrls = useRef<string[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [query, setQuery] = useState('')
  const [source, setSource] = useState<'all' | AssetSource>('all')
  const [status, setStatus] = useState<'all' | AssetStatus>('all')
  const [view, setView] = useState<ViewMode>('grid')
  const [uploadOpen, setUploadOpen] = useState(false)
  const [assetToRemove, setAssetToRemove] = useState<ProjectAsset | null>(null)
  const [removing, setRemoving] = useState(false)
  const [removeError, setRemoveError] = useState('')
  const deferredQuery = useDeferredValue(query.trim().toLowerCase())

  const loadLibrary = useCallback(async (signal?: AbortSignal) => {
    await Promise.resolve()
    setLoading(true)
    setError('')
    try {
      const [assetList, projectContext, featureList] = await Promise.all([
        listProjectAssets(projectId, signal),
        getProjectContext(projectId, signal),
        listAssetFeatures(projectId, signal),
      ])
      setAssets(assetList.items)
      setFeatures(featureList.items)
      setContext(projectContext)

      const previews = await Promise.all(assetList.items.map(async (item) => {
        try {
          const signed = await getAssetPreview(projectId, item.asset.id, item.version.version, signal)
          return [item.asset.id, signed.url, !signed.url] as const
        } catch (caught) {
          if (caught instanceof DOMException && caught.name === 'AbortError') throw caught
          return [item.asset.id, '', true] as const
        }
      }))
      setPreviewUrls(Object.fromEntries(previews.filter(([, url]) => Boolean(url)).map(([id, url]) => [id, url])))
      setUnavailablePreviewIds(new Set(previews.filter(([, , unavailable]) => unavailable).map(([id]) => id)))
    } catch (caught) {
      if (caught instanceof DOMException && caught.name === 'AbortError') return
      setError(caught instanceof Error ? caught.message : '素材库加载失败。')
    } finally {
      if (!signal?.aborted) setLoading(false)
    }
  }, [projectId])

  useEffect(() => {
    const controller = new AbortController()
    const task = window.setTimeout(() => void loadLibrary(controller.signal), 0)
    return () => {
      window.clearTimeout(task)
      controller.abort()
    }
  }, [loadLibrary])

  useEffect(() => () => ownedObjectUrls.current.forEach((url) => URL.revokeObjectURL(url)), [])

  const filteredAssets = useMemo(() => assets.filter((asset) => {
    if (source !== 'all' && asset.version.source_type !== source) return false
    if (status !== 'all' && asset.asset.status !== status) return false
    if (!deferredQuery) return true
    return `${asset.asset.id} ${asset.version.mime_type} ${sourceLabels[asset.version.source_type]}`.toLowerCase().includes(deferredQuery)
  }), [assets, deferredQuery, source, status])
  const readyCount = assets.filter((asset) => asset.asset.status === 'ready').length
  const generatedCount = assets.filter((asset) => asset.version.source_type === 'provider_generated').length
  const projectCanAcceptAssets = project?.status === 'draft' ? false : context ? Boolean(context.brand_id) : true
  const featuresByAsset = useMemo(() => new Map(features.map((feature) => [`${feature.asset_id}:${feature.asset_version}`, feature])), [features])

  function handleUploadComplete(file: File, session: UploadSession) {
    const assetId = session.project_asset_ref?.asset_version.asset_id
    if (assetId) {
      const url = URL.createObjectURL(file)
      ownedObjectUrls.current.push(url)
      setLocalPreviewUrls((current) => ({ ...current, [assetId]: url }))
      setUnavailablePreviewIds((current) => {
        const next = new Set(current)
        next.delete(assetId)
        return next
      })
    }
    void loadLibrary()
  }

  async function handleRemove() {
    if (!assetToRemove || removing) return
    setRemoving(true)
    setRemoveError('')
    const assetId = assetToRemove.asset.id
    const version = assetToRemove.version.version
    try {
      await removeProjectAsset(projectId, assetId, version)
      setAssets((current) => current.filter((item) => item.asset.id !== assetId || item.version.version !== version))
      setPreviewUrls((current) => {
        const next = { ...current }
        delete next[assetId]
        return next
      })
      setUnavailablePreviewIds((current) => {
        const next = new Set(current)
        next.delete(assetId)
        return next
      })
      const localUrl = localPreviewUrls[assetId]
      if (localUrl) {
        URL.revokeObjectURL(localUrl)
        ownedObjectUrls.current = ownedObjectUrls.current.filter((url) => url !== localUrl)
        setLocalPreviewUrls((current) => {
          const next = { ...current }
          delete next[assetId]
          return next
        })
      }
      setAssetToRemove(null)
    } catch (caught) {
      setRemoveError(removeErrorMessage(caught))
    } finally {
      setRemoving(false)
    }
  }

  return <section className={uploadOpen ? 'asset-page asset-page--drawer-open' : 'asset-page'}>
    <header className="page-header">
      <div>
        <h1>{project?.name ? `${project.name} · 素材库` : '项目素材库'}</h1>
      <p>统一管理上传与 Provider 生成素材。删除会移除项目关联，底层不可变版本仍保留用于审计和溯源。</p>
      </div>
      <div className="context-summary" title={context?.brand_id || '未绑定品牌'}>
        <span className={context?.brand_id ? 'context-dot context-dot--active' : 'context-dot'} />
        <span>{context?.brand_id ? '品牌上下文已绑定' : '等待品牌上下文'}</span>
        {context ? <strong>v{context.project_context_version}</strong> : null}
      </div>
    </header>

    <div className="asset-summary" aria-label="素材库概况">
      <div><span>全部素材</span><strong>{assets.length}</strong></div>
      <div><span>已就绪</span><strong>{readyCount}</strong></div>
      <div><span>Provider 生成</span><strong>{generatedCount}</strong></div>
      <div><span>上下文版本</span><strong>{context ? `v${context.project_context_version}` : '—'}</strong></div>
      <p><span className="immutability-mark" aria-hidden="true">↳</span> 统计当前项目中未删除的素材；预览链接按访问即时签发。</p>
    </div>
    <details className="asset-metric-help"><summary>这些指标怎么看？</summary><p><strong>全部素材</strong>是当前项目可见数量；<strong>已就绪</strong>表示可预览；<strong>Provider 生成</strong>按来源统计；<strong>上下文版本</strong>表示生成或上传时采用的品牌与产品上下文版本。</p></details>

    {!projectCanAcceptAssets ? <div className="context-warning" role="status"><strong>该项目尚未启用素材写入</strong><span>需要先绑定有效品牌并激活项目。</span></div> : null}

    <div className="asset-toolbar">
      <label className="search-control"><AssetIcon name="search" /><span className="sr-only">搜索素材</span><input onChange={(event) => setQuery(event.target.value)} placeholder="搜索资产 ID 或类型" value={query} /></label>
      <label className="select-control"><span className="sr-only">素材来源</span><select onChange={(event) => setSource(event.target.value as 'all' | AssetSource)} value={source}><option value="all">全部来源</option><option value="upload">用户上传</option><option value="provider_generated">Provider 生成</option><option value="imported">导入</option><option value="captured">采集</option></select></label>
      <label className="select-control"><span className="sr-only">素材状态</span><select onChange={(event) => setStatus(event.target.value as 'all' | AssetStatus)} value={status}><option value="all">全部状态</option><option value="ready">已就绪</option><option value="processing">处理中</option><option value="quarantined">已隔离</option><option value="failed">失败</option><option value="archived">已归档</option></select></label>
      <div className="toolbar-spacer" />
      <button className="icon-button" aria-label="刷新素材" disabled={loading} onClick={() => void loadLibrary()} type="button"><AssetIcon name="refresh" /></button>
      <Link className="button button--secondary upload-button" to={`/projects/${encodeURIComponent(projectId)}/assets/remix`}>AI 混剪</Link>
      <div className="view-toggle" aria-label="视图方式">
        <button aria-label="网格视图" aria-pressed={view === 'grid'} onClick={() => setView('grid')} type="button"><AssetIcon name="grid" /></button>
        <button aria-label="列表视图" aria-pressed={view === 'list'} onClick={() => setView('list')} type="button"><AssetIcon name="list" /></button>
      </div>
      <button className="button button--primary upload-button" disabled={!projectCanAcceptAssets} onClick={() => setUploadOpen(true)} title={projectCanAcceptAssets ? '添加新素材' : '项目未绑定有效品牌'} type="button"><AssetIcon name="upload" />添加素材</button>
    </div>

    {error ? <div className="library-error" role="alert"><div><strong>无法加载素材库</strong><span>{error}</span></div><button className="text-button" onClick={() => void loadLibrary()} type="button">重试</button></div> : null}

    {loading && assets.length === 0 ? <div className="asset-loading" aria-label="正在加载素材"><span /><span /><span /></div> : null}
    {!loading && !error && filteredAssets.length === 0 ? <div className="library-empty">
      <div className="empty-image"><AssetIcon name="image" size={32} /></div>
      <h2>{assets.length === 0 ? '项目还没有素材' : '没有匹配的素材'}</h2>
      <p>{assets.length === 0 ? '上传第一张 PNG 或 JPEG，建立项目的可复用资产库。' : '尝试清除搜索词或调整筛选条件。'}</p>
      {assets.length === 0 && projectCanAcceptAssets ? <button className="button button--primary" onClick={() => setUploadOpen(true)} type="button">添加第一张素材</button> : null}
    </div> : null}

    {filteredAssets.length > 0 ? <div className={view === 'list' ? 'asset-collection asset-collection--list' : 'asset-collection'} aria-busy={loading}>
      {filteredAssets.map((asset) => <AssetCard asset={asset} editUrl={asset.asset.status === 'ready' && (localPreviewUrls[asset.asset.id] || previewUrls[asset.asset.id]) ? `/projects/${encodeURIComponent(projectId)}/assets/${encodeURIComponent(asset.asset.id)}/versions/${asset.version.version}/edit` : undefined} feature={featuresByAsset.get(assetFeatureKey(asset))} key={`${asset.asset.id}:${asset.version.version}`} onRemove={() => {
        setRemoveError('')
        setAssetToRemove(asset)
      }} previewUnavailable={unavailablePreviewIds.has(asset.asset.id)} previewUrl={localPreviewUrls[asset.asset.id] || previewUrls[asset.asset.id]} view={view} />)}
    </div> : null}

    {assets.length > 0 ? <footer className="library-count">显示 {filteredAssets.length} / {assets.length} 条素材</footer> : null}

    <UploadDrawer open={uploadOpen} projectId={projectId} onClose={() => setUploadOpen(false)} onComplete={handleUploadComplete} />
    <RemoveAssetDialog asset={assetToRemove} busy={removing} error={removeError} onClose={() => {
      if (!removing) {
        setAssetToRemove(null)
        setRemoveError('')
      }
    }} onConfirm={() => void handleRemove()} />
  </section>
}
