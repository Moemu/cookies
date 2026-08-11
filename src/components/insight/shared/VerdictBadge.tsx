import { type Judgement, upgradeLabel, verdictIcon, verdictTone } from '../../../data/verdict'

// 三档徽章。整个模块只有这一个地方决定档位长什么样——以前每个页面自己写一段
// confidence → 文案的映射，改一次要改七处，改漏一处就有一页在说另一套话。
export function VerdictBadge({ judgement, onUpgrade }: {
  judgement: Judgement
  // onUpgrade 给出去时才显示升级按钮。不是每一屏都有地方接这个动作。
  onUpgrade?: () => void
}) {
  const upgrade = upgradeLabel(judgement.upgrade)
  return (
    <span className={`verdict-badge verdict-${verdictTone[judgement.verdict]}`} title={judgement.note}>
      <span aria-hidden="true">{verdictIcon[judgement.verdict]}</span>
      <span>{judgement.verdict_label}</span>
      {upgrade && onUpgrade ? (
        <button type="button" className="verdict-upgrade" onClick={onUpgrade}>{upgrade}</button>
      ) : null}
    </span>
  )
}
