import type { ApiSimilarAssetResult } from '../../../data/api'
import { featureSourceLabel } from '../../../data/featureSource'
import { shortId } from '../../../data/shortId'

/**
 * 相似素材结果。每条都列出「像在哪」——这是这个功能和一个相似度分数的全部区别。
 *
 * 单独一个文件，是因为它有两个调用方：「素材 · 找相似」这一屏，和分析页上
 * ❓「算不出来」那个占位里的升级通道（Task 6）。两边渲染成两个样子的话，
 * 同一批结果在两屏上会被读成两回事。
 */
export function SimilarPanel({ result }: { result: ApiSimilarAssetResult }) {
  return <div className="similar-panel">
    {/* 探针要显示出来。不写「按哪几个变量找的」，人没法判断这批结果值不值得信
        ——按 5 个变量凑齐的一致，和按 1 个变量凑出来的一致，分量差很远。 */}
    {result.probe.length ? <p className="similar-probe">
      按这些变量找的：{result.probe.map(item => `${item.label}=${item.value}`).join('、')}
    </p> : null}

    {result.note ? <p className="similar-note">{result.note}</p> : null}

    {result.items.length ? <ul className="similar-list">
      {result.items.map(item => <li key={item.asset_id}>
        <div className="similar-head">
          <strong>{item.title || shortId(item.asset_id)}</strong>
          <span className="similar-count">重叠 {item.overlap} 个变量
            {item.admissible_overlap < item.overlap
              ? `（其中 ${item.admissible_overlap} 个能进归因）` : ''}</span>
        </div>
        <ul className="similar-reasons">
          {item.reasons.map(reason => <li key={reason.key}>
            {reason.label} = {reason.value}
            <small>（{featureSourceLabel[reason.source]}）</small>
          </li>)}
        </ul>
      </li>)}
    </ul> : <p className="panel-empty">没有在这些变量上一致的素材。</p>}
  </div>
}
