import type { ApiMetricAnomaly } from '../../../data/api'
import { PinFindingButton } from '../shared'
import type { ViewProps } from './AnalysisPage'
import { formatDate, formatNumber } from './format'
import { pinKey } from './usePinFinding'

const anomalyLabels: Record<ApiMetricAnomaly['kind'], string> = {
  spike: '异常偏高',
  drop: '异常偏低',
  gap: '当天没有数据',
}

// 后端传回来的是指标键名。整页都是产品语言，这一列漏出 impressions 会让人
// 以为它是个需要认识的技术字段。认不出来的键就原样显示，不编一个中文名。
const metricLabels: Record<string, string> = {
  impressions: '曝光',
  clicks: '点击',
  spend_cents: '花费',
  conversions: '转化',
  revenue_cents: '收入',
}

export function AnomalyView({ analysis, onPin, pinned, pinning }: ViewProps) {
  return <div className="prelaunch-table" role="list" aria-label="异常">
    <div className="prelaunch-row insight-anomaly-row header">
      <span>日期与范围</span><span>指标</span><span>类型</span><span>实测 / 常态</span><span>偏离</span>
    </div>
    {(analysis.anomalies ?? []).map((item, index) => {
      // 异常的主语是「哪个素材的哪一天」，所以变量用日期。整个项目那一档
      // 没有素材 id，只靠日期区分——它本来就一天只有一条。
      const target = { dimension: 'anomalies' as const, source_ref: item.asset_id, variable: item.date }
      return <div role="listitem" className="prelaunch-row insight-anomaly-row"
        key={`${item.date}-${item.scope}-${item.asset_id ?? ''}-${item.metric}-${index}`}>
        <span><b>{formatDate(item.date)}</b><small>{item.scope === 'project' ? '整个项目' : item.asset_title || item.asset_id}</small></span>
        <span>{metricLabels[item.metric] ?? item.metric}</span>
        <span>{anomalyLabels[item.kind]}</span>
        {/* 断档行没有「实测」这回事：那天根本没有数据。写 0 会被读成「测到了 0 次曝光」，
            那是另一件事——停投和没回流在数字上都不是 0。 */}
        <span>{item.kind === 'gap' ? '—' : `${formatNumber(item.observed)} / ${formatNumber(item.median)}`}</span>
        <span>{item.kind === 'gap' ? '—' : `${item.deviation.toFixed(1)} 倍 MAD`}</span>
        {/* 记一笔放进这条通栏的说明里，不单开一列：五列的表已经很挤，
            再挤一列会把日期那格压到换行。 */}
        <small className="insight-anomaly-note">
          {item.note}
          {' '}<PinFindingButton onPin={pinning ? undefined : () => onPin(target)} pinned={pinned.has(pinKey(target))}/>
        </small>
      </div>
    })}
  </div>
}
