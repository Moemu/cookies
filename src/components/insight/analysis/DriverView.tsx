import { PinFindingButton, VerdictBadge } from '../shared'
import type { ViewProps } from './AnalysisPage'
import { formatCount, formatMoney, formatRate, formatSigned } from './format'
import { pinKey } from './usePinFinding'

/**
 * 驱动因素。哪些内容特征和表现一起变化。
 *
 * 一起变化不等于起了作用：组内还有别的特征同向变化时，后端会把它标成
 * 「分不开」，这里照样把那几个特征原样列出来——人得看见分不开的是什么。
 */
export function DriverView({ analysis, onPin, pinned, pinning }: ViewProps) {
  return <div className="insight-analysis-list" role="list" aria-label="驱动因素">
    {(analysis.drivers ?? []).map(item => {
      // 驱动因素说的是「某个特征取某个值时怎么样」，没有素材主语，所以只给变量。
      const target = { dimension: 'drivers' as const, variable: item.key }
      return <article className="insight-analysis-card" role="listitem"
        key={`${item.asset_type ?? ''}-${item.key}-${item.value}`}>
        <header>
          <b>{item.label}：{item.value}</b>
          <VerdictBadge judgement={item}/>
        </header>
        <p>{item.note}</p>

        <div className="insight-analysis-metrics">
          <div>
            <small>这一组（{item.assets} 个素材）</small>
            <b>{formatRate(item.rates.ctr)}</b>
            <span>
              曝光 {formatCount(item.counts.impressions)} · 转化成本 {formatMoney(item.rates.cpa_cents)}
              <br/>{item.ctr_interval ? `95% 区间 ${formatRate(item.ctr_interval.low)} ~ ${formatRate(item.ctr_interval.high)}` : '样本不足以给出区间'}
            </span>
          </div>
          <div>
            <small>其余素材（{item.rest_assets} 个）</small>
            <b>{formatRate(item.rest_rates.ctr)}</b>
            <span>
              曝光 {formatCount(item.rest_counts.impressions)} · 转化成本 {formatMoney(item.rest_rates.cpa_cents)}
              <br/>{item.rest_ctr_interval ? `95% 区间 ${formatRate(item.rest_ctr_interval.low)} ~ ${formatRate(item.rest_ctr_interval.high)}` : '样本不足以给出区间'}
            </span>
          </div>
        </div>

        <footer>
          点击率相对差异 {formatSigned(item.ctr_lift)} ·
          {item.intervals_overlap ? ' 两组置信区间重叠，差异可能只是噪声' : ' 两组置信区间不重叠'}
          {' '}<PinFindingButton onPin={pinning ? undefined : () => onPin(target)} pinned={pinned.has(pinKey(target))}/>
        </footer>

        {item.covarying_features?.length ? <div className="insight-analysis-diff">
          <span>这些特征跟着一起变了，分不开谁在起作用</span>
          {item.covarying_features.map((feature, index) => <b key={index}>{feature}</b>)}
        </div> : null}
      </article>
    })}
  </div>
}
