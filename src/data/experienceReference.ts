import type { ApiExperienceReference } from './api'

/**
 * 引用记录的中文说法。经验库和投前洞察都在显示同一批记录，措辞必须一致——
 * 同一条记录在两页读出两种意思，人只会更不敢信它。
 *
 * consumer_kind 后端没有受控词表（自由文本，≤32 字），这里只翻译已知的几种，
 * 其余原样显示——猜错比显示英文更糟。
 */
const consumerLabels: Record<string, string> = {
  brief: 'Brief',
  strategy: '策略',
  creative_task: '创意任务',
  creative: '创意',
  report: '报告',
  experiment: '实验',
}

export function consumerLabel(kind: string): string {
  return consumerLabels[kind] ?? kind
}

const outcomeLabels: Record<string, string> = {
  referenced: '已引用',
  adopted: '照做了',
  modified: '改了之后用的',
  rejected: '没采纳',
}

/**
 * 实验写回来的那条记录要换一套说法。同样是 rejected，人引用时是「我看了但没采纳」，
 * 实验写回来时是「数据把这条假设推翻了」——后者跟人愿不愿意用没有关系，
 * 照搬「没采纳」会让人以为是谁不想用它。
 */
const experimentOutcomeLabels: Record<string, string> = {
  referenced: '实验看不出差别',
  adopted: '实验支持这条假设',
  modified: '实验后改过',
  rejected: '实验推翻了这条假设',
}

export function outcomeLabel(reference: Pick<ApiExperienceReference, 'consumer_kind' | 'outcome'>): string {
  const labels = reference.consumer_kind === 'experiment' ? experimentOutcomeLabels : outcomeLabels
  return labels[reference.outcome] ?? reference.outcome ?? '未记录'
}
