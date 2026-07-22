import { useCallback, useDeferredValue, useEffect, useMemo, useRef, useState } from 'react'
import { useParams } from 'react-router-dom'
import { getProjectContext } from '../platform/api'
import type { ProjectContext } from '../platform/types'
import { getAssetPreview, listProjectAssets } from './api'
import { AssetIcon } from './AssetIcon'
import type { AssetSource, AssetStatus, ProjectAsset, UploadSession } from './types'
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

function AssetCard({ asset, previewUrl, view }: { asset: ProjectAsset; previewUrl?: string; view: ViewMode }) {
  const dimensions = asset.version.width_pixels && asset.version.height_pixels
    ? `${asset.version.width_pixels} × ${asset.version.height_pixels}`
    : '尺寸未记录'
  const content = previewUrl
    ? <img alt={`${asset.asset.id} 预览`} loading="lazy" src={previewUrl} />
    : <div className="asset-thumbnail__fallback"><AssetIcon name="image" size={30} /><span>{asset.version.mime_type.replace('image/', '').toUpperCase()}</span></div>

  return <article className={view === 'list' ? 'asset-card asset-card--list' : 'asset-card'}>
    {previewUrl
      ? <a className="asset-thumbnail" href={previewUrl} rel="noreferrer" target="_blank" title="在新窗口查看预览">{content}</a>
      : <div className="asset-thumbnail">{content}</div>}
    <div className="asset-card__body">
      <h3 title={asset.asset.id}>{assetLabel(asset)}</h3>
      <div className="asset-source"><span className={`source-dot source-dot--${asset.version.source_type}`} />{sourceLabels[asset.version.source_type]}</div>
      <div className="asset-facts"><span>{dimensions}</span><span>{formatBytes(asset.version.size_bytes)}</span></div>
      <div className="asset-footer"><span className={`asset-status asset-status--${asset.asset.status}`}><i />{statusLabels[asset.asset.status]}</span><time dateTime={asset.created_at}>{formatDate(asset.created_at)}</time></div>
    </div>
  </article>
}

export function ProjectAssetsPage() {
  const { projectId = '' } = useParams()
  const [assets, setAssets] = useState<ProjectAsset[]>([])
  const [context, setContext] = useState<ProjectContext | null>(null)
  const [previewUrls, setPreviewUrls] = useState<Record<string, string>>({})
  const [localPreviewUrls, setLocalPreviewUrls] = useState<Record<string, string>>({})
  const ownedObjectUrls = useRef<string[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [query, setQuery] = useState('')
  const [source, setSource] = useState<'all' | AssetSource>('all')
  const [status, setStatus] = useState<'all' | AssetStatus>('all')
  const [view, setView] = useState<ViewMode>('grid')
  const [uploadOpen, setUploadOpen] = useState(false)
  const deferredQuery = useDeferredValue(query.trim().toLowerCase())

  const loadLibrary = useCallback(async (signal?: AbortSignal) => {
    await Promise.resolve()
    setLoading(true)
    setError('')
    try {
      const [assetList, projectContext] = await Promise.all([
        listProjectAssets(projectId, signal),
        getProjectContext(projectId, signal),
      ])
      setAssets(assetList.items)
      setContext(projectContext)

      const previews = await Promise.all(assetList.items.map(async (item) => {
        try {
          const signed = await getAssetPreview(projectId, item.asset.id, item.version.version, signal)
          return [item.asset.id, signed.url] as const
        } catch {
          return [item.asset.id, ''] as const
        }
      }))
      setPreviewUrls(Object.fromEntries(previews.filter(([, url]) => Boolean(url))))
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

  function handleUploadComplete(file: File, session: UploadSession) {
    const assetId = session.project_asset_ref?.asset_version.asset_id
    if (assetId) {
      const url = URL.createObjectURL(file)
      ownedObjectUrls.current.push(url)
      setLocalPreviewUrls((current) => ({ ...current, [assetId]: url }))
    }
    void loadLibrary()
  }

  return <section className={uploadOpen ? 'asset-page asset-page--drawer-open' : 'asset-page'}>
    <header className="page-header">
      <div>
        <h1>项目素材库</h1>
        <p>统一管理项目图片与生成素材，所有版本都保留来源和项目上下文。</p>
      </div>
      <div className="context-summary" title={context?.brand_id || '未绑定品牌'}>
        <span className={context?.brand_id ? 'context-dot context-dot--active' : 'context-dot'} />
        <span>{context?.brand_id ? '品牌上下文已绑定' : '等待品牌上下文'}</span>
        {context ? <strong>v{context.project_context_version}</strong> : null}
      </div>
    </header>

    <div className="asset-toolbar">
      <label className="search-control"><AssetIcon name="search" /><span className="sr-only">搜索素材</span><input onChange={(event) => setQuery(event.target.value)} placeholder="搜索资产 ID 或类型" value={query} /></label>
      <label className="select-control"><span className="sr-only">素材来源</span><select onChange={(event) => setSource(event.target.value as 'all' | AssetSource)} value={source}><option value="all">全部来源</option><option value="upload">用户上传</option><option value="provider_generated">Provider 生成</option><option value="imported">导入</option><option value="captured">采集</option></select></label>
      <label className="select-control"><span className="sr-only">素材状态</span><select onChange={(event) => setStatus(event.target.value as 'all' | AssetStatus)} value={status}><option value="all">全部状态</option><option value="ready">已就绪</option><option value="processing">处理中</option><option value="quarantined">已隔离</option><option value="failed">失败</option><option value="archived">已归档</option></select></label>
      <div className="toolbar-spacer" />
      <button className="icon-button" aria-label="刷新素材" disabled={loading} onClick={() => void loadLibrary()} type="button"><AssetIcon name="refresh" /></button>
      <div className="view-toggle" aria-label="视图方式">
        <button aria-label="网格视图" aria-pressed={view === 'grid'} onClick={() => setView('grid')} type="button"><AssetIcon name="grid" /></button>
        <button aria-label="列表视图" aria-pressed={view === 'list'} onClick={() => setView('list')} type="button"><AssetIcon name="list" /></button>
      </div>
      <button className="button button--primary upload-button" onClick={() => setUploadOpen(true)} type="button"><AssetIcon name="upload" />上传素材</button>
    </div>

    {error ? <div className="library-error" role="alert"><div><strong>无法加载素材库</strong><span>{error}</span></div><button className="text-button" onClick={() => void loadLibrary()} type="button">重试</button></div> : null}

    {loading && assets.length === 0 ? <div className="asset-loading" aria-label="正在加载素材"><span /><span /><span /></div> : null}
    {!loading && !error && filteredAssets.length === 0 ? <div className="library-empty">
      <div className="empty-image"><AssetIcon name="image" size={32} /></div>
      <h2>{assets.length === 0 ? '项目还没有素材' : '没有匹配的素材'}</h2>
      <p>{assets.length === 0 ? '上传第一张 PNG 或 JPEG，建立项目的可复用资产库。' : '尝试清除搜索词或调整筛选条件。'}</p>
      {assets.length === 0 ? <button className="button button--primary" onClick={() => setUploadOpen(true)} type="button">上传第一张素材</button> : null}
    </div> : null}

    {filteredAssets.length > 0 ? <div className={view === 'list' ? 'asset-collection asset-collection--list' : 'asset-collection'} aria-busy={loading}>
      {filteredAssets.map((asset) => <AssetCard asset={asset} key={`${asset.asset.id}:${asset.version.version}`} previewUrl={localPreviewUrls[asset.asset.id] || previewUrls[asset.asset.id]} view={view} />)}
    </div> : null}

    {assets.length > 0 ? <footer className="library-count">显示 {filteredAssets.length} / {assets.length} 条素材</footer> : null}

    <UploadDrawer open={uploadOpen} projectId={projectId} onClose={() => setUploadOpen(false)} onComplete={handleUploadComplete} />
  </section>
}
