import type { ApiFatigueSignal } from '../../../data/api'
import { PinFindingButton, VerdictBadge } from '../shared'
import type { ViewProps } from './AnalysisPage'
import { formatCount, formatMoney, formatRate, formatSigned } from './format'
import { pinKey } from './usePinFinding'

const severityLabels: Record<ApiFatigueSignal['severity'], string> = {
  none: '没有迹象',
  watch: '需要观察',
  likely: '疑似疲劳',
}

const severityTone: Record<ApiFatigueSignal['severity'], string> = {
  none: 'muted',
  watch: 'warning',
  likely: 'danger',
}

export function FatigueView({ analysis, onPin, pinned, pinning }: ViewProps) {
  return <div className="insight-analysis-list" role="list" aria-label="疲劳">
    {(analysis.fatigue ?? []).map(item => {
      const target = { dimension: 'fatigue' as const, source_ref: item.asset_id }
      return <article className="insight-analysis-card" role="listitem" key={item.asset_id}>
        <header>
          <b>{item.asset_title}</b>
          {/* 「没有迹象」和「判断不了」对读者是两句相反的话：前者是查过了没问题，
              后者是压根没法查。看三档就够了——unclear 的定义就是「连有没有差异都判断不了」，
              前端不必再自己拼一遍 severity 和 confidence 的组合条件。 */}
          <em className={item.verdict === 'unclear' ? 'muted' : severityTone[item.severity]}>
            {item.verdict === 'unclear' ? '判断不了' : severityLabels[item.severity]}
          </em>
        </header>
        <p>{item.note}</p>

        <div className="insight-analysis-metrics">
          <div>
            <small>前半段</small>
            <b>{formatRate(item.first_rates.ctr)}</b>
            <span>曝光 {formatCount(item.first_half.impressions)} · 转化成本 {formatMoney(item.first_rates.cpa_cents)}</span>
          </div>
          <div>
            <small>后半段</small>
            <b>{formatRate(item.last_rates.ctr)}</b>
            <span>曝光 {formatCount(item.second_half.impressions)} · 转化成本 {formatMoney(item.last_rates.cpa_cents)}</span>
          </div>
        </div>

        <footer>
          点击率 {formatSigned(item.ctr_change)} · 转化成本 {formatSigned(item.cpa_change)} ·
          曝光 {formatSigned(item.impression_change)} <VerdictBadge judgement={item}/>
          {' '}<PinFindingButton onPin={pinning ? undefined : () => onPin(target)} pinned={pinned.has(pinKey(target))}/>
        </footer>

        {/* 只列「排除不了的」，不写「已排除」——受众构成数据一条都没接入，
            声称排除了受众变化就是编。 */}
        {item.alternative_explanations?.length ? <div className="insight-analysis-diff">
          <span>下面这些解释还没被排除</span>
          {item.alternative_explanations.map((reason, index) => <b key={index}>{reason}</b>)}
        </div> : null}
      </article>
    })}
  </div>
}
