import type { ApiAssetTrend } from '../../../data/api'
import { PinFindingButton, VerdictBadge } from '../shared'
import type { ViewProps } from './AnalysisPage'
import { formatCount, formatDate, formatMoney, formatRate, formatSigned } from './format'
import { pinKey } from './usePinFinding'

/**
 * 趋势。每个素材在这段时间里是往上走还是往下走。
 *
 * 「看不出」和「持平」是两句不同的话：持平是查过了没变化，看不出是有数据的
 * 天数不够，连方向都判不了。
 */
const directionLabels: Record<ApiAssetTrend['direction'], string> = {
  rising: '上行',
  flat: '持平',
  declining: '下行',
  unknown: '看不出',
}

export function TrendView({ analysis, onPin, pinned, pinning }: ViewProps) {
  return <div className="insight-analysis-list" role="list" aria-label="趋势">
    {(analysis.trends ?? []).map(item => {
      const points = item.points ?? []
      // 条形按窗口内的峰值等比缩放。峰值兜到 1，是为了一整段全零时不去除以 0。
      const peak = Math.max(...points.map(point => point.counts.impressions), 1)
      const target = { dimension: 'trends' as const, source_ref: item.asset_id }
      return <article className="insight-analysis-card" role="listitem" key={item.asset_id}>
        <header>
          <b>{item.asset_title}</b>
          <em className={item.direction === 'declining' ? 'danger' : item.direction === 'rising' ? 'ok' : 'muted'}>
            {directionLabels[item.direction]}
          </em>
        </header>
        <p>{item.note}</p>
        <div className="insight-series">
          <span className="section-label">
            {item.active_days} 天有数据 · 点击率变化 {formatSigned(item.ctr_change)} <VerdictBadge judgement={item}/>
          </span>
          {points.map(point => <div className="insight-series-row" key={point.date}>
            <span>{formatDate(point.date)}</span>
            <span className="insight-series-track">
              <i style={{ width: `${Math.max(2, Math.round((point.counts.impressions / peak) * 100))}%` }}/>
            </span>
            <span>曝光 {formatCount(point.counts.impressions)}</span>
            <span>点击率 {formatRate(point.rates.ctr)}</span>
            <span>花费 {formatMoney(point.counts.spend_cents)}</span>
          </div>)}
        </div>
        <footer>
          <PinFindingButton onPin={pinning ? undefined : () => onPin(target)} pinned={pinned.has(pinKey(target))}/>
        </footer>
      </article>
    })}
  </div>
}
