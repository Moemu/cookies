import type { ApiCreativeDirectionBatch, ApiCreativeTaskSummary } from '../../data/api'

export const channelNeutralBrandDirectionPromptVersion = 'creative-direction/strategy-handoff-v4'

export function brandDirectionFailureMessage(code?: string) {
  switch (code) {
    case 'DIRECTION_PROVIDER_FAILED':
      return '模型服务响应超时或暂时不可用，请稍后重试。'
    case 'DIRECTION_QUALITY_VALIDATION_FAILED':
      return '生成结果未通过差异度或品牌质量校验，请重新生成。'
    case 'DIRECTION_INPUT_UNAVAILABLE':
    case 'DIRECTION_INPUT_CHANGED':
      return '上游策略输入已失效或发生变化，请返回策略页确认后重试。'
    case 'DIRECTION_JOB_SCHEDULE_FAILED':
      return '生成任务未能进入队列，请重试。'
    default:
      return '品牌方向生成失败，请重新生成；若持续失败，请检查模型服务配置。'
  }
}

export function availableBrandDirections(batch: ApiCreativeDirectionBatch) {
  const candidates = batch.candidates ?? []
  const confirmed = candidates.find(direction => direction.status === 'confirmed')
  return confirmed ? [confirmed] : candidates.filter(direction => direction.status === 'candidate')
}

export function isChannelNeutralBrandDirectionBatch(batch: ApiCreativeDirectionBatch) {
  return batch.prompt_version === channelNeutralBrandDirectionPromptVersion
}

export function isBrandDirectionGenerating(batch: ApiCreativeDirectionBatch | null) {
  return batch?.status === 'generating'
}

export function activeBrandVideoTasks(tasks: ApiCreativeTaskSummary[]) {
  return tasks
    .filter(task => task.format === 'video' && task.performance_mode === 'brand_video' && task.status !== 'archived')
    .sort((left, right) => Date.parse(right.updated_at) - Date.parse(left.updated_at))
}

export function brandVideoTaskStatusLabel(status: string) {
  switch (status) {
    case 'ready':
      return '可继续制作'
    case 'in_progress':
    case 'generating':
      return '制作中'
    case 'completed':
    case 'delivered':
      return '已完成'
    case 'failed':
      return '需处理'
    default:
      return '待完善'
  }
}
