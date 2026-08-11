// 三档结论的前端侧定义。后端权威定义在 internal/systems/insights/verdict.go，
// 这里的收敛表必须和它逐行对齐——两边各写一套，同一份数据就会在两个页面上
// 显示不同的档位，而没人解释得清哪个对。test/insight-verdict.test.ts 盯着。

import type { ApiConfidenceLevel } from './api'

export type Verdict = 'explained' | 'observed' | 'unclear'
export type UpgradePath = 'experiment' | 'similar_assets'

export interface Judgement {
  confidence: ApiConfidenceLevel
  verdict: Verdict
  verdict_label: string
  upgrade?: UpgradePath
  note: string
  // threshold_version 是判出这条结论时生效的阈值版本。0 = 出厂设定。
  // 可选是为了兼容还没带上它的老数据；缺失时 ThresholdStamp 什么都不显示，
  // 而不是替一条来历不明的结论说「按出厂阈值判定」。
  threshold_version?: number
}

export const verdictIcon: Record<Verdict, string> = {
  explained: '✅',
  observed: '👁',
  unclear: '❓',
}

export const verdictLabel: Record<Verdict, string> = {
  explained: '能归因',
  observed: '只是观察',
  unclear: '算不出来',
}

// tone 复用 styles.css 里已有的三个语义色，不新造颜色体系。
// 注意 tone 名 'ok' 对应的变量在这个仓库里叫 --success，映射写在 .verdict-ok 里。
export const verdictTone: Record<Verdict, 'ok' | 'warning' | 'muted'> = {
  explained: 'ok',
  observed: 'warning',
  unclear: 'muted',
}

// verdictOfConfidence 只给还没升级到新字段的旧接口兜底。新接口一律直接读
// 后端给的 verdict，不要在前端重算。
export function verdictOfConfidence(level: ApiConfidenceLevel): Verdict {
  switch (level) {
    case 'sufficient':
      return 'explained'
    case 'directional':
    case 'confounded':
      return 'observed'
    default:
      return 'unclear'
  }
}

const rank: Record<Verdict, number> = { unclear: 0, observed: 1, explained: 2 }

export function weakestVerdict(items: readonly { verdict: Verdict }[]): Verdict {
  if (items.length === 0) return 'unclear'
  return items.reduce<Verdict>(
    (weakest, item) => (rank[item.verdict] < rank[weakest] ? item.verdict : weakest),
    'explained',
  )
}

export function upgradeLabel(path: UpgradePath | undefined): string {
  if (path === 'experiment') return '做个实验'
  if (path === 'similar_assets') return '找相似素材'
  return ''
}
