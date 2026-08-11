import type { ApiMetricRates, ApiVariantVerdict } from '../../../data/api'
import { PinFindingButton, VerdictBadge } from '../shared'
import type { ViewProps } from './AnalysisPage'
import { formatCount, formatMoney, formatRate, formatSigned } from './format'
import { pinKey } from './usePinFinding'

/**
 * 素材对比。同类型素材两两配对，先识别变量，再谈差异。
 *
 * 这里的五档不是三档：它多回答一件事——归不了因，是因为变量太多，还是因为
 * 压根没有特征数据。判定文案刻意写成「能不能」而不是「好不好」：读者要先知道
 * 这条结论能不能用。三档（能不能归因）由行尾的 VerdictBadge 给。
 */
const verdictLabels: Record<ApiVariantVerdict, string> = {
  attributable: '可归因到该变量',
  directional: '只能看方向',
  confounded: '归不了因',
  low_sample: '样本不足',
  no_features: '缺内容特征',
}

const verdictTone: Record<ApiVariantVerdict, string> = {
  attributable: 'ok',
  directional: 'warning',
  confounded: 'danger',
  low_sample: 'muted',
  no_features: 'muted',
}

export function ComparisonView({ analysis, onPin, pinned, pinning }: ViewProps) {
  return <div className="insight-analysis-list" role="list" aria-label="素材对比">
    {(analysis.comparisons ?? []).map(item => {
      // 去重键取第一个改动的变量，和后端一致。一组对比改了不止一个变量时，
      // 它本来就归不了因，进复盘只是作为观察，键撞不撞无所谓。
      const target = {
        dimension: 'comparisons' as const,
        source_ref: item.variant_asset_id,
        variable: (item.changed_features ?? [])[0]?.key,
      }
      // 归因不可用的变量不给记一笔：这条差异里有模型推断出来的东西，
      // 把它记进复盘，下一轮就会有人拿一个猜出来的变量当依据。
      const inadmissible = (item.changed_features ?? []).some(diff => !diff.admissible)
      return <article className="insight-analysis-card" role="listitem"
        key={`${item.baseline_asset_id}-${item.variant_asset_id}`}>
        <header>
          <b>{item.baseline_title} ↔ {item.variant_title}</b>
          <em className={verdictTone[item.variant_verdict]}>{verdictLabels[item.variant_verdict]}</em>
        </header>
        <p>{item.note}</p>

        <div className="insight-analysis-diff">
          <span>改动的变量（{(item.changed_features ?? []).length} 个）· 两侧一致的特征 {item.controlled_count} 个</span>
          {(item.changed_features ?? []).length
            ? (item.changed_features ?? []).map(diff => <b key={diff.key}>
                {diff.label}：{diff.baseline} → {diff.variant}{diff.admissible ? '' : '（模型推断，不计入结论）'}
              </b>)
            : <b>已记录的特征上两边完全一致。</b>}
        </div>

        <div className="insight-analysis-metrics">
          <Side title={item.baseline_title} rates={item.baseline_rates}
            impressions={item.baseline_counts.impressions} interval={item.baseline_ctr_interval}/>
          <Side title={item.variant_title} rates={item.variant_rates}
            impressions={item.variant_counts.impressions} interval={item.variant_ctr_interval}/>
        </div>

        <footer>
          点击率相对变化 {formatSigned(item.ctr_lift)} ·
          {item.intervals_overlap ? ' 两侧置信区间重叠，这个差异可能只是噪声' : ' 两侧置信区间不重叠'} ·
          {' '}<VerdictBadge judgement={item}/>
          {' '}<PinFindingButton
            onPin={inadmissible || pinning ? undefined : () => onPin(target)}
            pinned={pinned.has(pinKey(target))}
            reason={inadmissible ? '这条差异里有模型推断的变量，不能进结论。' : undefined}/>
        </footer>
      </article>
    })}
  </div>
}

function Side({ title, rates, impressions, interval }:
  { title: string; rates: ApiMetricRates; impressions: number; interval?: { low: number; high: number } }) {
  return <div>
    <small>{title}</small>
    <b>{formatRate(rates.ctr)}</b>
    <span>
      曝光 {formatCount(impressions)} · 转化成本 {formatMoney(rates.cpa_cents)}
      <br/>{interval ? `95% 区间 ${formatRate(interval.low)} ~ ${formatRate(interval.high)}` : '样本不足以给出区间'}
    </span>
  </div>
}
