import { useEffect, useState } from 'react'
import { CircleAlert, ExternalLink, Layers3, Sparkles } from 'lucide-react'
import { useProject } from '../../../context/ProjectContext'
import { api, type ApiInsightAsset, type ApiInsightAssetFeature } from '../../../data/api'
import { admissibleForAttribution, featureSourceLabel } from '../../../data/featureSource'
import { shortId } from '../../../data/shortId'

const statusLabels: Record<string, string> = {
  awaiting_data: '待数据', awaiting_match: '待匹配', analysable: '可分析', analysing: '分析中',
  pending_confirmation: '待确认', confirmed: '已确认', needs_review: '待复审', retired: '已失效',
}

/**
 * 单条素材。
 *
 * **上半截和下半截视觉上是分开的**：上半截是素材库那边的东西（预览、时长、版本、
 * 血缘），下半截才是洞察自己的（内容变量、投放数据、分析状态）。分开是为了让人
 * 知道哪些东西改了要去别处改——混在一屏里，人会在这里找编辑时长的按钮然后找不到。
 *
 * 上半截现在**没有数据可读**：洞察侧只存了 PlatformAssetID / PlatformAssetVersion
 * 两个裸字段，没有真链接。所以这里只显示这两个号码和跳转，并明写「预览与版本信息
 * 在素材库那边」。不为了让它好看而在洞察这边复制一份素材元数据——复制出来的那份
 * 第二天就和素材库对不上了。真接上是创意侧交接时的事。
 */
export function AssetDetail({ asset, onOpenLibrary }: {
  asset: ApiInsightAsset
  onOpenLibrary: () => void
}) {
  const { currentProject } = useProject()
  const [features, setFeatures] = useState<ApiInsightAssetFeature[]>([])

  useEffect(() => {
    let active = true
    void api.listInsightAssetFeatures(currentProject.id, asset.id)
      .then(page => { if (active) setFeatures(page.items) })
      .catch(() => { if (active) setFeatures([]) })
    return () => { active = false }
  }, [currentProject.id, asset.id])

  const admissible = features.filter(feature => admissibleForAttribution(feature.source))

  return <div className="asset-detail">
    <section className="asset-detail-upper">
      <div className="asset-detail-upper-head">
        <span className="section-label">素材库那边的</span>
        <button type="button" className="secondary-button" onClick={onOpenLibrary}>
          <ExternalLink size={14}/>去素材库
        </button>
      </div>
      <h3>{asset.title}</h3>
      <p>预览与版本信息在素材库那边。洞察这边只记住它是哪一条，不复制一份元数据过来
        ——复制出来的那份第二天就和素材库对不上了。</p>
      <div className="prelaunch-fact"><Layers3 size={17}/><span><small>平台素材号</small><b>
        {asset.platform_asset_id || '还没有。这条素材还没和平台上的对象对上号。'}
        {asset.platform_asset_version ? ` · 第 ${asset.platform_asset_version} 版` : ''}
      </b></span></div>
    </section>

    <section className="asset-detail-lower">
      <span className="section-label">洞察这边的</span>
      <div className="prelaunch-fact"><Layers3 size={17}/><span><small>分析状态</small><b>
        {statusLabels[asset.analysis_status] ?? asset.analysis_status}
        {asset.analysis_status_reason ? ` · ${asset.analysis_status_reason}` : ''}
      </b></span></div>

      <div className="feature-stack">
        <span>内容变量（{features.length} 项 · 其中 {admissible.length} 项能进归因）</span>
        {features.length ? features.map(feature => <b key={feature.id}>
          {feature.key}：{formatValue(feature.value)}
          <small>（{featureSourceLabel[feature.source]}）</small>
        </b>) : <b>还没有内容变量。去「素材 · 变量」给它提一次。</b>}
      </div>

      {features.length && !admissible.length ? <div className="prelaunch-boundary">
        <CircleAlert size={16}/><span><small>这些变量还不能拿来归因</small>
          它们全是模型猜的。要让它们算数，得有人逐项看过并确认——在「素材 · 变量」里做。
        </span></div> : null}

      <div className="prelaunch-fact"><Sparkles size={17}/><span><small>编号</small><b>
        {shortId(asset.id)} · 血缘 {shortId(asset.lineage_id)} · 第 {asset.revision} 版
      </b></span></div>
    </section>
  </div>
}

function formatValue(value: ApiInsightAssetFeature['value']): string {
  if (value.text) return value.text
  if (value.terms?.length) return value.terms.join('、')
  if (value.number !== undefined) return String(value.number)
  if (value.bool !== undefined) return value.bool ? '是' : '否'
  return '—'
}
