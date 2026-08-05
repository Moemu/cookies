// 素材 ID → 素材名字的对照。
//
// 经验的「样例素材版本」存的是素材 ID。ID 对机器有用，对人没有：经验卡上写着
// insightasset_01J8… 的人不会去查那是哪条片子，于是这一格等于没写。凡是要把这串 ID
// 显示给人看的地方，都从这里换成标题；查不到的照原样显示 ID，不编一个名字出来。
import { useCallback, useEffect, useState } from 'react'
import { api, type ApiInsightAsset } from './api'
import { shortId } from './shortId'

export type InsightAssetIndex = {
  assets: ApiInsightAsset[]
  loading: boolean
  /** 拿 ID 换一个能读的名字。查不到就退回短 ID——宁可显示 ID，也不能显示一个不存在的素材名。 */
  labelOf: (assetId: string) => string
  /** 这个 ID 在当前 Project 里能不能找到。找不到通常意味着当初是手打上去的。 */
  known: (assetId: string) => boolean
}

export function useInsightAssets(projectId: string, limit = 200): InsightAssetIndex {
  const [assets, setAssets] = useState<ApiInsightAsset[]>([])
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    if (!projectId) {
      setAssets([])
      return
    }
    let active = true
    setLoading(true)
    void api.listInsightAssets(projectId, { limit }).then(page => {
      if (!active) return
      setAssets(page.items)
    }).catch(() => {
      // 查不到素材不该拖垮整屏。对照表空着，显示就退回短 ID。
      if (active) setAssets([])
    }).finally(() => {
      if (active) setLoading(false)
    })
    return () => { active = false }
  }, [projectId, limit])

  const labelOf = useCallback((assetId: string) => {
    const asset = assets.find(item => item.id === assetId)
    return asset ? `${asset.title}（v${asset.revision}）` : shortId(assetId)
  }, [assets])

  const known = useCallback((assetId: string) => assets.some(item => item.id === assetId), [assets])

  return { assets, loading, labelOf, known }
}
