import type { Experience, ExperienceReferenceOutcome, ExperienceStatus } from './types'

export const experienceStatusLabels: Record<ExperienceStatus, string> = {
  pending: '待确认',
  confirmed: '已确认',
  needs_review: '待复审',
  retired: '已失效',
}

export const experienceStatusOrder: ExperienceStatus[] = ['pending', 'confirmed', 'needs_review', 'retired']

export const referenceOutcomeLabels: Record<ExperienceReferenceOutcome, string> = {
  referenced: '仅引用',
  adopted: '直接采纳',
  modified: '修改后使用',
  rejected: '未采纳',
}

// 下游类型是自由文本，认识的翻成中文，不认识的原样显示。
const consumerKindLabels: Record<string, string> = {
  strategy: '策略包',
  creative: '创意任务',
  delivery: '投放计划',
  report: '复盘报告',
}

export const consumerKinds = Object.keys(consumerKindLabels)

export function consumerKindLabel(kind: string) {
  return consumerKindLabels[kind] || kind
}

export function groupExperiencesByStatus(experiences: Experience[]) {
  const result = { pending: [], confirmed: [], needs_review: [], retired: [] } as Record<ExperienceStatus, Experience[]>
  for (const item of experiences) result[item.status].push(item)
  return result
}

// 表单里的多行文本按行拆成条目，空行忽略。
export function toLines(value: string) {
  return value.split('\n').map((item) => item.trim()).filter(Boolean)
}
