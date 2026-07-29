import type { AgencyHealthStatus, ProjectProgressStage, ProjectRecord } from '../types'

export type ProjectProgressCalculation = {
  available: boolean
  stage: ProjectProgressStage | 'unavailable'
  stageLabel: string
  stagePercent: number | null
  taskPercent: number | null
  riskStatus: AgencyHealthStatus | 'unknown'
  blocker?: string
  reasons: string[]
  updatedAt: string
}

type StageDefinition = {
  stage: ProjectProgressStage
  label: string
  complete: (project: ProjectRecord) => boolean
  partial: (project: ProjectRecord) => number
}

const completedTaskStatuses = new Set<ProjectRecord['tasks'][number]['status']>(['ready', 'completed'])
const activeTaskStatuses = new Set<ProjectRecord['tasks'][number]['status']>(['draft', 'in_progress', 'ready'])
const taskStatusScore: Record<ProjectRecord['tasks'][number]['status'], number> = {
  draft: 15,
  in_progress: 55,
  ready: 85,
  completed: 100,
  failed: 0,
}

const stageDefinitions: StageDefinition[] = [
  {
    stage: 'intake',
    label: '需求下达',
    complete: project => hasConfirmedBrief(project),
    partial: project => project.artifacts.brief.id ? 60 : 20,
  },
  {
    stage: 'strategy',
    label: '策略确认',
    complete: project => hasStrategyEvidence(project),
    partial: project => hasConfirmedBrief(project) ? 55 : 10,
  },
  {
    stage: 'creative',
    label: '创意制作',
    complete: project => hasReadyCreative(project),
    partial: project => {
      const creativeTask = latestTask(project, task => task.type === 'creative' || task.type === 'brand_video')
      if (creativeTask) return taskStatusScore[creativeTask.status]
      return project.artifacts.creative.id ? 55 : hasStrategyEvidence(project) ? 30 : 10
    },
  },
  {
    stage: 'quality_check',
    label: '素材质检',
    complete: project => hasReadyCreative(project) && (hasQualityEvidence(project) || hasDeliveryEvidence(project)),
    partial: project => hasReadyCreative(project) ? 65 : 15,
  },
  {
    stage: 'human_review',
    label: '素材人工确认',
    complete: project => project.artifacts.creative.status === '已确认' || hasDeliveryEvidence(project),
    partial: project => hasQualityEvidence(project) ? 65 : hasReadyCreative(project) ? 35 : 10,
  },
  {
    stage: 'delivery',
    label: '投放预检',
    complete: project => project.changeSets.some(changeSet => changeSet.status === '已执行'),
    partial: project => {
      const changeSet = latestChangeSet(project)
      if (!changeSet) return project.artifacts.creative.status === '已确认' ? 35 : 10
      if (changeSet.status === '执行中') return 80
      if (changeSet.status === '已批准') return 70
      if (changeSet.status === '待审批') return 55
      if (changeSet.status === '草稿') return 35
      return 20
    },
  },
  {
    stage: 'completed',
    label: '项目完成',
    complete: project => project.status === '已完成',
    partial: project => project.changeSets.some(changeSet => changeSet.status === '已执行') ? 90 : 25,
  },
]

export function calculateProjectProgress(project: ProjectRecord): ProjectProgressCalculation {
  const reasons = validateProgressInputs(project)
  if (reasons.length) return unavailable(project, reasons)

  const firstIncompleteIndex = stageDefinitions.findIndex(stage => !stage.complete(project))
  const currentIndex = firstIncompleteIndex === -1 ? stageDefinitions.length - 1 : firstIncompleteIndex
  const currentStage = stageDefinitions[currentIndex]
  const stagePercent = clampPercent(currentStage.partial(project))
  const taskPercent = calculateTaskPercent(project)
  const riskStatus = calculateRiskStatus(project)
  const blocker = calculateBlocker(project, riskStatus)

  return {
    available: true,
    stage: currentStage.stage,
    stageLabel: currentStage.label,
    stagePercent: currentStage.stage === 'completed' && project.status === '已完成' ? 100 : stagePercent,
    taskPercent,
    riskStatus,
    blocker,
    reasons: [],
    updatedAt: project.updatedAt,
  }
}

export function progressPercentLabel(progress: ProjectProgressCalculation, key: 'stagePercent' | 'taskPercent' = 'taskPercent'): string {
  const value = progress[key]
  return progress.available && value !== null ? `${value}%` : '无法计算'
}

export function progressBarWidth(progress: ProjectProgressCalculation, key: 'stagePercent' | 'taskPercent' = 'taskPercent'): string {
  const value = progress[key]
  return progress.available && value !== null ? `${value}%` : '0%'
}

export function progressReasonLabel(progress: ProjectProgressCalculation): string {
  return progress.available ? progress.blocker ?? '当前进度可计算。' : `无法计算：${progress.reasons.join('；')}`
}

export function progressStatusLabel(progress: ProjectProgressCalculation): string {
  if (!progress.available) return '无法计算'
  if (progress.riskStatus === 'blocked') return '阻塞'
  if (progress.riskStatus === 'watch') return '需关注'
  return '正常'
}

function validateProgressInputs(project: ProjectRecord): string[] {
  const reasons: string[] = []
  if (!project.id) reasons.push('Project 尚未加载')
  if (!project.artifacts) reasons.push('缺少产物状态')
  const hasAnyEvidence = Object.values(project.artifacts ?? {}).some(artifact => Boolean(artifact.id))
    || project.tasks.length > 0
    || project.changeSets.length > 0
    || project.operations.some(record => record.kind === 'work_item')
  if (!hasAnyEvidence) reasons.push('缺少可验证的任务、产物或运营记录')

  const invalidProgressRecords = project.operations.filter(record => {
    if (record.kind !== 'work_item' || record.fields.progress === undefined) return false
    return !Number.isFinite(Number(record.fields.progress)) || Number(record.fields.progress) < 0 || Number(record.fields.progress) > 100
  })
  if (invalidProgressRecords.length) reasons.push(`${invalidProgressRecords[0].id} 的任务进度超出 0-100`)

  const laterEvidenceWithoutBrief = (hasReadyCreative(project) || project.changeSets.length > 0) && !hasConfirmedBrief(project)
  if (laterEvidenceWithoutBrief) reasons.push('后续阶段已有证据，但 Brief 未确认')
  return reasons
}

function unavailable(project: ProjectRecord, reasons: string[]): ProjectProgressCalculation {
  return {
    available: false,
    stage: 'unavailable',
    stageLabel: '无法计算',
    stagePercent: null,
    taskPercent: null,
    riskStatus: 'unknown',
    reasons,
    updatedAt: project.updatedAt,
  }
}

function calculateTaskPercent(project: ProjectRecord): number {
  const workItemProgress = project.operations
    .filter(record => record.kind === 'work_item' && record.fields.progress !== undefined)
    .map(record => Number(record.fields.progress))
    .filter(Number.isFinite)
  if (workItemProgress.length) return clampPercent(average(workItemProgress))
  if (project.tasks.length) return clampPercent(average(project.tasks.map(task => taskStatusScore[task.status])))
  return clampPercent(project.status === '已完成' ? 100 : 0)
}

function calculateRiskStatus(project: ProjectRecord): AgencyHealthStatus {
  if (project.tasks.some(task => task.status === 'failed')) return 'blocked'
  if (project.operations.some(record => /失败|异常|阻断/.test(record.status))) return 'blocked'
  if (project.changeSets.some(changeSet => changeSet.status === '草稿' || changeSet.status === '待审批')) return 'watch'
  if (project.tasks.some(task => activeTaskStatuses.has(task.status))) return 'watch'
  if (project.operations.some(record => /需处理|待审批|待评审|生成中/.test(record.status))) return 'watch'
  return 'healthy'
}

function calculateBlocker(project: ProjectRecord, riskStatus: AgencyHealthStatus): string | undefined {
  const failedTask = project.tasks.find(task => task.status === 'failed')
  if (failedTask) return `任务 ${failedTask.name} 失败，需先恢复。`
  const problemRecord = project.operations.find(record => /失败|异常|阻断|需处理/.test(record.status))
  if (problemRecord) return `${problemRecord.title}：${problemRecord.status}`
  const pendingChangeSet = project.changeSets.find(changeSet => changeSet.status === '草稿' || changeSet.status === '待审批')
  if (pendingChangeSet) return `${pendingChangeSet.id} 等待受控处理。`
  if (riskStatus === 'watch') return '存在进行中任务，需持续关注交接。'
  return undefined
}

function hasConfirmedBrief(project: ProjectRecord): boolean {
  if (Boolean(project.artifacts.brief.id) && project.artifacts.brief.status === '已确认') return true
  // Platform snapshots can contain task-to-asset references before the compact
  // artifact summary is materialized. A strategy task with a persisted source
  // asset is sufficient evidence that Brief intake has completed.
  return project.tasks.some(task => task.type === 'strategy' && task.sourceArtifactIds.length > 0)
}

function hasStrategyEvidence(project: ProjectRecord): boolean {
  return project.artifacts.strategy.status === '已确认'
    || project.tasks.some(task => task.type === 'strategy' && completedTaskStatuses.has(task.status))
    || hasReadyCreative(project)
    || hasDeliveryEvidence(project)
}

function hasReadyCreative(project: ProjectRecord): boolean {
  return Boolean(project.artifacts.creative.id) && ['已完成', '已确认'].includes(project.artifacts.creative.status)
}

function hasQualityEvidence(project: ProjectRecord): boolean {
  return project.operations.some(record => record.kind === 'unified_record' && /质检|审核|检查/.test(`${record.title} ${record.status}`))
}

function hasDeliveryEvidence(project: ProjectRecord): boolean {
  return project.changeSets.length > 0
}

function latestTask(project: ProjectRecord, predicate: (task: ProjectRecord['tasks'][number]) => boolean) {
  return project.tasks.filter(predicate).slice().sort((left, right) => right.updatedAt.localeCompare(left.updatedAt))[0]
}

function latestChangeSet(project: ProjectRecord) {
  return project.changeSets.slice().sort((left, right) => right.createdAt.localeCompare(left.createdAt))[0]
}

function average(values: number[]): number {
  return values.reduce((sum, value) => sum + value, 0) / values.length
}

function clampPercent(value: number): number {
  if (!Number.isFinite(value)) return 0
  return Math.min(100, Math.max(0, Math.round(value)))
}
