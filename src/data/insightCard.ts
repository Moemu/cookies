// 洞察卡九字段（03 §8.1）的展示口径。
//
// 同一条经验会出现在两屏上：经验库里是「待确认的结论」，投前洞察里是「可引用的证据」。
// 两屏的类型名、置信名、适用范围怎么读，必须是同一套——两份各写各的，迟早会出现同一条
// 经验在两个地方显示成不同的置信。
import type { ApiApplicability, ApiConfidenceLevel, ApiContentBasis, ApiDataBasis, ApiInsightCardType } from './api'

export const cardTypeLabels: Record<ApiInsightCardType, string> = {
  fact: '事实',
  statistic: '统计观察',
  hypothesis: '假设',
  recommendation: '建议',
}

// 类型不是标签，是这条结论能被怎么用。写在旁边，避免有人拿假设当证据。
export const cardTypeMeaning: Record<ApiInsightCardType, string> = {
  fact: '数据里直接读到的，没有加工。可以直接引用。',
  statistic: '在数据上算出来的观察，口径写在数据依据里。可以引用，但要连口径一起引。',
  hypothesis: '还没被验证的猜测。可以拿去做实验，不能拿去当证据。',
  recommendation: '人给出的行动建议。它的说服力来自它引用的证据，不来自它自己。',
}

export const confidenceLabels: Record<ApiConfidenceLevel, string> = {
  sufficient: '充分',
  directional: '方向性',
  low_sample: '样本不足',
  confounded: '存在混杂',
}

export function describeApplicability(scope: ApiApplicability): string {
  const parts = [
    ...(scope.channels ?? []),
    ...(scope.creative_types ?? []),
    ...(scope.objectives ?? []),
    ...(scope.audiences ?? []),
  ]
  if (!parts.length) return '没写适用范围——这条结论没说自己适用于哪里'
  return parts.join(' · ') + (scope.time_range_note ? ` · ${scope.time_range_note}` : '')
}

export function describeDataBasis(basis: ApiDataBasis): string {
  const parts: string[] = []
  if (basis.asset_count) parts.push(`${basis.asset_count} 个素材`)
  if (basis.sample_size) parts.push(`样本 ${basis.sample_size.toLocaleString('zh-CN')}`)
  if (basis.metrics?.length) parts.push(basis.metrics.join('、'))
  if (basis.window_start && basis.window_end) parts.push(`${formatBasisDate(basis.window_start)} ~ ${formatBasisDate(basis.window_end)}`)
  if (basis.baseline) parts.push(`对照：${basis.baseline}`)
  return parts.length ? parts.join(' · ') : '没写数据依据'
}

export function describeContentBasis(basis: ApiContentBasis): string {
  const parts: string[] = []
  if (basis.features?.length) parts.push(basis.features.join('、'))
  if (basis.example_asset_versions?.length) parts.push(`示例素材 ${basis.example_asset_versions.length} 个`)
  if (basis.note) parts.push(basis.note)
  return parts.length ? parts.join(' · ') : '没写内容依据——不知道这条结论指的是素材上的哪个特征'
}

function formatBasisDate(value: string): string {
  const date = new Date(value)
  return Number.isNaN(date.valueOf()) ? value : date.toLocaleDateString('zh-CN')
}
